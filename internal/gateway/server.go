package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/omnitun/omnitun/pkg/metrics"
)

type Server struct {
	cfg      *Config
	Hub      *Hub
	upgrader websocket.Upgrader
	authMgr  *AuthManager
	srv      *http.Server
}

type Config struct {
	ListenAddr   string
	CertFile     string
	KeyFile      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	JWTSecret    string
}

type Session struct {
	ID        string `json:"id"`
	TunnelID  string `json:"tunnel_id"`
	ClientIP  string `json:"client_ip"`
	CreatedAt int64  `json:"created_at"`
	BytesIn   int64  `json:"bytes_in"`
	BytesOut  int64  `json:"bytes_out"`
}

func NewServer(cfg *Config, authMgr *AuthManager) *Server {
	s := &Server{
		cfg:     cfg,
		Hub:     NewHub(),
		authMgr: authMgr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	go s.Hub.Run()

	return s
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent/v1/connect", s.HandleAgentConnect)
	mux.HandleFunc("/health", s.handleHealth)
	mux.Handle("/metrics", metrics.Handler())

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	s.srv = &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      mux,
		TLSConfig:    tlsConfig,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	go s.Hub.HeartbeatCheck(ctx)

	slog.Info("gateway server starting",
		"addr", s.cfg.ListenAddr,
		"tls_min_version", "1.3",
	)

	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("gateway listen failed: %w", err)
	}

	if s.cfg.CertFile != "" && s.cfg.KeyFile != "" {
		go func() {
			if err := s.srv.ServeTLS(ln, s.cfg.CertFile, s.cfg.KeyFile); err != nil && err != http.ErrServerClosed {
				slog.Error("gateway serve error", "error", err)
			}
		}()
	} else {
		go func() {
			if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				slog.Error("gateway serve error", "error", err)
			}
		}()
	}

	<-ctx.Done()
	return s.Shutdown(context.Background())
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("gateway server shutting down")

	s.Hub.Shutdown()

	if s.srv != nil {
		if err := s.srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("gateway shutdown failed: %w", err)
		}
	}

	slog.Info("gateway server stopped")
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","agents":` + fmt.Sprintf("%d", s.Hub.AgentCount()) + `}`))
}

// ProxyConnection proxies a raw TCP connection to an agent tunnel.
// This is a Phase 2 feature — currently, the Relay handles direct data connections.
func (s *Server) ProxyConnection(ctx context.Context, clientConn net.Conn, tunnelID string) error {
	return fmt.Errorf("proxy connection: use Relay data channel instead (not implemented in Phase 1)")
}

// HandleWebSocket handles an external WebSocket connection for tunneling.
// This is a Phase 2 feature — currently, the Relay handles WebSocket upgrade.
func (s *Server) HandleWebSocket(ctx context.Context, conn net.Conn) error {
	return fmt.Errorf("handle websocket: use Relay data channel instead (not implemented in Phase 1)")
}

// GetSession returns session info for a given session ID.
// This is a Phase 2 feature for session recording and replay.
func (s *Server) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return nil, fmt.Errorf("get session: session recording not implemented in Phase 1")
}

func (s *Server) Stop(ctx context.Context) error {
	return s.Shutdown(ctx)
}
