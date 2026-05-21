package billing

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	bytesPerGB = 1_073_741_824
)

type UsageTracker struct {
	pool *pgxpool.Pool
}

type OrgUsage struct {
	Plan           string
	TunnelsUsed    int
	TunnelsLimit   int
	BandwidthBytes int64
	BandwidthLimit int64
	PeriodStart    time.Time
	PeriodEnd      time.Time
}

type UsageRepository interface {
	GetOrganizationPlan(ctx context.Context, orgID string) (string, error)
	CountTunnels(ctx context.Context, orgID string) (int, error)
	GetBandwidthUsage(ctx context.Context, orgID string) (int64, error)
}

func NewUsageTracker(pool *pgxpool.Pool) *UsageTracker {
	return &UsageTracker{pool: pool}
}

func (t *UsageTracker) RecordUsage(ctx context.Context, orgID string, bytesIn, bytesOut int64) error {
	now := time.Now().UTC()
	periodStart := now.Truncate(1 * time.Hour)
	periodEnd := periodStart.Add(1 * time.Hour)
	_, err := t.pool.Exec(ctx,
		`INSERT INTO usage_records (organization_id, metric, quantity, period_start, period_end)
		 VALUES ($1, $2, $3, $4, $5)`,
		orgID, "bandwidth", bytesIn+bytesOut, periodStart, periodEnd,
	)
	return err
}

func (t *UsageTracker) GetCurrentUsage(ctx context.Context, orgID string) (*OrgUsage, error) {
	var plan string
	err := t.pool.QueryRow(ctx,
		`SELECT COALESCE(plan, 'free') FROM organizations WHERE id = $1`, orgID,
	).Scan(&plan)
	if err != nil {
		plan = "free"
	}

	var tunnelCount int
	err = t.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tunnels WHERE organization_id = $1 AND deleted_at IS NULL`, orgID,
	).Scan(&tunnelCount)
	if err != nil {
		tunnelCount = 0
	}

	var bwTotal int64
	periodStart := time.Now().UTC().Truncate(30 * 24 * time.Hour)
	err = t.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(quantity), 0)
		 FROM usage_records
		 WHERE organization_id = $1 AND metric = 'bandwidth' AND period_start >= $2`, orgID, periodStart,
	).Scan(&bwTotal)
	if err != nil {
		bwTotal = 0
	}

	p, ok := Plans[plan]
	if !ok {
		p = Plans["free"]
	}

	return &OrgUsage{
		Plan:           plan,
		TunnelsUsed:    tunnelCount,
		TunnelsLimit:   p.MaxTunnels,
		BandwidthBytes: bwTotal,
		BandwidthLimit: int64(p.MaxBandwidthGB) * bytesPerGB,
		PeriodStart:    periodStart,
		PeriodEnd:      periodStart.Add(30 * 24 * time.Hour),
	}, nil
}

func (t *UsageTracker) CheckQuota(ctx context.Context, orgID string, plan BillingPlan) error {
	usage, err := t.GetCurrentUsage(ctx, orgID)
	if err != nil {
		return err
	}

	if usage.TunnelsUsed >= plan.MaxTunnels {
		return &QuotaExceededError{
			Resource: "tunnels",
			Used:     usage.TunnelsUsed,
			Limit:    plan.MaxTunnels,
		}
	}

	if usage.BandwidthBytes >= int64(plan.MaxBandwidthGB)*bytesPerGB {
		return &QuotaExceededError{
			Resource: "bandwidth",
			Used:     int(usage.BandwidthBytes / bytesPerGB),
			Limit:    plan.MaxBandwidthGB,
		}
	}

	return nil
}

type QuotaExceededError struct {
	Resource string
	Used     int
	Limit    int
}

func (e *QuotaExceededError) Error() string {
	return "quota exceeded for " + e.Resource + ": " +
		fmtInt(e.Used) + "/" + fmtInt(e.Limit)
}

func fmtInt(n int) string {
	return intStr(n)
}

func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
