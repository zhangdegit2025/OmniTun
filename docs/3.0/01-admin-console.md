# OmniTun 3.0 — 管理后台完整需求

> **优先级**：P0（最高） | **Owner**：平台运营团队 | **目标上线**：Phase 3.0-A

---

## 一、管理后台定位

### 1.1 独立入口

| 属性 | 定义 |
|------|------|
| **域名** | `admin.omnitun.io`（独立于用户端 `app.omnitun.io`） |
| **访问者** | OmniTun 内部运营人员，持有 Super Admin 角色 |
| **认证网关** | 独立 OAuth2 登录入口 → 仅允许 Super Admin 角色的账号通过 |
| **数据范围** | 跨所有租户的全局数据，无租户隔离 |

### 1.2 约束与原则

| 原则 | 说明 |
|------|------|
| **最小权限** | 管理后台本身也分角色：只读运维 / 完全管理 / 超级根账号 |
| **全量审计** | 管理后台内的每一次操作（查看、修改、删除、模拟登录）均写入专用 `admin_audit_logs` 表 |
| **非侵入** | 管理后台禁止直接修改租户的业务数据（隧道配置 / 网络拓扑），仅限元数据操作和平台级管控动作 |
| **IP 白名单** | 管理后台登录限制在 OmniTun 内部 VPN/办公网段 |
| **MFA 强制** | 所有 Super Admin 账号强制启用 MFA |

### 1.3 Super Admin 角色模型

```
Super Admin 角色层级：
  Root Admin     — 紧急恢复、删除组织、修改系统配置（最多 2 人持有）
  Full Admin     — 管理用户/组织/Relay/安全/公告，不可删除组织
  Read-Only Ops  — 查看所有仪表板和详情，不可执行操作
  Security Admin — 访问安全中心、审计日志、IP 黑名单（不可管理组织/用户）
  Infrastructure — 管理 Relay 节点、证书、系统配置
```

### 1.4 与用户端的关系

```
app.omnitun.io (用户端)          admin.omnitun.io (管理后台)
─────────────────────            ─────────────────────────
租户 A 用户 → 管理 A 的隧道        Super Admin → 查看所有组织
租户 B 用户 → 管理 B 的隧道        Super Admin → 管理所有用户
租户隔离，看不到其他租户数据       全局视角，跨租户数据可见
```

**模拟登录（Impersonate）**：Super Admin 可从管理后台以某组织 Owner 身份跳转到 `app.omnitun.io`，此为唯一跨域操作。

---

## 二、全局仪表板（Home）

### 2.1 用户故事

**作为** 平台运营人员
**我想要** 登录管理后台后立即看到平台整体健康状态
**以便** 我能在 30 秒内判断"今天平台是否正常"，并在出现异常时快速定位问题

### 2.2 线框图描述

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console                                [SA: 张三] [⚙]│
├─────────────────────────────────────────────────────────────┤
│ 导航:  🏠首页 | Organizations | Users | Relay Nodes |        │
│        Certs | Security | Feature Flags | Announcements |   │
│        System Config                                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  Organizations │ Active   │ Today's  │   MRR     │       │
│  │     1,247      │ Tunnels  │ Traffic  │ $48,290   │       │
│  │   ↑12 本周     │   3,821  │ 2.4 TB   │ ↑5% MoM   │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ P2P Success │ Active   │ API Error │ Relays Up │       │
│  │   94.7%     │ Relays   │   0.03%   │  12/12   │       │
│  │   ↓0.3%     │    12     │  ✅正常  │  ✅全绿  │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │            Traffic Trend (7 Days)           [24h|7d|30d] │
│  │  📈                                                  │    │
│  │    ↑ 峰值: 2026-05-20 14:00  320 GB/h               │    │
│  │    → 平均值: 102 GB/h                               │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────────────┐  ┌──────────────────────────────┐ │
│  │  Recent Signups      │  │  System Health               │ │
│  │  #  Org       Plan   │  │  us-east-1   ● green  12 nodes│
│  │  1  AcmeCorp  Pro    │  │  eu-west-1   ● green   8 nodes│
│  │  2  DevShop   Free   │  │  ap-southeast-1 ● green 6 nodes│
│  │  3  CloudOps  Team   │  │  DB Primary  ● green  42ms  │ │
│  │  4  DataFlow  Pro    │  │  DB Replica  ● green  38ms  │ │
│  │  5  MicroApp  Free   │  │  Valkey      ● green   2ms  │ │
│  │  ...                 │  │  NATS        ● green   1ms  │ │
│  │  [View All]          │  │  MinIO       ● yellow 512ms │ │
│  └──────────────────────┘  └──────────────────────────────┘ │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Recent Alerts (Last 24h)                       [3]   │   │
│  │  🔴 21:03  Relay us-east-relay-03 offline        5m ago│   │
│  │  🟡 20:45  API error rate > 0.5%                 23m ago│   │
│  │  🟡 20:30  Disk usage > 80% on eu-west-relay-01 38m ago│   │
│  │  [View All Alerts]                                    │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 实时指标卡片定义

| 卡片名称 | 数据来源 | 刷新频率 | 计算公式 |
|----------|----------|----------|----------|
| Total Organizations | `organizations` 表 COUNT | 每 30s | `COUNT(*) WHERE status != 'deleted'` |
| Active Tunnels | Relay 心跳聚合 | 每 10s | `SUM(active_tunnels)` from all relays |
| Today's Traffic | ClickHouse `traffic_events` | 每 60s | `SUM(bytes_in + bytes_out) WHERE date = today()` |
| Monthly Traffic | ClickHouse 月度聚合 | 每 5min | `SUM(bytes_in + bytes_out) WHERE month = current_month()` |
| P2P Success Rate | Agent 上报 | 每 60s | `SUM(p2p_success_count) / SUM(total_connection_attempts)` |
| Active Relays | Relay 心跳状态 | 每 10s | `COUNT(*) WHERE last_heartbeat > NOW() - 90s` |
| MRR | Stripe + 聚合计算 | 每 1h | `SUM(monthly_subscription_amount) / 100` (单位：美元) |
| API Error Rate | API Gateway metrics | 每 30s | `SUM(5xx_count) / SUM(total_requests) * 100` |

### 2.4 系统健康状态指示

| 组件 | 监控指标 | 绿色阈值 | 黄色阈值 | 红色阈值 |
|------|----------|----------|----------|----------|
| Relay 节点 | 心跳延迟 + 负载率 | < 100ms & < 60% | < 500ms & < 80% | > 500ms 或 > 80% |
| DB Primary | 连接延迟 + 活跃连接数 | < 50ms & < 50 | < 200ms & < 100 | > 200ms 或 > 100 |
| DB Replica | 复制延迟 | < 1s | < 5s | > 5s |
| Valkey | PING 延迟 | < 5ms | < 20ms | > 20ms |
| NATS | 消息延迟 | < 10ms | < 50ms | > 50ms |
| MinIO | 读写延迟 | < 200ms | < 1s | > 1s |

### 2.5 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/dashboard/metrics` | 获取所有实时指标卡片数据 |
| `GET` | `/api/admin/v1/dashboard/traffic-trend?range=7d` | 获取流量趋势数据（range: `24h`/`7d`/`30d`） |
| `GET` | `/api/admin/v1/dashboard/recent-organizations?limit=10` | 获取最近注册组织列表 |
| `GET` | `/api/admin/v1/dashboard/system-health` | 获取所有组件健康状态 |
| `GET` | `/api/admin/v1/dashboard/recent-alerts?hours=24` | 获取最近告警列表 |

### 2.6 验收标准

- [ ] 登录管理后台后首页在 3 秒内完成所有指标卡片渲染
- [ ] 实时指标卡片（Relay 状态 / 活跃隧道）刷新延迟 < 15 秒
- [ ] 流量趋势图支持 24h / 7d / 30d 三个时间范围切换
- [ ] 流量趋势图数据更新延迟 < 5 分钟
- [ ] 系统健康状态任何一个组件变红时，导航栏出现红色警示圆点
- [ ] 异常告警面板同 Prometheus AlertManager 实时同步，延迟 < 30 秒
- [ ] 最近注册组织列表正确显示最近 10 条记录
- [ ] 所有数字使用千分位格式（如 1,247），流量使用自适应单位（GB/TB）

---

## 三、组织管理（Organizations）

### 3.1 用户故事

**作为** 运营人员
**我想要** 查看、搜索、筛选平台上的所有组织，并能进入任一组织的详情页查看完整信息
**以便** 我能快速定位特定客户、处理客户问题、执行风控操作

### 3.2 组织列表页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Organizations                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [🔍 Search...                        ] [Plan ▾] [Status ▾] │
│  [日期范围: 2026-05-01 ~ 2026-05-21            ] [Export CSV]│
│                                                              │
│  Total: 1,247 orgs   Page 1 of 125                          │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Name          │ Plan    │Users│Tunnels│BW Used │Status │⎸│
│  ├─────────────────────────────────────────────────────────┤│
│  │ Acme Corp     │Enterprise│  42 │  156  │ 8.2 TB │Active  │⎸│
│  │ DevStudio     │ Pro     │  12 │   34  │ 1.1 TB │Active  │⎸│
│  │ CloudOps Inc  │ Team    │   8 │   21  │ 420 GB │Active  │⎸│
│  │ StartupX      │ Free    │   3 │    2  │  12 GB │Active  │⎸│
│  │ DataFlow Ltd  │ Pro     │  15 │   48  │ 2.8 TB │Active  │⎸│
│  │ TestOrg       │ Free    │   1 │    0  │   0 GB │Inactive│⎸│
│  │ BannedCorp    │ Free    │   2 │    5  │  50 GB │Frozen  │⎸│
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Columns: Name | Plan | Users | Tunnels | BW Used | Status |│
│           | Tags | Created | Last Active | MFA Rate | Actions│
│                                                              │
│  排序：点击列头排序（Name / Created / Tunnels / BW Used）      │
│  筛选：Plan (Free/Pro/Team/Enterprise/Custom)                │
│        Status (Active/Inactive/Frozen/Deleted)               │
│        MFA Enabled (Yes/No)                                  │
│  Actions per row: [View] [Impersonate] [Freeze/Unfreeze]     │
│                   [Change Plan] [Delete]                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 组织列表字段说明

| 列名 | 来源 | 说明 |
|------|------|------|
| Name | `organizations.name` | 组织名称，点击进入详情页 |
| Plan | `subscriptions.plan_id` | 当前订阅计划 |
| Users | `org_members` COUNT | 组织内的用户总数 |
| Tunnels | `tunnels` COUNT | 隧道总数（含 active/inactive/stopped） |
| BW Used | 本月 | 当前结算周期的上行+下行总流量 |
| Status | `organizations.status` | active / inactive / frozen / deleted |
| Tags | `organizations.tags` | 运营标记（如 `enterprise`, `trial`, `vip`） |
| Created | `organizations.created_at` | 创建时间 |
| Last Active | `organizations.last_active_at` | 最后活跃时间（任一成员登录） |
| MFA Rate | 计算 | 启用 MFA 的用户数 / 总用户数 × 100% |

