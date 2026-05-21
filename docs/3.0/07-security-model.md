# OmniTun 3.0 — 管理后台安全模型

> **定位**：本文档是 [01-admin-console.md](./01-admin-console.md) 的安全补充，定义管理后台的认证体系、授权模型与数据隔离策略。

---

## 一、认证体系

管理后台使用**独立于用户端的认证链路**，确保 Super Admin 账号不与普通用户共享认证基础设施。

| 组件 | 方案 | 说明 |
|------|------|------|
| Admin 登录 | 独立 JWT (HMAC-SHA256) | 与用户 JWT 使用不同的签名密钥对，签名密钥定期轮换（90 天） |
| Admin 签名密钥 | `ADMIN_JWT_SECRET` | 存储在 Vault / 环境变量中，不进入代码仓库 |
| Session 超时 | 30 分钟绝对超时 | 无 Refresh Token，超时后强制重新登录 |
| 空闲超时 | 15 分钟无操作 | 最近一次 API 调用后 15 分钟无活动则 Session 失效 |
| MFA | 强制 TOTP (RFC 6238) | 所有 Super Admin 账号创建时必须绑定 TOTP，不可跳过 |
| MFA 恢复码 | 10 个一次性恢复码 | 管理员首次绑定 MFA 时生成，SHA-256 哈希存储 |
| 登录限流 | 5 次失败 / 15 分钟 / IP | 超过阈值后 IP 被临时锁定 30 分钟 |
| IP 白名单 | Office VPN / 办公网段 CIDR | 仅允许配置的 CIDR 范围访问 `admin.omnitun.io`，在反向代理层（Nginx/Caddy）强制校验 |
| SSO 接入 | 禁止 | Super Admin 必须使用独立账号登录，禁止通过 OIDC SSO 接入 |

### 1.1 Admin JWT Claims

```json
{
  "iss": "omnitun-admin",
  "sub": "sa_3k2j1h4x",
  "role": "full_admin",
  "iat": 1716300000,
  "exp": 1716301800,
  "jti": "unique-token-id",
  "mfa_verified": true,
  "ip": "10.0.0.42"
}
```

### 1.2 登录流程

```
1. POST /api/admin/v1/auth/login  { email, password }
2. 验证 IP ∈ 白名单 → 否则直接拒绝 (403)
3. 验证邮箱 + 密码 → 失败计数 +1
4. 若账号已绑定 MFA → 返回 { require_mfa: true, mfa_token: "..." }
5. POST /api/admin/v1/auth/mfa/verify { mfa_token, totp_code }
6. 验证 TOTP → 签发 Admin JWT（含 mfa_verified: true）
7. 返回 JWT + Session 信息
```

### 1.3 Session 管理

| 属性 | 值 |
|------|-----|
| 存储后端 | Valkey（Session ID → Admin JWT 映射） |
| Session ID 格式 | `sess_` + 32 字节随机 hex |
| Cookie 名称 | `admin_session` |
| Cookie 属性 | `HttpOnly; Secure; SameSite=Strict; Domain=admin.omnitun.io; Path=/` |
| 并发 Session 限制 | 每个 Admin 账号最多 3 个活跃 Session |

---

## 二、授权模型

### 2.1 Super Admin 角色层级

```
Super Admin 角色层级（权限从高到低）：

  Root Admin
  ├── 完全权限：所有操作，含删除组织、修改系统配置、管理其他 Admin 账号
  ├── 最多 2 人持有（由董事会 / CTO 审批授予）
  └── 所有操作要求双人确认（4-eyes principle）用于：删除组织、退役 Relay、吊销系统证书

  Full Admin
  ├── 管理用户/组织/Relay/安全/公告/Feature Flag
  ├── 不可删除组织（需 Root Admin 审批）
  └── 不可管理其他 Super Admin 账号

  Security Admin
  ├── 访问安全中心（Abuse Rules / Reports / IP Blacklist / Security Events）
  ├── 访问全局审计日志
  └── 不可管理组织/用户/Relay

  Infrastructure Admin
  ├── 管理 Relay 节点（注册/Drain/Maintenance/退役）
  ├── 管理证书（系统证书 + 租户证书）
  └── 管理系统配置（速率限制/维护模式/日志级别）

  Read-Only Ops
  ├── 查看所有仪表板和组织/用户/Relay 详情
  └── 不可执行任何写操作（POST/PUT/DELETE 返回 403）
```

### 2.2 角色-权限矩阵

| 权限 | Root | Full | Security | Infra | ReadOnly |
|------|------|------|----------|-------|----------|
| 查看全局仪表板 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 查看组织列表/详情 | ✅ | ✅ | ❌ | ❌ | ✅ |
| 冻结/解冻组织 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 删除组织 | ✅ | ❌ | ❌ | ❌ | ❌ |
| Impersonate (模拟登录) | ✅ | ✅ | ❌ | ❌ | ❌ |
| 管理用户（禁用/重置密码/强退） | ✅ | ✅ | ❌ | ❌ | ❌ |
| 管理 Relay 节点 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 管理证书 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 管理滥用规则 | ✅ | ✅ | ✅ | ❌ | ❌ |
| 处理举报队列 | ✅ | ✅ | ✅ | ❌ | ❌ |
| 管理 IP 黑名单 | ✅ | ✅ | ✅ | ❌ | ❌ |
| 查看安全事件 | ✅ | ✅ | ✅ | ❌ | ✅ |
| 管理 Feature Flag | ✅ | ✅ | ❌ | ❌ | ❌ |
| 管理系统公告 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 修改系统配置 | ✅ | ❌ | ❌ | ✅ | ❌ |
| 管理其他 Admin 账号 | ✅ | ❌ | ❌ | ❌ | ❌ |
| 查看全局审计日志 | ✅ | ✅ | ✅ | ❌ | ✅ |

