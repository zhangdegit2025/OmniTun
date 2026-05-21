package relay

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/omnitun/omnitun/internal/protocol"
	"github.com/quic-go/quic-go"
)

var ErrEmptyTunnelID = errors.New("tunnel ID must not be empty")

type Server struct {
	cfg        *Config
	dispatcher *Dispatcher
	proxy      *ReverseProxy
	streamMux  *StreamMultiplexer
	ctlClient  *ControlClient

	tokenValidator *TokenValidator

	trafficLogger *TrafficLogger

	quicServer *quic.Listener
	wsServer   *http.Server
	mu         sync.Mutex
}

type Config struct {
	ListenAddr      string
	Region          string
	ControlAddr     string
	CertificateFile string
	KeyFile         string
	TokenSecret     string
	ClickHouseURL   string
	TrafficLogging  bool
}

func NewServer(cfg *Config) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config must not be nil")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:443"
	}

	d := NewDispatcher()
	sm := NewStreamMultiplexer()

	proxy := NewReverseProxy(d, sm)

	ctlClient, err := NewControlClient(cfg.ControlAddr, cfg.Region, cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("create control client: %w", err)
	}

	secret := cfg.TokenSecret
	if secret == "" {
		return nil, fmt.Errorf("TokenSecret is required. Set in config or via OMNI_RELAY_TOKEN_SECRET env var")
	}

	s := &Server{
		cfg:            cfg,
		dispatcher:     d,
		proxy:          proxy,
		streamMux:      sm,
		ctlClient:      ctlClient,
		tokenValidator: NewTokenValidator(secret, 5*time.Minute),
	}

	if cfg.TrafficLogging && cfg.ClickHouseURL != "" {
		chLogger := slog.With("component", "traffic_logger")
		s.trafficLogger = NewTrafficLogger(chLogger, cfg.ClickHouseURL, cfg.Region)
		proxy.SetTrafficLogger(s.trafficLogger)
		slog.Info("traffic logging enabled", "clickhouse_url", cfg.ClickHouseURL)
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("build tls config: %w", err)
	}

	if err := s.startQUICServer(ctx, tlsConfig); err != nil {
		return fmt.Errorf("start quic server: %w", err)
	}

	s.startWSServer(ctx, tlsConfig)

	if err := s.ctlClient.Register(ctx); err != nil {
		slog.Warn("initial registration had issues, will retry via heartbeat",
			"error", err,
		)
	}

	go s.ctlClient.StartHeartbeat(ctx)

	if err := s.ctlClient.SubscribeConfig(ctx, s.dispatcher.OnConfigUpdate); err != nil {
		slog.Warn("config subscription failed, will use polling fallback",
			"error", err,
		)
	}

	go s.updateMetrics(ctx)

	slog.Info("relay server started",
		"listen_addr", s.cfg.ListenAddr,
		"relay_id", s.ctlClient.RelayID(),
		"region", s.cfg.Region,
	)

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Info("shutting down relay server")

	if s.quicServer != nil {
		if err := s.quicServer.Close(); err != nil {
			slog.Error("error closing QUIC server", "error", err)
		}
	}

	if s.wsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := s.wsServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("error shutting down WS server", "error", err)
		}
	}

	if s.ctlClient != nil {
		if err := s.ctlClient.Close(); err != nil {
			slog.Error("error closing control client", "error", err)
		}
	}

	slog.Info("relay server shutdown complete")
	return nil
}

func (s *Server) buildTLSConfig() (*tls.Config, error) {
	if s.cfg.CertificateFile == "" || s.cfg.KeyFile == "" {
		slog.Warn("no TLS certificate configured, using auto-generated self-signed cert")

		cert, err := generateSelfSignedCert()
		if err != nil {
			return nil, fmt.Errorf("generate self-signed cert: %w", err)
		}

		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}

	cert, err := tls.LoadX509KeyPair(s.cfg.CertificateFile, s.cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func (s *Server) startQUICServer(ctx context.Context, tlsConfig *tls.Config) error {
	addr := s.cfg.ListenAddr
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr %s: %w", addr, err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", addr, err)
	}

	quicListener, err := quic.Listen(udpConn, tlsConfig, &quic.Config{
		MaxIdleTimeout: 120 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
	})
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("quic listen: %w", err)
	}

	s.quicServer = quicListener

	go func() {
		slog.Info("QUIC server listening", "addr", addr)
		for {
			conn, err := quicListener.Accept(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("quic accept error", "error", err)
				continue
			}

			go s.handleQUICConnection(ctx, conn)
		}
	}()

	return nil
}

