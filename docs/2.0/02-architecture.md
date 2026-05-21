# OmniTun 2.0 — 满血版完整技术方案

## 一、端到端隧道数据路径

这是 2.0 最核心的交付——从"只管理元数据"变为"真实流量穿透"。

### 1.1 完整数据流

```
┌──────────┐                    ┌──────────┐                    ┌──────────┐
│ Browser  │                    │  Relay   │                    │  Agent   │
│          │                    │          │                    │          │
│ GET /api │──── HTTPS ────────▶│ Dispatcher│──── QUIC/WS ────▶│ localhost│
│          │                    │  Lookup   │                    │  :8080   │
│          │◀─── Response ──────│ Vector    │◀─── Response ─────│          │
│          │                    │  Frame    │                    │          │
└──────────┘                    └──────────┘                    └──────────┘

详细步骤：
1. 用户 localhost:8080 启动 Web 服务
2. CLI: omnitun http 8080
   → POST /v1/tunnels (REST API) 创建隧道
   → POST /v1/tunnels/{id}/start → Orchestrator 选择 Relay 节点
   → 返回 relay_address + tunnel_token
3. Agent 接收配置：
   → 建立到 Relay 的 QUIC 连接（回退 WebSocket）
   → 发送 TUNNEL_CONNECT 控制帧（tunnel_id + short-term token）
   → Relay 验证 token → 注册 tunnel_id → 返回 TUNNEL_CONNECT_ACK
4. 隧道就绪，状态 = active
5. Internet 请求到来：
   → Relay Edge 接收 HTTPS 请求
   → Dispatcher.Lookup(Host header) → 找到 tunnel_id → 找到 Agent Stream
   → 封装 Vector DataFrame（zstd 压缩）→ QUIC Stream 转发
   → Agent 接收 → 解压 → TCP 连接到 localhost:8080 → 转发请求
   → 读取响应 → 封装 Vector DataFrame → QUIC Stream 返回
   → Relay 解压 → HTTP Response 返回浏览器
```

### 1.2 各组件职责与改动

#### Control Plane（已有，需增强）
```
新增功能：
  - Tunnel Start 时分配 Relay + 生成短期 tunnel_token（HMAC-SHA256，5 分钟有效）
  - 下发 tunnel_token 到 Relay（gRPC 或 NATS）
  - Tunnel Stop 时通知 Relay 注销 tunnel_id
```

#### Relay Node（代码已有，需验证贯通）
```
已有代码：
  internal/relay/server.go     → QUIC + WS + HTTP 监听
  internal/relay/dispatcher.go → Host header → tunnel_id 查找
  internal/relay/proxy.go      → HTTP 反向代理 + WS upgrade
  internal/relay/stream.go     → 多路复用
  internal/relay/control_client.go → 控制面通信

需验证：
  - Relay 启动后能注册到控制面
  - 能接受 Agent 的 QUIC 连接
  - Dispatcher 能正确路由请求
  - Proxy 能正确转发 HTTP 请求
  - WebSocket upgrade 正常工作

改动点：
  - tunnel_token 验证逻辑（验证 HMAC + 过期检查）
  - Agent 连接池管理（agent_id → connection）
  - 真实流量统计（bytes_in / bytes_out 写入 PG 或 CK）
```

#### Agent Client（代码已有，需验证贯通）
```
已有代码：
  internal/control/agent.go       → WS 控制连接
  internal/control/tunnel_conn.go → QUIC/WS 数据通道
  internal/control/api.go         → REST API 客户端
  cmd/client/                     → CLI 入口

需验证：
  - CLI http 8080 → Agent 能建立到 Relay 的数据通道
  - 本地 TCP 连接转发正确
  - 断线重连 + 指数退避
  - 心跳保活

改动点：
  - 接收 tunnel_token 并用于 TUNNEL_CONNECT 帧
  - 实时流量统计上报
```

#### WebSocket Gateway（已有代码，需启动）
```
已有代码：
  internal/gateway/server.go   → WS 服务器
  internal/gateway/hub.go      → Agent 连接池
  internal/gateway/handler.go  → 消息路由
  internal/gateway/auth.go     → Agent 认证

改动点：
  - 集成到主 server 进程（共用端口，path=/gateway/agent）
  - 或在独立端口启动（:9443）
  - 转发 agent.tunnel.start 消息到 Tunnel Orchestrator
```

---

## 二、TLS 与域名系统

### 2.1 架构

```
                      ┌─────────────────────┐
                      │   Let's Encrypt      │
                      │   (ACME Server)      │
                      └──────────┬──────────┘
                                 │ DNS-01 Challenge
                      ┌──────────▼──────────┐
                      │   Certificate       │
                      │   Manager           │
                      │                     │
                      │  - ACME Client(lego)│
                      │  - 证书存储(S3/PG)  │
                      │  - 定时续期(60天)   │
                      │  - 推送到 Relay     │
                      └─────────────────────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
               ┌────▼────┐ ┌────▼────┐ ┌────▼────┐
               │ Relay 1 │ │ Relay 2 │ │ Relay N │
               └─────────┘ └─────────┘ └─────────┘
```

### 2.2 自定义域名流程

```
1. 用户通过 Dashboard/API 添加自定义域名 api.example.com
2. OmniTun 返回 DNS 验证要求：
   - 添加 CNAME: api.example.com → tunnel-xxx.omnitun-edge.com
   - 或添加 TXT: _omnitun-challenge.api.example.com → "verification-token"
3. OmniTun 定期检查 DNS 解析状态（每 30s，最多 30 分钟）
4. DNS 验证通过 → ACME 签发证书（DNS-01 challenge）
5. 证书存储到 S3 + PG certificates 表
6. 证书推送到对应 Relay 节点
7. 隧道状态更新为 tls_ready
```

