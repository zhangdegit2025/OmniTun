CREATE TABLE announcements (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    severity    TEXT NOT NULL DEFAULT 'info',
    target      TEXT NOT NULL DEFAULT 'all',
    active      BOOLEAN NOT NULL DEFAULT true,
    start_at    TIMESTAMPTZ,
    end_at      TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_announcements_active ON announcements(active, start_at, end_at);
