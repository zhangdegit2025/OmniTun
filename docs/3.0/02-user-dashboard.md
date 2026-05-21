# OmniTun 3.0 — 用户端完整需求

<!--
  修订说明：
  2026-05-21:
  - 初始版本：在 2.0 基础上定义 Dashboard 3.0 的完整功能矩阵
  - 当前基线：12 页基础 CRUD + 6 项左侧导航 + 简单顶栏
  基准代码：web/src/App.tsx (8 routes), web/src/components/Layout.tsx (6 nav items),
           web/src/pages/Dashboard.tsx (4 stat cards + traffic chart + events list),
           web/src/pages/Tunnels.tsx (list + create + start/stop + delete + pagination)
-->

---

## 一、Onboarding 流程（首次使用体验）

### 1.1 触发条件

新注册用户在完成邮箱验证后，首次登录自动进入 Onboarding 向导。老用户（已有隧道或完成过 Onboarding）跳过该流程，直接进入 Dashboard。

| 用户状态 | 行为 |
|----------|------|
| 新注册，`onboarding_completed = false` | 自动进入 5 步向导 |
| 新注册，点击 "Skip tour" | 设置 `onboarding_completed = true`，进入空状态 Dashboard |
| 老用户登录 | 直接进入 Dashboard |
| 老用户手动访问 `/onboarding` | 可重新体验向导（标记为 "Review mode"） |

### 1.2 五步向导

每个步骤占用一个全屏卡片，顶部有 5 步进度条（StepIndicator 组件），支持上一步/下一步/跳过。

| 步骤 | 页面内容 | 核心交互 |
|------|---------|----------|
| **Step 1: Welcome** | 产品口号 + 价值主张（3 个亮点卡片：即时公网暴露 / 全协议穿透 / 私有 Mesh 组网） | "Get Started" 按钮 + "Skip tour" 链接 |
| **Step 2: Create Organization** | 组织名称输入框，slug 自动生成，可选头像上传 | 创建组织 → 下一步；如果已有组织（被邀请加入），显示 "You're already in `<org>`" → 跳过 |
| **Step 3: Install CLI** | 3 个平台的安装命令 Tab（macOS / Linux / Windows），每个 Tab 包含 `curl` / `brew` / `choco` / 直接下载按钮 | 点击 "Copy" 复制安装命令；"I've installed, verify" → 调用 `GET /v1/agents/status` 检查 CLI 是否已安装 |
| **Step 4: Create First Tunnel** | 简化的隧道创建表单（只有端口 + 协议），左侧 Live Preview 展示生成的 URL | 输入端口 → 点击 "Create & Start" → 左侧显示 `https://<slug>.omnitun.io → localhost:8080`；成功后有彩带动画 |
| **Step 5: Success** | 庆祝页面：隧道已激活的链接 + 3 个快捷入口（Dashboard / Tunnels / Docs） | "Go to Dashboard" 完成 Onboarding |

### 1.3 首次访问上下文帮助

用户首次进入每个页面（不受 Onboarding 影响），在页面顶部显示一条可关闭的提示条（Banner），内容是当前页面的功能简介。

| 页面 | 提示内容 |
|------|---------|
| Dashboard | "这里是您的隧道概览。活跃隧道数、流量趋势、最近事件一览无余。" |
| Tunnels | "管理您的所有隧道。点击隧道名称查看详情，或者创建新隧道开始穿透。" |
| Domains | "绑定您的自定义域名。添加后需完成 DNS 验证即可启用。" |
| Networks | "创建私有 Mesh 网络，连接多个内网节点。" |
| Billing | "查看您的用量和订阅计划。" |
| Settings | "管理组织、团队成员和 API Key。" |

提示条持久化到 localStorage，关闭后不再显示（key: `omnitun_banner_dismissed_<route>`）。

---

## 二、全局导航增强

### 2.1 左侧导航扩展

当前 6 项导航需扩展为 9 项：

| # | 导航项 | 图标 | 路由 | 说明 |
|---|--------|------|------|------|
| 1 | Dashboard | `LayoutDashboard` | `/` | 保持不变 |
| 2 | Tunnels | `Server` | `/tunnels` | 保持不变 |
| 3 | Domains | `Globe` | `/domains` | 保持不变 |
| 4 | Networks | `Share2` | `/networks` | 保持不变 |
| 5 | **Notifications** | `Bell` | `/notifications` | **新增**，未读数量 Badge（红色数字） |
| 6 | Billing | `CreditCard` | `/billing` | 保持位置 |
| 7 | Settings | `Settings` | `/settings` | 保持位置 |
| 8 | **API Docs** | `BookOpen` | `/docs/api` | **新增**，内嵌 Swagger UI |
| 9 | **Downloads** | `Download` | `/downloads` | **新增**，CLI 二进制下载 |

### 2.2 顶栏重新设计

当前顶栏仅显示用户名 + 语言切换 + 注销按钮。需完全重构：

```
┌─ [Logo] ── [面包屑] ────────────────────── [Cmd+K 搜索框] ── [🔔 通知铃] ── [🌙/☀️ 主题] ── [🌐 EN/中文] ── [👤 头像 ▼] ─┐
```

