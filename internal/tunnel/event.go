package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

type TunnelEvent struct {
	EventType string    `json:"event_type"`
	TunnelID  string    `json:"tunnel_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

type EventPublisher interface {
	PublishTunnelEvent(ctx context.Context, eventType string, tunnel *Tunnel) error
}

type NATSConn interface {
	Publish(subj string, data []byte) error
	io.Closer
}

type NATSPublisher struct {
	conn NATSConn
}

func NewNATSPublisher(conn NATSConn) *NATSPublisher {
	slog.Info("nats publisher initialized")
	return &NATSPublisher{conn: conn}
}

func (p *NATSPublisher) PublishTunnelEvent(ctx context.Context, eventType string, tunnel *Tunnel) error {
	event := TunnelEvent{
		EventType: eventType,
		TunnelID:  tunnel.ID,
		Timestamp: time.Now().UTC(),
		Data:      tunnel,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("nats publish: marshal event: %w", err)
	}

	subject := fmt.Sprintf("omnitun.tunnel.%s", eventType)
	if err := p.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	slog.Debug("tunnel event published",
		"subject", subject,
		"tunnel_id", tunnel.ID,
	)
	return nil
}

func (p *NATSPublisher) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
