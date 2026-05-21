package network

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

var (
	ErrTURNStopped         = errors.New("TURN relay has been stopped")
	ErrAllocationExists    = errors.New("allocation already exists")
	ErrAllocationNotFound  = errors.New("allocation not found")
	ErrAllocationExpired   = errors.New("allocation expired")
)

type Allocation struct {
	ID          string
	Username    string
	ClientAddr  *net.UDPAddr
	PeerAddrs   map[string]*net.UDPAddr
	CreatedAt   time.Time
	LastActive  time.Time
	BytesSent   uint64
	BytesRecv   uint64
	mu          sync.RWMutex
}

type TURNRelay struct {
	ServerAddr string
	addr       string
	conn       *net.UDPConn
	allocations map[string]*Allocation
	mu          sync.RWMutex
	closed      bool
	ctx         context.Context
	cancel      context.CancelFunc

	allocTTL          time.Duration
	cleanupInterval   time.Duration
}

func NewTURNRelay(addr string) *TURNRelay {
	return &TURNRelay{
		ServerAddr:       addr,
		addr:             addr,
		allocations:      make(map[string]*Allocation),
		allocTTL:         5 * time.Minute,
		cleanupInterval:  30 * time.Second,
	}
}

func (t *TURNRelay) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", t.addr)
	if err != nil {
		return fmt.Errorf("resolve TURN address %s: %w", t.addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen TURN on %s: %w", t.addr, err)
	}

	t.ctx, t.cancel = context.WithCancel(ctx)

	t.mu.Lock()
	t.conn = conn
	t.closed = false
	t.mu.Unlock()

	slog.Info("TURN relay started", "addr", t.addr)

	go t.readLoop()
	go t.cleanupLoop()

	go func() {
		<-ctx.Done()
		t.Stop()
	}()

	return nil
}

func (t *TURNRelay) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTURNStopped
	}

	t.closed = true

	if t.cancel != nil {
		t.cancel()
	}

	if t.conn != nil {
		if err := t.conn.Close(); err != nil {
			slog.Error("error closing TURN relay", "error", err)
			return err
		}
	}

	t.allocations = make(map[string]*Allocation)

	slog.Info("TURN relay stopped")
	return nil
}

func (t *TURNRelay) CreateAllocation(username string) (*Allocation, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, ErrTURNStopped
	}

	for _, a := range t.allocations {
		if a.Username == username {
			a.LastActive = time.Now()
			return a, ErrAllocationExists
		}
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate allocation ID: %w", err)
	}

	alloc := &Allocation{
		ID:         hex.EncodeToString(idBytes),
		Username:   username,
		PeerAddrs:  make(map[string]*net.UDPAddr),
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	t.allocations[alloc.ID] = alloc

	slog.Info("TURN allocation created",
		"alloc_id", alloc.ID,
		"username", username,
	)

	return alloc, nil
}

func (t *TURNRelay) RemoveAllocation(allocID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTURNStopped
	}

	alloc, ok := t.allocations[allocID]
	if !ok {
		return ErrAllocationNotFound
	}

	delete(t.allocations, allocID)

	slog.Info("TURN allocation removed",
		"alloc_id", alloc.ID,
		"username", alloc.Username,
		"bytes_sent", alloc.BytesSent,
		"bytes_recv", alloc.BytesRecv,
	)

	return nil
}

func (t *TURNRelay) GetAllocation(allocID string) (*Allocation, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	alloc, ok := t.allocations[allocID]
	if !ok {
		return nil, ErrAllocationNotFound
	}

	return alloc, nil
}

func (t *TURNRelay) RegisterPeer(allocID string, peerAddr *net.UDPAddr) (string, error) {
	t.mu.RLock()
	alloc, ok := t.allocations[allocID]
	t.mu.RUnlock()

	if !ok {
		return "", ErrAllocationNotFound
	}

	peerKey := peerAddr.String()

	alloc.mu.Lock()
	alloc.PeerAddrs[peerKey] = peerAddr
	alloc.LastActive = time.Now()
	alloc.mu.Unlock()

	return peerKey, nil
}

