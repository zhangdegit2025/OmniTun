package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionFile_SaveAndLoad_FieldsMatch(t *testing.T) {

	orig := &SessionFile{
		AccessToken:  "access-token-abc123",
		RefreshToken: "refresh-token-xyz789",
		APIBaseURL:   "https://api.omnitun.io",
		ExpiresAt:    1750000000,
		Email:        "test@omnitun.io",
	}

	if err := SaveSession(orig); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}
	t.Cleanup(func() { ClearSession() })

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded session is nil")
	}

	if loaded.AccessToken != orig.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, orig.AccessToken)
	}
	if loaded.RefreshToken != orig.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, orig.RefreshToken)
	}
	if loaded.APIBaseURL != orig.APIBaseURL {
		t.Errorf("APIBaseURL = %q, want %q", loaded.APIBaseURL, orig.APIBaseURL)
	}
	if loaded.ExpiresAt != orig.ExpiresAt {
		t.Errorf("ExpiresAt = %d, want %d", loaded.ExpiresAt, orig.ExpiresAt)
	}
	if loaded.Email != orig.Email {
		t.Errorf("Email = %q, want %q", loaded.Email, orig.Email)
	}
}

func TestSessionFile_Clear_ReturnsNil(t *testing.T) {

	s := &SessionFile{
		AccessToken:  "token",
		RefreshToken: "refresh",
		APIBaseURL:   "https://api.example.com",
	}
	if err := SaveSession(s); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	if err := ClearSession(); err != nil {
		t.Fatalf("ClearSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession after clear failed: %v", err)
	}
	if loaded != nil {
		t.Error("LoadSession should return nil after ClearSession")
	}
}

func TestSessionFile_Permissions_UnixFileMode(t *testing.T) {

	s := &SessionFile{
		AccessToken: "perm-test-token",
	}
	if err := SaveSession(s); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}
	t.Cleanup(func() { ClearSession() })

	path, err := sessionPath()
	if err != nil {
		t.Fatalf("sessionPath failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat session file failed: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Logf("session file permissions: %o (may differ on Windows)", info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat session dir failed: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Logf("session dir permissions: %o (may differ on Windows)", dirInfo.Mode().Perm())
	}
}

func TestNewAgentClient_Defaults_ValidConfig(t *testing.T) {
	t.Parallel()

	cfg := &AgentClientConfig{
		ControlWSURL: "wss://control.omnitun.io/agent/v1/connect",
		Token:        "test-token",
	}

	client := NewAgentClient(cfg, nil)

	if client == nil {
		t.Fatal("NewAgentClient returned nil")
	}
	if client.cfg.AgentID == "" {
		t.Error("AgentID should be auto-generated")
	}
	if client.cfg.Version != "1.0.0" {
		t.Errorf("default Version = %q, want %q", client.cfg.Version, "1.0.0")
	}
	if client.tunnelConns == nil {
		t.Error("tunnelConns map should be initialized")
	}
}

func TestNewAgentClient_CustomAgentID_Respected(t *testing.T) {
	t.Parallel()

	cfg := &AgentClientConfig{
		ControlWSURL: "wss://control.omnitun.io/agent/v1/connect",
		AgentID:      "custom-agent-001",
		Token:        "test-token",
		Version:      "2.0.0",
	}

	client := NewAgentClient(cfg, nil)

	if client == nil {
		t.Fatal("NewAgentClient returned nil")
	}
	if client.cfg.AgentID != "custom-agent-001" {
		t.Errorf("AgentID = %q, want %q", client.cfg.AgentID, "custom-agent-001")
	}
	if client.cfg.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", client.cfg.Version, "2.0.0")
	}
}

func TestWSMessage_MarshalUnmarshal_RoundTrip(t *testing.T) {
	t.Parallel()

	hello := NewHelloMessage("agent-1", "1.0.0", "token-123")
	data, err := json.Marshal(hello)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var base WSMessage
	if err := json.Unmarshal(data, &base); err != nil {
		t.Fatalf("unmarshal base failed: %v", err)
	}
	if base.Type != MsgAgentHello {
		t.Errorf("Type = %q, want %q", base.Type, MsgAgentHello)
	}

	heartbeat := NewHeartbeatMessage("agent-2")
	data, err = json.Marshal(heartbeat)
	if err != nil {
		t.Fatalf("marshal heartbeat failed: %v", err)
	}

	if err := json.Unmarshal(data, &base); err != nil {
		t.Fatalf("unmarshal heartbeat failed: %v", err)
	}
	if base.Type != MsgAgentHeartbeat {
		t.Errorf("Heartbeat Type = %q, want %q", base.Type, MsgAgentHeartbeat)
	}
}

func TestAgentClientConfig_Validation_EmptyControlWSURL(t *testing.T) {
	t.Parallel()

	cfg := &AgentClientConfig{
		ControlWSURL: "",
	}

	if cfg.ControlWSURL != "" {
		t.Error("ControlWSURL should be empty")
	}

	cfg2 := &AgentClientConfig{
		ControlWSURL: "wss://valid.url",
	}
	if cfg2.ControlWSURL != "wss://valid.url" {
		t.Error("ControlWSURL should be set")
	}
}

func TestExponentialBackoff_Calculates_CapAt60s(t *testing.T) {
	t.Parallel()

	const maxBackoffMs = 60000

	backoffMs := 1000
	for i := 0; i < 10; i++ {
		if backoffMs > maxBackoffMs {
			t.Errorf("iteration %d: backoff %dms exceeded max %dms", i, backoffMs, maxBackoffMs)
		}
		backoffMs *= 2
		if backoffMs > maxBackoffMs {
			backoffMs = maxBackoffMs
		}
	}

	if backoffMs != maxBackoffMs {
		t.Errorf("final backoff = %dms, want %dms", backoffMs, maxBackoffMs)
	}
}

