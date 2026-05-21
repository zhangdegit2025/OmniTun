CREATE TABLE feature_flags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL DEFAULT 'boolean',
    value       JSONB NOT NULL DEFAULT '{"enabled":true}',
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO feature_flags (key, name, description, type, value, enabled) VALUES
    ('p2p_enabled', 'P2P Mode', 'Enable peer-to-peer connections', 'boolean', '{"enabled":true}', true),
    ('mesh_beta', 'Mesh Network Beta', 'Enable mesh networking beta features', 'boolean', '{"enabled":true}', true)
ON CONFLICT (key) DO NOTHING;
