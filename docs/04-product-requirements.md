# OmniTun — 产品需求文档

<!-- 修订说明：
  2026-05-20:
  - 问题 A: 修正 Agent 内存规格（原 K8s 配置与 PRD 内存占用概念混淆）
  - 问题 B: 新增 3.6 Phase 1 MVP 安全最低要求章节
  - 问题 C: 新增 C.5 平台级角色（Super Admin）
-->

## 一、产品边界与范围

### 1.1 产品形态矩阵

| 组成部分 | 描述 | 交付形式 |
|----------|------|----------|
| **OmniTun Cloud** | SaaS 多租户控制面 + 全球 Relay 网络 | 托管服务 |
| **OmniTun Agent** | 内网客户端守护进程 | CLI 二进制 + Docker 镜像 |
| **OmniTun Relay** | 数据面中继节点 | 自建基础设施 |
| **OmniTun Dashboard** | Web 管理控制台 | SPA + API |
| **OmniTun API** | 所有功能的 REST + WebSocket API | OpenAPI 3.1 |
| **OmniTun SDK** | 多语言 SDK（Go/JS/Python） | 包管理器分发 |
| **OmniTun Private** | 企业私有部署方案 | Helm Chart |

### 1.2 长期不做清单

- 不做域名注册（对接 DNS 提供商 API）
- 不做 CDN 缓存（对接第三方 CDN）
- 不做 IaaS 层面的 Load Balancer
- 不做通用 VPN 替代品（IPSec/WireGuard 的完整实现留给 Tailscale）
- 不做邮件/IM 服务

---

## 二、功能需求分解

### 模块 A：隧道系统（Tunnel System）

**A.1 隧道生命周期**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| TN-001 | 创建隧道 | 用户通过 CLI/API/Dashboard 创建隧道，指定协议、本地端口、选项 | P0 |
| TN-002 | 启动隧道 | Agent 连接控制面，建立到 Relay 的反向通道，隧道状态变更为 Active | P0 |
| TN-003 | 停止隧道 | 优雅关闭连接，释放资源 | P0 |
| TN-004 | 暂停/恢复隧道 | 临时冻结隧道而不删除配置 | P2 |
| TN-005 | 删除隧道 | 清理所有关联资源（域名、证书、访问策略） | P0 |
| TN-006 | 隧道分组 | 将隧道归类到 Workspace 下的 Group | P1 |
| TN-007 | 隧道克隆 | 一键复制隧道配置 | P2 |

**A.2 协议支持**

| ID | 协议 | 关键特性 | 优先级 |
|----|------|----------|--------|
| TN-010 | HTTP/1.1 | 请求检查、Host 路由、Header 注入 | P0 |
| TN-011 | HTTP/2 | 多路复用、Server Push | P0 |
| TN-012 | HTTPS | 自动 TLS 终止（Let's Encrypt / 用户证书） | P0 |
| TN-013 | WebSocket | 二进制帧透传、自动 Ping/Pong | P0 |
| TN-014 | gRPC | HTTP/2 原生支持、反射 | P1 |
| TN-015 | TCP | 任意 TCP 端口转发 | P0 |
| TN-016 | UDP | 数据报中继、打洞 | P1 |
| TN-017 | ICMP | Ping 穿透 | P2 |
| TN-018 | SSH | 原生 SSH 隧道（免 omnitun 客户端） | P1 |
| TN-019 | RDP | 远程桌面协议穿透 | P2 |

**A.3 隧道配置**

| ID | 参数 | 描述 | 默认值 |
|----|------|------|--------|
| TN-030 | 域名 | 隧道的公网入口地址 | `<slug>.omnitun.io` |
| TN-031 | TLS 策略 | edge（终止于 Relay）/ passthrough（透传到 Agent） | edge |
| TN-032 | 压缩 | gzip/brotli/zstd 传输压缩 | 开启 |
| TN-033 | 缓冲窗口 | TCP 流控窗口大小 | 64KB |
| TN-034 | 连接限制 | 最大并发连接数 | 100（Free）/ 1000（Pro）/ 无限制（Team+） |
| TN-035 | 超时 | 空闲连接超时时间 | 300s |
| TN-036 | IP 策略 | 白名单/黑名单 CIDR | 无限制 |
| TN-037 | 地域限制 | 允许访问的国家/地区列表 | 无限制 |
| TN-038 | 请求头操作 | 添加/删除/重写 HTTP Header | 无 |

---

