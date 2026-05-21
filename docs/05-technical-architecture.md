# OmniTun — 技术架构设计

> **修订说明**
> - 2026-05-20: 新增 ADR-000（API Gateway 选型决策）；修正 ADR-003 关于 gRPC/HTTP/3 0-RTT 描述；新增"二、非功能指标"章节

## 一、架构总览

### 1.1 全局架构图

```
                            ┌──────────────────────────────┐
                  ┌─────────┤     Global DNS Anycast        │
                  │         └──────────────────────────────┘
                  │
    ┌─────────────┴──────────────────────────────────────────┐
    │                      接入层 (Edge Layer)                 │
    │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
    │  │Edge SGP  │  │Edge NRT  │  │Edge FRA  │  ...        │
    │  │(新加坡)  │  │(东京)    │  │(法兰克福)│             │
    │  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
    └───────┼──────────────┼──────────────┼───────────────────┘
            │              │              │
    ┌───────┴──────────────┴──────────────┴───────────────────┐
    │                    数据面 (Data Plane)                    │
    │                                                          │
    │  ┌───────────────────────────────────────────────────┐  │
    │  │              Relay Cluster (中继集群)               │  │
    │  │  ┌─────────┐ ┌──────────┐ ┌──────────────────┐   │  │
    │  │  │HTTP     │ │TCP Proxy │ │UDP Datagram Pool │   │  │
    │  │  │Reverse  │ │Stream    │ │(环形缓冲区)      │   │  │
    │  │  │Proxy    │ │Mux       │ │                  │   │  │
    │  │  └────┬────┘ └────┬─────┘ └───────┬──────────┘   │  │
    │  │       └───────────┴───────────────┘              │  │
    │  │                   │                                │  │
    │  │         ┌─────────▼──────────┐                    │  │
    │  │         │  Vector Stream      │                    │  │
    │  │         │  (统一流量向量)      │                    │  │
    │  │         │  - 多路复用          │                    │  │
    │  │         │  - 压缩 (zstd)       │                    │  │
    │  │         │  - TLS Encryption    │                    │  │
    │  │         │  - 零拷贝转发        │                    │  │
    │  │         └─────────┬──────────┘                    │  │
    │  └───────────────────┼───────────────────────────────┘  │
    └──────────────────────┼──────────────────────────────────┘
                           │
    ┌──────────────────────┴──────────────────────────────────┐
    │                    控制面 (Control Plane)                │
    │                                                          │
    │  ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐ │
    │  │Auth      │ │Tunnel     │ │Topology  │ │Billing   │ │
    │  │Service   │ │Orch.      │ │Planner   │ │Service   │ │
    │  └──────────┘ └───────────┘ └──────────┘ └──────────┘ │
    │  ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐ │
    │  │RBAC      │ │Certificate│ │Audit     │ │Monitoring│ │
    │  │Service   │ │Manager    │ │Service   │ │Service   │ │
    │  └──────────┘ └───────────┘ └──────────┘ └──────────┘ │
    └──────────────────────────────────────────────────────────┘
                                     │
    ┌────────────────────────────────┴────────────────────────┐
    │                   数据层 (Data Layer)                     │
    │  ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐ │
    │  │PostgreSQL│ │Redis      │ │ClickHouse│ │S3/MinIO  │ │
    │  │(OLTP)    │ │(Cache/MQ) │ │(分析)    │ │(对象存储) │ │
    │  └──────────┘ └───────────┘ └──────────┘ └──────────┘ │
    └──────────────────────────────────────────────────────────┘
```

### 1.2 设计原则

| 原则 | 说明 |
|------|------|
| **控制面与数据面分离** | 控制面管理状态、路由、策略；数据面只做流量转发，不感知业务逻辑 |
| **无状态服务** | 所有控制面服务无状态，水平扩展，状态存储在共享数据层 |
| **最终一致性** | 控制面变更通过事件总线异步同步到数据面，允许秒级不一致窗口 |
| **故障隔离** | 单个 Relay 节点故障只影响该节点的隧道，自动迁移到其他节点 |
| **零信任网络** | 所有服务间通信均需 TLS + 认证，默认拒绝 |
| **可观测性优先** | 每个组件暴露 Prometheus metrics、分布式追踪、结构化日志 |

---

## 二、非功能指标

### 2.1 性能指标

