# OmniTun 2.0 — 产品需求文档

## 一、隧道贯通（M1 — 最高优先级）

### 1.1 端到端 HTTP 隧道

**作为** 开发者
**我想要** 执行 `omnitun http 8080` 后立即获得一个公网可访问的 HTTPS URL
**以便** 我能从互联网访问我本地的开发服务

**验收标准**：
- [ ] `omnitun http 8080` → 控制台输出 `https://xxx.omnitun.io → http://localhost:8080`
- [ ] 浏览器访问 `https://xxx.omnitun.io` → 返回本地服务的响应
- [ ] WebSocket 升级请求正常工作
- [ ] 隧道断开后 Agent 自动重连，URL 保持不变
- [ ] Ctrl+C 优雅关闭隧道

### 1.2 Agent 连接管理

- [ ] Agent 启动后自动连接 WebSocket Gateway
- [ ] 30s 心跳，3 次超时判离线
- [ ] 断线指数退避重连（1s → 2s → 4s → ... → 60s cap）
- [ ] Agent 版本上报到控制面

### 1.3 Relay 运行时

- [ ] Relay 启动后自动注册到控制面
- [ ] 30s 心跳上报（活跃隧道数、连接数、带宽）
- [ ] 超过 90s 无心跳 → 控制面标记 offline → 该 Relay 的隧道迁移
- [ ] 优雅关闭：通知控制面 → 等待活跃连接排空 → 退出

---

## 二、TLS 与域名（M1）

### 2.1 自动 TLS (Let's Encrypt)

- [ ] 使用 `go-acme/lego` 库集成 ACME 客户端
- [ ] 支持 DNS-01 Challenge（首选）和 HTTP-01 Challenge（备选）
- [ ] 泛域名证书支持（`*.omnitun.io`）
- [ ] 证书到期前 30 天自动续期
- [ ] 证书存储到 S3 + PG certificates 表
- [ ] 证书推送到 Relay 节点（通过 NATS 或 gRPC）

### 2.2 自定义域名

- [ ] Dashboard + API 添加自定义域名
- [ ] DNS 验证流程：CNAME 或 TXT record
- [ ] 定期检查 DNS 解析状态（每 30s）
- [ ] 验证通过 → 自动签发证书
- [ ] 域名健康检查（Dashboard 显示状态）
- [ ] 同一域名可在多个隧道间迁移

---

## 三、企业安全（M2）

### 3.1 SSO/OIDC

- [ ] OIDC 通用协议支持（Discovery URL 自动发现）
- [ ] Okta / Azure AD / Google Workspace 三个 IdP 测试通过
- [ ] Just-in-Time Provisioning（首次 SSO 登录自动创建用户）
- [ ] SAML 2.0 支持

### 3.2 MFA 接入

- [ ] 登录时若用户启用 MFA → 返回 `mfa_required: true`
- [ ] 前端显示 MFA 验证码输入框
- [ ] TOTP 验证通过 → 签发 JWT
- [ ] MFA 注册/禁用已有代码，接入路由即可

### 3.3 RBAC 完整接入

- [ ] Owner / Admin / Editor / Viewer 四个角色的权限隔离
- [ ] Editor 可管理隧道但不能删除工作区
- [ ] Viewer 只读所有隧道和统计

### 3.4 API Key 认证

- [ ] `Authorization: Bearer ot_sk_xxx` 请求头验证
- [ ] `X-API-Key: ot_sk_xxx` 请求头验证
- [ ] API Key 支持作用域限制（只读/读写/管理员）
- [ ] API Key 支持 IP 白名单绑定
- [ ] API Key 过期自动失效

### 3.5 审计日志

- [ ] 所有写操作写入 audit_logs 表
- [ ] Dashboard 审计日志页面可搜索（按时间/操作/用户）
- [ ] 审计日志保留 12 个月

---

## 四、P2P 与 Mesh（M3）

### 4.1 NAT 穿透

- [ ] 自建 STUN 服务器（端口 3478）
- [ ] NAT 类型探测（Full Cone / Restricted / Port Restricted / Symmetric）
- [ ] UDP Hole Punching（打洞成功 → 直连）
- [ ] 自建 TURN 服务器（打洞失败 → 中继兜底）
- [ ] DERP（HTTPS 中继，端口 443，防火墙友好）

### 4.2 Mesh 组网

- [ ] 创建 Mesh 网络 → 生成 CIDR + 加密密钥
- [ ] Agent 通过邀请码加入网络
- [ ] 网络内 WireGuard 自动配置
- [ ] 自适应拓扑：P2P 直连 → TURN Relay → DERP 三级降级
- [ ] 网络内 DNS 解析（node-name.network → Mesh IP）

---

## 五、可观测性（M4）

### 5.1 监控

- [ ] Prometheus metrics 端点暴露全部关键指标
- [ ] Grafana Dashboard（JSON 模板）
- [ ] SLO 追踪 Dashboard

### 5.2 告警

- [ ] Relay 离线 > 2 分钟 → PagerDuty Warning
- [ ] API 错误率 > 1% → PagerDuty Critical
- [ ] 证书将过期（7 天内）→ Slack + Email
- [ ] 磁盘使用 > 80% → PagerDuty Warning

### 5.3 日志

- [ ] 所有服务结构化日志（JSON 格式）
- [ ] 日志包含 trace_id（支持分布式追踪）
- [ ] 日志通过 OpenTelemetry Collector 发送到 Loki

### 5.4 分布式追踪

- [ ] API Gateway → Orchestrator → Relay → Agent 全链路 trace
- [ ] Jaeger / Tempo 可查看 trace

---

## 六、计费系统（M5）

### 6.1 Stripe 集成

- [ ] Stripe Checkout（升级计划）
- [ ] Stripe Customer Portal（管理订阅）
- [ ] Webhook 处理（payment_succeeded / payment_failed / subscription_deleted）

### 6.2 用量计量

- [ ] 带宽统计（bytes_in + bytes_out）
- [ ] 隧道数统计
- [ ] 连接数统计
- [ ] 每小时聚合写入 usage_records 表

### 6.3 计划限制

- [ ] Free: 1 隧道 / 1GB / 月
- [ ] Pro: 10 隧道 / 100GB / 月
- [ ] Team: 50 隧道 / 500GB / 月
- [ ] Business: 无限 / 5TB / 月
- [ ] 超限时隧道停止创建，已有隧道不受影响

---

## 七、测试与验证（M6）

- [ ] 单元测试覆盖率 > 50%（当前 ~35%）
- [ ] 集成测试（真实 PG + Relay + Agent）> 20 个 case
- [ ] E2E 测试（Playwright）：注册→创建隧道→查看详情→删除隧道
- [ ] 负载测试：1000 并发隧道，单 Relay 1Gbps+
- [ ] 安全审计：CodeQL + Trivy 扫描通过

## 八、环境约束

当前开发环境为单机 Windows + Docker Desktop (WSL2)。以下功能在实现代码路径后，验证方式为本地 mock 测试，不要求真实外部服务：

| 功能 | 约束 | 验证方式 |
|------|------|----------|
| OIDC/SAML SSO | 无外部 IdP 账号 | 代码路径完整 + mock IdP server 测试 |
| ACME TLS 证书签发 | 无公网域名 | lego 客户端代码完整 + TLS 配置逻辑验证 |
| Stripe 计费 | 无 Stripe 测试密钥 | Stripe SDK 代码完整 + Webhook handler mock |
| 多区域部署 | 仅单机 | 配置模板准备 + 单机多 Relay 模拟 |
| NAT 穿透 (STUN/TURN) | 局域网内测试 | 同 Docker 网络模拟 |
