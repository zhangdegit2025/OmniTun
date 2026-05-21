package network

import "context"

type Route struct {
	ID       string `json:"id"`
	TunnelID string `json:"tunnel_id"`
	RelayID  string `json:"relay_id"`
	Gateway  string `json:"gateway"`
	Priority int    `json:"priority"`
}

type Service interface {
	RegisterRoute(ctx context.Context, route *Route) (*Route, error)
	RemoveRoute(ctx context.Context, id string) error
	GetRoute(ctx context.Context, tunnelID string) (*Route, error)
	ListRoutes(ctx context.Context, relayID string) ([]*Route, error)
	HealthCheck(ctx context.Context) error
}