func (t *TURNRelay) SendTo(allocID string, data []byte, peerAddr *net.UDPAddr) error {
	t.mu.RLock()
	alloc, ok := t.allocations[allocID]
	conn := t.conn
	t.mu.RUnlock()

	if !ok {
		return ErrAllocationNotFound
	}

	if conn == nil {
		return ErrTURNStopped
	}

	wrapped := wrapRelayData(data)

	n, err := conn.WriteToUDP(wrapped, peerAddr)
	if err != nil {
		return fmt.Errorf("send to peer %s: %w", peerAddr.String(), err)
	}

	alloc.mu.Lock()
	alloc.BytesSent += uint64(n)
	alloc.LastActive = time.Now()
	alloc.mu.Unlock()

	return nil
}

func (t *TURNRelay) IsAvailable() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.addr != ""
}

func (t *TURNRelay) ListenAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.conn != nil {
		return t.conn.LocalAddr()
	}
	return nil
}

func (t *TURNRelay) AllocationCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.allocations)
}

func (t *TURNRelay) readLoop() {
	buf := make([]byte, 65535)

	for {
		t.mu.RLock()
		closed := t.closed
		t.mu.RUnlock()

		if closed {
			return
		}

		n, remoteAddr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			t.mu.RLock()
			closed = t.closed
			t.mu.RUnlock()

			if closed {
				return
			}

			if t.ctx != nil && t.ctx.Err() != nil {
				return
			}

			slog.Error("TURN read error", "error", err)
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])
		t.handleIncomingData(data, remoteAddr)
	}
}

func (t *TURNRelay) handleIncomingData(data []byte, remoteAddr *net.UDPAddr) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, alloc := range t.allocations {
		alloc.mu.RLock()
		_, isPeer := alloc.PeerAddrs[remoteAddr.String()]
		hasClient := alloc.ClientAddr != nil && alloc.ClientAddr.String() == remoteAddr.String()
		alloc.mu.RUnlock()

		if isPeer {
			if alloc.ClientAddr != nil {
				relayed := wrapRelayForward(data, remoteAddr)
				if _, err := t.conn.WriteToUDP(relayed, alloc.ClientAddr); err != nil {
					slog.Error("TURN forward peer data to client failed",
						"alloc_id", alloc.ID,
						"peer", remoteAddr.String(),
						"error", err,
					)
				}
				alloc.mu.Lock()
				alloc.BytesRecv += uint64(len(data))
				alloc.LastActive = time.Now()
				alloc.mu.Unlock()
			}
			return
		}

		if hasClient {
			peerAddr := parsePeerFromRelayData(data)
			if peerAddr != nil {
				if _, err := t.conn.WriteToUDP(data, peerAddr); err != nil {
					slog.Error("TURN relay client data to peer failed",
						"alloc_id", alloc.ID,
						"peer", peerAddr.String(),
						"error", err,
					)
				}
				alloc.mu.Lock()
				alloc.BytesSent += uint64(len(data))
				alloc.LastActive = time.Now()
				alloc.mu.Unlock()
			}
			return
		}
	}
}

func (t *TURNRelay) cleanupLoop() {
	ticker := time.NewTicker(t.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.cleanupExpired()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *TURNRelay) cleanupExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for id, alloc := range t.allocations {
		alloc.mu.RLock()
		lastActive := alloc.LastActive
		alloc.mu.RUnlock()

		if now.Sub(lastActive) > t.allocTTL {
			slog.Info("TURN allocation expired",
				"alloc_id", id,
				"username", alloc.Username,
			)
			delete(t.allocations, id)
		}
	}
}

func wrapRelayData(data []byte) []byte {
	wrapped := make([]byte, 2+len(data))
	wrapped[0] = 0x40
	wrapped[1] = 0x01
	copy(wrapped[2:], data)
	return wrapped
}

func wrapRelayForward(data []byte, peer *net.UDPAddr) []byte {
	peerStr := peer.String()
	wrapped := make([]byte, 4+len(peerStr)+len(data))
	wrapped[0] = 0x40
	wrapped[1] = 0x02
	wrapped[2] = byte(len(peerStr) >> 8)
	wrapped[3] = byte(len(peerStr))
	copy(wrapped[4:], peerStr)
	copy(wrapped[4+len(peerStr):], data)
	return wrapped
}

func parsePeerFromRelayData(data []byte) *net.UDPAddr {
	if len(data) < 4 {
		return nil
	}

	if data[0] != 0x40 || data[1] != 0x03 {
		return nil
	}

	peerLen := int(binary.BigEndian.Uint16(data[2:4]))
	if len(data) < 4+peerLen {
		return nil
	}

	peerStr := string(data[4 : 4+peerLen])
	addr, err := net.ResolveUDPAddr("udp", peerStr)
	if err != nil {
		return nil
	}

	return addr
}
