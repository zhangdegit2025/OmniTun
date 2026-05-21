//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omnitun/omnitun/internal/protocol"
	"github.com/omnitun/omnitun/internal/relay"
)

type agentProxy struct {
	localAddr string
	agentConn net.Conn
	mu        sync.Mutex
	closed    bool
}

func (ap *agentProxy) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		frame, err := protocol.DecodeFrame(ap.agentConn)
		if err != nil {
			return
		}

		payload := frame.Payload
		if frame.IsCompressed() && len(payload) > 0 {
			payload, err = relay.DecompressPayload(payload)
			if err != nil {
				return
			}
		}

		localConn, err := net.DialTimeout("tcp", ap.localAddr, 5*time.Second)
		if err != nil {
			return
		}

		if len(payload) > 0 {
			if _, err := localConn.Write(payload); err != nil {
				localConn.Close()
				return
			}
		}

		if tcpConn, ok := localConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}

		respBuf := make([]byte, 0, 64*1024)
		tmp := make([]byte, 32*1024)
		deadline := time.Now().Add(500 * time.Millisecond)
		for {
			localConn.SetReadDeadline(deadline)
			n, err := localConn.Read(tmp)
			if n > 0 {
				respBuf = append(respBuf, tmp[:n]...)
				deadline = time.Now().Add(100 * time.Millisecond)
			}
			if err != nil {
				break
			}
		}
		localConn.Close()

		if len(respBuf) > 0 {
			compressed, err := relay.CompressPayload(respBuf)
			if err != nil {
				return
			}

			respFrame := protocol.NewDataFrame(frame.StreamID, compressed, true)
			if err := protocol.EncodeFrame(ap.agentConn, respFrame); err != nil {
				return
			}
		}
	}
}

func startLocalHTTPTestServer(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "omnitun-e2e")
		w.Header().Set("X-Request-Method", r.Method)
		w.Header().Set("X-Request-Path", r.URL.Path)
		w.Header().Set("X-Request-Host", r.Host)
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Request-Body", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from OmniTun E2E test"))
	})

	server := &http.Server{Handler: handler}
	go func() {
		server.Serve(listener)
	}()

	addr := listener.Addr().(*net.TCPAddr)
	localAddr := fmt.Sprintf("127.0.0.1:%d", addr.Port)

	cleanup := func() {
		server.Close()
		listener.Close()
	}
	return localAddr, cleanup
}

func setupE2EDispatcherAndAgent(t *testing.T, dispatcher *relay.Dispatcher, sm *relay.StreamMultiplexer, tunnelID, slug, domain, localAddr string) (*relay.TunnelContext, context.CancelFunc) {
	t.Helper()

	agentConn, relayConn := net.Pipe()

	sc := sm.NewStream(tunnelID, relayConn)

	tunnelCtx := &relay.TunnelContext{
		TunnelID:   tunnelID,
		AgentID:    "agent-e2e",
		Domain:     domain,
		StreamID:   sc.StreamID,
		Slug:       slug,
		Protocol:   "http",
		Connection: sc,
		CreatedAt:  time.Now(),
	}

	if err := dispatcher.Register(tunnelCtx); err != nil {
		t.Fatalf("register tunnel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ap := &agentProxy{
		localAddr: localAddr,
		agentConn: agentConn,
	}
	go func() {
		ap.run(ctx)
	}()

	return tunnelCtx, func() {
		cancel()
		agentConn.Close()
		relayConn.Close()
		dispatcher.Unregister(tunnelID)
		sm.CloseStream(sc.StreamID)
	}
}

func TestE2E_HTTPTunnel_FullFlow(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-fullflow"
	slug := "test-slug-" + strings.ReplaceAll(tunnelID, "tun-e2e-", "")
	domain := "e2e-fullflow.omnitun.local"

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	t.Run("route_by_domain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://"+domain+"/api/test", nil)
		req.Host = domain
		rec := httptest.NewRecorder()

		proxy.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "Hello from OmniTun E2E test" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
		if rec.Header().Get("X-Test") != "omnitun-e2e" {
			t.Errorf("expected X-Test header 'omnitun-e2e', got '%s'", rec.Header().Get("X-Test"))
		}
	})

	t.Run("route_by_slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/"+slug+"/some/path", nil)
		rec := httptest.NewRecorder()

		proxy.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "Hello from OmniTun E2E test" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
		if rec.Header().Get("X-Test") != "omnitun-e2e" {
			t.Errorf("expected X-Test header 'omnitun-e2e', got '%s'", rec.Header().Get("X-Test"))
		}
	})

	t.Run("verify_request_headers_forwarded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://"+domain+"/headers-test", strings.NewReader("test-body"))
		req.Host = domain
		rec := httptest.NewRecorder()

		proxy.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-Request-Method") != "POST" {
			t.Errorf("expected X-Request-Method 'POST', got '%s'", rec.Header().Get("X-Request-Method"))
		}
		if rec.Header().Get("X-Request-Path") != "/headers-test" {
			t.Errorf("expected X-Request-Path '/headers-test', got '%s'", rec.Header().Get("X-Request-Path"))
		}
		if rec.Header().Get("X-Request-Body") != "test-body" {
			t.Errorf("expected X-Request-Body 'test-body', got '%s'", rec.Header().Get("X-Request-Body"))
		}
	})
}