| 元素 | 组件 | 说明 |
|------|------|------|
| **全局搜索** | `CommandPalette` (cmdkey 或 ⌘+K) | 输入框占位文本 "Search tunnels, domains, settings..."；展开后为幕帘式搜索面板 |
| **通知铃铛** | `NotificationBell` | 显示未读数量 Badge（红色圆形，超过 99 显示 "99+"）；点击跳转 `/notifications` |
| **主题切换** | `ThemeToggle` | 太阳 ☀️ / 月亮 🌙 图标切换（详见第十二章深色模式） |
| **语言切换** | `LocaleSwitch` | 保持不变，从 Header 移至头像左侧 |
| **用户头像** | `UserDropdown` | 显示 Gravatar 或首字母头像；点击展开下拉菜单（见下文） |

### 2.3 用户头像下拉菜单

```
┌──────────────────────────┐
│  👤 username@email.com   │
│  ─────────────────────── │
│  👤 Profile              │ → /settings (或未来独立 Profile 页)
│  🔑 API Keys             │ → /settings?tab=apikeys
│  ─────────────────────── │
│  🌙 Dark Mode   [Toggle] │
│  ─────────────────────── │
│  🚪 Sign Out             │
└──────────────────────────┘
```

### 2.4 面包屑导航

所有二级及以上页面自动生成面包屑：

| 路由 | 面包屑 |
|------|--------|
| `/` | （不显示） |
| `/tunnels` | （不显示，顶级导航） |
| `/tunnels/:id` | Tunnels / My API Server |
| `/tunnels/:id/inspect` | Tunnels / My API Server / Request Inspector |
| `/domains` | （不显示） |
| `/networks` | （不显示） |
| `/networks/:id` | Networks / production-mesh |
| `/settings` | （不显示） |
| `/settings?tab=members` | Settings / Team Members |
| `/notifications` | （不显示） |
| `/billing` | （不显示） |
| `/docs/api` | API Docs |

实现方案：`useBreadcrumbs()` hook 从路由匹配中自动提取面包屑层级。

---

## 三、Dashboard 增强

### 3.1 当前基线

Dashboard 现有内容：
- 4 个统计卡片（活跃隧道 / 总流量 / 活跃连接 / 今日请求）
- TrafficAreaChart（24 小时流量趋势，mock 数据）
- 最近事件列表（Table，分页）

### 3.2 新增：快速操作区

位于统计卡片和流量图之间，一行 2~3 个快捷操作卡片：

```
┌───────────────────────────────────────────────────┐
│ ⚡ Quick Tunnel                                   │
│ [输入本地端口] [选择协议 ▼]     [Create & Start]  │
│ 示例输出: https://xxx.omnitun.io → localhost:8080 │
└───────────────────────────────────────────────────┘
```

- 端口输入框支持快捷键 `/` 聚焦
- 协议下拉：HTTP / TCP / gRPC / WebSocket
- 创建成功后，下方实时显示生成的 URL（可一键复制）
- 使用 `useMutation` + optimistic update

### 3.3 新增：隧道健康状态一览

位于流量图下方，最近 10 个隧道的状态指示器（紧凑行）：

```
┌──────────────────────────────────────────────────────────────┐
│ Tunnel Health                                                 │
│ ● my-api         Active     https://my-api.omnitun.io        │
│ ● dev-server     Active     https://dev.omnitun.io           │
│ ● staging-db     Stopped    tcp://staging-db.omnitun.io:5432 │
│ ⬤ redis-cache    Error      tcp://redis.omnitun.io:6379      │
└──────────────────────────────────────────────────────────────┘
```

- 绿色圆点 = Active（最近 1 分钟内有心跳）
- 黄色圆点 = Degraded（有心跳但延迟 > 500ms 或丢包 > 1%）
- 红色圆点 = Error/Disconnected（超过 1 分钟无心跳）
- 灰色圆点 = Stopped
- 每行右侧有跳转链接 `→` 进入隧道详情

### 3.4 新增：用量进度环

位于统计卡片区域，替换或添加一个环形图：

```
┌────────────────────┐
│ Bandwidth Usage    │
│                    │
│   ╭─────╮         │
│  ╱ 78%  ╲        │
│ │    ●    │       │
│  ╲       ╱        │
│   ╰─────╯         │
│                    │
│ 3.9 GB / 5 GB      │
│ Renews in 12 days  │
└────────────────────┘
```

使用 SVG `<circle>` + `stroke-dasharray` 实现，无需第三方图表库。
数据来源：`GET /v1/usage/bandwidth` 返回 `{used, limit, reset_date}`。

### 3.5 新增：最近通知

位于事件列表上方或右侧：

```
┌───────────────────────────────────────┐
│ Recent Notifications                  │
│ ● 🔴 tunnel-1 断线       2 min ago   │
│ ● ⚠️  SSL 证书即将过期    1 hour ago  │
│ ● ℹ️  v2.1.0 已发布       3 hours ago │
│ ● ✅ tunnel-2 已恢复      5 hours ago │
│             [View All →]              │
└───────────────────────────────────────┘
```

