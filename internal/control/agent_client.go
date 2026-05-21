package control

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type AgentClient struct {
	cfg         *AgentClientConfig
	wsConn      *websocket.Conn
	tunnelConns map[string]*TunnelConnection
	authToken   string
	mu          sync.RWMutex
	reconnectCh chan struct{}
	quit        chan struct{}
	done        chan struct{}
	connected   bool
	handler     MessageHandler
}

type AgentClientConfig struct {
	ControlWSURL string
	AgentID      string
	Token        string
	Version      string
}

type MessageHandler interface {
	OnTunnelStartAck(msg *TunnelStartAck)
	OnTunnelStopCmd(msg *TunnelStopCmd)
	OnTunnelConfig(msg *TunnelConfig)
	OnTunnelUpdate(msg *TunnelUpdate)
	OnShutdown(msg *ShutdownMessage)
}

func NewAgentClient(cfg *AgentClientConfig, handler MessageHandler) *AgentClient {
	if cfg.AgentID == "" {
		cfg.AgentID = generateAgentID()
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	return &AgentClient{
		cfg:         cfg,
		tunnelConns: make(map[string]*TunnelConnection),
		reconnectCh: make(chan struct{}, 1),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
		handler:     handler,
	}
}

func (a *AgentClient) Connect(ctx context.Context) error {
	a.mu.Lock()
	if a.connected {
		a.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	a.mu.Unlock()

	if err := a.dial(ctx); err != nil {
		return err
	}

	go a.heartbeatLoop()
	go a.reconnectLoop()
	go a.messageLoop()

	return nil
}

func (a *AgentClient) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return nil
	}

	close(a.quit)
	<-a.done

	if a.wsConn != nil {
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "agent shutting down")
		a.wsConn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
		a.wsConn.Close()
		a.wsConn = nil
	}

	for _, tc := range a.tunnelConns {
		tc.Close()
	}
	a.tunnelConns = make(map[string]*TunnelConnection)
	a.connected = false

	return nil
}

func (a *AgentClient) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

func (a *AgentClient) AddTunnelConnection(tc *TunnelConnection) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tunnelConns[tc.TunnelID] = tc
}

func (a *AgentClient) RemoveTunnelConnection(tunnelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if tc, ok := a.tunnelConns[tunnelID]; ok {
		tc.Close()
		delete(a.tunnelConns, tunnelID)
	}
}

func (a *AgentClient) dial(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  nil,
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+a.cfg.Token)
	headers.Set("User-Agent", "OmniTun-Agent/"+a.cfg.Version)

	conn, _, err := dialer.DialContext(ctx, a.cfg.ControlWSURL, headers)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	hello := NewHelloMessage(a.cfg.AgentID, a.cfg.Version, a.cfg.Token)
	if err := conn.WriteJSON(hello); err != nil {
		conn.Close()
		return fmt.Errorf("send hello failed: %w", err)
	}

	a.mu.Lock()
	if a.wsConn != nil {
		a.wsConn.Close()
	}
	a.wsConn = conn
	a.connected = true
	a.mu.Unlock()

	return nil
}

func (a *AgentClient) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	failCount := 0
	const maxFailures = 3

	for {
		select {
		case <-a.quit:
			return
		case <-ticker.C:
			a.mu.RLock()
			conn := a.wsConn
			a.mu.RUnlock()

			if conn == nil {
				failCount++
				if failCount >= maxFailures {
					select {
					case a.reconnectCh <- struct{}{}:
					default:
					}
				}
				continue
			}

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				failCount++
				if failCount >= maxFailures {
					a.mu.Lock()
					a.connected = false
					a.mu.Unlock()
					select {
					case a.reconnectCh <- struct{}{}:
					default:
					}
				}
			} else {
				failCount = 0
			}
		}
	}
}

func (a *AgentClient) reconnectLoop() {
	backoff := time.Second
	const maxBackoff = 60 * time.Second
	const jitterFactor = 0.25

	for {
		select {
		case <-a.quit:
			return
		case <-a.reconnectCh:
		}

		a.mu.Lock()
		if a.wsConn != nil {
			a.wsConn.Close()
			a.wsConn = nil
		}
		a.connected = false
		a.mu.Unlock()

		for {
			select {
			case <-a.quit:
				return
			default:
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := a.dial(ctx)
			cancel()
			if err == nil {
				backoff = time.Second
				break
			}

			jitterMs := int64(float64(backoff.Milliseconds()) * jitterFactor)
			jitter, _ := rand.Int(rand.Reader, big.NewInt(jitterMs*2+1))
			actualBackoff := backoff + time.Duration(jitter.Int64()-jitterMs)*time.Millisecond

			select {
			case <-a.quit:
				return
			case <-time.After(actualBackoff):
			}

			backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
		}
	}
}

func (a *AgentClient) messageLoop() {
	defer close(a.done)

	for {
		select {
		case <-a.quit:
			return
		default:
		}

		a.mu.RLock()
		conn := a.wsConn
		a.mu.RUnlock()

		if conn == nil {
			time.Sleep(time.Second)
			continue
		}

		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			a.mu.Lock()
			a.connected = false
			a.mu.Unlock()
			select {
			case a.reconnectCh <- struct{}{}:
			default:
			}
			time.Sleep(time.Second)
			continue
		}

		var base WSMessage
		if err := json.Unmarshal(rawMsg, &base); err != nil {
			continue
		}

		switch base.Type {
		case MsgTunnelStartAck:
			var msg TunnelStartAck
			if json.Unmarshal(rawMsg, &msg) == nil && a.handler != nil {
				a.handler.OnTunnelStartAck(&msg)
			}
		case MsgTunnelStopCmd:
			var msg TunnelStopCmd
			if json.Unmarshal(rawMsg, &msg) == nil && a.handler != nil {
				a.handler.OnTunnelStopCmd(&msg)
			}
		case MsgTunnelConfig:
			var msg TunnelConfig
			if json.Unmarshal(rawMsg, &msg) == nil && a.handler != nil {
				a.handler.OnTunnelConfig(&msg)
			}
		case MsgTunnelUpdate:
			var msg TunnelUpdate
			if json.Unmarshal(rawMsg, &msg) == nil && a.handler != nil {
				a.handler.OnTunnelUpdate(&msg)
			}
		case MsgServerShutdown:
			var msg ShutdownMessage
			if json.Unmarshal(rawMsg, &msg) == nil && a.handler != nil {
				a.handler.OnShutdown(&msg)
			}
		}
	}
}

func generateAgentID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