### 3.4 组织详情页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Organizations > Acme Corp                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Acme Corp                         [Enterprise] [🟢 Active] │
│  ID: org_3k2j1h4x       Created: 2025-11-15  (192 days ago)│
│                                                              │
│  Actions: [Impersonate] [Change Plan] [Freeze Org] [Delete]  │
│           [Impersonate as Owner]                             │
│                                                              │
│  ┌──Overview──┬──Users──┬──Tunnels──┬──Billing──┬──Audit Log┬│
│  │            │         │           │           │  ─┬──Timeline │
│  └────────────┘         │           │           │           ││
│                                                              │
│  ═══════════════ Overview Tab ═══════════════════════════════│
│                                                              │
│  ┌─────────────────────┐  ┌──────────────────────────────┐  │
│  │ Plan Information     │  │ Usage Summary (May 2026)     │  │
│  │                     │  │                              │  │
│  │ Plan:   Enterprise  │  │ Bandwidth: 8.2 TB / 10 TB   │  │
│  │ Price:  $499/mo     │  │ Tunnels:   156 / 500         │  │
│  │ Since:  2025-11-15  │  │ Users:     42 / Unlimited   │  │
│  │ Next Bill: Jun 1    │  │ P2P Hours: 4,820 / 10,000   │  │
│  │ Payment: 💳 ****4242│  │                              │  │
│  └─────────────────────┘  └──────────────────────────────┘  │
│                                                              │
│  ┌─────────────────────┐  ┌──────────────────────────────┐  │
│  │ Security Snapshot    │  │ Configuration                 │  │
│  │                     │  │                              │  │
│  │ MFA Rate:    85.7%  │  │ SSO:     ✅ Azure AD         │  │
│  │ API Keys:    12     │  │ Domains: 8 verified          │  │
│  │ SSO:          ✅    │  │ IP Allowlist: 192.168.0.0/16 │  │
│  │ Audit Logs:  2,847  │  │ Data Region: eu-west-1      │  │
│  │ Last Breach: Never  │  │ Feature Flags: P2P=on  Mesh=on│ │
│  └─────────────────────┘  └──────────────────────────────┘  │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Team Contacts                                            ││
│  │                                                          ││
│  │ Owner:   john@acmecorp.com (John Smith)    Last Login:   ││
│  │ Admin:   jane@acmecorp.com (Jane Doe)      today 09:42  ││
│  │ Billing: billing@acmecorp.com                            ││
│  │ Technical: devops@acmecorp.com                           ││
│  │                                                          ││
│  │ [Add Note]  Internal notes about this customer...        ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 3.5 详情页子标签说明

| 标签 | 内容 |
|------|------|
| **Overview** | 计划信息、用量摘要、安全快照、配置摘要、团队联系人、内部备注 |
| **Users** | 该组织的所有成员列表（姓名、邮箱、角色、MFA 状态、最后登录、API Key 数） |
| **Tunnels** | 该组织的所有隧道列表（名称、状态、域名、流量、在线时长、创建时间） |
| **Billing** | 订阅历史时间线、发票列表（可查看/下载 PDF）、支付方式、信用额度、已用额度 |
| **Audit Log** | 该组织内所有操作审计日志（可搜索、可筛选操作类型、可按时间排序） |
| **Timeline** | 可视化的关键事件时间轴（注册→首次创建隧道→首次付费→升级→添加 SSO→…） |

### 3.6 组织操作

| 操作 | 说明 | 后果 | 审计要求 |
|------|------|------|----------|
| **Freeze Organization** | 立即冻结组织所有功能 | 所有隧道断开、API 拒绝、Dashboard 只读 | 记录操作人、时间、原因、审批人 |
| **Unfreeze Organization** | 恢复被冻结的组织 | 隧道不会自动恢复，需用户手动重启 | 同上 |
| **Change Plan** | 手动修改订阅计划 | 附加选项：自定义价格、试用期天数、资源配额 | 记录变更前后计划、原因 |
| **Impersonate as Owner** | 以组织 Owner 身份跳转到 `app.omnitun.io` | 所有操作以 Impersonator 身份记录审计，用户在审计日志中看到 "Admin (xxx) impersonated you" | 强制记录，跳转前二次确认 |
| **Delete Organization** | 软删除组织（30 天可恢复） | 所有隧道停止、域名释放、证书吊销、数据保留 30 天 | Root Admin 审批方可执行 |
| **Add Internal Note** | 给组织添加内部运营备注（Markdown） | 仅管理后台可见，不对用户暴露 | 记录添加/修改时间 |

### 3.7 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/organizations` | 获取组织列表（支持分页/搜索/筛选/排序） |
| `GET` | `/api/admin/v1/organizations/{org_id}` | 获取组织详情 |
| `GET` | `/api/admin/v1/organizations/{org_id}/users` | 获取组织内用户列表 |
| `GET` | `/api/admin/v1/organizations/{org_id}/tunnels` | 获取组织内隧道列表 |
| `GET` | `/api/admin/v1/organizations/{org_id}/billing` | 获取组织计费信息 |
| `GET` | `/api/admin/v1/organizations/{org_id}/audit-logs` | 获取组织审计日志 |
| `GET` | `/api/admin/v1/organizations/{org_id}/timeline` | 获取组织活动时间轴 |
| `POST` | `/api/admin/v1/organizations/{org_id}/freeze` | 冻结组织 |
| `POST` | `/api/admin/v1/organizations/{org_id}/unfreeze` | 解冻组织 |
| `POST` | `/api/admin/v1/organizations/{org_id}/change-plan` | 变更订阅计划 |
| `POST` | `/api/admin/v1/organizations/{org_id}/impersonate` | 获取模拟登录 Token |
| `DELETE` | `/api/admin/v1/organizations/{org_id}` | 软删除组织 |
| `POST` | `/api/admin/v1/organizations/{org_id}/notes` | 添加/修改内部备注 |
| `POST` | `/api/admin/v1/organizations/{org_id}/tags` | 管理组织标签 |

### 3.8 验收标准

- [ ] 组织列表支持按名称模糊搜索（200ms 内返回结果）
- [ ] 组织列表支持按 Plan / Status / MFA / 创建日期范围筛选
- [ ] 组织列表支持多列排序，默认按创建时间倒序
- [ ] 组织列表支持导出为 CSV（当前筛选条件下的全部记录）
- [ ] 详情页 6 个标签切换流畅，每个标签数据独立加载
- [ ] Freeze 操作执行后 10 秒内生效（所有该组织的隧道断开）
- [ ] Impersonate 操作用户收到二次确认弹窗后才执行
- [ ] Impersonate 登录后在管理后台审计日志中生成专属事件类型
- [ ] Change Plan 操作自动同步到 Stripe（若为付费计划变更）
- [ ] Delete 操作需 Root Admin 确认，执行后 30 天内可在 "已删除组织" 中恢复

---

## 四、用户管理（Users）

### 4.1 用户故事

**作为** 运营人员
**我想要** 在全局范围内搜索任何用户，查看其完整信息和活动历史
**以便** 我能快速响应安全事件、处理客户支持请求、执行风控措施

### 4.2 用户列表页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Users                                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [🔍 Search by email/name...                    ] [Export]  │
│  筛选: [Status ▾] [MFA ▾] [Has API Key ▾]                  │
│                                                              │
│  Total: 5,231 users   Page 1 of 262                         │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Email                    │ Name     │Orgs│Roles│MFA│Status⎸│
│  ├────────────────────────────────────────────────────────┤ │
│  │ john@acmecorp.com        │John Smith│ 1  │Owner│✅ │Active │⎸│
│  │ jane@acmecorp.com        │Jane Doe  │ 1  │Admin│✅ │Active │⎸│
│  │ dev@startupx.io          │Dev Wang  │ 2  │Editor│❌│Active │⎸│
│  │ malicious@anon.mail      │Suspicious│ 1  │Viewer│❌│Frozen │⎸│
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Actions per row: [View Detail] [Login History]             │
│                   [Reset Password] [Force Logout]           │
│                   [Disable Account] [Impersonate]            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 用户详情页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Users > john@acmecorp.com                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  John Smith                               [🟢 Active]        │
│  john@acmecorp.com     User ID: usr_x7k3m2p1                │
│                                                              │
│  Actions: [Reset Password] [Force Logout All Sessions]       │
│           [Disable Account] [Impersonate]                    │
│                                                              │
│  ┌─Profile──┬──Organizations──┬──Login History──┬──Audit Log┐│
│  │          │                 │                 │           ││
│  │  ┌─────────────────────────────────────────────────────┐ ││
│  │  │ Basic Info                                          │ ││
│  │  │                                                     │ ││
│  │  │ Name:        John Smith                             │ ││
│  │  │ Email:       john@acmecorp.com          ✅ verified  │ ││
│  │  │ Phone:       +1-415-555-0123            ✅ verified  │ ││
│  │  │ Avatar:      [gravatar URL]                         │ ││
│  │  │ Created:     2025-11-15 09:23 UTC                   │ ││
│  │  │ Last Login:  2026-05-21 09:42 UTC                   │ ││
│  │  │ Login Method: Email + MFA                           │ ││
│  │  │ Timezone:    America/New_York                       │ ││
│  │  └─────────────────────────────────────────────────────┘ ││
│  │                                                           ││
│  │  ┌─────────────────────────────────────────────────────┐ ││
│  │  │ Security Status                                     │ ││
│  │  │                                                     │ ││
│  │  │ MFA:    ✅ Enabled (TOTP)        Enrolled: 2025-12-03│ ││
│  │  │ SSO:    ✅ Azure AD (acmecorp)                      │ ││
│  │  │ API Keys: 3 active / 0 revoked                      │ ││
│  │  │ Active Sessions: 2 (Chrome/Mac, CLI/Linux)          │ ││
│  │  │ Password Last Changed: 2026-04-10                   │ ││
│  │  │ Failed Login Attempts (24h): 0                      │ ││
│  │  │ Account Age: 192 days                               │ ││
│  │  └─────────────────────────────────────────────────────┘ ││
│  │                                                           ││
│  │  ┌─────────────────────────────────────────────────────┐ ││
│  │  │ IP Intelligence                                     │ ││
│  │  │                                                     │ ││
│  │  │ Last IP:      198.51.100.42  (San Francisco, US)   │ ││
│  │  │ Recent IPs:   198.51.100.42, 203.0.113.1, ...       │ ││
│  │  │ IP Anomaly:   ⚠️ Logged in from 3 countries in 24h  │ ││
│  │  │ VPN/Proxy:    ❌ Not detected                       │ ││
│  │  └─────────────────────────────────────────────────────┘ ││
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 4.4 用户详情页子标签

