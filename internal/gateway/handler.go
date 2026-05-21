package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func (s *Server) HandleAgentConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err, "remote_addr", r.RemoteAddr)
		gatewayConnectionErrors.Inc()
		return
	}

	gatewayWebSocketConnections.Inc()

	agentConn, err := s.performHelloHandshake(r, conn)
	if err != nil {
		slog.Error("agent hello handshake failed", "error", err, "remote_addr", r.RemoteAddr)
		gatewayConnectionErrors.Inc()

		errorMsg, _ := NewMessage(MsgServerError, ErrorPayload{
			Code:    "AUTH_FAILED",
			Message: err.Error(),
		})
		if data, mErr := errorMsg.Marshal(); mErr == nil {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			conn.WriteMessage(websocket.TextMessage, data)
		}
		conn.Close()
		return
	}

	s.Hub.Register(agentConn)

	welcomeMsg, err := NewMessage(MsgServerWelcome, WelcomePayload{
		AgentID:   agentConn.AgentID,
		SessionID: "",
		ServerVer: "1.0.0",
	})
	if err == nil {
		if data, mErr := welcomeMsg.Marshal(); mErr == nil {
			agentConn.WriteMessage(data)
			gatewayMessagesSent.WithLabelValues(MsgServerWelcome).Inc()
		}
	}

	s.Hub.Register(agentConn)

	slog.Info("agent connected",
		"agent_id", agentConn.AgentID,
		"org_id", agentConn.OrgID,
		"version", agentConn.Version,
		"remote_addr", r.RemoteAddr,
	)

	go agentConn.writePump()
	s.messageLoop(agentConn)
}

func (s *Server) performHelloHandshake(r *http.Request, conn *websocket.Conn) (*AgentConnection, error) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	_, raw, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	msg, err := ParseMessage(raw)
	if err != nil {
		return nil, err
	}

	if msg.Type != MsgAgentHello {
		return nil, fmt.Errorf("expected agent.hello as first message, got %s", msg.Type)
	}

	gatewayMessagesReceived.WithLabelValues(MsgAgentHello).Inc()

	authInfo, err := s.authMgr.AuthenticateHello(r.Context(), msg)
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Time{})

	return s.Hub.newAgentConnection(conn, authInfo), nil
}

func (s *Server) messageLoop(conn *AgentConnection) {
	defer func() {
		s.Hub.Unregister(conn)
		conn.Close()
		conn.Conn.Close()
		slog.Info("agent disconnected", "agent_id", conn.AgentID)
	}()

	for {
		_, raw, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("agent connection error", "agent_id", conn.AgentID, "error", err)
			}
			return
		}

		conn.LastPing = time.Now()

		msg, err := ParseMessage(raw)
		if err != nil {
			slog.Warn("failed to parse agent message", "agent_id", conn.AgentID, "error", err)
			continue
		}

		gatewayMessagesReceived.WithLabelValues(msg.Type).Inc()

		s.dispatchMessage(conn, msg)
	}
}

func (s *Server) dispatchMessage(conn *AgentConnection, msg *WSMessage) {
	switch msg.Type {
	case MsgAgentHello:
		slog.Warn("unexpected duplicate hello message", "agent_id", conn.AgentID)

	case MsgAgentTunnelStart:
		var payload TunnelStartPayload
		if err := msg.UnmarshalPayload(&payload); err != nil {
			slog.Warn("invalid tunnel.start payload", "agent_id", conn.AgentID, "error", err)
			return
		}
		slog.Info("tunnel start request",
			"agent_id", conn.AgentID,
			"tunnel_id", payload.TunnelID,
			"protocol", payload.Protocol,
			"local_host", payload.LocalHost,
			"local_port", payload.LocalPort,
		)

	case MsgAgentTunnelStop:
		var payload TunnelStopPayload
		if err := msg.UnmarshalPayload(&payload); err != nil {
			slog.Warn("invalid tunnel.stop payload", "agent_id", conn.AgentID, "error", err)
			return
		}
		slog.Info("tunnel stop request",
			"agent_id", conn.AgentID,
			"tunnel_id", payload.TunnelID,
			"reason", payload.Reason,
		)

	case MsgAgentTunnelStatus:
		var payload TunnelStatusPayload
		if err := msg.UnmarshalPayload(&payload); err != nil {
			slog.Warn("invalid tunnel.status payload", "agent_id", conn.AgentID, "error", err)
			return
		}
		slog.Debug("tunnel status update",
			"agent_id", conn.AgentID,
			"tunnel_id", payload.TunnelID,
			"status", payload.Status,
			"bytes_in", payload.BytesIn,
			"bytes_out", payload.BytesOut,
		)

	case MsgAgentTunnelError:
		var payload TunnelErrorPayload
		if err := msg.UnmarshalPayload(&payload); err != nil {
			slog.Warn("invalid tunnel.error payload", "agent_id", conn.AgentID, "error", err)
			return
		}
		slog.Error("tunnel error from agent",
			"agent_id", conn.AgentID,
			"tunnel_id", payload.TunnelID,
			"code", payload.Code,
			"message", payload.Message,
		)

	case MsgAgentNetworkJoin:
		slog.Info("agent network join", "agent_id", conn.AgentID)

	case MsgAgentNetworkLeave:
		slog.Info("agent network leave", "agent_id", conn.AgentID)

	case MsgAgentStunResult:
		slog.Debug("agent stun result", "agent_id", conn.AgentID)

	default:
		slog.Warn("unknown message type from agent",
			"agent_id", conn.AgentID,
			"msg_type", msg.Type,
		)
	}
}

func (h *Hub) newAgentConnection(conn *websocket.Conn, authInfo *HelloAuthInfo) *AgentConnection {
	ctx, cancel := context.WithCancel(context.Background())

	return &AgentConnection{
		AgentID:  authInfo.AgentID,
		OrgID:    authInfo.OrgID,
		Conn:     conn,
		Send:     make(chan []byte, sendChannelBuffer),
		Hub:      h,
		LastPing: time.Now(),
		Version:  authInfo.Version,
		ctx:      ctx,
		cancel:   cancel,
	}
}
