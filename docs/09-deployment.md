# OmniTun — 部署与运维方案

<!-- 修订说明：
  2026-05-20:
  - 问题 D: 修正 Phase 1 部署拓扑（PostgreSQL + ClickHouse 高可用配置）
  - 问题 E: 新增六、故障转移设计章节
  - 问题 F: 完善 5.3 SLO 定义（增加隧道持续在线率）
-->

## 一、部署拓扑

### 1.1 全球多区域架构

```
                        ┌──────────────┐
                        │  Global DNS   │  (Anycast / Route53 / GeoDNS)
                        └──────┬───────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
  ┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼─────┐
  │  APAC-1   │          │  US-1     │          │  EU-1     │
  │  Singapore│          │  Virginia │          │ Frankfurt │
  │           │          │           │          │           │
  │ ┌───────┐ │          │ ┌───────┐ │          │ ┌───────┐ │
  │ │Relay x2│ │          │ │Relay x2│ │          │ │Relay x2│ │
  │ └───────┘ │          │ └───────┘ │          │ └───────┘ │
  │ ┌───────┐ │          │ ┌───────┐ │          │ ┌───────┐ │
  │ │Control│ │  ◀gRPC──▶│ │Control│ │  ◀gRPC──▶│ │Control│ │
  │ │Plane  │ │          │ │Plane  │ │          │ │Plane  │ │
  │ └───────┘ │          │ └───────┘ │          │ └───────┘ │
  └─────┬─────┘          └─────┬─────┘          └─────┬─────┘
        │                      │                      │
        └──────────────────────┼──────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   Global Database    │
                    │  Primary (US)        │
                    │  ┌──────┐ ┌──────┐  │
                    │  │PG R/W│ │CK R/W│  │
                    │  └──┬───┘ └──┬───┘  │
                    └─────┼────────┼──────┘
                          │        │
              ┌───────────▼──┐ ┌───▼───────────┐
              │ Read Replica │ │ Read Replica   │
              │ (APAC)       │ │ (EU)           │
              └──────────────┘ └────────────────┘
```

### 1.2 区域部署规格

| 区域 | 云厂商 | 控制面副本 | Relay 副本 | 数据库 |
|------|--------|------------|------------|--------|
| **us-east-1** (弗吉尼亚) | AWS | 3 | 3 | PG Primary + CK Primary |
| **ap-southeast-1** (新加坡) | AWS | 3 | 3 | PG Read Replica |
| **eu-central-1** (法兰克福) | AWS | 3 | 3 | PG Read Replica |
| **ap-northeast-1** (东京) | AWS | — | 2 | — |
| **ap-east-1** (香港) | AWS / 阿里云 | — | 2 | — |
| **cn-north-1** (北京) | 阿里云 | 3（独立部署） | 3 | PG Primary (独立) |

### 1.3 初始阶段简化部署

Phase 1 阶段使用单区域 + 多 Relay 方式，降低运维复杂度：

```
Phase 1:
  AWS ap-southeast-1 × 1
    ├── K8s Cluster
    │   ├── Control Plane × 2 (pod anti-affinity)
    │   ├── Relay × 2
    │   ├── PostgreSQL Primary + Hot Standby (Patroni, 同步复制)
    │   ├── Valkey Sentinel Cluster (3 节点)
    │   └── ClickHouse 2 副本（ReplicatedMergeTree）
    └── S3 Compatible (MinIO, 多副本)

Phase 2:
  → 添加 us-east-1 区域（DB read replica + Relay）
  → 添加 eu-central-1（Relay only）

Phase 3:
  → 多区域完整 Control Plane
  → PG Primary-Standby 自动故障转移
  → 中国区独立部署
```

---

## 二、Kubernetes 部署

### 2.1 命名空间规划

| Namespace | 内容 |
|-----------|------|
| `omnitun-control` | API Gateway, Auth, Orchestrator, RBAC, Cert Manager, Billing |
| `omnitun-relay` | Relay 节点 DaemonSet |
| `omnitun-data` | PostgreSQL, Valkey, ClickHouse, MinIO |
| `omnitun-observability` | Prometheus, Grafana, Loki, Tempo, Mimir |
| `omnitun-ingress` | Envoy / Contour |

### 2.2 控制面 Deployment 示例