func TestE2E_WebSocketTunnel(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-ws"
	slug := "test-slug-ws"
	domain := "e2e-ws.omnitun.local"

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	t.Run("websocket_upgrade_routed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://"+domain+"/ws", nil)
		req.Host = domain
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		req.Header.Set("Sec-WebSocket-Version", "13")
		rec := httptest.NewRecorder()

		proxy.ServeHTTP(rec, req)

		if rec.Code != http.StatusSwitchingProtocols && rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 101 or 500 (httptest does not support hijack), got %d", rec.Code)
		}
	})

	t.Run("websocket_unknown_tunnel_returns_502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://unknown-ws.omnitun.local/ws", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		req.Header.Set("Sec-WebSocket-Version", "13")
		rec := httptest.NewRecorder()

		proxy.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Errorf("expected 502 for unknown tunnel, got %d", rec.Code)
		}
	})
}

func TestE2E_MultipleConcurrentTunnels(t *testing.T) {
	t.Parallel()

	numTunnels := 5
	localAddrs := make([]string, numTunnels)
	localCleanups := make([]func(), numTunnels)

	for i := 0; i < numTunnels; i++ {
		localAddrs[i], localCleanups[i] = startLocalHTTPTestServer(t)
	}
	defer func() {
		for _, cleanup := range localCleanups {
			cleanup()
		}
	}()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	type tunnelSetup struct {
		tunnelID string
		domain   string
		slug     string
		cleanup  func()
	}

	contexts := make([]tunnelSetup, numTunnels)
	for i := 0; i < numTunnels; i++ {
		ts := tunnelSetup{
			tunnelID: fmt.Sprintf("tun-e2e-%d", i),
			domain:   fmt.Sprintf("e2e-%d.omnitun.local", i),
			slug:     fmt.Sprintf("slug-e2e-%d", i),
		}
		_, ts.cleanup = setupE2EDispatcherAndAgent(t, dispatcher, sm, ts.tunnelID, ts.slug, ts.domain, localAddrs[i])
		contexts[i] = ts
	}
	defer func() {
		for _, ts := range contexts {
			ts.cleanup()
		}
	}()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	if count := dispatcher.TunnelCount(); count != numTunnels {
		t.Errorf("expected %d tunnels, got %d", numTunnels, count)
	}

	t.Run("all_tunnels_respond_independently", func(t *testing.T) {
		var wg sync.WaitGroup
		errCh := make(chan error, numTunnels)

		for i := 0; i < numTunnels; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				domain := contexts[idx].domain
				req := httptest.NewRequest(http.MethodGet, "http://"+domain+"/health", nil)
				req.Host = domain
				rec := httptest.NewRecorder()
				proxy.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					errCh <- fmt.Errorf("tunnel %d: expected 200, got %d", idx, rec.Code)
					return
				}
				if rec.Body.String() != "Hello from OmniTun E2E test" {
					errCh <- fmt.Errorf("tunnel %d: unexpected body", idx)
					return
				}
			}(i)
		}
		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Error(err)
		}
	})

	t.Run("route_by_slug_works_for_all", func(t *testing.T) {
		for i := 0; i < numTunnels; i++ {
			slug := contexts[i].slug
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:0/"+slug+"/path", nil)
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("tunnel %d via slug: expected 200, got %d", i, rec.Code)
			}
		}
	})
}

