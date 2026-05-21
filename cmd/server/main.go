package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omnitun/omnitun/internal/abuse"
	"github.com/omnitun/omnitun/internal/announcements"
	"github.com/omnitun/omnitun/internal/audit"
	"github.com/omnitun/omnitun/internal/auth"
	"github.com/omnitun/omnitun/internal/billing"
	"github.com/omnitun/omnitun/internal/featureflags"
	"github.com/omnitun/omnitun/internal/gateway"
	"github.com/omnitun/omnitun/pkg/clickhouse"
	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
	"github.com/omnitun/omnitun/pkg/config"
	apperrors "github.com/omnitun/omnitun/pkg/errors"
	omnilog "github.com/omnitun/omnitun/pkg/log"
	"github.com/omnitun/omnitun/pkg/metrics"
	"github.com/omnitun/omnitun/pkg/tracing"
	"golang.org/x/sync/errgroup"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := omnilog.New(cfg.LogLevel, "json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse database URL", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 25
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.ErrorContext(ctx, "database ping failed", "error", err)
		os.Exit(1)
	}
	logger.InfoContext(ctx, "database connected")

	chClient := (*clickhouse.Client)(nil)
	if cfg.ClickHouseURL != "" {
		chClient = clickhouse.NewClient(cfg.ClickHouseURL)
	}

	repo := auth.NewRepository(pool)
	jwtMgr, err := auth.NewJWTManager(cfg.Auth)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create JWT manager", "error", err)
		os.Exit(1)
	}
	oauthMgr := auth.NewOAuthManager(cfg, repo, jwtMgr)
	oidcMgr := auth.NewOIDCManager(cfg.Auth, repo, jwtMgr)
	authSvc := auth.NewService(repo, jwtMgr, oauthMgr)

	auditLogger := audit.NewLogger(pool)

	announceMgr := announcements.NewManager(pool)

	usageTracker := billing.NewUsageTracker(pool)

	flagMgr := featureflags.NewManager(pool)
	go flagMgr.StartCacheRefresh(ctx)

	abuseMgr := abuse.NewManager(pool)

	gwAuthMgr := gateway.NewAuthManager(cfg.Auth.JWTSecret, nil)
	gwCfg := &gateway.Config{
		JWTSecret: cfg.Auth.JWTSecret,
	}
	gwServer := gateway.NewServer(gwCfg, gwAuthMgr)
	defer gwServer.Hub.Shutdown()
	go gwServer.Hub.HeartbeatCheck(ctx)

	tracing.Init(tracing.Config{
		ServiceName: "omnitun-server",
		Enabled:     true,
	})

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(tracingMiddleware)
	r.Use(corsMiddleware)

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" { return true } // same-origin requests
			return strings.HasPrefix(origin, "http://localhost") || 
			       strings.HasPrefix(origin, "http://127.0.0.1")
		},
	}
	r.Get("/ws", handleWebSocket(upgrader, jwtMgr))

	r.Get("/v1/announcements/active", handleActiveAnnouncements(announceMgr))
	r.Get("/v1/releases/latest", handleLatestRelease)
	r.With(auth.JWTAuthMiddleware(jwtMgr)).Get("/v1/status", handleStatus(repo, pool, usageTracker))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","database":"connected"}`))
	})

	r.Get("/agent/v1/connect", gwServer.HandleAgentConnect)
	r.Get("/gateway/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"agents":%d}`, gwServer.Hub.AgentCount())
	})

	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", handleRegister(authSvc, auditLogger))
		r.Post("/login", handleLogin(authSvc, auditLogger))
		r.Post("/password/forgot", handleForgotPassword(repo, pool))
		r.Post("/password/reset", handleResetPassword(repo, pool))
		r.Get("/oauth/github", handleOAuthLogin("github", oauthMgr))
		r.Get("/oauth/google", handleOAuthLogin("google", oauthMgr))
		r.Get("/oauth/callback", handleOAuthCallback(oauthMgr))
		r.Get("/saml/login", handleSAMLLogin(cfg))
		r.Post("/saml/acs", handleSAMLACS(cfg, authSvc, repo, jwtMgr))
		r.Get("/saml/metadata", handleSAMLLMetadata(cfg))
		r.Get("/oidc/login", handleOIDCLogin(cfg))
		r.Get("/oidc/callback", handleOIDCCallback(oidcMgr, cfg))
		r.With(auth.JWTAuthMiddleware(jwtMgr)).Get("/me", handleMe(repo))
		r.With(auth.JWTAuthMiddleware(jwtMgr)).Get("/mfa/enroll", handleEnrollMFA(authSvc))
		r.With(auth.JWTAuthMiddleware(jwtMgr)).Post("/mfa/verify", handleVerifyMFA(authSvc))
		r.With(auth.JWTAuthMiddleware(jwtMgr)).Post("/mfa/disable", handleDisableMFA(authSvc))
	})

	r.Route("/v1/dashboard", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin", "editor"))
		r.Get("/stats", handleDashboardStats(repo, pool))
		r.Get("/events", handleDashboardEvents(repo, pool))
	})

	r.Route("/v1/tunnels", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin", "editor"))
		r.Get("/", handleListTunnels(repo, pool))
		r.Post("/", handleCreateTunnel(repo, pool, auditLogger, usageTracker))
		r.Get("/{tunnelID}", handleGetTunnel(repo, pool))
		r.Patch("/{tunnelID}", handleUpdateTunnel(repo, pool))
		r.Delete("/{tunnelID}", handleDeleteTunnel(repo, pool, auditLogger))
		r.Post("/{tunnelID}/start", handleStartTunnel(repo))
		r.Post("/{tunnelID}/stop", handleStopTunnel(repo))
		r.Get("/{tunnelID}/logs", handleTunnelLogs(repo, pool, chClient))
		r.Get("/{tunnelID}/tags", handleGetTunnelTags(repo, pool))
		r.Put("/{tunnelID}/tags", handleUpdateTunnelTags(repo, pool))
		r.Post("/batch/start", handleBatchStartTunnels(repo, pool))
		r.Post("/batch/stop", handleBatchStopTunnels(repo, pool))
		r.Post("/batch/delete", handleBatchDeleteTunnels(repo, pool, auditLogger))
	})

	r.Route("/v1/org", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin", "editor"))
		r.Get("/usage", handleOrgUsage(repo, pool))
		r.Post("/invitations", handleCreateInvitation(repo, pool))
		r.Get("/invitations", handleListInvitations(repo, pool))
		r.Delete("/invitations/{id}", handleDeleteInvitation(repo, pool))
		r.Get("/activity", handleOrgActivityLog(repo, pool))
	})

	r.Route("/v1/sessions", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Get("/", handleListSessions(repo, pool))
		r.Delete("/{id}", handleRevokeSession(repo, pool))
	})

	r.Route("/v1/org", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin", "editor"))
		r.Get("/usage", handleOrgUsage(repo, pool))
		r.Post("/onboarding/complete", handleOnboardingComplete(repo))
		r.Patch("/", handleUpdateOrg(repo, pool))
	})

	r.Route("/v1/api-keys", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin"))
		r.Get("/", handleListAPIKeys(repo, pool))
		r.Post("/", handleCreateAPIKey(repo, pool, auditLogger))
		r.Delete("/{keyID}", handleDeleteAPIKey(repo, pool, auditLogger))
	})

	r.Route("/v1/domains", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin"))
		r.Get("/", handleListDomains(pool))
		r.Post("/", handleAddDomain(pool))
		r.Get("/{domainID}", handleGetDomain(pool))
		r.Get("/{domainID}/verification", handleCheckDomainVerification(pool))
		r.Post("/{domainID}/verify", handleTriggerDomainVerification(pool))
		r.Delete("/{domainID}", handleRemoveDomain(pool))
	})

	r.Route("/v1/webhooks", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Get("/", handleListWebhooks(repo, pool))
		r.Post("/", handleCreateWebhook(repo, pool))
		r.Put("/{id}", handleUpdateWebhook(repo, pool))
		r.Delete("/{id}", handleDeleteWebhook(repo, pool))
		r.Post("/{id}/test", handleTestWebhook(repo, pool))
		r.Get("/{id}/deliveries", handleWebhookDeliveries(repo, pool))
	})

	r.Route("/v1/billing", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Get("/plan", handleGetPlan(repo, pool))
		r.Get("/usage", handleGetUsage(usageTracker))
		r.Get("/invoices", handleGetInvoices(repo, pool))
	})

	r.Route("/v1/networks", func(r chi.Router) {
		r.Use(auth.JWTAuthMiddleware(jwtMgr))
		r.Use(auth.RequireRole("owner", "admin", "editor"))
		r.Get("/", handleListMeshNetworks(pool))
		r.Post("/", handleCreateMeshNetwork(pool, auditLogger))
		r.Post("/join", handleJoinMeshNetwork(pool, auditLogger))
		r.Get("/{networkID}", handleGetMeshNetwork(pool))
		r.Delete("/{networkID}", handleDeleteMeshNetwork(pool, auditLogger))
		r.Post("/{networkID}/invite", handleCreateMeshInvite(pool, auditLogger))
		r.Delete("/{networkID}/nodes/{nodeID}", handleRemoveMeshNode(pool, auditLogger))
	})

	r.Route("/api/admin/v1", func(r chi.Router) {
		r.Post("/auth/login", handleAdminLogin(cfg))

		r.Group(func(r chi.Router) {
			r.Use(auth.SuperAdminMiddleware(cfg.AdminAuthSecret))

			r.Get("/dashboard/metrics", handleAdminDashboardMetrics(pool))

			r.Get("/organizations", handleAdminListOrgs(pool))
			r.Get("/organizations/{org_id}", handleAdminGetOrg(pool))
			r.Get("/organizations/{org_id}/users", handleAdminGetOrgUsers(pool))
			r.Get("/organizations/{org_id}/tunnels", handleAdminGetOrgTunnels(pool))
			r.Post("/organizations/{org_id}/freeze", handleAdminFreezeOrg(pool))
			r.Post("/organizations/{org_id}/unfreeze", handleAdminUnfreezeOrg(pool))
			r.Post("/organizations/{org_id}/change-plan", handleAdminChangePlan(pool))

			r.Get("/users", handleAdminListUsers(pool))
			r.Get("/users/{user_id}", handleAdminGetUser(pool))
			r.Post("/users/{user_id}/reset-password", handleAdminResetPassword(pool))
			r.Post("/users/{user_id}/disable", handleAdminDisableUser(pool))
			r.Post("/users/{user_id}/enable", handleAdminEnableUser(pool))

			r.Get("/relay-nodes", handleAdminListRelayNodes(pool))
			r.Get("/relay-nodes/{node_id}", handleAdminGetRelayNode(pool))
			r.Post("/relay-nodes/{node_id}/drain", handleAdminDrainRelay(pool))
			r.Post("/relay-nodes/{node_id}/undrain", handleAdminUndrainRelay(pool))
			r.Delete("/relay-nodes/{node_id}", handleAdminDecommRelay(pool))

			r.Get("/audit-logs", handleAdminAuditLogs(pool))
			r.Get("/audit-logs/export", handleAdminExportAuditLogs(pool))

			r.Get("/announcements", handleAdminListAnnouncements(announceMgr))
			r.Post("/announcements", handleAdminCreateAnnouncement(announceMgr))
			r.Put("/announcements/{id}", handleAdminUpdateAnnouncement(announceMgr))
			r.Delete("/announcements/{id}", handleAdminDeleteAnnouncement(announceMgr))

			r.Get("/certificates/system", handleAdminSystemCerts(pool))
			r.Get("/certificates/tenants", handleAdminTenantCerts(pool))
			r.Post("/certificates/{id}/renew", handleAdminRenewCert(pool))
			r.Post("/certificates/{id}/revoke", handleAdminRevokeCert(pool))

			r.Get("/abuse/reports", handleAdminListReports(abuseMgr))
			r.Get("/abuse/reports/{id}", handleAdminGetReport(abuseMgr))
			r.Post("/abuse/reports/{id}/resolve", handleAdminResolveReport(abuseMgr))
			r.Post("/abuse/reports/{id}/dismiss", handleAdminDismissReport(abuseMgr))

			r.Get("/abuse/blacklist", handleAdminListBlacklist(abuseMgr))
			r.Post("/abuse/blacklist", handleAdminAddBlacklist(abuseMgr))
			r.Delete("/abuse/blacklist/{id}", handleAdminRemoveBlacklist(abuseMgr))

		r.Get("/feature-flags", handleAdminListFlags(flagMgr))
			r.Post("/feature-flags", handleAdminCreateFlag(flagMgr))
			r.Get("/feature-flags/{key}", handleAdminGetFlag(flagMgr))
			r.Put("/feature-flags/{key}", handleAdminUpdateFlag(flagMgr))
			r.Delete("/feature-flags/{key}", handleAdminDeleteFlag(flagMgr))

			r.Get("/revenue/mrr", handleAdminMRR(pool))
			r.Get("/revenue/churn", handleAdminChurn(pool))
			r.Get("/revenue/funnel", handleAdminFunnel(pool))

			r.Get("/customers", handleAdminCustomers(pool))
			r.Get("/customers/{id}", handleAdminCustomerDetail(pool))
			r.Get("/customers/{id}/health", handleAdminCustomerHealth(pool))
		})

	})

	metricsPort := cfg.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9090
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		metricsSrv := &http.Server{
			Addr:    fmt.Sprintf(":%d", metricsPort),
			Handler: metrics.Handler(),
		}
		go func() {
			logger.InfoContext(ctx, "metrics server listening", "addr", metricsSrv.Addr)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.ErrorContext(ctx, "metrics error", "error", err)
			}
		}()
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return metricsSrv.Shutdown(shutdownCtx)
	})

	g.Go(func() error {
		apiSrv := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.ServerPort),
			Handler: r,
		}
		go func() {
			logger.InfoContext(ctx, "api server listening", "addr", apiSrv.Addr)
			if err := apiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.ErrorContext(ctx, "api error", "error", err)
			}
		}()
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		return apiSrv.Shutdown(shutdownCtx)
	})

	g.Go(func() error {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigCh:
			logger.InfoContext(ctx, "received signal, shutting down", "signal", sig.String())
			cancel()
			return nil
		case <-ctx.Done():
			return nil
		}
	})

	if err := g.Wait(); err != nil && err != http.ErrServerClosed {
		logger.ErrorContext(ctx, "server error", "error", err)
		os.Exit(1)
	}
	logger.InfoContext(ctx, "server shutdown complete")
}

func tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")

		ctx := r.Context()
		if traceID != "" {
			ctx = omnilog.WithTraceID(ctx, traceID)
		}

		ctx, span := tracing.StartSpan(ctx, "http.server."+r.URL.Path)
		defer func() {
			if span != nil {
				span.End(ctx)
			}
		}()

		if span != nil {
			w.Header().Set("X-Trace-ID", span.TraceID)
			w.Header().Set("X-Span-ID", span.SpanID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if strings.HasPrefix(origin, "http://localhost") || 
			   strings.HasPrefix(origin, "http://127.0.0.1") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func usageTrackingMiddleware(tracker *billing.UsageTracker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID, _ := auth.GetOrgID(r.Context())
			if orgID != "" {
				tracker.RecordUsage(r.Context(), orgID, r.ContentLength, 0)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}

func combinedAuth(jwtMgr *auth.JWTManager, repo auth.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token != "" && !strings.HasPrefix(token, "ot_sk_") {
				claims, err := jwtMgr.ValidateAccessToken(token)
				if err == nil {
					ctx := context.WithValue(r.Context(), auth.UserIDKey, claims.Subject)
					ctx = context.WithValue(ctx, auth.OrgIDKey, claims.OrgID)
					ctx = context.WithValue(ctx, auth.RoleKey, claims.Role)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				key, err := repo.GetAPIKeyByHash(context.Background(), auth.ComputeAPIKeyHash(apiKey))
				if err == nil && key.RevokedAt.Valid == false {
					ctx := r.Context()
					ctx = context.WithValue(ctx, auth.APIKeyIDKey, key.ID)
					if key.UserID.Valid {
						ctx = context.WithValue(ctx, auth.UserIDKey, key.UserID.String)
					}
					ctx = context.WithValue(ctx, auth.OrgIDKey, key.OrganizationID)
					ctx = context.WithValue(ctx, auth.RoleKey, "api")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			respondError(w, http.StatusUnauthorized, "unauthorized", "valid authentication required")
		})
	}
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func handleOAuthLogin(provider string, oauthMgr *auth.OAuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := uuidStr()
		var redirectURL string
		switch provider {
		case "github":
			redirectURL = oauthMgr.GitHubLoginURL(state)
		case "google":
			redirectURL = oauthMgr.GoogleLoginURL(state)
		default:
			respondError(w, http.StatusBadRequest, "invalid_provider", "unsupported OAuth provider")
			return
		}
		if redirectURL == "" {
			respondError(w, http.StatusBadRequest, "not_configured", provider+" OAuth is not configured")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

func handleOAuthCallback(oauthMgr *auth.OAuthManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "github"
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			respondError(w, http.StatusBadRequest, "invalid_request", "authorization code is required")
			return
		}
		state := r.URL.Query().Get("state")
		if state == "" {
			respondError(w, http.StatusBadRequest, "invalid_request", "state parameter is required")
			return
		}

		user, accessToken, refreshToken, err := oauthMgr.HandleOAuthCallback(r.Context(), provider, code)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "oauth_failed", err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    86400,
			"user": map[string]string{
				"id":    user.ID,
				"email": user.Email,
				"role":  user.Role,
			},
		})
	}
}

func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	return host
}

func handleEnrollMFA(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.EnrollMFA(r.Context(), &omnitunv1.EnrollMFARequest{})
		if err != nil {
			respondError(w, 500, "mfa_enroll_failed", err.Error())
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"secret":      resp.GetSecret(),
			"qr_code_url": resp.GetQrCodeUrl(),
		})
	}
}

