# OmniTun — 数据模型与存储设计

> **修订说明**
> - 2026-05-20: 修正 api_key 加密方案（bcrypt → HMAC-SHA256）；新增"二、数据流架构"章节（CDC 写入路径、读取路径、缓存策略）

## 一、存储选型

| 存储系统 | 用途 | 数据特征 | 容量预估 |
|----------|------|----------|----------|
| **PostgreSQL 16** | OLTP 主库（用户、隧道、配置、账单） | 强一致、关系型 | 初始 100GB |
| **Valkey 8 (Redis)** | 缓存（Session、速率限制、实时状态） | 临时、高吞吐 | 初始 8GB |
| **ClickHouse 24** | 分析（流量日志、审计日志、分析查询） | 追加写、列存 | 初始 500GB |
| **MinIO / S3** | 对象存储（TLS 证书、会话录制） | 不可变、二进制 | 初始 200GB |
| **NATS JetStream** | 消息队列（事件总线、Relay 配置推送） | 流式、短暂保留 | 内存 2GB |

---

## 二、数据流架构

### 2.1 写入路径

```
写入流程：

1. 应用服务（Orchestrator/Relay）
   │
   ├──▶ PostgreSQL (OLTP) ←── 同步写入
   │         │
   │         └──▶ WAL (Write-Ahead Log)
   │
   └──▶ ClickHouse (OLAP) ←── Debezium CDC 异步同步
             │
             └──▶ Kafka/Redpanda (缓冲)
                   │
                   └──▶ ClickHouse Sink Connector
```

**CDC 策略**：
- 使用 Debezium + Kafka Connect 监听 PG WAL
- 延迟目标：< 5 秒（P99）
- 转换逻辑：将 PG 行转换为 ClickHouse INSERT
- 失败处理：Kafka 保留 7 天，重试机制

### 2.2 读取路径

```
读取流程：

1. Dashboard 实时流量
   └──▶ Valkey (缓存) ←── WebSocket 推送

2. Dashboard 历史统计
   └──▶ ClickHouse (直接查询)

3. API 流量聚合
   └──▶ PostgreSQL (定时聚合任务，每 5 分钟)
```

### 2.3 缓存策略

| 数据类型 | 缓存位置 | TTL | 失效机制 |
|----------|----------|-----|----------|
| Session | Valkey | 24h | 主动删除 |
| 隧道状态 | Valkey | 无 TTL | 配置变更时推送更新 |
| 域名解析 | Valkey | 60s | TTL 过期或配置变更 |
| Relay 心跳 | Valkey | 30s | TTL 自动过期 |
| 证书内容 | Valkey | 1h | ACME 续期时推送 |

---

## 三、PostgreSQL 核心表结构

### 2.1 租户与用户

```sql
CREATE TABLE organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL UNIQUE,   -- URL-safe identifier
    plan          TEXT NOT NULL DEFAULT 'free', -- free/pro/team/business/enterprise
    billing_email TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ              -- soft delete
);

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    email           TEXT NOT NULL,
    password_hash   TEXT,                  -- NULL for OAuth-only users
    display_name    TEXT NOT NULL DEFAULT '',
    avatar_url      TEXT,
    role            TEXT NOT NULL DEFAULT 'member', -- owner/admin/editor/viewer
    auth_provider   TEXT NOT NULL DEFAULT 'email',  -- email/github/google/oidc
    auth_provider_id TEXT,                 -- external ID for OAuth
    mfa_enabled     BOOLEAN NOT NULL DEFAULT false,
    mfa_secret      TEXT,                  -- TOTP secret (encrypted)
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    UNIQUE(organization_id, email)
);

CREATE INDEX idx_users_org ON users(organization_id) WHERE deleted_at IS NULL;
```

### 2.2 工作区（Workspace）

```sql
CREATE TABLE workspaces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    UNIQUE(organization_id, slug)
);
```

### 2.3 API 密钥

```sql
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id         UUID REFERENCES users(id),
    name            TEXT NOT NULL,
    key_prefix      TEXT NOT NULL,          -- e.g. "ot_sk_" (first 8 chars for display)
    key_hash        TEXT NOT NULL UNIQUE,   -- HMAC-SHA256 tag of full key (not bcrypt)
    scopes          JSONB NOT NULL DEFAULT '["*"]',
    workspace_id    UUID REFERENCES workspaces(id), -- null = org-wide
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);
```