显示最近 5 条未读通知，点击通知跳转到对应隧道/页面，"View All" 进入通知中心。

---

## 四、隧道管理增强

### 4.1 当前基线

隧道页面 (`web/src/pages/Tunnels.tsx`) 现有功能：
- 列表视图（名称 / 协议 / 状态 / 域名 / 流量 / 操作）
- 创建隧道（名称 / 协议 / 域名 / 本地端口 / 远程端口）
- 启动 / 停止 / 删除隧道
- 复制域名到剪贴板
- 分页（每页 10 条）
- 进入详情页链接

### 4.2 新增：隧道模板

位于创建对话框顶部添加 Tab 切换：

```
[ Create from scratch ] [ Templates ]
```

模板预设：

| 模板名 | 协议 | 默认配置 | 图标 |
|--------|------|---------|------|
| HTTP API Server | http | port: 8080, TLS: edge, compression: gzip | `Globe` |
| gRPC Service | grpc | port: 50051, TLS: edge, http2: true | `Network` |
| WebSocket Server | http | port: 3000, websocket_upgrade: true | `ArrowLeftRight` |
| TCP Database | tcp | port: 5432, tls: passthrough | `Database` |
| TCP SSH | tcp | port: 22, tls: passthrough | `Terminal` |
| Custom | * | 空白表单 | `Plus` |

选择模板后自动填充表单字段（不含名称），用户只需填名称 + 端口即可创建。

### 4.3 新增：批量操作

在隧道列表中添加复选框列（每行最左侧）和顶部批量操作栏：

```
┌─ [☐ 全选] [▶ Start] [■ Stop] [🗑 Delete] ─ [已选 3 项] ─────────┐
│ ☐ │ Name      │ Protocol │ Status │ Domain              │ Actions │
├───┼───────────┼──────────┼────────┼─────────────────────┼─────────│
│ ☑ │ my-api    │ HTTP     │ Active │ https://x.omnitun.io │ ...     │
│ ☑ │ dev-svr   │ HTTP     │ Active │ https://y.omnitun.io │ ...     │
│ ☐ │ redis     │ TCP      │ Error  │ —                   │ ...     │
│ ☑ │ staging   │ gRPC     │ Stopped│ —                   │ ...     │
└────────────────────────────────────────────────────────────────┘
```

批量操作 API：
- `POST /v1/tunnels/batch/start` `{ tunnel_ids: string[] }`
- `POST /v1/tunnels/batch/stop` `{ tunnel_ids: string[] }`
- `POST /v1/tunnels/batch/delete` `{ tunnel_ids: string[] }`

### 4.4 新增：隧道克隆

在每行操作菜单（改为 DropdownMenu 三点菜单）中添加 "Clone" 选项：

```
[▶ Start/Stop] [📋 Clone] [🔗 Details] [🗑 Delete]
```

克隆行为：打开创建对话框，预填原隧道的所有字段（协议 / 端口 / 域名前缀 + "-copy"），用户修改后保存。

### 4.5 新增：隧道标签

数据模型扩展：

```sql
ALTER TABLE tunnels ADD COLUMN tags text[] DEFAULT '{}';
```

UI 展示：
- 列表中在每个隧道名称下方显示标签（彩色 Chip）
- 列表顶部添加标签过滤栏：点击标签 Chip 过滤，再次点击取消过滤
- 创建/编辑隧道时，添加 Tags 输入框（支持逗号分隔或回车添加，带自动补全）

标签自动补全来源：当前工作区下所有已使用的标签。

### 4.6 新增：隧道注解

数据模型扩展：

```sql
ALTER TABLE tunnels ADD COLUMN notes text DEFAULT '';
```

UI 展示：
- 详情页内嵌 Markdown 编辑器（使用轻量库如 `@uiw/react-md-editor`）
- 列表中显示注解缩略（前 80 个字符 + "...")
- 注解支持 Markdown 格式（纯文本存储，前端渲染）

### 4.7 新增：流量告警

数据模型扩展（新表）：

```sql
CREATE TABLE tunnel_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tunnel_id UUID NOT NULL REFERENCES tunnels(id) ON DELETE CASCADE,
    metric VARCHAR NOT NULL,           -- 'traffic_in' | 'traffic_out' | 'connections'
    threshold_type VARCHAR NOT NULL,   -- 'absolute' | 'percentage'
    threshold_value BIGINT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
```

UI 展示：
- 隧道详情页 > "Alerting" Tab
- 创建告警规则表单：选择指标 / 阈值 / 单位
- 触发时在通知中心生成通知 + 可选邮件通知

---

## 五、实时监控面板（新页面）

### 5.1 路由

`/tunnels/:id/inspect` — 每个隧道的独立检查器
`/inspect` — （未来）聚合所有隧道流量

### 5.2 实时流量瀑布 (Live Traffic Stream)

类似 Wireshark 的实时请求列表，通过 WebSocket 推送：