| 指标 | 目标 | 测量方法 |
|------|------|----------|
| 隧道建立延迟 | P95 < 1s | Agent 启动到隧道 Active |
| 中继转发延迟（同区域） | P50 < 5ms, P99 < 20ms | 中继 ingress → egress |
| 中继转发延迟（跨区域） | P50 < 50ms, P99 < 150ms | 中继 ingress → egress |
| 控制面 API 延迟 | P95 < 200ms | API Gateway → 响应 |
| P2P 打洞成功率 | > 90% | NAT 类型 > 对称型 NAT |
| Agent 内存占用 | < 50MB（空闲），< 200MB（高负载，含 P2P/加密） | 持续运行 24h 后测量 |
| Relay 吞吐量 | 单节点 > 1Gbps（实测） | 多核并行转发 |

### 2.2 可用性指标

| 指标 | 目标 | 说明 |
|------|------|------|
| 控制面可用性 | 99.9% | 跨 AZ 多副本 |
| 数据面可用性 | 99.95% | 跨区域多 Relay 自动故障转移 |
| 数据持久性 | 99.999999999%（11个9） | 对象存储多副本 |
| 隧道持续在线率 | 99.95% | Agent 断线自动重连 |

### 2.3 容量指标

| 指标 | 目标 |
|------|------|
| 单 Relay 最大隧道数 | 10,000 并发隧道 |
| 单区域最大隧道数 | 50,000 并发隧道 |
| 全局最大隧道数 | 100,000+（水平扩展） |
| 单隧道最大连接数 | 1,000（可配置） |

### 2.4 安全指标

| 指标 | 目标 |
|------|------|
| TLS 版本 | 1.3 强制（支持 1.2 回退） |
| mTLS 覆盖率 | Agent ↔ Relay 100% |
| 审计日志保留 | 12 个月（PG）|

---

## 三、控制面设计

### 2.1 服务分解

| 服务 | 职责 | 技术栈 | 端口 |
|------|------|--------|------|
| **API Gateway** | 统一入口、认证、限流、路由 | Envoy / 自研 Go | 443 |
| **Auth Service** | 注册、登录、OAuth、SSO、JWT | Go | gRPC :9001 |
| **Tunnel Orchestrator** | 隧道 CRUD、生命周期、Relay 分配 | Go | gRPC :9002 |
| **Topology Planner** | 拓扑计算、P2P 打洞调度、Relay 选择 | Go | gRPC :9003 |
| **RBAC Service** | 权限验证、策略评估 | Go + OPA | gRPC :9004 |
| **Certificate Manager** | ACME 自动化、证书轮换 | Go | gRPC :9005 |
| **Audit Service** | 审计日志收集、存储、查询 | Go | gRPC :9006 |
| **Billing Service** | 用量计量、计费、发票 | Go | gRPC :9007 |
| **Notification Service** | Webhook 推送、邮件、告警 | Go | gRPC :9008 |
| **WebSocket Gateway** | Agent 长连接管理、控制指令下发 | Go | WebSocket :9443 |

### 2.2 API Gateway 设计

```
Client Request
    │
    ▼
┌─────────────────────┐
│  TLS Termination     │  (Let's Encrypt / Wildcard cert)
├─────────────────────┤
│  Rate Limiter        │  (Token Bucket, per API Key)
├─────────────────────┤
│  Authentication      │  (JWT / API Key validation)
├─────────────────────┤
│  Router              │  (Path-based → gRPC service)
├─────────────────────┤
│  Request/Response Log│  (Audit event emission)
├─────────────────────┤
│  gRPC Client Call    │
└─────────────────────┘
```

### 2.3 Agent 连接管理

```
Agent ──TLS──▶ WebSocket Gateway ◀──gRPC──▶ Tunnel Orchestrator
  │                                              │
  │  1. Auth (JWT/API Key)                       │
  │  2. Register Tunnel Request                   │
  │  3. Allocate Relay Node ─────────────────────▶│
  │  4. Return Relay Address                       │
  │  ┌──────────────────────┐                     │
  │  │ Agent connects to    │                     │
  │  │ Relay (WebSocket/QUIC)│                    │
  │  └──────────────────────┘                     │
  │  5. Tunnel Active Event ─────────────────────▶│
  │                                               │
  │  ◀── Health Check Ping (30s interval) ────────│
  │  ◀── Config Update (instant) ─────────────────│
  │  ◀── Shutdown Signal ─────────────────────────│
```

