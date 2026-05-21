package network

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestSTUNClient_BuildBindingRequest(t *testing.T) {
	msg := buildBindingRequest()

	if len(msg) != 20 {
		t.Fatalf("binding request length = %d, want 20", len(msg))
	}

	msgType := binary.BigEndian.Uint16(msg[0:2])
	if msgType != StunBindingRequest {
		t.Errorf("message type = 0x%04x, want 0x%04x", msgType, StunBindingRequest)
	}

	msgLen := binary.BigEndian.Uint16(msg[2:4])
	if msgLen != 0 {
		t.Errorf("message length = %d, want 0 (no attributes)", msgLen)
	}

	magic := binary.BigEndian.Uint32(msg[4:8])
	if magic != StunMagicCookie {
		t.Errorf("magic cookie = 0x%08x, want 0x%08x", magic, StunMagicCookie)
	}

	var zeroTxID [12]byte
	if string(msg[8:20]) == string(zeroTxID[:]) {
		t.Error("transaction ID should not be all zeros")
	}
}

func TestNATType_String(t *testing.T) {
	tests := []struct {
		natType NATType
		want    string
	}{
		{NATUnknown, "Unknown"},
		{NATOpen, "Open Internet"},
		{NATFullCone, "Full Cone NAT"},
		{NATRestricted, "Restricted Cone NAT"},
		{NATPortRestricted, "Port Restricted NAT"},
		{NATSymmetric, "Symmetric NAT"},
	}

	for _, tt := range tests {
		got := tt.natType.String()
		if got != tt.want {
			t.Errorf("NATType(%d).String() = %q, want %q", tt.natType, got, tt.want)
		}
	}
}

func TestSTUNServer_BindingResponse(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	if addr == nil {
		t.Fatal("server listen addr is nil")
	}

	client := NewSTUNClient(addr.String())
	mappedAddr, err := client.GetMappedAddress()
	if err != nil {
		t.Fatalf("GetMappedAddress failed: %v", err)
	}

	host, _, err := net.SplitHostPort(mappedAddr)
	if err != nil {
		t.Fatalf("invalid mapped address %q: %v", mappedAddr, err)
	}

	if host != "127.0.0.1" {
		t.Errorf("expected mapped address 127.0.0.1, got %s", host)
	}
}

func TestSTUNServer_InvalidRequests(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()

	conn, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatalf("dial STUN server: %v", err)
	}
	defer conn.Close()

	shortMsg := []byte{0x00, 0x01}
	if _, err := conn.Write(shortMsg); err != nil {
		t.Fatalf("write short message: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	resp := make([]byte, 2048)
	_, err = conn.Read(resp)
	if err == nil {
		t.Error("expected timeout or no response for short message, but got data")
	}
}

func TestSTUNServer_MultipleClients(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	results := make(chan string, 5)

	for i := 0; i < 5; i++ {
		go func() {
			client := NewSTUNClient(addr.String())
			mapped, err := client.GetMappedAddress()
			if err != nil {
				t.Errorf("GetMappedAddress failed: %v", err)
				results <- ""
				return
			}
			results <- mapped
		}()
	}

	for i := 0; i < 5; i++ {
		mapped := <-results
		if mapped == "" {
			continue
		}
		host, _, err := net.SplitHostPort(mapped)
		if err != nil {
			t.Errorf("invalid mapped address: %v", err)
			continue
		}
		if host != "127.0.0.1" {
			t.Errorf("expected 127.0.0.1, got %s", host)
		}
	}
}

func TestSTUNClient_MappedAddressFormat(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	client := NewSTUNClient(addr.String())

	mapped, err := client.GetMappedAddress()
	if err != nil {
		t.Fatalf("GetMappedAddress failed: %v", err)
	}

	_, _, err = net.SplitHostPort(mapped)
	if err != nil {
		t.Errorf("mapped address %q is not valid host:port: %v", mapped, err)
	}
}

func TestTURNRelay_CreateAllocation(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, err := relay.CreateAllocation("user1")
	if err != nil {
		t.Fatalf("CreateAllocation failed: %v", err)
	}

	if alloc.ID == "" {
		t.Error("allocation ID should not be empty")
	}

	if alloc.Username != "user1" {
		t.Errorf("allocation username = %q, want %q", alloc.Username, "user1")
	}

	if alloc.CreatedAt.IsZero() {
		t.Error("allocation CreatedAt should be set")
	}

	count := relay.AllocationCount()
	if count != 1 {
		t.Errorf("allocation count = %d, want 1", count)
	}

	_, err = relay.CreateAllocation("user1")
	if err != ErrAllocationExists {
		t.Errorf("duplicate allocation error = %v, want ErrAllocationExists", err)
	}
}

func TestTURNRelay_MultipleAllocations(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc1, err := relay.CreateAllocation("user-a")
	if err != nil {
		t.Fatalf("CreateAllocation user-a failed: %v", err)
	}

	alloc2, err := relay.CreateAllocation("user-b")
	if err != nil {
		t.Fatalf("CreateAllocation user-b failed: %v", err)
	}

	if alloc1.ID == alloc2.ID {
		t.Error("allocation IDs should be unique")
	}

	if relay.AllocationCount() != 2 {
		t.Errorf("allocation count = %d, want 2", relay.AllocationCount())
	}
}

func TestTURNRelay_RemoveAllocation(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, _ := relay.CreateAllocation("user1")

	if err := relay.RemoveAllocation(alloc.ID); err != nil {
		t.Fatalf("RemoveAllocation failed: %v", err)
	}

	if relay.AllocationCount() != 0 {
		t.Errorf("allocation count = %d, want 0", relay.AllocationCount())
	}

	if err := relay.RemoveAllocation(alloc.ID); err != ErrAllocationNotFound {
		t.Errorf("removing nonexistent allocation error = %v, want ErrAllocationNotFound", err)
	}
}

func TestTURNRelay_RegisterPeer(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, _ := relay.CreateAllocation("user1")

	peerAddr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345}
	peerKey, err := relay.RegisterPeer(alloc.ID, peerAddr)
	if err != nil {
		t.Fatalf("RegisterPeer failed: %v", err)
	}

	fetched, err := relay.GetAllocation(alloc.ID)
	if err != nil {
		t.Fatalf("GetAllocation failed: %v", err)
	}

	fetched.mu.RLock()
	_, ok := fetched.PeerAddrs[peerKey]
	fetched.mu.RUnlock()

	if !ok {
		t.Errorf("peer %s not found in allocation", peerKey)
	}
}

