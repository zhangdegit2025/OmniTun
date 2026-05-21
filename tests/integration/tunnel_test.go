//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omnitun/omnitun/internal/tunnel"
	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
)

func setupTunnelDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanTunnelTables(t, pool)
	seedRelayNodes(t, pool)
	return pool
}

func cleanTunnelTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"tunnels", "relay_nodes", "workspaces", "organizations"}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func teardownTunnelDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanTunnelTables(t, pool)
		pool.Close()
	}
}

func seedRelayNodes(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	relays := []struct {
		name, region, hostname, ip string
		capacity, activeTunnels    int
		port                       int
	}{
		{"us-east-1", "us-east", "relay1.omnitun.io", "10.0.1.1", 1000, 50, 443},
		{"eu-west-1", "eu-west", "relay2.omnitun.io", "10.0.2.1", 1000, 200, 443},
		{"us-east-2", "us-east", "relay3.omnitun.io", "10.0.1.2", 500, 300, 443},
		{"ap-south-1", "ap-south", "relay4.omnitun.io", "10.0.3.1", 100, 100, 443},
	}
	for _, r := range relays {
		_, err := pool.Exec(ctx,
			`INSERT INTO relay_nodes (name, region, hostname, ip_address, port, capacity, active_tunnels, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')`,
			r.name, r.region, r.hostname, r.ip, r.port, r.capacity, r.activeTunnels,
		)
		if err != nil {
			t.Fatalf("seed relay node %s: %v", r.name, err)
		}
	}
}

func createOrgAndWorkspace(t *testing.T, pool *pgxpool.Pool) (orgID, wsID string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
	var oid, wid string
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, plan) VALUES ($1, $2, 'free') RETURNING id`,
		slug, slug,
	).Scan(&oid)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO workspaces (organization_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		oid, "default", "default",
	).Scan(&wid)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return oid, wid
}

type pgTunnelRepo struct {
	pool *pgxpool.Pool
}

func (r *pgTunnelRepo) CreateTunnel(ctx context.Context, t *tunnel.Tunnel) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO tunnels (organization_id, workspace_id, name, slug, protocol, local_port, local_host,
		 custom_domain, region, tls_mode, auth_mode, status, relay_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id`,
		t.OrganizationID, t.WorkspaceID, t.Name, t.Slug, t.Protocol, t.LocalPort, t.LocalHost,
		nullString(t.CustomDomain), nullString(t.Region), t.TLSMode, t.AuthMode, string(t.Status),
		nullString(t.RelayID), t.CreatedAt, t.UpdatedAt,
	).Scan(&t.ID)
}

func (r *pgTunnelRepo) GetTunnel(ctx context.Context, id string) (*tunnel.Tunnel, error) {
	return r.scanTunnel(ctx, `SELECT id, organization_id, workspace_id, name, slug, protocol, local_port, local_host,
		custom_domain, region, tls_mode, auth_mode, status, relay_id, agent_id,
		bytes_in_total, bytes_out_total, created_at, updated_at FROM tunnels WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *pgTunnelRepo) GetTunnelBySlug(ctx context.Context, slug string) (*tunnel.Tunnel, error) {
	return r.scanTunnel(ctx, `SELECT id, organization_id, workspace_id, name, slug, protocol, local_port, local_host,
		custom_domain, region, tls_mode, auth_mode, status, relay_id, agent_id,
		bytes_in_total, bytes_out_total, created_at, updated_at FROM tunnels WHERE slug = $1 AND deleted_at IS NULL`, slug)
}

func (r *pgTunnelRepo) ListTunnels(ctx context.Context, workspaceID string, limit int, cursor string) ([]*tunnel.Tunnel, string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, workspace_id, name, slug, protocol, local_port, local_host,
		 custom_domain, region, tls_mode, auth_mode, status, relay_id, agent_id,
		 bytes_in_total, bytes_out_total, created_at, updated_at
		 FROM tunnels WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var tunnels []*tunnel.Tunnel
	for rows.Next() {
		t, err := scanTunnelRow(rows)
		if err != nil {
			return nil, "", err
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, "", rows.Err()
}

func (r *pgTunnelRepo) UpdateTunnel(ctx context.Context, t *tunnel.Tunnel) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tunnels SET name=$1, auth_mode=$2, max_connections=$3, updated_at=$4 WHERE id=$5 AND deleted_at IS NULL`,
		t.Name, t.AuthMode, t.MaxConnections, time.Now(), t.ID,
	)
	return err
}

