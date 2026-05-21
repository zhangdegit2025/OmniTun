# OmniTun 2.0 — 验收标准与投产清单

> 注意：标注 🌐 的验收项在单机开发环境下使用 mock/本地回环方式验证，生产部署时需完整环境支持。

> 投产签收条件：以下所有项必须通过。逐项打勾后项目可投产。

## 一、功能验收

### 1.1 隧道能力

- [ ] `omnitun http 8080` → 3 秒内获得公网 URL
- [ ] 浏览器访问公网 URL → 返回本地 8080 服务响应
- [ ] WebSocket 升级请求通过隧道正常工作
- [ ] 多并发请求（100 RPS）无丢失
- [ ] Agent 断线 30s 内自动重连，URL 保持不变
- [ ] Ctrl+C 优雅关闭隧道 → 状态变为 stopped
- [ ] 同一 Agent 可同时运行 10 个隧道

### 1.2 TLS 与域名

- [ ] 系统域名 `*.omnitun.io` 自动签发泛域名证书
- [ ] 用户自定义域名 → 完成 DNS 验证 → 自动签发独立证书
- [ ] 证书到期前 30 天自动续期，无服务中断
- [ ] 用户可上传自有 TLS 证书
- [ ] 域名健康检查 Dashboard 可查看状态

### 1.3 认证与授权

- [ ] 邮箱注册 → 验证邮件 → 设置密码 → 登录 → 获取 JWT
- [ ] GitHub OAuth 登录正常
- [ ] Google OAuth 登录正常
- [ ] 🌐 OIDC SSO（Okta）登录正常 ← 至少一个 IdP
- [ ] 🌐 SAML SSO（Azure AD）登录正常
- [ ] MFA 注册 → 下次登录需输入 TOTP 码
- [ ] API Key 创建 → 用于 API 调用 → 撤销后立即失效
- [ ] Owner 可管理工作区和成员
- [ ] Editor 可管理隧道但不能删工作区
- [ ] Viewer 只能查看不能修改

### 1.4 审计

- [ ] 注册/登录/创建隧道/删除隧道/创建 API Key 全部有审计记录
- [ ] Dashboard 可按时间范围搜索审计日志
- [ ] 可按操作用户过滤审计日志

### 1.5 P2P 与 Mesh

- [ ] 两个 Agent 在同一局域网 → 自动 P2P 直连
- [ ] 🌐 两个 Agent 在不同 NAT 后 → UDP 打洞成功 → 直连
- [ ] 打洞失败 → 自动降级 TURN Relay → 数据正常
- [ ] TURN 也失败 → 自动降级 DERP → 数据正常
- [ ] Mesh 网络创建 → Agent 加入 → 网络内可互通
- [ ] 网络内 DNS 解析正常（node.service.network）

### 1.6 计费

- [ ] Free 用户创建第 2 个隧道 → 提示升级
- [ ] 🌐 点击升级 Pro → Stripe Checkout → 支付成功 → 计划变 Pro
- [ ] 用量 Dashboard 显示正确带宽和隧道数
- [ ] 发票可查看

---

## 二、非功能验收

### 2.1 性能

- [ ] P95 隧道建立延迟 < 3s
- [ ] 同区域 Relay 转发延迟 P50 < 5ms
- [ ] 同区域 Relay 转发延迟 P99 < 50ms
- [ ] Agent 内存占用 < 200MB（高负载）
- [ ] 单 Relay 支持 1000 并发隧道
- [ ] 100 并发隧道的 API 响应 P95 < 500ms

### 2.2 可用性

- [ ] 单 Relay 节点故障 → Agent 30s 内切换到其他 Relay
- [ ] API Server 重启期间已有隧道不受影响
- [ ] 控制面 2 副本 → 单副本故障不影响服务

### 2.3 安全

- [ ] 🌐 全部 HTTP 流量 → HTTPS（TLS 1.3）
- [ ] Agent ↔ Relay 通信 → mTLS
- [ ] 无硬编码密钥（gitleaks 扫描通过）
- [ ] 无 Critical/High CVE（Trivy 扫描通过）
- [ ] API 速率限制生效（Free 60/min）
- [ ] SQL 注入测试通过
- [ ] XSS 测试通过

### 2.4 可观测性

- [ ] Prometheus metrics 端点可访问（所有服务）
- [ ] Grafana Dashboard 展示关键 SLI
- [ ] Relay 离线告警正常触发
- [ ] API 错误率告警正常触发
- [ ] 分布式追踪在 Jaeger 可查看

---

## 三、测试验收

- [ ] `go test ./...` 全部通过
- [ ] `go test -tags=integration ./tests/integration/` 全部通过（20+ 用例）
- [ ] `npm run test:e2e` 全部通过
- [ ] 单元测试覆盖率 > 50%
- [ ] 负载测试 P95 < 50ms，错误率 < 0.1%

---

## 四、部署验收

- [ ] `docker compose up -d` 一键启动全栈
- [ ] 所有容器健康检查通过
- [ ] 数据库迁移自动执行
- [ ] 14 张 PG 表完整创建
- [ ] `helm install omnitun ./deploy/kubernetes/omnitun` 成功
- [ ] CI Pipeline 绿色通过

---

## 五、文档验收

- [ ] README.md 包含快速开始指南
- [ ] API 文档（OpenAPI 3.1 YAML）完整
- [ ] CLI 命令帮助完整（`omnitun --help`）
- [ ] Dashboard 帮助页 / 新手引导

---

## 六、投产签收

| 角色 | 签收项 | 签字 |
|------|--------|------|
| PM | 功能验收全部通过 | ☐ |
| 架构师 | 非功能验收全部通过 | ☐ |
| QA | 测试验收全部通过 | ☐ |
| SRE | 部署验收全部通过 | ☐ |
| 安全 | 安全验收全部通过 | ☐ |

---

**签收日期**：\_\_\_\_\_\_\_\_\_\_\_\_\_

**签收人**：\_\_\_\_\_\_\_\_\_\_\_\_\_