**Agent 连接协议**：
- 控制通道：WebSocket + TLS 1.3（用于指令下发和状态上报）
- 数据通道：优先 QUIC（支持多路复用和 0-RTT），到中继节点
- 心跳：30s Ping/Pong，3 次无响应判定离线
- 重连：指数退避（1s → 2s → 4s → ... → 60s cap），Jitter ±25%

---

## 三、数据面设计

### 3.1 Relay 节点架构

```
                    ┌─────────────────────────────┐
     Public         │       Envoy / Self-built     │
     Ingres ───────▶│       Reverse Proxy          │
    (HTTPS/TCP/UDP) │  ┌─────────────────────────┐ │
                    │  │ Route by Host Header     │ │
                    │  │ or SNI                   │ │
                    │  └────────┬────────────────┘ │
                    │           │                   │
                    │  ┌────────▼────────────────┐ │
                    │  │ Tunnel Dispatcher        │ │
                    │  │ (lookup tunnel_id by     │ │
                    │  │  domain/port in cache)   │ │
                    │  └────────┬────────────────┘ │
                    │           │                   │
                    │  ┌────────▼────────────────┐ │
                    │  │ Vector Stream Multiplexer│ │
                    │  │  - 多路复用 N 条隧道     │ │
                    │  │  - 帧头含 tunnel_id      │ │
                    │  │  - zstd 压缩             │ │
                    │  │  - 零拷贝 splice()       │ │
                    │  └────────┬────────────────┘ │
                    │           │                   │
                    └───────────┼───────────────────┘
                                │  QUIC / WebSocket
                                ▼
                           OmniTun Agent
```

### 3.2 统一流量向量 (Vector Stream)

核心思想：将所有协议（HTTP/TCP/UDP/ICMP）统一建模为「流（Stream）」，每个流携带 opaque 二进制帧。

```
Frame Format:
┌──────────────────────────────────────────────┐
│ Magic  (2B)  │  0x4F54  ("OT")                │
│ Version (1B) │  0x01                          │
│ Type    (1B) │  0x00=DATA, 0x01=CONTROL,      │
│              │  0x02=PING, 0x03=ERROR         │
│ Flags   (1B) │  0x01=COMPRESSED, 0x02=EOF     │
│ Stream ID (8B)│  Unique per tunnel connection  │
│ Length  (4B) │  Payload length                 │
│ Payload (N)  │  Compressed opaque bytes        │
└──────────────────────────────────────────────┘
```

**关键设计决策**：
- QUIC 优先：利用其内置的多路复用（Stream ID 映射到 QUIC Stream），无需自建 mux
- 到 Agent 的后备：当 QUIC 不可用时（如某些企业防火墙），回退到 WebSocket + TLS
- zstd 压缩：比 gzip 快 3-5 倍，HTTP 文本压缩比 >50%

### 3.3 P2P 连接建立流程

```
Agent A                              Agent B
   │                                    │
   │  1. 请求 P2P 连接 (via Control)     │
   ├───────────────────────────────────▶│ (信令)
   │                                    │
   │  2. STUN 探测                       │
   ├──────▶ STUN Server ◀───────────────┤
   │  (获取各自的公网IP:Port)            │
   │                                    │
   │  3. 交换 公网端点信息                │
   │◀───────── via Control ─────────────│
   │                                    │
   │  4. UDP Hole Punching               │
   │  A ────UDP───▶ B                   │
   │  (同时发包，双方各试 5 次)           │
   │                                    │
   │  5. WireGuard 密钥交换               │
   │  (Noise IK handshake)               │
   │◀══════ Direct P2P ===============▶│
   │                                    │
   │  ↓ 如果 5 次打洞均失败 ↓             │
   │                                    │
   │  6. TURN Relay 回退                 │
   │  A ──▶ TURN Server ──▶ B          │
   │                                    │
   │  ↓ 如果 TURN 也被阻断 ↓              │
   │                                    │
   │  7. DERP (HTTPS Relay) 回退         │
   │  A ──▶ Relay :443 ──▶ B           │
   │  (伪装成 HTTPS 流量)                │
```

---

## 四、多租户设计

### 4.1 租户隔离模型

