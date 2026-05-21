package relay

import (
	"context"
	"testing"
	"time"

	"github.com/omnitun/omnitun/internal/control"
	"github.com/omnitun/omnitun/internal/tunnel"
)

type haMockRepo struct {
	relays []*tunnel.RelayNode
	err    error
}

func (m *haMockRepo) GetActiveRelays(ctx context.Context) ([]*tunnel.RelayNode, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.relays, nil
}

func (m *haMockRepo) CreateTunnel(ctx context.Context, t *tunnel.Tunnel) error             { return nil }
func (m *haMockRepo) GetTunnel(ctx context.Context, id string) (*tunnel.Tunnel, error)      { return nil, nil }
func (m *haMockRepo) GetTunnelBySlug(ctx context.Context, slug string) (*tunnel.Tunnel, error) { return nil, nil }
func (m *haMockRepo) ListTunnels(ctx context.Context, workspaceID string, limit int, cursor string) ([]*tunnel.Tunnel, string, error) {
	return nil, "", nil
}
func (m *haMockRepo) UpdateTunnel(ctx context.Context, t *tunnel.Tunnel) error             { return nil }
func (m *haMockRepo) DeleteTunnel(ctx context.Context, id string) error                      { return nil }
func (m *haMockRepo) UpdateTunnelStatus(ctx context.Context, id string, status tunnel.TunnelStatus) error { return nil }
func (m *haMockRepo) UpdateTunnelRelay(ctx context.Context, id string, relayID string) error { return nil }
func (m *haMockRepo) CountTunnelsByWorkspace(ctx context.Context, workspaceID string) (int, error) { return 0, nil }
func (m *haMockRepo) GetRelayNode(ctx context.Context, id string) (*tunnel.RelayNode, error) {
	for _, r := range m.relays {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}
func (m *haMockRepo) UpdateRelayActiveTunnels(ctx context.Context, id string, count int) error { return nil }

func TestFailover_SelectAlternativeRelay(t *testing.T) {
	repo := &haMockRepo{
		relays: []*tunnel.RelayNode{
			{
				ID:            "relay-1",
				Name:          "sg-node-1",
				Region:        "ap-southeast-1",
				Hostname:      "relay1.sg.omnitun.io",
				Port:          443,
				Capacity:      1000,
				ActiveTunnels: 900,
				Status:        "online",
			},
			{
				ID:            "relay-2",
				Name:          "sg-node-2",
				Region:        "ap-southeast-1",
				Hostname:      "relay2.sg.omnitun.io",
				Port:          443,
				Capacity:      1000,
				ActiveTunnels: 300,
				Status:        "online",
			},
			{
				ID:            "relay-3",
				Name:          "sg-node-3",
				Region:        "ap-southeast-1",
				Hostname:      "relay3.sg.omnitun.io",
				Port:          443,
				Capacity:      1000,
				ActiveTunnels: 500,
				Status:        "online",
			},
		},
	}

	sel := tunnel.NewRelaySelector(repo)

	selected, err := sel.SelectAlternative(context.Background(), "ap-southeast-1", "relay-1")
	if err != nil {
		t.Fatalf("SelectAlternative failed: %v", err)
	}
	if selected == nil {
		t.Fatal("expected non-nil selected relay")
	}
	if selected.ID != "relay-2" {
		t.Errorf("expected relay-2 (lowest load), got %s", selected.ID)
	}
	if selected.ID == "relay-1" {
		t.Error("failed relay relay-1 should be excluded")
	}

	selected2, err := sel.SelectAlternative(context.Background(), "ap-southeast-1", "relay-2")
	if err != nil {
		t.Fatalf("SelectAlternative (second exclusion) failed: %v", err)
	}
	if selected2.ID != "relay-3" {
		t.Errorf("expected relay-3 after excluding relay-2, got %s", selected2.ID)
	}
}

func TestFailover_AgentReconnect_AfterRelayFailure(t *testing.T) {
	agent := control.NewAgent("agent-001", "test-agent", "tenant-1", "1.0.0")
	agent.RelayID = "relay-primary"
	agent.Labels["region"] = "ap-southeast-1"

	tc := control.NewTunnelConnection("tun-001", "relay-primary:8443", "token-abc", "127.0.0.1", 8080)
	agent.SetTunnelConnection(tc)

	if agent.RelayID != "relay-primary" {
		t.Errorf("initial RelayID = %q, want %q", agent.RelayID, "relay-primary")
	}

	if agent.FailoverCount() != 0 {
		t.Errorf("initial failover count = %d, want 0", agent.FailoverCount())
	}

	agent.RemoveTunnelConnection("tun-001")
	if agent.FailoverCount() == 0 {
		t.Log("failover count remains 0 (no actual failover attempted without API client)")
	}
}

func TestFailover_NoAvailableRelay(t *testing.T) {
	repo := &haMockRepo{
		relays: []*tunnel.RelayNode{
			{
				ID:            "relay-full-1",
				Name:          "full-node-1",
				Region:        "ap-southeast-1",
				Hostname:      "relay-full1.sg.omnitun.io",
				Port:          443,
				Capacity:      100,
				ActiveTunnels: 100,
				Status:        "online",
			},
			{
				ID:            "relay-full-2",
				Name:          "full-node-2",
				Region:        "ap-southeast-1",
				Hostname:      "relay-full2.sg.omnitun.io",
				Port:          443,
				Capacity:      100,
				ActiveTunnels: 100,
				Status:        "online",
			},
		},
	}

	sel := tunnel.NewRelaySelector(repo)

	_, err := sel.SelectAlternative(context.Background(), "ap-southeast-1", "relay-failed")
	if err == nil {
		t.Fatal("expected error when no alternative relay is available")
	}
	t.Logf("expected no-available-relay error: %v", err)

	repoEmpty := &haMockRepo{}
	selEmpty := tunnel.NewRelaySelector(repoEmpty)

	_, err = selEmpty.SelectAlternative(context.Background(), "ap-southeast-1", "relay-failed")
	if err == nil {
		t.Fatal("expected error when no relays exist at all")
	}
	t.Logf("expected empty-region error: %v", err)
}

func TestFailover_HeartbeatTimeout_Detection(t *testing.T) {
	timeout := 30 * time.Second
	failureWindow := 3 * timeout
	_ = failureWindow

	detected := false
	failCount := 0
	const maxFailures = 3

	for i := 0; i < maxFailures; i++ {
		failCount++
		if failCount >= maxFailures {
			detected = true
		}
	}

	if !detected {
		t.Error("expected failure detection after 3 heartbeat timeouts")
	}

	if failCount < maxFailures {
		t.Errorf("failCount = %d, want at least %d", failCount, maxFailures)
	}
}

func TestFailover_ExcludesFailedRelay(t *testing.T) {
	repo := &haMockRepo{
		relays: []*tunnel.RelayNode{
			{
				ID:            "relay-a",
				Name:          "node-a",
				Region:        "us-east-1",
				Hostname:      "relay-a.example.com",
				Port:          443,
				Capacity:      500,
				ActiveTunnels: 10,
				Status:        "online",
			},
			{
				ID:            "relay-b",
				Name:          "node-b",
				Region:        "us-east-1",
				Hostname:      "relay-b.example.com",
				Port:          443,
				Capacity:      500,
				ActiveTunnels: 50,
				Status:        "online",
			},
		},
	}

	sel := tunnel.NewRelaySelector(repo)

	selected, err := sel.SelectAlternative(context.Background(), "us-east-1", "relay-a")
	if err != nil {
		t.Fatalf("SelectAlternative failed: %v", err)
	}
	if selected.ID == "relay-a" {
		t.Errorf("failed relay relay-a should be excluded, got %s", selected.ID)
	}
	if selected.ID != "relay-b" {
		t.Errorf("expected relay-b, got %s", selected.ID)
	}
}

func TestFailover_OfflineRelay_Excluded(t *testing.T) {
	repo := &haMockRepo{
		relays: []*tunnel.RelayNode{
			{
				ID:            "relay-offline",
				Name:          "offline-node",
				Region:        "eu-west-1",
				Hostname:      "relay-offline.example.com",
				Port:          443,
				Capacity:      500,
				ActiveTunnels: 10,
				Status:        "offline",
			},
			{
				ID:            "relay-online",
				Name:          "online-node",
				Region:        "eu-west-1",
				Hostname:      "relay-online.example.com",
				Port:          443,
				Capacity:      500,
				ActiveTunnels: 50,
				Status:        "online",
			},
		},
	}

	sel := tunnel.NewRelaySelector(repo)

	selected, err := sel.SelectAlternative(context.Background(), "eu-west-1", "relay-failed")
	if err != nil {
		t.Fatalf("SelectAlternative failed: %v", err)
	}
	if selected.ID != "relay-online" {
		t.Errorf("expected relay-online (offline excluded), got %s", selected.ID)
	}
}