### 2.4 隧道（Tunnel）

```sql
CREATE TABLE tunnels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,           -- generated subdomain
    protocol        TEXT NOT NULL,           -- http/tcp/udp/icmp/ssh/rdp
    local_port      INTEGER NOT NULL,
    local_host      TEXT NOT NULL DEFAULT '127.0.0.1',

    -- 公网入口配置
    custom_domain   TEXT,                    -- user's custom domain
    region          TEXT NOT NULL DEFAULT 'auto',

    -- TLS
    tls_mode        TEXT NOT NULL DEFAULT 'edge', -- edge/passthrough
    tls_cert_id     UUID REFERENCES certificates(id),

    -- 访问控制
    auth_mode       TEXT NOT NULL DEFAULT 'none', -- none/basic/oauth/ip_whitelist
    auth_config     JSONB,                   -- credentials/policies

    -- 高级选项
    compression     BOOLEAN NOT NULL DEFAULT true,
    buffer_size     INTEGER NOT NULL DEFAULT 65536,
    max_connections INTEGER NOT NULL DEFAULT 100,
    idle_timeout    INTEGER NOT NULL DEFAULT 300, -- seconds

    -- 运行时状态 (缓存到 Redis，PG 做持久化)
    status          TEXT NOT NULL DEFAULT 'stopped', -- stopped/starting/active/error
    relay_id        UUID,                    -- 当前分配的 Relay
    agent_id        TEXT,                    -- 当前连接的 Agent 标识
    agent_version   TEXT,
    agent_ip        INET,
    connected_at    TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,

    -- 流量统计 (从 ClickHouse 聚合的快照)
    bytes_in_total  BIGINT NOT NULL DEFAULT 0,
    bytes_out_total BIGINT NOT NULL DEFAULT 0,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_tunnels_org_ws ON tunnels(organization_id, workspace_id);
CREATE INDEX idx_tunnels_domain ON tunnels(custom_domain) WHERE custom_domain IS NOT NULL;
CREATE INDEX idx_tunnels_slug ON tunnels(slug);
```

### 2.5 Mesh 网络

```sql
CREATE TABLE mesh_networks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    name            TEXT NOT NULL,
    cidr            CIDR NOT NULL,           -- e.g. 10.42.0.0/16
    encryption_key  BYTEA NOT NULL,          -- encrypted WireGuard private key
    dns_enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE mesh_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    network_id      UUID NOT NULL REFERENCES mesh_networks(id),
    tunnel_id       UUID REFERENCES tunnels(id), -- optional link
    name            TEXT NOT NULL,
    ip_address      INET NOT NULL,           -- assigned IP in mesh CIDR
    public_key      TEXT NOT NULL,           -- WireGuard public key
    nat_type        TEXT,                    -- STUN detected NAT type
    endpoints       JSONB,                   -- [{"ip":"x.x.x.x","port":12345}]
    status          TEXT NOT NULL DEFAULT 'offline',
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    UNIQUE(network_id, ip_address)
);
```

### 2.6 证书

```sql
CREATE TABLE certificates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    domain          TEXT NOT NULL,
    certificate_pem TEXT NOT NULL,           -- full chain
    private_key_pem TEXT NOT NULL,           -- encrypted at rest
    issuer          TEXT NOT NULL,           -- lets_encrypt / custom
    not_before      TIMESTAMPTZ NOT NULL,
    not_after       TIMESTAMPTZ NOT NULL,
    auto_renew      BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_certs_domain ON certificates(domain);
CREATE INDEX idx_certs_expiry ON certificates(not_after) WHERE auto_renew = true;
```

### 2.7 审计日志

```sql
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    user_id         UUID,
    action          TEXT NOT NULL,           -- tunnel.create / user.invite / etc.
    resource_type   TEXT NOT NULL,           -- tunnel / user / workspace / network
    resource_id     UUID,
    details         JSONB,                   -- diff / context
    client_ip       INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_org_time ON audit_logs(organization_id, created_at DESC);
CREATE INDEX idx_audit_action ON audit_logs(action, created_at DESC);
```

