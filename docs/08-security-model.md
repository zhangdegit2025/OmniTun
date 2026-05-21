# OmniTun — 安全模型与合规

> **修订说明**
> - 修订时间：2026-05-20
> - 修订内容：
>   - 增加"3.4 平台级权限（Super Admin）"章节
>   - 增加"3.5 密钥管理详细设计"章节
>   - 增加"3.6 依赖许可证审查"章节

## 一、安全原则

OmniTun 安全模型遵循 **纵深防御（Defense in Depth）** + **零信任（Zero Trust）** 原则。

| 原则 | 含义 |
|------|------|
| **默认拒绝** | 所有连接默认被拒绝，除非明确授权 |
| **最小权限** | 每个组件只拥有完成任务所必需的最小权限 |
| **持续验证** | 不信任持久会话，每次请求验证身份和权限 |
| **纵深防御** | 多层安全控制，单层失败不影响整体安全 |
| **安全透明** | 安全机制对用户可见、可审计、可验证 |

---

## 二、网络边界安全

### 2.1 网络分段

```
┌──────────────────────────────────────────────┐
│                 Internet                      │
└────────────┬─────────────────────────────────┘
             │ DDoS Protection (Cloudflare / AWS Shield)
             ▼
┌──────────────────────────────────────────────┐
│                 DMZ                           │
│  Edge Proxies (TLS Termination, WAF)          │
└────────────┬─────────────────────────────────┘
             │ mTLS
             ▼
┌──────────────────────────────────────────────┐
│              Data Plane (Relay)               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Relay 1  │  │ Relay 2  │  │ Relay N  │   │
│  └──────────┘  └──────────┘  └──────────┘   │
└────────────┬─────────────────────────────────┘
             │ Internal Network (VPC / VPN)
             ▼
┌──────────────────────────────────────────────┐
│            Control Plane                      │
│  API Gateway → Services → Databases           │
└──────────────────────────────────────────────┘
```

### 2.2 各层安全控制

| 层级 | 控制措施 |
|------|----------|
| **L3/L4** | DDoS 防护、IP 信誉过滤、Geo-IP 封禁（管理员配置） |
| **L7** | WAF（OWASP CRS）、速率限制、payload 大小限制 |
| **TLS** | TLS 1.3 强制、仅现代密码套件、HSTS preload |
| **应用** | JWT 验证、RBAC、输入验证、SQL 注入防护 |
| **服务间** | mTLS（服务网格）、gRPC auth interceptor |

---

## 三、身份与访问管理

### 3.1 认证体系

| 认证方式 | 用途 | 安全等级 |
|----------|------|----------|
| 邮箱 + 密码 | 标准用户登录 | Standard |
| 邮箱 + 密码 + TOTP MFA | 增强安全 | High |
| 邮箱 + 密码 + WebAuthn | 最强安全 | Maximum |
| OAuth (GitHub/Google) | 社交登录 | Standard（依赖第三方安全） |
| SSO (OIDC/SAML) | 企业统一认证 | Depends on IdP |
| API Key (ot_sk_xxx) | 程序化访问 | Standard（永久密钥） |
| Agent Token | 内网客户端认证 | Standard（自动轮换） |

### 3.2 令牌设计

**JWT Access Token**：
- 有效期：1 小时
- 签名：RS256（非对称，便于分布式验证）
- 包含：`sub`, `org_id`, `role`, `scopes`, `jti`
- 无状态验证（服务节点缓存 JWKS）

**Refresh Token**：
- 有效期：30 天（可配置）
- 存储：数据库（可撤销）
- 轮换策略：每次使用后轮换（Refresh Token Rotation）
- 检测：Refresh token 重用 → 立即撤销该用户所有 session（防泄露）

**API Key (ot_sk_xxx)**：
- 格式：`ot_sk_` + 32 bytes random (base64url)
- 存储：bcrypt hash（cost=12）
- 仅创建时返回完整 key，后续不可恢复
- 支持有效期限定和 IP 绑定

**Agent Token**：
- 短期 token（有效期 1 小时）
- Agent 首次认证用 API Key，后续用 refresh 自动轮换
- 绑定 Agent 实例（agent_id），不允许跨 Agent 使用

### 3.3 权限模型

```
Role Hierarchy:
  Super Admin (platform)
    │
  Organization Owner
    ├── Workspace Admin
    │     ├── Workspace Editor
    │     └── Workspace Viewer
    └── (跨 Workspace)

Permission Scopes (fine-grained):
  tunnel:*            → 完全管理隧道
  tunnel:read         → 只读隧道
  tunnel:create       → 创建隧道
  tunnel:delete       → 删除隧道
  network:*           → 完全管理网络
  member:*            → 管理成员
  billing:*           → 管理计费
  audit:read          → 查看审计日志
```