### 模块 B：组网系统（Mesh Network）

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| ME-001 | 创建网络 | 创建加密的虚拟网络，分配网络 CIDR | P0 |
| ME-002 | 加入网络 | Agent 通过邀请码或 API 加入网络 | P0 |
| ME-003 | 节点发现 | 自动发现网络内其他节点及其暴露的服务 | P0 |
| ME-004 | P2P 直连 | STUN+UDP 打洞，成功后点对点加密通信 | P0 |
| ME-005 | Relay 回退 | P2P 失败时自动降级到中继转发 | P0 |
| ME-006 | 路由策略 | 网络级 ACL、流量策略 | P1 |
| ME-007 | 跨网络路由 | 多个 Mesh 网络之间的路由策略 | P2 |
| ME-008 | DNS | 网络内 DNS 解析（节点名.service.network） | P1 |

**NAT 穿透引擎规格**：

| 技术 | 用途 | 说明 |
|------|------|------|
| STUN | 探测 NAT 类型和公网地址 | 标准 STUN 服务器（RFC 5389） |
| UDP Hole Punching | 简单 NAT 穿透 | 两端同时发 UDP 包打通 |
| UPnP/NAT-PMP | 端口映射 | 路由器端口自动映射 |
| TURN Relay | 对称 NAT 兜底 | 自建 TURN 服务器 |
| DERP (借鉴Tailscale) | HTTPS 伪装中继 | 端口 443 的 HTTPS relay，无法被封锁 |

---

### 模块 C：认证与授权（Auth & RBAC）

**C.1 身份认证**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| AU-001 | 邮箱注册 | 邮箱 + 密码注册，邮箱验证 | P0 |
| AU-002 | 邮箱登录 | JWT Access Token + Refresh Token | P0 |
| AU-003 | OAuth 登录 | GitHub / Google / GitLab 登录 | P0 |
| AU-004 | MFA | TOTP / WebAuthn | P0 |
| AU-005 | 会话管理 | 查看/撤销活跃会话 | P1 |
| AU-006 | 密码策略 | 最小长度、复杂度、过期策略 | P1 |

**C.2 SSO（企业单点登录）**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| AU-010 | OIDC | OpenID Connect 通用 SSO | P0 |
| AU-011 | SAML 2.0 | SAML IdP 对接（Okta/Azure AD/OneLogin） | P0 |
| AU-012 | SCIM | 自动用户/组同步 | P1 |
| AU-013 | Just-in-Time Provisioning | 首次 SSO 登录自动创建用户 | P1 |
| AU-014 | 目录同步 | Google Workspace / Azure AD 目录同步 | P2 |

**C.3 RBAC 权限模型**

```
层级结构：
  Organization (租户)
  ├── Workspace A
  │   ├── Group 1
  │   │   ├── Member (role: Editor)
  │   │   └── ...
  │   └── Group 2
  │       ├── Member (role: Viewer)
  │       └── ...
  └── Workspace B
      └── ...
```

| 角色 | 权限范围 | 典型用户 |
|------|----------|----------|
| **Owner** | 组织完全管理权 | 创始人/CTO |
| **Admin** | 管理 Workspace/成员/账单 | 团队负责人 |
| **Editor** | 管理隧道/配置/网络 | 开发者 |
| **Viewer** | 只读查看隧道/监控 | QA/PM |
| **Agent** | 仅运行 Agent，无 Dashboard 权限 | CI/CD Runner |

**C.4 API 密钥管理**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| AU-020 | 创建 API Key | 生成 API Key（仅显示一次） | P0 |
| AU-021 | 权限范围 | 限制 API Key 的作用域（Workspace/Group） | P0 |
| AU-022 | 过期策略 | API Key 有效期设置 | P1 |
| AU-023 | IP 绑定 | API Key 绑定来源 IP/网段 | P2 |
| AU-024 | 使用审计 | API Key 历史调用记录 | P1 |

**C.5 平台级角色（Super Admin）**

| 角色 | 权限范围 |
|------|----------|
| **Super Admin** | 管理所有组织、查看全局配置、管理 Relay 节点、管理平台级功能开关 |

注意：Super Admin 仅供 OmniTun 内部运营使用，不对租户暴露。所有 Super Admin 操作均记录审计日志。

---

### 模块 D：隧道运行时能力

**D.1 流量管理**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| TR-001 | 请求检查 | Web Inspector 查看请求/响应 Headers + Body | P0 |
| TR-002 | 请求重放 | 重放历史请求 | P1 |
| TR-003 | 限速 | 按隧道/用户设置带宽上限 | P1 |
| TR-004 | 流量整形 | 优先级队列，QoS | P2 |
| TR-005 | 负载均衡 | 多 Agent 间的流量分发 | P1 |
| TR-006 | 断路器 | 后端异常时自动熔断 | P2 |

