package control

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/omnitun/omnitun/internal/protocol"
)

type TunnelConnection struct {
	TunnelID  string
	RelayAddr string
	Token     string
	localPort int
	localHost string
	quicConn  net.Conn
	wsConn    *websocket.Conn
	cancel    context.CancelFunc
	mu        sync.Mutex
	closed    bool
}

func NewTunnelConnection(tunnelID, relayAddr, token, localHost string, localPort int) *TunnelConnection {
	return &TunnelConnection{
		TunnelID:  tunnelID,
		RelayAddr: relayAddr,
		Token:     token,
		localHost: localHost,
		localPort: localPort,
	}
}

func (tc *TunnelConnection) Establish(ctx context.Context) error {
	if tc.RelayAddr == "" {
		return fmt.Errorf("relay address is empty")
	}

	var err error
	tc.quicConn, err = dialQUIC(ctx, tc.RelayAddr)
	if err != nil {
		tc.wsConn, _, err = websocket.DefaultDialer.DialContext(ctx, tc.RelayAddr, nil)
		if err != nil {
			return fmt.Errorf("failed to establish data channel (QUIC and WS fallback): %w", err)
		}
	}

	if err := tc.sendConnectFrame(); err != nil {
		tc.closeTransport()
		return fmt.Errorf("send TUNNEL_CONNECT failed: %w", err)
	}

	if err := tc.waitConnectAck(ctx); err != nil {
		tc.closeTransport()
		return fmt.Errorf("wait TUNNEL_CONNECT_ACK failed: %w", err)
	}

	return nil
}

func (tc *TunnelConnection) ForwardLoop(ctx context.Context) {
	localAddr := net.JoinHostPort(tc.localHost, fmt.Sprintf("%d", tc.localPort))
	localConn, err := net.DialTimeout("tcp", localAddr, 10*time.Second)
	if err != nil {
		return
	}
	defer localConn.Close()

	ctx, tc.cancel = context.WithCancel(ctx)
	defer tc.cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		tc.forwardRemoteToLocal(ctx, localConn)
	}()

	go func() {
		defer wg.Done()
		tc.forwardLocalToRemote(ctx, localConn)
	}()

	wg.Wait()
}

func (tc *TunnelConnection) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.closed {
		return nil
	}
	tc.closed = true

	if tc.cancel != nil {
		tc.cancel()
	}

	var lastErr error
	if tc.quicConn != nil {
		closeFrame := protocol.NewControlFrame(protocol.ControlTunnelClose, 0)
		protocol.EncodeFrame(tc.quicConn, closeFrame)
		if err := tc.quicConn.Close(); err != nil {
			lastErr = err
		}
		tc.quicConn = nil
	}
	if tc.wsConn != nil {
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "tunnel closed")
		tc.wsConn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
		tc.wsConn.Close()
		tc.wsConn = nil
	}
	return lastErr
}

func (tc *TunnelConnection) localAddr() string {
	return net.JoinHostPort(tc.localHost, fmt.Sprintf("%d", tc.localPort))
}

func (tc *TunnelConnection) sendConnectFrame() error {
	payload := []byte{protocol.ControlTunnelConnect}
	payload = append(payload, []byte(tc.TunnelID+"\n"+tc.Token)...)

	frame := &protocol.Frame{
		Version:  protocol.FrameVersion,
		Type:     protocol.FrameTypeControl,
		StreamID: 0,
		Payload:  payload,
	}

	if tc.quicConn != nil {
		return protocol.EncodeFrame(tc.quicConn, frame)
	}
	if tc.wsConn != nil {
		data := make([]byte, 17+len(payload))
		return tc.encodeAndWriteWS(frame, data)
	}
	return fmt.Errorf("no transport connection available")
}

func (tc *TunnelConnection) waitConnectAck(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		var frame *protocol.Frame
		var err error
		if tc.quicConn != nil {
			frame, err = protocol.DecodeFrame(tc.quicConn)
		} else if tc.wsConn != nil {
			frame, err = tc.readFrameWS()
		}
		if err != nil {
			done <- err
			return
		}
		if frame.Type != protocol.FrameTypeControl || frame.ControlSubType() != protocol.ControlTunnelConnectAck {
			done <- fmt.Errorf("unexpected frame type: %d sub: %d", frame.Type, frame.ControlSubType())
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (tc *TunnelConnection) forwardRemoteToLocal(ctx context.Context, localConn net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var frame *protocol.Frame
		var err error

		if tc.quicConn != nil {
			frame, err = protocol.DecodeFrame(tc.quicConn)
		} else if tc.wsConn != nil {
			frame, err = tc.readFrameWS()
		} else {
			return
		}

		if err != nil {
			return
		}

		if frame.Type == protocol.FrameTypeControl {
			if frame.ControlSubType() == protocol.ControlTunnelClose {
				return
			}
			continue
		}

		if len(frame.Payload) > 0 {
			if _, err := localConn.Write(frame.Payload); err != nil {
				return
			}
		}

		if frame.IsEOF() {
			return
		}
	}
}

func (tc *TunnelConnection) forwardLocalToRemote(ctx context.Context, localConn net.Conn) {
	buf := make([]byte, 32*1024)
	var streamID uint64 = 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := localConn.Read(buf)
		if err != nil {
			if err == io.EOF {
				eofFrame := protocol.NewDataFrame(streamID, nil, false)
				eofFrame.SetEOF()
				tc.writeFrame(eofFrame)
			}
			return
		}

		frame := protocol.NewDataFrame(streamID, buf[:n], false)
		if err := tc.writeFrame(frame); err != nil {
			return
		}
		streamID++
	}
}

func (tc *TunnelConnection) writeFrame(f *protocol.Frame) error {
	if tc.quicConn != nil {
		return protocol.EncodeFrame(tc.quicConn, f)
	}
	if tc.wsConn != nil {
		data := make([]byte, 17+len(f.Payload))
		return tc.encodeAndWriteWS(f, data)
	}
	return fmt.Errorf("no transport connection available")
}

func (tc *TunnelConnection) encodeAndWriteWS(f *protocol.Frame, buf []byte) error {
	_ = buf
	return tc.wsConn.WriteJSON(f)
}

func (tc *TunnelConnection) readFrameWS() (*protocol.Frame, error) {
	var f protocol.Frame
	if err := tc.wsConn.ReadJSON(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (tc *TunnelConnection) closeTransport() {
	if tc.quicConn != nil {
		tc.quicConn.Close()
		tc.quicConn = nil
	}
	if tc.wsConn != nil {
		tc.wsConn.Close()
		tc.wsConn = nil
	}
}

func dialQUIC(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.DialContext(ctx, "udp", addr)
}
