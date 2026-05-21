# OmniTun 3.0 — 分阶段执行计划

> 将 3.0 的五大支柱分解为可交付的执行阶段，按依赖关系和业务优先级排序。总工期 13 周。

---

## 一、总体架构

```
              ┌─────────────────────────────────────────┐
              │              admin.omnitun.io            │
              │           Super Admin Console            │
              │         管理后台 (独立前端应用)            │
              └──────────────────┬──────────────────────┘
                                 │
              ┌──────────────────┴──────────────────────┐
              │              app.omnitun.io              │
              │            User Dashboard                │
              │          用户端 (独立前端应用)             │
              └──────────────────┬──────────────────────┘
                                 │
              ┌──────────────────┴──────────────────────┐
              │              api.omnitun.io              │
              │         OmniTun REST API (v1)            │
              │           共用 API 网关 + 业务层           │
              └─────────────────────────────────────────┘
```

### 技术边界决策

| 关注点 | 决策 | 理由 |
|--------|------|------|
| 管理后台 vs 用户端 | 两个独立前端应用 | 安全隔离：管理后台代码不应出现在用户端 bundle 中 |
| 管理后台 API | 共用 API 网关，新增 `/super-admin` 命名空间 | 减少服务爆炸，利用现有认证/限流/日志中间件 |
| Super Admin 认证 | 独立于用户体系，MFA 强制 | 最高权限账号不能和普通用户共享认证链路 |
| 数据库 | 共享 PostgreSQL 集群，新增 schema 隔离 Admin 数据 | 运营分析需 join 用户数据，分库增加同步复杂度 |
| 前端框架 | React 18 + Vite + TypeScript (一致性) | 和 2.0 用户端保持一致 |

---

## 二、分阶段计划

---

### Phase 3.1 — 管理后台 MVP ⏱ 2 周

> **目标**：运营团队无需直接操作数据库即可完成日常管理。这是 3.0 的基石——没有管理后台，其他阶段无法验证。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 无 — 3.0 基石，所有后续 Phase 的基础 |
| 可并行 | — |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 1.1 | **管理后台项目脚手架** | Vite + React 18 + TypeScript + Tailwind 初始化；`admin/` 独立前端项目；路由框架 (React Router)；API 层 (Axios + interceptors) | P0 |
| 1.2 | **Super Admin 认证** | Super Admin 独立登录页；JWT 认证（独立于用户 JWT）；MFA 强制开启 (TOTP)；Session 超时 30 分钟 | P0 |
| 1.3 | **全局仪表板** | MRR (Monthly Recurring Revenue)；DAU (Daily Active Users)；活跃隧道总数；Relay 节点健康状态 (Green/Yellow/Red)；近期告警事件卡片 | P0 |
| 1.4 | **组织管理** | 组织列表（分页、搜索、按计划/状态过滤）；组织详情（基本信息、用户列表、活跃隧道、订阅状态）；操作：冻结/恢复组织、变更计划、模拟登录（Impersonate） | P0 |
| 1.5 | **用户全局搜索** | 跨租户用户搜索（邮箱/ID/姓名）；用户详情（关联组织、角色、最后登录、登录历史）；操作：重置密码、强制下线、封禁/解封 | P0 |
| 1.6 | **Relay 节点管理** | Relay 节点列表（名称、地域、IP、状态、负载、在线时长）；注册新 Relay 节点；下架/Drain Relay 节点（优雅排空连接）；Relay Health Dashboard（按地域热力图） | P0 |

**验收标准**：
- 运营人员可独立完成组织冻结、用户搜索、Relay 下架操作
- 全局仪表板数据准确（与 Stripe / 数据库直接查询对比验证）
- Super Admin 登录强制 MFA 且 30 分钟超时生效

---

### Phase 3.2 — 运营增强 ⏱ 2 周

> **目标**：安全的运营能力 + 灰度发布基建 + 全局可观测性。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 3.1（管理后台基础设施） |
| 可并行 | 前端任务可与 3.3 并行推进 |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 2.1 | **安全与滥用管理** | 举报队列（用户提交的 Abuse Report 列表 + 详情 + 处理）；IP 黑名单管理（添加/移除 IP/CIDR + 生效范围）；账号冻结（冻结/恢复 + 批量操作 + 冻结原因记录） | P0 |
| 2.2 | **Feature Flag 系统** | Flag 管理后台（创建/编辑/归档 Flag）；策略类型：Boolean / Percentage Rollout / User Whitelist / Org Whitelist；Flag 变更审计日志；SDK 侧：前端 React Context + 后端 API 端点 | P1 |
| 2.3 | **全局审计日志** | 跨租户审计日志搜索（操作者、操作类型、时间范围、目标资源）；审计日志详情（Before/After Diff 渲染）；导出（CSV/JSON + 日期范围过滤） | P1 |
| 2.4 | **系统公告管理** | 公告创建（标题、正文、严重等级 Info/Warning/Critical）；定时发布（指定开始/结束时间）；展示位置：Dashboard 顶部横幅 + Login 页面；已读/隐藏状态追踪 | P1 |
| 2.5 | **证书管理** | 系统证书列表（API Gateway TLS、Relay ↔ Control Plane mTLS 证书）；证书到期前 30/14/7/1 天自动告警；租户证书监控（自定义域名 TLS 证书状态面板） | P1 |