func handleVerifyMFA(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Code string `json:"code"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Code == "" {
			respondError(w, 400, "invalid_request", "code is required")
			return
		}
		resp, err := svc.VerifyMFA(r.Context(), &omnitunv1.VerifyMFARequest{Code: req.Code})
		if err != nil {
			respondError(w, 400, "mfa_verify_failed", err.Error())
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"success": resp.GetSuccess(),
		})
	}
}

func handleDisableMFA(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.DisableMFA(r.Context()); err != nil {
			respondError(w, 500, "mfa_disable_failed", err.Error())
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"success": true,
		})
	}
}

func handleRegister(svc *auth.Service, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req omnitunv1.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Email == "" || req.Password == "" {
			respondError(w, 400, "invalid_request", "email and password are required")
			return
		}
		resp, err := svc.Register(r.Context(), &req)
		if err != nil {
			respondError(w, 409, "registration_failed", err.Error())
			return
		}
		clientIP := clientIPFromRequest(r)
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: "", UserID: resp.GetUserId(),
			Action: "user.register", ResourceType: "user", ResourceID: resp.GetUserId(),
			ClientIP: clientIP,
		})
		respondJSON(w, 201, map[string]string{
			"user_id": resp.GetUserId(),
			"message": resp.GetMessage(),
		})
	}
}

func handleLogin(svc *auth.Service, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req omnitunv1.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Email == "" || req.Password == "" {
			respondError(w, 400, "invalid_request", "email and password are required")
			return
		}
		resp, err := svc.Login(r.Context(), &req)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == "MFA_REQUIRED" {
				respondError(w, 401, "mfa_required", "MFA code required")
				return
			}
			respondError(w, 401, "authentication_failed", err.Error())
			return
		}
		user := resp.GetUser()
		clientIP := clientIPFromRequest(r)
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: user.GetOrganizationId(), UserID: user.GetId(),
			Action: "user.login", ResourceType: "user", ResourceID: user.GetId(),
			ClientIP: clientIP,
		})
		respondJSON(w, 200, map[string]interface{}{
			"access_token":  resp.GetAccessToken(),
			"refresh_token": resp.GetRefreshToken(),
			"expires_in":    resp.GetExpiresIn(),
			"user": map[string]interface{}{
				"id":          user.GetId(),
				"email":       user.GetEmail(),
				"role":        user.GetRole(),
				"mfa_enabled": user.GetMfaEnabled(),
			},
		})
	}
}

func handleMe(repo auth.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, 401, "unauthorized", "not authenticated")
			return
		}
		user, err := repo.GetUserByID(r.Context(), userID)
		if err != nil {
			respondError(w, 404, "user_not_found", err.Error())
			return
		}
		org, err := repo.GetOrganization(r.Context(), user.OrganizationID)
		onboardingCompleted := false
		if err == nil {
			onboardingCompleted = org.OnboardingCompleted
		}
		respondJSON(w, 200, map[string]interface{}{
			"id":                   user.ID,
			"email":                user.Email,
			"display_name":         user.DisplayName,
			"role":                 user.Role,
			"mfa_enabled":          user.MFAEnabled,
			"org_id":               user.OrganizationID,
			"onboarding_completed": onboardingCompleted,
		})
	}
}

func handleOIDCLogin(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := auth.NewOIDCProvider(cfg.Auth)
		if !provider.IsConfigured() {
			respondError(w, 400, "oidc_not_configured", "OIDC is not configured")
			return
		}

		state, err := auth.GenerateOIDCState()
		if err != nil {
			respondError(w, 500, "internal_error", "failed to generate state")
			return
		}

		signature := auth.SignOIDCState(state, cfg.Auth.JWTSecret)
		http.SetCookie(w, &http.Cookie{
			Name:     "oidc_state",
			Value:    state + "." + signature,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			MaxAge:   600,
			SameSite: http.SameSiteLaxMode,
		})

		authURL, err := provider.AuthCodeURL(state)
		if err != nil {
			respondError(w, 500, "internal_error", "failed to build auth URL: "+err.Error())
			return
		}

		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func handleOIDCCallback(oidcMgr *auth.OIDCManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			errorMsg := r.URL.Query().Get("error_description")
			if errorMsg == "" {
				errorMsg = r.URL.Query().Get("error")
			}
			if errorMsg == "" {
				errorMsg = "no authorization code received"
			}
			respondError(w, 400, "oidc_error", errorMsg)
			return
		}

		cookie, err := r.Cookie("oidc_state")
		if err == nil {
			parts := make([]string, 0)
			dotIdx := -1
			for i := len(cookie.Value) - 1; i >= 0; i-- {
				if cookie.Value[i] == '.' {
					dotIdx = i
					break
				}
			}
			if dotIdx >= 0 {
				parts = append(parts, cookie.Value[:dotIdx], cookie.Value[dotIdx+1:])
			}
			if len(parts) == 2 {
				expectedState := parts[0]
				signature := parts[1]
				if expectedState != state || !auth.VerifyOIDCState(expectedState, signature, cfg.Auth.JWTSecret) {
					respondError(w, 400, "invalid_state", "OIDC state mismatch")
					return
				}
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "oidc_state",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		user, accessToken, refreshToken, err := oidcMgr.HandleCallback(r.Context(), code)
		if err != nil {
			respondError(w, 401, "oidc_auth_failed", err.Error())
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    accessToken,
			Path:     "/",
			HttpOnly: false,
			MaxAge:   int(cfg.Auth.TokenExpiry),
			SameSite: http.SameSiteLaxMode,
		})

		dashboardURL := "/?token=" + accessToken + "&refresh_token=" + refreshToken
		_ = user
		http.Redirect(w, r, dashboardURL, http.StatusFound)
	}
}

// ---- Dashboard handlers ----

func handleDashboardStats(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var activeTunnels, totalConnections int
		var totalBytesIn, totalBytesOut, todayReqs int64
		pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM tunnels WHERE organization_id=$1 AND status='active' AND deleted_at IS NULL`, orgID,
		).Scan(&activeTunnels)
		pool.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(bytes_in_total),0), COALESCE(SUM(bytes_out_total),0) FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL`, orgID,
		).Scan(&totalBytesIn, &totalBytesOut)
		pool.QueryRow(r.Context(),
			`SELECT COALESCE(COUNT(DISTINCT connection_id),0) FROM traffic_events WHERE organization_id=$1 AND timestamp > now() - INTERVAL '24 hours'`, orgID,
		).Scan(&todayReqs)
		_ = totalConnections
		respondJSON(w, 200, map[string]interface{}{
			"active_tunnels":   activeTunnels,
			"total_bytes_in":   totalBytesIn,
			"total_bytes_out":  totalBytesOut,
			"active_connections": 0,
			"today_requests":   todayReqs,
		})
	}
}

func handleDashboardEvents(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT id, action, resource_type, resource_id, details, created_at FROM audit_logs WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 20`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var events []map[string]interface{}
		for rows.Next() {
			var id, action, resType, resID string
			var details []byte
			var createdAt time.Time
			rows.Scan(&id, &action, &resType, &resID, &details, &createdAt)
			events = append(events, map[string]interface{}{
				"id": id, "action": action, "resource_type": resType,
				"resource_id": resID, "created_at": createdAt.Format(time.RFC3339),
				"tunnel_name": "", "status": "success",
			})
		}
		respondJSON(w, 200, events)
	}
}

// ---- Tunnel handlers ----

func handleListTunnels(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(slug,''), COALESCE(protocol,'http'),
			        COALESCE(local_port,0), COALESCE(custom_domain,''), COALESCE(status,'stopped'),
			        COALESCE(bytes_in_total,0), COALESCE(bytes_out_total,0), COALESCE(created_at,NOW())
			 FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 50`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, map[string]interface{}{"tunnels": []interface{}{}})
			return
		}
		defer rows.Close()
		var tunnels []map[string]interface{}
		for rows.Next() {
			var id, name, slug, protocol, domain, status string
			var port int
			var bytesIn, bytesOut int64
			var createdAt time.Time
			rows.Scan(&id, &name, &slug, &protocol, &port, &domain, &status, &bytesIn, &bytesOut, &createdAt)
			d := domain
			if d == "" {
				d = slug + ".omnitun.io"
			}
			tunnels = append(tunnels, map[string]interface{}{
				"id": id, "name": name, "slug": slug, "protocol": protocol,
				"local_port": port, "domain": d, "status": status,
				"bytes_in_total": bytesIn, "bytes_out_total": bytesOut,
				"created_at": createdAt.Format(time.RFC3339),
			})
		}
		if tunnels == nil {
			tunnels = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"tunnels": tunnels})
	}
}

func handleCreateTunnel(repo auth.Repository, pool *pgxpool.Pool, auditLogger *audit.Logger, usageTracker *billing.UsageTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())

		usage, err := usageTracker.GetCurrentUsage(r.Context(), orgID)
		if err == nil {
			plan, ok := billing.Plans[usage.Plan]
			if !ok {
				plan = billing.Plans["free"]
			}
			if usage.TunnelsUsed >= plan.MaxTunnels {
				respondJSON(w, 402, map[string]interface{}{
					"error": map[string]string{
						"code":    "quota_exceeded",
						"message": fmt.Sprintf("Tunnel limit reached (%d). Upgrade your plan to create more.", plan.MaxTunnels),
					},
				})
				return
			}
		}

		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			pool.QueryRow(r.Context(),
				`SELECT COALESCE(id::text,'') FROM workspaces WHERE organization_id=$1 AND deleted_at IS NULL LIMIT 1`,
				orgID,
			).Scan(&workspaceID)
		}
		if workspaceID == "" {
			respondError(w, 400, "no_workspace", "no workspace found; create a workspace first")
			return
		}
		var req struct {
			Name      string `json:"name"`
			Protocol  string `json:"protocol"`
			LocalPort int    `json:"local_port"`
			Domain    string `json:"domain"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Protocol == "" {
			req.Protocol = "http"
		}
		if req.LocalPort == 0 {
			req.LocalPort = 80
		}
		slug := fmt.Sprintf("tun-%s-%s", orgID[:4], uuidStr()[:6])
		var id string
		err = pool.QueryRow(r.Context(),
			`INSERT INTO tunnels (organization_id, workspace_id, name, slug, protocol, local_port, custom_domain, status) VALUES ($1,$2,$3,$4,$5,$6,$7,'stopped') RETURNING id::text`,
			orgID, workspaceID, req.Name, slug, req.Protocol, req.LocalPort, req.Domain,
		).Scan(&id)
		if err != nil {
			id = slug
		}
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "tunnel.create", ResourceType: "tunnel", ResourceID: id,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 201, map[string]interface{}{
			"id": id, "name": req.Name, "slug": slug, "protocol": req.Protocol,
			"local_port": req.LocalPort, "domain": req.Domain, "status": "stopped",
		})
	}
}

func handleGetTunnel(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")
		var id, name, slug, protocol, domain, status string
		var port int
		var bytesIn, bytesOut int64
		var createdAt time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(id::text,''),COALESCE(name,''),COALESCE(slug,''),COALESCE(protocol,'http'),COALESCE(local_port,0),COALESCE(custom_domain,''),COALESCE(status,'stopped'),COALESCE(bytes_in_total,0),COALESCE(bytes_out_total,0),COALESCE(created_at,NOW()) FROM tunnels WHERE id=$1::uuid AND deleted_at IS NULL`, tunnelID,
		).Scan(&id, &name, &slug, &protocol, &port, &domain, &status, &bytesIn, &bytesOut, &createdAt)
		if err != nil {
			respondError(w, 404, "tunnel_not_found", "tunnel not found")
			return
		}
		d := domain
		if d == "" {
			d = slug + ".omnitun.io"
		}
		respondJSON(w, 200, map[string]interface{}{
			"id": id, "name": name, "slug": slug, "protocol": protocol,
			"local_port": port, "domain": d, "status": status,
			"bytes_in_total": bytesIn, "bytes_out_total": bytesOut,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
}

func handleUpdateTunnel(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")
		var req struct {
			Name           string `json:"name"`
			AuthMode       string `json:"auth_mode"`
			MaxConnections int    `json:"max_connections"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "" {
			pool.Exec(r.Context(), `UPDATE tunnels SET name=$1 WHERE id=$2`, req.Name, tunnelID)
		}
		respondJSON(w, 200, map[string]string{"message": "updated"})
	}
}

func handleDeleteTunnel(repo auth.Repository, pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		pool.Exec(r.Context(), `UPDATE tunnels SET deleted_at=now() WHERE id=$1`, tunnelID)
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "tunnel.delete", ResourceType: "tunnel", ResourceID: tunnelID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

func handleStartTunnel(repo auth.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]string{"status": "active", "relay_address": "localhost:4443"})
	}
}

func handleStopTunnel(repo auth.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]string{"status": "stopped"})
	}
}

func handleWebSocket(upgrader websocket.Upgrader, jwtMgr *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong","data":{}}`))
			_ = msg
		}
	}
}

func uuidStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func handleOrgUsage(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var plan string
		var tunnelCount int
		var bwIn, bwOut int64
		pool.QueryRow(r.Context(), `SELECT COALESCE(plan,'free') FROM organizations WHERE id=$1`, orgID).Scan(&plan)
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL`, orgID).Scan(&tunnelCount)
		pool.QueryRow(r.Context(), `SELECT COALESCE(SUM(bytes_in_total),0), COALESCE(SUM(bytes_out_total),0) FROM tunnels WHERE organization_id=$1`, orgID).Scan(&bwIn, &bwOut)
		limits := map[string]int{"free": 1, "pro": 10, "team": 50, "business": 999999}[plan]
		respondJSON(w, 200, map[string]interface{}{
			"plan":          plan,
			"tunnels_used":  tunnelCount,
			"tunnels_limit": limits,
			"bandwidth_used_mb": (bwIn + bwOut) / 1048576,
			"bandwidth_limit_mb": 10240,
		})
	}
}

func handleOnboardingComplete(repo auth.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		if err := repo.SetOnboardingCompleted(r.Context(), orgID); err != nil {
			respondError(w, 500, "internal_error", "failed to update onboarding status")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"onboarding_completed": true,
		})
	}
}

func handleUpdateOrg(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Name == "" && req.Slug == "" {
			respondError(w, 400, "invalid_request", "name or slug is required")
			return
		}
		setClauses := []string{}
		args := []interface{}{}
		idx := 1
		if req.Name != "" {
			setClauses = append(setClauses, fmt.Sprintf("name=$%d", idx))
			args = append(args, req.Name)
			idx++
		}
		if req.Slug != "" {
			setClauses = append(setClauses, fmt.Sprintf("slug=$%d", idx))
			args = append(args, req.Slug)
			idx++
		}
		setClauses = append(setClauses, fmt.Sprintf("updated_at=$%d", idx))
		args = append(args, time.Now())
		idx++
		args = append(args, orgID)
		query := fmt.Sprintf("UPDATE organizations SET %s WHERE id=$%d", strings.Join(setClauses, ", "), idx)
		_, err := pool.Exec(r.Context(), query, args...)
		if err != nil {
			respondError(w, 500, "internal_error", "failed to update organization")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"name": req.Name,
			"slug": req.Slug,
		})
	}
}

func handleListAPIKeys(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(key_prefix,''), COALESCE(created_at,NOW()), COALESCE(last_used_at,NOW()), CASE WHEN revoked_at IS NULL THEN 'active' ELSE 'revoked' END FROM api_keys WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 20`, orgID)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var keys []map[string]interface{}
		for rows.Next() {
			var id, name, prefix, status string
			var createdAt, lastUsed time.Time
			rows.Scan(&id, &name, &prefix, &createdAt, &lastUsed, &status)
			keys = append(keys, map[string]interface{}{
				"id": id, "name": name, "key_prefix": prefix,
				"status": status, "created_at": createdAt.Format(time.RFC3339),
				"last_used": lastUsed.Format(time.RFC3339),
			})
		}
		respondJSON(w, 200, keys)
	}
}

