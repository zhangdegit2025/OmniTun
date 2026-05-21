# OmniTun — 里程碑路线图

<!-- 修订说明：
- 2026-05-20：修正 Phase 1 时间线，Sprint 7-8 砍掉 dogfooding（DOG-01 移至 Phase 2），压缩为 6 Sprint / 12 周
-->

## 一、总览

```
2026 Q3 ────── 2026 Q4 ────── 2027 Q1 ────── 2027 Q2 ────── 2027 Q3 ────── 2027 Q4
  │              │              │              │              │              │
  ▼              ▼              ▼              ▼              ▼              ▼
Phase 1        Phase 2        Phase 3        Phase 4        Phase 5        Phase 6
Foundation     Core SaaS      Mesh & P2P     Enterprise     Scale &        Ecosystem
                                        & Self-host    Global
```

---

## 二、Phase 1：Foundation（2026 Q3-Q4，12 周）

**目标**：控制面 + 单协议隧道 + 基础 Dashboard。可演示、可内测。

### Sprint 1-2：核心基础设施

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| INFRA-01 | 项目脚手架 | Go monorepo + React SPA 项目结构 | `go build ./...` && `npm run build` 通过 |
| INFRA-02 | PostgreSQL Schema | 全部 OLTP 表创建 | Migration 脚本可运行 |
| INFRA-03 | ClickHouse Schema | 分析表创建 | Migration 脚本可运行 |
| INFRA-04 | Docker Compose 开发环境 | 一键启动全部依赖 | `docker compose up` 后所有服务健康 |
| INFRA-05 | CI/CD Pipeline | GitHub Actions 基础流水线 | Lint + Test + Build 自动运行 |
| INFRA-06 | 配置管理 | Viper 配置加载 | 支持文件/环境变量/命令行 |

### Sprint 3-4：Auth 服务

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| AUTH-01 | 邮箱注册 + 登录 | 注册/登录 API | 注册 → 验证邮箱 → 登录 → 获得 JWT |
| AUTH-02 | JWT 中间件 | Gin/Chi middleware | 未认证请求返回 401 |
| AUTH-03 | API Key 管理 | 创建/撤销 API Key | API Key 可通过 header 认证 |
| AUTH-04 | OAuth (GitHub) | GitHub OAuth 登录 | 点击 GitHub 登录 → 回调 → 创建用户 |
| AUTH-05 | MFA (TOTP) | MFA 注册与验证 | 注册 TOTP → 登录需 6 位验证码 |

### Sprint 5-6：Tunnel 核心

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| TUN-01 | Tunnel Orchestrator | Tunnel CRUD gRPC 服务 | 创建/删除/启动/停止隧道 |
| TUN-02 | Relay 节点基础框架 | QUIC + WebSocket 双模 Relay | Relay 可接受 Agent 连接 |
| TUN-03 | HTTP 反向代理 | Relay → Agent HTTP 转发 | `omnitun http 8080` → 公网可访问 |
| TUN-04 | WebSocket 代理 | WebSocket over tunnel | WebSocket 连接畅通无阻 |
| TUN-05 | 随机子域名 | `<slug>.omnitun.dev` | 创建隧道自动分配域名 |

### Sprint 7-8：Dashboard + CLI MVP

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| CLI-01 | CLI 框架 | Cobra CLI | `omnitun login` / `omnitun http 8080` 可工作 |
| CLI-02 | HTTP 快速隧道 | `omnitun http` 命令 | 一键获得公网 URL |
| UI-01 | Dashboard 框架 | React SPA + shadcn/ui | 登录 → Dashboard 首页 |
| UI-02 | 隧道管理页面 | Tunnel 列表/创建/详情 | CRUD 隧道从 Dashboard |
| UI-03 | 实时流量监控 | WebSocket 实时更新 | 隧道详情页可看实时 QPS 和连接 |

**Phase 1 交付物**：内部 Alpha 版本，7 人内测团队可用。（DOG-01 Dogfooding 移至 Phase 2）

---

## 三、Phase 2：Core SaaS（2026 Q4，8 周）

**目标**：多租户完整 SaaS + 付费体系上线。Public Beta。

### Sprint 9-10：多租户

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| MT-01 | Organization 管理 | 组织创建/设置 | 注册时自动创建组织 |
| MT-02 | Workspace 管理 | 工作区 CRUD | 创建/切换工作区 |
| MT-03 | RBAC 权限 | 角色 + 权限评估 | Owner/Admin/Editor/Viewer 权限隔离 |
| MT-04 | 成员邀请 | 邀请链接 + 邮箱邀请 | 邀请 → 接受 → 加入组织 |
| MT-05 | 租户数据隔离 | RLS 策略 | 用户只能看到自己组织的数据 |

