CREATE TABLE tunnel_sessions (
    tunnel_id       UUID,
    started_at      DateTime64(3),
    ended_at        DateTime64(3),
    duration_secs   UInt32,
    bytes_in        UInt64,
    bytes_out       UInt64,
    relay_id        UUID,
    agent_version   LowCardinality(String),
    disconnect_reason LowCardinality(String)
)
ENGINE = MergeTree()
ORDER BY (tunnel_id, started_at)
TTL ended_at + INTERVAL 90 DAY;
