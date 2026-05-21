# OmniTun 3.0 — 企业级能力矩阵

> 面向中大型企业客户（$10K+ ACV）的完整企业级功能定义。每一项功能都是 Closed Won 的关键 deal-breaker。

---

## 一、SSO & Identity

企业客户采购 SaaS 的第一道门槛：能否融入其现有身份体系。

### 1.1 OIDC (OpenID Connect)

| 属性 | 说明 |
|------|------|
| **状态** | ✅ 已接入 |
| **协议** | OpenID Connect 1.0 |
| **支持 Provider** | Google, GitHub, GitLab, Apple, 任意 OIDC 兼容 IdP |
| **配置方式** | Organization Settings → SSO → OIDC 配置 |
| **Claim 映射** | sub → external_id, email → email, name → display_name, groups → role |
| **Token 验证** | RS256/HS256 签名验证, JWKS endpoint 自动轮换 |
| **Session 管理** | Refresh token rotation, 强制 re-auth 周期可配置 |

### 1.2 SAML 2.0

| 属性 | 说明 |
|------|------|
| **状态** | ✅ 已接入 |
| **支持 Provider** | Azure AD / Entra ID, Okta, OneLogin, PingIdentity, JumpCloud |
| **SP-initiated** | ✅ 已支持（从 OmniTun 发起登录） |
| **IdP-initiated** | ✅ 已支持（从 IdP 仪表板直接跳转） |
| **Metadata** | SP metadata XML 自动生成, IdP metadata XML 导入 / URL 拉取 |
| **签名** | SHA-256 with RSA, 证书自动校验 |
| **属性映射** | NameID → external_id, AttributeStatement 映射 email/name/role/groups |
| **SLO** | Single Logout (front-channel + back-channel) |
| **ForceAuthn** | 支持，可在安全策略中按组织配置 |

### 1.3 SCIM 2.0

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **协议** | SCIM 2.0 (RFC 7643/7644) |
| **端点** | `GET/POST /scim/v2/Users`, `GET/PUT/PATCH/DELETE /scim/v2/Users/{id}`, `GET/POST /scim/v2/Groups` |
| **支持操作** | Create / Update / Patch / Delete / Sync |
| **Provisioning** | IdP 创建用户 → 自动同步到 OmniTun 组织 |
| **Deprovisioning** | IdP 禁用/删除用户 → OmniTun 自动冻结/移除 |
| **Group Push** | IdP 组映射到 OmniTun 角色（如 `omnitun-admin` → Org Admin） |
| **兼容 IdP** | Azure AD / Entra ID, Okta, OneLogin, JumpCloud |
| **Bearer Token** | SCIM endpoint 使用独立 token 鉴权，不限组织 JWT |

### 1.4 Just-in-Time Provisioning

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **触发条件** | 用户首次通过 SSO (OIDC/SAML) 成功认证但本地无账号 |
| **行为** | 自动创建用户 + 加入组织 + 分配默认角色 |
| **属性来源** | IdP assertion/claim 中的 email, name, groups |
| **默认角色** | 组织可配置新用户的默认角色（如 Viewer） |
| **安全策略** | 可限制 JIT 仅对特定域名生效（如 `@acme.com`） |
| **审计日志** | JIT 创建事件单独标记，便于审计追踪 |

### 1.5 Directory Sync

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **Google Workspace** | Google Directory API 集成，同步用户和组 |
| **Azure AD** | Microsoft Graph API 集成，同步用户和管理单元 |
| **同步频率** | 可配置：实时 (webhook) / 每 5 分钟 / 每小时 |
| **同步范围** | 可限定同步特定 OU / Group / Administrative Unit |
| **冲突策略** | 可配置：IdP 优先 / OmniTun 优先 / 合并 / 跳过 |

### 1.6 IdP-initiated Login

| 属性 | 说明 |
|------|------|
| **状态** | ✅ 已支持 |
| **流程** | 用户从 IdP 仪表板点击 OmniTun 图标 → IdP 发送 SAMLResponse → OmniTun 验证 → 登录 |
| **RelayState** | 支持，可指定登录后跳转目标页面 |
| **错误处理** | 友好的 IdP-init 失败页面，含原因说明和重试引导 |

---

## 二、RBAC 增强

2.0 已实现基础 RBAC（Admin / Editor / Viewer），3.0 需要达到企业级精细粒度。

### 2.1 自定义角色

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **定义** | Organization Owner 可创建自定义角色，命名并配置权限集 |
| **权限粒度** | 精细到每个 API Action（如 `tunnel:create`, `tunnel:delete`, `domain:create`, `billing:read`） |
| **角色模板** | 提供预设模板：Security Auditor, Billing Manager, Developer, Read-only Support |
| **修改传播** | 修改角色权限后，所有持有该角色的用户即时生效 |
| **角色上限** | 每组织最多 20 个自定义角色（Enterprise 无限制） |
| **角色层级** | 不支持角色继承（避免 RBAC 复杂性爆炸），采用扁平权限集 |