| 层级 | 隔离方式 | 示例 |
|------|----------|------|
| **Organization** | PostgreSQL Row-Level Security (RLS) + Tenant ID Column | `acme-corp` |
| **Workspace** | 逻辑隔离（同表，workspace_id 过滤） | `production` / `staging` |
| **Tunnel** | 关联到 Workspace，Relay 侧隧道独立 | `api-gateway` |

**数据库层**：
- 共享数据库 + RLS 策略，而非 `per-tenant database/schema`（避免运维噩梦）
- 关键表（users, tunnels, api_keys）按 `tenant_id` 分区
- Enterprise 可选专属数据库实例（通过代理路由）

### 4.2 流量隔离

```
Relay 节点上的隧道隔离：

  Tenant A
    tunnel-1 (domain: a1.example.com)  ─── Agent A-1
    tunnel-2 (domain: a2.example.com)  ─── Agent A-2

  Tenant B
    tunnel-3 (domain: b1.example.com)  ─── Agent B-1

  → 同一 Relay 节点上，通过 tunnel_id 严格隔离
  → 配置变更通过控制面推送，Relay 无状态切换
```

---

## 五、关键数据流

### 5.1 HTTP 请求处理流程

```
Internet ──────▶ Edge (TLS Termination)
                    │
                    │ 1. 解析 Host header → Tunnel ID
                    │    (from cache/local KV)
                    ▼
                 Relay Node
                    │
                    │ 2. 查找活跃 Agent 连接
                    │    (from local connection pool)
                    ▼
                    │ 3. 封装为 Vector Frame
                    │    (compress, encrypt)
                    ▼
                 QUIC Stream ────────▶ Agent
                    │
                    │ 4. Agent 解封
                    │    (decompress, decrypt)
                    ▼
                 127.0.0.1:8080 (本地服务)
                    │
                    │ 5. 响应原路返回
                    ▼
```

### 5.2 隧道创建流程（控制面）

```
User (CLI) ──POST──▶ API Gateway ──▶ Tunnel Orchestrator
                                        │
                                        │ 1. Validate request
                                        │    (quota, permissions)
                                        │ 2. Select optimal Relay
                                        │    (nearest to Agent by geo IP)
                                        │ 3. Create Tunnel record (PG)
                                        │ 4. Provision DNS
                                        │    (create CNAME → Relay)
                                        │ 5. Issue TLS Certificate
                                        │    (ACME order → S3)
                                        │ 6. Push config to Relay
                                        │    (via gRPC / Redis PubSub)
                                        │ 7. Return tunnel config
                                        │    to Agent via WebSocket
                                        ▼
                                    Agent starts tunnel
```

### 5.3 DNS 系统

```
用户自定义域名 example.com

1. 用户在 OmniTun 添加自定义域
2. OmniTun 返回验证要求：添加 TXT 记录 _omnitun-challenge.example.com
3. 用户 DNS 配置：（两种情况）
   a. CNAME: example.com → tunnel-xxxx.omnitun-edge.com
   b. A/AAAA: example.com → Edge IP（可选 Anycast）
4. 证书：ACME DNS-01 challenge → Let's Encrypt → wildcard cert
5. 证书分发：控制面 → S3 → Relay 节点定期拉取

系统域名 <slug>.omnitun.io
  直接 CNAME 到 Edge，泛域名证书预置
```

---

## 六、技术栈

### 6.1 后端

| 组件 | 技术 | 理由 |
|------|------|------|
| 编程语言 | **Go 1.23+** | 高并发原生支持、单二进制部署、内存效率 |
| HTTP 框架 | net/http + chi router | 标准库优先，微小依赖 |
| gRPC | google.golang.org/grpc | 服务间通信 |
| WebSocket | gorilla/websocket | 控制面 Agent 通信 |
| QUIC | quic-go | 数据面传输 |
| WireGuard | wireguard-go / tailscale 库 | P2P 加密通道 |
| ACME | lego (go-acme/lego) | 自动 TLS 证书 |
| ORM | sqlc + pgx | 类型安全 SQL，避免 ORM 性能陷阱 |
| 配置 | Viper | 支持文件/环境变量/远程配置 |

### 6.2 前端

| 组件 | 技术 | 理由 |
|------|------|------|
| 框架 | React 19 + TypeScript | 生态最广、招聘容易 |
| 构建 | Vite | 快速 HMR |
| UI 组件 | shadcn/ui + Tailwind CSS | 高质量无样式锁定 |
| 状态管理 | TanStack Query + Zustand | 服务器状态与客户端状态分离 |
| 实时通信 | WebSocket (reconnecting) | Dashboard 实时更新 |
| 图表 | recharts / d3 | 流量可视化 |