| 标签 | 内容 |
|------|------|
| **Profile** | 基本信息、安全状态、IP 情报（最近登录 IP 列表 + 异常检测） |
| **Organizations** | 该用户所属的所有组织及其角色（一个用户可属于多个组织） |
| **Login History** | 完整登录历史（时间、IP、User-Agent、国家、成功/失败、MFA 状态） |
| **Audit Log** | 该用户执行过的所有操作审计日志 |
| **API Keys** | 该用户的所有 API Key（名称、创建时间、最后使用、权限范围、状态） |

### 4.5 用户操作

| 操作 | 说明 | 适用角色 | 审计要求 |
|------|------|----------|----------|
| **Reset Password** | 发送密码重置邮件 | Full Admin | 记录操作人和目标用户 |
| **Force Logout All Sessions** | 立即失效该用户的所有 JWT Token 和 WebSocket 连接 | Full Admin | 记录操作人、时间、影响的会话数 |
| **Disable Account** | 禁止登录，但不删除数据 | Full Admin | 记录操作人和原因 |
| **Enable Account** | 恢复已禁用的账号 | Full Admin | 记录操作人 |
| **Impersonate** | 以该用户身份登录 Dashboard | Full Admin | 强制二次确认 + 完整审计 |
| **View Login History** | 查看最近 100 次登录记录 | Read-Only Ops | 查看操作本身也记录审计 |

### 4.6 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/users` | 获取用户列表（分页/搜索/筛选） |
| `GET` | `/api/admin/v1/users/{user_id}` | 获取用户详情 |
| `GET` | `/api/admin/v1/users/{user_id}/organizations` | 获取用户所属组织列表 |
| `GET` | `/api/admin/v1/users/{user_id}/login-history` | 获取用户登录历史 |
| `GET` | `/api/admin/v1/users/{user_id}/audit-logs` | 获取用户操作审计日志 |
| `GET` | `/api/admin/v1/users/{user_id}/api-keys` | 获取用户的 API Key 列表 |
| `POST` | `/api/admin/v1/users/{user_id}/reset-password` | 发送密码重置邮件 |
| `POST` | `/api/admin/v1/users/{user_id}/force-logout` | 强制退出所有会话 |
| `POST` | `/api/admin/v1/users/{user_id}/disable` | 禁用账号 |
| `POST` | `/api/admin/v1/users/{user_id}/enable` | 启用账号 |
| `POST` | `/api/admin/v1/users/{user_id}/impersonate` | 获取用户模拟登录 Token |

### 4.7 验收标准

- [ ] 全局用户搜索支持邮箱/姓名的模糊匹配，200ms 内返回结果
- [ ] 用户列表支持按 MFA、Status、API Key 数量筛选
- [ ] 用户详情页 5 个子标签数据独立加载，任一标签加载失败不影响其他标签
- [ ] IP 情报检测到单用户在 24 小时内从 3 个以上不同国家登录 → 自动标记 ⚠️
- [ ] Force Logout 执行后 5 秒内所有该用户的活跃 JWT 失效
- [ ] Disable Account 执行后该用户立即无法登录，返回 "Account disabled. Contact support."
- [ ] Reset Password 操作发送真实邮件（使用邮件模板）
- [ ] Impersonate 操作需 Root Admin 审批（或本人为 Root Admin）

---

## 五、Relay 节点管理（Infrastructure）

### 5.1 用户故事

**作为** 基础设施运维工程师
**我想要** 查看所有 Relay 节点的实时状态、指标和健康信息，并能对节点执行 Drain、下线、退役等操作
**以便** 我能在不停服的情况下完成节点维护、故障处理和容量规划

### 5.2 节点列表页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Relay Nodes                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  筛选: [Region ▾] [Status ▾] [K8s Cluster ▾]               │
│                                                              │
│  Summary: 12/12 Online | 0 Degraded | 0 Offline              │
│  Total Capacity: 89% utilized | Total Tunnels: 3,821         │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │Name               │Region   │Addr        │Status│Tunnels│C│
│  ├─────────────────────────────────────────────────────────┤│
│  │us-east-relay-01   │us-east-1│10.0.1.101  │🟢 Up │  412  │7│
│  │us-east-relay-02   │us-east-1│10.0.1.102  │🟢 Up │  389  │6│
│  │us-east-relay-03   │us-east-1│10.0.1.103  │🟡 High│ 401  │9│
│  │eu-west-relay-01   │eu-west-1│10.0.2.101  │🟢 Up │  318  │5│
│  │eu-west-relay-02   │eu-west-1│10.0.2.102  │🟢 Up │  305  │4│
│  │ap-se-relay-01     │ap-se-1  │10.0.3.101  │🟢 Up │  278  │6│
│  │ap-se-relay-02     │ap-se-1  │10.0.3.102  │🔴 Down│  —  │—│
│  │ap-se-relay-03     │ap-se-1  │10.0.3.103  │🟡 Drain│ 89  │3│
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Operations: [+ Register New Node]                           │
│  Actions per row: [View] [Drain] [Offline Maint] [Decomm]   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.3 节点列表字段

| 列名 | 说明 |
|------|------|
| Name | 节点名称（唯一标识） |
| Region | 部署区域（us-east-1 / eu-west-1 / ap-southeast-1 / ...） |
| Address | 节点 IP 或内部 DNS 名 |
| Status | Up (🟢) / High Load (🟡) / Draining (🟡) / Down (🔴) / Maintenance (🔵) |
| Tunnels | 当前承载的隧道数 |
| Capacity % | CPU/Memory/Bandwidth 利用率（取最高值） |
| Health | 健康检查分数 (0-100)，基于心跳延迟、错误率、资源使用 |
| Uptime | 节点持续运行时间 |
| Ingress BW | 当前入站带宽 (Mbps) |
| Egress BW | 当前出站带宽 (Mbps) |
| Connections | 当前活跃 TCP/QUIC 连接数 |
| Version | Relay 二进制版本号 |

### 5.4 节点详情页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Relay Nodes > us-east-relay-01             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  us-east-relay-01                     [🟢 UP]  [Uptime: 34d]│
│  10.0.1.101:8443     Version: v2.0.3     Region: us-east-1   │
│                                                              │
│  Actions: [Drain] [Schedule Maintenance] [Decommission]      │
│           [View Logs] [SSH Console]                          │
│                                                              │
│  ┌─Metrics────┬──Tunnels──┬──Connections──┬──Logs──┐       │
│  │            │           │               │        │       │
│  │  ═════════ Metrics Tab ═══════════════════════ │       │
│  │                                                 │       │
│  │  ┌──────────────────┐  ┌──────────────────────┐ │       │
│  │  │ Real-time Gauges  │  │ Resource Usage (24h) │ │       │
│  │  │                  │  │ 📈 CPU:  ████░░ 62%  │ │       │
│  │  │ CPU:     62%     │  │ 📈 MEM:  ███░░░ 45%  │ │       │
│  │  │ Memory:  45%     │  │ 📈 BW In: ████░░ 48% │ │       │
│  │  │ Disk:    31%     │  │ 📈 BW Out:████░░ 52% │ │       │
│  │  │ Bandwidth:42 Gbps│  │                      │ │       │
│  │  │ Conn:    3,124   │  │                      │ │       │
│  │  └──────────────────┘  └──────────────────────┘ │       │
│  │                                                 │       │
│  │  ┌────────────────────────────────────────────┐ │       │
│  │  │ Traffic Metrics (per second, last 60s)     │ │       │
│  │  │                                            │ │       │
│  │  │ Ingress:  2.4 Gbps ( ████████████████████ )│ │       │
│  │  │ Egress:   2.6 Gbps ( █████████████████████ )│ │       │
│  │  │ New Conn: 142/sec  ( ████████████████     )│ │       │
│  │  │ Errors:   0/sec    ( ░░░░░░░░░░░░░░░░░░░░ )│ │       │
│  │  │ P95 Lat:  3.2ms   ( ████░░░░░░░░░░░░░░░░ )│ │       │
│  │  └────────────────────────────────────────────┘ │       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.5 全球拓扑视图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Relay Nodes > Global Topology              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [Map View - 节点在地图上显示为圆点，颜色表示健康状态]        │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Latency Matrix (P50 ms between regions)               │ │
│  │                                                        │ │
│  │              us-east-1  eu-west-1  ap-se-1  ap-ne-1   │ │
│  │  us-east-1      0            72      178       162     │ │
│  │  eu-west-1     72             0      168       205     │ │
│  │  ap-se-1      178           168        0        86     │ │
│  │  ap-ne-1      162           205       86         0     │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Inter-region Tunnel Distribution                      │ │
│  │                                                        │ │
│  │  us-east-1 → eu-west-1 : 214 tunnels                   │ │
│  │  us-east-1 → ap-se-1   : 89 tunnels                    │ │
│  │  eu-west-1 → ap-se-1   : 156 tunnels                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.6 节点操作

| 操作 | 说明 | 前提条件 | 恢复方式 |
|------|------|----------|----------|
| **Register New Node** | 注册新的 Relay 节点到控制面 | 新节点已部署并能访问控制面 API | — |
| **Drain** | 优雅排空：停止接受新隧道，已有隧道迁移到同区域其他节点，排空完毕后节点变 Idle | 同区域有可用节点承载迁移 | 手动 Undrain |
| **Offline Maintenance** | 进入维护模式：同 Drain 但保留节点在线供运维操作 | 排空完成 | 手动退出维护模式 |
| **Decommission** | 永久下线退役节点，从控制面注销 | 隧道已全部迁移，节点已排空 | 不可逆（需重新注册） |
| **View Logs** | 实时查看该节点的结构化和非结构化日志 | 日志已接入集中式日志系统 | — |
| **SSH Console** | 通过管理后台的 Web Terminal 连接到节点 SSH | 节点 IP 在管理后台网络可达范围内 | — |

### 5.7 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/relay-nodes` | 获取节点列表（筛选/排序） |
| `GET` | `/api/admin/v1/relay-nodes/{node_id}` | 获取节点详情 |
| `GET` | `/api/admin/v1/relay-nodes/{node_id}/metrics` | 获取节点实时指标 |
| `GET` | `/api/admin/v1/relay-nodes/{node_id}/tunnels` | 获取节点上的隧道列表 |
| `GET` | `/api/admin/v1/relay-nodes/{node_id}/connections` | 获取节点上的活跃连接 |
| `GET` | `/api/admin/v1/relay-nodes/topology` | 获取全球拓扑数据（延迟矩阵） |
| `POST` | `/api/admin/v1/relay-nodes` | 注册新节点 |
| `POST` | `/api/admin/v1/relay-nodes/{node_id}/drain` | 排空节点 |
| `POST` | `/api/admin/v1/relay-nodes/{node_id}/undrain` | 取消排空 |
| `POST` | `/api/admin/v1/relay-nodes/{node_id}/maintenance` | 设置维护模式 |
| `POST` | `/api/admin/v1/relay-nodes/{node_id}/exit-maintenance` | 退出维护模式 |
| `DELETE` | `/api/admin/v1/relay-nodes/{node_id}` | 退役节点 |

