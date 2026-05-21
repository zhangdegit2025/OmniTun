# 开发者入职指南

本文档帮助新开发者快速搭建 OmniTun 本地开发环境并熟悉项目工作流。

---

## 环境要求

| 工具              | 最低版本    | 说明                                   |
| ----------------- | ----------- | -------------------------------------- |
| Go                | 1.23+       | 后端开发语言                           |
| Node.js           | 20+         | 前端开发运行时                         |
| Docker            | 24+         | 容器运行时（包含 `docker compose`）     |
| Docker Compose    | v2+         | 用于启动开发环境基础设施               |
| golangci-lint     | 最新版      | Go 代码 Lint 工具                      |
| buf               | 最新版      | Protobuf 代码生成工具（可选）          |
| golang-migrate    | 最新版      | 数据库迁移工具（可选，也可用 make）    |

### 本地运行（不使用 Docker 的替代方案）

如需在本地直接运行数据库，需要额外安装：

- PostgreSQL 16
- ClickHouse 24
- Valkey 8（或 Redis 7+）
- NATS Server

推荐使用 Docker 启动基础设施，开发时仅后端/前端在本地运行。

---

## 5 分钟快速启动

```bash
# === 1. 创建 .env 文件 ===
# 如果 .env 不存在，从示例文件复制
cp .env.example .env

# === 2. 启动基础设施 ===
docker compose -f deploy/docker/docker-compose.yml up -d

# 所有服务健康检查通过后继续（约 30 秒）
docker compose -f deploy/docker/docker-compose.yml ps

# === 3. 运行数据库迁移 ===
make migrate-up

# === 4. 启动后端服务 ===
go run ./cmd/server/

# 后端默认监听 localhost:8080
# 看到 "server starting on :8080" 即表示启动成功

# === 5. 新开终端，启动前端 ===
cd web && npm run dev

# 前端默认监听 localhost:3000，自动代理 API 请求到后端

# === 6. 打开浏览器 ===
# http://localhost:3000
```

> **提示**：也可以使用 `scripts/dev-setup.sh` 一键启动基础设施。

---

## 项目结构详解

### cmd/ — 程序入口

```
cmd/
├── server/          # 控制面 API 服务
│   ├── main.go      # 服务启动入口：初始化 DB/配置/gRPC
│   └── auth/        # Auth 子命令入口
├── client/          # 内网 Agent CLI
│   └── main.go      # CLI 入口：omnitun http 8080 等子命令
├── relay/           # 数据面中继节点
│   └── main.go      # Relay 节点启动：QUIC/WS 监听
├── admin/           # 管理后台 CLI
│   └── main.go      # 管理命令入口
└── tools/           # 辅助工具
    ├── tools.go     # 工具模块声明
    └── migrate/     # 数据库迁移工具
```

### internal/ — 核心业务逻辑

```
internal/
├── auth/            # 认证服务
│                    # 注册 / 登录 / JWT 签发与验证 / OAuth 集成 / MFA TOTP
│                    # 密码重置 / Refresh Token 轮换 / API Key 管理
├── tunnel/          # 隧道编排层
│                    # CRUD / 生命周期状态机（创建→就绪→运行→关闭）
│                    # Relay 节点选择 / 流量路由 / 自定义域名绑定
│                    # 后端实现：pg/mysql/sqlite 查询生成 (sqlc)
├── relay/           # 数据面中继引擎
│                    # QUIC 多路复用 / WebSocket 代理 / HTTP 反向代理
│                    # StreamMux 流复用 / TLS 终止 / 流量转发
├── gateway/         # WebSocket Gateway
│                    # Agent 长连接管理 / 消息路由 / 连接池
├── control/         # Agent 控制器
│                    # 连接建立与握手 / 心跳保活 / 断线重连
│                    # 指令下发 / 状态同步
├── protocol/        # Vector Stream 帧协议
│                    # 自定义二进制帧格式定义 / 编解码
│                    # 流控制 / 错误帧处理
└── network/         # 网络拓扑与路由
                     # P2P 路径发现 / Mesh 拓扑管理 / 最优路径选择
```