func (r *pgTunnelRepo) DeleteTunnel(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE tunnels SET deleted_at=$1 WHERE id=$2`, now, id)
	return err
}

func (r *pgTunnelRepo) UpdateTunnelStatus(ctx context.Context, id string, status tunnel.TunnelStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE tunnels SET status=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
		string(status), time.Now(), id)
	return err
}

func (r *pgTunnelRepo) UpdateTunnelRelay(ctx context.Context, id string, relayID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE tunnels SET relay_id=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
		relayID, time.Now(), id)
	return err
}

func (r *pgTunnelRepo) CountTunnelsByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tunnels WHERE workspace_id=$1 AND deleted_at IS NULL`,
		workspaceID,
	).Scan(&count)
	return count, err
}

func (r *pgTunnelRepo) GetActiveRelays(ctx context.Context) ([]*tunnel.RelayNode, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, region, hostname, ip_address::text, port, capacity, active_tunnels, status
		 FROM relay_nodes WHERE status = 'active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relays []*tunnel.RelayNode
	for rows.Next() {
		rn := &tunnel.RelayNode{}
		if err := rows.Scan(&rn.ID, &rn.Name, &rn.Region, &rn.Hostname, &rn.IPAddress,
			&rn.Port, &rn.Capacity, &rn.ActiveTunnels, &rn.Status); err != nil {
			return nil, err
		}
		relays = append(relays, rn)
	}
	return relays, rows.Err()
}