### 5.8 验收标准

- [ ] 节点列表页面 2 秒内加载完成，所有实时指标刷新延迟 < 10 秒
- [ ] Drain 命令执行后，节点状态变为 "Draining"，不再接受新隧道创建
- [ ] Drain 完成后，已有隧道在 30 秒内迁移到同区域其他节点
- [ ] 迁移过程中隧道中断时间 < 5 秒（无缝迁移）
- [ ] 拓扑视图延迟矩阵数据来源于实际 agent 探针测量，非 IP 地理推算
- [ ] 节点宕机 90 秒后被自动标记为 Down，触发 Prometheus 告警
- [ ] 节点退役操作需要二次确认，二次确认按钮在点击后倒计时 10 秒才可点
- [ ] Decommission 操作不可逆（除非重新注册），需 Root Admin 确认
- [ ] Register New Node 支持填写节点地址、区域、标签等信息

---

## 六、证书管理（TLS / Certificates）

### 6.1 用户故事

**作为** 安全运维工程师
**我想要** 查看平台所有 TLS 证书的状态（包括系统级泛域名证书和租户自定义域名证书）
**以便** 我能预防证书过期导致的服务中断，并在证书异常时快速响应

### 6.2 证书管理页线框图

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Certificates                                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─System Certificates──┬──Tenant Certificates──┐          │
│  │                      │                       │          │
│  │  ═══════ System Certs ═══════════            │          │
│  │                                               │          │
│  │  ┌──────────────────────────────────────────┐ │          │
│  │  │Domain              │Expiry   │Status│Renew│ │          │
│  │  ├──────────────────────────────────────────┤ │          │
│  │  │*.omnitun.io        │Jul 14,26│🟢 Val│Auto │ │          │
│  │  │api.omnitun.io      │Jul 14,26│🟢 Val│Auto │ │          │
│  │  │relay.omnitun.io    │Jul 14,26│🟢 Val│Auto │ │          │
│  │  │admin.omnitun.io    │Aug 03,26│🟢 Val│Auto │ │          │
│  │  │app.omnitun.io      │Aug 03,26│🟢 Val│Auto │ │          │
│  │  └──────────────────────────────────────────┘ │          │
│  │                                               │          │
│  │  ACME Provider: Let's Encrypt (Production)    │          │
│  │  ACME Account:   admin@omnitun.io             │          │
│  │  Challenge Type: DNS-01 (Cloudflare API)      │          │
│  │  Last Renewal:   2026-05-14 03:12 UTC  ✅     │          │
│  │  Next Renewal:   2026-06-13 (auto)           │          │
│  │                                               │          │
│  │  Actions: [Manual Renew Now] [Force Revoke]   │          │
│  │           [Change ACME Provider]              │          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 6.3 租户证书列表

```
┌─────────────────────────────────────────────────────────────┐
│  ┌──System Certificates──┬──Tenant Certificates──┐         │
│                          │                        │         │
│  筛选: [Status ▾] [Org ▾] [Expires Within ▾]     │         │
│                          │                        │         │
│  ┌───────────────────────────────────────────────┼─────────┐│
│  │Domain                │Org       │Expiry │Status│Actions ││
│  ├───────────────────────────────────────────────┼─────────┤│
│  │api.acmecorp.com      │Acme Corp │Sep 1  │🟢 Val│[Revoke]││
│  │tunnel.devshop.io     │DevShop   │Aug 12 │🟡 Exp│[Renew] ││
│  │*.cloudops.internal   │CloudOps  │Jun 5  │🔴 Err│[Retry] ││
│  │app.dataflow.io       │DataFlow  │Oct 20 │🟢 Val│[Revoke]││
│  │status.startupx.io    │StartupX  │Jul 30 │🟢 Val│[Revoke]││
│  └───────────────────────────────────────────────┼─────────┘│
│                                                    │         │
│  Total: 847 certs | Valid: 812 | Expiring <30d: 28 |         │
│  | Expired: 5 | Error: 2                                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 6.4 证书状态定义

| 状态 | 图标 | 定义 | 触发条件 |
|------|------|------|----------|
| Valid | 🟢 | 证书有效 | 到期 > 30 天 |
| Expiring Soon | 🟡 | 即将到期 | 到期 ≤ 30 天 |
| Expired | 🔴 | 已过期 | 到期 < 当前时间 |
| Error | 🔴 | 签发/续期失败 | ACME 错误、DNS 验证失败等 |
| Revoked | ⚫ | 已吊销 | 管理员手动吊销 |

### 6.5 告警策略

| 告警 | 触发条件 | 严重度 | 通知渠道 |
|------|----------|--------|----------|
| 泛域名证书 30 天到期 | `expiry - now <= 30d` | Warning | Slack #infra |
| 泛域名证书 7 天到期 | `expiry - now <= 7d` | Critical | PagerDuty + Slack |
| 租户证书 14 天到期 | `expiry - now <= 14d` | Info | 仅 Dashboard |
| ACME 续期失败 | 连续 3 次失败 | Critical | PagerDuty |
| DNS 验证失败 | 连续 3 次失败 | Warning | Slack #infra |

### 6.6 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/certificates/system` | 获取系统证书列表 |
| `GET` | `/api/admin/v1/certificates/tenants` | 获取租户证书列表（分页/筛选） |
| `GET` | `/api/admin/v1/certificates/tenants/{cert_id}` | 获取租户证书详情 |
| `POST` | `/api/admin/v1/certificates/system/renew` | 手动续期系统证书 |
| `POST` | `/api/admin/v1/certificates/tenants/{cert_id}/renew` | 手动续期租户证书 |
| `POST` | `/api/admin/v1/certificates/tenants/{cert_id}/revoke` | 吊销租户证书 |
| `POST` | `/api/admin/v1/certificates/system/revoke` | 吊销系统证书（需 Root Admin） |
| `PUT` | `/api/admin/v1/certificates/acme-config` | 更新 ACME 配置 |

### 6.7 验收标准

- [ ] 系统证书和租户证书分两个标签展示，默认显示系统证书
- [ ] 泛域名证书到期前 30 天自动续期（无需人工介入）
- [ ] 续期失败时，错误信息清晰展示（如 "DNS-01 Challenge failed: Cloudflare API timeout"）
- [ ] 租户证书支持按状态、组织、到期时间范围筛选
- [ ] Force Revoke 操作需二次确认，执行后证书立即失效且无法恢复
- [ ] 证书过期触发 PagerDuty 告警，持续直到问题解决
- [ ] 证书管理页显示最近一次 ACME 操作日志（如 "2026-05-14 03:12: Order created → Challenge validated → Certificate issued"）

---

## 七、安全与滥用管理（Security）

### 7.1 用户故事

**作为** 安全运营分析师
**我想要** 配置滥用检测规则、审查用户举报、管理 IP 黑名单、查看全局安全事件
**以便** 我能快速识别和响应平台上的恶意行为，保护合法用户的服务稳定

### 7.2 安全中心页面

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Security Center                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─Overview──┬──Abuse Rules──┬──Reports──┬──IP Blacklist──┬─│
│  │           │              │          │                │  │
│  │  ═════════ Overview ═════════════════════════════════  │
│  │                                                         │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐          │
│  │  │ Open        │ │ Rules      │ │ Blocked    │          │
│  │  │ Reports     │ │ Triggered  │ │ IPs Today  │          │
│  │  │    12       │ │  Today     │ │   1,247    │          │
│  │  │  ↑3 new     │ │    48      │ │  ↓12%      │          │
│  │  └────────────┘ └────────────┘ └────────────┘          │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐          │
│  │  │ Banned      │ │ Failed     │ │ Suspicious  │          │
│  │  │ Orgs        │ │ Logins/24h │ │ Tunnels    │          │
│  │  │    3        │ │   4,281    │ │    5        │          │
│  │  └────────────┘ └────────────┘ └────────────┘          │
│  │                                                         │
│  │  ┌───────────────────────────────────────────────────┐  │
│  │  │ Recent Security Events (Live)                     │  │
│  │  │                                                   │  │
│  │  │ 21:08  🔴 C2 Pattern Detected   org_xyz 5 conns  │  │
│  │  │ 20:52  🟡 Unusual Traffic        org_abc 12 GB/h │  │
│  │  │ 20:31  🟡 Brute Force Attempt    user@test.com   │  │
│  │  │ 20:15  🟡 Port Scan Detected     from 198.51.x   │  │
│  │  │ 19:58  🔴 Malware C2 回连        org_mal         │  │
│  │  └───────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.3 滥用检测规则

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Security Center > Abuse Detection Rules    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [+ Create Rule]                                            │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Rule Name         │Condition                │Action│Status││
│  ├────────────────────────────────────────────────────────┤ │
│  │Bandwidth Spike   │BW > 5x baseline in 1h   │Alert │🟢 On ││
│  │Malicious Pattern │matches C2 regex         │Block │🟢 On ││
│  │Port Scanner      │>100 unique dest ports/h │Alert │🟢 On ││
│  │Brute Force Login │>10 failed logins/5min   │Block │🟢 On ││
│  │Tor Exit Node     │IP in TorDB              │Flag  │🟡 Dry││
│  │Crypto Mining     │Stratum protocol detect  │Block │🟢 On ││
│  │Phishing Host     │Domain ML score > 0.8    │Flag  │🟢 On ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Actions per rule: [Edit] [Toggle On/Off] [Delete]          │
│  Action types: Alert (通知) / Flag (标记不阻断) / Block (阻断)│
│  Rule engine: CEL (Common Expression Language)              │
│               or Rego (OPA) policies                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.4 举报队列

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Security Center > Report Queue             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  筛选: [Status ▾] [Type ▾] [Priority ▾]                    │
│                                                              │
│  Open: 12 | In Review: 3 | Resolved: 247                    │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │#  │Reporter    │Target     │Type     │Status│Date      ││
│  ├────────────────────────────────────────────────────────┤ │
│  │ 1 │user@co.com │tunnel_123 │Phishing │Open  │Today 14:││
│  │ 2 │admin@org.io│org_abc    │SPAM     │Open  │Today 11:││
│  │ 3 │dev@prod.io │tunnel_456 │Malware  │Review│Yesterday││
│  │ 4 │ops@serv.io │tunnel_789 │DDoS     │Open  │May 19   ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Click row to see full report detail →                       │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Report #1 Detail                                       │ │
│  │                                                        │ │
│  │ Reporter:   user@company.com                           │ │
│  │ Target:     tunnel_123 (api.fakeservice.com)           │ │
│  │ Type:       Phishing                                   │ │
│  │ Priority:   High                                       │ │
│  │ Evidence:   Screenshot + URL + Description             │ │
│  │                                 [View Evidence Files]  │ │
│  │                                                        │ │
│  │ Resolution Actions:                                    │ │
│  │  [Warn Owner] [Suspend Tunnel] [Ban Org] [Dismiss]     │ │
│  │  [Add to IP Blacklist] [Escalate]                      │ │
│  │                                                        │ │
│  │ Internal Notes: _______________________________        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.5 IP 黑名单管理

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Security Center > IP Blacklist             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [+ Add Entry]  [Bulk Import]  [Export List]               │
│                                                              │
│  Total: 1,247 entries | Active: 1,203 | Expired: 44         │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │CIDR/Prefix   │Reason      │Source     │Expires  │Actio││
│  ├────────────────────────────────────────────────────────┤ │
│  │192.0.2.0/24  │DDoS source │Auto(AI)   │Never    │[Del]││
│  │198.51.100.0/24│SPAM botnet│Manual     │Never    │[Del]││
│  │203.0.113.0/28│Port scan  │Auto(Rule) │Jun 15   │[Del]││
│  │10.10.10.5    │Abuse rep  │Manual(ops)│Jul 01   │[Del]││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Add Entry Form:                                            │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ CIDR/Prefix:  [                ]  e.g. 192.0.2.0/24   │ │
│  │ Reason:       [                ]                        │ │
│  │ Source:       [Manual ▾]  (Manual / Auto(AI) / Auto(Rule))│
│  │ Expires:      [Never ▾]  (24h / 7d / 30d / 90d / Never)│ │
│  │ Apply To:     ☑ API Access  ☑ Tunnel Ingress           │ │
│  │ Notes:        [                ]                        │ │
│  │                                                        │ │
│  │              [Add to Blacklist]                         │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.6 安全事件列表

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Security Center > Security Events           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  筛选: [Severity ▾] [Type ▾] [Org ▾] [Date Range]          │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Time   │Severity│Type        │Org/User    │Detail       ││
│  ├────────────────────────────────────────────────────────┤ │
│  │21:08  │🔴 Crit │C2 Activity  │org_xyz     │5 conns to  ││
│  │20:52  │🟡 Warn │Unusual BW   │org_abc     │12 GB/h (10x)│
│  │20:31  │🟡 Warn │Brute Force  │user@test   │15 fails/3m ││
│  │20:15  │🟡 Warn │Port Scan    │(anon  IP)  │198.51.100.1││
│  │19:58  │🔴 Crit │Malware      │org_mal     │tunnel_456  ││
│  │19:30  │🟢 Info │Login Anom   │user@g.cn   │CN logins   ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Actions per event: [View Detail] [Acknowledge]              │
│                     [Ban Org/User] [Add to Blacklist]        │
│                     [Mark False Positive]                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 7.7 安全事件类型定义

