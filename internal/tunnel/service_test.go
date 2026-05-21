package tunnel

import (
	"context"
	"testing"
	"time"

	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
)

type mockRepo struct {
	tunnels     map[string]*Tunnel
	relayNodes  []*RelayNode
	createErr   error
	quotaCount  int
	quotaErr    error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		tunnels:    make(map[string]*Tunnel),
		relayNodes: []*RelayNode{},
	}
}

func (m *mockRepo) CreateTunnel(ctx context.Context, t *Tunnel) error {
	if m.createErr != nil {
		return m.createErr
	}
	t.ID = "mock-tunnel-id-" + t.Slug
	m.tunnels[t.ID] = t
	return nil
}

func (m *mockRepo) GetTunnel(ctx context.Context, id string) (*Tunnel, error) {
	t, ok := m.tunnels[id]
	if !ok {
		return nil, errTunnelNotFound
	}
	return t, nil
}

func (m *mockRepo) GetTunnelBySlug(ctx context.Context, slug string) (*Tunnel, error) {
	for _, t := range m.tunnels {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, errTunnelNotFound
}

func (m *mockRepo) ListTunnels(ctx context.Context, workspaceID string, limit int, cursor string) ([]*Tunnel, string, error) {
	var result []*Tunnel
	for _, t := range m.tunnels {
		if t.WorkspaceID == workspaceID {
			result = append(result, t)
		}
	}
	return result, "", nil
}

func (m *mockRepo) UpdateTunnel(ctx context.Context, t *Tunnel) error {
	m.tunnels[t.ID] = t
	return nil
}

func (m *mockRepo) DeleteTunnel(ctx context.Context, id string) error {
	t, ok := m.tunnels[id]
	if !ok {
		return errTunnelNotFound
	}
	now := time.Now().UTC()
	t.DeletedAt = &now
	return nil
}

func (m *mockRepo) UpdateTunnelStatus(ctx context.Context, id string, status TunnelStatus) error {
	t, ok := m.tunnels[id]
	if !ok {
		return errTunnelNotFound
	}
	t.Status = status
	return nil
}

func (m *mockRepo) UpdateTunnelRelay(ctx context.Context, id string, relayID string) error {
	t, ok := m.tunnels[id]
	if !ok {
		return errTunnelNotFound
	}
	t.RelayID = relayID
	return nil
}

func (m *mockRepo) CountTunnelsByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	if m.quotaErr != nil {
		return 0, m.quotaErr
	}
	return m.quotaCount, nil
}

func (m *mockRepo) GetActiveRelays(ctx context.Context) ([]*RelayNode, error) {
	return m.relayNodes, nil
}

