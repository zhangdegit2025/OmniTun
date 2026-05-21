package audit

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	pool *pgxpool.Pool
}

func NewLogger(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

func (l *Logger) Log(ctx context.Context, event AuditEvent) error {
	_, err := l.pool.Exec(ctx,
		`INSERT INTO audit_logs (organization_id, user_id, action, resource_type, resource_id, details, client_ip, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		event.OrgID, event.UserID, event.Action, event.ResourceType, event.ResourceID, event.Details, event.ClientIP, time.Now(),
	)
	return err
}

type AuditEvent struct {
	OrgID        string `json:"organization_id"`
	UserID       string `json:"user_id"`
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Details      string `json:"details"`
	ClientIP     string `json:"client_ip"`
}