### 2.2 资源级权限

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **隧道级** | 每个隧道可单独配置哪些用户/角色有访问/管理权限 |
| **网络级** | P2P 网络可配置哪些用户可加入/查看 |
| **域名级** | 自定义域名可配置哪些用户可绑定/修改 DNS |
| **默认策略** | 新建资源继承组织级默认 ACL |
| **批量操作** | 支持批量修改多个资源的 ACL |

### 2.3 临时访问

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **应用场景** | 外部承包商临时访问、渗透测试、审计、On-call 工程师 |
| **时长选项** | 1h / 4h / 8h / 24h / 48h / 72h / 7d / 自定义 |
| **权限限定** | 可指定临时访问的具体资源范围 |
| **自动失效** | 到期后自动撤销所有权限，并记录审计日志 |
| **访问 Token** | 生成一次性或时限性 API Token，到期自动作废 |
| **审批** | 可选：临时访问需 Owner/Admin 审批 |

### 2.4 审批流程

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **敏感操作** | 删除隧道（含活跃连接时）、变更套餐、删除团队成员、解散组织 |
| **审批链** | 可配置单级审批（Owner/Admin）或两级审批 |
| **通知** | In-app 通知 + Email + （可选）Slack Webhook |
| **时效** | 审批请求 72h 未处理自动关闭 |
| **审计** | 所有审批请求及结果记录在审计日志 |

---

## 三、审计与合规

### 3.1 完整审计日志

| 属性 | 说明 |
|------|------|
| **状态** | ✅ 基础已有 (2.0 已写入审计日志) |
| **记录范围** | 所有 CRUD 操作：创建、读取、更新、删除、登录、登出、权限变更、计费事件 |
| **Before/After Diff** | Update 操作记录完整的变更前后差异 |
| **不可篡改** | 审计日志写入后不可修改/删除（append-only） |
| **加密存储** | At-rest 使用 AES-256-GCM 加密 |
| **保留期限** | 按数据保留策略配置（默认 1 年） |

### 3.2 审计报告生成

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **SOC2 模板** | 预建报告：用户访问审计、变更管理审计、安全事件审计 |
| **ISO27001 模板** | 预建报告：A.9 访问控制、A.12 操作安全、A.16 事件管理 |
| **自定义报告** | 自选时间范围、操作类型、用户/资源过滤 |
| **生成格式** | PDF (带公司 Logo) / CSV / JSON |
| **签名** | 报告可添加数字签名，保证完整性 |
| **调度** | 支持定期自动生成（月/季/年）并发送给指定收件人 |

### 3.3 审计日志导出

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **格式** | CSV / JSON / NDJSON |
| **过滤条件** | 日期范围、操作类型、用户、资源、IP、结果(成功/失败) |
| **导出范围** | 全量导出或过滤后导出 |
| **超大导出** | 超过 100 万条自动分片多个文件，提供下载链接 |
| **API 导出** | 支持通过 API 拉取审计日志（用于 SIEM 集成） |

### 3.4 数据保留策略

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **配置层级** | 全局默认策略 + 组织级覆盖策略 |
| **保留期选项** | 30 天 / 90 天 / 1 年 / 3 年 / 永久 |
| **数据类型** | 审计日志、用量数据、隧道流量元数据、会话记录 |
| **自动清理** | Cron job 定期执行过期数据清理 |
| **清理前通知** | 自动发送通知给组织 Owner |
| **合规保留** | 支持 Legal Hold（禁止清理特定数据以满足法律要求） |
| **Plan 限制** | Free: 30d / Pro: 90d / Team: 1y / Business: 3y / Enterprise: 自定义 |

### 3.5 合规认证路径

| 认证 | 目标时间 | 状态 |
|------|----------|------|
| **SOC2 Type II** | Phase 3.6 完成后 6 个月 | 🗺️ 路径规划 |
| **ISO27001:2022** | SOC2 完成后 3 个月 | 🗺️ 路径规划 |
| **GDPR 合规** | Phase 3.3 完成后 | 🗺️ 路径规划 |
| **HIPAA 合规** | Enterprise 定制 (需 BAA) | 🗺️ 路径规划 |
| **PCI DSS** | 不计划（不直接处理持卡人数据，由 Stripe 承担） | — |

**SOC2 / ISO27001 前置依赖清单**：
- 完整审计日志 (3.1) ✅
- 数据保留策略 (3.4)
- RBAC 增强 (二)
- 审批流程 (2.4)
- 安全事件响应流程
- 渗透测试 (外部第三方)
- 漏洞管理流程
- 员工背景审查流程
- 供应商风险评估

