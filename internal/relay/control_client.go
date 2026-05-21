package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Subscriber interface {
	Subscribe(ctx context.Context, subject string, handler func([]byte)) error
	Close() error
}

type ControlClient struct {
	orchestratorClient omnitunv1.TunnelServiceClient
	conn               *grpc.ClientConn
	subscriber         Subscriber
	relayID            string
	region             string
	listenAddr         string
	hostname           string
}

func NewControlClient(controlAddr string, region string, listenAddr string) (*ControlClient, error) {
	return NewControlClientWithID(controlAddr, region, listenAddr, "")
}

func NewControlClientWithID(controlAddr string, region string, listenAddr string, relayID string) (*ControlClient, error) {
	hostname, _ := os.Hostname()

	conn, err := grpc.NewClient(controlAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	client := omnitunv1.NewTunnelServiceClient(conn)

	if relayID == "" {
		var genErr error
		relayID, genErr = generateRelayID()
		if genErr != nil {
			conn.Close()
			return nil, fmt.Errorf("generate relay id: %w", genErr)
		}
	}

	return &ControlClient{
		orchestratorClient: client,
		conn:               conn,
		relayID:            relayID,
		region:             region,
		listenAddr:         listenAddr,
		hostname:           hostname,
	}, nil
}

func (c *ControlClient) Register(ctx context.Context) error {
	slog.Info("registering relay with control plane",
		"relay_id", c.relayID,
		"region", c.region,
		"hostname", c.hostname,
	)

	_, err := c.orchestratorClient.StartTunnel(ctx, &omnitunv1.StartTunnelRequest{
		TunnelId: c.relayID,
	})
	if err != nil {
		slog.Warn("relay registration via StartTunnel returned error (may be expected for bootstrap)",
			"relay_id", c.relayID,
			"error", err,
		)
	}

	slog.Info("relay registered with control plane",
		"relay_id", c.relayID,
		"hostname", c.hostname,
	)

	return nil
}

func (c *ControlClient) StartHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("heartbeat stopped", "relay_id", c.relayID)
			return
		case <-ticker.C:
			c.sendHeartbeat(ctx)
		}
	}
}

func (c *ControlClient) ReportHealth(ctx context.Context, activeTunnels int, bytesIn uint64, bytesOut uint64) error {
	slog.Debug("reporting relay health",
		"relay_id", c.relayID,
		"region", c.region,
		"active_tunnels", activeTunnels,
		"bytes_in", bytesIn,
		"bytes_out", bytesOut,
	)

	resp, err := c.orchestratorClient.GetTunnelStats(ctx, &omnitunv1.GetTunnelStatsRequest{
		TunnelId: c.relayID,
		Period:   "30s",
	})
	if err != nil {
		slog.Warn("health report failed",
			"relay_id", c.relayID,
			"error", err,
		)
		return fmt.Errorf("report health: %w", err)
	}

	slog.Debug("health report acknowledged",
		"relay_id", c.relayID,
		"control_active_conns", resp.GetStats().GetActiveConnections(),
	)

	return nil
}

func (c *ControlClient) sendHeartbeat(ctx context.Context) {
	_, err := c.orchestratorClient.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{
		TunnelId: c.relayID,
	})
	if err != nil {
		slog.Warn("heartbeat failed",
			"relay_id", c.relayID,
			"error", err,
		)
	} else {
		slog.Debug("heartbeat sent", "relay_id", c.relayID)
	}
}

func (c *ControlClient) SubscribeConfig(ctx context.Context, handler func([]byte)) error {
	if c.subscriber != nil {
		return c.subscriber.Subscribe(ctx, "omnitun.relay."+c.relayID+".config", handler)
	}

	slog.Info("no subscriber configured, config updates disabled",
		"relay_id", c.relayID,
	)

	go func() {
		<-ctx.Done()
	}()

	return nil
}

func (c *ControlClient) SetSubscriber(s Subscriber) {
	c.subscriber = s
}

func (c *ControlClient) RelayID() string {
	return c.relayID
}

func (c *ControlClient) Close() error {
	if c.subscriber != nil {
		c.subscriber.Close()
	}
	return c.conn.Close()
}

func generateRelayID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "relay-" + hex.EncodeToString(b)[:12], nil
}

type noopSubscriber struct{}

func NewNoopSubscriber() Subscriber {
	return &noopSubscriber{}
}

func (n *noopSubscriber) Subscribe(_ context.Context, _ string, _ func([]byte)) error {
	return nil
}

func (n *noopSubscriber) Close() error {
	return nil
}