### Sprint 11-12：计费系统

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| BIL-01 | Stripe 集成 | 支付流程 | Pro/Team/Business 的 Stripe Checkout |
| BIL-02 | 用量计量 | 带宽/隧道数/连接数统计 | 实时用量面板 |
| BIL-03 | 计划限制 | 按计划限制功能 | Free 用户操作超限时提示升级 |
| BIL-04 | 发票系统 | 自动生成发票 | Stripe Invoice → Dashboard 展示 |

### Sprint 13-14：增强功能

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| TUN-10 | TCP 隧道 | TCP 端口转发 | `omnitun tcp 3306` → 公网可连接 |
| TUN-11 | UDP 隧道 | UDP 数据报转发 | `omnitun udp 1194` → UDP 畅通 |
| TUN-12 | gRPC 隧道 | gRPC over HTTP/2 | gRPC 客户端可调用隧道后端 |
| TUN-13 | 自定义域名 | 绑定自有域名 | 添加域名 → DNS CNAME 验证 → 自动 TLS |
| AUTH-10 | OIDC SSO | OIDC 通用 SSO | 对接 Okta / Google Workspace |
| AUTH-11 | SAML SSO | SAML 2.0 SSO | 对接 Azure AD |

### Sprint 15-16：Public Beta 准备

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| DOC-01 | 文档站 | docs.omnitun.io | 快速开始/CLI 参考/API 文档 |
| DOC-02 | Landing Page | omnitun.io | 产品首页 + 定价页 |
| OPS-01 | 生产环境部署 | K8s 生产集群 | 99.9% SLA 目标 |
| OPS-02 | 监控告警 | Prometheus + Grafana + PagerDuty | 关键指标告警就绪 |
| SEC-01 | 安全审计 | 第三方安全审查 | 通过审查 |
| BLOG-01 | 发布博客 | "Introducing OmniTun" | HN / V2EX / Reddit 推广 |

**Phase 2 交付物**：Public Beta。对外免费注册。

---

## 四、Phase 3：Mesh & P2P（2027 Q1，8 周）

**目标**：私有 Mesh 组网 + P2P 直连。核心竞争力建立。

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| MESH-01 | STUN 服务器 | STUN RFC 5389 实现 | NAT 类型正确探测 |
| MESH-02 | UDP Hole Punching | 简单 NAT 打洞 | 非对称 NAT 打洞成功 |
| MESH-03 | TURN Relay | TURN 服务器 | 对称 NAT 兜底转发 |
| MESH-04 | DERP 中继 | HTTPS 伪装中继 | 端口 443 伪装转发 |
| MESH-05 | Mesh 网络管理 | 网络创建/加入/管理 | 多节点组建加密网络 |
| MESH-06 | WireGuard 集成 | Noise IK + ChaCha20 | 点对点 WireGuard 隧道 |
| MESH-07 | 网络内 DNS | 节点名.service.network | mesh 内 DNS 解析 |
| MESH-08 | 自适应拓扑 | 自动选择 P2P/Relay/TURN | 网络变化时无感切换 |
| MESH-09 | Mesh Dashboard | 网络拓扑可视化 | 节点图 + 延迟 + 路由 |

**Phase 3 交付物**：P2P + Mesh 功能对外可用。Pro 用户可用 P2P 直连。

---

## 五、Phase 4：Enterprise & Self-hosted（2027 Q2，8 周）

**目标**：企业级功能 + 私有部署方案。Enterprise 可售。

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| ENT-01 | 私有 Relay 节点 | 企业专属中继 | 数据不经公共 Relay |
| ENT-02 | Helm Chart | 完整 K8s 部署 Chart | 一键部署全栈 |
| ENT-03 | Air-gapped 部署 | 离线部署包 | 无外网环境下完整运行 |
| ENT-04 | SCIM 集成 | 自动用户/组同步 | Okta/Azure AD SCIM |
| ENT-05 | 会话录制 | HTTP 请求录制回放 | 审计级会话保存 |
| ENT-06 | SIEM 推送 | Splunk/Elastic 日志推送 | 审计事件实时推送 SIEM |
| ENT-07 | 合规报告 | SOC 2 / ISO 报告 | 自动生成合规报告 |
| ENT-08 | SLA 管理 | 99.9% / 99.99% SLA | SLA Dashboard + 违约告警 |
| ENT-09 | BYOK | 客户自带加密密钥 | KMS 对接 |
| ENT-10 | 专属支持 | 7×24 + 客户成功经理 | 4 小时响应 SLA |