**D.2 访问控制（请求级别）**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| TR-010 | Basic Auth | 用户名/密码访问保护 | P0 |
| TR-011 | OAuth 2.0 Proxy | 第三方 OAuth 验证（如 Google） | P0 |
| TR-012 | IP 白名单/黑名单 | 按 CIDR 限制访问 | P0 |
| TR-013 | Country Block | 按国家/地区限制 | P1 |
| TR-014 | JWT 验证 | 验证请求中的 JWT Token | P2 |
| TR-015 | HMAC 签名验证 | 预共享密钥签名验证 | P2 |
| TR-016 | Mutual TLS | 客户端证书验证 | P2 |

**D.3 响应处理**

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| TR-020 | 自定义错误页面 | 404/503/被拒绝的自定义页面 | P1 |
| TR-021 | URL 重写 | 路径重写规则 | P1 |
| TR-022 | 缓存头注入 | 添加 Cache-Control 等头 | P2 |
| TR-023 | CORS 配置 | 自动处理跨域请求 | P1 |

---

### 模块 E：监控与可观测性

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| MO-001 | 实时流量监控 | Dashboard 显示实时 QPS / 带宽 / 活跃连接 | P0 |
| MO-002 | 历史流量统计 | 1h/24h/7d/30d 流量图表 | P1 |
| MO-003 | 连接日志 | 每条连接的时间、来源 IP、协议、字节数 | P0 |
| MO-004 | 错误追踪 | 隧道失败/超时/拒绝的错误收集 | P1 |
| MO-005 | 审计日志 | 完整的操作审计（谁在何时做了什么事） | P0 |
| MO-006 | 会话录制 | HTTP 请求/响应的完整录制与回放 | P2 |
| MO-007 | 告警规则 | 可配置的告警（流量异常、隧道断开、安全事件） | P1 |
| MO-008 | Webhook 通知 | 事件推送到外部 Webhook（Slack/Discord/自定义） | P1 |
| MO-009 | Prometheus 指标 | 标准 Prometheus metrics endpoint | P1 |
| MO-010 | 使用配额仪表 | 用户可查看剩余带宽/隧道数/连接数 | P0 |

---

### 模块 F：域名与证书管理

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| DM-001 | 自定义域名 | 绑定用户自有域名 | P0 |
| DM-002 | 域名验证 | DNS TXT / CNAME 验证所有权 | P0 |
| DM-003 | 自动 TLS | Let's Encrypt ACME 自动签发/续期 | P0 |
| DM-004 | 上传自有证书 | 用户上传 PEM 证书 | P1 |
| DM-005 | 泛域名证书 | *.domain.com 的泛域名配置 | P1 |
| DM-006 | 域名健康检查 | 自动检测 DNS 配置是否正确 | P1 |
| DM-007 | 域名迁移 | 域名在隧道间迁移 | P2 |

---

### 模块 G：Dashboard 管理控制台

**G.1 页面清单**

| 页面 | 核心功能 | 优先级 |
|------|----------|--------|
| 登录/注册 | 邮箱/OAuth 登录、注册、密码重置 | P0 |
| 工作区概览 | 活跃隧道数、流量总览、近期事件 | P0 |
| 隧道列表 | CRUD 隧道、启动/停止、状态显示 | P0 |
| 隧道详情 | 配置编辑、实时流量、日志、请求检查 | P0 |
| 网络列表 | Mesh 网络管理、节点列表 | P1 |
| 成员管理 | 邀请成员、角色分配、API Key 管理 | P0 |
| 审计日志 | 可搜索/过滤的操作审计日志 | P1 |
| 账单 & 用量 | 计划管理、用量统计、发票 | P0 |
| 组织设置 | SSO 配置、安全策略、域名设置 | P1 |
| 文档 & 帮助 | 快速入门、CLI 参考、API 文档 | P1 |

**G.2 Dashboard 技术规格**

- SPA 架构（React + TypeScript）
- 暗色/亮色主题
- 响应式布局（Desktop 优先，Tablet 可用）
- WebSocket 实时数据推送
- Dashboard 本身的请求也走 OmniTun 隧道（dogfooding）

---

### 模块 H：CLI 工具

**H.1 命令清单**

```
omnitun login              → 登录账号
omnitun logout             → 退出登录
omnitun whoami             → 查看当前身份

omnitun http <port>        → 快速创建 HTTP 隧道（最常用）
omnitun tcp <port>         → 快速创建 TCP 隧道
omnitun udp <port>         → 快速创建 UDP 隧道

omnitun tunnel list        → 列出所有隧道
omnitun tunnel create      → 交互式创建隧道
omnitun tunnel start <id>  → 启动隧道
omnitun tunnel stop <id>   → 停止隧道
omnitun tunnel delete <id> → 删除隧道
omnitun tunnel config <id> → 查看/编辑配置
omnitun tunnel logs <id>   → 实时查看流量日志

omnitun network create     → 创建 Mesh 网络
omnitun network join       → 加入已有网络
omnitun network list       → 列出网络

omnitun status             → 系统状态概览
omnitun version            → 版本信息
omnitun update             → 自动更新到最新版本

omnitun config             → 管理 CLI 配置文件
omnitun completion         → 生成 Shell 自动补全
```

