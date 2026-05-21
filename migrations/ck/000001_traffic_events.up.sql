CREATE TABLE traffic_events (
    timestamp       DateTime64(3) CODEC(DoubleDelta),
    organization_id UUID,
    tunnel_id       UUID,
    connection_id   String,
    protocol        LowCardinality(String),
    direction       LowCardinality(String),
    bytes           UInt64,
    method          LowCardinality(String),
    path            String,
    status_code     UInt16,
    client_ip       IPv6,
    client_country  LowCardinality(String),
    duration_ms     UInt32,
    error           LowCardinality(String)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (organization_id, tunnel_id, timestamp)
TTL timestamp + INTERVAL 30 DAY;
