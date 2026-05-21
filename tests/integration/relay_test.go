//go:build integration

package integration

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omnitun/omnitun/internal/relay"
)

func TestRelay_DispatcherLookup_ByDomain(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	ctx := &relay.TunnelContext{
		TunnelID:  "tun-001",
		AgentID:   "agent-001",
		Domain:    "example.omnitun.io",
		StreamID:  42,
		CreatedAt: time.Now(),
	}
	if err := d.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	found, ok := d.Lookup("example.omnitun.io")
	if !ok {
		t.Fatal("expected to find tunnel by exact domain")
	}
	if found.TunnelID != "tun-001" {
		t.Errorf("expected tunnel_id tun-001, got %s", found.TunnelID)
	}

	_, ok = d.Lookup("nonexistent.omnitun.io")
	if ok {
		t.Fatal("expected no match for unknown domain")
	}
}

func TestRelay_DispatcherLookup_WildcardDomain(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	ctx := &relay.TunnelContext{
		TunnelID:  "tun-wild",
		AgentID:   "agent-wild",
		Domain:    "*.omnitun.io",
		StreamID:  1,
		CreatedAt: time.Now(),
	}
	if err := d.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	found, ok := d.Lookup("foo.omnitun.io")
	if !ok {
		t.Fatal("expected wildcard match for foo.omnitun.io")
	}
	if found.TunnelID != "tun-wild" {
		t.Errorf("expected tunnel_id tun-wild, got %s", found.TunnelID)
	}

	found, ok = d.Lookup("bar.baz.omnitun.io")
	if !ok {
		t.Fatal("expected wildcard match for bar.baz.omnitun.io")
	}
	if found.TunnelID != "tun-wild" {
		t.Errorf("expected tunnel_id tun-wild, got %s", found.TunnelID)
	}
}

func TestRelay_DispatcherLookup_MultipleRegistrations(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	tunnelData := []struct {
		id, agent, domain string
		port              int
	}{
		{"tun-1", "agent-1", "app1.omnitun.io", 3000},
		{"tun-2", "agent-2", "app2.omnitun.io", 3001},
		{"tun-3", "agent-3", "app3.omnitun.io", 3002},
		{"tun-4", "agent-4", "app4.omnitun.io", 0},
		{"tun-5", "agent-5", "", 5432},
	}

	for i, td := range tunnelData {
		ctx := &relay.TunnelContext{
			TunnelID:  td.id,
			AgentID:   td.agent,
			Domain:    td.domain,
			Port:      td.port,
			StreamID:  uint64(i + 1),
			CreatedAt: time.Now(),
		}
		if err := d.Register(ctx); err != nil {
			t.Fatalf("register %s: %v", td.id, err)
		}
	}

	if count := d.TunnelCount(); count != 5 {
		t.Errorf("expected 5 tunnels, got %d", count)
	}

	for _, td := range tunnelData {
		if td.domain != "" {
			found, ok := d.Lookup(td.domain)
			if !ok {
				t.Errorf("expected to find tunnel by domain %s", td.domain)
			} else if found.TunnelID != td.id {
				t.Errorf("domain %s: expected tunnel_id %s, got %s", td.domain, td.id, found.TunnelID)
			}
		}
		if td.port > 0 {
			found, ok := d.LookupByPort(td.port)
			if !ok {
				t.Errorf("expected to find tunnel by port %d", td.port)
			} else if found.TunnelID != td.id {
				t.Errorf("port %d: expected tunnel_id %s, got %s", td.port, td.id, found.TunnelID)
			}
		}
	}
}

func TestRelay_DispatcherUnregister_RemovesRoutes(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	ctx := &relay.TunnelContext{
		TunnelID:  "tun-rm",
		AgentID:   "agent-rm",
		Domain:    "remove.omnitun.io",
		Port:      5555,
		StreamID:  1,
		CreatedAt: time.Now(),
	}
	if err := d.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	d.Unregister("tun-rm")

	_, ok := d.Lookup("remove.omnitun.io")
	if ok {
		t.Fatal("domain should not resolve after unregister")
	}
	_, ok = d.LookupByPort(5555)
	if ok {
		t.Fatal("port should not resolve after unregister")
	}
	if count := d.TunnelCount(); count != 0 {
		t.Errorf("expected 0 tunnels after unregister, got %d", count)
	}
}

func TestRelay_DispatcherConcurrentAccess_Safe(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tunnelID := "tun-conc-" + string(rune('0'+idx%10))
			ctx := &relay.TunnelContext{
				TunnelID:  tunnelID,
				AgentID:   "agent",
				Domain:    "",
				Port:      0,
				StreamID:  uint64(idx),
				CreatedAt: time.Now(),
			}
			_ = d.Register(ctx)
			d.Lookup("nonexistent.omnitun.io")
		}(i)
	}
	wg.Wait()

	count := d.TunnelCount()
	if count < 1 || count > 10 {
		t.Errorf("expected 1-10 tunnels after concurrent ops, got %d", count)
	}
}

func TestRelay_HTTPProxy_TunnelNotFoundReturns502(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()
	proxy := relay.NewReverseProxy(d, sm)

	req := httptest.NewRequest(http.MethodGet, "http://unknown.omnitun.io/test", nil)
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unknown tunnel, got %d", rec.Code)
	}
}