**H.2 CLI 设计原则**

- 单二进制文件，零依赖
- `omnitun http 8080` 是最短路径，所有高级选项通过 flag 指定
- 彩色输出、进度条、友好错误信息
- 支持 `--json` 输出模式，可被脚本消费
- 自动检测更新（静默后台检查，不阻塞）

---

### 模块 I：SDK 与 API

**I.1 多语言 SDK 优先级**

| 语言 | 优先级 | 目标用户 | 核心场景 |
|------|--------|----------|----------|
| Go | P0 | 后端/CLI 工具开发者 | 程序化隧道管理、K8s Operator |
| Python | P0 | AI/数据工程师 | Jupyter Notebook、ML 模型暴露 |
| JavaScript/TypeScript | P1 | 前端全栈开发者 | Next.js 插件、Node.js 后端 |
| Java | P2 | 企业开发者 | Spring Boot 集成 |
| Rust | P2 | 系统/嵌入式开发者 | IoT 设备 Agent |

**I.2 API 设计原则**

- RESTful API（主要）+ WebSocket（实时）+ gRPC（内部服务间）
- OpenAPI 3.1 规范，自动生成 SDK 与文档
- 所有 API 端点均需认证（API Key 或 JWT）
- 速率限制（Free: 60 req/min, Pro: 600, Team+: 6000）
- 分页、过滤、排序统一规范

---

### 模块 J：企业功能

| ID | 功能 | 描述 | 优先级 |
|----|------|------|--------|
| EN-001 | 私有 Relay 节点 | 企业专属中继节点，数据不外传 | P0 |
| EN-002 | 私有部署 | 完整平台自托管（Air-gapped 可选） | P0 |
| EN-003 | 数据驻留 | 选择数据处理的地理区域 | P0 |
| EN-004 | 自定义策略引擎 | 基于 OPA/Rego 的策略定义 | P1 |
| EN-005 | SIEM 集成 | Splunk/Elastic/Sentinel 日志推送 | P1 |
| EN-006 | 合规报告 | SOC2/ISO27001 合规报告的自动生成 | P2 |
| EN-007 | 专属 SLA | 99.9%/99.99% 可选 | P0 |
| EN-008 | 专属支持 | 指定客户成功经理、7×24响应 | P0 |
| EN-009 | BYOK | 客户自带加密密钥（Bring Your Own Key） | P2 |

---

## 三、非功能需求

### 3.1 性能

| 指标 | 目标 | 测量方法 |
|------|------|----------|
| 隧道建立延迟 | P95 < 1s | Agent 启动到隧道 Active |
| 中继转发延迟 | P50 < 5ms, P99 < 20ms（同区域） | 中继 ingress → egress |
| 控制面 API 延迟 | P95 < 200ms | API Gateway → 响应 |
| Agent 内存占用 | < 50MB（空闲），< 200MB（高负载，含 P2P/加密） | 持续运行 24h 后测量 |
| Relay 吞吐量 | 单节点 > 1Gbps（实测） | 多核并行转发 |

### 3.2 可用性

| 指标 | 目标 | 说明 |
|------|------|------|
| 控制面可用性 | 99.9% | 跨可用区多副本 |
| 数据面可用性 | 99.95% | 跨区域多 Relay 自动故障转移 |
| 数据持久性 | 99.999999999%（11个9） | 对象存储多副本 |

### 3.3 安全

详见 [08-security-model.md](./08-security-model.md)

### 3.4 可扩展性

- 控制面支持水平扩展（无状态设计）
- Relay 节点支持按需扩缩容（连接数驱动）
- 数据库采用分片策略（按 Tenant ID）
- 支持 10 万+ 并发隧道连接

### 3.5 国际化

- Phase 1：英文 UI（全球默认）
- Phase 2：简体中文、日文
- Phase 3：繁体中文、韩文、西班牙文

### 3.6 Phase 1 MVP 安全最低要求

| 安全措施 | Phase 1 | Phase 2+ |
|----------|---------|----------|
| TLS 1.3 加密 | ✅ | ✅ |
| JWT 认证 | ✅ | ✅ |
| API Key 认证 | ✅ | ✅ |
| 租户数据隔离（RLS） | ✅ | ✅ |
| Agent mTLS | Phase 2 | ✅ |
| MFA | Phase 2 | ✅ |
| SSO/OIDC | Phase 2 | ✅ |
| 审计日志 | Phase 1（基础） | ✅（完整） |
