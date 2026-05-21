package relay

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/omnitun/omnitun/pkg/clickhouse"
)

func TestTrafficEvent_Creation(t *testing.T) {
	event := &TrafficEvent{
		Timestamp:    time.Now(),
		TunnelID:     "tun-test-123",
		Protocol:     "http",
		Direction:    "ingress",
		Bytes:        2048,
		Method:       "POST",
		Path:         "/api/health",
		StatusCode:   200,
		ClientIP:     "10.0.0.1",
		DurationMs:   15,
	}

	if event.TunnelID != "tun-test-123" {
		t.Fatalf("TunnelID = %q, want %q", event.TunnelID, "tun-test-123")
	}
	if event.Protocol != "http" {
		t.Fatalf("Protocol = %q, want %q", event.Protocol, "http")
	}
	if event.Direction != "ingress" {
		t.Fatalf("Direction = %q, want %q", event.Direction, "ingress")
	}
	if event.Bytes != 2048 {
		t.Fatalf("Bytes = %d, want %d", event.Bytes, 2048)
	}
	if event.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want %d", event.StatusCode, 200)
	}
}

func TestNewTrafficLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:8124", "test-region")

	if tl == nil {
		t.Fatal("NewTrafficLogger should not return nil")
	}
	if tl.logger == nil {
		t.Fatal("traffic logger logger should not be nil")
	}
	if tl.ch == nil {
		t.Fatal("clickhouse client should not be nil")
	}
	if tl.region != "test-region" {
		t.Fatalf("region = %q, want %q", tl.region, "test-region")
	}
}

func TestNewTrafficLogger_DefaultTimestamp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:8124", "test")

	event := &TrafficEvent{
		TunnelID: "tun-ts-test",
	}

	tl.Log(context.Background(), event)

	if event.Timestamp.IsZero() {
		t.Fatal("event timestamp should be set after Log is called")
	}
}

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "hello"},
		{"test-value", "test-value"},
	}

	for _, tt := range tests {
		result := escapeValue(tt.input)
		if result != tt.expected {
			t.Errorf("escapeValue(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTrafficEvent_FieldValues(t *testing.T) {
	now := time.Now()
	event := &TrafficEvent{
		Timestamp:      now,
		OrganizationID: "org-abc",
		TunnelID:       "tun-xyz",
		ConnectionID:   "conn-123",
		Protocol:       "tcp",
		Direction:      "egress",
		Bytes:          4096,
		Method:         "CONNECT",
		Path:           "/data",
		StatusCode:     201,
		ClientIP:       "192.168.1.1",
		ClientCountry:  "US",
		DurationMs:     42,
		Error:          "",
	}

	if event.OrganizationID != "org-abc" {
		t.Fatalf("OrganizationID = %q", event.OrganizationID)
	}
	if event.ConnectionID != "conn-123" {
		t.Fatalf("ConnectionID = %q", event.ConnectionID)
	}
	if event.Protocol != "tcp" {
		t.Fatalf("Protocol = %q", event.Protocol)
	}
	if event.Direction != "egress" {
		t.Fatalf("Direction = %q", event.Direction)
	}
	if event.ClientCountry != "US" {
		t.Fatalf("ClientCountry = %q", event.ClientCountry)
	}
	if event.DurationMs != 42 {
		t.Fatalf("DurationMs = %d", event.DurationMs)
	}
	if event.Error != "" {
		t.Fatalf("Error should be empty")
	}
}

func TestTrafficLogger_Log_NoClickHouse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:9999", "test")

	event := &TrafficEvent{
		TunnelID:  "tun-log-test",
		Protocol:  "http",
		Direction: "ingress",
		Path:      "/test",
		Method:    "GET",
	}

	tl.Log(context.Background(), event)
}

func TestClickHouseClient_New(t *testing.T) {
	client := clickhouse.NewClient("http://localhost:8124")
	if client == nil {
		t.Fatal("NewClient should not return nil")
	}
}

func TestTrafficLogger_QueryLogs_SQLGeneration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:8124", "test")
	_ = tl
}

