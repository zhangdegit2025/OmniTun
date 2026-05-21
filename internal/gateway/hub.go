package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	heartbeatInterval  = 30 * time.Second
	heartbeatTimeout   = 90 * time.Second
	sendChannelBuffer  = 256
)

type AgentConnection struct {
	AgentID  string
	OrgID    string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	LastPing time.Time
	Version  string
	ctx      context.Context
	cancel   context.CancelFunc
}

func (c *AgentConnection) Context() context.Context {
	return c.ctx
}

func (c *AgentConnection) Close() {
	c.cancel()
}

func (c *AgentConnection) WriteMessage(data []byte) error {
	select {
	case c.Send <- data:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		slog.Warn("agent send channel full, dropping message",
			"agent_id", c.AgentID,
		)
		return nil
	}
}

func (c *AgentConnection) writePump() {
	ticker := time.NewTicker(heartbeatInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				slog.Warn("write message failed", "agent_id", c.AgentID, "error", err)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Warn("ping failed", "agent_id", c.AgentID, "error", err)
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

type Hub struct {
	agents     map[string]*AgentConnection
	byOrg      map[string]map[string]*AgentConnection
	register   chan *AgentConnection
	unregister chan *AgentConnection
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		agents:     make(map[string]*AgentConnection),
		byOrg:      make(map[string]map[string]*AgentConnection),
		register:   make(chan *AgentConnection, 64),
		unregister: make(chan *AgentConnection, 64),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.add(conn)
		case conn := <-h.unregister:
			h.remove(conn)
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *Hub) add(conn *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	existing, exists := h.agents[conn.AgentID]
	if exists {
		slog.Warn("agent reconnecting, closing existing connection",
			"agent_id", conn.AgentID,
		)
		existing.Close()
		delete(h.agents, conn.AgentID)
		if orgAgents, ok := h.byOrg[existing.OrgID]; ok {
			delete(orgAgents, conn.AgentID)
		}
	}

	h.agents[conn.AgentID] = conn

	if conn.OrgID != "" {
		if _, ok := h.byOrg[conn.OrgID]; !ok {
			h.byOrg[conn.OrgID] = make(map[string]*AgentConnection)
		}
		h.byOrg[conn.OrgID][conn.AgentID] = conn
	}

	gatewayAgentsConnected.Set(float64(len(h.agents)))

	slog.Info("agent registered",
		"agent_id", conn.AgentID,
		"org_id", conn.OrgID,
		"total_agents", len(h.agents),
	)
}

func (h *Hub) remove(conn *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.agents[conn.AgentID]; ok && existing == conn {
		delete(h.agents, conn.AgentID)
		if orgAgents, ok := h.byOrg[conn.OrgID]; ok {
			delete(orgAgents, conn.AgentID)
			if len(orgAgents) == 0 {
				delete(h.byOrg, conn.OrgID)
			}
		}
		gatewayAgentsConnected.Set(float64(len(h.agents)))

		slog.Info("agent unregistered",
			"agent_id", conn.AgentID,
			"org_id", conn.OrgID,
			"total_agents", len(h.agents),
		)
	}
}

func (h *Hub) Register(conn *AgentConnection) {
	conn.LastPing = time.Now()
	select {
	case h.register <- conn:
	case <-h.ctx.Done():
	}
}

func (h *Hub) Unregister(conn *AgentConnection) {
	select {
	case h.unregister <- conn:
	case <-h.ctx.Done():
	}
}

func (h *Hub) Get(agentID string) (*AgentConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.agents[agentID]
	return conn, ok
}

func (h *Hub) SendToAgent(agentID string, msgType string, payload interface{}) error {
	h.mu.RLock()
	conn, ok := h.agents[agentID]
	h.mu.RUnlock()

	if !ok {
		return nil
	}

	msg, err := NewMessage(msgType, payload)
	if err != nil {
		return err
	}

	data, err := msg.Marshal()
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(data); err != nil {
		return err
	}

	gatewayMessagesSent.WithLabelValues(msgType).Inc()
	return nil
}

func (h *Hub) BroadcastToOrg(orgID string, msgType string, payload interface{}) {
	h.mu.RLock()
	orgAgents, ok := h.byOrg[orgID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	conns := make([]*AgentConnection, 0, len(orgAgents))
	for _, conn := range orgAgents {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		msg, err := NewMessage(msgType, payload)
		if err != nil {
			slog.Warn("broadcast message creation failed",
				"org_id", orgID,
				"msg_type", msgType,
				"error", err,
			)
			continue
		}

		data, err := msg.Marshal()
		if err != nil {
			slog.Warn("broadcast message marshal failed",
				"org_id", orgID,
				"msg_type", msgType,
				"error", err,
			)
			continue
		}

		conn.WriteMessage(data)
	}

	gatewayMessagesSent.WithLabelValues(msgType).Add(float64(len(conns)))
}

func (h *Hub) AgentCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.agents)
}

func (h *Hub) GetAgentsByOrg(orgID string) []*AgentConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	orgAgents, ok := h.byOrg[orgID]
	if !ok {
		return nil
	}

	result := make([]*AgentConnection, 0, len(orgAgents))
	for _, conn := range orgAgents {
		result = append(result, conn)
	}
	return result
}

func (h *Hub) HeartbeatCheck(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkHeartbeats()
		case <-ctx.Done():
			return
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *Hub) checkHeartbeats() {
	now := time.Now()

	h.mu.RLock()
	timeouts := make([]*AgentConnection, 0)
	for _, conn := range h.agents {
		if now.Sub(conn.LastPing) > heartbeatTimeout {
			timeouts = append(timeouts, conn)
		}
	}
	h.mu.RUnlock()

	for _, conn := range timeouts {
		slog.Warn("agent heartbeat timeout, disconnecting",
			"agent_id", conn.AgentID,
			"org_id", conn.OrgID,
			"last_ping", conn.LastPing,
		)

		gatewayHeartbeatTimeouts.Inc()

		h.Unregister(conn)
		conn.Close()
	}
}

func (h *Hub) Shutdown() {
	h.cancel()
}

func (h *Hub) MarshalJSON() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := struct {
		AgentCount int                 `json:"agent_count"`
		OrgCount   int                 `json:"org_count"`
		Agents     map[string][]string `json:"agents"`
	}{
		AgentCount: len(h.agents),
		OrgCount:   len(h.byOrg),
		Agents:     make(map[string][]string),
	}

	for orgID, orgAgents := range h.byOrg {
		ids := make([]string, 0, len(orgAgents))
		for agentID := range orgAgents {
			ids = append(ids, agentID)
		}
		stats.Agents[orgID] = ids
	}

	return json.Marshal(stats)
}
