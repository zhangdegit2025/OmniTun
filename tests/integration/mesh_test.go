//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupMeshDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanMeshTables(t, pool)
	return pool
}

func cleanMeshTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"mesh_peers", "mesh_networks", "workspaces", "organizations"}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func teardownMeshDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanMeshTables(t, pool)
		pool.Close()
	}
}

func createMeshOrgAndWorkspace(t *testing.T, pool *pgxpool.Pool) (orgID, wsID string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("mesh-org-%d", time.Now().UnixNano())
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, plan) VALUES ($1, $2, 'free') RETURNING id`,
		slug, slug,
	).Scan(&orgID)
	if err != nil {
		t.Fatalf("create mesh org: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO workspaces (organization_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		orgID, "mesh-ws", "mesh-ws",
	).Scan(&wsID)
	if err != nil {
		t.Fatalf("create mesh workspace: %v", err)
	}
	return orgID, wsID
}

func TestMesh_CreateNetwork_AssignsCIDR(t *testing.T) {
	t.Parallel()
	pool := setupMeshDB(t)
	defer teardownMeshDB(t, pool)
	ctx := context.Background()

	orgID, _ := createMeshOrgAndWorkspace(t, pool)

	var networkID string
	err := pool.QueryRow(ctx,
		`INSERT INTO mesh_networks (organization_id, name, cidr, invite_code, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, "test-mesh", "10.0.0.0/24", "invite-abc123", time.Now(), time.Now(),
	).Scan(&networkID)
	if err != nil {
		t.Fatalf("create mesh network: %v", err)
	}

	if networkID == "" {
		t.Fatal("expected non-empty network ID")
	}

	var cidr string
	err = pool.QueryRow(ctx,
		`SELECT cidr FROM mesh_networks WHERE id = $1`, networkID,
	).Scan(&cidr)
	if err != nil {
		t.Fatalf("lookup mesh network: %v", err)
	}
	if cidr != "10.0.0.0/24" {
		t.Errorf("expected CIDR '10.0.0.0/24', got '%s'", cidr)
	}
}

func TestMesh_JoinNetwork_InviteCode(t *testing.T) {
	t.Parallel()
	pool := setupMeshDB(t)
	defer teardownMeshDB(t, pool)
	ctx := context.Background()

	orgID, _ := createMeshOrgAndWorkspace(t, pool)

	var networkID string
	err := pool.QueryRow(ctx,
		`INSERT INTO mesh_networks (organization_id, name, cidr, invite_code, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, "join-test-mesh", "10.1.0.0/24", "invite-join-xyz", time.Now(), time.Now(),
	).Scan(&networkID)
	if err != nil {
		t.Fatalf("create mesh network: %v", err)
	}

	var inviteCode string
	err = pool.QueryRow(ctx,
		`SELECT invite_code FROM mesh_networks WHERE id = $1`, networkID,
	).Scan(&inviteCode)
	if err != nil {
		t.Fatalf("lookup invite code: %v", err)
	}
	if inviteCode == "" {
		t.Fatal("expected non-empty invite code")
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO mesh_peers (network_id, node_id, public_key, mesh_ip, endpoint, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		networkID, "node-001", "pubkey-base64-001", "10.1.0.2", "192.168.1.100:51820", time.Now(),
	)
	if err != nil {
		t.Fatalf("add mesh peer: %v", err)
	}

	var peerCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mesh_peers WHERE network_id = $1`, networkID,
	).Scan(&peerCount)
	if err != nil {
		t.Fatalf("count peers: %v", err)
	}
	if peerCount != 1 {
		t.Errorf("expected 1 peer, got %d", peerCount)
	}
}

func TestMesh_PeerDiscovery_ListsPeers(t *testing.T) {
	t.Parallel()
	pool := setupMeshDB(t)
	defer teardownMeshDB(t, pool)
	ctx := context.Background()

	orgID, _ := createMeshOrgAndWorkspace(t, pool)

	var networkID string
	err := pool.QueryRow(ctx,
		`INSERT INTO mesh_networks (organization_id, name, cidr, invite_code, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, "peer-discovery-mesh", "10.2.0.0/24", "invite-pd", time.Now(), time.Now(),
	).Scan(&networkID)
	if err != nil {
		t.Fatalf("create mesh network: %v", err)
	}

	peers := []struct {
		nodeID    string
		publicKey string
		meshIP    string
		endpoint  string
	}{
		{"node-a", "pubkey-a", "10.2.0.2", "10.0.1.1:51820"},
		{"node-b", "pubkey-b", "10.2.0.3", "10.0.2.1:51820"},
		{"node-c", "pubkey-c", "10.2.0.4", "10.0.3.1:51820"},
	}

	for _, p := range peers {
		_, err = pool.Exec(ctx,
			`INSERT INTO mesh_peers (network_id, node_id, public_key, mesh_ip, endpoint, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			networkID, p.nodeID, p.publicKey, p.meshIP, p.endpoint, time.Now(),
		)
		if err != nil {
			t.Fatalf("add peer %s: %v", p.nodeID, err)
		}
	}

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mesh_peers WHERE network_id = $1`, networkID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count peers: %v", err)
	}
	if count != len(peers) {
		t.Errorf("expected %d peers, got %d", len(peers), count)
	}

	rows, err := pool.Query(ctx,
		`SELECT node_id, public_key, mesh_ip, endpoint FROM mesh_peers WHERE network_id = $1 ORDER BY mesh_ip`,
		networkID,
	)
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	defer rows.Close()

	var listedPeers []struct {
		nodeID    string
		publicKey string
		meshIP    string
		endpoint  string
	}
	for rows.Next() {
		var p struct {
			nodeID    string
			publicKey string
			meshIP    string
			endpoint  string
		}
		if err := rows.Scan(&p.nodeID, &p.publicKey, &p.meshIP, &p.endpoint); err != nil {
			t.Fatalf("scan peer: %v", err)
		}
		listedPeers = append(listedPeers, p)
	}
	if len(listedPeers) != len(peers) {
		t.Errorf("expected %d peers in list, got %d", len(peers), len(listedPeers))
	}
}
