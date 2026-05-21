package control

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/omnitun/omnitun/internal/network"
)

func (a *Agent) JoinMesh(ctx context.Context, networkID string, inviteCode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.apiClient == nil {
		return fmt.Errorf("no api client configured")
	}

	slog.Info("agent joining mesh network",
		"agent_id", a.ID,
		"network_id", networkID,
	)

	resp := struct {
		NetworkID string `json:"network_id"`
		Cidr      string `json:"cidr"`
		PeerIP    string `json:"peer_ip"`
		Peers     []struct {
			NodeID    string `json:"node_id"`
			PublicKey string `json:"public_key"`
			MeshIP    string `json:"mesh_ip"`
			Endpoint  string `json:"endpoint"`
		} `json:"peers"`
	}{}

	path := fmt.Sprintf("/v1/networks/%s/join", networkID)
	reqBody := map[string]string{"invite_code": inviteCode, "agent_id": a.ID}
	if err := a.apiClient.do("POST", path, reqBody, &resp); err != nil {
		return fmt.Errorf("join mesh network: %w", err)
	}

	slog.Info("agent joined mesh network",
		"agent_id", a.ID,
		"network_id", networkID,
		"peer_ip", resp.PeerIP,
		"peer_count", len(resp.Peers),
	)

	return nil
}

func (a *Agent) LeaveMesh(ctx context.Context, networkID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.apiClient == nil {
		return fmt.Errorf("no api client configured")
	}

	slog.Info("agent leaving mesh network",
		"agent_id", a.ID,
		"network_id", networkID,
	)

	path := fmt.Sprintf("/v1/networks/%s", networkID)
	if err := a.apiClient.do("DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("leave mesh network: %w", err)
	}

	return nil
}

func (a *Agent) GetMeshPeers(ctx context.Context, networkID string) ([]*network.MeshPeer, error) {
	if a.apiClient == nil {
		return nil, fmt.Errorf("no api client configured")
	}

	type peerResponse struct {
		Peers []struct {
			NodeID     string   `json:"node_id"`
			PublicKey  string   `json:"public_key"`
			MeshIP     string   `json:"mesh_ip"`
			Endpoint   string   `json:"endpoint"`
			AllowedIPs []string `json:"allowed_ips"`
		} `json:"peers"`
	}

	var resp peerResponse
	path := fmt.Sprintf("/v1/networks/%s", networkID)
	if err := a.apiClient.do("GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get mesh peers: %w", err)
	}

	peers := make([]*network.MeshPeer, 0, len(resp.Peers))
	for _, p := range resp.Peers {
		peers = append(peers, &network.MeshPeer{
			NodeID:     p.NodeID,
			PublicKey:  p.PublicKey,
			MeshIP:     p.MeshIP,
			Endpoint:   p.Endpoint,
			AllowedIPs: p.AllowedIPs,
		})
	}

	return peers, nil
}
