package control

type WSMessage struct {
	Type string `json:"type"`
}

type HelloMessage struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Version string `json:"version"`
	Token   string `json:"token"`
}

type HeartbeatMessage struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
}

type TunnelStartAck struct {
	Type         string `json:"type"`
	TunnelID     string `json:"tunnel_id"`
	RelayAddress string `json:"relay_address"`
	Token        string `json:"token"`
}

type TunnelStopCmd struct {
	Type     string `json:"type"`
	TunnelID string `json:"tunnel_id"`
}

type TunnelConfig struct {
	Type     string            `json:"type"`
	TunnelID string            `json:"tunnel_id"`
	Config   map[string]string `json:"config"`
}

type TunnelUpdate struct {
	Type      string `json:"type"`
	TunnelID  string `json:"tunnel_id"`
	LocalHost string `json:"local_host,omitempty"`
	LocalPort int    `json:"local_port,omitempty"`
}

type ShutdownMessage struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type RelayFailoverMessage struct {
	Type          string `json:"type"`
	AgentID       string `json:"agent_id"`
	FailedRelayID string `json:"failed_relay_id"`
	Region        string `json:"region"`
}

type RelayFailoverAck struct {
	Type         string `json:"type"`
	AgentID      string `json:"agent_id"`
	NewRelayID   string `json:"new_relay_id"`
	RelayAddress string `json:"relay_address"`
	Token        string `json:"token"`
}

const (
	MsgAgentHello       = "agent.hello"
	MsgAgentHeartbeat   = "agent.heartbeat"
	MsgTunnelStartAck   = "server.tunnel.start_ack"
	MsgTunnelStopCmd    = "server.tunnel.stop_cmd"
	MsgTunnelConfig     = "server.tunnel.config"
	MsgTunnelUpdate     = "server.tunnel.update"
	MsgServerShutdown   = "server.shutdown"
	MsgRelayFailover    = "agent.relay.failover"
	MsgRelayFailoverAck = "server.relay.failover_ack"
)

func NewHelloMessage(agentID, version, token string) HelloMessage {
	return HelloMessage{
		Type:    MsgAgentHello,
		AgentID: agentID,
		Version: version,
		Token:   token,
	}
}

func NewHeartbeatMessage(agentID string) HeartbeatMessage {
	return HeartbeatMessage{
		Type:    MsgAgentHeartbeat,
		AgentID: agentID,
	}
}