### 2.3 证书生命周期

| 事件 | 触发 | 动作 |
|------|------|------|
| 创建隧道 | 用户指定自定义域名 | DNS 验证 + ACME 签发 |
| 续期 | 到期前 30 天 | 自动 ACME 续期 → 推送到 Relay |
| 删除隧道 | 用户删除隧道 | 保留证书 7 天（可能其他隧道在用）|
| Relay 启动 | Relay 注册 | 拉取该 Relay 所有活跃隧道的证书 |

---

## 三、企业安全架构

### 3.1 认证体系（接入已有代码）

```
现有代码：
  internal/auth/service.go   → 所有 gRPC 方法已完成
  internal/auth/jwt.go        → JWT RS256/HS256 签发验证
  internal/auth/mfa.go        → TOTP 生成验证
  internal/auth/oauth.go      → GitHub/Google OAuth
  internal/auth/middleware.go  → JWT + API Key 中间件
  internal/auth/repository.go  → 全部数据操作

接入 task（代码已存在，仅需接线）：
  1. API Key 中间件挂载到路由
  2. OAuth 回调端点注册
  3. MFA 验证插入登录流
  4. OIDC 端点对接 Okta/Azure AD 测试
  5. SAML 端点对接 Azure AD 测试
```

### 3.2 审计日志写入点

| 操作 | 写入时机 | 记录字段 |
|------|----------|----------|
| 用户注册 | Register 成功后 | action=user.register, resource=user |
| 用户登录 | Login 成功后 | action=user.login, resource=user |
| 隧道创建 | CreateTunnel 成功后 | action=tunnel.create, resource=tunnel |
| 隧道启动 | StartTunnel 成功后 | action=tunnel.start, resource=tunnel |
| 隧道停止 | StopTunnel 成功后 | action=tunnel.stop, resource=tunnel |
| 隧道删除 | DeleteTunnel 成功后 | action=tunnel.delete, resource=tunnel |
| API Key 创建 | CreateAPIKey 成功后 | action=apikey.create |
| API Key 撤销 | DeleteAPIKey 成功后 | action=apikey.revoke |
| SSO 配置变更 | 更新 OIDC/SAML 配置 | action=organization.sso_update |

---

## 四、P2P 与 Mesh 架构

### 4.1 NAT 穿透栈

```
优先级降级链：
  1. 直接连接（同一局域网）         → 0ms 建立
  2. UDP Hole Punching              → <500ms 建立
  3. UPnP/NAT-PMP 端口映射          → <1s 建立
  4. TURN Relay（自建中转）         → <50ms 延迟增加
  5. DERP（HTTPS 伪装中继）         → 端口 443 兜底

STUN 服务器：自建（端口 3478/UDP）
TURN 服务器：自建（端口 3478/TCP+UDP）
DERP 中继：复用 Relay 节点（端口 443，伪装 HTTPS）
```

### 4.2 WireGuard 集成

```
Mesh 网络建立流程：
1. Agent A 创建 Mesh 网络 → 生成 CIDR + WireGuard 密钥对
2. Agent B 通过邀请码加入 → 交换公钥
3. Topology Planner 计算最优路径：P2P 直连 / Relay 中继
4. 若 P2P 可行 → 配置 WireGuard peer → 直连
5. 若 P2P 不可行 → 配置 WireGuard 通过 Relay 路由
6. 网络内 DNS 解析：node-name.service.network → Mesh IP
```

---

## 五、全球边缘部署

### 5.1 多区域架构（Phase 1 的演进）

```
Phase 2.0 目标：至少 3 个区域

┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│ ap-southeast-1  │   │ us-east-1       │   │ eu-central-1    │
│ (新加坡)        │   │ (弗吉尼亚)      │   │ (法兰克福)      │
│                 │   │                 │   │                 │
│ Relay × 2       │   │ Relay × 2       │   │ Relay × 2       │
│ Control × 2     │   │ Control × 1     │   │ Control × 1     │
│ PG Read Replica │   │ PG Primary      │   │ PG Read Replica │
└─────────────────┘   └─────────────────┘   └─────────────────┘
         │                      │                      │
         └──────────────────────┼──────────────────────┘
                                │
                         GeoDNS / Anycast
                    agent.connect.omnitun.io
```

### 5.2 Agent 就近接入算法

```
Agent 启动 → DNS 解析 control.omnitun.io → 获取最近 Relay IP
  → 或通过 STUN 探测延迟 → 选择延迟最低的 Relay
  → 连接 Relay → 上报延迟数据 → Topology Planner 动态调整
```

---

## 六、NATS 消息总线

### 6.1 接入计划

```
当前状态：Subscriber 接口已定义（internal/relay/control_client.go）
          NATS 依赖因网络未下载（nats.go）

接入步骤：
1. 联网下载 go get github.com/nats-io/nats.go
2. 实现 NATSSubscriber（封装 nats.Conn）
3. 在 server main 中创建 NATS 连接并传入 Relay
4. Relay 订阅 tunnel.config.{relay_id} 主题
5. Orchestrator 发布配置更新到 NATS

Topics 设计：
  tunnel.config.{relay_id}     → Relay 接收配置更新
  tunnel.event.{event_type}    → 隧道事件广播
  agent.command.{agent_id}     → Agent 指令下发
```
