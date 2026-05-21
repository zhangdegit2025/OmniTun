package network

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

var ErrSTUNServerStopped = errors.New("STUN server has been stopped")

type STUNServer struct {
	addr   string
	conn   *net.UDPConn
	mu     sync.Mutex
	closed bool
}

func NewSTUNServer(addr string) *STUNServer {
	return &STUNServer{
		addr: addr,
	}
}

func (s *STUNServer) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve STUN address %s: %w", s.addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen STUN on %s: %w", s.addr, err)
	}

	s.mu.Lock()
	s.conn = conn
	s.closed = false
	s.mu.Unlock()

	slog.Info("STUN server started", "addr", s.addr)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				s.mu.Lock()
				closed := s.closed
				s.mu.Unlock()

				if closed {
					return
				}

				if ctx.Err() != nil {
					return
				}

				slog.Error("STUN read error", "error", err)
				continue
			}

			data := make([]byte, n)
			copy(data, buf[:n])
			go s.handleBindingRequest(data, remoteAddr)
		}
	}()

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func (s *STUNServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSTUNServerStopped
	}

	s.closed = true

	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			slog.Error("error closing STUN server", "error", err)
			return err
		}
	}

	slog.Info("STUN server stopped")
	return nil
}

func (s *STUNServer) ListenAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return s.conn.LocalAddr()
	}
	return nil
}

func (s *STUNServer) handleBindingRequest(data []byte, clientAddr *net.UDPAddr) {
	if len(data) < 20 {
		slog.Debug("STUN request too short", "len", len(data))
		return
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType != StunBindingRequest {
		slog.Debug("STUN unexpected message type", "type", fmt.Sprintf("0x%04x", msgType))
		return
	}

	magic := binary.BigEndian.Uint32(data[4:8])
	if magic != StunMagicCookie {
		slog.Debug("STUN invalid magic cookie", "cookie", fmt.Sprintf("0x%08x", magic))
		return
	}

	txID := data[8:20]

	resp := s.buildBindingResponse(txID, clientAddr)

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return
	}

	if _, err := conn.WriteToUDP(resp, clientAddr); err != nil {
		slog.Error("STUN failed to send response",
			"client", clientAddr.String(),
			"error", err,
		)
	}
}

func (s *STUNServer) buildBindingResponse(txID []byte, clientAddr *net.UDPAddr) []byte {
	xorAddr := buildXorMappedAddrAttr(clientAddr)

	attrLen := len(xorAddr)
	resp := make([]byte, 20+attrLen)

	binary.BigEndian.PutUint16(resp[0:2], StunBindingSuccess)
	binary.BigEndian.PutUint16(resp[2:4], uint16(attrLen))
	binary.BigEndian.PutUint32(resp[4:8], StunMagicCookie)
	copy(resp[8:20], txID)
	copy(resp[20:], xorAddr)

	return resp
}

func buildXorMappedAddrAttr(addr *net.UDPAddr) []byte {
	ip := addr.IP.To4()
	port := addr.Port

	if ip != nil {
		attrVal := make([]byte, 8)
		attrVal[0] = 0
		attrVal[1] = 0x01

		xport := uint16(port) ^ uint16(StunMagicCookie>>16)
		binary.BigEndian.PutUint16(attrVal[2:4], xport)

		magicBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(magicBytes, StunMagicCookie)

		for i := 0; i < 4; i++ {
			attrVal[4+i] = ip[i] ^ magicBytes[i]
		}

		attr := make([]byte, 4)
		binary.BigEndian.PutUint16(attr[0:2], StunAttrXorMappedAddress)
		binary.BigEndian.PutUint16(attr[2:4], 8)
		return append(attr, attrVal...)
	}

	ip16 := addr.IP.To16()
	attrVal := make([]byte, 20)
	attrVal[0] = 0
	attrVal[1] = 0x02

	xport := uint16(port) ^ uint16(StunMagicCookie>>16)
	binary.BigEndian.PutUint16(attrVal[2:4], xport)

	magicBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(magicBytes, StunMagicCookie)

	xorKey := make([]byte, 16)
	copy(xorKey[0:4], magicBytes)

	for i := 0; i < 16; i++ {
		attrVal[4+i] = ip16[i] ^ xorKey[i]
	}

	attr := make([]byte, 4)
	binary.BigEndian.PutUint16(attr[0:2], StunAttrXorMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 20)
	return append(attr, attrVal...)
}

func (s *STUNServer) String() string {
	return fmt.Sprintf("STUNServer(%s)", s.addr)
}