| 类型 | 严重度 | 描述 | 自动动作 |
|------|--------|------|----------|
| Brute Force Attack | Warning | 单一账号短时间内大量失败登录 | 临时锁定 IP 10 分钟 |
| Credential Stuffing | Critical | 多个账号来自同一 IP 的失败登录 | 自动拦截 IP |
| C2 Activity | Critical | 隧道流量匹配已知 C2 模式 | 自动断开隧道 + 冻结组织 |
| Malware Distribution | Critical | 隧道托管恶意软件 | 自动断开隧道 + 冻结组织 |
| Phishing Site | Critical | 隧道托管钓鱼页面 | 自动断开隧道 + 通知组织 |
| DDoS Source | Warning | 从 OmniTun 隧道发起出站 DDoS | 自动限速 |
| Port Scanning | Warning | 通过隧道进行端口扫描 | 自动限速 + 告警 |
| Unusual Bandwidth | Warning | 短时间内流量是基线的 10 倍+ | 告警通知运营人工确认 |
| Login Anomaly | Info | 异常地点/设备登录 | 仅记录，通知用户 |
| TOR Exit Node | Info | 隧道流量来自已知 TOR 节点 | 标记，可选阻断 |

### 7.8 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/security/overview` | 获取安全概览数据 |
| `GET` | `/api/admin/v1/security/rules` | 获取滥用检测规则列表 |
| `POST` | `/api/admin/v1/security/rules` | 创建检测规则 |
| `PUT` | `/api/admin/v1/security/rules/{rule_id}` | 更新检测规则 |
| `DELETE` | `/api/admin/v1/security/rules/{rule_id}` | 删除检测规则 |
| `POST` | `/api/admin/v1/security/rules/{rule_id}/toggle` | 启用/禁用规则 |
| `GET` | `/api/admin/v1/security/reports` | 获取举报队列（筛选/分页） |
| `GET` | `/api/admin/v1/security/reports/{report_id}` | 获取举报详情 |
| `POST` | `/api/admin/v1/security/reports/{report_id}/action` | 对举报采取行动 |
| `GET` | `/api/admin/v1/security/blacklist` | 获取 IP 黑名单 |
| `POST` | `/api/admin/v1/security/blacklist` | 添加 IP 黑名单条目 |
| `DELETE` | `/api/admin/v1/security/blacklist/{entry_id}` | 删除 IP 黑名单条目 |
| `GET` | `/api/admin/v1/security/events` | 获取安全事件列表（筛选/分页） |
| `GET` | `/api/admin/v1/security/events/{event_id}` | 获取安全事件详情 |
| `POST` | `/api/admin/v1/security/events/{event_id}/acknowledge` | 确认事件 |
| `POST` | `/api/admin/v1/security/events/{event_id}/false-positive` | 标记误报 |
| `POST` | `/api/admin/v1/security/orgs/{org_id}/ban` | 封禁组织 |
| `POST` | `/api/admin/v1/security/orgs/{org_id}/unban` | 解禁组织 |
| `POST` | `/api/admin/v1/security/orgs/{org_id}/force-password-reset` | 强制组织内所有用户重置密码 |

### 7.9 验收标准

- [ ] 滥用检测规则支持 CEL 表达式语法（或 Rego 策略），可自定义字段
- [ ] 检测规则支持三种动作：Alert（告警）/ Flag（标记）/ Block（阻断）
- [ ] Block 动作执行后，匹配流量在 1 秒内被丢弃
- [ ] 举报队列支持按状态、类型、优先级筛选和按时间排序
- [ ] 举报处理操作（Warn/Suspend/Ban/Dismiss）完整记录审计日志
- [ ] Warn 操作发送邮件通知组织 Owner，包含违规说明和改进建议
- [ ] Ban 操作执行后，被禁组织所有 API 请求返回 403 "Organization banned"
- [ ] IP 黑名单支持 CIDR 格式（包括 IPv6），且提供有效性校验
- [ ] IP 黑名单支持过期时间（从不 / 24h / 7d / 30d / 90d）
- [ ] 安全事件列表实时更新（WebSocket 推送），延迟 < 5 秒
- [ ] C2 Activity / Malware / Phishing 类型事件自动触发 Block + 通知

---

## 八、Feature Flag 管理

### 8.1 用户故事

**作为** 产品经理 / 运营人员
**我想要** 在不发布新版本的情况下，按组织或百分比灰度开启新功能
**以便** 我能安全地进行功能验证、A/B 测试和渐进式发布

### 8.2 Feature Flag 页面

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Feature Flags                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [+ Create Flag]                                             │
│                                                              │
│  Active Flags: 12 | Total: 23 | Archived: 8                 │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Flag Key          │Status│Rollout│Target    │Modified   ││
│  ├────────────────────────────────────────────────────────┤ │
│  │p2p_mesh          │🟢 ON │100%   │All orgs  │2026-05-15 ││
│  │traffic_inspection│🟡 50% │50%    │Percentage│2026-05-18 ││
│  │custom_domain_v2  │🟡 10% │10%    │Whitelist │2026-05-20 ││
│  │udp_tunnel_beta   │🔴 OFF │0%     │—         │2026-04-01 ││
│  │advanced_analytics│🟢 ON │100%   │Team+     │2026-05-10 ││
│  │dark_mode_v2      │🟡 75% │75%    │Percentage│2026-05-21 ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Click row to expand detail/settings:                        │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Flag: traffic_inspection                                 ││
│  │                                                          ││
│  │ Type:    Percentage Rollout                              ││
│  │ Current: 50%                                             ││
│  │ Target:  All organizations (by org_id hash)              ││
│  │                                                          ││
│  │ Rollout History:                                         ││
│  │  2026-05-18 14:00 → 25%  (by admin@omnitun.io)          ││
│  │  2026-05-19 09:30 → 50%  (by admin@omnitun.io)          ││
│  │                                                          ││
│  │  [Adjust to: 75% ▼]   [Update Rollout]                  ││
│  │                                                          ││
│  │  ⚠️ Whitelist Overrides (always ON for these orgs):      ││
│  │  org_abc, org_xyz                   [Manage Whitelist]   ││
│  │                                                          ││
│  │  [Toggle OFF]  [Archive Flag]                            ││
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 8.3 Feature Flag 定义

| 属性 | 类型 | 说明 |
|------|------|------|
| Flag Key | string | 唯一标识，如 `p2p_mesh` |
| Display Name | string | 中文名，如 "P2P Mesh 组网" |
| Description | text | 功能描述 |
| Type | enum | `boolean` / `percentage` / `whitelist` / `plan_based` |
| Status | enum | `on` / `off` / `archived` |
| Rollout % | int | 0-100，仅 percentage 类型生效 |
| Whitelist Orgs | []string | 白名单组织 ID 列表 |
| Target Plans | []string | 适用计划（如 Team+），仅 plan_based 类型生效 |
| Created By | user_id | 创建人 |
| Created At | timestamp | 创建时间 |
| Last Modified | timestamp | 最后修改时间 |

### 8.4 变更生效机制

| 机制 | 说明 |
|------|------|
| **即时生效** | 所有 Flag 变更（Toggle / 调整百分比 / 修改白名单）立即反映在 API 响应中 |
| **SDK 缓存** | 客户端 SDK 每 30 秒拉取一次 Flag 状态（可配置） |
| **强制刷新** | 提供 `POST /api/admin/v1/feature-flags/force-refresh` 端点强制所有 Relays/Clients 立即刷新 |
| **回滚能力** | 任意 Flag 可一键回滚到上一个状态 |

### 8.5 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/feature-flags` | 获取 Flag 列表 |
| `GET` | `/api/admin/v1/feature-flags/{flag_key}` | 获取 Flag 详情 |
| `POST` | `/api/admin/v1/feature-flags` | 创建新 Flag |
| `PUT` | `/api/admin/v1/feature-flags/{flag_key}` | 更新 Flag |
| `DELETE` | `/api/admin/v1/feature-flags/{flag_key}` | 删除 Flag（仅限未发布的） |
| `POST` | `/api/admin/v1/feature-flags/{flag_key}/toggle` | 切换开/关 |
| `PUT` | `/api/admin/v1/feature-flags/{flag_key}/rollout` | 调整灰度百分比 |
| `PUT` | `/api/admin/v1/feature-flags/{flag_key}/whitelist` | 管理白名单 |
| `POST` | `/api/admin/v1/feature-flags/{flag_key}/archive` | 归档 Flag |
| `POST` | `/api/admin/v1/feature-flags/force-refresh` | 强制所有节点刷新 Flag |

