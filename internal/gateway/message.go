package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	MsgAgentHello        = "agent.hello"
	MsgAgentTunnelStart  = "agent.tunnel.start"
	MsgAgentTunnelStop   = "agent.tunnel.stop"
	MsgAgentTunnelStatus = "agent.tunnel.status"
	MsgAgentTunnelError  = "agent.tunnel.error"
	MsgAgentNetworkJoin  = "agent.network.join"
	MsgAgentNetworkLeave = "agent.network.leave"
	MsgAgentStunResult   = "agent.stun.result"

	MsgServerTunnelConfig   = "server.tunnel.config"
	MsgServerTunnelStartAck = "server.tunnel.start_ack"
	MsgServerTunnelStopCmd  = "server.tunnel.stop_cmd"
	MsgServerTunnelUpdate   = "server.tunnel.update"
	MsgServerShutdown       = "server.shutdown"
	MsgServerWelcome        = "server.welcome"
	MsgServerError          = "server.error"
)

type WSMessage struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type HelloPayload struct {
	Token     string `json:"token"`
	AgentID   string `json:"agent_id"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname,omitempty"`
	OSType    string `json:"os_type,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

type WelcomePayload struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
	ServerVer string `json:"server_version"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type TunnelStartPayload struct {
	TunnelID   string `json:"tunnel_id"`
	Protocol   string `json:"protocol"`
	LocalHost  string `json:"local_host"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port,omitempty"`
}

type TunnelStopPayload struct {
	TunnelID string `json:"tunnel_id"`
	Reason   string `json:"reason,omitempty"`
}

type TunnelStatusPayload struct {
	TunnelID string `json:"tunnel_id"`
	Status   string `json:"status"`
	BytesIn  int64  `json:"bytes_in,omitempty"`
	BytesOut int64  `json:"bytes_out,omitempty"`
}

type TunnelErrorPayload struct {
	TunnelID string `json:"tunnel_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func NewMessage(msgType string, payload interface{}) (*WSMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = data
	}

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}

	return &WSMessage{
		Type:      msgType,
		ID:        hex.EncodeToString(id),
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}, nil
}

func ParseMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (m *WSMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *WSMessage) UnmarshalPayload(v interface{}) error {
	if len(m.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}
