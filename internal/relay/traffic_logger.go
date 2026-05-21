package relay

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/omnitun/omnitun/pkg/clickhouse"
)

type TrafficLogger struct {
	logger *slog.Logger
	ch     *clickhouse.Client
	region string
}

type TrafficEvent struct {
	Timestamp      time.Time
	OrganizationID string
	TunnelID       string
	ConnectionID   string
	Protocol       string
	Direction      string
	Bytes          int64
	Method         string
	Path           string
	StatusCode     int
	ClientIP       string
	ClientCountry  string
	DurationMs     int64
	Error          string
}

func NewTrafficLogger(logger *slog.Logger, clickhouseURL, region string) *TrafficLogger {
	return &TrafficLogger{
		logger: logger,
		ch:     clickhouse.NewClient(clickhouseURL),
		region: region,
	}
}

func (l *TrafficLogger) Log(ctx context.Context, event *TrafficEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	query := fmt.Sprintf(
		`INSERT INTO traffic_events (timestamp, organization_id, tunnel_id, connection_id, protocol, direction, bytes, method, path, status_code, client_ip, client_country, duration_ms, error) VALUES ('%s','%s','%s','%s','%s','%s',%d,'%s','%s',%d,'%s','%s',%d,'%s')`,
		event.Timestamp.UTC().Format("2006-01-02 15:04:05.999"),
		escapeValue(event.OrganizationID),
		escapeValue(event.TunnelID),
		escapeValue(event.ConnectionID),
		escapeValue(event.Protocol),
		escapeValue(event.Direction),
		event.Bytes,
		escapeValue(event.Method),
		escapeValue(event.Path),
		event.StatusCode,
		escapeValue(event.ClientIP),
		escapeValue(event.ClientCountry),
		event.DurationMs,
		escapeValue(event.Error),
	)

	if err := l.ch.Exec(ctx, query); err != nil {
		l.logger.Warn("failed to write traffic event",
			"tunnel_id", event.TunnelID,
			"error", err,
		)
	}
}

func (l *TrafficLogger) QueryLogs(ctx context.Context, tunnelID string, limit int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(
		`SELECT timestamp, tunnel_id, connection_id, protocol, direction, bytes, method, path, status_code, client_ip, client_country, duration_ms, error FROM traffic_events WHERE tunnel_id='%s' ORDER BY timestamp DESC LIMIT %d`,
		escapeValue(tunnelID),
		limit,
	)
	return l.ch.Query(ctx, query)
}

func (l *TrafficLogger) ClickHouseClient() *clickhouse.Client {
	return l.ch
}

func escapeValue(s string) string {
	if s == "" {
		return ""
	}
	result := strings.ReplaceAll(s, "\\", "\\\\")
	result = strings.ReplaceAll(result, "'", "''")
	return result
}
