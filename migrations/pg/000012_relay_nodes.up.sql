CREATE TABLE relay_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    region          TEXT NOT NULL,
    hostname        TEXT NOT NULL,
    ip_address      INET NOT NULL,
    port            INTEGER NOT NULL DEFAULT 443,
    capacity        INTEGER NOT NULL DEFAULT 10000,
    active_tunnels  INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'active',
    metadata        JSONB,
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
