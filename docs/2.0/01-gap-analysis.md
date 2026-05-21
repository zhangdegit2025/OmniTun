# OmniTun 2.0 — 差距分析与满血定义

## 一、当前状态审计

### 已具备（基线能力）

| 模块 | 实际状态 | 代码位置 |
|------|----------|----------|
| 用户认证 | 注册 / 登录 / JWT 签发+验证 / MFA 代码已写好但未接入 | `internal/auth/` |
| 隧道元数据 | CRUD API + PG 持久化 + 前端完整 UI | `cmd/server/main.go` |
| Dashboard | 统计卡片 + 事件列表 + 隧道管理 + 设置页 + 中英 i18n | `web/src/pages/` |
| CLI | 6 子命令，session 管理 | `cmd/client/` |
| 数据库 | 14 张表已迁移，PG+CK+Valkey+NATS+MinIO 容器运行 | `migrations/` |
| WebSocket | 前端客户端 + 服务端 /ws 端点 + Gateway Hub 代码 | `internal/gateway/` + `web/src/lib/websocket.ts` |
| 协议层 | Vector Stream 帧编解码（94% 测试覆盖） | `internal/protocol/` |
| Relay 骨架 | QUIC+WS 监听 / Dispatcher / Proxy 代码完整 | `internal/relay/` |
| Agent 骨架 | WS 连接 / 心跳 / 重连 / 数据通道转发逻辑 | `internal/control/` |
| 测试 | 11 套件 157+ 测试 | `internal/*/` + `tests/` |
| 部署 | Docker Compose + K8s + CI/CD + 镜像代理 | `deploy/` + `.github/` |

### 未具备（核心缺口）

| 缺口 | 严重性 | 影响 |
|------|--------|------|
| **隧道数据路径未贯通** | 🔴 致命 | CLI 创建隧道成功，但 Agent 没有实际连接到 Relay 转发流量 |
| **TLS 自动签发未实现** | 🔴 致命 | 无 Let's Encrypt ACME 集成，自定义域名无法使用 |
| **域名验证未实现** | 🔴 致命 | DNS TXT/CNAME 验证流程不存在 |
| **SSO/OIDC 未接入** | 🟡 重度 | 企业客户无法单点登录 |
| **MFA 未接入** | 🟡 重度 | 代码已有但未集成到 API 路由 |
| **审计日志未写入** | 🟡 重度 | audit_logs 表存在但从未有数据写入 |
| **API Key 验证未接入** | 🟡 重度 | 创建了 key 但 middleware 未挂载 |
| **P2P 打洞未实现** | 🟡 重度 | STUN/TURN/打洞逻辑完全空白 |
| **Mesh 组网未实现** | 🟡 重度 | WireGuard 集成空白 |
| **NATS 消息未连接** | 🟡 重度 | `Subscriber` 接口已定义，无实现 |
| **计费系统未接入** | 🟢 轻度 | 无 Stripe、无用量计费 |
| **可观测性不完整** | 🟢 轻度 | 有 Prometheus metrics 但无告警、无分布式追踪 |
| **生产监控告警未就绪** | 🟢 轻度 | PagerDuty/Slack 告警规则未配置 |

---

## 二、满血版定义

### 满血基线 = 当前基线 ∩ 以下全部

| 领域 | 满血标准 |
|------|----------|
| **隧道能力** | `omnitun http 8080` → 公网 URL → 浏览器可访问 → 实时流量可见 → 请求可检查 |
| **域名 & TLS** | 绑定自定义域名 → DNS 自动验证 → Let's Encrypt 自动签发 → 自动续期 |
| **企业安全** | GitHub/Google OAuth 登录 + OIDC SSO + TOTP MFA + RBAC 角色隔离 + API Key 认证 |
| **审计合规** | 所有操作写入 audit_logs，可按时间/操作/用户检索，保留 12 个月 |
| **P2P & Mesh** | 两个 Agent 之间自动 NAT 穿透→WireGuard 加密直连，失败回退 Relay |
| **全球边缘** | 至少 3 个区域 Relay 节点，Agent 就近接入，跨区域故障自动切换 |
| **可观测性** | Metrics + 结构化日志 + 分布式追踪 + PagerDuty 告警 |
| **计费** | Stripe 订阅 + 用量计量 + 计划限制 + 发票 |
| **测试** | 单元测试 50%+ 覆盖率 + 集成测试（真实 PG）+ E2E 测试 |
| **部署** | Docker Compose 一键启动 + K8s Helm Chart + 生产级 systemd 服务 |

### 满血版不在范围的（明确不做）

| 项目 | 原因 |
|------|------|
| IDE 插件 | Phase 6 生态 |
| K8s Ingress Controller | Phase 6 生态 |
| Terraform Provider | Phase 6 生态 |
| SOC2/ISO27001 正式认证 | 需要 6+ 月观察期 |
| 裸金属 / Air-Gapped 部署 | Phase 4 Enterprise |
| IoT 硬件 Agent | Phase 5 Scale |

---

## 三、差距量化

### 隧道能力差距

```
当前:
  CLI → REST API → PG:INSERT tunnel → Dashboard 显示 "stopped"
  没有真实网络数据包流经任何 OmniTun 组件。

目标:
  CLI → REST API → PG:INSERT tunnel → Agent 连接 Relay
  → Internet 请求 → Relay 入口 → QUIC Stream → Agent → 本地服务 → 响应返回
```

### 代码完成度

| 模块 | 已写代码 | 已接入 | 差值 |
|------|----------|--------|------|
| Auth Service | 100% | 60%（缺 MFA/SSO 接入）| 40% |
| Tunnel CRUD | 100% | 100% | 0% |
| Relay | 100% | 30%（缺真实转发验证）| 70% |
| Agent | 100% | 20%（缺真实连接验证）| 80% |
| WS Gateway | 100% | 20%（未启动未验证）| 80% |
| Dashboard | 100% | 100% | 0% |
| CLI | 80% | 60% | 40% |

---

## 四、满血版里程碑

| 里程碑 | 内容 | 交付物 |
|--------|------|--------|
| **M1: 隧道贯通** | Agent↔Relay 数据路径 + ACME TLS + 自定义域名 | 公网可访问的隧道 |
| **M2: 企业安全** | SSO/OIDC + MFA + RBAC + API Key 认证 + 审计日志 | 完整认证体系 |
| **M3: P2P + Mesh** | STUN/TURN 打洞 + WireGuard + 自适应拓扑 | 私有组网 |
| **M4: 全球部署** | 3 区域 Relay + GeoDNS + 可观测性 | 全球低延迟 |
| **M5: 计费上线** | Stripe + 用量计量 + 发票 | 营收闭环 |
| **M6: 投产验证** | 集成测试 + E2E + 负载测试 + 安全审计 | 投产签收 |