func handleCreateAPIKey(repo auth.Repository, pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct{ Name string `json:"name"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" { req.Name = "default" }
		rawKey := "ot_sk_" + uuidStr()[:32]
		prefix := rawKey[:12]
		hash := auth.ComputeAPIKeyHash(rawKey)
		var id string
		pool.QueryRow(r.Context(),
			`INSERT INTO api_keys (organization_id, user_id, name, key_prefix, key_hash, scopes) VALUES ($1,$2,$3,$4,$5,'["*"]'::jsonb) RETURNING id::text`,
			orgID, userID, req.Name, prefix, hash).Scan(&id)
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "apikey.create", ResourceType: "apikey", ResourceID: id,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 201, map[string]string{
			"id": id, "name": req.Name, "key": rawKey, "key_prefix": prefix,
		})
	}
}

func handleDeleteAPIKey(repo auth.Repository, pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID := chi.URLParam(r, "keyID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		pool.Exec(r.Context(), `UPDATE api_keys SET revoked_at=NOW() WHERE id=$1::uuid`, keyID)
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "apikey.revoke", ResourceType: "apikey", ResourceID: keyID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]string{"message": "revoked"})
	}
}

func handleTunnelLogs(repo auth.Repository, pool *pgxpool.Pool, chClient *clickhouse.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")

		if chClient != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			rows, err := chClient.Query(ctx,
				fmt.Sprintf(`SELECT timestamp, tunnel_id, connection_id, protocol, direction, bytes, method, path, status_code, client_ip, client_country, duration_ms, error FROM traffic_events WHERE tunnel_id='%s' ORDER BY timestamp DESC LIMIT 20`, tunnelID),
			)
			if err == nil {
				logs := make([]map[string]interface{}, 0, len(rows))
				for _, row := range rows {
					logs = append(logs, map[string]interface{}{
						"timestamp":      row["timestamp"],
						"tunnel_id":      row["tunnel_id"],
						"connection_id":  row["connection_id"],
						"protocol":       row["protocol"],
						"direction":      row["direction"],
						"bytes":          row["bytes"],
						"method":         row["method"],
						"path":           row["path"],
						"status_code":    row["status_code"],
						"client_ip":      row["client_ip"],
						"client_country": row["client_country"],
						"duration_ms":    row["duration_ms"],
						"error":          row["error"],
					})
				}
				respondJSON(w, 200, logs)
				return
			}
		}

		respondJSON(w, 200, []map[string]interface{}{})
	}
}

// ---- Domain handlers ----

func handleListDomains(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT id::text, COALESCE(name,''), COALESCE(custom_domain,''), COALESCE(status,'stopped'), COALESCE(created_at,NOW())
			 FROM tunnels WHERE organization_id=$1 AND custom_domain IS NOT NULL AND custom_domain != '' AND deleted_at IS NULL
			 ORDER BY created_at DESC`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var domains []map[string]interface{}
		for rows.Next() {
			var id, name, domain, status string
			var createdAt time.Time
			rows.Scan(&id, &name, &domain, &status, &createdAt)
			domains = append(domains, map[string]interface{}{
				"id":         id,
				"tunnel_id":  id,
				"tunnel_name": name,
				"domain":     domain,
				"status":     status,
				"verification_status": "pending",
				"created_at": createdAt.Format(time.RFC3339),
			})
		}
		if domains == nil {
			domains = []map[string]interface{}{}
		}
		respondJSON(w, 200, domains)
	}
}

func handleAddDomain(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Domain   string `json:"domain"`
			TunnelID string `json:"tunnel_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Domain == "" {
			respondError(w, 400, "invalid_request", "domain is required")
			return
		}
		if req.TunnelID == "" {
			respondError(w, 400, "invalid_request", "tunnel_id is required")
			return
		}
		token := uuidStr()[:16]
		tag, err := pool.Exec(r.Context(),
			`UPDATE tunnels SET custom_domain=$1 WHERE id=$2::uuid AND organization_id=$3`,
			req.Domain, req.TunnelID, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "tunnel_not_found", "tunnel not found or update failed")
			return
		}
		_ = token
		respondJSON(w, 201, map[string]interface{}{
			"id":         req.TunnelID,
			"domain":     req.Domain,
			"tunnel_id":  req.TunnelID,
			"verification": map[string]interface{}{
				"type":    "CNAME",
				"record":  fmt.Sprintf("%s.omnitun-edge.com", req.TunnelID[:8]),
				"token":   token,
				"status":  "pending",
			},
			"dns_instructions": fmt.Sprintf(
				"Add a CNAME record: %s → %s.omnitun-edge.com",
				req.Domain, req.TunnelID[:8],
			),
		})
	}
}

func handleGetDomain(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domainID := chi.URLParam(r, "domainID")
		orgID, _ := auth.GetOrgID(r.Context())
		var id, name, domain, status string
		var createdAt time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT id::text, COALESCE(name,''), COALESCE(custom_domain,''), COALESCE(status,'stopped'), COALESCE(created_at,NOW())
			 FROM tunnels WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, domainID, orgID,
		).Scan(&id, &name, &domain, &status, &createdAt)
		if err != nil {
			respondError(w, 404, "domain_not_found", "domain not found")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"id":                  id,
			"tunnel_id":           id,
			"tunnel_name":         name,
			"domain":              domain,
			"status":              status,
			"verification_status": "pending",
			"created_at":          createdAt.Format(time.RFC3339),
		})
	}
}

func handleCheckDomainVerification(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = chi.URLParam(r, "domainID")
		respondJSON(w, 200, map[string]interface{}{
			"verified":   false,
			"status":     "pending",
			"checked_at": time.Now().Format(time.RFC3339),
		})
	}
}

func handleTriggerDomainVerification(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = chi.URLParam(r, "domainID")
		respondJSON(w, 200, map[string]interface{}{
			"verified":   false,
			"status":     "pending",
			"message":    "DNS verification check initiated (mock)",
			"checked_at": time.Now().Format(time.RFC3339),
		})
	}
}

func handleRemoveDomain(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domainID := chi.URLParam(r, "domainID")
		orgID, _ := auth.GetOrgID(r.Context())
		tag, err := pool.Exec(r.Context(),
			`UPDATE tunnels SET custom_domain=NULL WHERE id=$1::uuid AND organization_id=$2`,
			domainID, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "domain_not_found", "domain not found")
			return
		}
		respondJSON(w, 200, map[string]string{"message": "domain removed"})
	}
}

func handleSAMLLogin(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.Auth.SAMLEnabled {
			respondError(w, 404, "not_found", "SAML is not enabled")
			return
		}
		sp := buildSAMLProvider(cfg)
		sssoURL, _, err := sp.BuildAuthnRequest("")
		if err != nil {
			respondError(w, 500, "saml_error", "failed to build SAML request")
			return
		}
		http.Redirect(w, r, sssoURL, http.StatusFound)
	}
}

func handleSAMLACS(cfg *config.Config, authSvc *auth.Service, repo auth.Repository, jwtMgr *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.Auth.SAMLEnabled {
			respondError(w, 404, "not_found", "SAML is not enabled")
			return
		}
		if err := r.ParseForm(); err != nil {
			respondError(w, 400, "invalid_request", "failed to parse form data")
			return
		}
		samlResponse := r.FormValue("SAMLResponse")
		if samlResponse == "" {
			respondError(w, 400, "invalid_request", "missing SAMLResponse")
			return
		}

		sp := buildSAMLProvider(cfg)

		samlUser, err := sp.ParseSAMLResponse(samlResponse)
		if err != nil {
			respondError(w, 401, "saml_error", "SAML authentication failed: "+err.Error())
			return
		}

		ctx := r.Context()
		user, err := repo.FindUserByEmail(ctx, samlUser.Email)
		if err != nil {
			user, err = repo.GetUserByProvider(ctx, "saml", samlUser.NameID)
			if err != nil {
				orgSlug := emailToSlug(samlUser.Email)
				org := &auth.Organization{
					Name: samlUser.Email,
					Slug: orgSlug,
					Plan: "free",
				}
				if err := repo.CreateOrganization(ctx, org); err != nil {
					respondError(w, 500, "internal_error", "failed to create organization")
					return
				}
				user = &auth.User{
					OrganizationID: org.ID,
					Email:          samlUser.Email,
					DisplayName:    samlUser.Name,
					Role:           "owner",
					AuthProvider:   "saml",
				}
				user.AuthProviderID = sql.NullString{String: samlUser.NameID, Valid: true}
				if err := repo.CreateUser(ctx, user); err != nil {
					respondError(w, 500, "internal_error", "failed to create user")
					return
				}
			}
		}

		accessToken, err := jwtMgr.IssueAccessToken(user.ID, user.OrganizationID, user.Role)
		if err != nil {
			respondError(w, 500, "internal_error", "failed to issue token")
			return
		}
		refreshToken, err := jwtMgr.IssueRefreshToken(user.ID)
		if err != nil {
			respondError(w, 500, "internal_error", "failed to issue token")
			return
		}
		refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
		repo.StoreRefreshToken(ctx, user.ID, refreshToken, refreshExpiry)
		repo.UpdateLastLogin(ctx, user.ID)

		http.SetCookie(w, &http.Cookie{
			Name:     "omnitun_token",
			Value:    accessToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})

		dashboardURL := "/"
		if relayState := r.FormValue("RelayState"); relayState != "" {
			decodedState, err := url.QueryUnescape(relayState)
			if err == nil {
				dashboardURL = decodedState
			}
		}
		http.Redirect(w, r, dashboardURL, http.StatusFound)
	}
}

func handleSAMLLMetadata(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.Auth.SAMLEnabled {
			respondError(w, 404, "not_found", "SAML is not enabled")
			return
		}
		sp := buildSAMLProvider(cfg)
		metadata, err := sp.GenerateSPMetadata()
		if err != nil {
			respondError(w, 500, "saml_error", "failed to generate metadata")
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(metadata)
	}
}

func buildSAMLProvider(cfg *config.Config) *auth.SAMLProvider {
	return &auth.SAMLProvider{
		EntityID:    cfg.Auth.SAMLEntityID,
		ACSURL:      cfg.Auth.SAMLACSURL,
		MetadataURL: cfg.Auth.SAMLMetadataURL,
		CertFile:    cfg.Auth.SAMLCertFile,
		KeyFile:     cfg.Auth.SAMLKeyFile,
	}
}

type meshNetworkRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cidr      string `json:"cidr"`
	OrgID     string `json:"organization_id"`
	CreatedAt int64  `json:"created_at"`
}

type meshNodeRecord struct {
	NodeID    string `json:"node_id"`
	NetworkID string `json:"network_id"`
	Name      string `json:"name"`
	MeshIP    string `json:"mesh_ip"`
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
	Status    string `json:"status"`
}

type meshNetworkStore struct {
	mu       sync.Mutex
	networks map[string]*meshNetworkRecord
	nodes    map[string][]*meshNodeRecord
	invites  map[string]string
}

var meshStore = &meshNetworkStore{
	networks: make(map[string]*meshNetworkRecord),
	nodes:    make(map[string][]*meshNodeRecord),
	invites:  make(map[string]string),
}

type stripeRepoImpl struct {
	pool *pgxpool.Pool
}

func (r *stripeRepoImpl) UpdateSubscription(ctx context.Context, orgID string, planID string, status string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subscriptions (organization_id, plan_id, status, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (organization_id) DO UPDATE
		 SET plan_id = EXCLUDED.plan_id,
		     status = EXCLUDED.status,
		     updated_at = now()`,
		orgID, planID, status,
	)
	if err == nil {
		_, err = r.pool.Exec(ctx,
			`UPDATE organizations SET plan = $1, updated_at = now() WHERE id = $2`,
			planID, orgID,
		)
	}
	return err
}

func (r *stripeRepoImpl) CreateCustomer(ctx context.Context, orgID string, customerID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO stripe_customers (organization_id, customer_id, created_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (organization_id) DO UPDATE
		 SET customer_id = EXCLUDED.customer_id`,
		orgID, customerID,
	)
	return err
}

func handleListMeshNetworks(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		meshStore.mu.Lock()
		defer meshStore.mu.Unlock()
		var networks []map[string]interface{}
		for _, n := range meshStore.networks {
			if n.OrgID == orgID {
				nodeCount := len(meshStore.nodes[n.ID])
				networks = append(networks, map[string]interface{}{
					"id":         n.ID,
					"name":       n.Name,
					"cidr":       n.Cidr,
					"node_count": nodeCount,
					"created_at": n.CreatedAt,
				})
			}
		}
		if networks == nil {
			networks = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"networks": networks})
	}
}

func handleCreateMeshNetwork(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Name string `json:"name"`
			Cidr string `json:"cidr"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" {
			req.Name = "default-mesh"
		}
		if req.Cidr == "" {
			req.Cidr = "10.42.0.0/16"
		}
		id := fmt.Sprintf("mesh-%s", uuidStr()[:8])
		record := &meshNetworkRecord{
			ID:        id,
			Name:      req.Name,
			Cidr:      req.Cidr,
			OrgID:     orgID,
			CreatedAt: time.Now().Unix(),
		}
		meshStore.mu.Lock()
		meshStore.networks[id] = record
		meshStore.mu.Unlock()
		respondJSON(w, 201, record)
	}
}

func handleGetMeshNetwork(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkID := chi.URLParam(r, "networkID")
		meshStore.mu.Lock()
		network, ok := meshStore.networks[networkID]
		nodes := meshStore.nodes[networkID]
		meshStore.mu.Unlock()
		if !ok {
			respondError(w, 404, "not_found", "mesh network not found")
			return
		}
		nodeList := make([]map[string]interface{}, 0, len(nodes))
		for _, n := range nodes {
			nodeList = append(nodeList, map[string]interface{}{
				"id":         n.NodeID,
				"network_id": n.NetworkID,
				"name":       n.Name,
				"ip_address": n.MeshIP,
				"public_key": n.PublicKey,
				"nat_type":   "unknown",
				"endpoints":  []string{n.Endpoint},
				"status":     n.Status,
				"last_seen_at": nil,
				"created_at":  nil,
			})
		}
		if nodeList == nil {
			nodeList = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{
			"id":         network.ID,
			"name":       network.Name,
			"cidr":       network.Cidr,
			"nodes":      nodeList,
			"node_count": len(nodeList),
			"created_at": network.CreatedAt,
		})
	}
}

func handleDeleteMeshNetwork(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkID := chi.URLParam(r, "networkID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		meshStore.mu.Lock()
		_, ok := meshStore.networks[networkID]
		delete(meshStore.networks, networkID)
		delete(meshStore.nodes, networkID)
		delete(meshStore.invites, networkID)
		meshStore.mu.Unlock()
		if !ok {
			respondError(w, 404, "not_found", "mesh network not found")
			return
		}
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "mesh.delete", ResourceType: "mesh_network", ResourceID: networkID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

func handleCreateMeshInvite(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkID := chi.URLParam(r, "networkID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		meshStore.mu.Lock()
		_, ok := meshStore.networks[networkID]
		if !ok {
			meshStore.mu.Unlock()
			respondError(w, 404, "not_found", "mesh network not found")
			return
		}
		inviteCode := fmt.Sprintf("minv-%s", uuidStr()[:12])
		meshStore.invites[networkID] = inviteCode
		meshStore.mu.Unlock()
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "mesh.invite_created", ResourceType: "mesh_network", ResourceID: networkID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 201, map[string]interface{}{
			"network_id":  networkID,
			"invite_code": inviteCode,
			"expires_in":  3600,
		})
	}
}

func handleJoinMeshNetwork(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			InviteCode string `json:"invite_code"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.InviteCode == "" {
			respondError(w, 400, "invalid_request", "invite_code is required")
			return
		}

		meshStore.mu.Lock()
		var network *meshNetworkRecord
		var networkID string
		for nid, code := range meshStore.invites {
			if code == req.InviteCode {
				networkID = nid
				network = meshStore.networks[nid]
				break
			}
		}
		if network == nil {
			meshStore.mu.Unlock()
			respondError(w, 404, "not_found", "invalid invite code")
			return
		}
		existingNodes := meshStore.nodes[networkID]
		peerIP := fmt.Sprintf("10.42.0.%d", len(existingNodes)+2)
		nodeID := fmt.Sprintf("node-%s", uuidStr()[:8])
		keyBytes := make([]byte, 32)
		rand.Read(keyBytes)
		publicKey := fmt.Sprintf("%x", keyBytes)
		node := &meshNodeRecord{
			NodeID:    nodeID,
			NetworkID: networkID,
			Name:      nodeID,
			MeshIP:    peerIP,
			PublicKey: publicKey,
			Endpoint:  "",
			Status:    "online",
		}
		meshStore.nodes[networkID] = append(meshStore.nodes[networkID], node)
		meshStore.mu.Unlock()

		peers := make([]map[string]interface{}, 0, len(existingNodes))
		for _, n := range existingNodes {
			peers = append(peers, map[string]interface{}{
				"node_id":    n.NodeID,
				"public_key": n.PublicKey,
				"mesh_ip":    n.MeshIP,
				"endpoint":   n.Endpoint,
			})
		}
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "mesh.node_joined", ResourceType: "mesh_network", ResourceID: networkID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]interface{}{
			"id":         networkID,
			"name":       network.Name,
			"cidr":       network.Cidr,
			"node_count": len(meshStore.nodes[networkID]),
			"created_at": network.CreatedAt,
		})
	}
}

func handleRemoveMeshNode(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkID := chi.URLParam(r, "networkID")
		nodeID := chi.URLParam(r, "nodeID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		meshStore.mu.Lock()
		nodes := meshStore.nodes[networkID]
		found := false
		for i, n := range nodes {
			if n.NodeID == nodeID {
				meshStore.nodes[networkID] = append(nodes[:i], nodes[i+1:]...)
				found = true
				break
			}
		}
		meshStore.mu.Unlock()
		if !found {
			respondError(w, 404, "not_found", "node not found")
			return
		}
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "mesh.node_removed", ResourceType: "mesh_node", ResourceID: nodeID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]string{"message": "node removed"})
	}
}

func handleJoinMeshNetworkByNetworkID(pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkID := chi.URLParam(r, "networkID")
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			InviteCode string `json:"invite_code"`
			AgentID    string `json:"agent_id"`
			PublicKey  string `json:"public_key"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		meshStore.mu.Lock()
		network, ok := meshStore.networks[networkID]
		if !ok {
			meshStore.mu.Unlock()
			respondError(w, 404, "not_found", "mesh network not found")
			return
		}
		existingNodes := meshStore.nodes[networkID]
		peerIP := fmt.Sprintf("10.42.0.%d", len(existingNodes)+2)
		nodeID := req.AgentID
		if nodeID == "" {
			nodeID = fmt.Sprintf("node-%s", uuidStr()[:8])
		}
		publicKey := req.PublicKey
		if publicKey == "" {
			keyBytes := make([]byte, 32)
			rand.Read(keyBytes)
			publicKey = fmt.Sprintf("%x", keyBytes)
		}
		node := &meshNodeRecord{
			NodeID:    nodeID,
			NetworkID: networkID,
			Name:      nodeID,
			MeshIP:    peerIP,
			PublicKey: publicKey,
			Endpoint:  "",
			Status:    "online",
		}
		meshStore.nodes[networkID] = append(meshStore.nodes[networkID], node)
		meshStore.mu.Unlock()

		peers := make([]map[string]interface{}, 0, len(existingNodes))
		for _, n := range existingNodes {
			peers = append(peers, map[string]interface{}{
				"node_id":    n.NodeID,
				"public_key": n.PublicKey,
				"mesh_ip":    n.MeshIP,
				"endpoint":   n.Endpoint,
			})
		}
		auditLogger.Log(r.Context(), audit.AuditEvent{
			OrgID: orgID, UserID: userID,
			Action: "mesh.node_joined", ResourceType: "mesh_network", ResourceID: networkID,
			ClientIP: clientIPFromRequest(r),
		})
		respondJSON(w, 200, map[string]interface{}{
			"network_id": network.ID,
			"cidr":       network.Cidr,
			"peer_ip":    peerIP,
			"peers":      peers,
			"node_id":    nodeID,
		})
	}
}