```
┌──────────────────────────────────────────────────────────────┐
│ ⏸ Pause  │  🔍 Filter: [POST /api/users      ]  │ Clear    │
├────┬──────────────────┬─────────┬──────┬──────┬─────────────┤
│ #  │ Time             │ Method  │ Path │ Status │ Duration   │
├────┼──────────────────┼─────────┼──────┼──────┼─────────────┤
│ 42 │ 11:05:32.145     │ POST    │ /api │ 201  │ 45ms        │
│ 41 │ 11:05:32.103     │ GET     │ /api │ 200  │ 12ms        │
│ 40 │ 11:05:31.987     │ GET     │ /    │ 304  │ 8ms         │
│ 39 │ 11:05:31.456     │ POST    │ /api │ 422  │ 23ms        │
│ ...│ ...              │ ...     │ ...  │ ...  │ ...         │
└────┴──────────────────┴─────────┴──────┴──────┴─────────────┘
```

- 支持暂停/恢复实时流（暂停后仍缓存数据，恢复后一次性补全）
- 过滤器：按 Method / Path（含正则）/ Status Code / Duration 范围
- 新请求高亮动画（黄色背景闪烁 500ms）
- 每行颜色编码： 2xx 绿色 / 3xx 蓝色 / 4xx 黄色 / 5xx 红色
- 连接状态指示器（WebSocket connected ⬤ / reconnecting ⬤ / disconnected ⬤）

### 5.3 请求检查器 (Request Inspector)

点击流量瀑布中的某条请求，右侧（或底部）展开详情面板：

```
┌─ Request Inspector ──────────────────────────┐
│ ── Request ──                                │
│ Method:   POST                               │
│ Path:     /api/users                         │
│ Host:     my-api.omnitun.io                  │
│ ── Headers ──                                │
│ Content-Type:     application/json           │
│ Authorization:    Bearer eyJ***...           │
│ User-Agent:       curl/8.0.0                 │
│ X-Forwarded-For:  203.0.113.42               │
│ ── Body ──                                   │
│ {                                            │
│   "username": "alice",                       │
│   "email": "alice@example.com"               │
│ }                                            │
│ ── Response ──                               │
│ Status:   201 Created                        │
│ Duration: 45ms                               │
│ ── Headers ──                                │
│ Content-Type:   application/json             │
│ ── Body ──                                   │
│ {                                            │
│   "id": "usr_abc123",                        │
│   "username": "alice"                        │
│ }                                            │
│ [📋 Copy Request] [📋 Copy Response] [🔁 Replay] │
└──────────────────────────────────────────────┘
```

对标 ngrok 的 Request Inspector 体验。

### 5.4 请求重放 (Replay)

- 点击 "Replay" 打开请求编辑对话框
- 可修改 Method / URL / Headers (KV 编辑器) / Body
- 点击 "Send" → 通过 WebSocket 发送重放请求 → 在流量瀑布中显示新条目
- 支持多轮修改（修改 → 发送 → 再修改 → 再发送）

---

## 六、通知中心（新页面）

### 6.1 路由

`/notifications`

### 6.2 页面布局

```
┌─ Notifications ─────────────────────────────────────────────┐
│ [All (12)] [🔴 Tunnels (3)] [💰 Billing (1)] [📢 System (2)]│
├─────────────────────────────────────────────────────────────┤
│ 🔴 [Tunnel Alert]  my-api 断线                   2 min ago  │
│    Tunnel "my-api" stopped responding. Status: Disconnected  │
│    [View Tunnel →]                     [✕ Dismiss]          │
├─────────────────────────────────────────────────────────────┤
│ ⚠️ [Cert Expiring]  example.com certificate                 │
│    will expire in 7 days. Renew now to avoid interruption.   │
│    [Manage Domain →]                   [✕ Dismiss]          │
├─────────────────────────────────────────────────────────────┤
│ 💰 [Quota Warning]  Bandwidth usage at 85%       1 hour ago  │
│    You've used 4.2 GB of your 5 GB monthly limit.            │
│    [Upgrade Plan →]                     [✕ Dismiss]          │
├─────────────────────────────────────────────────────────────┤
│ ...                                                          │
├─────────────────────────────────────────────────────────────┤
│ [Load More]                              [Mark All as Read]  │
└─────────────────────────────────────────────────────────────┘
```

### 6.3 通知类型

| 类别 | 通知类型 | 触发条件 | 默认通知渠道 |
|------|---------|---------|-------------|
| **Tunnel** | `tunnel.disconnected` | 隧道心跳超时（> 30s） | 站内 + 邮件 |
| | `tunnel.reconnected` | 隧道恢复心跳 | 站内 |
| | `tunnel.error` | 隧道出现 error 状态 | 站内 + 邮件 |
| | `tunnel.deleted` | 隧道被删除（非本人操作） | 站内 + 邮件 |
| **Cert** | `cert.expiring_30d` | SSL 证书 30 天内过期 | 站内 |
| | `cert.expiring_7d` | SSL 证书 7 天内过期 | 站内 + 邮件 |
| | `cert.expired` | SSL 证书已过期 | 站内 + 邮件 |
| **Billing** | `quota.bandwidth_80` | 带宽使用达 80% | 站内 |
| | `quota.bandwidth_100` | 带宽使用达 100% | 站内 + 邮件 |
| | `billing.payment_failed` | 付款失败 | 站内 + 邮件 |
| | `billing.invoice_ready` | 新发票生成 | 站内 |
| **Org** | `org.member_joined` | 新成员加入组织 | 站内 |
| | `org.member_removed` | 成员被移除 | 站内 |
| | `org.role_changed` | 成员角色变更 | 站内 |
| **System** | `system.maintenance` | 计划维护公告 | 站内 + 邮件 |
| | `system.release` | 新版本发布 | 站内 |

