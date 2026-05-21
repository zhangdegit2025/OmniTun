CREATE TABLE mesh_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    network_id      UUID NOT NULL REFERENCES mesh_networks(id),
    tunnel_id       UUID REFERENCES tunnels(id),
    name            TEXT NOT NULL,
    ip_address      INET NOT NULL,
    public_key      TEXT NOT NULL,
    nat_type        TEXT,
    endpoints       JSONB,
    status          TEXT NOT NULL DEFAULT 'offline',
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE(network_id, ip_address)
);