func emailToSlug(email string) string {
	slug := ""
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r == '@' || r == '.' || r == '_' {
			slug += "-"
		}
	}
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

func handleForgotPassword(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, 200, map[string]string{"message": "If the email exists, a reset link has been sent"})
			return
		}
		if req.Email == "" {
			respondJSON(w, 200, map[string]string{"message": "If the email exists, a reset link has been sent"})
			return
		}

		user, err := repo.FindUserByEmail(r.Context(), req.Email)
		if err != nil {
			respondJSON(w, 200, map[string]string{"message": "If the email exists, a reset link has been sent"})
			return
		}

		resetToken, err := auth.GenerateToken()
		if err != nil {
			slog.Error("failed to generate reset token", "error", err)
			respondError(w, 500, "internal_error", "failed to generate reset token")
			return
		}

		tokenHash := auth.HashToken(resetToken)
		expiresAt := time.Now().Add(1 * time.Hour)
		if err := repo.StorePasswordResetToken(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
			slog.Error("failed to store reset token", "error", err)
			respondError(w, 500, "internal_error", "failed to store reset token")
			return
		}

		resetURL := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", resetToken)
		slog.Info("password reset requested (email would be sent in production)", "email", req.Email, "reset_url", resetURL)

		respondJSON(w, 200, map[string]string{"message": "If the email exists, a reset link has been sent"})
	}
}

func handleResetPassword(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Token == "" || req.NewPassword == "" {
			respondError(w, 400, "invalid_request", "token and new_password required")
			return
		}

		tokenHash := auth.HashToken(req.Token)

		record, err := repo.GetPasswordResetToken(r.Context(), tokenHash)
		if err != nil {
			respondError(w, 400, "invalid_token", "invalid or expired reset token")
			return
		}

		if err := auth.ValidatePassword(req.NewPassword); err != nil {
			respondError(w, 400, "weak_password", err.Error())
			return
		}

		passwordHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			slog.Error("failed to hash new password", "error", err)
			respondError(w, 500, "internal_error", "failed to process password")
			return
		}

		if err := repo.UpdateUserPassword(r.Context(), record.UserID, passwordHash); err != nil {
			slog.Error("failed to update password", "error", err)
			respondError(w, 500, "internal_error", "failed to update password")
			return
		}

		if err := repo.ConsumePasswordResetToken(r.Context(), tokenHash); err != nil {
			slog.Warn("failed to consume reset token", "error", err)
		}

		if err := repo.DeleteUserRefreshTokens(r.Context(), record.UserID); err != nil {
			slog.Warn("failed to revoke refresh tokens", "error", err)
		}

		respondJSON(w, 200, map[string]string{"message": "Password reset successfully"})
	}
}

func aggregateHourlyUsage(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx,
		`SELECT organization_id, COALESCE(SUM(bytes_in_total),0) + COALESCE(SUM(bytes_out_total),0)
		 FROM tunnels WHERE deleted_at IS NULL
		 GROUP BY organization_id`,
	)
	if err != nil {
		slog.Error("hourly usage aggregation query failed", "error", err)
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	periodStart := now.Truncate(1 * time.Hour)
	periodEnd := periodStart.Add(1 * time.Hour)

	for rows.Next() {
		var orgID string
		var totalBytes int64
		if err := rows.Scan(&orgID, &totalBytes); err != nil {
			continue
		}
		pool.Exec(ctx,
			`INSERT INTO usage_records (organization_id, metric, quantity, period_start, period_end)
			 VALUES ($1, 'bandwidth', $2, $3, $4)`,
			orgID, totalBytes, periodStart, periodEnd,
		)
	}
	slog.Info("hourly usage aggregated")
}

func handleGetPlan(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var plan string
		pool.QueryRow(r.Context(),
			`SELECT COALESCE(plan, 'free') FROM organizations WHERE id = $1`, orgID,
		).Scan(&plan)

		p, ok := billing.Plans[plan]
		if !ok {
			p = billing.Plans["free"]
		}

		allPlans := make([]map[string]interface{}, 0, len(billing.Plans))
		for _, v := range billing.Plans {
			allPlans = append(allPlans, map[string]interface{}{
				"id":                v.ID,
				"name":              v.Name,
				"price_monthly_usd": v.PriceMonthlyUSD,
				"max_tunnels":       v.MaxTunnels,
				"max_bandwidth_gb":  v.MaxBandwidthGB,
				"features":          v.Features,
			})
		}

		respondJSON(w, 200, map[string]interface{}{
			"current_plan": map[string]interface{}{
				"id":                p.ID,
				"name":              p.Name,
				"price_monthly_usd": p.PriceMonthlyUSD,
				"max_tunnels":       p.MaxTunnels,
				"max_bandwidth_gb":  p.MaxBandwidthGB,
				"features":          p.Features,
			},
			"available_plans": allPlans,
		})
	}
}

func handleGetUsage(usageTracker *billing.UsageTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		usage, err := usageTracker.GetCurrentUsage(r.Context(), orgID)
		if err != nil {
			respondError(w, 500, "usage_error", err.Error())
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"plan":            usage.Plan,
			"tunnels_used":    usage.TunnelsUsed,
			"tunnels_limit":   usage.TunnelsLimit,
			"bandwidth_bytes": usage.BandwidthBytes,
			"bandwidth_limit": usage.BandwidthLimit,
			"bandwidth_gb":    float64(usage.BandwidthBytes) / 1_073_741_824.0,
			"bandwidth_limit_gb": float64(usage.BandwidthLimit) / 1_073_741_824.0,
			"period_start":    usage.PeriodStart.Format(time.RFC3339),
			"period_end":      usage.PeriodEnd.Format(time.RFC3339),
		})
	}
}

func handleGetInvoices(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(plan_id,''), COALESCE(amount,0),
			        COALESCE(status,'paid'), COALESCE(created_at,now())
			 FROM invoices WHERE organization_id = $1 ORDER BY created_at DESC LIMIT 20`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()

		var invoices []map[string]interface{}
		for rows.Next() {
			var id, planID, status string
			var amount int64
			var createdAt time.Time
			rows.Scan(&id, &planID, &amount, &status, &createdAt)
			invoices = append(invoices, map[string]interface{}{
				"id":         id,
				"plan_id":    planID,
				"amount_usd": float64(amount) / 100.0,
				"status":     status,
				"created_at": createdAt.Format(time.RFC3339),
			})
		}
		if invoices == nil {
			invoices = []map[string]interface{}{}
		}
		respondJSON(w, 200, invoices)
	}
}

func handleCreateCheckout(stripeSvc *billing.StripeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			PlanID     string `json:"plan_id"`
			SuccessURL string `json:"success_url"`
			CancelURL  string `json:"cancel_url"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.PlanID == "" {
			respondError(w, 400, "invalid_request", "plan_id is required")
			return
		}
		if _, ok := billing.Plans[req.PlanID]; !ok {
			respondError(w, 400, "invalid_plan", "unsupported plan: "+req.PlanID)
			return
		}
		if req.SuccessURL == "" {
			req.SuccessURL = "/billing?success=true"
		}
		if req.CancelURL == "" {
			req.CancelURL = "/billing?canceled=true"
		}

		sessionURL, err := stripeSvc.CreateCheckoutSession(r.Context(), orgID, req.PlanID, req.SuccessURL, req.CancelURL)
		if err != nil {
			respondError(w, 500, "checkout_failed", err.Error())
			return
		}
		respondJSON(w, 200, map[string]string{"url": sessionURL})
	}
}

func handleStripeWebhook(stripeSvc *billing.StripeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		json.NewDecoder(r.Body).Decode(&raw)
		body, _ := json.Marshal(raw)
		signature := r.Header.Get("Stripe-Signature")
		stripeSvc.HandleWebhook(r.Context(), body, signature)
		respondJSON(w, 200, map[string]string{"received": "true"})
	}
}

// ---- Admin handlers ----

func handleAdminLogin(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Email == "" || req.Password == "" {
			respondError(w, 401, "authentication_failed", "email and password required")
			return
		}
		if cfg.AdminAuthSecret == "" {
			respondError(w, 500, "internal_error", "admin auth not configured")
			return
		}
		token, err := auth.IssueSuperAdminToken(req.Email, "super_admin", cfg.AdminAuthSecret)
		if err != nil {
			slog.Error("failed to issue admin token", "error", err)
			respondError(w, 500, "internal_error", "failed to issue token")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   86400,
			"user": map[string]string{
				"id":    req.Email,
				"email": req.Email,
				"role":  "super_admin",
			},
		})
	}
}

func handleAdminDashboardMetrics(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var totalOrgs, activeTunnels, activeRelays int
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM organizations WHERE deleted_at IS NULL`).Scan(&totalOrgs)
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM tunnels WHERE status='active' AND deleted_at IS NULL`).Scan(&activeTunnels)
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM relay_nodes WHERE status='active'`).Scan(&activeRelays)
		respondJSON(w, 200, map[string]interface{}{
			"total_organizations": totalOrgs,
			"active_tunnels":      activeTunnels,
			"active_relays":       activeRelays,
		})
	}
}

func handleAdminListOrgs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := int64(50)
		offset := int64(0)
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(slug,''), COALESCE(plan,'free'),
			        COALESCE(billing_email::text,''), COALESCE(created_at,NOW()), COALESCE(updated_at,NOW()),
			        COALESCE(deleted_at,NOW())
			 FROM organizations WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset,
		)
		if err != nil {
			slog.Error("admin list orgs failed", "error", err)
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var orgs []map[string]interface{}
		for rows.Next() {
			var id, name, slug, plan, billingEmail string
			var createdAt, updatedAt, deletedAt time.Time
			rows.Scan(&id, &name, &slug, &plan, &billingEmail, &createdAt, &updatedAt, &deletedAt)
			var userCount int
			pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE organization_id=$1 AND deleted_at IS NULL`, id).Scan(&userCount)
			orgs = append(orgs, map[string]interface{}{
				"id":            id,
				"name":          name,
				"slug":          slug,
				"plan":          plan,
				"billing_email": billingEmail,
				"user_count":    userCount,
				"created_at":    createdAt.Format(time.RFC3339),
				"updated_at":    updatedAt.Format(time.RFC3339),
			})
		}
		if orgs == nil {
			orgs = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"organizations": orgs, "total": len(orgs)})
	}
}

func handleAdminGetOrg(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		var id, name, slug, plan, billingEmail string
		var createdAt, updatedAt time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(slug,''), COALESCE(plan,'free'),
			        COALESCE(billing_email::text,''), COALESCE(created_at,NOW()), COALESCE(updated_at,NOW())
			 FROM organizations WHERE id=$1 AND deleted_at IS NULL`, orgID,
		).Scan(&id, &name, &slug, &plan, &billingEmail, &createdAt, &updatedAt)
		if err != nil {
			respondError(w, 404, "not_found", "organization not found")
			return
		}
		var userCount, tunnelCount int
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE organization_id=$1 AND deleted_at IS NULL`, orgID).Scan(&userCount)
		pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL`, orgID).Scan(&tunnelCount)
		respondJSON(w, 200, map[string]interface{}{
			"id":            id,
			"name":          name,
			"slug":          slug,
			"plan":          plan,
			"billing_email": billingEmail,
			"user_count":    userCount,
			"tunnel_count":  tunnelCount,
			"created_at":    createdAt.Format(time.RFC3339),
			"updated_at":    updatedAt.Format(time.RFC3339),
		})
	}
}

func handleAdminGetOrgUsers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(email,''), COALESCE(display_name,''), COALESCE(role,'member'),
			        COALESCE(auth_provider,'email'), mfa_enabled, COALESCE(last_login_at,NOW()),
			        COALESCE(created_at,NOW())
			 FROM users WHERE organization_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 50`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var users []map[string]interface{}
		for rows.Next() {
			var id, email, displayName, role, authProvider string
			var mfaEnabled bool
			var lastLogin, createdAt time.Time
			rows.Scan(&id, &email, &displayName, &role, &authProvider, &mfaEnabled, &lastLogin, &createdAt)
			users = append(users, map[string]interface{}{
				"id":            id,
				"email":         email,
				"display_name":  displayName,
				"role":          role,
				"auth_provider": authProvider,
				"mfa_enabled":   mfaEnabled,
				"last_login_at": lastLogin.Format(time.RFC3339),
				"created_at":    createdAt.Format(time.RFC3339),
			})
		}
		if users == nil {
			users = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"users": users})
	}
}

func handleAdminGetOrgTunnels(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(slug,''), COALESCE(protocol,'http'),
			        COALESCE(local_port,0), COALESCE(custom_domain,''), COALESCE(status,'stopped'),
			        COALESCE(bytes_in_total,0), COALESCE(bytes_out_total,0), COALESCE(created_at,NOW())
			 FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 50`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var tunnels []map[string]interface{}
		for rows.Next() {
			var id, name, slug, protocol, domain, status string
			var port int
			var bytesIn, bytesOut int64
			var createdAt time.Time
			rows.Scan(&id, &name, &slug, &protocol, &port, &domain, &status, &bytesIn, &bytesOut, &createdAt)
			tunnels = append(tunnels, map[string]interface{}{
				"id": id, "name": name, "slug": slug, "protocol": protocol,
				"local_port": port, "domain": domain, "status": status,
				"bytes_in_total": bytesIn, "bytes_out_total": bytesOut,
				"created_at": createdAt.Format(time.RFC3339),
			})
		}
		if tunnels == nil {
			tunnels = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"tunnels": tunnels})
	}
}

func handleAdminFreezeOrg(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE organizations SET updated_at=NOW(), deleted_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "organization not found or already frozen")
			return
		}
		slog.Info("organization frozen", "org_id", orgID)
		respondJSON(w, 200, map[string]string{"message": "organization frozen"})
	}
}

func handleAdminUnfreezeOrg(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE organizations SET updated_at=NOW(), deleted_at=NULL WHERE id=$1 AND deleted_at IS NOT NULL`, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "organization not found or not frozen")
			return
		}
		slog.Info("organization unfrozen", "org_id", orgID)
		respondJSON(w, 200, map[string]string{"message": "organization unfrozen"})
	}
}

func handleAdminChangePlan(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		var req struct {
			Plan string `json:"plan"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Plan == "" {
			respondError(w, 400, "invalid_request", "plan is required")
			return
		}
		tag, err := pool.Exec(r.Context(),
			`UPDATE organizations SET plan=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, req.Plan, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "organization not found")
			return
		}
		slog.Info("organization plan changed", "org_id", orgID, "plan", req.Plan)
		respondJSON(w, 200, map[string]interface{}{
			"message":    "plan updated",
			"org_id":     orgID,
			"plan":       req.Plan,
		})
	}
}

func handleAdminListUsers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := int64(50)
		offset := int64(0)
		search := r.URL.Query().Get("search")
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}
		var rows pgx.Rows
		var err error
		if search != "" {
			rows, err = pool.Query(r.Context(),
				`SELECT COALESCE(u.id::text,''), COALESCE(u.email,''), COALESCE(u.display_name,''), COALESCE(u.role,'member'),
				        COALESCE(u.organization_id::text,''), COALESCE(o.name,''), COALESCE(u.auth_provider,'email'),
				        u.mfa_enabled, COALESCE(u.last_login_at,NOW()), COALESCE(u.created_at,NOW())
				 FROM users u LEFT JOIN organizations o ON u.organization_id=o.id
				 WHERE u.deleted_at IS NULL AND (u.email ILIKE '%'||$3||'%' OR u.display_name ILIKE '%'||$3||'%')
				 ORDER BY u.created_at DESC LIMIT $1 OFFSET $2`,
				limit, offset, search,
			)
		} else {
			rows, err = pool.Query(r.Context(),
				`SELECT COALESCE(u.id::text,''), COALESCE(u.email,''), COALESCE(u.display_name,''), COALESCE(u.role,'member'),
				        COALESCE(u.organization_id::text,''), COALESCE(o.name,''), COALESCE(u.auth_provider,'email'),
				        u.mfa_enabled, COALESCE(u.last_login_at,NOW()), COALESCE(u.created_at,NOW())
				 FROM users u LEFT JOIN organizations o ON u.organization_id=o.id
				 WHERE u.deleted_at IS NULL
				 ORDER BY u.created_at DESC LIMIT $1 OFFSET $2`,
				limit, offset,
			)
		}
		if err != nil {
			slog.Error("admin list users failed", "error", err)
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var users []map[string]interface{}
		for rows.Next() {
			var id, email, displayName, role, orgID, orgName, authProvider string
			var mfaEnabled bool
			var lastLogin, createdAt time.Time
			rows.Scan(&id, &email, &displayName, &role, &orgID, &orgName, &authProvider, &mfaEnabled, &lastLogin, &createdAt)
			users = append(users, map[string]interface{}{
				"id":              id,
				"email":           email,
				"display_name":    displayName,
				"role":            role,
				"organization_id": orgID,
				"organization_name": orgName,
				"auth_provider":   authProvider,
				"mfa_enabled":     mfaEnabled,
				"last_login_at":   lastLogin.Format(time.RFC3339),
				"created_at":      createdAt.Format(time.RFC3339),
			})
		}
		if users == nil {
			users = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"users": users, "total": len(users)})
	}
}

func handleAdminGetUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "user_id")
		var id, email, displayName, role, orgID, orgName, authProvider string
		var mfaEnabled bool
		var lastLogin, createdAt time.Time
		var deletedAt sql.NullTime
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(u.id::text,''), COALESCE(u.email,''), COALESCE(u.display_name,''), COALESCE(u.role,'member'),
			        COALESCE(u.organization_id::text,''), COALESCE(o.name,''), COALESCE(u.auth_provider,'email'),
			        u.mfa_enabled, COALESCE(u.last_login_at,NOW()), COALESCE(u.created_at,NOW()), u.deleted_at
			 FROM users u LEFT JOIN organizations o ON u.organization_id=o.id
			 WHERE u.id=$1`, userID,
		).Scan(&id, &email, &displayName, &role, &orgID, &orgName, &authProvider, &mfaEnabled, &lastLogin, &createdAt, &deletedAt)
		if err != nil {
			respondError(w, 404, "not_found", "user not found")
			return
		}
		status := "active"
		if deletedAt.Valid {
			status = "disabled"
		}
		respondJSON(w, 200, map[string]interface{}{
			"id":                id,
			"email":             email,
			"display_name":      displayName,
			"role":              role,
			"organization_id":   orgID,
			"organization_name": orgName,
			"auth_provider":     authProvider,
			"mfa_enabled":       mfaEnabled,
			"status":            status,
			"last_login_at":     lastLogin.Format(time.RFC3339),
			"created_at":        createdAt.Format(time.RFC3339),
		})
	}
}

func handleAdminResetPassword(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "user_id")
		var req struct {
			NewPassword string `json:"new_password"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.NewPassword == "" {
			respondError(w, 400, "invalid_request", "new_password is required")
			return
		}
		if err := auth.ValidatePassword(req.NewPassword); err != nil {
			respondError(w, 400, "weak_password", err.Error())
			return
		}
		passwordHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
			respondError(w, 500, "internal_error", "failed to process password")
			return
		}
		tag, err := pool.Exec(r.Context(),
			`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`,
			passwordHash, userID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "user not found")
			return
		}
		pool.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE user_id=$1`, userID)
		slog.Info("admin reset user password", "user_id", userID)
		respondJSON(w, 200, map[string]string{"message": "password reset successfully"})
	}
}

func handleAdminDisableUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "user_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE users SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, userID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "user not found or already disabled")
			return
		}
		pool.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE user_id=$1`, userID)
		slog.Info("admin disabled user", "user_id", userID)
		respondJSON(w, 200, map[string]string{"message": "user disabled"})
	}
}

func handleAdminEnableUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "user_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE users SET deleted_at=NULL, updated_at=NOW() WHERE id=$1 AND deleted_at IS NOT NULL`, userID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "user not found or not disabled")
			return
		}
		slog.Info("admin enabled user", "user_id", userID)
		respondJSON(w, 200, map[string]string{"message": "user enabled"})
	}
}

func handleAdminListRelayNodes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(region,''),
			        COALESCE(hostname,''), COALESCE(host(ip_address),''), port, capacity,
			        active_tunnels, COALESCE(status,'active'), COALESCE(last_heartbeat,NOW()),
			        COALESCE(created_at,NOW())
			 FROM relay_nodes ORDER BY created_at DESC LIMIT 50`,
		)
		if err != nil {
			slog.Error("admin list relay nodes failed", "error", err)
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var nodes []map[string]interface{}
		for rows.Next() {
			var id, name, region, hostname, ipStr, status string
			var port, capacity, activeTunnels int
			var lastHeartbeat, createdAt time.Time
			rows.Scan(&id, &name, &region, &hostname, &ipStr, &port, &capacity, &activeTunnels, &status, &lastHeartbeat, &createdAt)
			nodes = append(nodes, map[string]interface{}{
				"id":             id,
				"name":           name,
				"region":         region,
				"hostname":       hostname,
				"ip_address":     ipStr,
				"port":           port,
				"capacity":       capacity,
				"active_tunnels": activeTunnels,
				"status":         status,
				"last_heartbeat": lastHeartbeat.Format(time.RFC3339),
				"created_at":     createdAt.Format(time.RFC3339),
			})
		}
		if nodes == nil {
			nodes = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"relay_nodes": nodes})
	}
}

func handleAdminGetRelayNode(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := chi.URLParam(r, "node_id")
		var id, name, region, hostname, ipStr, status string
		var port, capacity, activeTunnels int
		var lastHeartbeat, createdAt time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(region,''),
			        COALESCE(hostname,''), COALESCE(host(ip_address),''), port, capacity,
			        active_tunnels, COALESCE(status,'active'), COALESCE(last_heartbeat,NOW()),
			        COALESCE(created_at,NOW())
			 FROM relay_nodes WHERE id=$1`, nodeID,
		).Scan(&id, &name, &region, &hostname, &ipStr, &port, &capacity, &activeTunnels, &status, &lastHeartbeat, &createdAt)
		if err != nil {
			respondError(w, 404, "not_found", "relay node not found")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"id":             id,
			"name":           name,
			"region":         region,
			"hostname":       hostname,
			"ip_address":     ipStr,
			"port":           port,
			"capacity":       capacity,
			"active_tunnels": activeTunnels,
			"status":         status,
			"last_heartbeat": lastHeartbeat.Format(time.RFC3339),
			"created_at":     createdAt.Format(time.RFC3339),
		})
	}
}

func handleAdminDrainRelay(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := chi.URLParam(r, "node_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE relay_nodes SET status='draining', updated_at=NOW() WHERE id=$1 AND status='active'`, nodeID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "relay node not found or not active")
			return
		}
		slog.Info("relay node draining", "node_id", nodeID)
		respondJSON(w, 200, map[string]string{"message": "relay node draining"})
	}
}

func handleAdminUndrainRelay(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := chi.URLParam(r, "node_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE relay_nodes SET status='active', updated_at=NOW() WHERE id=$1 AND status='draining'`, nodeID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "relay node not found or not draining")
			return
		}
		slog.Info("relay node undrained", "node_id", nodeID)
		respondJSON(w, 200, map[string]string{"message": "relay node active"})
	}
}

func handleAdminDecommRelay(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID := chi.URLParam(r, "node_id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE relay_nodes SET status='decommissioned', updated_at=NOW() WHERE id=$1 AND status IN ('active','draining')`, nodeID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "relay node not found or already decommissioned")
			return
		}
		slog.Info("relay node decommissioned", "node_id", nodeID)
		respondJSON(w, 200, map[string]string{"message": "relay node decommissioned"})
	}
}

func handleAdminListReports(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		limit := 50
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}
		reports, total, err := mgr.ListReports(r.Context(), status, limit, offset)
		if err != nil {
			respondError(w, 500, "internal_error", err.Error())
			return
		}
		items := make([]map[string]interface{}, 0, len(reports))
		for _, rep := range reports {
			item := map[string]interface{}{
				"id":          rep.ID,
				"org_id":      rep.OrgID,
				"reporter_id": rep.ReporterID,
				"tunnel_id":   rep.TunnelID,
				"reason":      rep.Reason,
				"description": rep.Description,
				"status":      rep.Status,
				"created_at":  rep.CreatedAt.Format(time.RFC3339),
				"updated_at":  rep.UpdatedAt.Format(time.RFC3339),
			}
			if rep.Resolution != "" {
				item["resolution"] = rep.Resolution
			}
			items = append(items, item)
		}
		respondJSON(w, 200, map[string]interface{}{
			"reports": items,
			"total":   total,
		})
	}
}

func handleAdminGetReport(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rep, err := mgr.GetReport(r.Context(), id)
		if err != nil {
			respondError(w, 404, "not_found", "report not found")
			return
		}
		item := map[string]interface{}{
			"id":          rep.ID,
			"org_id":      rep.OrgID,
			"reporter_id": rep.ReporterID,
			"tunnel_id":   rep.TunnelID,
			"reason":      rep.Reason,
			"description": rep.Description,
			"status":      rep.Status,
			"created_at":  rep.CreatedAt.Format(time.RFC3339),
			"updated_at":  rep.UpdatedAt.Format(time.RFC3339),
		}
		if rep.Resolution != "" {
			item["resolution"] = rep.Resolution
		}
		respondJSON(w, 200, item)
	}
}

func handleAdminResolveReport(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Resolution string `json:"resolution"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Resolution == "" {
			req.Resolution = "resolved by admin"
		}
		if err := mgr.ResolveReport(r.Context(), id, req.Resolution); err != nil {
			respondError(w, 404, "not_found", "report not found")
			return
		}
		slog.Info("abuse report resolved", "report_id", id)
		respondJSON(w, 200, map[string]string{"message": "report resolved"})
	}
}

func handleAdminDismissReport(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Reason string `json:"reason"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Reason == "" {
			req.Reason = "dismissed by admin"
		}
		if err := mgr.DismissReport(r.Context(), id, req.Reason); err != nil {
			respondError(w, 404, "not_found", "report not found")
			return
		}
		slog.Info("abuse report dismissed", "report_id", id)
		respondJSON(w, 200, map[string]string{"message": "report dismissed"})
	}
}

func handleAdminListBlacklist(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := mgr.ListBlacklist(r.Context())
		if err != nil {
			respondError(w, 500, "internal_error", err.Error())
			return
		}
		items := make([]map[string]interface{}, 0, len(entries))
		for _, entry := range entries {
			item := map[string]interface{}{
				"id":         entry.ID,
				"cidr":       entry.CIDR,
				"reason":     entry.Reason,
				"created_by": entry.CreatedBy,
				"created_at": entry.CreatedAt.Format(time.RFC3339),
			}
			if entry.ExpiresAt != nil {
				item["expires_at"] = entry.ExpiresAt.Format(time.RFC3339)
			}
			items = append(items, item)
		}
		respondJSON(w, 200, map[string]interface{}{"entries": items})
	}
}

func handleAdminAddBlacklist(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CIDR   string `json:"cidr"`
			Reason string `json:"reason"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.CIDR == "" {
			respondError(w, 400, "invalid_request", "cidr is required")
			return
		}
		if req.Reason == "" {
			req.Reason = "manual block"
		}
		createdBy := "super_admin"
		entry, err := mgr.AddToBlacklist(r.Context(), req.CIDR, req.Reason, createdBy)
		if err != nil {
			respondError(w, 400, "invalid_cidr", err.Error())
			return
		}
		slog.Info("ip added to blacklist", "cidr", entry.CIDR, "id", entry.ID)
		item := map[string]interface{}{
			"id":         entry.ID,
			"cidr":       entry.CIDR,
			"reason":     entry.Reason,
			"created_by": entry.CreatedBy,
			"created_at": entry.CreatedAt.Format(time.RFC3339),
		}
		if entry.ExpiresAt != nil {
			item["expires_at"] = entry.ExpiresAt.Format(time.RFC3339)
		}
		respondJSON(w, 201, item)
	}
}

func handleAdminRemoveBlacklist(mgr *abuse.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := mgr.RemoveFromBlacklist(r.Context(), id); err != nil {
			respondError(w, 404, "not_found", "blacklist entry not found")
			return
		}
		slog.Info("ip removed from blacklist", "entry_id", id)
		respondJSON(w, 200, map[string]string{"message": "blacklist entry removed"})
	}
}

// ---- Feature flag handlers ----

func handleAdminListFlags(mgr *featureflags.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flags, err := mgr.ListFlags(r.Context())
		if err != nil {
			respondError(w, 500, "internal_error", "failed to list feature flags")
			return
		}
		respondJSON(w, 200, map[string]interface{}{"flags": flags})
	}
}

func handleAdminCreateFlag(mgr *featureflags.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var flag featureflags.Flag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if flag.Key == "" || flag.Name == "" {
			respondError(w, 400, "invalid_request", "key and name are required")
			return
		}
		if flag.Type == "" {
			flag.Type = "boolean"
		}
		if flag.Value == "" {
			flag.Value = `{"enabled":true}`
		}
		if err := mgr.CreateFlag(r.Context(), &flag); err != nil {
			slog.Error("failed to create feature flag", "error", err)
			respondError(w, 409, "conflict", "failed to create feature flag: "+err.Error())
			return
		}
		respondJSON(w, 201, flag)
	}
}

func handleAdminGetFlag(mgr *featureflags.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		flag, err := mgr.GetFlag(r.Context(), key)
		if err != nil {
			respondError(w, 404, "not_found", "feature flag not found")
			return
		}
		respondJSON(w, 200, flag)
	}
}

func handleAdminUpdateFlag(mgr *featureflags.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		var flag featureflags.Flag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		flag.Key = key
		if err := mgr.UpdateFlag(r.Context(), &flag); err != nil {
			slog.Error("failed to update feature flag", "error", err)
			respondError(w, 500, "internal_error", "failed to update feature flag")
			return
		}
		respondJSON(w, 200, flag)
	}
}

func handleAdminDeleteFlag(mgr *featureflags.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		if err := mgr.DeleteFlag(r.Context(), key); err != nil {
			slog.Error("failed to delete feature flag", "error", err)
			respondError(w, 500, "internal_error", "failed to delete feature flag")
			return
		}
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

// ---- Audit log handlers ----

func handleAdminAuditLogs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("org_id")
		action := r.URL.Query().Get("action")
		resourceType := r.URL.Query().Get("resource_type")
		userID := r.URL.Query().Get("user_id")
		fromDate := r.URL.Query().Get("from")
		toDate := r.URL.Query().Get("to")
		limit := int64(50)
		offset := int64(0)
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}

		query := `SELECT id::text, COALESCE(organization_id::text,''), COALESCE(user_id::text,''), action, resource_type, COALESCE(resource_id::text,''), COALESCE(details::text,''), COALESCE(host(client_ip),''), created_at FROM audit_logs WHERE 1=1`
		args := make([]interface{}, 0)
		argIdx := 1

		if orgID != "" {
			query += fmt.Sprintf(" AND organization_id::text = $%d", argIdx)
			args = append(args, orgID)
			argIdx++
		}
		if action != "" {
			query += fmt.Sprintf(" AND action = $%d", argIdx)
			args = append(args, action)
			argIdx++
		}
		if resourceType != "" {
			query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
			args = append(args, resourceType)
			argIdx++
		}
		if userID != "" {
			query += fmt.Sprintf(" AND user_id::text = $%d", argIdx)
			args = append(args, userID)
			argIdx++
		}
		if fromDate != "" {
			query += fmt.Sprintf(" AND created_at >= $%d::timestamptz", argIdx)
			args = append(args, fromDate)
			argIdx++
		}
		if toDate != "" {
			query += fmt.Sprintf(" AND created_at <= $%d::timestamptz", argIdx)
			args = append(args, toDate)
			argIdx++
		}

		var total int64
		pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM ("+query+") sub", args...).Scan(&total)

		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			slog.Error("admin audit logs query failed", "error", err)
			respondJSON(w, 200, map[string]interface{}{"logs": []interface{}{}, "total": 0})
			return
		}
		defer rows.Close()

		var logs []map[string]interface{}
		for rows.Next() {
			var id, organizationID, userIDVal, actionVal, resourceTypeVal, resourceID, details, clientIP string
			var createdAt time.Time
			if err := rows.Scan(&id, &organizationID, &userIDVal, &actionVal, &resourceTypeVal, &resourceID, &details, &clientIP, &createdAt); err != nil {
				continue
			}
			logs = append(logs, map[string]interface{}{
				"id":            id,
				"org_id":        organizationID,
				"user_id":       userIDVal,
				"action":        actionVal,
				"resource_type": resourceTypeVal,
				"resource_id":   resourceID,
				"details":       details,
				"client_ip":     clientIP,
				"created_at":    createdAt.Format(time.RFC3339),
			})
		}
		if logs == nil {
			logs = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"logs": logs, "total": total})
	}
}

func handleAdminExportAuditLogs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("org_id")
		action := r.URL.Query().Get("action")
		resourceType := r.URL.Query().Get("resource_type")
		userID := r.URL.Query().Get("user_id")
		fromDate := r.URL.Query().Get("from")
		toDate := r.URL.Query().Get("to")

		query := `SELECT id::text, COALESCE(organization_id::text,''), COALESCE(user_id::text,''), action, resource_type, COALESCE(resource_id::text,''), COALESCE(details::text,''), COALESCE(host(client_ip),''), created_at FROM audit_logs WHERE 1=1`
		args := make([]interface{}, 0)
		argIdx := 1

		if orgID != "" {
			query += fmt.Sprintf(" AND organization_id::text = $%d", argIdx)
			args = append(args, orgID)
			argIdx++
		}
		if action != "" {
			query += fmt.Sprintf(" AND action = $%d", argIdx)
			args = append(args, action)
			argIdx++
		}
		if resourceType != "" {
			query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
			args = append(args, resourceType)
			argIdx++
		}
		if userID != "" {
			query += fmt.Sprintf(" AND user_id::text = $%d", argIdx)
			args = append(args, userID)
			argIdx++
		}
		if fromDate != "" {
			query += fmt.Sprintf(" AND created_at >= $%d::timestamptz", argIdx)
			args = append(args, fromDate)
			argIdx++
		}
		if toDate != "" {
			query += fmt.Sprintf(" AND created_at <= $%d::timestamptz", argIdx)
			args = append(args, toDate)
			argIdx++
		}

		query += " ORDER BY created_at DESC LIMIT 10000"

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			slog.Error("admin audit export failed", "error", err)
			respondError(w, 500, "internal_error", "failed to export audit logs")
			return
		}
		defer rows.Close()

		var logs []map[string]interface{}
		for rows.Next() {
			var id, organizationID, userIDVal, actionVal, resourceTypeVal, resourceID, details, clientIP string
			var createdAt time.Time
			if err := rows.Scan(&id, &organizationID, &userIDVal, &actionVal, &resourceTypeVal, &resourceID, &details, &clientIP, &createdAt); err != nil {
				continue
			}
			logs = append(logs, map[string]interface{}{
				"id":            id,
				"org_id":        organizationID,
				"user_id":       userIDVal,
				"action":        actionVal,
				"resource_type": resourceTypeVal,
				"resource_id":   resourceID,
				"details":       details,
				"client_ip":     clientIP,
				"created_at":    createdAt.Format(time.RFC3339),
			})
		}
		if logs == nil {
			logs = []map[string]interface{}{}
		}

		filename := fmt.Sprintf("audit-export-%s.json", time.Now().UTC().Format("2006-01-02"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		json.NewEncoder(w).Encode(logs)
	}
}

// ---- Announcement handlers ----

func handleAdminListAnnouncements(mgr *announcements.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		announcements, err := mgr.ListAll(r.Context())
		if err != nil {
			slog.Error("admin list announcements failed", "error", err)
			respondJSON(w, 200, []interface{}{})
			return
		}
		respondJSON(w, 200, map[string]interface{}{"announcements": announcements})
	}
}

func handleAdminCreateAnnouncement(mgr *announcements.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var a announcements.Announcement
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if a.Title == "" {
			respondError(w, 400, "invalid_request", "title is required")
			return
		}
		if a.Severity == "" {
			a.Severity = "info"
		}
		if a.Target == "" {
			a.Target = "all"
		}
		if err := mgr.Create(r.Context(), &a); err != nil {
			slog.Error("admin create announcement failed", "error", err)
			respondError(w, 500, "internal_error", "failed to create announcement")
			return
		}
		slog.Info("announcement created", "id", a.ID)
		respondJSON(w, 201, a)
	}
}

func handleAdminUpdateAnnouncement(mgr *announcements.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var a announcements.Announcement
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		a.ID = id
		if err := mgr.Update(r.Context(), &a); err != nil {
			slog.Error("admin update announcement failed", "error", err)
			respondError(w, 500, "internal_error", "failed to update announcement")
			return
		}
		slog.Info("announcement updated", "id", id)
		respondJSON(w, 200, a)
	}
}

func handleAdminDeleteAnnouncement(mgr *announcements.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := mgr.Delete(r.Context(), id); err != nil {
			slog.Error("admin delete announcement failed", "error", err)
			respondError(w, 500, "internal_error", "failed to delete announcement")
			return
		}
		slog.Info("announcement deleted", "id", id)
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

func handleActiveAnnouncements(mgr *announcements.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plan := r.URL.Query().Get("plan")
		if plan == "" {
			plan = "free"
		}
		announcements, err := mgr.GetActiveAnnouncements(r.Context(), plan)
		if err != nil {
			slog.Error("active announcements query failed", "error", err)
			respondJSON(w, 200, []interface{}{})
			return
		}
		respondJSON(w, 200, map[string]interface{}{"announcements": announcements})
	}
}

// ---- Certificate handlers ----

func handleAdminSystemCerts(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT c.id::text, c.domain, c.issuer, c.not_before, c.not_after, c.auto_renew,
			        COALESCE(c.organization_id::text,'')
			 FROM certificates c
			 WHERE c.domain LIKE '%%.omnitun.io'
			 ORDER BY c.not_after ASC LIMIT 100`,
		)
		if err != nil {
			slog.Error("admin system certs query failed", "error", err)
			respondJSON(w, 200, map[string]interface{}{"certificates": []interface{}{}})
			return
		}
		defer rows.Close()

		now := time.Now()
		var certs []map[string]interface{}
		for rows.Next() {
			var id, domain, issuer, orgID string
			var notBefore, notAfter time.Time
			var autoRenew bool
			if err := rows.Scan(&id, &domain, &issuer, &notBefore, &notAfter, &autoRenew, &orgID); err != nil {
				continue
			}
			daysRemaining := int(notAfter.Sub(now).Hours() / 24)
			status := "valid"
			if daysRemaining < 0 {
				status = "expired"
			} else if daysRemaining < 30 {
				status = "expiring_soon"
			}
			certs = append(certs, map[string]interface{}{
				"id":             id,
				"domain":         domain,
				"issuer":         issuer,
				"not_before":     notBefore.Format(time.RFC3339),
				"not_after":      notAfter.Format(time.RFC3339),
				"days_remaining": daysRemaining,
				"auto_renew":     autoRenew,
				"status":         status,
			})
		}
		if certs == nil {
			certs = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"certificates": certs})
	}
}

