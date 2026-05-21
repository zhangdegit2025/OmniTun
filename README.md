# OmniTun

> 企业级内网穿透与私有网络平台 — 一行命令，让任何网络角落的服务都能被安全、即时、零配置地访问。

OmniTun 是一个面向企业的全协议内网穿透 SaaS 平台，融合 ngrok 的易用性、Cloudflare Tunnel 的安全性与 Tailscale 的 P2P 能力。支持 HTTP/gRPC/WebSocket/TCP/UDP/SSH 等全协议隧道，覆盖中继、P2P 直连、Mesh 组网等全拓扑场景。

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-00ADD8.svg)](https://go.dev)

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/omnitun/omnitun.git
cd omnitun

# 安装依赖
go mod download
cd web && npm install && cd ..

# 启动开发环境基础设施（PostgreSQL / ClickHouse / Valkey / NATS / MinIO）
docker compose -f deploy/docker/docker-compose.yml up -d

# 迁移数据库
make migrate-up

# 编译全部二进制
make build

# 运行测试
make test
```

## 项目结构

```
├── cmd/
│   ├── server/          # 控制面 API 服务入口
│   ├── client/          # 内网 Agent CLI 入口
│   ├── relay/           # 数据面中继节点入口
│   ├── admin/           # 管理后台 CLI
│   └── tools/           # 辅助工具（迁移等）
├── internal/
│   ├── auth/            # 认证服务（注册/登录/JWT/OAuth/MFA）
│   ├── tunnel/          # 隧道编排（CRUD/状态机/Relay 选择）
│   ├── relay/           # 数据面中继（QUIC/WS/HTTP 代理/StreamMux）
│   ├── gateway/         # WebSocket Gateway（Agent 连接管理）
│   ├── control/         # Agent 控制器（连接建立/心跳/重连）
│   ├── protocol/        # Vector Stream 帧协议
│   └── network/         # 网络拓扑与路由
├── pkg/
│   ├── config/          # 配置加载（Viper）
│   ├── log/             # 结构化日志（slog）
│   ├── errors/          # 统一错误处理
│   └── metrics/         # Prometheus 指标
├── web/                 # Dashboard 前端（React + TypeScript）
├── proto/               # Protobuf 定义
├── migrations/
│   ├── pg/              # PostgreSQL 迁移脚本
│   └── ck/              # ClickHouse 迁移脚本
├── deploy/
│   ├── docker/          # Docker Compose 部署配置
│   ├── kubernetes/      # K8s Helm Charts
│   └── terraform/       # 基础设施即代码
├── scripts/             # 构建与开发脚本
├── tests/               # 集成测试与烟雾测试
└── docs/                # 产品与技术文档
```

## 技术栈

| 层           | 技术                                      |
| ------------ | ----------------------------------------- |
| 后端语言     | Go 1.23+                                  |
| RPC 框架     | gRPC + Protobuf (buf)                     |
| 传输协议     | QUIC (quic-go)、HTTP、WebSocket           |
| 前端         | React 18 + TypeScript、Vite、Tailwind CSS |
| 主数据库     | PostgreSQL 16                             |
| 分析数据库   | ClickHouse 24                             |
| 缓存 / KV    | Valkey 8                                  |
| 消息队列     | NATS JetStream                            |
| 对象存储     | MinIO（兼容 S3）                          |
| 容器编排     | Docker Compose（开发）/ Kubernetes（生产） |
| 可观测性     | Prometheus + OpenTelemetry                |
| Go 代码生成  | sqlc（数据库查询）、buf（Protobuf）        |

## 开发指南

请阅读 [docs/development.md](./docs/development.md) 了解完整的开发者入职流程、环境配置、调试技巧和常见问题。

## 文档

| 文档                                                          | 内容                     |
| ------------------------------------------------------------- | ------------------------ |
| [00-overview](./docs/00-overview.md)                           | 文档导航与总览           |
| [01-product-vision](./docs/01-product-vision.md)               | 产品愿景与商业战略       |
| [02-market-analysis](./docs/02-market-analysis.md)             | 市场分析与用户研究       |
| [03-competitive-analysis](./docs/03-competitive-analysis.md)   | 竞品深度分析             |
| [04-product-requirements](./docs/04-product-requirements.md)   | 完整产品需求文档         |
| [05-technical-architecture](./docs/05-technical-architecture.md) | 技术架构设计             |
| [06-data-model](./docs/06-data-model.md)                       | 数据模型与存储设计       |
| [07-api-design](./docs/07-api-design.md)                       | API 与协议设计           |
| [08-security-model](./docs/08-security-model.md)               | 安全模型与合规           |
| [09-deployment](./docs/09-deployment.md)                       | 部署与运维方案           |
| [10-roadmap](./docs/10-roadmap.md)                             | 里程碑路线图             |
| [11-adversarial-review](./docs/11-adversarial-review.md)       | 安全对抗评审             |
| [development](./docs/development.md)                           | 开发者入职指南           |

## 路线图

参见 [docs/10-roadmap.md](./docs/10-roadmap.md)

- **2026 Q3**：Alpha 内测（HTTP Tunnel + CLI + Dashboard）
- **2026 Q4**：Public Beta（多租户 SaaS + 计费）
- **2027 Q1**：Mesh & P2P GA
- **2027 Q2**：Enterprise + 私有部署
- **2027 Q3**：全球多区域部署
- **2027 Q4**：生态建设（开源 + SDK + IDE 插件）

## License

[MIT](./LICENSE)