**Phase 4 交付物**：Enterprise 可正式销售。目标 5-10 个 Enterprise 签约客户。

---

## 六、Phase 5：Scale & Global（2027 Q3，8 周）

**目标**：全球多区域部署 + 性能优化 + SDK 生态。

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| GLO-01 | 多区域控制面 | 3 区域 Control Plane 部署 | 任一区域故障不影响全球 |
| GLO-02 | GeoDNS 路由 | 用户就近接入 Relay | 同区域延迟 < 5ms |
| GLO-03 | 中国区部署 | 国内独立集群 | 国内合规 + 低延迟 |
| GLO-04 | Go SDK | `omnitun-go` 库 | 隧道管理/启动/监控 |
| GLO-05 | Python SDK | `omnitun-python` 库 | 同上 |
| GLO-06 | JS/TS SDK | `omnitun-js` 库 | 同上 |
| GLO-07 | 性能优化 | Relay 10Gbps+ 吞吐 | 零拷贝、内核旁路 |
| GLO-08 | 全球加速 | 跨境链路优化 | 跨洋 P2P 延迟 < 100ms |
| GLO-09 | 负载测试 | 10 万并发隧道压测 | 系统平稳运行 |

**Phase 5 交付物**：全球生产集群就绪，SDK 生态初步建立。

---

## 七、Phase 6：Ecosystem（2027 Q4，8 周）

**目标**：生态建设 + IDE 集成 + 市场领导地位巩固。

| ID | 任务 | 产出 | 验收标准 |
|----|------|------|----------|
| ECO-01 | VSCode 插件 | IDE 内一键暴露服务 | VSCode Marketplace 上架 |
| ECO-02 | JetBrains 插件 | 同上 | JetBrains Marketplace 上架 |
| ECO-03 | Next.js 插件 | `@omnitun/next` | Next.js dev 自动隧道 |
| ECO-04 | K8s Ingress Controller | OmniTun Ingress | K8s Service 自动暴露 |
| ECO-05 | Docker Compose 集成 | OmniTun sidecar | `omnitun` 作为 compose service |
| ECO-06 | CI/CD 集成 | GitHub Actions / GitLab CI | Actions 市场 / 模板 |
| ECO-07 | Terraform Provider | `terraform-provider-omnitun` | IaC 管理隧道 |
| ECO-08 | Open Core 开源 | GitHub 发布核心引擎 | MIT 协议开源 |
| ECO-09 | Bug Bounty 启动 | HackerOne 项目 | 漏洞报告流程就绪 |
| ECO-10 | Community 建设 | Discord/论坛 | 活跃社区用户 > 1000 |

**Phase 6 交付物**：全生态覆盖，开发者工具链完整集成。

---

## 八、关键里程碑总结

| 时间 | 里程碑 | 关键指标 |
|------|--------|----------|
| 2026 Q3 | Alpha 内测 | 7 人可用 |
| 2026 Q4 | Public Beta | 免费开放注册，首批 1000 用户 |
| 2027 Q1 | Mesh & P2P GA | P2P 打洞成功率 > 90%，500 付费用户 |
| 2027 Q2 | Enterprise GA | 10 个 Enterprise 签约 |
| 2027 Q3 | 全球多区域 | 3 区域 + 中国区，2000 付费用户 |
| 2027 Q4 | 生态系统 | 开源 + IDE 插件 + SDKs |

---

## 九、风险路线图

| 风险 | 影响阶段 | 触发条件 | 预案 |
|------|----------|----------|------|
| 核心团队流失 | Phase 1-2 | 关键技术成员离职 | 文档完善、知识分散、招聘储备 |
| 安全漏洞 | Phase 2+ | 严重 CVE 被发现 | 安全响应流程、24h 修复 SLA |
| 云成本超预期 | Phase 3+ | Relay 流量激增 | Relay 成本分摊到定价、P2P 降低 Relay 依赖 |
| 竞品大幅降价 | Phase 2+ | ngrok 降价对抗 | 加速差异化功能交付 |
| 国内合规困难 | Phase 5 | 等保审批受阻 | 与已有资质企业合作落地 |