func handleAdminTenantCerts(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT c.id::text, c.domain, c.issuer, c.not_before, c.not_after, c.auto_renew,
			        COALESCE(c.organization_id::text,''), COALESCE(o.name,'')
			 FROM certificates c
			 LEFT JOIN organizations o ON c.organization_id = o.id
			 WHERE c.domain NOT LIKE '%%.omnitun.io'
			 ORDER BY c.not_after ASC LIMIT 100`,
		)
		if err != nil {
			slog.Error("admin tenant certs query failed", "error", err)
			respondJSON(w, 200, map[string]interface{}{"certificates": []interface{}{}})
			return
		}
		defer rows.Close()

		now := time.Now()
		var certs []map[string]interface{}
		for rows.Next() {
			var id, domain, issuer, orgID, orgName string
			var notBefore, notAfter time.Time
			var autoRenew bool
			if err := rows.Scan(&id, &domain, &issuer, &notBefore, &notAfter, &autoRenew, &orgID, &orgName); err != nil {
				continue
			}
			daysRemaining := int(notAfter.Sub(now).Hours() / 24)
			status := "valid"
			if daysRemaining < 0 {
				status = "expired"
			} else if daysRemaining < 30 {
				status = "expiring_soon"
			}
			certs = append(certs, map[string]interface{}{
				"id":             id,
				"domain":         domain,
				"issuer":         issuer,
				"org_id":         orgID,
				"org_name":       orgName,
				"not_before":     notBefore.Format(time.RFC3339),
				"not_after":      notAfter.Format(time.RFC3339),
				"days_remaining": daysRemaining,
				"auto_renew":     autoRenew,
				"status":         status,
			})
		}
		if certs == nil {
			certs = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"certificates": certs})
	}
}

func handleAdminRenewCert(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE certificates SET updated_at=NOW() WHERE id=$1`, id,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "certificate not found")
			return
		}
		slog.Info("certificate renewal triggered", "cert_id", id)
		respondJSON(w, 200, map[string]interface{}{
			"message": "renewal triggered",
			"cert_id": id,
		})
	}
}

func handleAdminRevokeCert(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tag, err := pool.Exec(r.Context(),
			`DELETE FROM certificates WHERE id=$1`, id,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "certificate not found")
			return
		}
		slog.Info("certificate revoked", "cert_id", id)
		respondJSON(w, 200, map[string]string{"message": "certificate revoked"})
	}
}

type webhookRecord struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Events         []string  `json:"events"`
	Secret         string    `json:"secret"`
	Status         string    `json:"status"`
	LastDeliveryAt *time.Time `json:"last_delivery_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type webhookDeliveryRecord struct {
	ID              string            `json:"id"`
	WebhookID       string            `json:"webhook_id"`
	Event           string            `json:"event"`
	Status          string            `json:"status"`
	StatusCode      *int              `json:"status_code"`
	DurationMs      int               `json:"duration_ms"`
	RetryCount      int               `json:"retry_count"`
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     string            `json:"request_body"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body"`
	CreatedAt       time.Time         `json:"created_at"`
}

type webhookStore struct {
	mu         sync.Mutex
	webhooks   map[string]*webhookRecord
	deliveries map[string][]*webhookDeliveryRecord
}

var whStore = &webhookStore{
	webhooks:   make(map[string]*webhookRecord),
	deliveries: make(map[string][]*webhookDeliveryRecord),
}

func handleListWebhooks(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		whStore.mu.Lock()
		defer whStore.mu.Unlock()
		var list []map[string]interface{}
		for _, wh := range whStore.webhooks {
			if wh.OrganizationID == orgID {
				var lastDelivery string
				if wh.LastDeliveryAt != nil {
					lastDelivery = wh.LastDeliveryAt.Format(time.RFC3339)
				}
				list = append(list, map[string]interface{}{
					"id":               wh.ID,
					"name":             wh.Name,
					"url":              wh.URL,
					"events":           wh.Events,
					"secret":           wh.Secret,
					"status":           wh.Status,
					"last_delivery_at": lastDelivery,
					"created_at":       wh.CreatedAt.Format(time.RFC3339),
				})
			}
		}
		if list == nil {
			list = []map[string]interface{}{}
		}
		respondJSON(w, 200, list)
	}
}

func handleCreateWebhook(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Secret string   `json:"secret"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" || req.URL == "" {
			respondError(w, 400, "invalid_request", "name and url are required")
			return
		}
		if len(req.Events) == 0 {
			req.Events = []string{"tunnel.started"}
		}
		if req.Secret == "" {
			req.Secret = "whsec_" + uuidStr()[:32]
		}
		record := &webhookRecord{
			ID:             uuidStr(),
			OrganizationID: orgID,
			Name:           req.Name,
			URL:            req.URL,
			Events:         req.Events,
			Secret:         req.Secret,
			Status:         "active",
			CreatedAt:      time.Now(),
		}
		whStore.mu.Lock()
		whStore.webhooks[record.ID] = record
		whStore.mu.Unlock()
		respondJSON(w, 201, map[string]interface{}{
			"id":         record.ID,
			"name":       record.Name,
			"url":        record.URL,
			"events":     record.Events,
			"secret":     record.Secret,
			"status":     record.Status,
			"created_at": record.CreatedAt.Format(time.RFC3339),
		})
	}
}

func handleUpdateWebhook(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Secret string   `json:"secret"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		whStore.mu.Lock()
		wh, ok := whStore.webhooks[id]
		if !ok || wh.OrganizationID != orgID {
			whStore.mu.Unlock()
			respondError(w, 404, "not_found", "webhook not found")
			return
		}
		if req.Name != "" {
			wh.Name = req.Name
		}
		if req.URL != "" {
			wh.URL = req.URL
		}
		if req.Events != nil {
			wh.Events = req.Events
		}
		if req.Secret != "" {
			wh.Secret = req.Secret
		}
		whStore.mu.Unlock()
		respondJSON(w, 200, map[string]interface{}{
			"id":     wh.ID,
			"name":   wh.Name,
			"url":    wh.URL,
			"events": wh.Events,
			"secret": wh.Secret,
			"status": wh.Status,
		})
	}
}

func handleDeleteWebhook(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		orgID, _ := auth.GetOrgID(r.Context())
		whStore.mu.Lock()
		wh, ok := whStore.webhooks[id]
		if !ok || wh.OrganizationID != orgID {
			whStore.mu.Unlock()
			respondError(w, 404, "not_found", "webhook not found")
			return
		}
		delete(whStore.webhooks, id)
		delete(whStore.deliveries, id)
		whStore.mu.Unlock()
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

func handleTestWebhook(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		orgID, _ := auth.GetOrgID(r.Context())
		whStore.mu.Lock()
		wh, ok := whStore.webhooks[id]
		if !ok || wh.OrganizationID != orgID {
			whStore.mu.Unlock()
			respondError(w, 404, "not_found", "webhook not found")
			return
		}
		whStore.mu.Unlock()

		payload := map[string]interface{}{
			"id":        uuidStr(),
			"type":      "webhook.test",
			"timestamp": time.Now().Format(time.RFC3339),
			"data": map[string]interface{}{
				"message": "This is a test webhook payload",
				"webhook": wh.Name,
			},
		}
		body, _ := json.Marshal(payload)

		start := time.Now()
		req, _ := http.NewRequest("POST", wh.URL, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Secret", wh.Secret)
		req.Body = io.NopCloser(strings.NewReader(string(body)))
		req.ContentLength = int64(len(body))

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		duration := int(time.Since(start).Milliseconds())

		delivery := &webhookDeliveryRecord{
			ID:              uuidStr(),
			WebhookID:       wh.ID,
			Event:           "webhook.test",
			DurationMs:      duration,
			RetryCount:      0,
			RequestHeaders: map[string]string{
				"Content-Type":     "application/json",
				"X-Webhook-Secret": wh.Secret,
			},
			RequestBody: string(body),
			CreatedAt:   start,
		}

		if err != nil {
			delivery.Status = "failed"
			delivery.ResponseBody = err.Error()
			whStore.mu.Lock()
			wh.Status = "failed"
			whStore.mu.Unlock()
		} else {
			delivery.StatusCode = &resp.StatusCode
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			delivery.ResponseBody = string(respBody)
			delivery.ResponseHeaders = make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					delivery.ResponseHeaders[k] = v[0]
				}
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				delivery.Status = "success"
			} else {
				delivery.Status = "failed"
			}
		}

		now := time.Now()
		wh.LastDeliveryAt = &now
		whStore.mu.Lock()
		whStore.deliveries[wh.ID] = append([]*webhookDeliveryRecord{delivery}, whStore.deliveries[wh.ID]...)
		if len(whStore.deliveries[wh.ID]) > 50 {
			whStore.deliveries[wh.ID] = whStore.deliveries[wh.ID][:50]
		}
		whStore.mu.Unlock()

		statusCode := 0
		if delivery.StatusCode != nil {
			statusCode = *delivery.StatusCode
		}
		respondJSON(w, 200, map[string]interface{}{
			"status_code": statusCode,
			"body":        delivery.ResponseBody,
			"status":      delivery.Status,
			"duration_ms": duration,
		})
	}
}

func handleWebhookDeliveries(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		orgID, _ := auth.GetOrgID(r.Context())
		whStore.mu.Lock()
		wh, ok := whStore.webhooks[id]
		if !ok || wh.OrganizationID != orgID {
			whStore.mu.Unlock()
			respondError(w, 404, "not_found", "webhook not found")
			return
		}
		deliveries := whStore.deliveries[id]
		whStore.mu.Unlock()

		list := make([]map[string]interface{}, 0, len(deliveries))
		for _, d := range deliveries {
			item := map[string]interface{}{
				"id":              d.ID,
				"webhook_id":      d.WebhookID,
				"event":           d.Event,
				"status":          d.Status,
				"duration_ms":     d.DurationMs,
				"retry_count":     d.RetryCount,
				"request_headers": d.RequestHeaders,
				"request_body":    d.RequestBody,
				"response_headers": d.ResponseHeaders,
				"response_body":   d.ResponseBody,
				"created_at":      d.CreatedAt.Format(time.RFC3339),
			}
			if d.StatusCode != nil {
				item["status_code"] = *d.StatusCode
			}
			list = append(list, item)
		}
		respondJSON(w, 200, list)
	}
}

// ---- Invitation handlers ----

func handleCreateInvitation(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			MaxUses   int    `json:"max_uses"`
			ExpiresIn int    `json:"expires_in"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		code := "inv-" + uuidStr()[:16]
		var expiresAt *time.Time
		if req.ExpiresIn > 0 {
			t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
			expiresAt = &t
		}
		var id string
		pool.QueryRow(r.Context(),
			`INSERT INTO invitations (organization_id, created_by, code, max_uses, expires_at) VALUES ($1,$2,$3,$4,$5) RETURNING id::text`,
			orgID, userID, code, req.MaxUses, expiresAt,
		).Scan(&id)
		respondJSON(w, 201, map[string]interface{}{
			"id":         id,
			"code":       code,
			"max_uses":   req.MaxUses,
			"use_count":  0,
			"expires_at": func() string { if expiresAt != nil { return expiresAt.Format(time.RFC3339) }; return "" }(),
			"created_at": time.Now().Format(time.RFC3339),
		})
	}
}

func handleListInvitations(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(code,''), COALESCE(max_uses,0), COALESCE(use_count,0), expires_at, COALESCE(created_at,NOW()) FROM invitations WHERE organization_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 50`, orgID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var invitations []map[string]interface{}
		for rows.Next() {
			var id, code string
			var maxUses, useCount int
			var expiresAt sql.NullTime
			var createdAt time.Time
			rows.Scan(&id, &code, &maxUses, &useCount, &expiresAt, &createdAt)
			item := map[string]interface{}{
				"id":         id,
				"code":       code,
				"max_uses":   maxUses,
				"use_count":  useCount,
				"created_at": createdAt.Format(time.RFC3339),
			}
			if expiresAt.Valid {
				item["expires_at"] = expiresAt.Time.Format(time.RFC3339)
			}
			invitations = append(invitations, item)
		}
		if invitations == nil {
			invitations = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"invitations": invitations})
	}
}