---

## 三、数据隔离

### 3.1 基本原则

管理后台可访问**所有租户的元数据和运营数据**，但受以下约束：

| 约束 | 说明 |
|------|------|
| **不可查看隧道 payload** | 管理后台仅能查看隧道元数据（名称、状态、域名、流量统计），无法解密或查看隧道中传输的实际内容 |
| **不可修改业务配置** | 管理后台不能修改租户的隧道配置、网络拓扑、P2P 设置等业务数据 |
| **全量审计记录** | 管理后台内的每一次操作（查看、修改、删除、模拟登录）均写入 `admin_audit_logs` 表 |
| **Impersonate 双份审计** | 模拟登录操作在 `admin_audit_logs` 和租户 `audit_logs` 中各生成一条记录，租户端日志中可见 "Admin (xxx) impersonated you" |

### 3.2 Admin 审计日志表结构

```sql
CREATE TABLE admin_audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id        UUID NOT NULL,              -- Super Admin 账号 ID
    admin_email     VARCHAR(255) NOT NULL,       -- 操作时冗余，防止账号删除后无法追溯
    admin_role      VARCHAR(50) NOT NULL,        -- 操作时的角色 (root/full/security/infra/readonly)
    action          VARCHAR(100) NOT NULL,       -- 操作类型 (org.freeze, user.disable, relay.drain, ...)
    resource_type   VARCHAR(50) NOT NULL,        -- 资源类型 (organization, user, relay_node, ...)
    resource_id     UUID,                        -- 目标资源 ID
    resource_name   VARCHAR(255),                -- 目标资源名称（冗余）
    org_id          UUID,                        -- 所属组织 ID（如适用）
    detail          JSONB,                       -- 操作详情（Before/After diff）
    impersonate_id  UUID,                        -- 若为 Impersonate 操作，被模拟用户 ID
    ip_address      INET NOT NULL,               -- 操作者 IP
    user_agent      TEXT,                        -- 操作者 User-Agent
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    
    INDEX idx_admin_audit_admin_id (admin_id),
    INDEX idx_admin_audit_action (action),
    INDEX idx_admin_audit_created_at (created_at DESC),
    INDEX idx_admin_audit_org_id (org_id),
    INDEX idx_admin_audit_resource (resource_type, resource_id)
);
```

### 3.3 审计日志保留策略

| 级别 | 保留期 | 存储 |
|------|--------|------|
| 热存储（PostgreSQL） | 90 天 | 可即时查询、Dashboard 展示 |
| 温存储（ClickHouse） | 3 年 | 支持聚合查询，用于合规审计报告 |
| 冷存储（MinIO/S3） | 7 年 | 压缩归档，按需恢复，用于法务取证 |

---

## 四、网络安全

### 4.1 管理后台网络拓扑

```
                         Internet
                            │
                            ▼
                  ┌─────────────────┐
                  │   Nginx/Caddy    │  ← IP 白名单准入控制
                  │  (reverse proxy) │
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │ admin.omnitun.io │  ← 独立前端应用 (SPA)
                  │  (Vite + React)  │
                  └────────┬────────┘
                           │ API calls (/api/admin/v1/*)
                  ┌────────▼────────┐
                  │  API Gateway     │  ← Admin JWT 验证中间件
                  │  (统一网关)       │     + 角色权限校验
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │  Admin Service   │  ← Admin 业务逻辑层
                  │  (Go 微服务)      │
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │   PostgreSQL     │  ← admin_audit_logs + 业务数据
                  └─────────────────┘
```

### 4.2 通信安全

| 链路 | 协议 | 说明 |
|------|------|------|
| 浏览器 → Nginx | HTTPS (TLS 1.3) | 证书由 Let's Encrypt 签发 |
| Nginx → API Gateway | HTTPS (mTLS) | 双向 TLS 认证 |
| API Gateway → Admin Service | gRPC + mTLS | 服务间通信加密 |
| Admin Service → PostgreSQL | TLS 1.3 | 数据库连接加密 |

### 4.3 IP 白名单实施

```
Nginx 配置示例：

allow 10.0.0.0/8;       # Internal network
allow 172.16.0.0/12;    # Docker / K8s internal
allow 192.168.0.0/16;   # Office VPN
deny all;                # 拒绝所有其他来源

# 管理后台路径匹配
location / {
    if ($allowed_ip = 0) {
        return 403;
    }
    proxy_pass http://admin_frontend;
}
```

---

## 五、密钥管理

| 密钥 | 存储位置 | 轮换周期 | 说明 |
|------|----------|----------|------|
| `ADMIN_JWT_SECRET` | HashiCorp Vault / K8s Secret | 90 天 | Admin JWT 签名密钥 |
| TOTP Seed | PostgreSQL (AES-256-GCM 加密) | 不变 | 每个 Admin 的 MFA 种子 |
| MFA Recovery Codes | PostgreSQL (SHA-256 Hash) | 使用后作废 | 一次性恢复码 |
| API Gateway → Admin Service mTLS Cert | Vault PKI | 30 天 | 自动轮换 |
| Session Encryption Key | Vault | 90 天 | Valkey Session 数据加密 |

---

## 六、版本记录

| 版本 | 日期 | 作者 | 变更 |
|------|------|------|------|
| v1.0 | 2026-05-21 | OmniTun Security Team | 管理后台安全模型首次发布 |