### 2.8 订阅与计费

```sql
CREATE TABLE subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) UNIQUE,
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT,
    plan            TEXT NOT NULL,           -- free/pro/team/business/enterprise
    status          TEXT NOT NULL DEFAULT 'active', -- active/past_due/canceled
    current_period_start TIMESTAMPTZ,
    current_period_end   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE usage_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    metric          TEXT NOT NULL,           -- bandwidth / tunnels / connections
    quantity        BIGINT NOT NULL,
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_usage_org_period ON usage_records(organization_id, period_start);
```

### 2.9 Relay 节点

```sql
CREATE TABLE relay_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    region          TEXT NOT NULL,            -- ap-southeast-1 / us-east-1 / eu-central-1
    hostname        TEXT NOT NULL,
    ip_address      INET NOT NULL,
    port            INTEGER NOT NULL DEFAULT 443,
    capacity        INTEGER NOT NULL DEFAULT 10000, -- max tunnels
    active_tunnels  INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'active',  -- active / maintenance / offline
    metadata        JSONB,                    -- extra tags, labels
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 三、ClickHouse 分析表结构

```sql
-- 流量日志（每条 HTTP 请求 / TCP 连接）
CREATE TABLE traffic_events (
    timestamp       DateTime64(3) CODEC(DoubleDelta),
    organization_id UUID,
    tunnel_id       UUID,
    connection_id   String,
    protocol        LowCardinality(String),
    direction       LowCardinality(String), -- ingress / egress
    bytes           UInt64,
    method          LowCardinality(String), -- GET/POST or NULL for non-HTTP
    path            String,                 -- or empty for non-HTTP
    status_code     UInt16,                -- 0 for non-HTTP
    client_ip       IPv6,
    client_country  LowCardinality(String),
    duration_ms     UInt32,
    error           LowCardinality(String) -- '' = success
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (organization_id, tunnel_id, timestamp)
TTL timestamp + INTERVAL 30 DAY;

-- 隧道会话（每次启动 → 停止为一个 session）
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
```

---

## 四、Valkey 缓存设计

| Key Pattern | 类型 | TTL | 说明 |
|-------------|------|-----|------|
| `session:{jti}` | String (JSON) | 24h | JWT session data |
| `ratelimit:{api_key}:{minute}` | String (counter) | 60s | Rate limit counter |
| `tunnel:status:{tunnel_id}` | String | — | Runtime tunnel status |
| `tunnel:connections:{tunnel_id}` | Set | — | Active connection IDs |
| `relay:heartbeat:{relay_id}` | String | 30s | Relay health heartbeat |
| `relay:tunnels:{relay_id}` | Set | — | Tunnels on this relay |
| `dns:resolve:{domain}` | String (JSON) | 60s | Domain → Tunnel ID cache |
| `cert:{cert_id}` | String (PEM) | 1h | Certificate cache for relays |
| `agent:sub:{agent_id}` | PubSub Channel | — | Control messages to agent |
| `event:bus` | Stream (via NATS) | — | Internal event bus |

---

## 五、数据分区与归档策略

| 表 | 分区策略 | 热数据 | 温数据 | 冷数据(归档) |
|-----|----------|--------|--------|-------------|
| audit_logs | 按月范围分区 | 3个月 (PG) | 12个月 (PG) | S3 (Parquet) |
| traffic_events | 按月分区（CK） | 30天 (CK) | 90天 (CK) | S3 (Parquet) |
| tunnel_sessions | TTL 自动删除 | 90天 (CK) | — | 聚合到 PG |
| usage_records | 按月范围分区 | 12个月 (PG) | 36个月 (PG) | 删除 |

---

## 六、敏感数据加密

| 数据 | 加密方案 | 密钥管理 |
|------|----------|----------|
| password_hash | bcrypt (cost=12) | N/A |
| api_key | HMAC-SHA256（验证时重新计算 tag 比较） | N/A（不可逆比较） |
| private_key_pem | AES-256-GCM（envelope encryption） | KMS (Vault/Cloud KMS) |
| mfa_secret | AES-256-GCM | KMS |
| encryption_key (mesh) | AES-256-GCM | KMS |