func TestE2E_AgentReconnectAfterFailure(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-reconnect"
	slug := "test-slug-reconnect"
	domain := "e2e-reconnect.omnitun.local"

	proxy := relay.NewReverseProxy(dispatcher, sm)

	sendRequest := func(expectedOK bool) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "http://"+domain+"/test", nil)
		req.Host = domain
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if expectedOK && rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !expectedOK && rec.Code == http.StatusOK {
			t.Errorf("expected failure, got 200")
		}
	}

	connectAgent := func() func() {
		_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
		return cleanup
	}

	cleanup1 := connectAgent()
	sendRequest(true)

	cleanup1()
	dispatcher.Unregister(tunnelID)

	time.Sleep(50 * time.Millisecond)

	sendRequest(false)

	cleanup2 := connectAgent()
	defer cleanup2()

	time.Sleep(100 * time.Millisecond)

	sendRequest(true)

	sendRequest(true)
}

func TestE2E_SlugRouting_DomainFallback(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-fallback"
	slug := "test-slug-fallback"
	domain := "e2e-fallback.omnitun.local"

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	t.Run("domain_match_has_priority", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://"+domain+"/some-path", nil)
		req.Host = domain
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("domain match failed: %d", rec.Code)
		}
	})

	t.Run("slug_match_when_domain_unknown", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://unknown-host.local/"+slug+"/some-path", nil)
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("slug fallback failed: %d", rec.Code)
		}
		if rec.Body.String() != "Hello from OmniTun E2E test" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("unknown_slug_returns_502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/nonexistent-slug/path", nil)
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Errorf("expected 502 for unknown slug, got %d", rec.Code)
		}
	})

	t.Run("root_path_no_slug_returns_502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Errorf("expected 502 for root path, got %d", rec.Code)
		}
	})
}

func TestE2E_EmptyHost_UsesSlugRouting(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-emptyhost"
	slug := "test-slug-emptyhost"
	domain := ""

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:1234/"+slug+"/deep/path", nil)
	req.Host = ""
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "Hello from OmniTun E2E test" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestE2E_LargeRequestBody(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-largebody"
	slug := "test-slug-large"
	domain := "e2e-large.omnitun.local"

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	largeBody := strings.Repeat("ABCDEFGHIJ", 4096)

	req := httptest.NewRequest(http.MethodPost, "http://"+domain+"/upload", bytes.NewReader([]byte(largeBody)))
	req.Host = domain
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-Method") != "POST" {
		t.Errorf("expected POST, got %s", rec.Header().Get("X-Request-Method"))
	}
	if len(rec.Header().Get("X-Request-Body")) != len(largeBody) {
		t.Errorf("expected body length %d, got %d", len(largeBody), len(rec.Header().Get("X-Request-Body")))
	}
}

func TestE2E_ContentTypePreservation(t *testing.T) {
	t.Parallel()

	localAddr, localCleanup := startLocalHTTPTestServer(t)
	defer localCleanup()

	dispatcher := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelID := "tun-e2e-ctype"
	slug := "test-slug-ctype"
	domain := "e2e-ctype.omnitun.local"

	_, cleanup := setupE2EDispatcherAndAgent(t, dispatcher, sm, tunnelID, slug, domain, localAddr)
	defer cleanup()

	proxy := relay.NewReverseProxy(dispatcher, sm)

	jsonBody := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "http://"+domain+"/api/json", bytes.NewReader([]byte(jsonBody)))
	req.Host = domain
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Test") != "omnitun-e2e" {
		t.Errorf("expected X-Test 'omnitun-e2e', got '%s'", rec.Header().Get("X-Test"))
	}
}