RBAC 评估使用 **Open Policy Agent (OPA)** 的 Rego 策略：
```rego
package omnitun.auth

default allow = false

allow {
    input.role == "owner"
}

allow {
    input.role == "admin"
    input.workspace_id == input.target_workspace
}

allow {
    input.role == "editor"
    input.workspace_id == input.target_workspace
    input.action in {"tunnel:create", "tunnel:read", "tunnel:update"}
}
```

### 3.4 平台级权限（Super Admin）

| 角色 | 权限范围 | 典型用户 |
|------|----------|----------|
| **Super Admin** | 所有组织的数据查看、Relay 节点管理、平台配置、Feature Flag、审计日志 | OmniTun SRE/运营 |

**Super Admin 操作限制**：
- 不得主动查看租户隧道流量内容
- 所有操作必须附带理由并记录审计日志
- 紧急维护需经过值班 SRE 二次确认
- 客户数据在未经授权情况下不可导出

### 3.5 密钥管理详细设计

**密钥类型与轮换策略**：

| 密钥类型 | 加密方式 | 存储 | 轮换周期 |
|----------|----------|------|----------|
| Master Key | HSM（Cloud KMS / Vault） | KMS | 年度轮换 |
| DEK | AES-256-GCM | Vault | 30 天轮换 |
| JWT Signing Key | RS256 Key Pair | Vault | 90 天轮换 |
| API Key HMAC | HMAC-SHA256 | Vault | 按需 |
| TLS 证书 | RSA/ECDSA | Vault + S3 | 90 天（Let's Encrypt）|

**Envelope Encryption 流程**：
```
1. 生成随机 DEK
2. 用 Master Key 加密 DEK → DEK (encrypted)
3. 用 DEK 加密数据 → ciphertext
4. 存储：DEK(encrypted) + ciphertext
```

### 3.6 依赖许可证审查

**白名单（允许使用）**：
- MIT
- Apache 2.0
- BSD (2-clause, 3-clause)
- ISC
- Go standard library

**灰名单（需法律意见）**：
- LGPL（动态链接可接受）
- MPL

**黑名单（禁止使用）**：
- GPL v2 / v3
- AGPL
- SSPL

**当前依赖审计**：
- WireGuard（GPL v2）：用于 P2P Mesh ⚠️ 需法律意见
- 所有其他直接依赖已确认为白名单许可证

---

## 四、数据加密

### 4.1 传输加密

| 通信路径 | 加密方式 |
|----------|----------|
| 用户浏览器 ↔ Edge | TLS 1.3 (ECDHE + AES-256-GCM) |
| Agent ↔ Relay (数据) | QUIC + TLS 1.3 (mTLS) |
| Agent ↔ Control (控制) | WebSocket over TLS 1.3 |
| Control ↔ Relay | gRPC over TLS 1.3 (mTLS) |
| 服务间 | gRPC over mTLS (Linkerd) |
| P2P (Mesh) | WireGuard (Noise IK + ChaCha20-Poly1305) |

### 4.2 静态加密

| 数据 | 加密方案 | 密钥管理 |
|------|----------|----------|
| 用户密码 | bcrypt (cost=12) | N/A（不可逆） |
| API Key | bcrypt hash | N/A（不可逆） |
| TLS 私钥 | AES-256-GCM envelope | Vault / Cloud KMS |
| MFA TOTP Secret | AES-256-GCM | Vault / Cloud KMS |
| Mesh 私钥 (WireGuard) | AES-256-GCM | Vault / Cloud KMS |
| 审计日志敏感字段 | AES-256-GCM | Application Key |
| 数据库备份 | 存储层加密（LUKS / Cloud KMS） | 基础设施密钥 |
| 对象存储（S3/MinIO） | SSE (Server-Side Encryption) | 基础设施密钥 |

### 4.3 密钥管理

```
Key Hierarchy:
  Master Key (Vault auto-unseal / Cloud KMS)
    ├── Data Encryption Key (DEK, 定期轮换)
    │     ├── TLS cert private keys
    │     ├── MFA secrets
    │     └── Mesh keys
    ├── JWT Signing Key (RS256 key pair, 90天轮换)
    └── API Key HMAC Key
```

密钥轮换策略：
- JWT 签名密钥：90 天自动轮换（支持多 key 并存）
- DEK：30 天自动轮换
- TLS 私钥：每签发新证书自动生成

---

## 五、隧道安全

### 5.1 隧道隔离保证

