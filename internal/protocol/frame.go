// Package protocol implements the OmniTun Vector Stream binary frame protocol.
//
// Frame format (17 bytes header + variable payload):
//
//	Magic(2) | Version(1) | Type(1) | Flags(1) | StreamID(8) | Length(4) | Payload(N)
//
// Magic: 0x4F54 ("OT")
// Max payload: 64KB per frame.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// MagicValue is the 2-byte magic number identifying OmniTun frames.
	MagicValue = 0x4F54

	// FrameVersion is the current frame protocol version.
	FrameVersion = 0x01

	// FrameTypeData carries application data payload.
	FrameTypeData = 0x00
	// FrameTypeControl carries a control command (sub-type in first payload byte).
	FrameTypeControl = 0x01
	// FrameTypePing is a keepalive / latency probe frame.
	FrameTypePing = 0x02
	// FrameTypeError carries an error code and human-readable message.
	FrameTypeError = 0x03
)

// Control frame sub-types. The first byte of a Control frame payload
// identifies the specific control operation.
const (
	ControlTunnelConnect    = 0x01
	ControlTunnelConnectAck = 0x02
	ControlTunnelClose      = 0x03
	ControlKeepalive        = 0x04
)

// Error frame sub-types. The first byte of an Error frame payload
// identifies the error category.
const (
	ErrorUnknown        = 0x00
	ErrorAuthFailed     = 0x01
	ErrorTunnelNotFound = 0x02
	ErrorStreamClosed   = 0x03
	ErrorRateLimited    = 0x04
)

// Frame header flags.
const (
	// FlagCompressed indicates the payload is compressed with zstd.
	FlagCompressed = 0x01
	// FlagEOF marks the end of a logical stream.
	FlagEOF = 0x02
)

const (
	// HeaderSize is the fixed frame header length in bytes.
	HeaderSize = 17
	// MaxPayload is the maximum allowed payload length per frame.
	MaxPayload = 64 * 1024
)

// Common protocol errors returned by EncodeFrame and DecodeFrame.
var (
	ErrPayloadTooLarge       = errors.New("payload exceeds maximum frame size")
	ErrInvalidMagic          = errors.New("invalid magic number")
	ErrUnsupportedVersion    = errors.New("unsupported frame version")
	ErrPayloadLengthMismatch = errors.New("payload length exceeds maximum")
)

// Frame represents a single Vector Stream protocol frame.
type Frame struct {
	Version  uint8
	Type     uint8
	Flags    uint8
	StreamID uint64
	Payload  []byte
}

// EncodeFrame serialises f and writes it to w.
// It returns an error if the payload exceeds MaxPayload or the write fails.
func EncodeFrame(w io.Writer, f *Frame) error {
	if len(f.Payload) > MaxPayload {
		return fmt.Errorf("%w: %d bytes", ErrPayloadTooLarge, len(f.Payload))
	}

	header := make([]byte, HeaderSize)
	binary.BigEndian.PutUint16(header[0:2], MagicValue)
	header[2] = f.Version
	header[3] = f.Type
	header[4] = f.Flags
	binary.BigEndian.PutUint64(header[5:13], f.StreamID)
	binary.BigEndian.PutUint32(header[13:17], uint32(len(f.Payload)))

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if len(f.Payload) > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return fmt.Errorf("write payload: %w", err)
		}
	}
	return nil
}

// DecodeFrame reads and parses a single frame from r.
// It validates the magic number, version, and payload length bounds.
func DecodeFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	magic := binary.BigEndian.Uint16(header[0:2])
	if magic != MagicValue {
		return nil, ErrInvalidMagic
	}

	version := header[2]
	if version != FrameVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, version, FrameVersion)
	}

	payloadLen := binary.BigEndian.Uint32(header[13:17])
	if payloadLen > MaxPayload {
		return nil, fmt.Errorf("%w: %d bytes", ErrPayloadLengthMismatch, payloadLen)
	}

	f := &Frame{
		Version:  version,
		Type:     header[3],
		Flags:    header[4],
		StreamID: binary.BigEndian.Uint64(header[5:13]),
	}

	if payloadLen > 0 {
		f.Payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
	}

	return f, nil
}

// NewDataFrame creates a DATA frame with the given stream ID and payload.
// If compressed is true, the FlagCompressed bit is set.
func NewDataFrame(streamID uint64, payload []byte, compressed bool) *Frame {
	f := &Frame{
		Version:  FrameVersion,
		Type:     FrameTypeData,
		StreamID: streamID,
		Payload:  payload,
	}
	if compressed {
		f.Flags |= FlagCompressed
	}
	return f
}

// NewControlFrame creates a CONTROL frame with the given sub-type.
// The sub-type is stored as the first byte of the payload.
func NewControlFrame(subType uint8, streamID uint64) *Frame {
	return &Frame{
		Version:  FrameVersion,
		Type:     FrameTypeControl,
		StreamID: streamID,
		Payload:  []byte{subType},
	}
}

// NewPingFrame creates a PING frame for the given stream.
func NewPingFrame(streamID uint64) *Frame {
	return &Frame{
		Version:  FrameVersion,
		Type:     FrameTypePing,
		StreamID: streamID,
	}
}

// NewErrorFrame creates an ERROR frame with a sub-type code and a
// human-readable message. The message is stored as UTF-8 bytes following
// the sub-type byte.
func NewErrorFrame(subType uint8, message string) *Frame {
	payload := make([]byte, 1+len(message))
	payload[0] = subType
	copy(payload[1:], message)
	return &Frame{
		Version: FrameVersion,
		Type:    FrameTypeError,
		Payload: payload,
	}
}

// IsCompressed reports whether the frame payload is compressed.
func (f *Frame) IsCompressed() bool {
	return f.Flags&FlagCompressed != 0
}

// IsEOF reports whether this frame marks the end of the logical stream.
func (f *Frame) IsEOF() bool {
	return f.Flags&FlagEOF != 0
}

// ControlSubType returns the control sub-type for CONTROL and ERROR frames.
// The sub-type is the first byte of the payload. Returns 0 if the frame
// is not a CONTROL or ERROR frame, or if the payload is empty.
func (f *Frame) ControlSubType() uint8 {
	if f.Type != FrameTypeControl && f.Type != FrameTypeError {
		return 0
	}
	if len(f.Payload) == 0 {
		return 0
	}
	return f.Payload[0]
}

// ErrorMessage returns the human-readable message from an ERROR frame.
// Returns an empty string if the frame is not an ERROR frame or has no message.
func (f *Frame) ErrorMessage() string {
	if f.Type != FrameTypeError || len(f.Payload) <= 1 {
		return ""
	}
	return string(f.Payload[1:])
}

// SetEOF sets the EOF flag on the frame.
func (f *Frame) SetEOF() {
	f.Flags |= FlagEOF
}
