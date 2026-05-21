# OmniTun — API 与协议设计

> **修订说明**
> - 修订时间：2026-05-20
> - 修订内容：
>   - 增加"1.3 安全说明"章节，明确 API Key、JWT、传输加密规范

## 一、API 设计总则

### 1.1 协议分层

| 层级 | 协议 | 用途 | 受众 |
|------|------|------|------|
| **Public API** | REST + WebSocket (OpenAPI 3.1) | Dashboard、CLI、第三方集成 | 外部开发者 |
| **Internal RPC** | gRPC + Protobuf | 控制面服务间通信 | 内部服务 |
| **Agent Control** | WebSocket + JSON (自定义协议) | 控制面 ↔ Agent 指令通道 | Agent |
| **Agent Data** | QUIC / WebSocket + Vector Stream | Relay ↔ Agent 数据通道 | Agent |

### 1.2 公约

- **Base URL**：`https://api.omnitun.io/v1`
- **认证**：`Authorization: Bearer <JWT>` 或 `X-API-Key: ot_sk_<key>`
- **内容类型**：`application/json`（请求/响应）
- **时间格式**：ISO 8601（`2026-05-20T14:00:00Z`）
- **分页**：Cursor-based（`?cursor=<token>&limit=50`），避免 OFFSET
- **错误格式**：

```json
{
  "error": {
    "code": "tunnel_not_found",
    "message": "Tunnel with id 'xxx' not found",
    "details": {}
  }
}
```

- **幂等性**：所有 POST/PUT/DELETE 支持 `Idempotency-Key` header
- **速率限制头**：`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- **字段命名**：snake_case（JSON）

### 1.3 安全说明

- **API Key 存储**：使用 HMAC-SHA256 验证，不存储明文或 bcrypt
- **JWT 签名**：RS256，密钥定期轮换（90 天）
- **传输加密**：全部 HTTPS/TLS 1.3

---

## 二、REST API 端点

### 2.1 认证

```
POST   /v1/auth/register          # 邮箱注册
POST   /v1/auth/login             # 邮箱登录 → JWT
POST   /v1/auth/refresh           # 刷新 Token
POST   /v1/auth/logout            # 撤销会话
GET    /v1/auth/me                # 当前用户信息
POST   /v1/auth/mfa/enroll        # 注册 MFA（返回 TOTP 密钥 + QR）
POST   /v1/auth/mfa/verify        # 验证并启用 MFA
DELETE /v1/auth/mfa               # 禁用 MFA
POST   /v1/auth/oauth/github      # GitHub OAuth 登录
POST   /v1/auth/oauth/google      # Google OAuth 登录
POST   /v1/auth/oauth/callback    # OAuth 回调（通用）
POST   /v1/auth/password/reset    # 发送重置邮件
PUT    /v1/auth/password/reset    # 执行密码重置（带 token）
```

**POST /v1/auth/login 请求体**：
```json
{
  "email": "user@example.com",
  "password": "secure-password",
  "mfa_code": "123456"
}
```

**响应**：
```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "rt_xxxxxxxxxxxxx",
  "expires_in": 3600,
  "user": {
    "id": "550e8400-...",
    "email": "user@example.com",
    "display_name": "User Name",
    "organization": { "id": "...", "name": "ACME Corp" },
    "role": "owner"
  }
}
```

### 2.2 隧道（Tunnel）

```
GET    /v1/workspaces/{ws_id}/tunnels           # 隧道列表
POST   /v1/workspaces/{ws_id}/tunnels           # 创建隧道
GET    /v1/tunnels/{tunnel_id}                  # 隧道详情
PATCH  /v1/tunnels/{tunnel_id}                  # 更新隧道配置
DELETE /v1/tunnels/{tunnel_id}                  # 删除隧道
POST   /v1/tunnels/{tunnel_id}/start            # 启动隧道
POST   /v1/tunnels/{tunnel_id}/stop             # 停止隧道
POST   /v1/tunnels/{tunnel_id}/restart          # 重启隧道
GET    /v1/tunnels/{tunnel_id}/logs             # 流量日志（cursor分页）
GET    /v1/tunnels/{tunnel_id}/stats            # 流量统计
GET    /v1/tunnels/{tunnel_id}/connections      # 活跃连接列表
GET    /v1/tunnels/{tunnel_id}/sessions         # 历史会话列表
POST   /v1/tunnels/{tunnel_id}/clone            # 克隆隧道
```

**POST /v1/workspaces/{ws_id}/tunnels 请求体**：
```json
{
  "name": "My API Tunnel",
  "protocol": "http",
  "local_port": 8080,
  "local_host": "127.0.0.1",
  "custom_domain": "api.example.com",
  "tls_mode": "edge",
  "auth_mode": "basic",
  "auth_config": {
    "username": "admin",
    "password": "generated-secure-pass"
  },
  "region": "ap-southeast-1",
  "max_connections": 500
}
```

**隧道状态机**：
```
          create
            │
            ▼
        ┌──────┐   start    ┌────────┐  connected  ┌────────┐
        │stopped│──────────▶│starting│────────────▶│ active │
        └──────┘            └────────┘              └───┬────┘
            ▲                     │                     │
            │     stop/fail       │      disconnect     │ stop
            └─────────────────────┘◀────────────────────┘
                                  │
                                  ▼
                             ┌────────┐
                             │  error │
                             └────────┘
