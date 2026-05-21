package control

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Agent struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	TenantID  string            `json:"tenant_id"`
	Version   string            `json:"version"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels"`
	RelayID   string            `json:"relay_id"`
	LastSeen  int64             `json:"last_seen"`
	CreatedAt int64             `json:"created_at"`

	apiClient     *APIClient
	tunnelConns   map[string]*TunnelConnection
	tunnelInfos   map[string]*TunnelInfo
	failoverCount int
	mu            sync.RWMutex
}

type TunnelInfo struct {
	TunnelID     string `json:"tunnel_id"`
	RelayAddress string `json:"relay_address"`
	TunnelToken  string `json:"tunnel_token"`
}

func NewAgent(id, name, tenantID, version string) *Agent {
	return &Agent{
		ID:          id,
		Name:        name,
		TenantID:    tenantID,
		Version:     version,
		Status:      "offline",
		Labels:      make(map[string]string),
		tunnelConns: make(map[string]*TunnelConnection),
		tunnelInfos: make(map[string]*TunnelInfo),
		CreatedAt:   time.Now().Unix(),
	}
}

func (a *Agent) SetAPIClient(client *APIClient) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.apiClient = client
}

func (a *Agent) SetTunnelConnection(tc *TunnelConnection) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tunnelConns == nil {
		a.tunnelConns = make(map[string]*TunnelConnection)
	}
	if a.tunnelInfos == nil {
		a.tunnelInfos = make(map[string]*TunnelInfo)
	}
	a.tunnelConns[tc.TunnelID] = tc
	a.tunnelInfos[tc.TunnelID] = &TunnelInfo{
		TunnelID:     tc.TunnelID,
		RelayAddress: tc.RelayAddr,
		TunnelToken:  tc.Token,
	}
}

func (a *Agent) RemoveTunnelConnection(tunnelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tunnelConns, tunnelID)
	delete(a.tunnelInfos, tunnelID)
}

func (a *Agent) HandleTunnelStartAck(tunnelID, relayAddress, tunnelToken string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tunnelInfos == nil {
		a.tunnelInfos = make(map[string]*TunnelInfo)
	}
	a.tunnelInfos[tunnelID] = &TunnelInfo{
		TunnelID:     tunnelID,
		RelayAddress: relayAddress,
		TunnelToken:  tunnelToken,
	}
	slog.Info("tunnel start acknowledged",
		"agent_id", a.ID,
		"tunnel_id", tunnelID,
		"relay_address", relayAddress,
	)
}

func (a *Agent) HandleTunnelStopCmd(tunnelID string) {
	a.mu.RLock()
	tc, ok := a.tunnelConns[tunnelID]
	a.mu.RUnlock()

	if ok && tc != nil {
		tc.Close()
	}

	a.mu.Lock()
	delete(a.tunnelInfos, tunnelID)
	a.mu.Unlock()

	slog.Info("tunnel stopped",
		"agent_id", a.ID,
		"tunnel_id", tunnelID,
	)
}

func (a *Agent) GetTunnelInfo(tunnelID string) *TunnelInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.tunnelInfos == nil {
		return nil
	}
	return a.tunnelInfos[tunnelID]
}

func (a *Agent) OnRelayFailed(ctx context.Context) error {
	a.mu.Lock()
	a.failoverCount++
	failoverAttempt := a.failoverCount
	apiClient := a.apiClient
	region := a.Labels["region"]
	failedRelayID := a.RelayID
	a.mu.Unlock()

	slog.Warn("relay failure detected, initiating failover",
		"agent_id", a.ID,
		"failed_relay_id", failedRelayID,
		"region", region,
		"attempt", failoverAttempt,
	)

	const maxAttempts = 5
	if failoverAttempt > maxAttempts {
		return fmt.Errorf("relay failover: exceeded max attempts (%d)", maxAttempts)
	}

	var newRelayAddress string
	var newRelayID string
	var newToken string

	if apiClient != nil {
		resp, err := apiClient.GetAlternativeRelay(region, failedRelayID)
		if err != nil {
			slog.Error("failed to get alternative relay from orchestrator",
				"agent_id", a.ID,
				"region", region,
				"failed_relay_id", failedRelayID,
				"error", err,
			)
			return fmt.Errorf("relay failover: get alternative relay: %w", err)
		}
		newRelayID = resp.RelayID
		newRelayAddress = fmt.Sprintf("%s:%d", resp.Hostname, resp.Port)
		newToken = ""
		slog.Info("alternative relay assigned by orchestrator",
			"agent_id", a.ID,
			"new_relay_id", newRelayID,
			"new_relay_address", newRelayAddress,
		)
	} else {
		return fmt.Errorf("relay failover: no API client configured")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	oldRelayID := a.RelayID
	a.RelayID = newRelayID

	reestablished := 0
	failed := 0
	for tunnelID, tc := range a.tunnelConns {
		tc.RelayAddr = newRelayAddress
		tc.Token = newToken
		tc.closeTransport()

		estCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := tc.Establish(estCtx); err != nil {
			slog.Error("failed to re-establish tunnel to new relay",
				"agent_id", a.ID,
				"tunnel_id", tunnelID,
				"new_relay_id", newRelayID,
				"error", err,
			)
			failed++
		} else {
			reestablished++
		}
		cancel()
	}

	slog.Info("relay failover completed",
		"agent_id", a.ID,
		"old_relay_id", oldRelayID,
		"new_relay_id", newRelayID,
		"tunnels_reestablished", reestablished,
		"tunnels_failed", failed,
	)

	if reestablished == 0 && failed > 0 {
		return fmt.Errorf("relay failover: all %d tunnels failed to re-establish", failed)
	}

	return nil
}

func (a *Agent) FailoverCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.failoverCount
}

type AgentController interface {
	Register(ctx context.Context, agent *Agent) (*Agent, error)
	Heartbeat(ctx context.Context, agentID string) error
	Unregister(ctx context.Context, agentID string) error
	Get(ctx context.Context, agentID string) (*Agent, error)
	List(ctx context.Context, tenantID string) ([]*Agent, error)
	DispatchCommand(ctx context.Context, agentID string, cmd string, payload []byte) error
	UpdateLabels(ctx context.Context, agentID string, labels map[string]string) error
}