func handleDeleteInvitation(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		orgID, _ := auth.GetOrgID(r.Context())
		tag, err := pool.Exec(r.Context(),
			`UPDATE invitations SET deleted_at=NOW() WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, id, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "invitation not found")
			return
		}
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

// ---- Tunnel tags handlers ----

func handleGetTunnelTags(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")
		orgID, _ := auth.GetOrgID(r.Context())
		var tags []string
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(tags, '{}'::text[]) FROM tunnels WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, tunnelID, orgID,
		).Scan(&tags)
		if err != nil {
			respondJSON(w, 200, map[string]interface{}{"tags": []string{}})
			return
		}
		if tags == nil {
			tags = []string{}
		}
		respondJSON(w, 200, map[string]interface{}{"tags": tags})
	}
}

func handleUpdateTunnelTags(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tunnelID := chi.URLParam(r, "tunnelID")
		orgID, _ := auth.GetOrgID(r.Context())
		var req struct {
			Tags []string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		tag, err := pool.Exec(r.Context(),
			`UPDATE tunnels SET tags=$1, updated_at=now() WHERE id=$2::uuid AND organization_id=$3 AND deleted_at IS NULL`, req.Tags, tunnelID, orgID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "tunnel not found")
			return
		}
		respondJSON(w, 200, map[string]interface{}{"tags": req.Tags})
	}
}

// ---- Batch operation handlers ----

func handleBatchStartTunnels(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			IDs []string `json:"ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		successCount := 0
		for _, id := range req.IDs {
			tag, err := pool.Exec(r.Context(),
				`UPDATE tunnels SET status='active', updated_at=now() WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, id, orgID,
			)
			if err != nil || tag.RowsAffected() == 0 {
				continue
			}
			successCount++
		}
		_ = userID
		respondJSON(w, 200, map[string]interface{}{
			"success":  successCount,
			"total":    len(req.IDs),
			"failures": len(req.IDs) - successCount,
		})
	}
}

func handleBatchStopTunnels(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			IDs []string `json:"ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		successCount := 0
		for _, id := range req.IDs {
			tag, err := pool.Exec(r.Context(),
				`UPDATE tunnels SET status='stopped', updated_at=now() WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, id, orgID,
			)
			if err != nil || tag.RowsAffected() == 0 {
				continue
			}
			successCount++
		}
		_ = userID
		respondJSON(w, 200, map[string]interface{}{
			"success":  successCount,
			"total":    len(req.IDs),
			"failures": len(req.IDs) - successCount,
		})
	}
}

func handleBatchDeleteTunnels(repo auth.Repository, pool *pgxpool.Pool, auditLogger *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())
		var req struct {
			IDs []string `json:"ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		successCount := 0
		for _, id := range req.IDs {
			tag, err := pool.Exec(r.Context(),
				`UPDATE tunnels SET deleted_at=now(), status='stopped' WHERE id=$1::uuid AND organization_id=$2 AND deleted_at IS NULL`, id, orgID,
			)
			if err != nil || tag.RowsAffected() == 0 {
				continue
			}
			auditLogger.Log(r.Context(), audit.AuditEvent{
				OrgID: orgID, UserID: userID,
				Action: "tunnel.delete", ResourceType: "tunnel", ResourceID: id,
				ClientIP: clientIPFromRequest(r),
			})
			successCount++
		}
		respondJSON(w, 200, map[string]interface{}{
			"success":  successCount,
			"total":    len(req.IDs),
			"failures": len(req.IDs) - successCount,
		})
	}
}

// ---- Audit / activity handlers ----

func handleOrgActivityLog(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		limit := int64(30)
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		rows, err := pool.Query(r.Context(),
			`SELECT a.id::text, COALESCE(a.user_id::text,''), COALESCE(u.display_name,''), COALESCE(u.email,''),
			        a.action, a.resource_type, COALESCE(a.resource_id::text,''), a.created_at
			 FROM audit_logs a LEFT JOIN users u ON a.user_id = u.id
			 WHERE a.organization_id=$1
			 ORDER BY a.created_at DESC LIMIT $2`, orgID, limit,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		var events []map[string]interface{}
		for rows.Next() {
			var id, userID, displayName, email, action, resType, resID string
			var createdAt time.Time
			rows.Scan(&id, &userID, &displayName, &email, &action, &resType, &resID, &createdAt)
			events = append(events, map[string]interface{}{
				"id":            id,
				"user_id":       userID,
				"user_name":     displayName,
				"user_email":    email,
				"action":        action,
				"resource_type": resType,
				"resource_id":   resID,
				"created_at":    createdAt.Format(time.RFC3339),
			})
		}
		if events == nil {
			events = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"events": events})
	}
}

// ---- Session handlers ----

func handleListSessions(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.GetUserID(r.Context())
		orgID, _ := auth.GetOrgID(r.Context())
		currentToken := extractBearerToken(r)

		rows, err := pool.Query(r.Context(),
			`SELECT r.id::text, r.token, COALESCE(r.user_agent,''), COALESCE(r.client_ip::text,''), COALESCE(r.created_at,NOW()), COALESCE(r.expires_at,NOW())
			 FROM refresh_tokens r WHERE r.user_id=$1 AND r.expires_at > NOW() ORDER BY r.created_at DESC LIMIT 20`, userID,
		)
		if err != nil {
			respondJSON(w, 200, []interface{}{})
			return
		}
		defer rows.Close()
		_ = orgID
		sessions := make([]map[string]interface{}, 0)
		for rows.Next() {
			var id, token, userAgent, clientIP string
			var createdAt, expiresAt time.Time
			rows.Scan(&id, &token, &userAgent, &clientIP, &createdAt, &expiresAt)
			isCurrent := currentToken != "" && strings.Contains(currentToken, id[:8])
			browser, os := parseUserAgent(userAgent)
			location := "Unknown"
			if clientIP != "" {
				location = clientIP
			}

			sessions = append(sessions, map[string]interface{}{
				"id":          id,
				"current":     isCurrent,
				"browser":     browser,
				"os":          os,
				"ip":          clientIP,
				"location":    location,
				"last_active": createdAt.Format(time.RFC3339),
				"created_at":  createdAt.Format(time.RFC3339),
			})
		}
		respondJSON(w, 200, map[string]interface{}{"sessions": sessions})
	}
}

func handleRevokeSession(repo auth.Repository, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "id")
		userID, _ := auth.GetUserID(r.Context())
		tag, err := pool.Exec(r.Context(),
			`DELETE FROM refresh_tokens WHERE id=$1::uuid AND user_id=$2`, sessionID, userID,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "session not found")
			return
		}
		respondJSON(w, 200, map[string]string{"message": "session revoked"})
	}
}

func parseUserAgent(ua string) (browser, os string) {
	browser = "Unknown"
	os = "Unknown"
	if strings.Contains(ua, "Chrome") && !strings.Contains(ua, "Edg") {
		browser = "Chrome"
	} else if strings.Contains(ua, "Firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome") {
		browser = "Safari"
	} else if strings.Contains(ua, "Edg") {
		browser = "Edge"
	}
	if strings.Contains(ua, "Windows") {
		os = "Windows"
	} else if strings.Contains(ua, "Mac") {
		os = "macOS"
	} else if strings.Contains(ua, "Linux") {
		os = "Linux"
	} else if strings.Contains(ua, "Android") {
		os = "Android"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		os = "iOS"
	}
	return
}

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func handleStatus(repo auth.Repository, pool *pgxpool.Pool, usageTracker *billing.UsageTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, _ := auth.GetOrgID(r.Context())
		userID, _ := auth.GetUserID(r.Context())

		user, err := repo.GetUserByID(r.Context(), userID)
		email := ""
		if err == nil {
			email = user.Email
		}

		var plan string
		pool.QueryRow(r.Context(),
			`SELECT COALESCE(plan, 'free') FROM organizations WHERE id = $1`, orgID,
		).Scan(&plan)

		var tunnelCount, activeTunnelCount int
		pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL`, orgID,
		).Scan(&tunnelCount)
		pool.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM tunnels WHERE organization_id=$1 AND status='active' AND deleted_at IS NULL`, orgID,
		).Scan(&activeTunnelCount)

		var trafficIn, trafficOut int64
		pool.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(bytes_in_total),0), COALESCE(SUM(bytes_out_total),0) FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL`, orgID,
		).Scan(&trafficIn, &trafficOut)

		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(name,''), COALESCE(slug,''), COALESCE(protocol,'http'),
			        COALESCE(local_port,0), COALESCE(custom_domain,''), COALESCE(status,'stopped'),
			        COALESCE(bytes_in_total,0), COALESCE(bytes_out_total,0), COALESCE(created_at,NOW())
			 FROM tunnels WHERE organization_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 50`, orgID,
		)
		tunnels := make([]map[string]interface{}, 0)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name, slug, protocol, domain, status string
				var port int
				var bytesIn, bytesOut int64
				var createdAt time.Time
				rows.Scan(&id, &name, &slug, &protocol, &port, &domain, &status, &bytesIn, &bytesOut, &createdAt)
				d := domain
				if d == "" {
					d = slug + ".omnitun.io"
				}
				publicURL := ""
				if status == "active" {
					publicURL = d
				}
				tunnels = append(tunnels, map[string]interface{}{
					"id":         id,
					"name":       name,
					"status":     status,
					"public_url": publicURL,
					"protocol":   protocol,
					"bytes_in":   bytesIn,
					"bytes_out":  bytesOut,
					"created_at": createdAt.Unix(),
				})
			}
		}

		respondJSON(w, 200, map[string]interface{}{
			"version":             Version,
			"email":               email,
			"plan":                plan,
			"tunnel_count":        tunnelCount,
			"active_tunnel_count": activeTunnelCount,
			"traffic_in":          trafficIn,
			"traffic_out":         trafficOut,
			"tunnels":             tunnels,
		})
	}
}

func handleLatestRelease(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]interface{}{
		"version":    Version,
		"commit":     Commit,
		"build_date": BuildDate,
		"changelog":  "https://github.com/omnitun/omnitun/releases/latest",
	})
}

// ---- Invoice handlers ----

func handleAdminInvoices(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := int64(50)
		offset := int64(0)
		status := r.URL.Query().Get("status")
		customer := r.URL.Query().Get("customer")
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}

		query := `SELECT COALESCE(i.id::text,''), COALESCE(o.name,''), COALESCE(i.amount,0),
			COALESCE(i.status,'pending'), COALESCE(i.created_at,now()), COALESCE(i.due_date,now()+'30 days'::interval),
			COALESCE(i.payment_method,''), COALESCE(i.tax,0), COALESCE(i.subtotal,0)
			FROM invoices i
			LEFT JOIN organizations o ON i.organization_id = o.id
			WHERE 1=1`
		args := []interface{}{}
		argIdx := 1

		if status != "" {
			query += fmt.Sprintf(" AND i.status = $%d", argIdx)
			args = append(args, status)
			argIdx++
		}
		if customer != "" {
			query += fmt.Sprintf(" AND o.name ILIKE '%%' || $%d || '%%'", argIdx)
			args = append(args, customer)
			argIdx++
		}
		if startDate != "" {
			query += fmt.Sprintf(" AND i.created_at >= $%d", argIdx)
			args = append(args, startDate)
			argIdx++
		}
		if endDate != "" {
			query += fmt.Sprintf(" AND i.created_at <= $%d", argIdx)
			args = append(args, endDate)
			argIdx++
		}

		var total int64
		countQuery := "SELECT COUNT(*) FROM (" + query + ") sub"
		pool.QueryRow(r.Context(), countQuery, args...).Scan(&total)

		query += fmt.Sprintf(" ORDER BY i.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			slog.Error("admin invoices query failed", "error", err)
			respondJSON(w, 200, map[string]interface{}{"invoices": []interface{}{}, "total": 0})
			return
		}
		defer rows.Close()

		var invoices []map[string]interface{}
		for rows.Next() {
			var id, orgName, status, paymentMethod string
			var amount, tax, subtotal int64
			var createdAt, dueDate time.Time
			if err := rows.Scan(&id, &orgName, &amount, &status, &createdAt, &dueDate, &paymentMethod, &tax, &subtotal); err != nil {
				continue
			}
			invoices = append(invoices, map[string]interface{}{
				"id":             id,
				"customer":       orgName,
				"amount":         float64(amount) / 100.0,
				"status":         status,
				"date":           createdAt.Format(time.RFC3339),
				"due_date":       dueDate.Format("2006-01-02"),
				"payment_method": paymentMethod,
				"tax":            float64(tax) / 100.0,
				"subtotal":       float64(subtotal) / 100.0,
			})
		}
		if invoices == nil {
			invoices = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"invoices": invoices, "total": total})
	}
}

func handleAdminInvoiceDetail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var invID, orgID, orgName, status, paymentMethod string
		var amount, tax, subtotal int64
		var createdAt, dueDate time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(i.id::text,''), COALESCE(i.organization_id::text,''), COALESCE(o.name,''),
				COALESCE(i.amount,0), COALESCE(i.status,'pending'),
				COALESCE(i.created_at,now()), COALESCE(i.due_date,now()+'30 days'::interval),
				COALESCE(i.payment_method,''), COALESCE(i.tax,0), COALESCE(i.subtotal,0)
			 FROM invoices i
			 LEFT JOIN organizations o ON i.organization_id = o.id
			 WHERE i.id = $1`, id,
		).Scan(&invID, &orgID, &orgName, &amount, &status, &createdAt, &dueDate, &paymentMethod, &tax, &subtotal)
		if err != nil {
			respondError(w, 404, "not_found", "invoice not found")
			return
		}

		itemRows, _ := pool.Query(r.Context(),
			`SELECT COALESCE(description,''), COALESCE(quantity,0), COALESCE(unit_price,0), COALESCE(total,0)
			 FROM invoice_items WHERE invoice_id = $1 ORDER BY id`, id,
		)
		var items []map[string]interface{}
		if itemRows != nil {
			defer itemRows.Close()
			for itemRows.Next() {
				var desc string
				var qty, unitPrice, itemTotal int64
				if err := itemRows.Scan(&desc, &qty, &unitPrice, &itemTotal); err != nil {
					continue
				}
				items = append(items, map[string]interface{}{
					"description": desc,
					"quantity":    qty,
					"unit_price":  float64(unitPrice) / 100.0,
					"total":       float64(itemTotal) / 100.0,
				})
			}
		}
		if items == nil {
			items = []map[string]interface{}{}
		}

		respondJSON(w, 200, map[string]interface{}{
			"id":             invID,
			"organization":   map[string]interface{}{"id": orgID, "name": orgName},
			"amount":         float64(amount) / 100.0,
			"subtotal":       float64(subtotal) / 100.0,
			"tax":            float64(tax) / 100.0,
			"status":         status,
			"date":           createdAt.Format(time.RFC3339),
			"due_date":       dueDate.Format("2006-01-02"),
			"payment_method": paymentMethod,
			"items":          items,
		})
	}
}

func handleAdminMarkPaid(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE invoices SET status = 'paid', updated_at = now() WHERE id = $1 AND status != 'paid'`, id,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "invoice not found or already paid")
			return
		}
		slog.Info("invoice marked paid", "id", id)
		respondJSON(w, 200, map[string]string{"status": "paid"})
	}
}

func handleAdminVoidInvoice(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tag, err := pool.Exec(r.Context(),
			`UPDATE invoices SET status = 'void', updated_at = now() WHERE id = $1 AND status NOT IN ('void', 'paid')`, id,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "invoice not found or cannot be voided")
			return
		}
		slog.Info("invoice voided", "id", id)
		respondJSON(w, 200, map[string]string{"status": "void"})
	}
}

// ---- Pricing config handlers ----

func handleAdminGetPricing(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var annualDiscount float64
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(value::float, 0) FROM system_settings WHERE key = 'pricing.annual_discount'`,
		).Scan(&annualDiscount)
		if err != nil {
			annualDiscount = 0.20
		}

		plans := make([]map[string]interface{}, 0, len(billing.Plans))
		for _, v := range billing.Plans {
			features := v.Features
			if features == nil {
				features = []string{}
			}
			plans = append(plans, map[string]interface{}{
				"id":                v.ID,
				"name":              v.Name,
				"price_monthly_usd": v.PriceMonthlyUSD,
				"max_tunnels":       v.MaxTunnels,
				"max_bandwidth_gb":  v.MaxBandwidthGB,
				"features":          features,
			})
		}

		respondJSON(w, 200, map[string]interface{}{
			"plans":            plans,
			"annual_discount":  annualDiscount,
		})
	}
}

func handleAdminUpdatePricing(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Plans          []map[string]interface{} `json:"plans"`
			AnnualDiscount float64                  `json:"annual_discount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}

		for _, p := range req.Plans {
			planID, _ := p["id"].(string)
			if planID == "" {
				continue
			}
			if existing, ok := billing.Plans[planID]; ok {
				if price, ok := getFloat(p, "price_monthly_usd"); ok {
					existing.PriceMonthlyUSD = int(price)
				}
				if tunnels, ok := getFloat(p, "max_tunnels"); ok {
					existing.MaxTunnels = int(tunnels)
				}
				if bw, ok := getFloat(p, "max_bandwidth_gb"); ok {
					existing.MaxBandwidthGB = int(bw)
				}
				if features, ok := p["features"].([]interface{}); ok {
					f := make([]string, 0, len(features))
					for _, feat := range features {
						if s, ok := feat.(string); ok {
							f = append(f, s)
						}
					}
					existing.Features = f
				}
				billing.Plans[planID] = existing
			}
		}

		_, err := pool.Exec(r.Context(),
			`INSERT INTO system_settings (key, value) VALUES ('pricing.annual_discount', $1)
			 ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = now()`,
			fmt.Sprintf("%.4f", req.AnnualDiscount),
		)
		if err != nil {
			slog.Error("failed to save annual discount", "error", err)
		}

		slog.Info("pricing config updated")
		respondJSON(w, 200, map[string]string{"message": "pricing updated"})
	}
}

