package tunnel

import (
	"context"
	"time"
)

type Tunnel struct {
	ID             string
	OrganizationID string
	WorkspaceID    string
	Name           string
	Slug           string
	Protocol       string
	LocalPort      int
	LocalHost      string
	CustomDomain   string
	Region         string
	TLSMode        string
	AuthMode       string
	MaxConnections int
	Status         TunnelStatus
	RelayID        string
	AgentID        string
	BytesInTotal   int64
	BytesOutTotal  int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type RelayNode struct {
	ID            string
	Name          string
	Region        string
	Hostname      string
	IPAddress     string
	Port          int
	Capacity      int
	ActiveTunnels int
	Status        string
}

type Repository interface {
	CreateTunnel(ctx context.Context, t *Tunnel) error
	GetTunnel(ctx context.Context, id string) (*Tunnel, error)
	GetTunnelBySlug(ctx context.Context, slug string) (*Tunnel, error)
	ListTunnels(ctx context.Context, workspaceID string, limit int, cursor string) ([]*Tunnel, string, error)
	UpdateTunnel(ctx context.Context, t *Tunnel) error
	DeleteTunnel(ctx context.Context, id string) error
	UpdateTunnelStatus(ctx context.Context, id string, status TunnelStatus) error
	UpdateTunnelRelay(ctx context.Context, id string, relayID string) error
	CountTunnelsByWorkspace(ctx context.Context, workspaceID string) (int, error)
	GetActiveRelays(ctx context.Context) ([]*RelayNode, error)
	GetRelayNode(ctx context.Context, id string) (*RelayNode, error)
	UpdateRelayActiveTunnels(ctx context.Context, id string, count int) error
}

type QuotaChecker interface {
	GetMaxTunnels(ctx context.Context, workspaceID string) (int, error)
}

type ConnectionCounter interface {
	GetActiveConnections(ctx context.Context, tunnelID string) (int32, error)
}
