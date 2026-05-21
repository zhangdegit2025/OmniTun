package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func captureStderrLogger(t *testing.T, level, format string) (*slog.Logger, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(&buf, opts)
	} else {
		handler = slog.NewTextHandler(&buf, opts)
	}
	return slog.New(handler), &buf
}

func TestNewLogger_JSON_OutputsValidJSON(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "info", "json")

	logger.Info("test message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Fatal("logger produced no output")
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(output), &m); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput: %s", err, output)
	}
}

func TestNewLogger_Text_ContainsFields(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "info", "text")

	logger.Info("hello world", "user_id", "abc123")

	output := buf.String()
	if output == "" {
		t.Fatal("logger produced no output")
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("output missing message: %s", output)
	}
	if !strings.Contains(output, "user_id") {
		t.Errorf("output missing field key: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("output missing field value: %s", output)
	}
}

func TestLogLevel_Enforcement_DebugMessagesHiddenAtInfo(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "info", "text")

	logger.Debug("this should not appear")

	output := buf.String()
	if output != "" {
		t.Errorf("debug message should be hidden at info level: %s", output)
	}
}

func TestLogLevel_DebugLevel_ShowsAllMessages(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "debug", "text")

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Error("debug level should show debug messages")
	}
	if !strings.Contains(output, "info message") {
		t.Error("debug level should show info messages")
	}
}

func TestLogLevel_ErrorLevel_OnlyShowsErrors(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "error", "text")

	logger.Info("should be hidden")
	logger.Error("error is visible")

	output := buf.String()
	if strings.Contains(output, "should be hidden") {
		t.Error("info message should be hidden at error level")
	}
	if !strings.Contains(output, "error is visible") {
		t.Error("error level should show error messages")
	}
}

func TestWithTraceID_InjectsTraceID(t *testing.T) {
	t.Parallel()

	ctx := WithTraceID(context.Background(), "trace-abc-123")

	id := TraceID(ctx)
	if id != "trace-abc-123" {
		t.Errorf("TraceID = %q, want %q", id, "trace-abc-123")
	}
}

func TestTraceID_EmptyContext_ReturnsEmptyString(t *testing.T) {
	t.Parallel()

	id := TraceID(context.Background())
	if id != "" {
		t.Errorf("TraceID should return empty string for empty context, got %q", id)
	}
}

func TestNewLogger_DefaultLevel_OnUnknownInput(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "invalid", "text")

	logger.Info("test with default level")

	output := buf.String()
	if !strings.Contains(output, "test with default level") {
		t.Errorf("default level should be info, output: %s", output)
	}
}

func TestNewLogger_WarnLevel_ShowsWarnAndAbove(t *testing.T) {
	t.Parallel()

	logger, buf := captureStderrLogger(t, "warn", "text")

	logger.Info("info hidden")
	logger.Warn("warn visible")
	logger.Error("error visible")

	output := buf.String()
	if strings.Contains(output, "info hidden") {
		t.Error("info should be hidden at warn level")
	}
	if !strings.Contains(output, "warn visible") {
		t.Error("warn should be visible at warn level")
	}
	if !strings.Contains(output, "error visible") {
		t.Error("error should be visible at warn level")
	}
}

func TestWithTraceID_MultipleValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = WithTraceID(ctx, "first-id")
	ctx = WithTraceID(ctx, "second-id")

	id := TraceID(ctx)
	if id != "second-id" {
		t.Errorf("TraceID = %q, want %q (last set should win)", id, "second-id")
	}
}