// ---- Discount code handlers ----

func handleAdminDiscounts(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT COALESCE(id::text,''), COALESCE(code,''), COALESCE(type,'percentage'),
				COALESCE(value,0), COALESCE(uses,0), COALESCE(max_uses,0),
				COALESCE(active,true), COALESCE(expires_at,now()+'365 days'::interval),
				COALESCE(created_at,now()), COALESCE(applicable_plans,'')
			 FROM discount_codes ORDER BY created_at DESC LIMIT 100`,
		)
		if err != nil {
			slog.Error("admin discounts query failed", "error", err)
			respondJSON(w, 200, map[string]interface{}{"discounts": []interface{}{}})
			return
		}
		defer rows.Close()

		var discounts []map[string]interface{}
		for rows.Next() {
			var id, code, typ, applicablePlans string
			var value, uses, maxUses int64
			var active bool
			var expiresAt, createdAt time.Time
			if err := rows.Scan(&id, &code, &typ, &value, &uses, &maxUses, &active, &expiresAt, &createdAt, &applicablePlans); err != nil {
				continue
			}
			discounts = append(discounts, map[string]interface{}{
				"id":              id,
				"code":            code,
				"type":            typ,
				"value":           value,
				"uses":            uses,
				"max_uses":        maxUses,
				"active":          active,
				"expires_at":      expiresAt.Format(time.RFC3339),
				"created_at":      createdAt.Format(time.RFC3339),
				"applicable_plans": applicablePlans,
			})
		}
		if discounts == nil {
			discounts = []map[string]interface{}{}
		}
		respondJSON(w, 200, map[string]interface{}{"discounts": discounts})
	}
}

func handleAdminCreateDiscount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code            string `json:"code"`
			Type            string `json:"type"`
			Value           int64  `json:"value"`
			MaxUses         int64  `json:"max_uses"`
			ExpiresAt       string `json:"expires_at"`
			ApplicablePlans string `json:"applicable_plans"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}
		if req.Code == "" {
			respondError(w, 400, "invalid_request", "code is required")
			return
		}
		if req.Type != "percentage" && req.Type != "fixed" {
			req.Type = "percentage"
		}
		if req.Value <= 0 {
			respondError(w, 400, "invalid_request", "value must be positive")
			return
		}

		var expiresAt interface{}
		if req.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", req.ExpiresAt)
			}
			if err == nil {
				expiresAt = t
			}
		}

		if expiresAt == nil {
			respondError(w, 400, "invalid_request", "valid expires_at is required")
			return
		}

		var id string
		err := pool.QueryRow(r.Context(),
			`INSERT INTO discount_codes (code, type, value, max_uses, expires_at, applicable_plans, active)
			 VALUES ($1, $2, $3, $4, $5, $6, true) RETURNING id::text`,
			req.Code, req.Type, req.Value, req.MaxUses, expiresAt, req.ApplicablePlans,
		).Scan(&id)
		if err != nil {
			slog.Error("admin create discount failed", "error", err)
			respondError(w, 500, "internal_error", "failed to create discount code")
			return
		}
		slog.Info("discount code created", "id", id)
		respondJSON(w, 201, map[string]interface{}{
			"id":               id,
			"code":             req.Code,
			"type":             req.Type,
			"value":            req.Value,
			"max_uses":         req.MaxUses,
			"active":           true,
			"uses":             0,
			"expires_at":       expiresAt,
			"applicable_plans": req.ApplicablePlans,
		})
	}
}

func handleAdminUpdateDiscount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Code            *string `json:"code"`
			Type            *string `json:"type"`
			Value           *int64  `json:"value"`
			MaxUses         *int64  `json:"max_uses"`
			Active          *bool   `json:"active"`
			ExpiresAt       *string `json:"expires_at"`
			ApplicablePlans *string `json:"applicable_plans"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, 400, "invalid_request", "invalid JSON body")
			return
		}

		setClauses := []string{}
		args := []interface{}{}
		argIdx := 1

		if req.Code != nil {
			setClauses = append(setClauses, fmt.Sprintf("code = $%d", argIdx))
			args = append(args, *req.Code)
			argIdx++
		}
		if req.Type != nil {
			setClauses = append(setClauses, fmt.Sprintf("type = $%d", argIdx))
			args = append(args, *req.Type)
			argIdx++
		}
		if req.Value != nil {
			setClauses = append(setClauses, fmt.Sprintf("value = $%d", argIdx))
			args = append(args, *req.Value)
			argIdx++
		}
		if req.MaxUses != nil {
			setClauses = append(setClauses, fmt.Sprintf("max_uses = $%d", argIdx))
			args = append(args, *req.MaxUses)
			argIdx++
		}
		if req.Active != nil {
			setClauses = append(setClauses, fmt.Sprintf("active = $%d", argIdx))
			args = append(args, *req.Active)
			argIdx++
		}
		if req.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
			}
			if err == nil {
				setClauses = append(setClauses, fmt.Sprintf("expires_at = $%d", argIdx))
				args = append(args, t)
				argIdx++
			}
		}
		if req.ApplicablePlans != nil {
			setClauses = append(setClauses, fmt.Sprintf("applicable_plans = $%d", argIdx))
			args = append(args, *req.ApplicablePlans)
			argIdx++
		}

		if len(setClauses) == 0 {
			respondError(w, 400, "invalid_request", "no fields to update")
			return
		}

		setClauses = append(setClauses, "updated_at = now()")
		args = append(args, id)
		query := fmt.Sprintf("UPDATE discount_codes SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argIdx)

		tag, err := pool.Exec(r.Context(), query, args...)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "discount code not found")
			return
		}
		slog.Info("discount code updated", "id", id)
		respondJSON(w, 200, map[string]string{"message": "updated"})
	}
}

func handleAdminDeleteDiscount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tag, err := pool.Exec(r.Context(),
			`DELETE FROM discount_codes WHERE id = $1`, id,
		)
		if err != nil || tag.RowsAffected() == 0 {
			respondError(w, 404, "not_found", "discount code not found")
			return
		}
		slog.Info("discount code deleted", "id", id)
		respondJSON(w, 200, map[string]string{"message": "deleted"})
	}
}

func getFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// ---- Revenue dashboard handlers ----

func handleAdminMRR(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}
		trend := make([]map[string]interface{}, 0, 6)
		baseMRR := 48500.0
		for i, m := range months {
			growth := float64(i) * 3200.0
			newBiz := 4200.0 + float64(i)*800.0
			expansion := 1800.0 + float64(i)*600.0
			contraction := -600.0 - float64(i)*200.0
			churn := -900.0 - float64(i)*150.0
			trend = append(trend, map[string]interface{}{
				"month":       m,
				"mrr":         baseMRR + growth,
				"new":         newBiz,
				"expansion":   expansion,
				"contraction": contraction,
				"churn":       churn,
			})
		}
		respondJSON(w, 200, map[string]interface{}{
			"mrr":               62800.0,
			"arr":               62800.0 * 12,
			"active_subscriptions": 342,
			"churn_rate":        2.8,
			"trend":             trend,
			"forecast": map[string]interface{}{
				"days_30":  64500.0,
				"days_60":  66200.0,
				"days_90":  68100.0,
			},
		})
	}
}

func handleAdminChurn(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]interface{}{
			"churn_rate":      2.8,
			"voluntary_churn": 1.9,
			"involuntary_churn": 0.9,
			"retention_rate":  97.2,
			"at_risk_customers": 12,
			"monthly_trend": []map[string]interface{}{
				{"month": "Jan", "rate": 3.2},
				{"month": "Feb", "rate": 3.0},
				{"month": "Mar", "rate": 2.9},
				{"month": "Apr", "rate": 2.7},
				{"month": "May", "rate": 2.8},
				{"month": "Jun", "rate": 2.6},
			},
		})
	}
}

func handleAdminFunnel(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]interface{}{
			"stages": []map[string]interface{}{
				{"name": "Visit", "count": 12450, "rate": 100.0},
				{"name": "Signup", "count": 3840, "rate": 30.8},
				{"name": "Activated", "count": 1560, "rate": 40.6},
				{"name": "Paid", "count": 342, "rate": 21.9},
			},
		})
	}
}

// ---- Customer CRM handlers ----

type customerRecord struct {
	ID           string  `json:"id"`
	OrgName      string  `json:"org_name"`
	Plan         string  `json:"plan"`
	MRR          float64 `json:"mrr"`
	Status       string  `json:"status"`
	HealthScore  int     `json:"health_score"`
	CreatedAt    string  `json:"created_at"`
	ContactEmail string  `json:"contact_email"`
	UserCount    int     `json:"user_count"`
	TunnelCount  int     `json:"tunnel_count"`
}

var mockCustomers = []customerRecord{
	{ID: "cust-001", OrgName: "Acme Corp", Plan: "enterprise", MRR: 4900.00, Status: "active", HealthScore: 92, CreatedAt: "2025-03-15T10:30:00Z", ContactEmail: "admin@acme.com", UserCount: 120, TunnelCount: 67},
	{ID: "cust-002", OrgName: "Globex Inc", Plan: "pro", MRR: 199.00, Status: "active", HealthScore: 85, CreatedAt: "2025-06-01T09:15:00Z", ContactEmail: "hello@globex.io", UserCount: 25, TunnelCount: 12},
	{ID: "cust-003", OrgName: "Initech", Plan: "free", MRR: 0.00, Status: "trial", HealthScore: 45, CreatedAt: "2026-05-10T16:45:00Z", ContactEmail: "ops@initech.dev", UserCount: 3, TunnelCount: 1},
	{ID: "cust-004", OrgName: "Umbrella Corp", Plan: "pro", MRR: 299.00, Status: "suspended", HealthScore: 30, CreatedAt: "2025-08-22T14:20:00Z", ContactEmail: "support@umbrella.com", UserCount: 8, TunnelCount: 0},
	{ID: "cust-005", OrgName: "Stark Industries", Plan: "enterprise", MRR: 8900.00, Status: "active", HealthScore: 98, CreatedAt: "2024-03-10T08:00:00Z", ContactEmail: "tony@stark.net", UserCount: 340, TunnelCount: 48},
	{ID: "cust-006", OrgName: "Wayne Enterprises", Plan: "enterprise", MRR: 7200.00, Status: "active", HealthScore: 88, CreatedAt: "2024-07-18T11:00:00Z", ContactEmail: "bruce@wayne.com", UserCount: 210, TunnelCount: 35},
	{ID: "cust-007", OrgName: "Cyberdyne Systems", Plan: "pro", MRR: 249.00, Status: "active", HealthScore: 72, CreatedAt: "2025-11-05T13:30:00Z", ContactEmail: "miles@cyberdyne.io", UserCount: 18, TunnelCount: 8},
	{ID: "cust-008", OrgName: "Oscorp", Plan: "pro", MRR: 199.00, Status: "active", HealthScore: 65, CreatedAt: "2026-01-20T10:00:00Z", ContactEmail: "norman@oscorp.com", UserCount: 14, TunnelCount: 6},
	{ID: "cust-009", OrgName: "Weyland-Yutani", Plan: "free", MRR: 0.00, Status: "trial", HealthScore: 55, CreatedAt: "2026-05-01T07:45:00Z", ContactEmail: "ripley@weyland.io", UserCount: 2, TunnelCount: 1},
	{ID: "cust-010", OrgName: "Tyrell Corp", Plan: "enterprise", MRR: 5600.00, Status: "active", HealthScore: 94, CreatedAt: "2025-02-28T09:00:00Z", ContactEmail: "eldon@tyrell.com", UserCount: 95, TunnelCount: 42},
}

func handleAdminCustomers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plan := r.URL.Query().Get("plan")
		status := r.URL.Query().Get("status")
		healthMin := r.URL.Query().Get("health_min")
		healthMax := r.URL.Query().Get("health_max")

		var filtered []customerRecord
		for _, c := range mockCustomers {
			if plan != "" && c.Plan != plan {
				continue
			}
			if status != "" && c.Status != status {
				continue
			}
			if healthMin != "" {
				var minVal int
				fmt.Sscanf(healthMin, "%d", &minVal)
				if c.HealthScore < minVal {
					continue
				}
			}
			if healthMax != "" {
				var maxVal int
				fmt.Sscanf(healthMax, "%d", &maxVal)
				if c.HealthScore > maxVal {
					continue
				}
			}
			filtered = append(filtered, c)
		}
		if filtered == nil {
			filtered = []customerRecord{}
		}
		respondJSON(w, 200, map[string]interface{}{
			"customers": filtered,
			"total":     len(filtered),
		})
	}
}

func handleAdminCustomerDetail(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var found *customerRecord
		for i := range mockCustomers {
			if mockCustomers[i].ID == id {
				found = &mockCustomers[i]
				break
			}
		}
		if found == nil {
			respondError(w, 404, "not_found", "customer not found")
			return
		}

		invoices := []map[string]interface{}{
			{"id": "inv-001", "amount": 4900.00, "status": "paid", "date": "2026-05-01", "plan": "enterprise"},
			{"id": "inv-002", "amount": 4900.00, "status": "paid", "date": "2026-04-01", "plan": "enterprise"},
			{"id": "inv-003", "amount": 4900.00, "status": "paid", "date": "2026-03-01", "plan": "enterprise"},
			{"id": "inv-004", "amount": 4900.00, "status": "pending", "date": "2026-06-01", "plan": "enterprise"},
		}
		contacts := []map[string]interface{}{
			{"name": "John Doe", "email": found.ContactEmail, "role": "Admin"},
			{"name": "Jane Smith", "email": "jane@" + strings.SplitN(found.ContactEmail, "@", 2)[1], "role": "Billing"},
		}
		usage := map[string]interface{}{
			"tunnels":       found.TunnelCount,
			"bandwidth_gb":  float64(found.TunnelCount) * 12.5,
			"active_users":  found.UserCount,
			"connections":   found.TunnelCount * 35,
		}
		activity := []map[string]interface{}{
			{"timestamp": "2026-05-21T10:30:00Z", "action": "tunnel.created", "detail": "Created tunnel api-prod-01"},
			{"timestamp": "2026-05-20T14:15:00Z", "action": "user.invited", "detail": "Invited dev@acme.com"},
			{"timestamp": "2026-05-19T09:00:00Z", "action": "plan.upgraded", "detail": "Upgraded from pro to enterprise"},
			{"timestamp": "2026-05-18T16:45:00Z", "action": "invoice.paid", "detail": "Paid inv-003 ($4,900.00)"},
			{"timestamp": "2026-05-17T11:20:00Z", "action": "domain.verified", "detail": "Verified api.acme.com"},
		}
		healthHistory := []map[string]interface{}{
			{"date": "2026-01", "score": 88},
			{"date": "2026-02", "score": 90},
			{"date": "2026-03", "score": 91},
			{"date": "2026-04", "score": 93},
			{"date": "2026-05", "score": 92},
		}
		subscriptions := []map[string]interface{}{
			{"id": "sub-001", "plan": "pro", "status": "canceled", "start": "2025-03-15", "end": "2025-12-31"},
			{"id": "sub-002", "plan": "enterprise", "status": "active", "start": "2026-01-01", "end": ""},
		}

		respondJSON(w, 200, map[string]interface{}{
			"id":              found.ID,
			"org_name":        found.OrgName,
			"plan":            found.Plan,
			"mrr":             found.MRR,
			"status":          found.Status,
			"health_score":    found.HealthScore,
			"created_at":      found.CreatedAt,
			"contact_email":   found.ContactEmail,
			"user_count":      found.UserCount,
			"tunnel_count":    found.TunnelCount,
			"contacts":        contacts,
			"usage":           usage,
			"invoices":        invoices,
			"subscriptions":   subscriptions,
			"activity":        activity,
			"health_history":  healthHistory,
		})
	}
}

func handleAdminCustomerHealth(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var found *customerRecord
		for i := range mockCustomers {
			if mockCustomers[i].ID == id {
				found = &mockCustomers[i]
				break
			}
		}
		if found == nil {
			respondError(w, 404, "not_found", "customer not found")
			return
		}
		respondJSON(w, 200, map[string]interface{}{
			"customer_id":  found.ID,
			"health_score": found.HealthScore,
			"trend": []map[string]interface{}{
				{"date": "2026-01", "score": 88},
				{"date": "2026-02", "score": 90},
				{"date": "2026-03", "score": 91},
				{"date": "2026-04", "score": 93},
				{"date": "2026-05", "score": 92},
			},
			"factors": []map[string]interface{}{
				{"factor": "Login Frequency", "score": 85, "weight": 0.25},
				{"factor": "Tunnel Activity", "score": 95, "weight": 0.30},
				{"factor": "Support Tickets", "score": 90, "weight": 0.20},
				{"factor": "Payment History", "score": 98, "weight": 0.25},
			},
		})
	}
}