| 威胁 | 防护 |
|------|------|
| 租户 A 访问租户 B 的隧道 | 每个连接基于 Host header / SNI 解析 → tunnel_id → tenant_id 验证 |
| Agent 冒充其他隧道 | Agent Token 绑定 agent_id + tunnel_id，Relay 端二次验证 |
| 恶意流量注入 | 每个隧道独立的连接池，帧协议携带 tunnel_id，Relay 侧校验 |
| 中间人攻击 | mTLS + 证书固定（Agent 内置 Relay 公钥指纹） |

### 5.2 访问控制执行点

```
                     ┌──────────────┐
  Internet ─────────▶│ Edge Proxy   │◀── 1. IP whitelist/blacklist
                     └──────┬───────┘    2. Country block
                            │
                     ┌──────▼───────┐
                     │ Relay Node   │◀── 3. Basic Auth / OAuth Proxy
                     └──────┬───────┘    4. Rate limiting
                            │
                     ┌──────▼───────┐
                     │ Agent        │◀── 5. 自定义 Header 验证
                     └──────────────┘    6. HMAC 签名验证
                                         7. JWT 验证
```

### 5.3 防滥用

| 滥用类型 | 检测与防护 |
|----------|------------|
| C2 (Command & Control) | 请求模式分析、已知恶意域名/IP 库、ASN 信誉 |
| 钓鱼 | 新域名注册监控、内容检查（仅自定义域名）、举报机制 |
| DDoS 出口 | 出口字节数限速、异常流量自动熔断 |
| 加密货币挖矿 | 高持久连接 + 高带宽模式检测 |
| 非法内容 | 自定义域名内容抽样检查 + 用户举报 |
| 签名滥用 | 新用户隧道观察期（24h 内限制带宽和连接数） |

---

## 六、运维安全

### 6.1 生产环境访问

| 访问方式 | 策略 |
|----------|------|
| 生产 SSH | 仅通过堡垒机（Teleport / AWS SSM），双因素认证 |
| 数据库直接访问 | 禁止。仅通过 Audit Proxy 或只读副本 |
| CI/CD 部署 | GitHub Actions + OIDC trust（无长期密钥） |
| K8s 集群操作 | kubectl via OIDC（与 SSO 绑定） |

### 6.2 安全监控

- **WAF 日志** → SIEM（安全信息和事件管理）
- **异常登录检测**：地理位置变化、新设备、非常规时间
- **API 异常检测**：速率异常、错误率突增、非典型 endpoint 访问
- **隧道异常检测**：流量突增、连接数异常、从未见地区访问

### 6.3 事件响应

| 阶段 | 动作 |
|------|------|
| 检测 | 自动化告警（Prometheus AlertManager + PagerDuty） |
| 隔离 | 自动撤销受影响 token、封禁 IP、冻结隧道 |
| 分析 | 审计日志回溯、流量录制回放 |
| 恢复 | 证书轮换、key 轮换、系统恢复 |
| 报告 | 客户通知（SLA 24h 内）、合规报告生成 |

---

## 七、合规

### 7.1 目标认证

| 认证 | 目标时间 | 说明 |
|------|----------|------|
| SOC 2 Type I | Year 1 Q4 | 安全、可用性、机密性 |
| SOC 2 Type II | Year 2 Q2 | 持续合规（6个月观察期） |
| ISO 27001 | Year 2 Q3 | 信息安全管理体系 |
| GDPR | Year 1 Q2 | 数据处理合规 |
| CCPA | Year 2 Q1 | 加州消费者隐私 |
| 中国等保 2.0 | Year 1 Q4 | 国内节点独立认证 |

### 7.2 数据驻留

- 默认数据处理区域：用户选择（US / EU / APAC）
- 控制面多区域部署，数据不跨区域复制（除非用于 HA）
- Enterprise 可选专属区域和私有部署
- 中国区独立部署，数据不出境

### 7.3 数据处理协议 (DPA)

作为数据处理方（Data Processor），OmniTun 承诺：
- 仅按客户指令处理数据（Tunnel 流量是 passthrough）
- 不挖掘用户流量内容
- 支持数据删除请求（30 天内执行）
- 安全事件 72 小时内通知

---

## 八、安全开发实践

| 实践 | 要求 |
|------|------|
| 代码审查 | 必须至少 1 人审查（关键模块 2 人） |
| SAST | CodeQL / Semgrep 集成到 CI |
| DAST | 定期 OWASP ZAP 扫描 staging 环境 |
| 依赖扫描 | Dependabot / Renovate 自动 PR |
| 容器扫描 | Trivy / Grype 扫描所有镜像 |
| 密钥扫描 | Gitleaks / truffleHog，阻止密钥提交 |
| 安全培训 | 全员 OWASP Top 10 培训（年） |
| Bug Bounty | Year 1 Q3 启动（HackerOne / Intigriti） |
| Penetration Test | Year 1 Q3（第三方安全公司），此后每半年 |