---

## 四、网络安全

### 4.1 IP 白名单

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **层级** | 组织级 + 隧道级（隧道级覆盖组织级） |
| **格式** | IPv4 / IPv6 单地址或 CIDR |
| **行为** | 白名单外的 IP 无法建立隧道连接 |
| **管理** | UI 批量添加 + API 管理 |
| **默认** | 不设置白名单 = 无 IP 限制 |
| **审计** | 被白名单拒绝的连接记录在审计日志 |

### 4.2 Private Network Peering

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **AWS PrivateLink** | Relay 节点通过 VPC Endpoint 暴露，客户 VPC 内直接互联 |
| **GCP Private Service Connect** | GCP 等效方案 |
| **Azure Private Link** | Azure 等效方案 |
| **优势** | 流量不经过公网，降低延迟，满足合规要求 |
| **定价** | Enterprise 专属，按 endpoint 计费 |
| **前置条件** | 客户需在 Relay 所在地域有 VPC/VNet |

### 4.3 Mutual TLS (mTLS)

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **能力** | 隧道连接要求客户端出示 X.509 证书进行双向认证 |
| **证书管理** | OmniTun 可签发客户端证书，或客户自行上传 CA |
| ** revocation** | 支持 CRL 和 OCSP |
| **策略** | 可按隧道配置：disabled / optional / required |
| **客户端** | CLI Agent 支持 `--client-cert` 和 `--client-key` 参数 |
| **Agent 自动获取** | Agent 可自动从 OmniTun API 获取短效客户端证书 |

### 4.4 WAF 规则

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **规则类型** | IP 黑白名单、请求频率限制、请求大小限制、Header 过滤、路径过滤、Method 过滤 |
| **预设规则集** | OWASP Top 10 防护（SQLi, XSS, CSRF, LFI 等） |
| **自定义规则** | 支持正则表达式匹配 URL / Header / Body |
| **操作** | Allow / Block / Log / Challenge (JS Challenge) |
| **作用范围** | 全局 / 隧道级 / 域名级 |
| **日志** | 触发 WAF 规则的请求详情记录 |

### 4.5 DDoS 防护

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **Rate-based** | 单 IP / 单隧道的请求速率限制，超过阈值自动限流 |
| **Pattern-based** | 识别 SYN flood、HTTP flood、Slowloris 等攻击模式 |
| **多层防护** | Edge (CDN 层) → Relay (入口层) → Origin (隧道出口层) |
| **自动缓解** | 检测到攻击自动启用限流规则 |
| **告警** | 攻击检测 → 通知组织 Owner + 平台运维 |
| **报表** | 攻击事件报表：来源、规模、持续时长、缓解效果 |

---

## 五、数据驻留

### 5.1 区域选择

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **区域选项** | United States, European Union, Asia-Pacific (Singapore/Tokyo) |
| **粒度** | 组织级选择数据驻留区域 |
| **数据范围** | 隧道元数据、用户 PII、审计日志、TLS 证书 |
| **Relay 数据** | Relay 仅做数据转发，不解包用户 payload，不存储 |
| **变更限制** | 数据驻留区域一旦选择不可更改（或需人工迁移） |
| **合规声明** | 在设置页面显示各区域的具体合规认证状态 |

### 5.2 数据不出境（China Region）

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **基础设施** | 中国区独立集群，与全球集群物理隔离 |
| **合规** | 满足《数据安全法》《个人信息保护法》要求 |
| **网络** | 使用中国本土 CDN、对象存储、数据库服务 |
| **Relay** | 中国区 Relay 仅部署在境内节点 |
| **运维** | 由中国区团队独立运维，境外团队无数据访问权限 |
| **前置** | 需完成 ICP 备案、等保认证 |

### 5.3 Customer-Managed Keys (CMK)

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **方案** | 客户通过 AWS KMS / GCP Cloud KMS / Azure Key Vault 管理主密钥 |
| **加密范围** | 隧道元数据、TLS 私钥、审计日志、用户 PII |
| **密钥轮换** | 支持自动轮换（按客户 KMS 策略） |
| **密钥吊销** | 客户吊销密钥 → OmniTun 无法解密数据（加密销毁的等效效果） |
| **定价** | Enterprise 专属 |

### 5.4 VPC 部署

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **方案** | 在客户 AWS/GCP/Azure VPC 内部署专用 Relay 节点 |
| **管理** | Relay 由 OmniTun 远程管理，但数据面完全在客户网络中 |
| **优势** | 零公网暴露，满足金融/医疗/政府合规要求 |
| **HA** | 支持多 AZ 冗余部署 |
| **监控** | Relay 健康状态回传 OmniTun（仅遥测数据，不含 payload） |

---

## 六、私有部署