func TestTURNRelay_GetAllocation_NotFound(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	_, err := relay.GetAllocation("nonexistent")
	if err != ErrAllocationNotFound {
		t.Errorf("GetAllocation error = %v, want ErrAllocationNotFound", err)
	}
}

func TestTURNRelay_SendTo(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, _ := relay.CreateAllocation("user1")
	peerAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: relay.ListenAddr().(*net.UDPAddr).Port + 1}

	peerConn, err := net.ListenUDP("udp", peerAddr)
	if err != nil {
		t.Fatalf("listen peer: %v", err)
	}
	defer peerConn.Close()

	testData := []byte("hello peer")
	if err := relay.SendTo(alloc.ID, testData, peerAddr); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	if err := peerConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Fatalf("set peer read deadline: %v", err)
	}

	buf := make([]byte, 2048)
	n, _, err := peerConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("peer read failed: %v", err)
	}

	received := buf[:n]
	if len(received) < 2 {
		t.Fatalf("received data too short: %d bytes", len(received))
	}

	payload := received[2:]
	if string(payload) != string(testData) {
		t.Errorf("received payload = %q, want %q", string(payload), string(testData))
	}

	fetched, _ := relay.GetAllocation(alloc.ID)
	fetched.mu.RLock()
	bytesSent := fetched.BytesSent
	fetched.mu.RUnlock()

	if bytesSent == 0 {
		t.Error("BytesSent should be > 0 after SendTo")
	}
}

func TestTURNRelay_AllocationExpiration(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")
	relay.allocTTL = 100 * time.Millisecond
	relay.cleanupInterval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}
	defer relay.Stop()

	alloc, _ := relay.CreateAllocation("ephemeral")
	_ = alloc

	time.Sleep(300 * time.Millisecond)

	if relay.AllocationCount() != 0 {
		t.Errorf("allocation count = %d, want 0 (should be expired)", relay.AllocationCount())
	}
}

func TestTURNRelay_Stop(t *testing.T) {
	relay := NewTURNRelay("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Start(ctx); err != nil {
		t.Fatalf("failed to start TURN relay: %v", err)
	}

	if err := relay.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if err := relay.Stop(); err != ErrTURNStopped {
		t.Errorf("second Stop error = %v, want ErrTURNStopped", err)
	}

	_, err := relay.CreateAllocation("user1")
	if err != ErrTURNStopped {
		t.Errorf("CreateAllocation after stop error = %v, want ErrTURNStopped", err)
	}
}

func TestSTUNClient_GetMappedAddress(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	client := NewSTUNClient(addr.String())

	mapped, err := client.GetMappedAddress()
	if err != nil {
		t.Fatalf("GetMappedAddress failed: %v", err)
	}

	host, _, err := net.SplitHostPort(mapped)
	if err != nil {
		t.Fatalf("invalid mapped address format: %v", err)
	}

	if host != "127.0.0.1" {
		t.Errorf("expected loopback address, got %s", host)
	}
}

