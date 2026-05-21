package relay

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

type Dispatcher struct {
	tunnels  sync.Map
	byDomain sync.Map
	byPort   sync.Map
	bySlug   sync.Map
}

type TunnelContext struct {
	TunnelID   string
	AgentID    string
	StreamID   uint64
	Connection *StreamConnection
	CreatedAt  time.Time
	BytesIn    uint64
	BytesOut   uint64
	Domain     string
	Port       int
	Protocol   string
	Slug       string
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

func (d *Dispatcher) Lookup(host string) (*TunnelContext, bool) {
	if host == "" {
		return nil, false
	}

	if v, ok := d.byDomain.Load(host); ok {
		tunnelID := v.(string)
		return d.lookupByID(tunnelID)
	}

	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			wildcard := "*" + host[i:]
			if v, ok := d.byDomain.Load(wildcard); ok {
				tunnelID := v.(string)
				return d.lookupByID(tunnelID)
			}
		}
	}

	return nil, false
}

func (d *Dispatcher) LookupByPort(port int) (*TunnelContext, bool) {
	if v, ok := d.byPort.Load(port); ok {
		tunnelID := v.(string)
		return d.lookupByID(tunnelID)
	}
	return nil, false
}

func (d *Dispatcher) LookupBySlug(slug string) (*TunnelContext, bool) {
	if slug == "" {
		return nil, false
	}
	if v, ok := d.bySlug.Load(slug); ok {
		tunnelID := v.(string)
		return d.lookupByID(tunnelID)
	}
	return nil, false
}

func (d *Dispatcher) lookupByID(tunnelID string) (*TunnelContext, bool) {
	v, ok := d.tunnels.Load(tunnelID)
	if !ok {
		return nil, false
	}
	return v.(*TunnelContext), true
}

func (d *Dispatcher) Register(ctx *TunnelContext) error {
	if ctx.TunnelID == "" {
		return ErrEmptyTunnelID
	}

	d.tunnels.Store(ctx.TunnelID, ctx)

	if ctx.Domain != "" {
		d.byDomain.Store(ctx.Domain, ctx.TunnelID)
		slog.Debug("domain registered",
			"domain", ctx.Domain,
			"tunnel_id", ctx.TunnelID,
		)
	}

	if ctx.Port > 0 {
		d.byPort.Store(ctx.Port, ctx.TunnelID)
		slog.Debug("port registered",
			"port", ctx.Port,
			"tunnel_id", ctx.TunnelID,
		)
	}

	if ctx.Slug != "" {
		d.bySlug.Store(ctx.Slug, ctx.TunnelID)
		slog.Debug("slug registered",
			"slug", ctx.Slug,
			"tunnel_id", ctx.TunnelID,
		)
	}

	slog.Info("tunnel registered",
		"tunnel_id", ctx.TunnelID,
		"agent_id", ctx.AgentID,
		"domain", ctx.Domain,
		"port", ctx.Port,
	)

	return nil
}

func (d *Dispatcher) Unregister(tunnelID string) {
	v, ok := d.tunnels.Load(tunnelID)
	if !ok {
		return
	}

	ctx := v.(*TunnelContext)

	if ctx.Domain != "" {
		d.byDomain.Delete(ctx.Domain)
	}

	if ctx.Port > 0 {
		d.byPort.Delete(ctx.Port)
	}

	if ctx.Slug != "" {
		d.bySlug.Delete(ctx.Slug)
	}

	d.tunnels.Delete(tunnelID)

	slog.Info("tunnel unregistered",
		"tunnel_id", tunnelID,
		"domain", ctx.Domain,
		"port", ctx.Port,
	)
}

func (d *Dispatcher) OnConfigUpdate(msg []byte) {
	var config struct {
		Tunnels []struct {
			TunnelID string `json:"tunnel_id"`
			AgentID  string `json:"agent_id"`
			Domain   string `json:"domain"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			StreamID uint64 `json:"stream_id,omitempty"`
		} `json:"tunnels"`
	}

	if err := json.Unmarshal(msg, &config); err != nil {
		slog.Error("failed to parse config update", "error", err)
		return
	}

	for _, t := range config.Tunnels {
		ctx := &TunnelContext{
			TunnelID:  t.TunnelID,
			AgentID:   t.AgentID,
			StreamID:  t.StreamID,
			Domain:    t.Domain,
			Port:      t.Port,
			Protocol:  t.Protocol,
			CreatedAt: time.Now(),
		}
		if err := d.Register(ctx); err != nil {
			slog.Error("failed to register tunnel from config update",
				"tunnel_id", t.TunnelID,
				"error", err,
			)
		}
	}

	slog.Info("config update applied", "tunnel_count", len(config.Tunnels))
}

func (d *Dispatcher) TunnelCount() int {
	count := 0
	d.tunnels.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (d *Dispatcher) AddBytesIn(tunnelID string, n uint64) {
	if v, ok := d.tunnels.Load(tunnelID); ok {
		ctx := v.(*TunnelContext)
		ctx.BytesIn += n
	}
}

func (d *Dispatcher) AddBytesOut(tunnelID string, n uint64) {
	if v, ok := d.tunnels.Load(tunnelID); ok {
		ctx := v.(*TunnelContext)
		ctx.BytesOut += n
	}
}