### 6.3 基础设施

| 组件 | 技术 |
|------|------|
| 数据库 | PostgreSQL 16 (主) + ClickHouse 24 (分析) |
| 缓存 | Valkey 8 (Redis 兼容) |
| 消息队列 | NATS JetStream |
| 对象存储 | MinIO (自建) / S3 兼容 |
| 容器编排 | Kubernetes (控制面) + Nomad (Relay 节点，轻量) |
| 服务网格 | Linkerd（零配置 mTLS，轻量） |
| 可观测性 | OpenTelemetry Collector → Grafana + Mimir + Loki + Tempo |
| CI/CD | GitHub Actions + ArgoCD + ko (Go 镜像构建) |
| IaC | Terraform + Helm |
| DNSSEC | CoreDNS + external-dns |

---

## 七、关键设计决策记录 (ADR)

### ADR-000: API Gateway 选型决策

**决策**：Phase 1 使用 Envoy 作为 API Gateway，不自研。

**理由**：
- Envoy 是生产验证的代理（Lyft、Netflix、AWS App Mesh 都在用）
- 内置限流、TLS 终止、重试、熔断等隧道场景所需功能
- 与 Kubernetes 原生集成（Envoy xDS API）
- 自研 Go HTTP Server 需要额外 4-6 周开发时间

**过渡方案**：Phase 1 用 Envoy，当前端需求复杂后考虑自研替换。

**Phase 2 考虑**：如果流量特征需要深度定制（如自定义 TCP 协议），评估自研 Go 网关。

### ADR-001: 为什么不用 HTTP/3 只用 QUIC？

**决策**：用 QUIC transport + 自定义帧协议，而非 HTTP/3。

**理由**：
- HTTP/3 语义太重（强制 HTTP 头解析），Tunnel 场景需要流式二进制转发
- QUIC 提供多路复用、0-RTT、连接迁移，正是 Tunnel 数据面所需
- 自定义帧协议可以零开销转发 TCP/UDP 的原始字节
- 按需回退到 WebSocket（企业防火墙兼容）

### ADR-002: 控制面与数据面严格分离

**决策**：控制面只做业务逻辑和状态管理，数据面只做流量转发。

**理由**：
- 数据面需要高性能（零拷贝、内核旁路），控制面需要灵活迭代
- 分离后数据面可以用更底层的技术（eBPF/XDP），不影响控制面
- 故障隔离：控制面挂了，已有隧道不受影响
- 未来可开放 Relay 协议，允许第三方 Relay 节点加入网络

### ADR-003: 不用 gRPC 做 Agent 数据通道

**决策**：Agent ↔ Relay 之间的数据通道用 QUIC + 自定义帧协议，不用 gRPC stream。

**理由**：
- gRPC 官方不支持 HTTP/3（即不支持 QUIC 的 0-RTT 和连接迁移特性）
- 注意：HTTP/3 协议本身支持 0-RTT，但 gRPC 库尚未实现此能力
- gRPC 不适合 UDP 透传（所有数据须包装成 protobuf）
- QUIC 原生的流控制和拥塞控制更适合隧道场景

### ADR-004: 使用 NATS 而非 Kafka

**决策**：消息队列使用 NATS JetStream，而非 Apache Kafka。

**理由**：
- NATS 部署运维成本极低（单二进制、< 50MB 内存）
- 隧道事件是高频低延迟场景（毫秒级），NATS 在此类场景优于 Kafka
- 不需要 Kafka 的长持久化能力（隧道事件 > 24h 没有意义）
- NATS 的 subject-based routing 天然适合推送到 Relays

### ADR-005: 不直接用 Tailscale 依赖

**决策**：自建 NAT 穿透引擎，不直接依赖 Tailscale/tailscale Go 模块。

**理由**：
- Tailscale 的 coordination server 不开源，自建受限
- 我们要支持大规模 IoT 设备（单个租户数万节点），Tailscale 的 node 管理模型不适合
- 自建引擎可以深度定制（隧道场景的 P2P 不同于通用 VPN）
- 但可以研究和借鉴 WireGuard-go 和 DERP 的实现思路
