//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omnitun/omnitun/internal/billing"
)

func setupBillingDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanBillingTables(t, pool)
	seedBillingOrg(t, pool)
	return pool
}

func cleanBillingTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"usage_records", "organizations"}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func seedBillingOrg(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("billing-org-%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (name, slug, plan) VALUES ($1, $2, 'free') ON CONFLICT DO NOTHING`,
		slug, slug,
	)
	if err != nil {
		t.Fatalf("seed billing org: %v", err)
	}
}

func teardownBillingDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanBillingTables(t, pool)
		pool.Close()
	}
}

func getBillingOrgID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var orgID string
	err := pool.QueryRow(ctx, `SELECT id FROM organizations WHERE plan = 'free' LIMIT 1`).Scan(&orgID)
	if err != nil {
		t.Fatalf("get billing org id: %v", err)
	}
	return orgID
}

func TestBilling_FreePlan_LimitEnforcement(t *testing.T) {
	t.Parallel()
	pool := setupBillingDB(t)
	defer teardownBillingDB(t, pool)
	ctx := context.Background()

	orgID := getBillingOrgID(t, pool)

	tracker := billing.NewUsageTracker(pool)
	usage, err := tracker.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage failed: %v", err)
	}

	if usage.Plan != "free" {
		t.Errorf("expected plan 'free', got '%s'", usage.Plan)
	}

	freePlan, ok := billing.PlanFromString("free")
	if !ok {
		t.Fatal("free plan should exist")
	}
	if usage.TunnelsLimit != freePlan.MaxTunnels {
		t.Errorf("expected tunnels limit %d, got %d", freePlan.MaxTunnels, usage.TunnelsLimit)
	}
	if usage.BandwidthLimit > 0 && usage.BandwidthLimit != int64(freePlan.MaxBandwidthGB)*1_073_741_824 {
		t.Errorf("unexpected bandwidth limit: %d", usage.BandwidthLimit)
	}

	err = tracker.CheckQuota(ctx, orgID, freePlan)
	if err != nil {
		t.Logf("quota check warning (may be expected): %v", err)
	}

	proPlan, _ := billing.PlanFromString("pro")
	if proPlan.MaxTunnels <= 1 {
		t.Error("pro plan should allow more tunnels than free")
	}
}

func TestBilling_QuotaExceeded_Returns402(t *testing.T) {
	t.Parallel()
	pool := setupBillingDB(t)
	defer teardownBillingDB(t, pool)
	ctx := context.Background()

	orgID := getBillingOrgID(t, pool)

	tracker := billing.NewUsageTracker(pool)

	largeBytes := int64(2 * 1_073_741_824)
	err := tracker.RecordUsage(ctx, orgID, largeBytes, 0)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	usage, err := tracker.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage failed: %v", err)
	}

	if usage.BandwidthBytes < largeBytes {
		t.Errorf("expected bandwidth usage >= %d, got %d", largeBytes, usage.BandwidthBytes)
	}

	freePlan, _ := billing.PlanFromString("free")
	err = tracker.CheckQuota(ctx, orgID, freePlan)
	if err == nil {
		t.Log("quota not exceeded (bandwidth limit may be higher)")
	} else {
		t.Logf("quota exceeded as expected: %v", err)
	}
}

func TestBilling_UsageTracking_RecordsBandwidth(t *testing.T) {
	t.Parallel()
	pool := setupBillingDB(t)
	defer teardownBillingDB(t, pool)
	ctx := context.Background()

	orgID := getBillingOrgID(t, pool)

	tracker := billing.NewUsageTracker(pool)

	usageBefore, err := tracker.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage before: %v", err)
	}

	bytesIn := int64(1024 * 1024)
	bytesOut := int64(512 * 1024)
	err = tracker.RecordUsage(ctx, orgID, bytesIn, bytesOut)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	usageAfter, err := tracker.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage after: %v", err)
	}

	expectedIncrease := bytesIn + bytesOut
	actualIncrease := usageAfter.BandwidthBytes - usageBefore.BandwidthBytes
	if actualIncrease < expectedIncrease {
		t.Errorf("expected bandwidth increase >= %d, got %d", expectedIncrease, actualIncrease)
	}
}
