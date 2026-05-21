package abuse

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Report struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	ReporterID  string     `json:"reporter_id"`
	TunnelID    string     `json:"tunnel_id"`
	Reason      string     `json:"reason"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Resolution  string     `json:"resolution,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type IPBlacklistEntry struct {
	ID        string     `json:"id"`
	CIDR      string     `json:"cidr"`
	Reason    string     `json:"reason"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Manager struct {
	pool     *pgxpool.Pool
	mu       sync.RWMutex
	reports  []*Report
	blacklist []*IPBlacklistEntry
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{
		pool:      pool,
		reports:   make([]*Report, 0),
		blacklist: make([]*IPBlacklistEntry, 0),
	}
}

func uuidStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (m *Manager) ListReports(ctx context.Context, status string, limit, offset int) ([]*Report, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []*Report
	for _, r := range m.reports {
		if status != "" && r.Status != status {
			continue
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)

	if offset >= len(filtered) {
		return []*Report{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]*Report, end-offset)
	copy(result, filtered[offset:end])
	return result, total, nil
}

func (m *Manager) GetReport(ctx context.Context, id string) (*Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("report not found")
}

func (m *Manager) ResolveReport(ctx context.Context, id, resolution string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.reports {
		if r.ID == id {
			r.Status = "resolved"
			r.Resolution = resolution
			r.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("report not found")
}

func (m *Manager) DismissReport(ctx context.Context, id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.reports {
		if r.ID == id {
			r.Status = "dismissed"
			r.Resolution = reason
			r.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("report not found")
}

func (m *Manager) SubmitReport(ctx context.Context, orgID, reporterID, tunnelID, reason, description string) (*Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	report := &Report{
		ID:          uuidStr(),
		OrgID:       orgID,
		ReporterID:  reporterID,
		TunnelID:    tunnelID,
		Reason:      reason,
		Description: description,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.reports = append(m.reports, report)
	return report, nil
}

func (m *Manager) AddToBlacklist(ctx context.Context, cidr, reason, createdBy string) (*IPBlacklistEntry, error) {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	entry := &IPBlacklistEntry{
		ID:        uuidStr(),
		CIDR:      cidr,
		Reason:    reason,
		CreatedBy: createdBy,
		CreatedAt: now,
	}
	m.blacklist = append(m.blacklist, entry)
	return entry, nil
}

func (m *Manager) RemoveFromBlacklist(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, entry := range m.blacklist {
		if entry.ID == id {
			m.blacklist = append(m.blacklist[:i], m.blacklist[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("blacklist entry not found")
}

func (m *Manager) IsBlacklisted(ctx context.Context, ip string) (bool, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, fmt.Errorf("invalid IP address")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.blacklist {
		if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
			continue
		}
		_, cidrNet, err := net.ParseCIDR(entry.CIDR)
		if err != nil {
			continue
		}
		if cidrNet.Contains(parsedIP) {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) ListBlacklist(ctx context.Context) ([]*IPBlacklistEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IPBlacklistEntry, len(m.blacklist))
	copy(result, m.blacklist)
	return result, nil
}
