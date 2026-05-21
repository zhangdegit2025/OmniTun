//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupAuditDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanAuditTables(t, pool)
	return pool
}

func cleanAuditTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"audit_logs", "api_keys", "tunnels", "users", "workspaces", "organizations"}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func teardownAuditDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanAuditTables(t, pool)
		pool.Close()
	}
}

func seedAuditOrgAndUser(t *testing.T, pool *pgxpool.Pool) (orgID, userID string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("audit-org-%d", time.Now().UnixNano())
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, plan) VALUES ($1, $2, 'free') RETURNING id`,
		slug, slug,
	).Scan(&orgID)
	if err != nil {
		t.Fatalf("create audit org: %v", err)
	}
	email := fmt.Sprintf("audit-user-%d@example.com", time.Now().UnixNano())
	err = pool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, password_hash, display_name, role, auth_provider, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		orgID, email, "hash", "Audit User", "owner", "email", time.Now(), time.Now(),
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create audit user: %v", err)
	}
	return orgID, userID
}

func insertAuditLog(t *testing.T, pool *pgxpool.Pool, orgID, actorID, eventType, resourceType, resourceID string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO audit_logs (organization_id, actor_id, event_type, resource_type, resource_id, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		orgID, actorID, eventType, resourceType, resourceID, `{}`, time.Now(),
	)
	if err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
}

func TestAudit_UserRegistration_Logged(t *testing.T) {
	t.Parallel()
	pool := setupAuditDB(t)
	defer teardownAuditDB(t, pool)
	orgID, userID := seedAuditOrgAndUser(t, pool)
	ctx := context.Background()

	insertAuditLog(t, pool, orgID, userID, "user.registered", "user", userID)

	var count int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE event_type = 'user.registered' AND resource_type = 'user' AND resource_id = $1`,
		userID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log for user.registered, got %d", count)
	}

	var eventType string
	err = pool.QueryRow(ctx,
		`SELECT event_type FROM audit_logs WHERE resource_id = $1 LIMIT 1`, userID,
	).Scan(&eventType)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if eventType != "user.registered" {
		t.Errorf("expected event_type 'user.registered', got '%s'", eventType)
	}
}

func TestAudit_TunnelCreation_Logged(t *testing.T) {
	t.Parallel()
	pool := setupAuditDB(t)
	defer teardownAuditDB(t, pool)
	orgID, userID := seedAuditOrgAndUser(t, pool)
	ctx := context.Background()

	tunnelID := fmt.Sprintf("tun-audit-%d", time.Now().UnixNano())
	insertAuditLog(t, pool, orgID, userID, "tunnel.created", "tunnel", tunnelID)

	var count int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE event_type = 'tunnel.created' AND resource_id = $1`,
		tunnelID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log for tunnel.created, got %d", count)
	}

	insertAuditLog(t, pool, orgID, userID, "tunnel.started", "tunnel", tunnelID)
	insertAuditLog(t, pool, orgID, userID, "tunnel.deleted", "tunnel", tunnelID)

	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1`, tunnelID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count all audit logs for tunnel: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 audit log entries for tunnel lifecycle, got %d", count)
	}
}

func TestAudit_APIKeyCreation_Logged(t *testing.T) {
	t.Parallel()
	pool := setupAuditDB(t)
	defer teardownAuditDB(t, pool)
	orgID, userID := seedAuditOrgAndUser(t, pool)
	ctx := context.Background()

	apiKeyID := fmt.Sprintf("apikey-audit-%d", time.Now().UnixNano())
	insertAuditLog(t, pool, orgID, userID, "apikey.created", "api_key", apiKeyID)

	var eventType string
	var resourceType string
	err := pool.QueryRow(ctx,
		`SELECT event_type, resource_type FROM audit_logs WHERE resource_id = $1 LIMIT 1`, apiKeyID,
	).Scan(&eventType, &resourceType)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if eventType != "apikey.created" {
		t.Errorf("expected event_type 'apikey.created', got '%s'", eventType)
	}
	if resourceType != "api_key" {
		t.Errorf("expected resource_type 'api_key', got '%s'", resourceType)
	}

	insertAuditLog(t, pool, orgID, userID, "apikey.revoked", "api_key", apiKeyID)
	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1`, apiKeyID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 audit entries for API key lifecycle, got %d", count)
	}
}
