package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	omnilog "github.com/omnitun/omnitun/pkg/log"
)

type Config struct {
	ServiceName string
	Enabled     bool
}

var enabled bool

func Init(cfg Config) {
	enabled = cfg.Enabled
}

type Span struct {
	TraceID   string
	SpanID    string
	Name      string
	StartTime time.Time
}

func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	if !enabled {
		return ctx, nil
	}

	traceID := omnilog.TraceID(ctx)
	if traceID == "" {
		traceID = generateID()
		ctx = omnilog.WithTraceID(ctx, traceID)
	}

	spanID := generateID()
	span := &Span{
		TraceID:   traceID,
		SpanID:    spanID,
		Name:      name,
		StartTime: time.Now(),
	}

	slog.InfoContext(ctx, "span.start",
		"name", name,
		"trace_id", traceID,
		"span_id", spanID,
	)

	return ctx, span
}

func (s *Span) End(ctx context.Context) {
	if s == nil {
		return
	}

	duration := time.Since(s.StartTime)
	slog.InfoContext(ctx, "span.end",
		"name", s.Name,
		"trace_id", s.TraceID,
		"span_id", s.SpanID,
		"duration_ms", duration.Milliseconds(),
	)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
