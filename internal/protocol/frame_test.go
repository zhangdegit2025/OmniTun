package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		f    *Frame
	}{
		{
			name: "empty data frame",
			f:    NewDataFrame(1, nil, false),
		},
		{
			name: "data frame with payload",
			f:    NewDataFrame(42, []byte("hello, vector stream"), false),
		},
		{
			name: "data frame compressed",
			f:    NewDataFrame(7, []byte("compressed payload"), true),
		},
		{
			name: "data frame with EOF",
			f: func() *Frame {
				f := NewDataFrame(99, []byte("final chunk"), false)
				f.SetEOF()
				return f
			}(),
		},
		{
			name: "control frame",
			f:    NewControlFrame(ControlTunnelConnect, 0),
		},
		{
			name: "control frame with stream",
			f:    NewControlFrame(ControlTunnelClose, 13),
		},
		{
			name: "ping frame",
			f:    NewPingFrame(8),
		},
		{
			name: "error frame with message",
			f:    NewErrorFrame(ErrorAuthFailed, "invalid credentials"),
		},
		{
			name: "error frame empty message",
			f:    NewErrorFrame(ErrorUnknown, ""),
		},
		{
			name: "max payload frame",
			f: func() *Frame {
				payload := make([]byte, MaxPayload)
				for i := range payload {
					payload[i] = byte(i % 256)
				}
				return NewDataFrame(1, payload, false)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := EncodeFrame(&buf, tt.f); err != nil {
				t.Fatalf("EncodeFrame error: %v", err)
			}

			decoded, err := DecodeFrame(&buf)
			if err != nil {
				t.Fatalf("DecodeFrame error: %v", err)
			}

			if decoded.Version != tt.f.Version {
				t.Errorf("Version: got %d, want %d", decoded.Version, tt.f.Version)
			}
			if decoded.Type != tt.f.Type {
				t.Errorf("Type: got %d, want %d", decoded.Type, tt.f.Type)
			}
			if decoded.Flags != tt.f.Flags {
				t.Errorf("Flags: got %d, want %d", decoded.Flags, tt.f.Flags)
			}
			if decoded.StreamID != tt.f.StreamID {
				t.Errorf("StreamID: got %d, want %d", decoded.StreamID, tt.f.StreamID)
			}
			if !bytes.Equal(decoded.Payload, tt.f.Payload) {
				t.Errorf("Payload mismatch: got len=%d, want len=%d", len(decoded.Payload), len(tt.f.Payload))
			}
		})
	}
}

func TestFrameTypes(t *testing.T) {
	t.Run("DataFrame", func(t *testing.T) {
		f := NewDataFrame(10, []byte("data"), false)
		if f.Type != FrameTypeData {
			t.Errorf("expected FrameTypeData, got %d", f.Type)
		}
		if f.IsCompressed() {
			t.Error("expected not compressed")
		}
		if f.IsEOF() {
			t.Error("expected not EOF")
		}
	})

	t.Run("DataFrameCompressed", func(t *testing.T) {
		f := NewDataFrame(10, []byte("data"), true)
		if !f.IsCompressed() {
			t.Error("expected compressed")
		}
	})

	t.Run("ControlFrame", func(t *testing.T) {
		f := NewControlFrame(ControlTunnelConnect, 5)
		if f.Type != FrameTypeControl {
			t.Errorf("expected FrameTypeControl, got %d", f.Type)
		}
		if sub := f.ControlSubType(); sub != ControlTunnelConnect {
			t.Errorf("ControlSubType: got %d, want %d", sub, ControlTunnelConnect)
		}
		if f.ErrorMessage() != "" {
			t.Error("Control frame should return empty error message")
		}
	})

	t.Run("PingFrame", func(t *testing.T) {
		f := NewPingFrame(3)
		if f.Type != FrameTypePing {
			t.Errorf("expected FrameTypePing, got %d", f.Type)
		}
		if len(f.Payload) != 0 {
			t.Errorf("Ping frame should have empty payload, got %d bytes", len(f.Payload))
		}
	})

	t.Run("ErrorFrame", func(t *testing.T) {
		msg := "authentication failed"
		f := NewErrorFrame(ErrorAuthFailed, msg)
		if f.Type != FrameTypeError {
			t.Errorf("expected FrameTypeError, got %d", f.Type)
		}
		if sub := f.ControlSubType(); sub != ErrorAuthFailed {
			t.Errorf("ControlSubType: got %d, want %d", sub, ErrorAuthFailed)
		}
		if em := f.ErrorMessage(); em != msg {
			t.Errorf("ErrorMessage: got %q, want %q", em, msg)
		}
	})
}

