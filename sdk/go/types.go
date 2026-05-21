package omnitun

import "time"

type TunnelProtocol string

const (
	ProtocolTCP   TunnelProtocol = "tcp"
	ProtocolHTTP  TunnelProtocol = "http"
	ProtocolHTTPS TunnelProtocol = "https"
)

type TunnelStatus string

const (
	StatusActive  TunnelStatus = "active"
	StatusStopped TunnelStatus = "stopped"
	StatusError   TunnelStatus = "error"
)

type Tunnel struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Protocol    TunnelProtocol `json:"protocol"`
	LocalPort   int            `json:"local_port"`
	RemotePort  int            `json:"remote_port"`
	Domain      string         `json:"domain,omitempty"`
	Status      TunnelStatus   `json:"status"`
	TrafficIn   int64          `json:"traffic_in"`
	TrafficOut  int64          `json:"traffic_out"`
	Tags        []string       `json:"tags,omitempty"`
	AuthMode    string         `json:"auth_mode,omitempty"`
	TLSMode     string         `json:"tls_mode,omitempty"`
	Compression bool           `json:"compression,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CreateTunnelOptions struct {
	Name        string         `json:"name"`
	Protocol    TunnelProtocol `json:"protocol"`
	LocalPort   int            `json:"local_port"`
	RemotePort  int            `json:"remote_port,omitempty"`
	Domain      string         `json:"domain,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	AuthMode    string         `json:"auth_mode,omitempty"`
	TLSMode     string         `json:"tls_mode,omitempty"`
	Compression bool           `json:"compression,omitempty"`
}

type UpdateTunnelOptions struct {
	Name        *string         `json:"name,omitempty"`
	Protocol    *TunnelProtocol `json:"protocol,omitempty"`
	LocalPort   *int            `json:"local_port,omitempty"`
	Domain      *string         `json:"domain,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	AuthMode    *string         `json:"auth_mode,omitempty"`
	TLSMode     *string         `json:"tls_mode,omitempty"`
	Compression *bool           `json:"compression,omitempty"`
}

type ListTunnelsParams struct {
	Status   string `url:"status,omitempty"`
	Protocol string `url:"protocol,omitempty"`
	Page     int    `url:"page,omitempty"`
	PerPage  int    `url:"per_page,omitempty"`
}

type Domain struct {
	ID           string    `json:"id"`
	Domain       string    `json:"domain"`
	TunnelID     string    `json:"tunnel_id,omitempty"`
	Verified     bool      `json:"verified"`
	Verification string    `json:"verification,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateDomainOptions struct {
	Domain   string `json:"domain"`
	TunnelID string `json:"tunnel_id,omitempty"`
}

type MeshNetwork struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	NodeCount int       `json:"node_count"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateNetworkOptions struct {
	Name string `json:"name"`
	CIDR string `json:"cidr"`
}

type MeshNode struct {
	ID        string    `json:"id"`
	NetworkID string    `json:"network_id"`
	Name      string    `json:"name"`
	IPAddress string    `json:"ip_address"`
	PublicKey string    `json:"public_key"`
	NATType   string    `json:"nat_type"`
	Endpoints []string  `json:"endpoints"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen_at"`
	CreatedAt time.Time `json:"created_at"`
}

type JoinNetworkOptions struct {
	InviteCode string `json:"invite_code"`
}

type APIError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type PaginatedList[T any] struct {
	Data       []T  `json:"data"`
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}