### pkg/ — 公共库

```
pkg/
├── config/          # 配置加载（Viper）
│                    # 支持 YAML 文件 + 环境变量覆盖 + 默认值
├── log/             # 结构化日志（slog）
│                    # JSON 格式输出 + 日志级别控制 + 请求 ID 注入
├── errors/          # 统一错误处理
│                    # 错误码枚举 + 错误包装 + HTTP/gRPC 错误映射
└── metrics/         # Prometheus 指标
                     # Counter / Gauge / Histogram 封装
                     # HTTP 请求耗时 / 隧道数量 / 连接数 等指标
```

### web/ — Dashboard 前端

```
web/
├── src/
│   ├── pages/          # 页面组件
│   │   ├── Login.tsx       # 登录页
│   │   ├── Register.tsx    # 注册页
│   │   ├── Dashboard.tsx   # 首页仪表盘
│   │   ├── Tunnels.tsx     # 隧道列表页
│   │   ├── TunnelDetail.tsx# 隧道详情页
│   │   └── Settings.tsx    # 设置页
│   ├── components/     # 通用组件
│   │   ├── Layout.tsx      # 页面布局（侧边栏 + 顶栏）
│   │   └── ui/             # UI 原语（Button / Card / Table / Input / Dialog 等）
│   ├── hooks/          # 自定义 Hooks
│   │   ├── useAuth.ts      # 认证状态管理
│   │   ├── useTunnels.ts   # 隧道数据获取与操作
│   │   └── useWebSocket.ts # WebSocket 连接管理
│   ├── lib/            # 工具模块
│   │   ├── api.ts          # HTTP API 客户端封装
│   │   ├── websocket.ts    # WebSocket 客户端
│   │   ├── auth.ts         # 认证工具函数
│   │   ├── types.ts        # TypeScript 类型定义
│   │   └── utils.ts        # 通用工具函数
│   ├── store/          # Zustand 状态存储
│   │   ├── auth.ts         # 用户认证状态
│   │   └── tunnels.ts      # 隧道列表状态
│   ├── App.tsx         # 根组件（路由配置）
│   ├── main.tsx        # 应用入口
│   └── index.css       # 全局样式（Tailwind CSS）
├── package.json        # 依赖与脚本
├── vite.config.ts      # Vite 构建配置（含 API 代理）
├── tsconfig.json       # TypeScript 配置
├── tailwind.config.ts  # Tailwind 配置
└── postcss.config.js   # PostCSS 配置
```

### 其他目录

```
proto/              # Protobuf 定义文件
migrations/
├── pg/             # PostgreSQL 迁移脚本（按序号编号）
└── ck/             # ClickHouse 迁移脚本
tests/              # 集成测试 + 烟雾测试
deploy/
├── docker/         # Docker Compose + 多阶段 Dockerfile
├── kubernetes/     # K8s 部署清单（Namespace / ConfigMap / Secrets）
└── terraform/      # IaC 基础设施配置
scripts/            # 构建与开发辅助脚本
├── build.sh        # 构建全部二进制 + 前端
├── dev-setup.sh    # 一键启动开发环境
└── dev-teardown.sh # 停止并清理开发环境
```

---

## 开发工作流

### 编码前

```bash
# Go 代码 Lint 检查
make lint

# 前端代码 Lint 检查
cd web && npm run lint
```

### 单元测试

```bash
# Go 单元测试（含竞态检测 + 覆盖率报告）
make test

# 前端测试
cd web && npm run test

# 前端测试（监听模式，自动重跑）
cd web && npm run test:watch
```

### 构建

```bash
# 构建全部 Go 二进制（server / client / relay / admin）
make build

# 构建指定二进制
go build ./cmd/server/

# 构建前端
cd web && npm run build

# 一键构建全部（Go 二进制 + 前端）
bash scripts/build.sh
```

### 提交前全量检查

```bash
# 推荐在提交前运行全量检查
make lint && make test && make build

# 前端也记得检查
cd web && npm run lint && npm run test
```