```

### 2.3 Mesh 网络

```
GET    /v1/workspaces/{ws_id}/networks           # 网络列表
POST   /v1/workspaces/{ws_id}/networks           # 创建网络
GET    /v1/networks/{net_id}                     # 网络详情
PATCH  /v1/networks/{net_id}                     # 更新网络
DELETE /v1/networks/{net_id}                     # 删除网络
GET    /v1/networks/{net_id}/nodes               # 节点列表
POST   /v1/networks/{net_id}/nodes               # 手动添加节点
DELETE /v1/networks/{net_id}/nodes/{node_id}     # 移除节点
POST   /v1/networks/{net_id}/invite              # 生成加入邀请
POST   /v1/networks/{net_id}/join                # 通过邀请码加入
```

### 2.4 工作区

```
GET    /v1/workspaces                  # 当前用户的工作区列表
POST   /v1/workspaces                  # 创建工作区
GET    /v1/workspaces/{ws_id}          # 工作区详情
PATCH  /v1/workspaces/{ws_id}          # 更新工作区
DELETE /v1/workspaces/{ws_id}          # 删除工作区
```

### 2.5 成员管理

```
GET    /v1/workspaces/{ws_id}/members          # 成员列表
POST   /v1/workspaces/{ws_id}/members          # 邀请成员
PATCH  /v1/workspaces/{ws_id}/members/{uid}    # 修改角色
DELETE /v1/workspaces/{ws_id}/members/{uid}    # 移除成员
POST   /v1/workspaces/{ws_id}/members/{uid}/resend-invite
```

### 2.6 API 密钥

```
GET    /v1/api-keys                   # API Key 列表
POST   /v1/api-keys                   # 创建 API Key
DELETE /v1/api-keys/{key_id}          # 撤销 API Key
```

### 2.7 域名与证书

```
GET    /v1/domains                               # 自定义域名列表
POST   /v1/domains                               # 添加域名
GET    /v1/domains/{domain_id}                   # 域名详情
GET    /v1/domains/{domain_id}/verification      # 域名验证状态
POST   /v1/domains/{domain_id}/verify            # 触发验证
DELETE /v1/domains/{domain_id}                   # 移除域名
GET    /v1/domains/{domain_id}/certificate       # 证书信息
POST   /v1/domains/{domain_id}/certificate       # 上传自定义证书
```

### 2.8 审计

```
GET    /v1/audit-logs                              # 审计日志（支持筛选）
  ?action=tunnel.create
  &resource_type=tunnel
  &user_id=xxx
  &from=2026-01-01T00:00:00Z
  &to=2026-01-31T23:59:59Z
  &cursor=xxx
  &limit=50
```

### 2.9 计费

```
GET    /v1/billing/plan                  # 当前计划
GET    /v1/billing/usage                 # 当前周期用量
GET    /v1/billing/usage/history         # 历史用量
GET    /v1/billing/invoices              # 发票列表
POST   /v1/billing/checkout              # 升级计划（返回 Stripe Checkout URL）
POST   /v1/billing/portal                # 管理订阅（返回 Stripe Customer Portal）
POST   /v1/billing/cancel                # 取消订阅
```

### 2.10 Admin

```
GET    /v1/admin/orgs                    # 组织列表（仅 Super Admin）
PATCH  /v1/admin/orgs/{org_id}           # 更新组织
GET    /v1/admin/relays                  # Relay 节点列表
POST   /v1/admin/relays                  # 注册新的 Relay 节点
PATCH  /v1/admin/relays/{relay_id}       # 更新 Relay
GET    /v1/admin/metrics                 # 全局指标
GET    /v1/admin/flags                   # Feature flags
```

---

## 三、WebSocket 端点

### 3.1 Agent ↔ Control Plane

**连接**：`wss://control.omnitun.io/agent/v1/connect`

**消息格式**：

```json
{
  "type": "agent.message_type",
  "id": "unique-message-id",
  "timestamp": "2026-05-20T14:00:00Z",
  "payload": {}
}
```

**Agent → Server 消息**：

| Type | 触发时机 |
|------|----------|
| `agent.hello` | 首次连接，携带 agent_id 和 JWT |
| `agent.tunnel.start` | 隧道启动请求 |
| `agent.tunnel.stop` | 隧道停止请求 |
| `agent.tunnel.status` | 定期状态报告（心跳） |
| `agent.tunnel.error` | 隧道异常报告 |
| `agent.network.join` | 请求加入 Mesh 网络 |
| `agent.network.leave` | 离开 Mesh 网络 |
| `agent.stun.result` | STUN 探测结果上报 |

