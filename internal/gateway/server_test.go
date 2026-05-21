package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid message",
			input:   `{"type":"agent.hello","id":"abc123","timestamp":"2024-01-01T00:00:00Z","payload":{"token":"test"}}`,
			want:    "agent.hello",
			wantErr: false,
		},
		{
			name:    "minimal message",
			input:   `{"type":"agent.ping"}`,
			want:    "agent.ping",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid`,
			wantErr: true,
		},
		{
			name:    "empty object",
			input:   `{}`,
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg.Type != tt.want {
				t.Fatalf("expected type %q, got %q", tt.want, msg.Type)
			}
		})
	}
}

func TestNewMessage(t *testing.T) {
	payload := map[string]string{"key": "value"}
	msg, err := NewMessage("agent.test", payload)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Type != "agent.test" {
		t.Fatalf("expected type 'agent.test', got %q", msg.Type)
	}

	if msg.ID == "" {
		t.Fatal("expected non-empty message ID")
	}

	if msg.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}

	var decoded map[string]string
	if err := msg.UnmarshalPayload(&decoded); err != nil {
		t.Fatalf("UnmarshalPayload failed: %v", err)
	}

	if decoded["key"] != "value" {
		t.Fatalf("expected payload key 'value', got %q", decoded["key"])
	}
}

func TestNewMessageNilPayload(t *testing.T) {
	msg, err := NewMessage("agent.test", nil)
	if err != nil {
		t.Fatalf("NewMessage with nil payload failed: %v", err)
	}
	if msg.Payload != nil {
		t.Fatal("expected nil payload for nil input")
	}
}

func TestWSMessageMarshal(t *testing.T) {
	msg := &WSMessage{
		Type:      "agent.test",
		ID:        "test-id",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	parsed, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if parsed.Type != msg.Type {
		t.Fatalf("roundtrip type mismatch: %q vs %q", msg.Type, parsed.Type)
	}
	if parsed.ID != msg.ID {
		t.Fatalf("roundtrip ID mismatch: %q vs %q", msg.ID, parsed.ID)
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	conn := newTestAgentConnection(hub, "agent-001", "org-001")

	hub.Register(conn)
	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 1 {
		t.Fatalf("expected 1 agent, got %d", hub.AgentCount())
	}

	retrieved, ok := hub.Get("agent-001")
	if !ok {
		t.Fatal("expected to find agent-001")
	}
	if retrieved.AgentID != "agent-001" {
		t.Fatalf("expected agent-001, got %s", retrieved.AgentID)
	}

	hub.Unregister(conn)
	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 0 {
		t.Fatalf("expected 0 agents after unregister, got %d", hub.AgentCount())
	}

	_, ok = hub.Get("agent-001")
	if ok {
		t.Fatal("expected agent-001 to be gone")
	}
}

func TestHub_DuplicateRegister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	conn1 := newTestAgentConnection(hub, "agent-001", "org-001")
	conn2 := newTestAgentConnection(hub, "agent-001", "org-001")

	hub.Register(conn1)
	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 1 {
		t.Fatalf("expected 1 agent, got %d", hub.AgentCount())
	}

	hub.Register(conn2)
	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 1 {
		t.Fatalf("expected 1 agent after duplicate, got %d", hub.AgentCount())
	}

	retrieved, ok := hub.Get("agent-001")
	if !ok {
		t.Fatal("expected to find agent-001")
	}
	if retrieved != conn2 {
		t.Fatal("expected conn2 to replace conn1")
	}
}

func TestHub_GetAgentsByOrg(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	hub.Register(newTestAgentConnection(hub, "agent-001", "org-001"))
	hub.Register(newTestAgentConnection(hub, "agent-002", "org-001"))
	hub.Register(newTestAgentConnection(hub, "agent-003", "org-002"))

	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 3 {
		t.Fatalf("expected 3 agents, got %d", hub.AgentCount())
	}

	org1Agents := hub.GetAgentsByOrg("org-001")
	if len(org1Agents) != 2 {
		t.Fatalf("expected 2 agents in org-001, got %d", len(org1Agents))
	}

	org2Agents := hub.GetAgentsByOrg("org-002")
	if len(org2Agents) != 1 {
		t.Fatalf("expected 1 agent in org-002, got %d", len(org2Agents))
	}

	emptyAgents := hub.GetAgentsByOrg("org-nonexistent")
	if len(emptyAgents) != 0 {
		t.Fatalf("expected 0 agents in nonexistent org, got %d", len(emptyAgents))
	}
}

func TestHub_HeartbeatCheck(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	conn1 := newTestAgentConnection(hub, "agent-001", "org-001")
	conn2 := newTestAgentConnection(hub, "agent-002", "org-001")

	hub.Register(conn1)
	hub.Register(conn2)

	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 2 {
		t.Fatalf("expected 2 agents, got %d", hub.AgentCount())
	}

	conn2.LastPing = time.Now().Add(-2 * time.Minute)

	hub.checkHeartbeats()

	time.Sleep(10 * time.Millisecond)

	if hub.AgentCount() != 1 {
		t.Fatalf("expected 1 agent after heartbeat check, got %d", hub.AgentCount())
	}

	_, ok := hub.Get("agent-001")
	if !ok {
		t.Fatal("expected agent-001 to still be connected")
	}

	_, ok = hub.Get("agent-002")
	if ok {
		t.Fatal("expected agent-002 to be removed due to timeout")
	}
}

func TestHub_ConcurrentRegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	numAgents := 100

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agentID := "agent-" + string(rune('A'+idx%26)) + string(rune('0'+idx%10))
			conn := newTestAgentConnection(hub, agentID, "org-001")
			hub.Register(conn)
			time.Sleep(1 * time.Millisecond)
			hub.Unregister(conn)
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	count := hub.AgentCount()
	if count > 0 {
		t.Logf("remaining agents after concurrent test: %d (may have stale registrations)", count)
	}
}

func TestHub_ConcurrentSendToAgent(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	conn := newTestAgentConnection(hub, "agent-001", "org-001")
	hub.Register(conn)

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	numMessages := 500

	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := hub.SendToAgent("agent-001", "server.test", map[string]string{"value": "test"})
			if err != nil {
				t.Errorf("SendToAgent failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestHub_BroadcastToOrg(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	time.Sleep(10 * time.Millisecond)

	conn1 := newTestAgentConnection(hub, "agent-001", "org-001")
	conn2 := newTestAgentConnection(hub, "agent-002", "org-001")
	conn3 := newTestAgentConnection(hub, "agent-003", "org-002")

	hub.Register(conn1)
	hub.Register(conn2)
	hub.Register(conn3)

	time.Sleep(10 * time.Millisecond)

	hub.BroadcastToOrg("org-001", "server.broadcast", map[string]string{"msg": "hello"})
}

func TestAuthManager_AuthenticateAgent_MissingToken(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)

	_, _, err := authMgr.AuthenticateAgent(t.Context(), "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestAuthManager_AuthenticateAgent_InvalidJWT(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)

	_, _, err := authMgr.AuthenticateAgent(t.Context(), "invalid.token.here")
	if err == nil {
		t.Fatal("expected error for invalid JWT")
	}
}

func TestAuthManager_AuthenticateAgent_ExpiredJWT(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)

	claims := jwtClaims{
		Sub:     "test-agent",
		Org:     "test-org",
		AgentID: "test-agent",
		Iat:     time.Now().Add(-2 * time.Hour).Unix(),
		Exp:     time.Now().Add(-1 * time.Hour).Unix(),
	}

	token := authMgr.generateJWT(claims)

	_, _, err := authMgr.AuthenticateAgent(t.Context(), token)
	if err == nil {
		t.Fatal("expected error for expired JWT")
	}
}

func TestAuthManager_AuthenticateAgent_ValidJWT(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)

	claims := jwtClaims{
		Sub:     "test-agent",
		Org:     "test-org",
		AgentID: "test-agent",
		Iat:     time.Now().Unix(),
		Exp:     time.Now().Add(1 * time.Hour).Unix(),
	}

	token := authMgr.generateJWT(claims)

	agentID, orgID, err := authMgr.AuthenticateAgent(t.Context(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agentID != "test-agent" {
		t.Fatalf("expected agentID 'test-agent', got %q", agentID)
	}
	if orgID != "test-org" {
		t.Fatalf("expected orgID 'test-org', got %q", orgID)
	}
}

func TestAuthManager_AuthenticateHello(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)

	claims := jwtClaims{
		Sub:     "test-agent",
		Org:     "test-org",
		AgentID: "test-agent",
		Iat:     time.Now().Unix(),
		Exp:     time.Now().Add(1 * time.Hour).Unix(),
	}

	token := authMgr.generateJWT(claims)

	helloMsg, err := NewMessage(MsgAgentHello, HelloPayload{
		Token:   token,
		AgentID: "test-agent",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	authInfo, err := authMgr.AuthenticateHello(t.Context(), helloMsg)
	if err != nil {
		t.Fatalf("AuthenticateHello failed: %v", err)
	}

	if authInfo.AgentID != "test-agent" {
		t.Fatalf("expected agentID 'test-agent', got %q", authInfo.AgentID)
	}
	if authInfo.OrgID != "test-org" {
		t.Fatalf("expected orgID 'test-org', got %q", authInfo.OrgID)
	}
	if authInfo.Version != "1.0.0" {
		t.Fatalf("expected version '1.0.0', got %q", authInfo.Version)
	}
}

func TestServer_HandlerRegistration(t *testing.T) {
	mux := http.NewServeMux()
	authMgr := NewAuthManager("test-secret", nil)
	srv := NewServer(&Config{
		ListenAddr:   "127.0.0.1:0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, authMgr)

	mux.HandleFunc("/agent/v1/connect", srv.HandleAgentConnect)
	mux.HandleFunc("/health", srv.handleHealth)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), "healthy") {
		t.Fatalf("expected healthy status, got %s", string(body[:n]))
	}
}

func TestHandleAgentConnect_InvalidToken(t *testing.T) {
	authMgr := NewAuthManager("test-secret", nil)
	srv := NewServer(&Config{
		ListenAddr:   "127.0.0.1:0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, authMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("/agent/v1/connect", srv.HandleAgentConnect)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/agent/v1/connect"

	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer wsConn.Close()

	helloMsg, err := NewMessage(MsgAgentHello, HelloPayload{
		Token:   "invalid-token",
		AgentID: "test-agent",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	helloData, _ := helloMsg.Marshal()
	wsConn.WriteMessage(websocket.TextMessage, helloData)

	_, errMsg, readErr := wsConn.ReadMessage()
	if readErr != nil {
		t.Fatalf("expected to read error response: %v", readErr)
	}

	msg, parseErr := ParseMessage(errMsg)
	if parseErr != nil {
		t.Fatalf("failed to parse error message: %v", parseErr)
	}

	if msg.Type != MsgServerError {
		t.Fatalf("expected server.error message, got %q", msg.Type)
	}

	_, _, err = wsConn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close error after auth failure")
	}
}

func newTestAgentConnection(hub *Hub, agentID, orgID string) *AgentConnection {
	return hub.newAgentConnection(
		&websocket.Conn{},
		&HelloAuthInfo{
			AgentID: agentID,
			OrgID:   orgID,
			Version: "1.0.0",
		},
	)
}