### 6.4 通知偏好

`/settings?tab=notifications`（在 Settings 页面新增 Tab）：

| 通知类型 | 站内推送 | 邮件通知 |
|---------|---------|---------|
| Tunnel 断线/恢复 | ✓（不可关闭） | [Toggle] |
| SSL 证书到期 | ✓（不可关闭） | [Toggle] |
| 用量告警 | ✓（不可关闭） | [Toggle] |
| 新成员加入 | [Toggle] | [Toggle] |
| 系统公告 | ✓（不可关闭） | [Toggle] |

关键通知（Tunnel 断线、证书到期、用量 100%、系统维护）站内推送不可关闭，确保用户不会遗漏。

---

## 七、Webhook 配置（新页面）

### 7.1 路由

`/settings?tab=webhooks`

### 7.2 创建 Webhook

```
┌─ Create Webhook ─────────────────────────────┐
│ Name:         [My Slack Notifier         ]    │
│ URL:          [https://hooks.slack.com/...]   │
│ Events:       ☑ tunnel.started               │
│               ☑ tunnel.stopped               │
│               ☑ tunnel.error                 │
│               ☐ cert.expiring                │
│               ☐ quota.warning                │
│               ☐ org.member_joined            │
│ ──────────────────────────────────────────── │
│ Secret:       [whsec_xxxxxxxxxxxxxxxxxxxx]   │
│               [🔄 Regenerate]                 │
│                                               │
│ [Test Send]                    [Save Webhook] │
└───────────────────────────────────────────────┘
```

### 7.3 Webhook 签名

使用 HMAC-SHA256 对 payload 进行签名：

```
X-OmniTun-Signature: sha256=<hex-encoded-hmac>
X-OmniTun-Event: tunnel.started
X-OmniTun-Delivery: <unique-uuid>
```

接收方验证逻辑 (伪代码)：

```python
import hmac
import hashlib

def verify_signature(payload: bytes, signature: str, secret: str) -> bool:
    expected = hmac.new(secret.encode(), payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)
```

### 7.4 事件 Payload 格式

```json
{
  "id": "evt_abc123",
  "type": "tunnel.started",
  "created_at": "2026-05-21T03:10:56Z",
  "data": {
    "tunnel_id": "tun_xyz789",
    "tunnel_name": "my-api",
    "tunnel_url": "https://my-api.omnitun.io",
    "org_id": "org_def456"
  },
  "delivery_id": "dlv_001"
}
```

### 7.5 Webhook 日志

每个 Webhook 展示最近 100 次投递记录：

```
┌─ Delivery Log — "My Slack Notifier" ────────────────────┐
│ Time               │ Status │ Duration │ Retry │ Event    │
├────────────────────┼────────┼──────────┼───────┼──────────│
│ 2026-05-21 11:10   │ ✓ 200  │ 230ms    │ 0     │ tunnel.. │
│ 2026-05-21 11:05   │ ✗ 503  │ 5.1s     │ 2     │ tunnel.. │
│ 2026-05-21 11:00   │ ✓ 200  │ 180ms    │ 0     │ cert...  │
└──────────────────────────────────────────────────────────┘
```

- 失败自动重试 3 次（指数退避：1s → 5s → 25s）
- 全部失败后标记为 `permanently_failed`，生成通知
- 每行可展开查看完整 Request/Response Headers + Body

### 7.6 测试发送

"Test Send" 发送一个示例 payload 到目标 URL，展示响应状态码 + Body，用于验证接收端配置。

---

## 八、团队协作增强

### 8.1 当前基线

成员管理位于 Settings 页面，当前有基础的邀请/角色功能。
路由：`/settings?tab=members`

### 8.2 新增：邀请链接

```
┌─ Invite Members ─────────────────────────────┐
│ Email:  [alice@example.com            ]       │
│ Role:   [Editor ▼]                           │
│            [Send Invitation]                  │
│                                               │
│ ─── OR ───                                   │
│                                               │
│ Invite Link:                                  │
│ https://omnitun.io/join/org_xxx/inv_yyy       │
│ [📋 Copy Link]                                │
│                                               │
│ Link Settings:                                │
│ ☐ Set expiration: [24 hours ▼]               │
│ ☐ Limit uses: [10        ] uses               │
│                                               │
│ [Generate New Link]                           │
│                                               │
│ Active Links:                                 │
│ ● inv_yyy  expires in 23h  · 3/10 used       │
│ ● inv_zzz  no expiration   · 5/unlimited      │
└───────────────────────────────────────────────┘
```

### 8.3 新增：角色矩阵

在邀请对话框中内嵌权限对照表：

