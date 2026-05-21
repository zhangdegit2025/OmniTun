package relay

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/omnitun/omnitun/internal/protocol"
)

type StreamMultiplexer struct {
	activeStreams map[uint64]*StreamConnection
	nextStreamID  uint64
	mu            sync.RWMutex
}

type StreamConnection struct {
	StreamID uint64
	TunnelID string
	Conn     io.ReadWriteCloser
	created  time.Time
}

func NewStreamMultiplexer() *StreamMultiplexer {
	return &StreamMultiplexer{
		activeStreams: make(map[uint64]*StreamConnection),
		nextStreamID:  1,
	}
}

func (sm *StreamMultiplexer) NewStream(tunnelID string, conn io.ReadWriteCloser) *StreamConnection {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	id := sm.nextStreamID
	sm.nextStreamID++

	sc := &StreamConnection{
		StreamID: id,
		TunnelID: tunnelID,
		Conn:     conn,
		created:  time.Now(),
	}
	sm.activeStreams[id] = sc

	slog.Info("stream created",
		"stream_id", id,
		"tunnel_id", tunnelID,
	)

	return sc
}

func (sm *StreamMultiplexer) CloseStream(streamID uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sc, ok := sm.activeStreams[streamID]
	if !ok {
		return
	}

	if err := sc.Conn.Close(); err != nil {
		slog.Warn("error closing stream",
			"stream_id", streamID,
			"error", err,
		)
	}

	delete(sm.activeStreams, streamID)
	slog.Info("stream closed", "stream_id", streamID)
}

func (sm *StreamMultiplexer) Forward(streamID uint64, payload []byte) error {
	sm.mu.RLock()
	sc, ok := sm.activeStreams[streamID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	frame := protocol.NewDataFrame(streamID, payload, true)
	return protocol.EncodeFrame(sc.Conn, frame)
}

func (sm *StreamMultiplexer) ForwardFrame(streamID uint64, frame *protocol.Frame) error {
	sm.mu.RLock()
	sc, ok := sm.activeStreams[streamID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	return protocol.EncodeFrame(sc.Conn, frame)
}

func (sm *StreamMultiplexer) Receive(streamID uint64) (*protocol.Frame, error) {
	sm.mu.RLock()
	sc, ok := sm.activeStreams[streamID]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("stream %d not found", streamID)
	}

	return protocol.DecodeFrame(sc.Conn)
}

func (sm *StreamMultiplexer) StreamCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.activeStreams)
}

func (sm *StreamMultiplexer) GetStream(streamID uint64) (*StreamConnection, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sc, ok := sm.activeStreams[streamID]
	return sc, ok
}
