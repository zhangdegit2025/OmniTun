package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

type HolePunchCoordinator struct {
	stunClient *STUNClient
	timeout    time.Duration
}

type PunchResult struct {
	Success      bool
	LocalAddr    string
	RemoteAddr   string
	NATType      NATType
	Duration     time.Duration
	FallbackUsed bool
}

func NewHolePunchCoordinator(stunServer string) *HolePunchCoordinator {
	return &HolePunchCoordinator{
		stunClient: NewSTUNClient(stunServer),
		timeout:    10 * time.Second,
	}
}

func (c *HolePunchCoordinator) Punch(ctx context.Context, localPort int, remoteAddr string) (*PunchResult, error) {
	startTime := time.Now()

	natType, publicAddr, err := c.stunClient.DetectNATType()
	if err != nil {
		natType = NATSymmetric
		publicAddr = "0.0.0.0:0"
	}

	localAddrStr := publicAddr
	if localAddrStr == "" {
		localAddrStr = fmt.Sprintf("0.0.0.0:%d", localPort)
	}

	if natType == NATUnknown {
		return &PunchResult{
			Success:      false,
			NATType:      natType,
			Duration:     time.Since(startTime),
			FallbackUsed: true,
		}, fmt.Errorf("unknown nat type")
	}

	localUDPAddr := fmt.Sprintf("0.0.0.0:%d", localPort)

	conn, err := c.SimultaneousOpen(localUDPAddr, remoteAddr, 10)
	if err != nil {
		slog.Warn("hole punch simultaneous open failed", "error", err)
		return &PunchResult{
			Success:      false,
			LocalAddr:    localAddrStr,
			RemoteAddr:   remoteAddr,
			NATType:      natType,
			Duration:     time.Since(startTime),
			FallbackUsed: true,
		}, err
	}
	conn.Close()

	return &PunchResult{
		Success:      true,
		LocalAddr:    localAddrStr,
		RemoteAddr:   remoteAddr,
		NATType:      natType,
		Duration:     time.Since(startTime),
		FallbackUsed: false,
	}, nil
}

func (c *HolePunchCoordinator) SimultaneousOpen(localAddr, remoteAddr string, attempts int) (*net.UDPConn, error) {
	laddr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve local addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}

	raddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("resolve remote addr: %w", err)
	}

	if attempts < 1 {
		attempts = 10
	}

	receivedCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	var once sync.Once

	go func() {
		buf := make([]byte, 1500)
		for i := 0; i < attempts; i++ {
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, recvAddr, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				continue
			}
			if recvAddr.IP.Equal(raddr.IP) && n > 0 {
				once.Do(func() {
					close(receivedCh)
				})
				return
			}
		}
		once.Do(func() {
			errCh <- fmt.Errorf("hole punch timeout: no packet received from %s", remoteAddr)
		})
	}()

	punchPacket := []byte("OMNITUN_PUNCH")
	for i := 0; i < attempts; i++ {
		select {
		case <-receivedCh:
			return conn, nil
		case e := <-errCh:
			conn.Close()
			return nil, e
		case <-time.After(100 * time.Millisecond):
			conn.WriteToUDP(punchPacket, raddr)
		}
	}

	conn.Close()
	return nil, fmt.Errorf("hole punch failed after %d attempts", attempts)
}

func (c *HolePunchCoordinator) IsPunchable(nat1, nat2 NATType) bool {
	switch {
	case nat1 == NATSymmetric && nat2 == NATSymmetric:
		return false
	case nat1 == NATUnknown || nat2 == NATUnknown:
		return false
	default:
		return true
	}
}

func (c *HolePunchCoordinator) TryHolePunch(ctx context.Context, sourceNAT, targetNAT NATType, sourceAddr, targetAddr string) bool {
	return c.IsPunchable(sourceNAT, targetNAT)
}