**验收标准**：
- 举报可在 3 分钟内从查看→判定→处理完成
- Feature Flag 创建后 30 秒内生效
- 系统公告按时自动上架和下架

---

### Phase 3.3 — 用户端增强 ⏱ 3 周

> **目标**：将用户端从"可用"提升到"好用"。这是 3.0 最大的用户感知提升阶段。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 3.1（API 基础设施） |
| 可并行 | 前端任务可与 3.2 并行推进 |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 3.1 | **Onboarding Wizard** | 5 步引导：① 创建组织 ② 下载 CLI ③ 启动首条隧道 ④ 访问公开 URL ⑤ 邀请团队成员；进度条 + Skip 选项；重新进入 Wizard 入口 |
| 3.2 | **通知中心** | 站内通知（Bell icon + 未读数量 badge）；通知分类：隧道事件 / 账单 / 团队 / 安全 / 产品更新；通知偏好设置（哪些类型推送到 Email）；通知持久化 + 已读/未读状态 |
| 3.3 | **实时监控面板** | Request Inspector（隧道 HTTP 请求实时流）；Request Detail（Headers, Body, Query Params, Response Status, Latency）；Request Replay（修改 + 重发请求）；每条隧道独立 Inspector Tab |
| 3.4 | **Webhook 配置** | Webhook 管理页面（创建/编辑/删除/启用禁用）；事件类型：`tunnel.created`, `tunnel.deleted`, `tunnel.online`, `tunnel.offline`, `domain.verified`, `domain.failed`；签名验证 (HMAC-SHA256)；投递日志（成功/失败 + Retry 状态 + Payload）；手动重试 |
| 3.5 | **全局搜索 (Cmd+K)** | Cmd+K / Ctrl+K 唤起搜索面板；搜索范围：隧道名、域名、团队成员、API Key 名、设置项；键盘导航；最近访问快速入口 |
| 3.6 | **深色模式** | Light / Dark / System 三选一；CSS 变量驱动主题切换；无闪烁切换（SSR 时读取 localStorage/Cookie）；所有组件适配两套主题 |
| 3.7 | **隧道模板** | 预设模板库（HTTP、TCP、File Server、WebSocket、SSH）；从模板创建隧道（预填配置参数）；批量操作（多选 + 批量启动/停止/删除）；克隆隧道 |
| 3.8 | **团队协作增强** | 邀请链接（生成链接 + 过期时间 + 最大使用次数）；团队活动日志（谁做了什么、什么时候）；成员会话管理（查看活跃会话 + 强制下线） |

**验收标准**：
- 新用户注册到首条隧道上线 < 5 分钟
- Cmd+K 搜索延迟 < 100ms
- 深色模式无视觉异常

---

### Phase 3.4 — 开发者平台 ⏱ 2 周