func (s *Server) handleQUICConnection(ctx context.Context, conn *quic.Conn) {
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		slog.Error("quic accept stream failed", "error", err)
		return
	}

	slog.Info("quic stream accepted",
		"remote_addr", conn.RemoteAddr().String(),
	)

	frame, err := protocol.DecodeFrame(stream)
	if err != nil {
		slog.Error("failed to decode initial frame", "error", err)
		return
	}

	if frame.Type != protocol.FrameTypeControl || frame.ControlSubType() != protocol.ControlTunnelConnect {
		slog.Error("expected TUNNEL_CONNECT as first frame",
			"frame_type", frame.Type,
			"sub_type", frame.ControlSubType(),
		)
		protocol.EncodeFrame(stream, protocol.NewErrorFrame(protocol.ErrorAuthFailed, "expected TUNNEL_CONNECT"))
		return
	}

	tunnelID, token, err := parseTunnelConnectPayload(frame.Payload)
	if err != nil {
		slog.Error("invalid TUNNEL_CONNECT payload", "error", err)
		protocol.EncodeFrame(stream, protocol.NewErrorFrame(protocol.ErrorAuthFailed, err.Error()))
		return
	}

	validatedID, err := s.tokenValidator.Validate(token)
	if err != nil {
		slog.Error("token validation failed",
			"tunnel_id", tunnelID,
			"error", err,
		)
		protocol.EncodeFrame(stream, protocol.NewErrorFrame(protocol.ErrorAuthFailed, "invalid token: "+err.Error()))
		return
	}

	if validatedID != tunnelID {
		slog.Error("token tunnel_id mismatch",
			"declared", tunnelID,
			"validated", validatedID,
		)
		protocol.EncodeFrame(stream, protocol.NewErrorFrame(protocol.ErrorAuthFailed, "tunnel_id mismatch"))
		return
	}

	tunnelCtx := &TunnelContext{
		TunnelID:  tunnelID,
		CreatedAt: time.Now(),
	}

	if err := s.dispatcher.Register(tunnelCtx); err != nil {
		slog.Error("failed to register tunnel", "error", err)
		protocol.EncodeFrame(stream, protocol.NewErrorFrame(protocol.ErrorAuthFailed, "registration failed"))
		return
	}

	smConn := &quicStreamConn{stream: stream, conn: conn}
	sc := s.streamMux.NewStream(tunnelID, smConn)
	tunnelCtx.StreamID = sc.StreamID

	ackPayload := make([]byte, 1+len(tunnelID))
	ackPayload[0] = protocol.ControlTunnelConnectAck
	copy(ackPayload[1:], tunnelID)
	ackFrame := &protocol.Frame{
		Version:  protocol.FrameVersion,
		Type:     protocol.FrameTypeControl,
		StreamID: sc.StreamID,
		Payload:  ackPayload,
	}
	if err := protocol.EncodeFrame(stream, ackFrame); err != nil {
		slog.Error("failed to send TUNNEL_CONNECT_ACK", "error", err)
		return
	}

	slog.Info("tunnel connected and validated",
		"tunnel_id", tunnelID,
		"stream_id", sc.StreamID,
		"remote_addr", conn.RemoteAddr().String(),
	)

	<-ctx.Done()
}

func parseTunnelConnectPayload(payload []byte) (tunnelID, token string, err error) {
	if len(payload) <= 1 {
		return "", "", fmt.Errorf("payload too short")
	}
	data := payload[1:]
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			return string(data[:i]), string(data[i+1:]), nil
		}
	}
	return "", "", fmt.Errorf("missing token separator")
}

func (s *Server) startWSServer(ctx context.Context, tlsConfig *tls.Config) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", s.handleWSConnection)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.proxy.ServeHTTP)

	s.wsServer = &http.Server{
		Addr:      s.cfg.ListenAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	go func() {
		slog.Info("WebSocket/HTTP server listening", "addr", s.cfg.ListenAddr)

		var err error
		if s.cfg.CertificateFile != "" {
			err = s.wsServer.ListenAndServeTLS("", "")
		} else {
			err = s.wsServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("ws/http server error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.wsServer.Shutdown(shutdownCtx)
	}()
}

func (s *Server) handleWSConnection(w http.ResponseWriter, r *http.Request) {
	slog.Debug("websocket agent connection",
		"remote_addr", r.RemoteAddr,
	)

	tunnelID := r.URL.Query().Get("tunnel_id")
	if tunnelID == "" {
		tunnelID = r.Header.Get("X-Tunnel-ID")
	}

	slog.Info("agent websocket connected",
		"tunnel_id", tunnelID,
		"remote_addr", r.RemoteAddr,
	)

	s.proxy.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","relay_id":"%s","tunnels":%d,"streams":%d}`,
		s.ctlClient.RelayID(),
		s.dispatcher.TunnelCount(),
		s.streamMux.StreamCount(),
	)
}

func (s *Server) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			setTunnelsActive(s.dispatcher.TunnelCount())
			setConnectionsActive(s.streamMux.StreamCount())
		}
	}
}

func (s *Server) Dispatcher() *Dispatcher {
	return s.dispatcher
}

func (s *Server) StreamMultiplexer() *StreamMultiplexer {
	return s.streamMux
}

func (s *Server) Proxy() *ReverseProxy {
	return s.proxy
}

type quicStreamConn struct {
	stream *quic.Stream
	conn   *quic.Conn
}

func (c *quicStreamConn) Read(b []byte) (int, error)   { return c.stream.Read(b) }
func (c *quicStreamConn) Write(b []byte) (int, error)  { return c.stream.Write(b) }
func (c *quicStreamConn) Close() error {
	c.stream.CancelRead(0)
	return c.stream.Close()
}

func (c *quicStreamConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *quicStreamConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
