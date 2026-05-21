CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    email           TEXT NOT NULL,
    password_hash   TEXT,
    display_name    TEXT NOT NULL DEFAULT '',
    avatar_url      TEXT,
    role            TEXT NOT NULL DEFAULT 'member',
    auth_provider   TEXT NOT NULL DEFAULT 'email',
    auth_provider_id TEXT,
    mfa_enabled     BOOLEAN NOT NULL DEFAULT false,
    mfa_secret      TEXT,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE(organization_id, email)
);
CREATE INDEX idx_users_org ON users(organization_id) WHERE deleted_at IS NULL;