```yaml
# deploy/kubernetes/control/tunnel-orchestrator.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tunnel-orchestrator
  namespace: omnitun-control
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: tunnel-orchestrator
  template:
    metadata:
      labels:
        app: tunnel-orchestrator
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: tunnel-orchestrator
              topologyKey: kubernetes.io/hostname
      containers:
        - name: orchestrator
          image: ghcr.io/omnitun/tunnel-orchestrator:v1.0.0
          ports:
            - containerPort: 9002
              name: grpc
            - containerPort: 9090
              name: metrics
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: omnitun-secrets
                  key: database-url
            - name: NATS_URL
              value: "nats://nats.omnitun-data:4222"
            - name: VALKEY_URL
              value: "valkey.omnitun-data:6379"
          resources:
            requests:
              cpu: 500m
              memory: 256Mi
            limits:
              cpu: 2000m
              memory: 1Gi
          livenessProbe:
            grpc:
              port: 9002
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            grpc:
              port: 9002
            initialDelaySeconds: 5
            periodSeconds: 10
```

### 2.3 Relay DaemonSet

```yaml
# deploy/kubernetes/relay/relay-daemonset.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: omnitun-relay
  namespace: omnitun-relay
spec:
  selector:
    matchLabels:
      app: omnitun-relay
  template:
    metadata:
      labels:
        app: omnitun-relay
    spec:
      hostNetwork: true  # 直接使用主机网络，减少一层 NAT
      dnsPolicy: ClusterFirstWithHostNet
      containers:
        - name: relay
          image: ghcr.io/omnitun/relay:v1.0.0
          ports:
            - containerPort: 443
              name: https
              protocol: TCP
            - containerPort: 443
              name: quic
              protocol: UDP
            - containerPort: 3478
              name: stun
              protocol: UDP
            - containerPort: 9090
              name: metrics
          securityContext:
            capabilities:
              add: ["NET_BIND_SERVICE"]
          env:
            - name: RELAY_REGION
              value: "ap-southeast-1"
          resources:
            requests:
              cpu: 2000m
              memory: 2Gi
            limits:
              cpu: 8000m
              memory: 8Gi
```

---

## 三、Docker Compose（开发/小型部署）

```yaml
# deploy/docker/docker-compose.yml
version: "3.9"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: omnitun
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: omnitun
    volumes:
      - pg_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  valkey:
    image: valkey/valkey:8-alpine
    ports:
      - "6379:6379"

  nats:
    image: nats:2-alpine
    command: -js
    ports:
      - "4222:4222"

  clickhouse:
    image: clickhouse/clickhouse-server:24-alpine
    ports:
      - "8123:8123"
      - "9000:9000"

  minio:
    image: minio/minio
    command: server /data --console-address :9001
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: ${MINIO_PASSWORD}
    volumes:
      - minio_data:/data
    ports:
      - "9000:9000"
      - "9001:9001"

  api-gateway:
    image: ghcr.io/omnitun/api-gateway:v1.0.0
    depends_on: [postgres, valkey]
    ports:
      - "443:443"
    environment:
      DATABASE_URL: postgres://omnitun:${DB_PASSWORD}@postgres:5432/omnitun
      VALKEY_URL: valkey:6379
      NATS_URL: nats:4222

  orchestrator:
    image: ghcr.io/omnitun/tunnel-orchestrator:v1.0.0
    depends_on: [postgres, nats]
    environment:
      DATABASE_URL: postgres://omnitun:${DB_PASSWORD}@postgres:5432/omnitun
      NATS_URL: nats:4222

  relay:
    image: ghcr.io/omnitun/relay:v1.0.0
    ports:
      - "4443:443"
      - "4443:443/udp"
    environment:
      RELAY_REGION: local
      CONTROL_URL: api-gateway:9002

  web:
    image: ghcr.io/omnitun/web:v1.0.0
    ports:
      - "3000:3000"
    environment:
      API_URL: https://localhost

volumes:
  pg_data:
  minio_data:
```

---

## 四、CI/CD 流程

### 4.1 流水线

```
Git Push
  │
  ▼
GitHub Actions
  │
  ├──▶ Lint (golangci-lint, eslint)
  ├──▶ Test (go test, vitest)
  ├──▶ SAST (CodeQL, Semgrep)
  ├──▶ Build Images (ko / Docker)
  │     ├──▶ ghcr.io/omnitun/api-gateway:sha-xxx
  │     ├──▶ ghcr.io/omnitun/orchestrator:sha-xxx
  │     ├──▶ ghcr.io/omnitun/relay:sha-xxx
  │     └──▶ ghcr.io/omnitun/web:sha-xxx
  ├──▶ Scan Images (Trivy)
  │
  ▼
Deploy to Staging (Auto on main)
  │
  ▼
E2E Tests (Playwright + tunnel smoke test)
  │
  ▼
Deploy to Production (Manual approval)
  │
  ▼
ArgoCD Sync → K8s Rolling Update
```

### 4.2 构建工具选择

- **Go 服务**：使用 `ko` 构建（无 Dockerfile，自动生成最小镜像）
- **前端**：Vite build → Nginx Alpine static image
- **镜像仓库**：GitHub Container Registry (`ghcr.io/omnitun`)
- **部署方式**：ArgoCD (GitOps)