> **目标**：降低开发者接入摩擦，形成网络效应。开发者的平滑体验是最强增长引擎。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 3.1（API 基础设施） |
| 可并行 | SDK 开发可与 3.3 后半并行推进 |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 4.1 | **CLI 增强** | `omnitun status` — 显示 Agent 进程状态、活跃隧道、版本、最后连接时间；`omnitun logs` — 实时查看隧道日志、支持 `--follow` `--tail N` `--filter`；`omnitun inspect` — 隧道请求实时检查（同 3.3 Request Inspector CLI 版）；`omnitun network` — 管理 P2P 网络（list/join/leave）；Shell Completion — zsh + bash + fish + powershell；`omnitun update` — 自动检查更新并自升级 |
| 4.2 | **Go SDK** | `client.Tunnels.List(ctx)` / `client.Tunnels.Create(ctx, params)` / `client.Tunnels.Delete(ctx, id)`；`client.Connect(ctx, tunnelID)` — 返回 context-manager，自动管理隧道连接生命周期；GoDoc 文档 + pkg.go.dev 发布；示例项目（HTTP Server + Tunnel） |
| 4.3 | **Python SDK** | `async with omnitun.connect(tunnel_id) as tunnel: ...` — Async context manager；`client.tunnels.list()` / `client.tunnels.create(...)` / `client.tunnels.delete(id)`；PyPI 发布 `omnitun` 包；类型注解完整（mypy strict）；示例项目（FastAPI + Tunnel） |
| 4.4 | **JS/TS SDK** | npm 发布 `@omnitun/sdk`；`client.tunnels.list()` / `client.tunnels.create()` / `client.tunnels.delete()`；`connect(tunnelID)` — spawn Agent 进程并返回连接 URL；TypeScript 类型完整；示例项目（Next.js + Tunnel） |
| 4.5 | **API 文档内嵌** | Swagger UI 嵌入 app.omnitun.io；"Try It" 按钮 — 直接在文档中调用 API；代码示例生成（cURL, Go, Python, JS, CLI）；每个 endpoint 含使用场景说明 |
| 4.6 | **CLI 下载中心** | 页面：`app.omnitun.io/download`；多平台：macOS (Intel + Apple Silicon), Linux (x64 + arm64), Windows (x64)；一键安装脚本：`curl -fsSL https://omnitun.io/install.sh \| bash`；版本选择（Latest / 历史版本） |

**验收标准**：
- Go SDK: `go get` 后 3 行代码即可创建 + 连接隧道
- Python SDK: `pip install omnitun` 后 async context manager 开箱即用
- CLI Update 命令正确检测并升级到最新版本

---

### Phase 3.5 — 商业运营 ⏱ 2 周

> **目标**：让商业团队有数据驱动的决策能力，从"凭直觉"到"看数据"。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 3.1（数据基础）、3.2（Feature Flag） |
| 可并行 | 可与 3.6 并行（无技术冲突） |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 5.1 | **收入仪表板** | MRR/ARR 趋势折线图（日/周/月/季度）；MRR 分解瀑布图（New/Expansion/Contraction/Churn）；Net New MRR；付费转化率漏斗；LTV 分群对比；CAC 分渠道对比；Churn Rate 趋势；AI 收入预测（30/60/90 天） |
| 5.2 | **客户 360 视图** | 客户详情单页：基本信息、联系人、账单历史、用量趋势、健康评分趋势、活动时间线；健康评分 3 色模型 (Red/Yellow/Green) |
| 5.3 | **发票管理** | 全平台发票列表 + 筛选搜索；发票详情（项目明细、税费、付款记录）；手动操作（开票/作废/退款/调整）；Dunning 配置（邮件模板、触发时间点） |
| 5.4 | **定价配置** | 套餐配置页（调整各计划价格和配额）；折扣码管理（创建/启用/停用/查看使用统计）；试用期配置 |
| 5.5 | **客户成功工具** | 活跃度追踪 (DAU/WAU/MAU per customer)；功能使用率矩阵；流失预警列表（高风险客户优先）；NPS 仪表板和 Detractor 列表 |

**验收标准**：
- 收入仪表板数据与 Stripe 对账误差 < 1%
- 客户 360 页面加载时间 < 3s
- 流失预警模型对高风险客户的识别准确率 > 70%（基于历史数据回测）

---

### Phase 3.6 — 企业级功能 ⏱ 2 周

> **目标**：通过 SOC2/ISO 合规就绪 + 企业级安全能力，解锁 $10K+ ACV 大客户交易。

| 属性 | 值 |
|------|-----|
| 依赖 Phase | 3.1（RBAC 基础）、3.2（审计日志） |
| 可并行 | 可与 3.5 并行（无技术冲突） |

| # | 任务 | 内容 | 优先级 |
|---|------|------|--------|
| 6.1 | **SCIM 集成** | SCIM 2.0 端点实现 (Users + Groups)；Azure AD / Okta 兼容性测试；自动 provisioning / deprovisioning；SCIM Bearer Token 管理 |
| 6.2 | **自定义角色** | 自定义角色 CRUD（组织级）；权限集编辑器（每个 API Action 的 toggle）；角色模板库；角色分配给用户 |
| 6.3 | **IP 白名单** | 组织级 IP/CIDR 白名单配置；隧道级覆盖白名单；被拒绝连接的审计日志 |
| 6.4 | **数据保留策略** | 可配置数据保留期 (30d/90d/1y/3y)；按数据类型设置；自动清理 Cron Job；清理前通知机制 |
| 6.5 | **审计报告** | SOC2 报告模板（用户访问审计、变更管理审计、安全事件审计）；ISO27001 报告模板（A.9/A.12/A.16）；报告生成（PDF/CSV/JSON）；定期自动生成 + 邮件发送 |
| 6.6 | **SLA 追踪** | 可用性指标面板（API 可用性 %、Tunnel 控制面可用性 %、Relay 数据面可用性 %）；SLA 违约自动告警；Service Credit 计算 + 发放 |

