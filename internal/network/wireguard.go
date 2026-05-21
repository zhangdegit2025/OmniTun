package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"golang.org/x/crypto/curve25519"
)

type WireGuardKeyPair struct {
	PrivateKey string
	PublicKey  string
}

type MeshPeer struct {
	NodeID     string
	PublicKey  string
	MeshIP     string
	Endpoint   string
	AllowedIPs []string
}

type MeshNetwork struct {
	ID      string
	Name    string
	CIDR    string
	Peers   map[string]*MeshPeer
	KeyPair *WireGuardKeyPair
	mu      sync.RWMutex
}

func GenerateWireGuardKeyPair() (*WireGuardKeyPair, error) {
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("compute public key: %w", err)
	}

	return &WireGuardKeyPair{
		PrivateKey: hex.EncodeToString(privateKey),
		PublicKey:  hex.EncodeToString(publicKey),
	}, nil
}

func (m *MeshNetwork) GenerateWGConfig(peerID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peer, ok := m.Peers[peerID]
	if !ok {
		return "", fmt.Errorf("peer not found: %s", peerID)
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
ListenPort = 51820

`, m.KeyPair.PrivateKey, peer.MeshIP)

	for id, p := range m.Peers {
		if id == peerID {
			continue
		}
		endpoint := p.Endpoint
		if endpoint == "" {
			endpoint = "0.0.0.0:51820"
		}
		config += fmt.Sprintf(`[Peer]
PublicKey = %s
AllowedIPs = %s/32
Endpoint = %s
PersistentKeepalive = 25

`, p.PublicKey, p.MeshIP, endpoint)
	}

	return config, nil
}

func NewMeshNetwork(name, cidr string) (*MeshNetwork, error) {
	keyPair, err := GenerateWireGuardKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate network id: %w", err)
	}

	return &MeshNetwork{
		ID:      hex.EncodeToString(idBytes),
		Name:    name,
		CIDR:    cidr,
		Peers:   make(map[string]*MeshPeer),
		KeyPair: keyPair,
	}, nil
}

func (m *MeshNetwork) AddPeer(peer *MeshPeer) error {
	if peer == nil {
		return fmt.Errorf("peer is nil")
	}
	if peer.NodeID == "" {
		return fmt.Errorf("peer NodeID is required")
	}
	if peer.MeshIP == "" {
		return fmt.Errorf("peer MeshIP is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Peers[peer.NodeID]; exists {
		return fmt.Errorf("peer %s already exists", peer.NodeID)
	}

	m.Peers[peer.NodeID] = peer
	return nil
}

func (m *MeshNetwork) RemovePeer(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Peers[nodeID]; !exists {
		return fmt.Errorf("peer not found: %s", nodeID)
	}

	delete(m.Peers, nodeID)
	return nil
}

func (m *MeshNetwork) GetPeer(nodeID string) (*MeshPeer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peer, ok := m.Peers[nodeID]
	return peer, ok
}

func (m *MeshNetwork) ListPeers() []*MeshPeer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*MeshPeer, 0, len(m.Peers))
	for _, p := range m.Peers {
		peers = append(peers, p)
	}
	return peers
}

func (m *MeshNetwork) PeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Peers)
}