---

## 常用命令

### CLI 本地调试

```bash
# 启动 HTTP 隧道（本地 3000 端口暴露到公网）
go run ./cmd/client/ http 3000

# 启动 TCP 隧道
go run ./cmd/client/ tcp 5432

# 指定自定义域名
go run ./cmd/client/ http 3000 --domain api.dev.example.com

# 指定 Relay 节点地址
go run ./cmd/client/ http 3000 --relay relay.omnitun.io:8443

# 查看详细输出
go run ./cmd/client/ http 3000 --verbose
```

### Protobuf 代码生成

```bash
# 生成 Go 和 TypeScript 的 Protobuf 代码
buf generate
```

### 数据库迁移

```bash
# 使用 make 命令执行迁移（推荐）
make migrate-up      # 升级到最新版本
make migrate-down    # 回滚一个版本

# 使用 golang-migrate 直接操作
migrate -path migrations/pg -database "$DATABASE_URL" up
migrate -path migrations/pg -database "$DATABASE_URL" down 1
migrate -path migrations/pg -database "$DATABASE_URL" version
```

### Docker 操作

```bash
# 构建 Server 镜像
make docker-build

# 构建 Client 镜像
make docker-build-client

# 构建 Relay 镜像
make docker-build-relay

# 直接使用 Docker 命令
docker build -f deploy/docker/Dockerfile.server -t omnitun-server .
docker build -f deploy/docker/Dockerfile.client -t omnitun-client .
docker build -f deploy/docker/Dockerfile.relay -t omnitun-relay .

# 启动 / 停止开发环境
bash scripts/dev-setup.sh
bash scripts/dev-teardown.sh

# 查看服务日志
docker compose -f deploy/docker/docker-compose.yml logs -f postgres
docker compose -f deploy/docker/docker-compose.yml logs -f clickhouse
```

### 代码生成

```bash
# 重新生成所有生成的代码（Protobuf + sqlc）
make generate
```

---

## 调试技巧

### 启用详细日志

```bash
# 方式一：环境变量
LOG_LEVEL=debug go run ./cmd/server/

# 方式二：修改 config.example.yaml 中的 log_level 字段
# log_level: "debug"

# 方式三：修改 .env 文件
# LOG_LEVEL=debug
```

### CLI 调试

```bash
# 使用 --verbose flag 查看 CLI 详细输出
go run ./cmd/client/ http 8080 --verbose

# 查看 Agent 连接状态
go run ./cmd/client/ status
```

### Relay 本地调试

Relay 支持使用自签名证书在本地进行测试：

```bash
# 生成自签名证书
openssl req -x509 -newkey rsa:4096 -keyout relay-key.pem -out relay-cert.pem -days 365 -nodes -subj "/CN=localhost"

# 启动 Relay 并指定证书
go run ./cmd/relay/ --cert relay-cert.pem --key relay-key.pem

# 启动 Client 并跳过证书验证（仅本地测试！）
go run ./cmd/client/ http 8080 --relay localhost:8443 --insecure
```

### 数据库调试

```bash
# 连接 PostgreSQL 查看数据
docker compose -f deploy/docker/docker-compose.yml exec postgres psql -U omnitun -d omnitun

# 查看隧道表
docker compose -f deploy/docker/docker-compose.yml exec postgres psql -U omnitun -d omnitun -c "SELECT id, name, status FROM tunnels LIMIT 10;"

# 连接 ClickHouse 查询分析数据
curl "http://localhost:8123/?query=SHOW+TABLES"

# 查看 NATS 监控面板
# 浏览器访问 http://localhost:8222
```

### 前端调试

```bash
# Vite 开发服务器会在浏览器控制台输出源码位置
# 使用 React DevTools 浏览器扩展调试组件状态

# 查看 API 代理是否正常
curl -s http://localhost:3000/v1/health

# 模拟 API 延迟
# 在后端代码中设置断点，使用 dlv 调试
```

### 使用 Delve 调试 Go 代码