### 8.6 验收标准

- [ ] Flag 变更后（Toggle / 调整百分比），用户端 / Relay 在 30 秒内感知到变化
- [ ] 百分比灰度基于 `hash(org_id + flag_key) % 100` 确定性算法，同一组织始终在同一桶内
- [ ] 白名单组织不受百分比灰度限制，白名单中的组织始终为 ON
- [ ] 强制刷新端点调用后，确认所有 Relay 节点已更新 Flag 缓存的回执
- [ ] Flag 归档后不再参与判断，但保留历史记录不删除
- [ ] 每次 Flag 变更记录完整审计日志（谁、何时、改了哪个 Flag、从什么改到什么）
- [ ] 支持一键回滚到上一个状态（保留最近 10 次变更历史）

---

## 九、全局公告（Announcements）

### 9.1 用户故事

**作为** 运营人员
**我想要** 创建、定时发布系统公告，并控制公告的受众和展示位置
**以便** 我能有效传达维护通知、功能更新、安全警告等信息给目标用户群体

### 9.2 公告管理页面

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > Announcements                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [+ Create Announcement]                                     │
│                                                              │
│  Active: 3 | Scheduled: 2 | Expired: 47                     │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Title                 │Severity│Status  │Schedule   │Live││
│  ├────────────────────────────────────────────────────────┤ │
│  │Scheduled Maintenance │⚠️ Wrn  │🟢 Live │May 25 02:│ Yes││
│  │v2.1 Feature Update   │ℹ️ Info │🟢 Live │May 20 09:│ Yes││
│  │Security: MFA Required │🔴 Crit │🟢 Live │May 15 00:│ Yes││
│  │Holiday Price Discount│ℹ️ Info │⏳ Schd │Jun 01 00:│ No ││
│  │New Region: ap-ne-1   │ℹ️ Info │⏳ Schd │Jun 15 00:│ No ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Actions per row: [Edit] [Duplicate] [Archive] [Delete]     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 9.3 创建/编辑公告表单

```
┌─────────────────────────────────────────────────────────────┐
│  Create New Announcement                                     │
│                                                              │
│  Title:     [                               ]                │
│                                                              │
│  Severity:  ○ ℹ️ Information   ● ⚠️ Warning   ○ 🔴 Critical │
│                                                              │
│  Body:      ┌─────────────────────────────────────────┐     │
│             │ # Scheduled Maintenance                  │     │
│             │                                          │     │
│             │ OmniTun will undergo scheduled           │     │
│             │ maintenance on **May 25, 2026**          │     │
│             │ from 02:00 to 04:00 UTC.                 │     │
│             │                                          │     │
│             │ During this time:                        │     │
│             │ - New tunnels will be temporarily        │     │
│             │   unavailable                            │     │
│             │ - Existing tunnels will not be affected  │     │
│             │ - API may have degraded performance      │     │
│             │                                          │     │
│             │ [Preview]                                │     │
│             └─────────────────────────────────────────┘     │
│                                                              │
│  Target Audience:                                           │
│    ○ All Users                                              │
│    ○ Specific Plans: [Free] [Pro] [Team] [Enterprise]       │
│    ○ Specific Organizations: [_____] [Search Orgs...]       │
│    ○ Specific Regions:    [us-east-1] [eu-west-1] [...]     │
│                                                              │
│  Display Location:                                          │
│    ☑ Dashboard Banner (top of app.omnitun.io)               │
│    ☑ Admin Dashboard Banner (top of admin.omnitun.io)       │
│    ☐ Email notification to affected users                   │
│                                                              │
│  Schedule:                                                  │
│    ○ Publish immediately                                    │
│    ● Schedule for later: [2026-05-25] [02:00] UTC          │
│    ○ Set expiry: [2026-05-25] [04:00] UTC                   │
│                                                              │
│  Dismissible: ☑ Users can dismiss this announcement         │
│                                                              │
│                                    [Save Draft] [Publish]   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 9.4 公告属性定义

| 属性 | 类型 | 说明 |
|------|------|------|
| Title | string | 公告标题（最多 120 字符） |
| Body | markdown | 公告正文（Markdown 格式） |
| Severity | enum | `information` / `warning` / `critical` |
| Target Audience | enum | `all` / `plans` / `organizations` / `regions` |
| Target Plan IDs | []string | 适用计划列表 |
| Target Org IDs | []string | 适用组织列表 |
| Target Regions | []string | 适用区域列表 |
| Display Locations | []enum | `dashboard_banner` / `admin_banner` / `email` |
| Publish At | timestamp | 定时发布时间（NULL = 立即发布） |
| Expire At | timestamp | 过期时间（NULL = 永不过期） |
| Dismissible | bool | 用户是否可以关闭横幅 |
| Status | enum | `draft` / `scheduled` / `live` / `expired` / `archived` |

### 9.5 用户端展示效果规范

| 严重度 | 横幅颜色 | 图标 | 可否关闭 | 示例场景 |
|--------|----------|------|----------|----------|
| Information | 蓝色 | ℹ️ | 可关闭 | 新功能上线通知 |
| Warning | 黄色/橙色 | ⚠️ | 可关闭 | 计划维护预告 |
| Critical | 红色 | 🛑 | **不可关闭** | 安全事件应急通知 |

### 9.6 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/announcements` | 获取公告列表（分页/筛选） |
| `GET` | `/api/admin/v1/announcements/{id}` | 获取公告详情 |
| `POST` | `/api/admin/v1/announcements` | 创建新公告 |
| `PUT` | `/api/admin/v1/announcements/{id}` | 更新公告 |
| `DELETE` | `/api/admin/v1/announcements/{id}` | 删除公告（仅限 Draft） |
| `POST` | `/api/admin/v1/announcements/{id}/publish` | 立即发布 |
| `POST` | `/api/admin/v1/announcements/{id}/archive` | 归档公告 |
| `POST` | `/api/admin/v1/announcements/{id}/duplicate` | 复制公告 |

### 9.7 验收标准

- [ ] 公告支持 Markdown 格式正文，预览功能实时渲染
- [ ] 定时发布在指定 UTC 时间准时上线，误差 < 1 分钟
- [ ] 过期公告在指定时间后自动从用户 Dashboard 横幅移除
- [ ] Critical 级别的公告在用户端不可关闭（Dismissible = false）
- [ ] 按计划/组织/地区筛选的受众限制正确生效（非目标用户看不到公告）
- [ ] 同一时间最多显示 3 条 Dashboard 横幅公告（超出按优先级排序）
- [ ] 公告创建/修改/删除操作记录完整审计日志
- [ ] 支持公告历史版本对比（查看编辑前后的差异）

---

## 十、系统配置（System Config）

### 10.1 用户故事

**作为** 平台架构师 / SRE
**我想要** 在管理后台中调整全局系统参数（速率限制、邮件模板、维护模式、日志级别）
**以便** 我能在不修改代码/不重启服务的情况下快速响应线上问题和运营需求