func (m *mockRepo) GetRelayNode(ctx context.Context, id string) (*RelayNode, error) {
	for _, r := range m.relayNodes {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, errTunnelNotFound
}

func (m *mockRepo) UpdateRelayActiveTunnels(ctx context.Context, id string, count int) error {
	return nil
}

type mockEventBus struct {
	events []TunnelEvent
}

func (m *mockEventBus) PublishTunnelEvent(ctx context.Context, eventType string, tunnel *Tunnel) error {
	m.events = append(m.events, TunnelEvent{
		EventType: eventType,
		TunnelID:  tunnel.ID,
		Timestamp: time.Now().UTC(),
		Data:      tunnel,
	})
	return nil
}

type mockQuotaChecker struct {
	maxTunnels int
	err        error
}

func (m *mockQuotaChecker) GetMaxTunnels(ctx context.Context, workspaceID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.maxTunnels, nil
}

type mockConnCounter struct {
	conns int32
}

func (m *mockConnCounter) GetActiveConnections(ctx context.Context, tunnelID string) (int32, error) {
	return m.conns, nil
}

func newTestService() (*Service, *mockRepo, *mockEventBus) {
	repo := newMockRepo()
	repo.relayNodes = []*RelayNode{
		{
			ID:            "relay-1",
			Name:          "us-east",
			Region:        "us-east",
			Hostname:      "relay1.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 50,
			Status:        "online",
		},
		{
			ID:            "relay-2",
			Name:          "eu-west",
			Region:        "eu-west",
			Hostname:      "relay2.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 200,
			Status:        "online",
		},
	}
	relaySel := NewRelaySelector(repo)
	eventBus := &mockEventBus{}
	svc := NewService(repo, relaySel, eventBus)
	return svc, repo, eventBus
}

func TestCreateTunnel_Success(t *testing.T) {
	svc, _, eventBus := newTestService()

	resp, err := svc.CreateTunnel(context.Background(), &omnitunv1.CreateTunnelRequest{
		WorkspaceId: "ws-1",
		Name:        "test-tunnel",
		Protocol:    "tcp",
		LocalPort:   8080,
		LocalHost:   "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}

	if resp.Tunnel == nil {
		t.Fatal("expected tunnel in response")
	}
	if resp.Tunnel.Name != "test-tunnel" {
		t.Errorf("expected name 'test-tunnel', got '%s'", resp.Tunnel.Name)
	}
	if resp.Tunnel.Protocol != "tcp" {
		t.Errorf("expected protocol 'tcp', got '%s'", resp.Tunnel.Protocol)
	}
	if resp.Tunnel.LocalPort != 8080 {
		t.Errorf("expected local_port 8080, got %d", resp.Tunnel.LocalPort)
	}
	if resp.Tunnel.Status != string(StatusStopped) {
		t.Errorf("expected status '%s', got '%s'", StatusStopped, resp.Tunnel.Status)
	}
	if resp.Tunnel.Slug == "" {
		t.Error("expected non-empty slug")
	}
	if len(resp.Tunnel.Slug) != slugLength {
		t.Errorf("expected slug length %d, got %d", slugLength, len(resp.Tunnel.Slug))
	}

	if len(eventBus.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventBus.events))
	}
	if eventBus.events[0].EventType != "tunnel.created" {
		t.Errorf("expected event type 'tunnel.created', got '%s'", eventBus.events[0].EventType)
	}
}

func TestCreateTunnel_ExceedQuota(t *testing.T) {
	repo := newMockRepo()
	repo.relayNodes = []*RelayNode{
		{
			ID:            "relay-1",
			Name:          "us-east",
			Region:        "us-east",
			Hostname:      "relay1.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 50,
			Status:        "online",
		},
	}
	repo.quotaCount = 25
	relaySel := NewRelaySelector(repo)
	eventBus := &mockEventBus{}
	svc := NewService(repo, relaySel, eventBus)

	qc := &mockQuotaChecker{maxTunnels: 3}
	svc.WithQuotaChecker(qc)

	_, err := svc.CreateTunnel(context.Background(), &omnitunv1.CreateTunnelRequest{
		WorkspaceId: "ws-1",
		Name:        "test-tunnel",
		Protocol:    "tcp",
		LocalPort:   8080,
	})
	if err == nil {
		t.Fatal("expected quota exceeded error")
	}
}

func TestValidateTransition_Valid(t *testing.T) {
	tests := []struct {
		from TunnelStatus
		to   TunnelStatus
	}{
		{StatusStopped, StatusStarting},
		{StatusStarting, StatusActive},
		{StatusActive, StatusStopped},
		{StatusStopped, StatusError},
		{StatusError, StatusStopped},
		{StatusError, StatusStarting},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if err != nil {
				t.Errorf("expected valid transition %s -> %s, got error: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	tests := []struct {
		from TunnelStatus
		to   TunnelStatus
	}{
		{StatusStopped, StatusActive},
		{StatusActive, StatusStarting},
		{StatusStarting, StatusStopped},
		{StatusError, StatusActive},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if err == nil {
				t.Errorf("expected invalid transition %s -> %s, got nil error", tt.from, tt.to)
			}
		})
	}
}

func TestValidateTransition_UnknownStatus(t *testing.T) {
	err := ValidateTransition("unknown", StatusStopped)
	if err == nil {
		t.Error("expected error for unknown status")
	}
}

func TestRelaySelector_Select(t *testing.T) {
	repo := newMockRepo()
	repo.relayNodes = []*RelayNode{
		{
			ID:            "relay-1",
			Name:          "us-east",
			Region:        "us-east",
			Hostname:      "relay1.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 900,
			Status:        "online",
		},
		{
			ID:            "relay-2",
			Name:          "eu-west",
			Region:        "eu-west",
			Hostname:      "relay2.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 100,
			Status:        "online",
		},
		{
			ID:            "relay-3",
			Name:          "us-east-2",
			Region:        "us-east",
			Hostname:      "relay3.example.com",
			Port:          443,
			Capacity:      1000,
			ActiveTunnels: 500,
			Status:        "online",
		},
		{
			ID:            "relay-4",
			Name:          "full-node",
			Region:        "ap-south",
			Hostname:      "relay4.example.com",
			Port:          443,
			Capacity:      100,
			ActiveTunnels: 100,
			Status:        "online",
		},
	}

	sel := NewRelaySelector(repo)

	selected, err := sel.Select(context.Background(), "us-east")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if selected.ID != "relay-3" {
		t.Errorf("expected relay-3 (us-east with lower load ratio), got %s", selected.ID)
	}
}

func TestRelaySelector_NoCapacity(t *testing.T) {
	repo := newMockRepo()
	repo.relayNodes = []*RelayNode{
		{
			ID:            "relay-full",
			Name:          "full-node",
			Region:        "us-east",
			Hostname:      "relay.example.com",
			Port:          443,
			Capacity:      100,
			ActiveTunnels: 100,
			Status:        "online",
		},
	}

	sel := NewRelaySelector(repo)

	_, err := sel.Select(context.Background(), "us-east")
	if err == nil {
		t.Fatal("expected error for no capacity")
	}
}
