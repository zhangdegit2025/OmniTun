package tracing

import (
	"context"
	"testing"
	"time"

	omnilog "github.com/omnitun/omnitun/pkg/log"
)

func TestInit(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: false})
	if enabled {
		t.Fatal("tracing should be disabled")
	}

	Init(Config{ServiceName: "test", Enabled: true})
	if !enabled {
		t.Fatal("tracing should be enabled")
	}
}

func TestStartSpanDisabled(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: false})
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test.operation")
	if span != nil {
		t.Fatal("span should be nil when tracing disabled")
	}
	if newCtx != ctx {
		t.Fatal("context should be unchanged when tracing disabled")
	}
}

func TestStartSpanAndEnd(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: true})
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test.operation")
	if span == nil {
		t.Fatal("span should not be nil")
	}
	if span.Name != "test.operation" {
		t.Fatalf("span name mismatch: got %s", span.Name)
	}
	if span.TraceID == "" {
		t.Fatal("trace_id should not be empty")
	}
	if span.SpanID == "" {
		t.Fatal("span_id should not be empty")
	}

	time.Sleep(10 * time.Millisecond)
	span.End(ctx)
}

func TestStartSpanPropagatesTraceID(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: true})
	ctx := context.Background()

	ctx, span1 := StartSpan(ctx, "parent.operation")
	if span1 == nil {
		t.Fatal("span1 should not be nil")
	}
	traceID1 := span1.TraceID

	ctx2, span2 := StartSpan(ctx, "child.operation")
	if span2 == nil {
		t.Fatal("span2 should not be nil")
	}

	if span2.TraceID != traceID1 {
		t.Fatalf("child trace_id should match parent: got %s, want %s", span2.TraceID, traceID1)
	}

	if span2.SpanID == span1.SpanID {
		t.Fatal("child span_id should differ from parent")
	}

	_ = ctx2
	span2.End(ctx)
	span1.End(ctx)
}

func TestSpanEndNil(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: true})
	var span *Span
	span.End(context.Background())
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if len(id1) != 16 {
		t.Fatalf("expected id length 16, got %d", len(id1))
	}
	if id1 == id2 {
		t.Fatal("generated IDs should be unique")
	}
}

func TestTraceIDInContext(t *testing.T) {
	Init(Config{ServiceName: "test", Enabled: true})
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "operation")
	if span == nil {
		t.Fatal("span should not be nil")
	}

	traceID := omnilog.TraceID(ctx)
	if traceID == "" {
		t.Fatal("trace_id should be present in context")
	}

	span.End(ctx)
}