| 权限 | Owner | Admin | Editor | Viewer |
|------|-------|-------|--------|--------|
| 管理支付 | ✓ | — | — | — |
| 删除组织 | ✓ | — | — | — |
| 管理成员 | ✓ | ✓ | — | — |
| 管理 API Key | ✓ | ✓ | — | — |
| 创建/删除隧道 | ✓ | ✓ | ✓ | — |
| 启动/停止隧道 | ✓ | ✓ | ✓ | — |
| 编辑隧道配置 | ✓ | ✓ | ✓ | — |
| 查看隧道详情 | ✓ | ✓ | ✓ | ✓ |
| 查看统计面板 | ✓ | ✓ | ✓ | ✓ |
| 查看流量日志 | ✓ | ✓ | ✓ | ✓ |

### 8.4 新增：活动日志

`/settings?tab=activity`

```
┌─ Activity Log ─────────────────────────────────────────────┐
│ 🔍 Filter: [All ▼] [member ▼] [tunnel ▼] [Last 7 days ▼]  │
├────────┬─────────┬─────────────────────────────┬────────────┤
│ Time   │ User    │ Action                      │ Resource    │
├────────┼─────────┼─────────────────────────────┼────────────┤
│ 11:05  │ bob     │ Stopped tunnel              │ my-api      │
│ 10:45  │ alice   │ Invited carol@example.com   │ (Editor)    │
│ 10:30  │ bob     │ Deleted tunnel              │ old-dev     │
│ 09:15  │ admin   │ Changed plan to Pro         │ org         │
│ 08:00  │ carol   │ Joined organization         │ —           │
└────────┴─────────┴─────────────────────────────┴────────────┘
```

数据来源：`GET /v1/org/activity?filter=tunnel&range=7d`

### 8.5 新增：会话管理

`/settings?tab=sessions`

```
┌─ Active Sessions ───────────────────────────────────┐
│ ● Current Session                                   │
│   Chrome / Windows  ·  192.168.1.100                │
│   Last active: now                                  │
│                                                     │
│ ● Safari / macOS  ·  10.0.0.15                      │
│   Last active: 2 hours ago          [Revoke]        │
│                                                     │
│ ● omnitun CLI v2.1.0  ·  198.51.100.42              │
│   Last active: 5 min ago             [Revoke]       │
└─────────────────────────────────────────────────────┘
```

- 展示所有活跃会话（浏览器 + CLI + API Key）
- 每个会话显示 User-Agent / IP / 最后活跃时间
- 可撤销非当前会话

---

## 九、API 文档内嵌（新页面）

### 9.1 路由

`/docs/api`

### 9.2 实现方案

嵌入 Swagger UI（`swagger-ui-dist` + React Wrapper），加载 OpenAPI 3.1 规范文件。

```tsx
// 使用 swagger-ui-react 或自建 wrapper
import SwaggerUI from 'swagger-ui-react'
import 'swagger-ui-react/swagger-ui.css'
```

API 规范来源：`GET /v1/openapi.json` — 由后端 API Gateway 动态生成。

### 9.3 自动认证

登录用户在 Dashboard 中查看 API 文档时，Swagger UI 的 `Authorize` 对话框自动填入用户的默认 API Key：

```tsx
// 通过 SwaggerUI plugin 注入 API Key
requestInterceptor: (req) => {
  req.headers.Authorization = `Bearer ${apiKey}`
  return req
}
```

### 9.4 代码示例

每个 API Endpoint 文档下方展示代码片段 Tab：

```
[ cURL ] [ Go ] [ Python ] [ JavaScript ]

curl -X POST https://api.omnitun.io/v1/tunnels \
  -H "Authorization: Bearer ot_sk_xxx" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-api","protocol":"http","local_port":8080}'
```

代码片段模板存储在 OpenAPI 规范的 `x-code-samples` 扩展字段中。

### 9.5 "Try It" 功能

Swagger UI 原生的 "Try it out" 按钮 → "Execute" 发起真实 API 请求，展示实际响应。

---

## 十、CLI 下载中心（新页面）

### 10.1 路由

`/downloads`

### 10.2 多平台下载矩阵

```
┌─ Download OmniTun CLI ──────────────────────────────────────────────────┐
│                                                                          │
│  Latest Version: v2.1.0 (2026-05-20)  [Changelog →]                     │
│                                                                          │
│  ┌──────────┬──────────────┬──────────────┬──────────────┐              │
│  │          │   macOS      │   Linux      │   Windows    │              │
│  ├──────────┼──────────────┼──────────────┼──────────────┤              │
│  │  amd64   │  [Download]  │  [Download]  │  [Download]  │              │
│  │  arm64   │  [Download]  │  [Download]  │  [Download]  │              │
│  └──────────┴──────────────┴──────────────┴──────────────┘              │
│                                                                          │
│  Checksums: [SHA256SUMS]  [SHA256SUMS.sig]                              │
└──────────────────────────────────────────────────────────────────────────┘
```

### 10.3 一键安装

