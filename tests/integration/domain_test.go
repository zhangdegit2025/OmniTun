//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupDomainDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanDomainTables(t, pool)
	return pool
}

func cleanDomainTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"custom_domains", "tunnels", "workspaces", "organizations"}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func teardownDomainDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanDomainTables(t, pool)
		pool.Close()
	}
}

func createDomainOrgAndWorkspace(t *testing.T, pool *pgxpool.Pool) (orgID, wsID string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("domain-org-%d", time.Now().UnixNano())
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, plan) VALUES ($1, $2, 'free') RETURNING id`,
		slug, slug,
	).Scan(&orgID)
	if err != nil {
		t.Fatalf("create domain org: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO workspaces (organization_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		orgID, "domain-ws", "domain-ws",
	).Scan(&wsID)
	if err != nil {
		t.Fatalf("create domain workspace: %v", err)
	}
	return orgID, wsID
}

func TestDomain_AddCustomDomain(t *testing.T) {
	t.Parallel()
	pool := setupDomainDB(t)
	defer teardownDomainDB(t, pool)
	ctx := context.Background()

	orgID, wsID := createDomainOrgAndWorkspace(t, pool)

	var tunnelID string
	err := pool.QueryRow(ctx,
		`INSERT INTO tunnels (organization_id, workspace_id, name, slug, protocol, local_port, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		orgID, wsID, "domain-tunnel", "dom-tun-1", "http", 8080, "stopped", time.Now(), time.Now(),
	).Scan(&tunnelID)
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	domain := "api.myapp.com"
	var domainID string
	err = pool.QueryRow(ctx,
		`INSERT INTO custom_domains (organization_id, tunnel_id, domain, verification_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, tunnelID, domain, "pending", time.Now(), time.Now(),
	).Scan(&domainID)
	if err != nil {
		t.Fatalf("add custom domain: %v", err)
	}

	if domainID == "" {
		t.Fatal("expected non-empty domain ID")
	}

	var storedDomain string
	err = pool.QueryRow(ctx,
		`SELECT domain FROM custom_domains WHERE id = $1`, domainID,
	).Scan(&storedDomain)
	if err != nil {
		t.Fatalf("lookup domain: %v", err)
	}
	if storedDomain != domain {
		t.Errorf("expected domain %q, got %q", domain, storedDomain)
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM custom_domains WHERE tunnel_id = $1`, tunnelID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count domains: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 custom domain, got %d", count)
	}
}

func TestDomain_VerificationStatus(t *testing.T) {
	t.Parallel()
	pool := setupDomainDB(t)
	defer teardownDomainDB(t, pool)
	ctx := context.Background()

	orgID, wsID := createDomainOrgAndWorkspace(t, pool)

	var tunnelID string
	err := pool.QueryRow(ctx,
		`INSERT INTO tunnels (organization_id, workspace_id, name, slug, protocol, local_port, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		orgID, wsID, "verify-domain-tunnel", "ver-dom-1", "https", 443, "stopped", time.Now(), time.Now(),
	).Scan(&tunnelID)
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	var domainID string
	err = pool.QueryRow(ctx,
		`INSERT INTO custom_domains (organization_id, tunnel_id, domain, verification_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, tunnelID, "verified.example.com", "pending", time.Now(), time.Now(),
	).Scan(&domainID)
	if err != nil {
		t.Fatalf("add domain: %v", err)
	}

	var status string
	err = pool.QueryRow(ctx,
		`SELECT verification_status FROM custom_domains WHERE id = $1`, domainID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("lookup domain: %v", err)
	}
	if status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", status)
	}

	_, err = pool.Exec(ctx,
		`UPDATE custom_domains SET verification_status = $1, updated_at = $2 WHERE id = $3`,
		"verified", time.Now(), domainID,
	)
	if err != nil {
		t.Fatalf("update verification status: %v", err)
	}

	err = pool.QueryRow(ctx,
		`SELECT verification_status FROM custom_domains WHERE id = $1`, domainID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("lookup domain after update: %v", err)
	}
	if status != "verified" {
		t.Errorf("expected status 'verified' after update, got '%s'", status)
	}
}
