ALTER TABLE tunnels ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_tunnels_tags ON tunnels USING GIN(tags) WHERE array_length(tags, 1) > 0;