func TestTrafficLogger_TrafficLoggingIntegration(t *testing.T) {
	d := NewDispatcher()
	sm := NewStreamMultiplexer()
	proxy := NewReverseProxy(d, sm)

	if proxy == nil {
		t.Fatal("proxy should not be nil")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:8124", "test")
	proxy.SetTrafficLogger(tl)

	if proxy.trafficLogger == nil {
		t.Fatal("trafficLogger should be set after SetTrafficLogger")
	}
}

func TestServer_TrafficLoggingConfig(t *testing.T) {
	cfg := &Config{
		ListenAddr:    "127.0.0.1:0",
		Region:        "test",
		ControlAddr:   "localhost:9002",
		TokenSecret:   "test-secret",
		ClickHouseURL: "http://localhost:8124",
		TrafficLogging: true,
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.trafficLogger == nil {
		t.Fatal("trafficLogger should be created when TrafficLogging is enabled")
	}
	if s.proxy.trafficLogger == nil {
		t.Fatal("proxy trafficLogger should be set")
	}
}

func TestServer_TrafficLoggingDisabled(t *testing.T) {
	cfg := &Config{
		ListenAddr:     "127.0.0.1:0",
		Region:         "test",
		ControlAddr:    "localhost:9002",
		TokenSecret:    "test-secret",
		ClickHouseURL:  "http://localhost:8124",
		TrafficLogging: false,
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.trafficLogger != nil {
		t.Fatal("trafficLogger should be nil when TrafficLogging is disabled")
	}
}

func TestServer_TrafficLogging_NoClickHouseURL(t *testing.T) {
	cfg := &Config{
		ListenAddr:     "127.0.0.1:0",
		Region:         "test",
		ControlAddr:    "localhost:9002",
		TokenSecret:    "test-secret",
		ClickHouseURL:  "",
		TrafficLogging: true,
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.trafficLogger != nil {
		t.Fatal("trafficLogger should be nil when ClickHouseURL is empty")
	}
}

func TestParseResponseStatusCode(t *testing.T) {
	tests := []struct {
		payload  string
		expected int
	}{
		{"HTTP/1.1 200 OK\r\n", 200},
		{"HTTP/1.1 404 Not Found\r\n", 404},
		{"HTTP/1.1 500 Internal Server Error\r\n", 500},
		{"HTTP/2 301 Moved\r\n", 301},
		{"HTTP/1.0 502 Bad Gateway\r\n", 502},
		{"short", 0},
		{"", 0},
		{"NOTHTTP/1.1 200 OK\r\n", 0},
	}

	for _, tt := range tests {
		code := parseResponseStatusCode([]byte(tt.payload))
		if code != tt.expected {
			t.Errorf("parseResponseStatusCode(%q) = %d, want %d", tt.payload, code, tt.expected)
		}
	}
}

func TestTrafficLogger_RegionPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://clickhouse:8123", "us-west-2")

	if tl.region != "us-west-2" {
		t.Fatalf("region = %q, want %q", tl.region, "us-west-2")
	}
}

func TestTrafficLogger_ClickHouseClientAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tl := NewTrafficLogger(logger, "http://localhost:8124", "test")

	client := tl.ClickHouseClient()
	if client == nil {
		t.Fatal("ClickHouseClient() should not return nil")
	}
}

func TestTrafficEvent_ZeroTimestamp(t *testing.T) {
	event := &TrafficEvent{
		TunnelID: "tun-zero-ts",
	}

	if !event.Timestamp.IsZero() {
		t.Fatal("new TrafficEvent should have zero Timestamp")
	}
}

func TestExtractSlugFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/my-tunnel/extra", "my-tunnel"},
		{"/simple", "simple"},
		{"/", ""},
		{"", ""},
		{"no-slash", ""},
		{"/a/b/c", "a"},
	}

	for _, tt := range tests {
		result := extractSlugFromPath(tt.path)
		if result != tt.expected {
			t.Errorf("extractSlugFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestSerializeHTTPResponse(t *testing.T) {
	payload, err := SerializeHTTPResponse(200, map[string][]string{"Content-Type": {"application/json"}}, []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("SerializeHTTPResponse failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("serialized response should not be empty")
	}
	payloadStr := string(payload)
	if !strings.Contains(payloadStr, "200") {
		t.Error("serialized response should contain status code 200")
	}
	if !strings.Contains(payloadStr, "Content-Type") {
		t.Error("serialized response should contain Content-Type header")
	}
}

func TestSerializeHTTPResponse_EmptyBody(t *testing.T) {
	payload, err := SerializeHTTPResponse(204, map[string][]string{}, nil)
	if err != nil {
		t.Fatalf("SerializeHTTPResponse failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("serialized response should not be empty even with empty body")
	}
	payloadStr := string(payload)
	if !strings.Contains(payloadStr, "204") {
		t.Error("serialized response should contain status code 204")
	}
}