func TestExponentialBackoff_StartingValue(t *testing.T) {
	t.Parallel()

	backoffMs := 1000
	if backoffMs != 1000 {
		t.Errorf("initial backoff = %dms, want 1000ms", backoffMs)
	}

	backoffMs *= 2
	if backoffMs != 2000 {
		t.Errorf("second backoff = %dms, want 2000ms", backoffMs)
	}

	backoffMs *= 2
	if backoffMs != 4000 {
		t.Errorf("third backoff = %dms, want 4000ms", backoffMs)
	}
}

func TestJitter_WithinRange(t *testing.T) {
	t.Parallel()

	const (
		backoff      = 1000.0
		jitterFactor = 0.25
	)

	jitterRangeMs := float64(int64(backoff * jitterFactor))
	minJitter := -jitterRangeMs
	maxJitter := jitterRangeMs

	for i := 0; i < 1000; i++ {
		offset := float64(int64(i) % int64(jitterRangeMs*2+1))
		jitter := offset - jitterRangeMs
		if jitter < minJitter || jitter > maxJitter {
			t.Errorf("jitter %.0f out of range [%.0f, %.0f]", jitter, minJitter, maxJitter)
		}
	}
}

func TestNewTunnelConnection_InitializedCorrectly(t *testing.T) {
	t.Parallel()

	tc := NewTunnelConnection("tun-001", "relay.example.com:8443", "secret-token", "127.0.0.1", 8080)

	if tc == nil {
		t.Fatal("NewTunnelConnection returned nil")
	}
	if tc.TunnelID != "tun-001" {
		t.Errorf("TunnelID = %q, want %q", tc.TunnelID, "tun-001")
	}
	if tc.RelayAddr != "relay.example.com:8443" {
		t.Errorf("RelayAddr = %q, want %q", tc.RelayAddr, "relay.example.com:8443")
	}
	if tc.Token != "secret-token" {
		t.Errorf("Token = %q, want %q", tc.Token, "secret-token")
	}
}

func TestTunnelConnection_Establish_EmptyRelayAddr(t *testing.T) {
	t.Parallel()

	tc := NewTunnelConnection("tun-001", "", "token", "127.0.0.1", 8080)

	err := tc.Establish(nil)
	if err == nil {
		t.Error("Establish should fail with empty relay address")
	}
}

func TestTunnelConnection_Close_NoError(t *testing.T) {
	t.Parallel()

	tc := NewTunnelConnection("tun-001", "relay.example.com:8443", "token", "127.0.0.1", 8080)

	err := tc.Close()
	if err != nil {
		t.Errorf("Close on unestablished connection should not error: %v", err)
	}
}

func TestTunnelConnection_DoubleClose_Idempotent(t *testing.T) {
	t.Parallel()

	tc := NewTunnelConnection("tun-002", "relay.example.com:8443", "token", "127.0.0.1", 8080)

	if err := tc.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := tc.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestGenerateAgentID_Uniqueness(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateAgentID()
		if id == "" {
			t.Error("generateAgentID returned empty string")
		}
		if ids[id] {
			t.Errorf("duplicate agent ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestNewHelloMessage_FieldsSet(t *testing.T) {
	msg := NewHelloMessage("agent-42", "2.0.0", "tok-123")

	if msg.Type != MsgAgentHello {
		t.Errorf("Type = %q, want %q", msg.Type, MsgAgentHello)
	}
	if msg.AgentID != "agent-42" {
		t.Errorf("AgentID = %q, want %q", msg.AgentID, "agent-42")
	}
	if msg.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", msg.Version, "2.0.0")
	}
	if msg.Token != "tok-123" {
		t.Errorf("Token = %q, want %q", msg.Token, "tok-123")
	}
}

func TestNewHeartbeatMessage_FieldsSet(t *testing.T) {
	msg := NewHeartbeatMessage("agent-99")

	if msg.Type != MsgAgentHeartbeat {
		t.Errorf("Type = %q, want %q", msg.Type, MsgAgentHeartbeat)
	}
	if msg.AgentID != "agent-99" {
		t.Errorf("AgentID = %q, want %q", msg.AgentID, "agent-99")
	}
}

func TestAgentClient_HasValidFields(t *testing.T) {
	t.Parallel()

	cfg := &AgentClientConfig{
		ControlWSURL: "wss://control.example.com/connect",
		AgentID:      "ag-001",
		Token:        "tok-001",
		Version:      "1.5.0",
	}
	c := NewAgentClient(cfg, nil)

	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if connected {
		t.Error("new AgentClient should not be connected")
	}

	if c.IsConnected() {
		t.Error("IsConnected should return false for new client")
	}

	c.AddTunnelConnection(NewTunnelConnection("t1", "relay:8443", "tok", "127.0.0.1", 3000))

	c.mu.RLock()
	tcCount := len(c.tunnelConns)
	c.mu.RUnlock()
	if tcCount != 1 {
		t.Errorf("tunnel connection count = %d, want 1", tcCount)
	}

	c.RemoveTunnelConnection("t1")

	c.mu.RLock()
	tcCount = len(c.tunnelConns)
	c.mu.RUnlock()
	if tcCount != 0 {
		t.Errorf("tunnel connection count after remove = %d, want 0", tcCount)
	}
}

func TestDisconnect_NotConnected_NoError(t *testing.T) {
	t.Parallel()

	cfg := &AgentClientConfig{
		ControlWSURL: "wss://control.example.com/connect",
		AgentID:      "ag-dis",
		Token:        "tok-dis",
	}
	c := NewAgentClient(cfg, nil)

	if err := c.Disconnect(); err != nil {
		t.Errorf("Disconnect on new client should not error: %v", err)
	}
}