func TestSTUNClient_Timeout(t *testing.T) {
	client := NewSTUNClient("127.0.0.1:19999")
	client.timeout = 500 * time.Millisecond

	_, err := client.GetMappedAddress()
	if err == nil {
		t.Error("expected timeout error for unreachable server")
	}
}

func TestNewSTUNClient(t *testing.T) {
	client := NewSTUNClient("stun.example.com:3478")
	if client == nil {
		t.Fatal("NewSTUNClient returned nil")
	}
	if client.serverAddr != "stun.example.com:3478" {
		t.Errorf("serverAddr = %q, want %q", client.serverAddr, "stun.example.com:3478")
	}
	if client.timeout != 3*time.Second {
		t.Errorf("timeout = %v, want 3s", client.timeout)
	}
}

func TestNewTURNRelay(t *testing.T) {
	relay := NewTURNRelay("0.0.0.0:3478")
	if relay == nil {
		t.Fatal("NewTURNRelay returned nil")
	}
	if relay.addr != "0.0.0.0:3478" {
		t.Errorf("addr = %q, want %q", relay.addr, "0.0.0.0:3478")
	}
	if relay.allocTTL != 5*time.Minute {
		t.Errorf("allocTTL = %v, want 5m", relay.allocTTL)
	}
}

func TestNewSTUNServer(t *testing.T) {
	server := NewSTUNServer("0.0.0.0:3478")
	if server == nil {
		t.Fatal("NewSTUNServer returned nil")
	}
	if server.addr != "0.0.0.0:3478" {
		t.Errorf("addr = %q, want %q", server.addr, "0.0.0.0:3478")
	}
}

func TestSTUNServer_Stop(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if err := server.Stop(); err != ErrSTUNServerStopped {
		t.Errorf("second Stop error = %v, want ErrSTUNServerStopped", err)
	}
}

func TestSTUNServer_String(t *testing.T) {
	server := NewSTUNServer("0.0.0.0:3478")
	got := server.String()
	want := "STUNServer(0.0.0.0:3478)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDetectNATType_ServerUnreachable(t *testing.T) {
	client := NewSTUNClient("127.0.0.1:19999")
	client.timeout = 500 * time.Millisecond

	natType, mapped, err := client.DetectNATType()
	if err == nil {
		t.Error("expected error for unreachable server")
	}
	if natType != NATSymmetric {
		t.Errorf("natType = %v, want NATSymmetric (default fallback)", natType)
	}
	if mapped != "" {
		t.Errorf("mapped = %q, want empty string", mapped)
	}
}

func TestBuildXorMappedAddrAttr_IPv4(t *testing.T) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 12345,
	}

	attr := buildXorMappedAddrAttr(addr)

	if len(attr) < 4 {
		t.Fatal("attribute too short")
	}

	attrType := binary.BigEndian.Uint16(attr[0:2])
	if attrType != StunAttrXorMappedAddress {
		t.Errorf("attr type = 0x%04x, want 0x%04x", attrType, StunAttrXorMappedAddress)
	}

	attrLen := binary.BigEndian.Uint16(attr[2:4])
	if attrLen != 8 {
		t.Errorf("attr length = %d, want 8 (IPv4)", attrLen)
	}
}

func TestBuildXorMappedAddrAttr_IPv6(t *testing.T) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("2001:db8::1"),
		Port: 12345,
	}

	attr := buildXorMappedAddrAttr(addr)

	attrLen := binary.BigEndian.Uint16(attr[2:4])
	if attrLen != 20 {
		t.Errorf("attr length = %d, want 20 (IPv6)", attrLen)
	}
}

func TestParseAddrPort(t *testing.T) {
	host, port, err := parseAddrPort("192.168.1.1:8080")
	if err != nil {
		t.Fatalf("parseAddrPort failed: %v", err)
	}
	if host != "192.168.1.1" {
		t.Errorf("host = %q, want 192.168.1.1", host)
	}
	if port != 8080 {
		t.Errorf("port = %d, want 8080", port)
	}

	_, _, err = parseAddrPort("invalid")
	if err == nil {
		t.Error("expected error for invalid format")
	}

	_, _, err = parseAddrPort("host:badport")
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestSTUNClient_DetectNATType_LocalServer(t *testing.T) {
	server := NewSTUNServer("127.0.0.1:0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start STUN server: %v", err)
	}
	defer server.Stop()

	addr := server.ListenAddr()
	client := NewSTUNClient(addr.String())

	natType, mapped, err := client.DetectNATType()
	if err != nil {
		t.Logf("DetectNATType returned error (expected when secondary port unavailable): %v", err)
	}

	t.Logf("NAT type: %s, mapped: %s", natType.String(), mapped)
}
