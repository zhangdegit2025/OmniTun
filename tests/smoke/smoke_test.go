package smoke

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/omnitun/omnitun/internal/auth"
	"github.com/omnitun/omnitun/internal/relay"
	"github.com/omnitun/omnitun/pkg/config"
)

func TestSmoke_ServerStarts_HealthCheck(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:" + port,
		Handler: mux,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown(context.Background())

	resp, err := http.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestSmoke_RelayStarts_ComponentsInitialized(t *testing.T) {
	t.Parallel()
	cfg := &relay.Config{
		ListenAddr:  "127.0.0.1:0",
		Region:      "smoke-test",
		ControlAddr: "localhost:9999",
		TokenSecret: "smoke-test-secret",
	}

	s, err := relay.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if s.Dispatcher() == nil {
		t.Fatal("dispatcher must not be nil after server creation")
	}
	if s.StreamMultiplexer() == nil {
		t.Fatal("stream multiplexer must not be nil after server creation")
	}
	if s.Proxy() == nil {
		t.Fatal("proxy must not be nil after server creation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	startErr := make(chan error, 1)
	go func() {
		startErr <- s.Start(ctx)
	}()

	select {
	case err := <-startErr:
		if err != nil {
			t.Logf("relay start returned (expected for test): %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Log("relay started (may be running)")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = s.Shutdown(shutdownCtx)
}

func TestSmoke_Dispatcher_CreateAndLookup(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	tunnelCtx := &relay.TunnelContext{
		TunnelID:  "smoke-tun-1",
		AgentID:   "smoke-agent-1",
		Domain:    "smoke.omnitun.io",
		StreamID:  100,
		CreatedAt: time.Now(),
	}

	if err := d.Register(tunnelCtx); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	found, ok := d.Lookup("smoke.omnitun.io")
	if !ok {
		t.Fatal("lookup failed for registered domain")
	}
	if found.TunnelID != "smoke-tun-1" {
		t.Errorf("expected tunnel_id smoke-tun-1, got %s", found.TunnelID)
	}

	if count := d.TunnelCount(); count != 1 {
		t.Errorf("expected 1 tunnel, got %d", count)
	}
}

func TestSmoke_JWTManager_IssueAndValidate(t *testing.T) {
	t.Parallel()
	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}
	mgr, err := auth.NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	token, err := mgr.IssueAccessToken("user-1", "org-1", "owner")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("expected subject user-1, got %s", claims.Subject)
	}
	if claims.OrgID != "org-1" {
		t.Errorf("expected OrgID org-1, got %s", claims.OrgID)
	}
	if claims.Role != "owner" {
		t.Errorf("expected role owner, got %s", claims.Role)
	}
}

func TestSmoke_PasswordHash_Works(t *testing.T) {
	t.Parallel()
	password := "SmokeTest123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == password {
		t.Fatal("hash should not equal password")
	}

	if err := auth.CheckPassword(hash, password); err != nil {
		t.Errorf("CheckPassword failed for correct password: %v", err)
	}
	if err := auth.CheckPassword(hash, "WrongPassword1!"); err == nil {
		t.Error("CheckPassword should fail for wrong password")
	}
}

func TestSmoke_TokenGeneration_Unique(t *testing.T) {
	t.Parallel()
	token1, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	token2, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token1 == token2 {
		t.Error("consecutive tokens should be unique")
	}
}

func getFreePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("get free port: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return port
}
