package relay

import (
	"sync"
	"testing"
	"time"
)

func TestDispatcherLookup(t *testing.T) {
	d := NewDispatcher()

	ctx := &TunnelContext{
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
		t.Fatal("Lookup failed for exact domain")
	}
	if found.TunnelID != "tun-001" {
		t.Fatalf("expected tunnel_id tun-001, got %s", found.TunnelID)
	}
	if found.AgentID != "agent-001" {
		t.Fatalf("expected agent_id agent-001, got %s", found.AgentID)
	}

	_, ok = d.Lookup("nonexistent.omnitun.io")
	if ok {
		t.Fatal("Lookup should fail for unknown domain")
	}
}

func TestDispatcherLookupWildcard(t *testing.T) {
	d := NewDispatcher()

	ctx := &TunnelContext{
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
		t.Fatal("Lookup failed for wildcard domain foo.omnitun.io")
	}
	if found.TunnelID != "tun-wild" {
		t.Fatalf("expected tunnel_id tun-wild, got %s", found.TunnelID)
	}

	found, ok = d.Lookup("bar.baz.omnitun.io")
	if !ok {
		t.Fatal("Lookup failed for wildcard domain bar.baz.omnitun.io")
	}
	if found.TunnelID != "tun-wild" {
		t.Fatalf("expected tunnel_id tun-wild, got %s", found.TunnelID)
	}
}

func TestDispatcherLookupByPort(t *testing.T) {
	d := NewDispatcher()

	ctx := &TunnelContext{
		TunnelID:  "tun-tcp",
		AgentID:   "agent-tcp",
		Port:      3306,
		StreamID:  7,
		CreatedAt: time.Now(),
	}

	if err := d.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	found, ok := d.LookupByPort(3306)
	if !ok {
		t.Fatal("LookupByPort failed for port 3306")
	}
	if found.TunnelID != "tun-tcp" {
		t.Fatalf("expected tunnel_id tun-tcp, got %s", found.TunnelID)
	}

	_, ok = d.LookupByPort(8080)
	if ok {
		t.Fatal("LookupByPort should fail for unregistered port")
	}
}

func TestDispatcherUnregister(t *testing.T) {
	d := NewDispatcher()

	ctx := &TunnelContext{
		TunnelID:  "tun-del",
		AgentID:   "agent-del",
		Domain:    "delete.omnitun.io",
		Port:      5432,
		StreamID:  3,
		CreatedAt: time.Now(),
	}

	if err := d.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	d.Unregister("tun-del")

	_, ok := d.Lookup("delete.omnitun.io")
	if ok {
		t.Fatal("Lookup should fail after Unregister")
	}

	_, ok = d.LookupByPort(5432)
	if ok {
		t.Fatal("LookupByPort should fail after Unregister")
	}
}

func TestDispatcherRegisterEmptyID(t *testing.T) {
	d := NewDispatcher()

	ctx := &TunnelContext{
		TunnelID: "",
		AgentID:  "agent-empty",
	}

	err := d.Register(ctx)
	if err == nil {
		t.Fatal("Register should fail with empty TunnelID")
	}
	if err != ErrEmptyTunnelID {
		t.Fatalf("expected ErrEmptyTunnelID, got %v", err)
	}
}

func TestDispatcherTunnelCount(t *testing.T) {
	d := NewDispatcher()

	if count := d.TunnelCount(); count != 0 {
		t.Fatalf("expected 0 tunnels, got %d", count)
	}

	for i := 0; i < 5; i++ {
		ctx := &TunnelContext{
			TunnelID:  "tun-" + string(rune('a'+i)),
			AgentID:   "agent",
			Domain:    "",
			Port:      0,
			StreamID:  uint64(i + 1),
			CreatedAt: time.Now(),
		}
		if err := d.Register(ctx); err != nil {
			t.Fatalf("Register %d failed: %v", i, err)
		}
	}

	if count := d.TunnelCount(); count != 5 {
		t.Fatalf("expected 5 tunnels, got %d", count)
	}
}

func TestDispatcherConcurrentAccess(t *testing.T) {
	d := NewDispatcher()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tunnelID := "tun-" + string(rune('0'+idx%10))
			ctx := &TunnelContext{
				TunnelID:  tunnelID,
				AgentID:   "agent",
				Domain:    "",
				Port:      0,
				StreamID:  uint64(idx),
				CreatedAt: time.Now(),
			}
			d.Register(ctx)
			d.Lookup("nonexistent")
		}(i)
	}

	wg.Wait()

	count := d.TunnelCount()
	if count < 1 || count > 10 {
		t.Fatalf("expected 1-10 tunnels, got %d", count)
	}
}