### 6.1 Self-Hosted 完整版

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **交付形式** | Helm Chart (Kubernetes) + Docker Compose (简单场景) |
| **组件** | API Server, Control Plane, Relay Controller, Dashboard, 数据库迁移 |
| **依赖** | PostgreSQL 16+, Redis 7+, S3-compatible object storage |
| **版本策略** | 跟随 SaaS 版本，每月发布稳定版，每季度 LTS |
| **升级** | Helm upgrade 一键升级 + 数据库迁移自动执行 |
| **回滚** | 支持一键回滚到前一版本 |
| **定价** | Enterprise 年度合同含 Self-Hosted 许可 |

### 6.2 Air-Gapped 部署

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **交付形式** | 离线安装包（tar.gz + 容器镜像离线 bundle） |
| **镜像仓库** | 离线 bundle 包含所有容器镜像，无需访问 Docker Hub |
| **文档** | 离线版安装手册（PDF + Markdown） |
| **Helm Repo** | 离线 Helm Chart，不依赖外部 Helm repo |
| **更新** | 定期发布离线更新包，通过安全介质交付 |
| **限制** | TLS 证书需自行签发/导入（无法调用公共 CA API）；自动更新、远程监控不可用 |

### 6.3 HA 配置

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **架构** | Multi-AZ, Multi-Region |
| **组件 HA** | API Server ≥ 3 副本, Relay Controller Active-Passive, DB Primary-Standby |
| **区域容灾** | 跨 Region 数据库异步复制, DNS failover |
| **RPO** | < 1 分钟（数据库同步延迟） |
| **RTO** | < 15 分钟（自动化故障切换） |
| **负载均衡** | Kubernetes Ingress + Service Mesh |

### 6.4 企业支持

| 属性 | 说明 |
|------|------|
| **状态** | 📋 待实现 |
| **响应时间** | Critical: 1h / High: 4h / Normal: 8h / Low: 24h |
| **支持时间** | 24×7×365 |
| **TAM** | 专属 Technical Account Manager |
| **Onboarding** | 专属实施工程师协助部署和配置 |
| **健康检查** | 季度健康检查：性能、安全、升级建议 |
| **紧急热线** | Dedicated phone line for P0 incidents |

---

## 七、SLA & 支持

### 7.1 SLA 层级

| 层级 | 可用性 | 适用计划 | 测量范围 |
|------|--------|----------|----------|
| **Standard** | 99.9% (月) | Team, Business | API 可用性 + Tunnel 控制面 |
| **Premium** | 99.99% (月) | Enterprise | API 可用性 + Tunnel 控制面 + Relay 数据面 |

**测量方法**：
- 可用性 = (计划正常运行时间 - 故障时长) / 计划正常运行时间 × 100%
- API 可用性：`GET /health` 5xx 错误率
- Tunnel 控制面：Tunnel 创建 / 连接成功率
- Relay 数据面：Relay 节点转发成功率
- 计划维护窗口不计入故障时间（提前 72h 通知）

### 7.2 SLA 赔付

| 月度可用性 | 赔付比例 (月费) | 记账方式 |
|------------|-----------------|----------|
| < 99.9% (Standard) / < 99.99% (Premium) | 10% | Service Credit |
| < 99.0% | 25% | Service Credit |
| < 95.0% | 50% | Service Credit |
| < 90.0% | 100% | Service Credit |

**细则**：
- 客户需在故障发生后 30 天内申请赔付
- Service Credit 仅可用于抵扣未来账单，不可兑换现金
- 单月最高赔付为当月费用的 100%
- 不可抗力（大规模 DDoS、上游云服务商全区域故障）可豁免

### 7.3 支持通道

| 通道 | Standard | Premium (Enterprise) |
|------|----------|----------------------|
| **Email** | support@omnitun.io | 专属邮箱 |
| **Slack** | — | Shared Slack Channel |
| **Phone** | — | Dedicated Hotline |
| **Priority Ticket** | — | 管理后台直接提 Priority 工单 |
| **Status Page** | status.omnitun.io | 含 Email/SMS/Webhook 订阅通知 |

### 7.4 状态页

| 属性 | 说明 |
|------|------|
| **域名** | status.omnitun.io |
| **内容** | 实时服务状态、组件状态（API / Relay / Dashboard / SSO）、当前事件、历史事件 |
| **事件记录** | Root Cause Analysis（RCA）在事件解决后 5 个工作日内发布 |
| **通知** | Email / SMS / Webhook / RSS 订阅 |
| **组件粒度** | 按 Region → Service → Component 三级展示 |
| **API** | 状态数据通过 API 可获取，用于客户自动监控 |

---

## 八、版本记录

| 版本 | 日期 | 作者 | 变更 |
|------|------|------|------|
| v1.0 | 2026-05-21 | OmniTun Enterprise Team | 企业级能力矩阵首次发布 |