```bash
# 安装 dlv
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试 Server
dlv debug ./cmd/server/

# 在 main 函数设置断点
(dlv) break main.main
(dlv) continue

# 调试 Client
dlv debug ./cmd/client/ -- http 8080
```

---

## 常见问题

### Q: `go build` 报 "missing go.sum entry" 怎么办？

A: 运行 `go mod tidy` 同步模块依赖和校验和。如果仍有问题，尝试删除 `go.sum` 然后重新 `go mod tidy`。

### Q: `make lint` 报 "golangci-lint: command not found"

A: 安装 golangci-lint：
```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# 或使用 go install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Q: `npm run dev` 前端页面请求 API 报 502 错误？

A: 确保后端 server 在 8080 端口运行。前端 Vite 配置了代理 `/v1` → `http://localhost:8080`，API 502 说明后端未启动或端口被占用。

```bash
# 检查 8080 端口是否被占用
lsof -i :8080           # macOS
netstat -tlnp | grep 8080  # Linux
netstat -ano | findstr 8080 # Windows

# 确认后端已启动
curl http://localhost:8080/v1/health
```

### Q: Docker Compose 启动失败？

A: 按以下步骤排查：

1. 检查 `.env` 文件是否存在：
   ```bash
   # 如果不存在，从示例文件复制
   cp .env.example .env
   ```

2. 检查 Docker 守护进程是否运行：
   ```bash
   docker info
   ```

3. 检查端口冲突（默认端口：5432 PostgreSQL / 9000 MinIO / 8123 ClickHouse / 6379 Valkey / 4222 NATS）：
   ```bash
   docker compose -f deploy/docker/docker-compose.yml ps
   ```

4. 清理旧容器和卷后重试：
   ```bash
   docker compose -f deploy/docker/docker-compose.yml down --volumes --remove-orphans
   docker compose -f deploy/docker/docker-compose.yml up -d
   ```

### Q: `make migrate-up` 失败？

A: 确保 PostgreSQL 已启动且 `.env` 中的 `DATABASE_URL` 配置正确：

```bash
# 检查 PostgreSQL 是否运行
docker compose -f deploy/docker/docker-compose.yml ps postgres

# 测试数据库连接
docker compose -f deploy/docker/docker-compose.yml exec postgres pg_isready -U omnitun
```

如果迁移已经部分执行但中断，可以强制覆盖：
```bash
migrate -path migrations/pg -database "$DATABASE_URL" force <version>
migrate -path migrations/pg -database "$DATABASE_URL" up
```

### Q: `buf generate` 报错？

A: 确保 `buf` CLI 已安装：

```bash
# macOS
brew install bufbuild/buf/buf

# 其他方式
go install github.com/bufbuild/buf/cmd/buf@latest
```

### Q: 前端热更新不生效？

A: 检查 `web/node_modules` 是否完整：

```bash
cd web
rm -rf node_modules package-lock.json
npm install
npm run dev
```

如果修改 `vite.config.ts` 或 `tailwind.config.ts` 后不生效，重启 Vite 开发服务器。

### Q: 如何贡献代码？

A: 请阅读 [CONTRIBUTING.md](../CONTRIBUTING.md) 了解代码风格、Commit 规范和 PR 流程。

---

## 服务端口速查

| 服务          | 端口  | 说明                   |
| ------------- | ----- | ---------------------- |
| Server API    | 8080  | 控制面 HTTP/gRPC API   |
| Dashboard     | 3000  | 前端开发服务器         |
| PostgreSQL    | 5432  | 主数据库               |
| ClickHouse    | 8123  | 分析数据库 HTTP 接口   |
| Valkey        | 6379  | 缓存 / KV 存储         |
| NATS          | 4222  | 消息队列 Client 端口   |
| NATS 监控     | 8222  | NATS 监控 Dashboard    |
| MinIO API     | 9000  | 对象存储 S3 API        |
| MinIO Console | 9001  | 对象存储 Web 控制台    |
| Relay QUIC    | 8443  | 数据面中继端口         |