**验收标准**：
- SCIM 通过 Azure AD 和 Okta 的官方 Test Suite
- 审计报告可在 5 分钟内生成（数据量 < 100 万条）
- SLA 追踪数据与状态页 incident 时间自动关联

---

## 三、时间线总览

```
Week:  1-2          3-4          5-7          8-9         10-11        12-13
      ┌─────┐     ┌─────┐     ┌─────┐     ┌─────┐     ┌─────┐     ┌─────┐
      │ 3.1 │────▶│ 3.2 │────▶│ 3.3 │────▶│ 3.4 │────▶│ 3.5 │────▶│ 3.6 │
      └─────┘     └─────┘     └─────┘     └─────┘     └─────┘     └─────┘
         │            │            │            │            │            │
         ▼            ▼            ▼            ▼            ▼            ▼
    管理后台MVP   运营增强     用户端增强   开发者平台   商业运营    企业级功能
                                              CLI/SDK                合规审计
```

**总计：13 周达到行业头部平台标准**

### 依赖关系

```
Phase 3.1 (管理后台 MVP)
    │
    ├──▶ Phase 3.2 (运营增强) ──── 依赖 3.1 的管理后台基础设施
    │
    ├──▶ Phase 3.3 (用户端增强) ──── 独立于 3.1/3.2，可与 3.2 部分并行
    │
    └──▶ Phase 3.4 (开发者平台) ──── 依赖 3.1 的 API 基础设施
              │
              └──▶ Phase 3.5 (商业运营) ──── 依赖 3.1 的数据 + 3.2 的 Feature Flag
                        │
                        └──▶ Phase 3.6 (企业级功能) ──── 依赖 3.2 的审计日志 + 3.1 的 RBAC
```

### 并行化机会

以下 pair 可在同一周期内并行（若团队规模允许）：

- **Phase 3.2 + Phase 3.3**：前端工程师可并行推进管理后台增强和用户端增强
- **Phase 3.4 + Phase 3.3 后半**：SDK 开发和用户端监控面板可并行
- **Phase 3.5 + Phase 3.6**：商业运营仪表板和 SCIM/自定义角色无技术冲突

### 阶段性里程碑交付物

| 阶段 | 里程碑 | 可演示成果 |
|------|--------|-----------|
| 3.1 | 管理后台上线 | 运营人员登录管理后台完成组织冻结、用户搜索、Relay 下架 |
| 3.2 | 运营平台就绪 | Feature Flag 首个功能灰度上线；系统公告 banner 展示 |
| 3.3 | 用户端升级 | Onboarding Wizard 上线；Cmd+K 搜索可用；深色模式可用 |
| 3.4 | 开发体验闭环 | SDK 三语言发布；CLI 增强命令可用；API Doc "Try It" |
| 3.5 | 商业数据可见 | 收入仪表板可访问；客户 360 视图可访问；发票管理可操作 |
| 3.6 | 企业就绪 | SCIM 演示通过；SOC2 审计报告可生成；SLA 面板可访问 |

---

## 四、风险与缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| Super Admin 认证与现有 OIDC 体系冲突 | 中 | 阻塞 Phase 3.1 | 独立用户体系 + 独立 JWT issuer + 独立登录页 |
| Feature Flag 引入性能开销 | 低 | 影响用户体验 | Flag 评估结果本地缓存 + 5 秒 TTL |
| CLI 多平台构建 CI 复杂 | 中 | 阻塞 Phase 3.4 | GoReleaser + GitHub Actions Matrix Build |
| 流失预警模型初期准确率低 | 高 | 误报影响 CSM 效率 | 先上线基于规则的预警 + 收集数据训练 ML 模型 |
| SCIM 各 IdP 兼容性差异 | 中 | 延期 Phase 3.6 | 优先测试 AzureAD + Okta，覆盖 80% 企业客户 |
| Dunning 误触发导致客户投诉 | 中 | 损害客户关系 | 所有自动操作前做 Dry Run 验证 + 人工确认开关 |

---

## 五、版本记录

| 版本 | 日期 | 作者 | 变更 |
|------|------|------|------|
| v1.0 | 2026-05-21 | OmniTun Engineering Team | 3.0 分阶段执行计划首次发布 |
| v1.1 | 2026-05-21 | OmniTun Engineering Team | 各 Phase 新增依赖关系与并行化说明 |