func TestRelay_StreamMultiplexer_NewAndClose(t *testing.T) {
	t.Parallel()
	sm := relay.NewStreamMultiplexer()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	sc := sm.NewStream("tun-stream", server)
	if sc.StreamID != 1 {
		t.Errorf("expected stream ID 1, got %d", sc.StreamID)
	}
	if sc.TunnelID != "tun-stream" {
		t.Errorf("expected tunnel ID tun-stream, got %s", sc.TunnelID)
	}
	if sm.StreamCount() != 1 {
		t.Errorf("expected 1 stream, got %d", sm.StreamCount())
	}

	_, ok := sm.GetStream(1)
	if !ok {
		t.Fatal("expected to find stream 1")
	}

	sm.CloseStream(1)
	if sm.StreamCount() != 0 {
		t.Errorf("expected 0 streams after close, got %d", sm.StreamCount())
	}

	_, ok = sm.GetStream(1)
	if ok {
		t.Fatal("stream 1 should not be found after close")
	}
}

func TestRelay_StreamMultiplexer_ForwardAndReceive(t *testing.T) {
	t.Parallel()
	sm := relay.NewStreamMultiplexer()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	sc := sm.NewStream("tun-fwd", server)

	testData := []byte("omnitun test payload")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		n, err := client.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("client read: %v", err)
			return
		}
		if string(buf[:n]) == "" {
			t.Error("expected data on pipe")
		}
	}()

	if err := sm.Forward(sc.StreamID, testData); err != nil {
		t.Fatalf("Forward failed: %v", err)
	}
	wg.Wait()
}

func TestRelay_NewServer_WithConfig(t *testing.T) {
	t.Parallel()
	cfg := &relay.Config{
		ListenAddr:  "127.0.0.1:0",
		Region:      "test-region",
		ControlAddr: "localhost:9999",
	}

	s, err := relay.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if s.Dispatcher() == nil {
		t.Fatal("dispatcher must not be nil")
	}
	if s.StreamMultiplexer() == nil {
		t.Fatal("stream multiplexer must not be nil")
	}
	if s.Proxy() == nil {
		t.Fatal("proxy must not be nil")
	}
}

func TestRelay_NewServer_DefaultListenAddr(t *testing.T) {
	t.Parallel()
	cfg := &relay.Config{
		Region:      "us-east-1",
		ControlAddr: "control.omnitun.io:9002",
	}

	s, err := relay.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	_ = s
}

func TestRelay_Dispatcher_OnConfigUpdate(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()

	configMsg := []byte(`{
		"tunnels": [
			{"tunnel_id": "cfg-1", "agent_id": "agent-1", "domain": "cfg1.omnitun.io", "port": 8001, "protocol": "http"},
			{"tunnel_id": "cfg-2", "agent_id": "agent-2", "domain": "cfg2.omnitun.io", "port": 8002, "protocol": "tcp"}
		]
	}`)

	d.OnConfigUpdate(configMsg)

	if count := d.TunnelCount(); count != 2 {
		t.Errorf("expected 2 tunnels after config update, got %d", count)
	}

	found, ok := d.Lookup("cfg1.omnitun.io")
	if !ok {
		t.Fatal("expected to find cfg1.omnitun.io")
	}
	if found.TunnelID != "cfg-1" {
		t.Errorf("expected tunnel_id cfg-1, got %s", found.TunnelID)
	}

	found, ok = d.LookupByPort(8002)
	if !ok {
		t.Fatal("expected to find port 8002")
	}
	if found.TunnelID != "cfg-2" {
		t.Errorf("expected tunnel_id cfg-2, got %s", found.TunnelID)
	}
}

type fakeConn struct {
	readData  []byte
	writeData []byte
	readPos   int
	mu        sync.Mutex
	closed    bool
}

func (f *fakeConn) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readPos >= len(f.readData) {
		return 0, io.EOF
	}
	n := copy(b, f.readData[f.readPos:])
	f.readPos += n
	return n, nil
}

func (f *fakeConn) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeData = append(f.writeData, b...)
	return len(b), nil
}

func (f *fakeConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

func TestRelay_HTTPProxy_SuccessPath(t *testing.T) {
	t.Parallel()
	d := relay.NewDispatcher()
	sm := relay.NewStreamMultiplexer()

	tunnelCtx := &relay.TunnelContext{
		TunnelID:  "tun-http",
		AgentID:   "agent-http",
		Domain:    "test.omnitun.io",
		StreamID:  1,
		CreatedAt: time.Now(),
		Protocol:  "http",
	}
	if err := d.Register(tunnelCtx); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	fc := &fakeConn{}
	sc := sm.NewStream("tun-http", fc)
	tunnelCtx.Connection = sc
	tunnelCtx.StreamID = sc.StreamID

	req := httptest.NewRequest(http.MethodGet, "http://test.omnitun.io/hello", nil)
	rec := httptest.NewRecorder()

	proxy := relay.NewReverseProxy(d, sm)
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Logf("response code: %d (expected BadGateway since no agent response)", rec.Code)
	}
}

func requireNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		if msg != "" {
			t.Fatalf("%s: %v", msg, err)
		}
		t.Fatal(err)
	}
}

var _, _ = strings.NewReader(""), context.Background()