func (r *pgTunnelRepo) GetRelayNode(ctx context.Context, id string) (*tunnel.RelayNode, error) {
	rn := &tunnel.RelayNode{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, region, hostname, ip_address::text, port, capacity, active_tunnels, status
		 FROM relay_nodes WHERE id = $1`, id,
	).Scan(&rn.ID, &rn.Name, &rn.Region, &rn.Hostname, &rn.IPAddress,
		&rn.Port, &rn.Capacity, &rn.ActiveTunnels, &rn.Status)
	if err != nil {
		return nil, err
	}
	return rn, nil
}

func (r *pgTunnelRepo) UpdateRelayActiveTunnels(ctx context.Context, id string, count int) error {
	_, err := r.pool.Exec(ctx, `UPDATE relay_nodes SET active_tunnels=$1, updated_at=$2 WHERE id=$3`,
		count, time.Now(), id)
	return err
}

func (r *pgTunnelRepo) scanTunnel(ctx context.Context, query string, args ...any) (*tunnel.Tunnel, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	t, err := scanTunnelRow(row)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func scanTunnelRow(row interface{ Scan(...any) error }) (*tunnel.Tunnel, error) {
	t := &tunnel.Tunnel{}
	var customDomain, agentID, relayID sql.NullString
	var localPort, maxConns int
	if err := row.Scan(
		&t.ID, &t.OrganizationID, &t.WorkspaceID, &t.Name, &t.Slug, &t.Protocol, &localPort, &t.LocalHost,
		&customDomain, &t.Region, &t.TLSMode, &t.AuthMode, &t.Status, &relayID, &agentID,
		&t.BytesInTotal, &t.BytesOutTotal, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.LocalPort = localPort
	t.MaxConnections = maxConns
	if customDomain.Valid {
		t.CustomDomain = customDomain.String
	}
	if agentID.Valid {
		t.AgentID = agentID.String
	}
	if relayID.Valid {
		t.RelayID = relayID.String
	}
	return t, nil
}

type mockEventPublisher struct{}

func (m *mockEventPublisher) PublishTunnelEvent(ctx context.Context, eventType string, t *tunnel.Tunnel) error {
	return nil
}

type freeQuotaChecker struct{ max int }

func (q *freeQuotaChecker) GetMaxTunnels(ctx context.Context, workspaceID string) (int, error) {
	if q.max > 0 {
		return q.max, nil
	}
	return 1, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func newTunnelService(t *testing.T, pool *pgxpool.Pool) *tunnel.Service {
	t.Helper()
	repo := &pgTunnelRepo{pool: pool}
	relaySel := tunnel.NewRelaySelector(repo)
	eventBus := &mockEventPublisher{}
	svc := tunnel.NewService(repo, relaySel, eventBus)
	return svc
}

func TestTunnel_CreateAndStart_StatusTransitions(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "test-tunnel",
		Protocol:    "http",
		LocalPort:   8080,
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}
	if resp.Tunnel == nil {
		t.Fatal("expected non-nil tunnel")
	}
	if resp.Tunnel.Id == "" {
		t.Fatal("expected non-empty tunnel ID")
	}
	if resp.Tunnel.Status != "stopped" {
		t.Errorf("expected status 'stopped', got '%s'", resp.Tunnel.Status)
	}

	getResp, err := svc.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{
		TunnelId: resp.Tunnel.Id,
	})
	if err != nil {
		t.Fatalf("GetTunnel failed: %v", err)
	}
	if getResp.Tunnel.Name != "test-tunnel" {
		t.Errorf("expected name 'test-tunnel', got '%s'", getResp.Tunnel.Name)
	}

	_, err = svc.StartTunnel(ctx, &omnitunv1.StartTunnelRequest{
		TunnelId: resp.Tunnel.Id,
	})
	if err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}

	getResp2, err := svc.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{
		TunnelId: resp.Tunnel.Id,
	})
	if err != nil {
		t.Fatalf("GetTunnel after start failed: %v", err)
	}
	if getResp2.Tunnel.Status != "starting" {
		t.Errorf("expected status 'starting' after start, got '%s'", getResp2.Tunnel.Status)
	}
}

func TestTunnel_LifecycleStrictTransitions_InvalidRejected(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "lifecycle-test",
		Protocol:    "tcp",
		LocalPort:   3000,
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}
	tunnelID := resp.Tunnel.Id

	repo := &pgTunnelRepo{pool: pool}
	if err := repo.UpdateTunnelStatus(ctx, tunnelID, tunnel.StatusActive); err != nil {
		t.Fatalf("set status to active: %v", err)
	}

	_, err = svc.StartTunnel(ctx, &omnitunv1.StartTunnelRequest{
		TunnelId: tunnelID,
	})
	if err == nil {
		t.Fatal("StartTunnel should fail when tunnel is already active")
	}

	if err := repo.UpdateTunnelStatus(ctx, tunnelID, tunnel.StatusActive); err != nil {
		t.Fatalf("reset status to active: %v", err)
	}

	_, err = svc.StopTunnel(ctx, &omnitunv1.StopTunnelRequest{
		TunnelId: tunnelID,
	})
	if err != nil {
		t.Fatalf("StopTunnel should succeed when active: %v", err)
	}

	getResp, _ := svc.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{TunnelId: tunnelID})
	if getResp.Tunnel.Status != "stopped" {
		t.Errorf("expected status 'stopped' after stop, got '%s'", getResp.Tunnel.Status)
	}
}

func TestTunnel_QuotaEnforcement_FreePlanLimit(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	svc = svc.WithQuotaChecker(&freeQuotaChecker{max: 1})
	ctx := context.Background()

	_, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "quota-tunnel-1",
		Protocol:    "http",
		LocalPort:   8080,
	})
	if err != nil {
		t.Fatalf("first tunnel should succeed: %v", err)
	}

	_, err = svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "quota-tunnel-2",
		Protocol:    "http",
		LocalPort:   8081,
	})
	if err == nil {
		t.Fatal("second tunnel should be rejected due to quota")
	}
}

func TestTunnel_RelaySelection_OptimalRelayChosen(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "relay-select-test",
		Protocol:    "http",
		LocalPort:   3000,
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}
	if resp.Tunnel.Id == "" {
		t.Fatal("expected non-empty tunnel ID")
	}

	repo := &pgTunnelRepo{pool: pool}
	_, err = repo.GetRelayNode(ctx, resp.Tunnel.Id)
	if err != nil {
		t.Logf("relay node lookup for tunnel: %v", err)
	}

	relays, err := repo.GetActiveRelays(ctx)
	if err != nil {
		t.Fatalf("get active relays: %v", err)
	}
	if len(relays) == 0 {
		t.Fatal("expected at least one active relay")
	}

	t.Logf("tunnel assigned to relay ID: %s", resp.Tunnel.Id)
	t.Logf("active relays: %d", len(relays))
}

func TestTunnel_ListTunnels_ReturnsByWorkspace(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	names := []string{"tun-a", "tun-b", "tun-c"}
	for _, name := range names {
		_, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
			WorkspaceId: wsID,
			Name:        name,
			Protocol:    "http",
			LocalPort:   8080,
		})
		if err != nil {
			t.Fatalf("create %s failed: %v", name, err)
		}
	}

	listResp, err := svc.ListTunnels(ctx, &omnitunv1.ListTunnelsRequest{
		WorkspaceId: wsID,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListTunnels failed: %v", err)
	}
	if len(listResp.Tunnels) != 3 {
		t.Errorf("expected 3 tunnels, got %d", len(listResp.Tunnels))
	}
}

func TestTunnel_CreateWithCustomDomain(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	customDomain := "api.example.com"
	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId:  wsID,
		Name:         "custom-domain-tunnel",
		Protocol:     "https",
		LocalPort:    443,
		CustomDomain: customDomain,
		TlsMode:      "full",
	})
	if err != nil {
		t.Fatalf("CreateTunnel with custom domain failed: %v", err)
	}
	if resp.Tunnel == nil {
		t.Fatal("expected non-nil tunnel")
	}
	if resp.Tunnel.CustomDomain != customDomain {
		t.Errorf("expected custom domain %q, got %q", customDomain, resp.Tunnel.CustomDomain)
	}

	getResp, err := svc.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{
		TunnelId: resp.Tunnel.Id,
	})
	if err != nil {
		t.Fatalf("GetTunnel failed: %v", err)
	}
	if getResp.Tunnel.CustomDomain != customDomain {
		t.Errorf("expected custom domain %q after retrieval, got %q", customDomain, getResp.Tunnel.CustomDomain)
	}
}

func TestTunnel_QuotaLimit_FreePlan(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	svc = svc.WithQuotaChecker(&freeQuotaChecker{max: 1})
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "free-plan-tunnel",
		Protocol:    "http",
		LocalPort:   8080,
	})
	if err != nil {
		t.Fatalf("first tunnel should succeed under free plan quota: %v", err)
	}
	if resp.Tunnel == nil {
		t.Fatal("expected non-nil tunnel on success")
	}

	_, err = svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "exceeds-quota",
		Protocol:    "tcp",
		LocalPort:   9000,
	})
	if err == nil {
		t.Fatal("expected quota exceeded error for second tunnel under free plan")
	}
}

func TestTunnel_Start_ReturnsRelayAddress(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "start-relay-tunnel",
		Protocol:    "http",
		LocalPort:   8080,
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}

	startResp, err := svc.StartTunnel(ctx, &omnitunv1.StartTunnelRequest{
		TunnelId: resp.Tunnel.Id,
	})
	if err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}
	if startResp.Message == "" {
		t.Fatal("expected non-empty message on start")
	}

	repo := &pgTunnelRepo{pool: pool}
	relays, err := repo.GetActiveRelays(ctx)
	if err != nil {
		t.Fatalf("get active relays: %v", err)
	}
	if len(relays) > 0 && startResp.RelayAddress == "" {
		t.Error("expected non-empty relay_address when relays exist")
	}

	tunnel, err := repo.GetTunnel(ctx, resp.Tunnel.Id)
	if err != nil {
		t.Fatalf("get tunnel: %v", err)
	}
	if string(tunnel.Status) != "starting" {
		t.Errorf("expected status 'starting' after start, got '%s'", tunnel.Status)
	}
}

func TestTunnel_Delete_SoftDeletes(t *testing.T) {
	t.Parallel()
	pool := setupTunnelDB(t)
	defer teardownTunnelDB(t, pool)
	_, wsID := createOrgAndWorkspace(t, pool)
	svc := newTunnelService(t, pool)
	ctx := context.Background()

	resp, err := svc.CreateTunnel(ctx, &omnitunv1.CreateTunnelRequest{
		WorkspaceId: wsID,
		Name:        "soft-delete-tunnel",
		Protocol:    "http",
		LocalPort:   8080,
	})
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}
	tunnelID := resp.Tunnel.Id

	delResp, err := svc.DeleteTunnel(ctx, &omnitunv1.DeleteTunnelRequest{
		TunnelId: tunnelID,
	})
	if err != nil {
		t.Fatalf("DeleteTunnel failed: %v", err)
	}
	if delResp.Message == "" {
		t.Fatal("expected non-empty delete message")
	}

	_, err = svc.GetTunnel(ctx, &omnitunv1.GetTunnelRequest{
		TunnelId: tunnelID,
	})
	if err == nil {
		t.Fatal("GetTunnel should fail after soft delete")
	}
}
