package featureflags

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Flag struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Value       string    `json:"value"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BooleanValue struct {
	Enabled bool `json:"enabled"`
}

type PercentageValue struct {
	Percentage int `json:"percentage"`
}

type WhitelistValue struct {
	Orgs []string `json:"orgs"`
}

type Manager struct {
	pool   *pgxpool.Pool
	cache  map[string]*Flag
	mu     sync.RWMutex
	expiry time.Time
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{
		pool:  pool,
		cache: make(map[string]*Flag),
	}
}

func (m *Manager) StartCacheRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	m.refreshCache(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshCache(ctx)
		}
	}
}

func (m *Manager) refreshCache(ctx context.Context) {
	flags, err := m.loadAllFlags(ctx)
	if err != nil {
		slog.Error("failed to refresh feature flags cache", "error", err)
		return
	}
	m.mu.Lock()
	m.cache = make(map[string]*Flag, len(flags))
	for _, f := range flags {
		m.cache[f.Key] = f
	}
	m.expiry = time.Now().Add(5 * time.Second)
	m.mu.Unlock()
}

func (m *Manager) loadAllFlags(ctx context.Context) ([]*Flag, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT id::text, key, name, description, type, value::text, enabled, created_at, updated_at
		 FROM feature_flags ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flags []*Flag
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.ID, &f.Key, &f.Name, &f.Description, &f.Type, &f.Value, &f.Enabled, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		flags = append(flags, &f)
	}
	return flags, nil
}

func (m *Manager) IsEnabled(ctx context.Context, flagKey string, orgID string) bool {
	m.mu.RLock()
	flag, ok := m.cache[flagKey]
	m.mu.RUnlock()

	if !ok {
		var err error
		flag, err = m.loadFlagByKey(ctx, flagKey)
		if err != nil || flag == nil {
			return false
		}
		m.mu.Lock()
		m.cache[flagKey] = flag
		m.mu.Unlock()
	}

	if !flag.Enabled {
		return false
	}

	switch flag.Type {
	case "boolean":
		var bv BooleanValue
		if err := json.Unmarshal([]byte(flag.Value), &bv); err != nil {
			return false
		}
		return bv.Enabled
	case "percentage":
		var pv PercentageValue
		if err := json.Unmarshal([]byte(flag.Value), &pv); err != nil {
			return false
		}
		return m.evaluatePercentage(orgID, pv.Percentage)
	case "whitelist":
		var wv WhitelistValue
		if err := json.Unmarshal([]byte(flag.Value), &wv); err != nil {
			return false
		}
		for _, o := range wv.Orgs {
			if o == orgID {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (m *Manager) evaluatePercentage(orgID string, percentage int) bool {
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}
	h := fnv.New32a()
	h.Write([]byte(orgID))
	bucket := int(h.Sum32() % 100)
	return bucket < percentage
}

func (m *Manager) loadFlagByKey(ctx context.Context, key string) (*Flag, error) {
	var f Flag
	err := m.pool.QueryRow(ctx,
		`SELECT id::text, key, name, description, type, value::text, enabled, created_at, updated_at
		 FROM feature_flags WHERE key=$1`, key,
	).Scan(&f.ID, &f.Key, &f.Name, &f.Description, &f.Type, &f.Value, &f.Enabled, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (m *Manager) GetFlag(ctx context.Context, key string) (*Flag, error) {
	m.mu.RLock()
	flag, ok := m.cache[key]
	m.mu.RUnlock()

	if ok {
		return flag, nil
	}
	return m.loadFlagByKey(ctx, key)
}

func (m *Manager) ListFlags(ctx context.Context) ([]*Flag, error) {
	m.mu.RLock()
	if time.Now().Before(m.expiry) && len(m.cache) > 0 {
		flags := make([]*Flag, 0, len(m.cache))
		for _, f := range m.cache {
			flags = append(flags, f)
		}
		m.mu.RUnlock()
		return flags, nil
	}
	m.mu.RUnlock()
	return m.loadAllFlags(ctx)
}

func (m *Manager) CreateFlag(ctx context.Context, flag *Flag) error {
	err := m.pool.QueryRow(ctx,
		`INSERT INTO feature_flags (key, name, description, type, value, enabled)
		 VALUES ($1,$2,$3,$4,$5::jsonb,$6)
		 RETURNING id::text, created_at, updated_at`,
		flag.Key, flag.Name, flag.Description, flag.Type, flag.Value, flag.Enabled,
	).Scan(&flag.ID, &flag.CreatedAt, &flag.UpdatedAt)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.cache[flag.Key] = flag
	m.mu.Unlock()
	return nil
}

func (m *Manager) UpdateFlag(ctx context.Context, flag *Flag) error {
	tag, err := m.pool.Exec(ctx,
		`UPDATE feature_flags SET name=$1, description=$2, type=$3, value=$4::jsonb, enabled=$5, updated_at=now()
		 WHERE key=$6`,
		flag.Name, flag.Description, flag.Type, flag.Value, flag.Enabled, flag.Key,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	err = m.pool.QueryRow(ctx,
		`SELECT id::text, created_at, updated_at FROM feature_flags WHERE key=$1`, flag.Key,
	).Scan(&flag.ID, &flag.CreatedAt, &flag.UpdatedAt)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.cache[flag.Key] = flag
	m.mu.Unlock()
	return nil
}

func (m *Manager) DeleteFlag(ctx context.Context, key string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM feature_flags WHERE key=$1`, key)
	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.cache, key)
	m.mu.Unlock()
	return nil
}