**Server → Agent 消息**：

| Type | 触发时机 |
|------|----------|
| `server.tunnel.config` | Tunnel 配置变更 |
| `server.tunnel.start_ack` | 隧道启动确认（含 Relay 地址） |
| `server.tunnel.stop_cmd` | 强制停止隧道 |
| `server.tunnel.update` | 配置热更新 |
| `server.network.p2p_offer` | P2P 连接信令（SDP offer） |
| `server.network.p2p_answer` | P2P 连接信令（SDP answer） |
| `server.shutdown` | 关闭 Agent 指令 |

### 3.2 Dashboard ↔ Control Plane

**连接**：`wss://control.omnitun.io/dashboard/v1/realtime`

**消息类型**：

| Channel | 内容 |
|---------|------|
| `tunnel.{id}.connections` | 实时连接/断开事件 |
| `tunnel.{id}.traffic` | 实时流量统计（1s 聚合） |
| `workspace.{id}.events` | 工作区事件流（隧道变更、成员变更） |
| `user.notifications` | 个人通知 |

---

## 四、Agent ↔ Relay 数据协议

### 4.1 QUIC 协议栈

```
┌─────────────────────────┐
│   Vector Stream Layer   │  ← 自定义帧协议（二进制）
├─────────────────────────┤
│   zstd Compression      │
├─────────────────────────┤
│   TLS 1.3               │  ← mTLS（双向证书验证）
├─────────────────────────┤
│   QUIC Transport        │  ← 多路复用、0-RTT、连接迁移
├─────────────────────────┤
│   UDP                   │
└─────────────────────────┘
```

### 4.2 连接建立握手

```
Agent                                    Relay
  │                                        │
  │ ──── QUIC Initial (TLS ClientHello) ──▶│
  │       含 Agent 证书                      │
  │                                        │
  │ ◀─── QUIC Handshake complete ──────    │
  │       含 Relay 证书                      │
  │                                        │
  │ ──── CONTROL Frame ───────────────────▶│
  │       type: TUNNEL_CONNECT              │
  │       tunnel_id: xxx                    │
  │       token: "..." (短期票据)             │
  │                                        │
  │ ◀─── CONTROL Frame ──────────────────  │
  │       type: TUNNEL_CONNECT_ACK          │
  │       status: ok                        │
  │                                        │
  │ ════ 数据通道建立 ═══════════════════  │
```

### 4.3 帧类型定义（二进制）

```
CONTROL Frames (Frame Type = 0x01):
  - 0x01: TUNNEL_CONNECT        → Agent 请求建立隧道
  - 0x02: TUNNEL_CONNECT_ACK    → Relay 确认
  - 0x03: TUNNEL_CLOSE          → 关闭隧道
  - 0x04: KEEPALIVE             → 心跳

DATA Frames (Frame Type = 0x00):
  - 0x00: DATA                  → 二进制 payload

ERROR Frames (Frame Type = 0x03):
  - 0x00: PROTOCOL_ERROR
  - 0x01: TUNNEL_NOT_FOUND
  - 0x02: RATE_LIMITED
  - 0x03: INTERNAL_ERROR
```

---

## 五、SDK 设计

### 5.1 Go SDK 示例

```go
package main

import (
    "context"
    "fmt"
    omnitun "github.com/omnitun/omnitun-go"
)

func main() {
    client := omnitun.NewClient(omnitun.WithAPIKey("ot_sk_xxx"))

    // 创建隧道
    tunnel, err := client.Tunnels.Create(context.Background(), &omnitun.CreateTunnelParams{
        WorkspaceID: "ws_xxx",
        Name:        "my-api",
        Protocol:    omnitun.ProtocolHTTP,
        LocalPort:   8080,
        Domain:      "api.example.com",
    })

    // 启动隧道
    err = tunnel.Start(context.Background())
    fmt.Printf("Tunnel URL: %s\n", tunnel.URL())

    // 获取流量统计
    stats, _ := tunnel.Stats(context.Background(),
        omnitun.WithPeriod(omnitun.Period24h))
    fmt.Printf("24h traffic: %d bytes\n", stats.BytesTotal)
}
```

### 5.2 Python SDK 示例

```python
from omnitun import OmniTun
from omnitun.models import TunnelProtocol

client = OmniTun(api_key="ot_sk_xxx")

# 创建并启动隧道
tunnel = client.tunnels.create(
    workspace_id="ws_xxx",
    name="my-api",
    protocol=TunnelProtocol.HTTP,
    local_port=8080,
    domain="api.example.com",
)

tunnel.start()
print(f"Tunnel URL: {tunnel.url}")

# 上下文管理器（自动停止）
with client.tunnels.connect(workspace_id="ws_xxx", local_port=8080) as t:
    print(f"Tunnel URL: {t.url}")
    # 做你的事...
```