### 4.3 发布策略

| 类型 | 策略 | 频率 |
|------|------|------|
| Canary | 新版本先部署到 10% Relay，观察 1 小时 | 每次 release |
| Blue/Green | 控制面服务切换（通过 K8s Service selector） | 每次 release |
| Agent 更新 | 自动更新通道（stable / beta），用户可选降级 | 按需 |

---

## 五、可观测性

### 5.1 三层可观测性

| 层 | 工具 | 数据 |
|-----|------|------|
| **Metrics** | Prometheus + Mimir | 系统指标、业务指标、SLO |
| **Logging** | OpenTelemetry → Loki | 结构化日志（JSON） |
| **Tracing** | OpenTelemetry → Tempo | 分布式追踪（跨服务、跨 Relay） |

### 5.2 关键 Metrics

**业务指标**：
```
omnitun_tunnels_active              # 活跃隧道数
omnitun_tunnels_created_total       # 累计创建隧道数
omnitun_traffic_bytes_total         # 累计流量字节
omnitun_connections_active          # 活跃连接数
omnitun_p2p_success_rate            # P2P 打洞成功率
omnitun_tunnel_start_duration_seconds # 隧道建立耗时
```

**系统指标**：
```
omnitun_api_request_duration_seconds  # API 响应时间
omnitun_api_requests_total            # API 请求总量
omnitun_relay_goroutines              # Relay goroutine 数
omnitun_relay_memory_bytes            # Relay 内存占用
```

### 5.3 SLO 定义

| SLI | SLO | 测量窗口 | 说明 |
|-----|-----|----------|------|
| 控制面 API 可用性 | 99.9% | 30 天滚动 | 任何 API 返回 5xx |
| 隧道建立成功率 | 99.5% | 7 天滚动 | Agent 成功建立隧道 |
| 隧道持续在线率 | 99.95% | 7 天滚动 | 活跃隧道不中断 |
| Relay 转发延迟 P99 | < 50ms | 1 天滚动 | 同区域 Relay |
| P2P 打洞成功率 | > 90% | 7 天滚动 | 非对称 NAT 以上 |

### 5.4 告警规则

| 告警 | 条件 | 严重性 | 通知渠道 |
|------|------|--------|----------|
| API 错误率 > 1% | 5 分钟窗口 | Critical | PagerDuty |
| Relay 节点下线 | 心跳丢失 90s | Warning | PagerDuty |
| 隧道建立延迟 P99 > 5s | 5 分钟窗口 | Warning | Slack |
| P2P 成功率 < 80% | 1 小时窗口 | Info | Slack |
| 磁盘使用 > 80% | 即时 | Warning | PagerDuty |
| 证书即将过期 | 7 天内 | Warning | Slack + Email |

---

## 六、故障转移设计

### 6.1 Relay 故障转移

| 场景 | 处理方式 | RTO |
|------|----------|-----|
| 单 Relay 节点故障 | Agent 检测心跳丢失（90s）后自动重连到同区域另一 Relay | < 2min |
| 区域 Relay 全灭 | Agent 重连到最近可用区域 Relay | < 5min |
| 隧道状态 | Relay 故障时 Tunnel Orchestrator 标记为 error，Agent 重连后自动恢复 | — |

### 6.2 数据库故障转移

| 场景 | 处理方式 | RTO |
|------|----------|-----|
| PG Primary 故障 | Patroni 自动选出新 Primary | < 5min |
| PG 数据不一致 | 从 Standby 重建或从 S3 恢复 | < 30min |
| ClickHouse 副本失效 | 副本自动从另一副本同步数据 | < 10min |

---

## 七、灾备与恢复

### 6.1 备份策略

| 数据 | 频率 | 保留 | 存储 |
|------|------|------|------|
| PostgreSQL | 每小时增量 + 每日全量 | 30 天 | S3 (跨区域) |
| ClickHouse | 每日全量 | 7 天 | S3 |
| Valkey RDB | 每 6 小时 | 3 天 | S3 |
| 对象存储 | 持续同步 | 90 天 | 跨区域复制 |
| K8s 资源 | Git (ArgoCD) | 永久 | GitHub |

### 6.2 故障转移

| 故障类型 | RTO | RPO | 切换方式 |
|----------|-----|-----|----------|
| 单 Pod 故障 | < 30s | 0 | K8s 自动调度 |
| 单节点故障 | < 2min | 0 | K8s 自动驱逐 |
| 单 AZ 故障 | < 5min | ~1s | 跨 AZ 自动切换 |
| 单区域故障 | < 15min | < 5min | DNS 切换 + DB Failover |
| PG 主库故障 | < 5min | < 1min | Patroni 自动 failover |