```
┌──────────────────────────────────────────────────────┐
│ Quick Install                                        │
│ ┌──────────────────────────────────────────────────┐ │
│ │ $ curl -fsSL https://omnitun.io/install.sh | bash│ │
│ └──────────────────────────────────────────────────┘ │
│                                        [📋 Copy]    │
│                                                      │
│ macOS (Homebrew):                                    │
│ ┌──────────────────────────────────────────────────┐ │
│ │ $ brew install omnitun/tap/omnitun               │ │
│ └──────────────────────────────────────────────────┘ │
│                                        [📋 Copy]    │
│                                                      │
│ Windows (Chocolatey):                                │
│ ┌──────────────────────────────────────────────────┐ │
│ │ > choco install omnitun                          │ │
│ └──────────────────────────────────────────────────┘ │
│                                        [📋 Copy]    │
│                                                      │
│ Windows (winget):                                    │
│ ┌──────────────────────────────────────────────────┐ │
│ │ > winget install OmniTun.OmniTun                 │ │
│ └──────────────────────────────────────────────────┘ │
│                                        [📋 Copy]    │
└──────────────────────────────────────────────────────┘
```

### 10.4 版本历史

页面下半部分展示版本列表（可从 `GET /v1/releases` 获取）：

```
┌─ Version History ──────────────────────────────────────────────────────┐
│ v2.1.0              2026-05-20         [Download]  [Changelog]        │
│ v2.0.3              2026-05-10         [Download]  [Changelog]        │
│ v2.0.2              2026-04-28         [Download]  [Changelog]        │
│ v2.0.1              2026-04-15         [Download]  [Changelog]        │
│ ...                                                                    │
└────────────────────────────────────────────────────────────────────────┘
```

### 10.5 快速开始示例

页面顶部嵌入一个简单的动图/视频展示快速使用流程：

```
┌─ Quick Start ──────────────────────────────────────┐
│                                                     │
│   [终端 GIF: omnitun http 8080 → URL 生成 → 浏览器  │
│    访问 → 返回 Hello World]                         │
│                                                     │
│   1. Install CLI: curl -fsSL https://...            │
│   2. Login:     omnitun login                       │
│   3. Expose:    omnitun http 8080                   │
│   4. Share:     https://xxx.omnitun.io              │
│                                                     │
│   [Get Started]                                     │
└─────────────────────────────────────────────────────┘
```

---

## 十一、全局搜索 (Cmd+K)

### 11.1 触发方式

- 键盘快捷键：`Ctrl+K` (Windows/Linux) / `⌘+K` (macOS)
- 点击顶栏搜索输入框
- 首次登录弹窗提示快捷键

### 11.2 搜索面板

```
┌────────────────────────────────────────────────────┐
│ 🔍 Search tunnels, domains, settings...            │
│ ─────────────────────────────────────────────────  │
│ Tunnels                                            │
│ ├─ my-api-server          https://x.omnitun.io     │
│ ├─ dev-frontend           https://y.omnitun.io     │
│ └─ staging-database       tcp://z.omnitun.io:5432  │
│                                                     │
│ Domains                                            │
│ ├─ api.example.com        Verified                 │
│ └─ staging.example.com    Pending DNS              │
│                                                     │
│ Settings                                           │
│ ├─ API Keys                                        │
│ ├─ Team Members                                    │
│ ├─ Webhooks                                        │
│ └─ Notification Preferences                        │
│                                                     │
│ Documentation                                      │
│ └─ "How to create a TCP tunnel"                    │
│                                                     │
│ [esc] to close  [↑↓] to navigate  [↵] to select    │
└────────────────────────────────────────────────────┘
```

### 11.3 搜索数据源

| 类别 | 数据来源 | 搜索字段 |
|------|---------|---------|
| Tunnels | `GET /v1/tunnels` (缓存) | name, domain, protocol |
| Domains | `GET /v1/domains` (缓存) | domain, status |
| Settings | 前端路由表 | 页面标题 (i18n) |
| Docs | OpenAPI spec (缓存) | path, summary, description |

### 11.4 实现

使用 `cmdk` (⌘K) 组件库：

```tsx
import { Command } from 'cmdk'

<Command.Dialog>
  <Command.Input />
  <Command.List>
    <Command.Group heading="Tunnels">
      <Command.Item onSelect={() => navigate('/tunnels/:id')}>...</Command.Item>
    </Command.Group>
  </Command.List>
</Command.Dialog>
```

---

## 十二、深色模式

### 12.1 实现方案

使用 Tailwind CSS `dark:` variant + `class` strategy：

```js
// tailwind.config.js
module.exports = {
  darkMode: 'class',
  // ...
}
```

在 `<html>` 标签上添加/移除 `class="dark"` 控制主题。

### 12.2 主题切换按钮

位于顶栏，太阳 ☀️ / 月亮 🌙 图标：

```tsx
function ThemeToggle() {
  const [theme, setTheme] = useLocalStorage('omnitun-theme', 'system')

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
  }, [theme])

  const cycle = () => setTheme(t => t === 'dark' ? 'light' : t === 'light' ? 'system' : 'dark')

  return (
    <Button variant="ghost" size="sm" onClick={cycle}>
      {theme === 'dark' ? <Moon /> : theme === 'light' ? <Sun /> : <Monitor />}
    </Button>
  )
}
```

