package announcements

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Announcement struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Severity  string     `json:"severity"`
	Target    string     `json:"target"`
	Active    bool       `json:"active"`
	StartAt   *time.Time `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

func (m *Manager) GetActiveAnnouncements(ctx context.Context, plan string) ([]*Announcement, error) {
	now := time.Now().UTC()
	rows, err := m.pool.Query(ctx,
		`SELECT id::text, title, COALESCE(body,''), severity, target, active, start_at, end_at, created_at, updated_at
		 FROM announcements
		 WHERE active = true
		   AND (start_at IS NULL OR start_at <= $1)
		   AND (end_at IS NULL OR end_at >= $1)
		   AND (target = 'all' OR target = $2)
		 ORDER BY created_at DESC`,
		now, plan,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var announcements []*Announcement
	for rows.Next() {
		a := &Announcement{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Severity, &a.Target, &a.Active, &a.StartAt, &a.EndAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}
	if announcements == nil {
		announcements = []*Announcement{}
	}
	return announcements, nil
}

func (m *Manager) ListAll(ctx context.Context) ([]*Announcement, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT id::text, title, COALESCE(body,''), severity, target, active, start_at, end_at, created_at, updated_at
		 FROM announcements
		 ORDER BY created_at DESC LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var announcements []*Announcement
	for rows.Next() {
		a := &Announcement{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Severity, &a.Target, &a.Active, &a.StartAt, &a.EndAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}
	if announcements == nil {
		announcements = []*Announcement{}
	}
	return announcements, nil
}

func (m *Manager) Create(ctx context.Context, a *Announcement) error {
	return m.pool.QueryRow(ctx,
		`INSERT INTO announcements (title, body, severity, target, active, start_at, end_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id::text`,
		a.Title, a.Body, a.Severity, a.Target, a.Active, a.StartAt, a.EndAt,
	).Scan(&a.ID)
}

func (m *Manager) Update(ctx context.Context, a *Announcement) error {
	tag, err := m.pool.Exec(ctx,
		`UPDATE announcements
		 SET title=$1, body=$2, severity=$3, target=$4, active=$5, start_at=$6, end_at=$7, updated_at=NOW()
		 WHERE id=$8`,
		a.Title, a.Body, a.Severity, a.Target, a.Active, a.StartAt, a.EndAt, a.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return err
	}
	return nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	_, err := m.pool.Exec(ctx,
		`DELETE FROM announcements WHERE id=$1`,
		id,
	)
	return err
}