func TestMaxPayload(t *testing.T) {
	t.Run("payload exactly at limit", func(t *testing.T) {
		f := NewDataFrame(1, make([]byte, MaxPayload), false)
		var buf bytes.Buffer
		if err := EncodeFrame(&buf, f); err != nil {
			t.Fatalf("payload at limit should succeed: %v", err)
		}
	})

	t.Run("payload exceeds limit", func(t *testing.T) {
		f := NewDataFrame(1, make([]byte, MaxPayload+1), false)
		var buf bytes.Buffer
		err := EncodeFrame(&buf, f)
		if err == nil {
			t.Fatal("expected error for oversized payload")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("decode with oversized length", func(t *testing.T) {
		header := make([]byte, HeaderSize)
		binary.BigEndian.PutUint16(header[0:2], MagicValue)
		header[2] = FrameVersion
		header[3] = FrameTypeData
		header[4] = 0
		binary.BigEndian.PutUint64(header[5:13], 1)
		binary.BigEndian.PutUint32(header[13:17], MaxPayload+1)

		buf := bytes.NewReader(header)
		_, err := DecodeFrame(buf)
		if err == nil {
			t.Fatal("expected error for oversized length in header")
		}
	})
}

func TestFrameFlags(t *testing.T) {
	t.Run("compressed flag", func(t *testing.T) {
		f := NewDataFrame(1, []byte("x"), true)
		if !f.IsCompressed() {
			t.Error("expected compressed flag")
		}

		var buf bytes.Buffer
		if err := EncodeFrame(&buf, f); err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := DecodeFrame(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !decoded.IsCompressed() {
			t.Error("round-trip: expected compressed flag")
		}
	})

	t.Run("EOF flag", func(t *testing.T) {
		f := NewDataFrame(1, []byte("x"), false)
		f.SetEOF()
		if !f.IsEOF() {
			t.Error("expected EOF flag")
		}

		var buf bytes.Buffer
		if err := EncodeFrame(&buf, f); err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := DecodeFrame(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !decoded.IsEOF() {
			t.Error("round-trip: expected EOF flag")
		}
	})

	t.Run("both flags", func(t *testing.T) {
		f := NewDataFrame(1, []byte("x"), true)
		f.SetEOF()
		if !f.IsCompressed() || !f.IsEOF() {
			t.Error("expected both flags set")
		}

		var buf bytes.Buffer
		if err := EncodeFrame(&buf, f); err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := DecodeFrame(&buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !decoded.IsCompressed() || !decoded.IsEOF() {
			t.Error("round-trip: expected both flags set")
		}
	})

	t.Run("no flags", func(t *testing.T) {
		f := NewDataFrame(1, []byte("x"), false)
		if f.IsCompressed() || f.IsEOF() {
			t.Error("expected no flags")
		}
	})
}

func TestInvalidMagic(t *testing.T) {
	header := make([]byte, HeaderSize)
	header[0] = 0xDE
	header[1] = 0xAD
	header[2] = FrameVersion
	header[3] = FrameTypeData

	buf := bytes.NewReader(header)
	_, err := DecodeFrame(buf)
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
	if !errors.Is(err, ErrInvalidMagic) {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	header := make([]byte, HeaderSize)
	binary.BigEndian.PutUint16(header[0:2], MagicValue)
	header[2] = 0xFF
	header[3] = FrameTypeData

	buf := bytes.NewReader(header)
	_, err := DecodeFrame(buf)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported frame version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShortRead(t *testing.T) {
	t.Run("truncated header", func(t *testing.T) {
		buf := bytes.NewReader([]byte{0x4F, 0x54, 0x01})
		_, err := DecodeFrame(buf)
		if err == nil {
			t.Fatal("expected error for truncated header")
		}
	})

	t.Run("truncated payload", func(t *testing.T) {
		var buf bytes.Buffer
		header := make([]byte, HeaderSize)
		binary.BigEndian.PutUint16(header[0:2], MagicValue)
		header[2] = FrameVersion
		header[3] = FrameTypeData
		header[4] = 0
		binary.BigEndian.PutUint64(header[5:13], 1)
		binary.BigEndian.PutUint32(header[13:17], 100)

		buf.Write(header)
		_, err := DecodeFrame(&buf)
		if err == nil {
			t.Fatal("expected error for truncated payload")
		}
	})
}

func TestStreamIDBoundaries(t *testing.T) {
	tests := []uint64{
		0,
		1,
		0xFFFFFFFF,
		0xFFFFFFFFFFFFFFFF,
	}

	for _, id := range tests {
		t.Run(fmt.Sprintf("streamID=%d", id), func(t *testing.T) {
			f := NewDataFrame(id, []byte("payload"), false)
			var buf bytes.Buffer
			if err := EncodeFrame(&buf, f); err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := DecodeFrame(&buf)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if decoded.StreamID != id {
				t.Errorf("StreamID: got %d, want %d", decoded.StreamID, id)
			}
		})
	}
}

func TestEncodeDecodeMultipleFrames(t *testing.T) {
	frames := []*Frame{
		NewDataFrame(1, []byte("first"), false),
		NewDataFrame(2, []byte("second"), true),
		NewControlFrame(ControlTunnelConnect, 0),
		NewPingFrame(5),
		NewErrorFrame(ErrorAuthFailed, "oauth token expired"),
	}

	var buf bytes.Buffer
	for _, f := range frames {
		if err := EncodeFrame(&buf, f); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}

	reader := bytes.NewReader(buf.Bytes())
	for i, want := range frames {
		got, err := DecodeFrame(reader)
		if err != nil {
			t.Fatalf("decode frame %d: %v", i, err)
		}
		if got.StreamID != want.StreamID || got.Type != want.Type {
			t.Errorf("frame %d mismatch: StreamID=%d Type=%d, want StreamID=%d Type=%d",
				i, got.StreamID, got.Type, want.StreamID, want.Type)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("frame %d payload mismatch", i)
		}
	}
}

func TestControlSubTypeOnNonControlFrame(t *testing.T) {
	f := NewDataFrame(1, []byte{0x01, 0x02}, false)
	if sub := f.ControlSubType(); sub != 0 {
		t.Errorf("data frame ControlSubType should return 0, got %d", sub)
	}
}

func TestErrorMessageOnNonErrorFrame(t *testing.T) {
	f := NewDataFrame(1, []byte("not an error"), false)
	if msg := f.ErrorMessage(); msg != "" {
		t.Errorf("data frame ErrorMessage should return empty, got %q", msg)
	}
}