### 12.3 三种模式

| 模式 | 图标 | 行为 |
|------|------|------|
| Light | ☀️ Sun | 始终浅色 |
| Dark | 🌙 Moon | 始终深色 |
| System | 🖥️ Monitor | 跟随 `prefers-color-scheme`，自动切换 |

点击按钮在三种模式间循环切换。

### 12.4 颜色系统

深色模式下的颜色 Token（需要在 globals.css 中覆盖）：

| Token | Light | Dark |
|-------|-------|------|
| `--background` | `#ffffff` | `#0f172a` (slate-900) |
| `--foreground` | `#0f172a` | `#f8fafc` (slate-50) |
| `--card` | `#ffffff` | `#1e293b` (slate-800) |
| `--border` | `#e2e8f0` | `#334155` (slate-700) |
| `--muted-foreground` | `#64748b` | `#94a3b8` (slate-400) |
| `--primary` | `#6366f1` (indigo-500) | `#818cf8` (indigo-400) |
| `--destructive` | `#ef4444` (red-500) | `#f87171` (red-400) |
| `--success` | `#22c55e` (green-500) | `#4ade80` (green-400) |
| `--warning` | `#f59e0b` (amber-500) | `#fbbf24` (amber-400) |

### 12.5 会话级切换

切换主题时立即生效，无需刷新页面。通过 CSS 变量 + Tailwind `dark:` 前缀实现所有组件的主题适配。

---

## 附录 A：页面路由总览（3.0）

| 路由 | 页面 | 状态 |
|------|------|------|
| `/` | Dashboard（增强版） | 存量增强 |
| `/onboarding` | Onboarding 向导 | 新增 |
| `/tunnels` | 隧道列表（增强版） | 存量增强 |
| `/tunnels/:id` | 隧道详情（增强版） | 存量增强 |
| `/tunnels/:id/inspect` | 请求检查器 | 新增 |
| `/domains` | 域名管理 | 存量保持 |
| `/networks` | Mesh 网络列表 | 存量保持 |
| `/networks/:id` | 网络详情 | 存量保持 |
| `/notifications` | 通知中心 | 新增 |
| `/billing` | 计费页面 | 存量保持 |
| `/settings` | 设置（增强版：增加 Activity/Sessions/Webhooks/Notifications Tab） | 存量增强 |
| `/settings?tab=webhooks` | Webhook 配置 | 新增 |
| `/settings?tab=activity` | 活动日志 | 新增 |
| `/settings?tab=sessions` | 会话管理 | 新增 |
| `/settings?tab=notifications` | 通知偏好 | 新增 |
| `/docs/api` | API 文档 | 新增 |
| `/downloads` | CLI 下载中心 | 新增 |

## 附录 B：新增 API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/v1/usage/bandwidth` | 带宽用量（进度环） |
| `GET` | `/v1/tunnels/health` | 隧道健康状态一览 |
| `POST` | `/v1/tunnels/batch/start` | 批量启动 |
| `POST` | `/v1/tunnels/batch/stop` | 批量停止 |
| `POST` | `/v1/tunnels/batch/delete` | 批量删除 |
| `POST` | `/v1/tunnels/:id/clone` | 隧道克隆 |
| `GET` | `/v1/tunnels/:id/alerts` | 隧道告警规则 |
| `POST` | `/v1/tunnels/:id/alerts` | 创建告警规则 |
| `PUT` | `/v1/tunnels/:id/alerts/:alertId` | 更新告警规则 |
| `DELETE` | `/v1/tunnels/:id/alerts/:alertId` | 删除告警规则 |
| `WS` | `/v1/tunnels/:id/inspect` | 实时流量流 |
| `GET` | `/v1/notifications` | 通知列表 |
| `PUT` | `/v1/notifications/:id/read` | 标记已读 |
| `PUT` | `/v1/notifications/read-all` | 全部已读 |
| `GET` | `/v1/notifications/preferences` | 通知偏好 |
| `PUT` | `/v1/notifications/preferences` | 更新通知偏好 |
| `GET` | `/v1/webhooks` | Webhook 列表 |
| `POST` | `/v1/webhooks` | 创建 Webhook |
| `PUT` | `/v1/webhooks/:id` | 更新 Webhook |
| `DELETE` | `/v1/webhooks/:id` | 删除 Webhook |
| `POST` | `/v1/webhooks/:id/test` | 测试发送 |
| `GET` | `/v1/webhooks/:id/deliveries` | 投递日志 |
| `POST` | `/v1/org/invitations` | 创建邀请 |
| `GET` | `/v1/org/invitations` | 邀请列表 |
| `DELETE` | `/v1/org/invitations/:id` | 撤销邀请 |
| `GET` | `/v1/org/activity` | 活动日志 |
| `GET` | `/v1/sessions` | 活跃会话列表 |
| `DELETE` | `/v1/sessions/:id` | 撤销会话 |
| `GET` | `/v1/releases` | CLI 版本列表 |
| `GET` | `/v1/openapi.json` | API 规范文件 |