func TestStreamMultiplexerNewStream(t *testing.T) {
	sm := NewStreamMultiplexer()

	r, w := dummyPipe()

	sc := sm.NewStream("tun-001", w)
	if sc.StreamID != 1 {
		t.Fatalf("expected stream ID 1, got %d", sc.StreamID)
	}
	if sc.TunnelID != "tun-001" {
		t.Fatalf("expected tunnel ID tun-001, got %s", sc.TunnelID)
	}

	if count := sm.StreamCount(); count != 1 {
		t.Fatalf("expected 1 stream, got %d", count)
	}

	sc2 := sm.NewStream("tun-002", r)
	if sc2.StreamID != 2 {
		t.Fatalf("expected stream ID 2, got %d", sc2.StreamID)
	}

	if count := sm.StreamCount(); count != 2 {
		t.Fatalf("expected 2 streams, got %d", count)
	}

	_, ok := sm.GetStream(1)
	if !ok {
		t.Fatal("should find stream 1")
	}

	sm.CloseStream(1)
	if count := sm.StreamCount(); count != 1 {
		t.Fatalf("expected 1 stream after close, got %d", count)
	}

	_, ok = sm.GetStream(1)
	if ok {
		t.Fatal("should not find closed stream 1")
	}
}

func TestStreamMultiplexerNotFound(t *testing.T) {
	sm := NewStreamMultiplexer()

	err := sm.Forward(999, []byte("test"))
	if err == nil {
		t.Fatal("Forward should fail for unknown stream")
	}

	_, err = sm.Receive(999)
	if err == nil {
		t.Fatal("Receive should fail for unknown stream")
	}
}

func TestNewServer(t *testing.T) {
	cfg := &Config{
		ListenAddr:  "127.0.0.1:0",
		Region:      "test-us",
		ControlAddr: "localhost:9002",
		TokenSecret: "test-secret",
	}
	s, err := NewServer(cfg)
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

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		Region:      "us-east-1",
		ControlAddr: "control.omnitun.io:9002",
		TokenSecret: "test-secret",
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.cfg.ListenAddr != "0.0.0.0:443" {
		t.Fatalf("expected default ListenAddr 0.0.0.0:443, got %s", s.cfg.ListenAddr)
	}
}

func TestNewServerNilConfig(t *testing.T) {
	_, err := NewServer(nil)
	if err == nil {
		t.Fatal("NewServer should fail with nil config")
	}
}

func TestGenerateRelayID(t *testing.T) {
	id, err := generateRelayID()
	if err != nil {
		t.Fatalf("generateRelayID failed: %v", err)
	}
	if len(id) == 0 {
		t.Fatal("relay ID must not be empty")
	}
	if id[:6] != "relay-" {
		t.Fatalf("expected relay ID prefix relay-, got %s", id)
	}
}

func TestControlClient(t *testing.T) {
	cc, err := NewControlClient("localhost:9999", "test", "127.0.0.1:443")
	if err != nil {
		t.Fatalf("NewControlClient failed: %v", err)
	}
	defer cc.Close()

	if cc.RelayID() == "" {
		t.Fatal("relay ID must not be empty")
	}

	if cc.RelayID()[:6] != "relay-" {
		t.Fatalf("expected relay ID prefix relay-, got %s", cc.RelayID())
	}
}

func TestCompressDecompress(t *testing.T) {
	original := []byte("hello omnitun relay data plane test payload")

	compressed, err := CompressPayload(original)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Fatal("compressed payload must not be empty")
	}

	decompressed, err := DecompressPayload(compressed)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	if string(decompressed) != string(original) {
		t.Fatalf("decompressed mismatch: got %q, want %q", string(decompressed), string(original))
	}
}

func TestCompressEmptyPayload(t *testing.T) {
	compressed, err := CompressPayload(nil)
	if err != nil {
		t.Fatalf("compress nil failed: %v", err)
	}
	if len(compressed) != 0 {
		t.Fatalf("compressed nil should be empty, got %d bytes", len(compressed))
	}

	decompressed, err := DecompressPayload(nil)
	if err != nil {
		t.Fatalf("decompress nil failed: %v", err)
	}
	if len(decompressed) != 0 {
		t.Fatalf("decompressed nil should be empty, got %d bytes", len(decompressed))
	}
}

type dummyReadWriteCloser struct {
	*pipeReader
	*pipeWriter
}

type pipeReader struct {
	ch chan []byte
}

type pipeWriter struct {
	ch chan []byte
}

func (r *pipeReader) Read(b []byte) (int, error) {
	data, ok := <-r.ch
	if !ok {
		return 0, nil
	}
	n := copy(b, data)
	return n, nil
}

func (w *pipeWriter) Write(b []byte) (int, error) {
	buf := make([]byte, len(b))
	copy(buf, b)
	w.ch <- buf
	return len(b), nil
}

func (dw *dummyReadWriteCloser) Close() error {
	close(dw.pipeReader.ch)
	close(dw.pipeWriter.ch)
	return nil
}

func dummyPipe() (*dummyReadWriteCloser, *dummyReadWriteCloser) {
	chA := make(chan []byte, 10)
	chB := make(chan []byte, 10)

	a := &dummyReadWriteCloser{
		pipeReader: &pipeReader{ch: chB},
		pipeWriter: &pipeWriter{ch: chA},
	}
	b := &dummyReadWriteCloser{
		pipeReader: &pipeReader{ch: chA},
		pipeWriter: &pipeWriter{ch: chB},
	}

	return a, b
}