### 10.2 系统配置页面

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > System Configuration                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─Rate Limits──┬──Email Templates──┬──Maintenance──┬──Logs─┐│
│  │              │                  │              │        ││
│  │  ═════════ Rate Limits ═════════════════════  │        ││
│  │                                                │        ││
│  │  Per-Plan API Rate Limits:                     │        ││
│  │                                                │        ││
│  │  Plan      │Requests/min│Burst│Concurrent     │        ││
│  │  ──────────┼───────────┼─────┼────────────── │        ││
│  │  Free      │    60     │ 10  │ 5             │        ││
│  │  Pro       │   600     │ 50  │ 20            │        ││
│  │  Team      │  3,000    │ 200 │ 50            │        ││
│  │  Enterprise│ 10,000    │ 500 │ 100           │        ││
│  │  Custom    │  Custom   │ —   │ —             │        ││
│  │                                                │        ││
│  │  Per-IP Rate Limits:                           │        ││
│  │  Login:    10 req/min (anti-brute-force)       │        ││
│  │  API:      300 req/min                         │        ││
│  │                                                │        ││
│  │  [Save Changes]                                 │        ││
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 10.3 邮件模板

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > System Config > Email Templates            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Template List:                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Template        │Subject           │Last Modified       ││
│  ├────────────────────────────────────────────────────────┤ │
│  │welcome         │Welcome to OmniTun│2026-05-01 (admin)  ││
│  │verify_email    │Verify your email │2026-05-01 (admin)  ││
│  │reset_password  │Reset your password│2026-05-10 (admin) ││
│  │invoice_ready   │Your invoice      │2026-04-15 (system) ││
│  │payment_failed  │Payment failed    │2026-04-15 (system) ││
│  │trial_ending    │Your trial ends   │2026-05-12 (admin)  ││
│  │ban_notification│Account suspended │2026-05-20 (admin)  ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Click to edit:                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Template: welcome                                       │ │
│  │                                                         │ │
│  │ Subject: [Welcome to OmniTun, {{.Name}}! 🎉             │ │
│  │                                                         │ │
│  │ ┌─────────────────────────────────────────────────────┐ │ │
│  │ │ HTML Body:                                          │ │ │
│  │ │                                                     │ │ │
│  │ │ <html>                                              │ │ │
│  │ │ <body>                                              │ │ │
│  │ │   <h1>Welcome, {{.Name}}!</h1>                      │ │ │
│  │ │   <p>Thanks for joining OmniTun.</p>                │ │ │
│  │ │   <a href="{{.DashboardURL}}">Go to Dashboard</a>   │ │ │
│  │ │   <p>Your plan: {{.Plan}}</p>                       │ │ │
│  │ │ </body>                                             │ │ │
│  │ │ </html>                                             │ │ │
│  │ └─────────────────────────────────────────────────────┘ │ │
│  │                                                         │ │
│  │ Available Variables:                                    │ │
│  │   {{.Name}} {{.Email}} {{.Plan}} {{.DashboardURL}}     │ │
│  │   {{.OrgName}} {{.VerificationURL}} {{.ResetURL}}      │ │
│  │   {{.InvoiceAmount}} {{.InvoiceURL}} {{.TrialDaysLeft}}│ │
│  │                                                         │ │
│  │ [Send Test Email to: _______________]                   │ │
│  │                                                         │ │
│  │ [Preview]  [Reset to Default]  [Save Changes]          │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 10.4 维护模式

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > System Config > Maintenance Mode           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Current Status:  🟢 NORMAL OPERATIONS                      │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Enable Maintenance Mode                                │ │
│  │                                                        │ │
│  │ ▢ API (all endpoints return 503)                       │ │
│  │ ▢ Dashboard (show maintenance page)                    │ │
│  │ ▢ Agent connections (reject new connections)           │ │
│  │                                                        │ │
│  │ Maintenance Message (shown to users):                  │ │
│  │ ┌────────────────────────────────────────────────────┐ │ │
│  │ │ OmniTun is currently undergoing maintenance.       │ │ │
│  │ │ We'll be back shortly.                             │ │ │
│  │ │                                                    │ │ │
│  │ │ Expected completion: 2026-05-25 04:00 UTC          │ │ │
│  │ │ Status page: https://status.omnitun.io             │ │ │
│  │ └────────────────────────────────────────────────────┘ │ │
│  │                                                        │ │
│  │ Whitelist IPs (can bypass maintenance):                │ │
│  │ ┌────────────────────────────────────────────────────┐ │ │
│  │ │ 10.0.0.0/8 (Internal network)                      │ │ │
│  │ │ [Add IP/CIDR...]                                   │ │ │
│  │ └────────────────────────────────────────────────────┘ │ │
│  │                                                        │ │
│  │                      [Enable Maintenance Mode]         │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Maintenance History:                                        │
│  2026-04-12 03:00 → 03:45  (by admin@omnitun.io)  API only │
│  2026-03-08 02:00 → 04:00  (by admin@omnitun.io)  Full site│
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 10.5 日志级别

```
┌─────────────────────────────────────────────────────────────┐
│  Admin Console > System Config > Log Levels                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Dynamically adjust log levels per service (no restart):     │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │Service     │Current│Options      │Last Changed         ││
│  ├────────────────────────────────────────────────────────┤ │
│  │server/api  │info   │debug/info/warn/error│2026-05-20   ││
│  │server/auth │warn   │debug/info/warn/error│2026-05-15   ││
│  │relay/proxy │info   │debug/info/warn/error│2026-05-01   ││
│  │relay/tls   │error  │debug/info/warn/erro│2026-04-01   ││
│  │agent/conn  │debug  │debug/info/warn/erro│2026-05-21   ││
│  │agent/p2p   │info   │debug/info/warn/erro│2026-05-18   ││
│  │control/nats│info   │debug/info/warn/erro│2026-04-20   ││
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ⚠️ Setting to 'debug' increases log volume significantly.  │
│  Log levels automatically revert to 'info' after 24 hours.   │
│  This setting affects all nodes running the target service.  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 10.6 其他系统配置项

| 配置模块 | 配置项 | 类型 | 说明 |
|----------|--------|------|------|
| **全局** | `default_data_region` | string | 新组织默认数据区域 |
| **全局** | `session_ttl_hours` | int | 用户登录 Session 过期时间 |
| **全局** | `max_organizations_per_user` | int | 单个用户可以创建/加入的最大组织数 |
| **全局** | `require_email_verification` | bool | 是否强制邮箱验证后才可使用 |
| **安全** | `password_min_length` | int | 密码最小长度 |
| **安全** | `password_require_special` | bool | 密码是否要求特殊字符 |
| **安全** | `mfa_required_for_org_admins` | bool | 组织 Admin 是否强制 MFA |
| **隧道** | `default_tunnel_timeout_seconds` | int | 隧道空闲超时默认值 |
| **隧道** | `max_tunnel_connections_free` | int | Free 计划最大连接数 |
| **隧道** | `max_tunnel_bandwidth_mbps_free` | int | Free 计划最大带宽 |
| **计费** | `trial_days` | int | 免费试用天数 |
| **计费** | `grace_period_days` | int | 欠费宽限天数 |
| **计费** | `invoice_prefix` | string | 发票编号前缀 |

### 10.7 API 端点

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/admin/v1/system/rate-limits` | 获取所有速率限制配置 |
| `PUT` | `/api/admin/v1/system/rate-limits` | 更新速率限制配置 |
| `GET` | `/api/admin/v1/system/email-templates` | 获取邮件模板列表 |
| `GET` | `/api/admin/v1/system/email-templates/{template_key}` | 获取邮件模板详情 |
| `PUT` | `/api/admin/v1/system/email-templates/{template_key}` | 更新邮件模板 |
| `POST` | `/api/admin/v1/system/email-templates/{template_key}/test` | 发送测试邮件 |
| `POST` | `/api/admin/v1/system/email-templates/{template_key}/reset` | 恢复默认模板 |
| `GET` | `/api/admin/v1/system/maintenance` | 获取维护模式状态 |
| `PUT` | `/api/admin/v1/system/maintenance` | 启用/禁用维护模式 |
| `GET` | `/api/admin/v1/system/log-levels` | 获取所有服务的日志级别 |
| `PUT` | `/api/admin/v1/system/log-levels/{service}` | 更新服务日志级别 |
| `GET` | `/api/admin/v1/system/config` | 获取所有全局配置项 |
| `PUT` | `/api/admin/v1/system/config` | 批量更新全局配置项 |
| `GET` | `/api/admin/v1/system/config/{key}` | 获取单项配置 |

### 10.8 验收标准

- [ ] 速率限制变更后 5 秒内生效（无需重启服务）
- [ ] 邮件模板编辑支持实时 HTML 预览 + 发送测试邮件
- [ ] 邮件模板变量补全提示（输入 `{{.` 时自动弹出可用变量列表）
- [ ] 维护模式开启后：
  - API 模式：所有非白名单 IP 的 API 返回 503 + 自定义消息
  - Dashboard 模式：用户访问 Dashboard 时展示维护页面
  - Agent 模式：拒绝新的 Agent WebSocket 连接，已有连接保持
- [ ] 维护模式支持 IP/CIDR 白名单（白名单内 IP 不受影响）
- [ ] 日志级别变更后 10 秒内推送到所有运行中实例（通过 NATS 广播）
- [ ] Debug 日志级别在 24 小时后自动回退到 info（防止遗忘）
- [ ] 所有系统配置项变更记录完整审计日志
- [ ] 敏感配置变更（如 rate limits、maintenance mode）需要二次确认

---

## 十一、通用需求

### 11.1 全局搜索

管理后台顶部提供全局搜索栏，支持：

```
[🔍 Search anything...                                    ]

搜索范围：
  - Organizations (by name)
  - Users (by email / name)
  - Tunnels (by domain / slug)
  - Relay Nodes (by name / region)

搜索结果格式：
  ┌──────────────────────────────────────────┐
  │ Organizations (3)                        │
  │   Acme Corp          Enterprise   Active │
  │   AcmeDev            Pro          Active │
  │   AcmeTest           Free        Frozen  │
  │ ───────────────────────────────────────── │
  │ Users (2)                                │
  │   john@acmecorp.com   John Smith  Active │
  │   admin@acmecorp.com  Jane Doe    Active │
  │ ───────────────────────────────────────── │
  │ Tunnels (1)                              │
  │   api.acmecorp.com    tunnel_123  Active │
  │ ───────────────────────────────────────── │
  │ Relay Nodes (1)                          │
  │   us-east-relay-01    10.0.1.101   Up    │
  └──────────────────────────────────────────┘
```

### 11.2 审计日志

| 要求 | 说明 |
|------|------|
| **记录内容** | 操作人 + 操作时间 + 操作类型 + 目标对象 + 操作前值 + 操作后值 + IP + User-Agent |
| **不可篡改** | 审计日志写入后不可修改、不可删除（数据库层面限制 UPDATE/DELETE） |
| **保留策略** | 在线保留 12 个月，超过 12 个月的归档到对象存储（Parquet 格式，可查询） |
| **导出** | 支持按时间范围导出为 CSV/JSON |
| **操作类型** | 详见下文分类定义 |

**管理后台审计事件分类**：

| 类别 | 事件示例 |
|------|----------|
| `auth` | `admin.login`, `admin.logout`, `admin.failed_login`, `admin.impersonate` |
| `organization` | `org.view`, `org.freeze`, `org.unfreeze`, `org.change_plan`, `org.delete`, `org.note_add` |
| `user` | `user.view`, `user.reset_password`, `user.force_logout`, `user.disable`, `user.enable`, `user.impersonate` |
| `relay` | `relay.view`, `relay.register`, `relay.drain`, `relay.maintenance`, `relay.decommission` |
| `certificate` | `cert.view`, `cert.renew`, `cert.revoke` |
| `security` | `security.rule_create`, `security.report_action`, `security.blacklist_add`, `security.org_ban` |
| `feature_flag` | `flag.create`, `flag.toggle`, `flag.rollout_change`, `flag.archive` |
| `announcement` | `announcement.create`, `announcement.publish`, `announcement.archive` |
| `system` | `system.rate_limit_update`, `system.email_template_edit`, `system.maintenance_toggle`, `system.log_level_change` |

### 11.3 通知与告警渠道

| 场景 | 渠道 | 接收人 |
|------|------|--------|
| Relay 节点下线 | PagerDuty + Slack #infra | Infrastructure Admin |
| 证书即将到期 | Slack #infra<br>PagerDuty（7天内） | Infrastructure Admin |
| API 错误率 > 1% | PagerDuty + Slack #oncall | Full Admin |
| 安全事件（Critical） | PagerDuty + Slack #security | Security Admin |
| 滥用举报（New） | Slack #security | Security Admin |
| MRR 异常下降 > 20% | Slack #bizops | Full Admin |
| Unsafe action（删除组织等） | Slack #admin-alerts | Root Admin |

### 11.4 性能要求

| 页面 | 首次加载 | 后续加载 | 数据刷新 |
|------|----------|----------|----------|
| 仪表板 | < 3s | < 1.5s | 10-60s 按组件 |
| 组织列表 | < 2s | < 1s | 手动 |
| 组织详情 | < 2s | < 1s | 手动 |
| 用户列表 | < 2s | < 1s | 手动 |
| Relay 列表 | < 2s | < 1s | 10s 自动 |
| Relay 详情 | < 2s | < 1s | 手动 |
| 证书管理 | < 1.5s | < 1s | 手动 |
| 安全中心 | < 3s | < 1.5s | 手动 |
| Feature Flags | < 1s | < 0.5s | 手动 |
| 公告管理 | < 1s | < 0.5s | 手动 |
| 系统配置 | < 1s | < 0.5s | 手动 |

---

## 十二、API 认证与授权（Admin 层）

### 12.1 Admin API 认证架构

```
┌────────────────────────────────────────────────────┐
│               admin.omnitun.io                      │
│                                                     │
│  Client Browser                                     │
│       │                                             │
│       ▼                                             │
│  ┌─────────────┐     ┌───────────────────────┐      │
│  │ Admin SPA   │────►│ Admin API Gateway     │      │
│  │ (Next.js)   │     │ /api/admin/v1/*       │      │
│  └─────────────┘     │                        │      │
│                       │ 1. JWT Validate        │      │
│                       │ 2. Check Super Admin   │      │
│                       │ 3. Check Role Perm     │      │
│                       │ 4. Write Audit Log     │      │
│                       │ 5. Forward to Service  │      │
│                       └───────────────────────┘      │
│                                                     │
└────────────────────────────────────────────────────┘
```

### 12.2 Admin 角色权限矩阵

| 操作 | Root Admin | Full Admin | Read-Only | Security | Infra |
|------|:---:|:---:|:---:|:---:|:---:|
| 查看仪表板 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 查看组织列表/详情 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Freeze / Unfreeze 组织 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 变更组织计划 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 删除组织 | ✅ | ❌ | ❌ | ❌ | ❌ |
| 模拟登录 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 查看用户列表/详情 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 重置密码 / 强制登出 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 禁用/启用用户 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 管理 Relay 节点 | ✅ | ❌ | ❌ | ❌ | ✅ |
| 管理证书 | ✅ | ✅ | ❌ | ❌ | ✅ |
| 管理滥用规则 | ✅ | ❌ | ❌ | ✅ | ❌ |
| 处理举报 | ✅ | ✅ | ❌ | ✅ | ❌ |
| 管理 IP 黑名单 | ✅ | ❌ | ❌ | ✅ | ❌ |
| 查看安全事件 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Ban/Unban 组织 | ✅ | ❌ | ❌ | ✅ | ❌ |
| Feature Flag 管理 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 公告管理 | ✅ | ✅ | ❌ | ❌ | ❌ |
| 系统配置修改 | ✅ | ❌ | ❌ | ❌ | ❌ |

### 12.3 Admin JWT 附加 Claims

```json
{
  "sub": "usr_root_001",
  "email": "root@omnitun.io",
  "admin_role": "root_admin",
  "admin_level": 100,
  "permissions": ["org:delete", "system:config", "user:impersonate"],
  "iplocked": true,
  "mfa_verified": true,
  "exp": 1716300000,
  "iat": 1716256800
}
```

### 12.4 Admin API 请求/响应格式统一要求

- 所有 Admin API 端点必须以 `/api/admin/v1/` 为前缀
- 所有分页返回统一格式：`{ "data": [...], "pagination": { "page": 1, "page_size": 25, "total": 1247, "total_pages": 50 } }`
- 所有错误返回统一格式：`{ "error": { "code": "ORG_NOT_FOUND", "message": "Organization not found", "details": {...} } }`
- 支持字段过滤：`?fields=id,name,plan,status` 减少不必要的数据传输
- 支持 `?expand=plan_details,owner_user` 内联关联数据

---

## 十三、数据模型补充（管理后台专属表）

### 13.1 admin_audit_logs

```sql
CREATE TABLE admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL,
    admin_email TEXT NOT NULL,
    admin_role TEXT NOT NULL,
    action_category TEXT NOT NULL,   -- auth, organization, user, relay, etc.
    action_type TEXT NOT NULL,        -- org.freeze, user.impersonate, etc.
    target_type TEXT,                 -- organization / user / relay / certificate
    target_id UUID,
    target_name TEXT,
    before_value JSONB,               -- snapshot before action
    after_value JSONB,                -- snapshot after action
    ip_address INET,
    user_agent TEXT,
    request_id UUID,                  -- correlation ID
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_admin_audit_logs_admin_user ON admin_audit_logs(admin_user_id);
CREATE INDEX idx_admin_audit_logs_category ON admin_audit_logs(action_category);
CREATE INDEX idx_admin_audit_logs_target ON admin_audit_logs(target_type, target_id);
CREATE INDEX idx_admin_audit_logs_created ON admin_audit_logs(created_at DESC);
```

### 13.2 admin_feature_flags

```sql
CREATE TABLE admin_feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_key TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    flag_type TEXT NOT NULL DEFAULT 'boolean',  -- boolean / percentage / whitelist / plan_based
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    rollout_percentage INT DEFAULT 0 CHECK (rollout_percentage >= 0 AND rollout_percentage <= 100),
    whitelist_org_ids UUID[] DEFAULT '{}',
    target_plan_ids TEXT[] DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'off',          -- on / off / archived
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    rollout_history JSONB DEFAULT '[]'
);
```

### 13.3 admin_announcements

```sql
CREATE TABLE admin_announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    body_markdown TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'information', -- information / warning / critical
    target_audience TEXT NOT NULL DEFAULT 'all',   -- all / plans / organizations / regions
    target_plan_ids TEXT[] DEFAULT '{}',
    target_org_ids UUID[] DEFAULT '{}',
    target_regions TEXT[] DEFAULT '{}',
    display_locations TEXT[] DEFAULT '{dashboard_banner}',
    is_dismissible BOOLEAN NOT NULL DEFAULT true,
    publish_at TIMESTAMPTZ,
    expire_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'draft',          -- draft / scheduled / live / expired / archived
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 13.4 admin_security_rules

```sql
CREATE TABLE admin_security_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_name TEXT NOT NULL,
    description TEXT,
    condition_expr TEXT NOT NULL,        -- CEL / Rego expression
    action_type TEXT NOT NULL DEFAULT 'alert',  -- alert / flag / block
    severity TEXT NOT NULL DEFAULT 'warning',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    dry_run BOOLEAN NOT NULL DEFAULT false,
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 13.5 admin_ip_blacklist

```sql
CREATE TABLE admin_ip_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr CIDR NOT NULL,
    reason TEXT,
    source TEXT NOT NULL DEFAULT 'manual',  -- manual / auto_ai / auto_rule
    applies_to TEXT[] DEFAULT '{api_access,tunnel_ingress}',
    expires_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_admin_ip_blacklist_cidr ON admin_ip_blacklist USING GIST (cidr inet_ops);
```

### 13.6 admin_security_events

```sql
CREATE TABLE admin_security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    org_id UUID,
    user_id UUID,
    tunnel_id UUID,
    source_ip INET,
    detail JSONB NOT NULL,
    is_acknowledged BOOLEAN DEFAULT false,
    is_false_positive BOOLEAN DEFAULT false,
    acknowledged_by UUID,
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_admin_security_events_type ON admin_security_events(event_type);
CREATE INDEX idx_admin_security_events_org ON admin_security_events(org_id);
CREATE INDEX idx_admin_security_events_created ON admin_security_events(created_at DESC);
```

### 13.7 admin_abuse_reports

```sql
CREATE TABLE admin_abuse_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_user_id UUID,
    reporter_email TEXT NOT NULL,
    target_type TEXT NOT NULL,  -- tunnel / organization / user
    target_id UUID NOT NULL,
    report_type TEXT NOT NULL,  -- phishing / malware / spam / ddos / other
    description TEXT NOT NULL,
    evidence_urls TEXT[] DEFAULT '{}',
    priority TEXT NOT NULL DEFAULT 'medium',
    status TEXT NOT NULL DEFAULT 'open',  -- open / in_review / resolved / dismissed
    resolution_action TEXT,
    resolution_notes TEXT,
    resolved_by UUID,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 13.8 admin_organization_notes

```sql
CREATE TABLE admin_organization_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    admin_user_id UUID NOT NULL,
    note_markdown TEXT NOT NULL,
    is_pinned BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_admin_org_notes_org ON admin_organization_notes(org_id);
```

---

## 十四、实施建议

### 14.1 技术选型建议

| 层面 | 推荐方案 | 理由 |
|------|----------|------|
| Admin SPA | React + TypeScript（独立项目 `web/admin/`） | 不与用户端 SPA 混入，独立构建部署 |
| UI 组件库 | shadcn/ui + Tailwind CSS | 快速构建，风格统一，可定制 |
| 图表库 | Tremor 或 recharts | 支持实时更新 |
| 表格库 | TanStack Table v8 | 支持虚拟滚动、排序、筛选、导出 |
| Admin API | Go（独立 handler 包：`internal/adminapi/`） | 复用现有 `internal/auth/` 中间件和 `pkg/` 工具 |
| WebSocket | 复用 `internal/gateway/`，增加 admin channel | 用于实时推送告警、系统健康、Relay 状态 |
| 权限中间件 | `RequireAdminRole(role string)` | 基于 JWT Claims 中的 `admin_role` 字段 |
| 审计日志 | 异步写入（NATS → 消费者 → PG）+ 定期归档到 MinIO | 不阻塞主业务流程 |
| 搜索 | PostgreSQL Full Text Search (tsvector) + trigram (pg_trgm) | 避免引入 Elasticsearch 的复杂度 |

### 14.2 目录结构建议

```
internal/
├── adminapi/                # 管理后台 API Handler
│   ├── dashboard.go
│   ├── organizations.go
│   ├── users.go
│   ├── relay_nodes.go
│   ├── certificates.go
│   ├── security.go
│   ├── feature_flags.go
│   ├── announcements.go
│   └── system_config.go
├── adminmiddleware/         # 管理后台专用中间件
│   ├── superadmin.go        # Super Admin 角色验证
│   ├── adminaudit.go        # 审计日志记录
│   └── iplock.go            # IP 白名单检查
└── ...

web/
├── admin/                   # 管理后台 SPA（独立项目）
│   ├── src/
│   │   ├── pages/
│   │   │   ├── dashboard/
│   │   │   ├── organizations/
│   │   │   ├── users/
│   │   │   ├── relays/
│   │   │   ├── certificates/
│   │   │   ├── security/
│   │   │   ├── feature-flags/
│   │   │   ├── announcements/
│   │   │   └── system-config/
│   │   ├── components/
│   │   ├── hooks/
│   │   ├── lib/
│   │   └── ...
│   ├── package.json
│   └── ...
└── ...
```

### 14.3 数据库变更策略

- 所有管理后台专属表以 `admin_` 为前缀，与 2.0 的表命名空间隔离
- 新增 migration 文件：`migrations/000015_admin_console_tables.up.sql`
- 不修改任何 2.0 已有的表结构（避免回滚风险）
- `admin_audit_logs` 使用 TimescaleDB 或按月分区表（数据量大时）

### 14.4 安全加固清单

- [ ] `admin.omnitun.io` 的 DNS 不在公网解析（或仅限 VPN 内网解析）
- [ ] Admin API Gateway 额外施加 IP 白名单检查（Layer 3/4）
- [ ] Admin JWT 的 `iat`/`exp` 校验比用户 JWT 更严格（默认 4 小时过期）
- [ ] 所有 Admin API 端点包含 `X-Request-ID` 用于审计追踪
- [ ] 关键操作（删除组织、吊销证书、退役 Relay）需同级别双人确认（Four-Eyes Principle）
- [ ] Admin SPA 不向公网 CDN 部署，走内网静态资源服务

---

## 十五、版本记录

| 版本 | 日期 | 作者 | 变更 |
|------|------|------|------|
| v1.0 | 2026-05-21 | OmniTun Team | 管理后台完整需求初稿，覆盖全部 10 个功能模块 |
