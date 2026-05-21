# OmniTun 3.0 — 开发者平台

<!--
  修订说明：
  2026-05-21:
  - 初始版本：定义面向开发者的 CLI / SDK / CI/CD / 框架插件完整产品需求
  - 当前基线 CLI：
    cmd/client/cmd/root.go: login, logout, http, tcp, tunnel (list/start/stop/delete)
    cmd/client/cmd/colors.go: ANSI 颜色函数 (green/red/yellow/cyan/dim/bold)
    cmd/client/cmd/http.go: HTTP 隧道全链路 (create → start → establish → forward)
    --json / --verbose 全局 flag 已定义，--json 尚未实际生效
-->

---

## 一、CLI 工具增强

### 1.1 当前基线

```
omnitun
├── login                     # 邮箱 + 密码认证
├── logout                    # 清除本地会话
├── http <port>               # HTTP 隧道 (--domain, --auth)
├── tcp <port>                # TCP 隧道
└── tunnel                    # 隧道管理
    ├── list                  # 列表展示 (ID / Status / Protocol / Address)
    ├── start <id>            # 启动隧道
    ├── stop <id>             # 停止隧道
    └── delete <id>           # 删除隧道 (带确认)
```

全局 Flag：`--api-url`, `--json`, `--verbose`
颜色输出：green ✓, red ✗, yellow !, cyan URL, dim →, bold 文本

### 1.2 命令树终态

```
omnitun
├── login                     # 邮箱 + 密码认证（已有）
├── logout                    # 清除本地会话（已有）
├── version                   # 显示版本 + 构建信息
├── update                    # 自动检查更新 + 下载
├── config                    # CLI 配置管理
│   ├── get <key>             # 读取配置
│   ├── set <key> <value>     # 写入配置
│   ├── list                  # 列出所有配置
│   └── unset <key>           # 删除配置
├── completion <shell>        # Shell 自动补全
├── status                    # 系统状态概览
├── http <port>               # HTTP 隧道（已有，增强）
├── tcp <port>                # TCP 隧道（已有，增强）
├── tunnel                    # 隧道管理
│   ├── list                  # 列表展示（已有，增强）
│   ├── create                # 创建隧道（非启动）
│   ├── start <id>            # 启动隧道（已有）
│   ├── stop <id>             # 停止隧道（已有）
│   ├── restart <id>          # 重启隧道
│   ├── delete <id>           # 删除隧道（已有）
│   ├── clone <id>            # 克隆隧道配置
│   ├── tags <id>             # 管理标签
│   └── logs <id>             # 实时流量日志
├── inspect <tunnel>          # 交互式请求检查器
└── network                   # Mesh 网络管理
    ├── list                  # 列出网络
    ├── create <name>         # 创建网络
    ├── join <network>        # 加入网络
    ├── leave <network>       # 离开网络
    └── status <network>      # 网络内节点状态
```

### 1.3 `omnitun status` — 系统状态概览

```
$ omnitun status

OmniTun v2.1.0 (build: abc1234, go1.22.0)
Session: alice@example.com (org: my-company)
─────────────────────────────────────────────
  Active Tunnels    ● 3 active
  Total Traffic     ↓ 1.2 GB / ↑ 340 MB
  Plan              Pro (5 GB bandwidth, 50 tunnels)
  Agent Version     2.1.0 (latest)
  API Latency       23ms (https://api.omnitun.io)
─────────────────────────────────────────────
Tunnels:
  my-api         ● active   https://my-api.omnitun.io
  dev-frontend   ● active   https://dev.omnitun.io
  staging-db     ◐ stopped  tcp://staging.omnitun.io:5432
```

`--json` 输出：

```json
{
  "version": "2.1.0",
  "build": "abc1234",
  "go_version": "go1.22.0",
  "user": {"email": "alice@example.com", "org": "my-company"},
  "tunnels": {
    "active": 3,
    "total": 3
  },
  "traffic": {"in_bytes": 1288490188, "out_bytes": 356515840},
  "plan": {"name": "Pro", "bandwidth_limit": 5368709120, "tunnel_limit": 50},
  "api_latency_ms": 23,
  "tunnel_list": [
    {"name": "my-api", "status": "active", "url": "https://my-api.omnitun.io"},
    {"name": "dev-frontend", "status": "active", "url": "https://dev.omnitun.io"},
    {"name": "staging-db", "status": "stopped", "url": "tcp://staging.omnitun.io:5432"}
  ]
}
```

API 调用：`GET /v1/status` (聚合端点，一次请求返回所有概览数据)

### 1.4 `omnitun logs <tunnel>` — 实时 Tail 流量日志

```bash
$ omnitun logs my-api --follow

11:05:32.145  POST /api/users     201 Created   45ms   203.0.113.42
11:05:32.103  GET  /api/health     200 OK       12ms   198.51.100.7
11:05:31.987  GET  /               304 Not Mod  8ms    198.51.100.15
11:05:31.456  POST /api/users     422 Valid.    23ms   203.0.113.42
11:05:30.001  GET  /api/items      200 OK       56ms   192.0.2.88
```

选项：

| Flag | 描述 | 默认值 |
|------|------|--------|
| `--follow`, `-f` | 持续输出（类似 `tail -f`） | `false` |
| `--lines`, `-n` | 显示最近 N 条 | 50 |
| `--filter-method` | 按 HTTP Method 过滤 | 全部 |
| `--filter-status` | 按 Status Code 过滤 | 全部 |
| `--filter-path` | 按 Path 正则过滤 | 全部 |
| `--output`, `-o` | 输出格式：`text` / `json` | `text` |

颜色编码：
- 2xx: 绿色
- 3xx: 蓝色
- 4xx: 黄色
- 5xx: 红色

实现：WebSocket 订阅隧道流量流 `WS /v1/tunnels/:id/logs`。

### 1.5 `omnitun inspect <tunnel>` — 交互式请求检查器

```bash
$ omnitun inspect my-api

Connected to my-api (https://my-api.omnitun.io). Press ? for help.

──────────────────────────────────────────────────────────────────────
#42  POST /api/users  201  45ms   203.0.113.42
──────────────────────────────────────────────────────────────────────
Request:
  POST /api/users HTTP/1.1
  Host: my-api.omnitun.io
  Content-Type: application/json
  Authorization: Bearer eyJ***
  User-Agent: curl/8.0.0
  X-Forwarded-For: 203.0.113.42

  {"username": "alice", "email": "alice@example.com"}

Response:
  HTTP/1.1 201 Created
  Content-Type: application/json

  {"id": "usr_abc123", "username": "alice"}

[↑/↓] navigate  [r] replay  [c] copy  [f] filter  [q] quit
```

交互命令：

| 按键 | 功能 |
|------|------|
| `↑` / `↓` | 浏览请求历史 |
| `r` | 重放当前请求（进入编辑模式） |
| `c` | 复制请求或响应到剪贴板 |
| `f` | 设置过滤器（正则匹配 Method/Path） |
| `p` | 暂停/恢复实时流 |
| `q` | 退出 |
| `h` / `?` | 显示帮助 |

实现：TUI (Terminal UI) 使用 `bubbletea` (Go) 库构建交互式界面。

### 1.6 `omnitun network` — Mesh 网络管理

```bash
$ omnitun network list
  Name              Nodes   CIDR              Status
  production-mesh   5       10.77.0.0/16      ● healthy
  staging-mesh      2       10.78.0.0/16      ● healthy

$ omnitun network create my-mesh --cidr 10.100.0.0/16
→ Creating network my-mesh ...
✓ Network created: net_abc123

$ omnitun network join my-mesh
→ Joining network my-mesh ...
✓ Joined network my-mesh. Assigned IP: 10.100.0.3

$ omnitun network status my-mesh
Nodes in my-mesh:
  node-01.local   10.100.0.1   ● active    pk_abc...
  node-02.local   10.100.0.2   ● active    pk_def...
  this-node       10.100.0.3   ● active    (you)

$ omnitun network leave my-mesh
! Are you sure you want to leave my-mesh? [y/N]: y
→ Leaving network my-mesh ...
✓ Left network my-mesh
```

### 1.7 `omnitun config` — CLI 配置管理

```bash
$ omnitun config list
  api-url        https://api.omnitun.io
  region         us-east-1
  log-level      info
  agent-id       (auto-generated)

$ omnitun config set region eu-west-1
✓ region set to eu-west-1

$ omnitun config set log-level debug
✓ log-level set to debug

$ omnitun config get api-url
https://api.omnitun.io

$ omnitun config unset region
✓ region removed (using default: us-east-1)
```

配置文件路径与格式：

| 平台 | 路径 |
|------|------|
| Linux | `~/.config/omnitun/config.yaml` |
| macOS | `~/Library/Application Support/omnitun/config.yaml` |
| Windows | `%APPDATA%\omnitun\config.yaml` |

支持的配置项：

| Key | 描述 | 可选值 | 默认值 |
|-----|------|--------|--------|
| `api-url` | API 基础 URL | 任意 HTTPS URL | `https://api.omnitun.io` |
| `region` | 默认 Relay 区域 | `us-east-1`, `eu-west-1`, `ap-southeast-1` 等 | `auto` |
| `log-level` | 日志级别 | `debug`, `info`, `warn`, `error` | `info` |
| `agent-id` | Agent 标识符 | 自定义字符串 | 自动生成 |
| `default-protocol` | 默认隧道协议 | `http`, `tcp`, `https` | `http` |
| `metrics-port` | 本地 metrics 端口 | 1–65535 | `0` (禁用) |

### 1.8 `omnitun completion` — Shell 自动补全

```bash
# bash
$ source <(omnitun completion bash)

# zsh
$ source <(omnitun completion zsh)

# fish
$ omnitun completion fish > ~/.config/fish/completions/omnitun.fish
```

使用 Cobra 内置的 `GenBashCompletion` / `GenZshCompletion` / `GenFishCompletion` 生成。

### 1.9 `omnitun update` — 自动检查更新

```bash
$ omnitun update check
  Current: v2.1.0
  Latest:  v2.1.1 (2026-05-20)
  Changes:
    - Fixed WebSocket reconnection race condition
    - Improved TLS handshake performance
    - Added --json output to tunnel list

  Download: https://omnitun.io/downloads/v2.1.1/windows-amd64/omnitun.exe

$ omnitun update install
  → Downloading v2.1.1 (14.2 MB) ...
  [████████████████████████████████████] 100%
  ✓ Downloaded. Replacing omnitun.exe ...
  ✓ Updated to v2.1.1
```

实现：
- `GET /v1/releases/latest` 获取最新版本信息
- 下载二进制（支持断点续传）
- 验证 SHA256 校验和
- 替换当前二进制（Windows 需通过临时脚本 `update.bat` 实现）

### 1.10 `omnitun version` — 版本信息

```bash
$ omnitun version
OmniTun CLI v2.1.0
  Build:      abc1234
  Go Version: go1.22.0
  Platform:   windows/amd64
  Built:      2026-05-20T10:30:00Z

$ omnitun version --json
{"version":"2.1.0","build":"abc1234","go_version":"go1.22.0","platform":"windows/amd64","built":"2026-05-20T10:30:00Z"}
```

### 1.11 颜色输出规范（增强）

现有颜色函数 (`cmd/client/cmd/colors.go`) 扩展：

```go
func green(s string) string   { return "\033[32m" + s + "\033[0m" }   // 成功 / Active
func red(s string) string     { return "\033[31m" + s + "\033[0m" }   // 错误 / 断线
func yellow(s string) string  { return "\033[33m" + s + "\033[0m" }   // 警告 / Degraded
func cyan(s string) string    { return "\033[36m" + s + "\033[0m" }   // URL / 信息
func dim(s string) string     { return "\033[2m" + s + "\033[0m" }    // 辅助文本
func bold(s string) string    { return "\033[1m" + s + "\033[0m" }    // 标题 / 关键信息
func blue(s string) string    { return "\033[34m" + s + "\033[0m" }   // 3xx 状态码
func gray(s string) string    { return "\033[90m" + s + "\033[0m" }   // 已停止隧道
```

状态图标规范：

| 状态 | 图标 | 颜色 |
|------|------|------|
| 成功 / Active | `✓` | 绿色 |
| 错误 / Error | `✗` | 红色 |
| 警告 / 进行中 | `!` / `→` | 黄色 |
| 信息 / URL | `●` / `◐` | 青色 |
| 帮助 / 说明 | `ℹ` | 灰色 |

### 1.12 `--json` 输出规范

所有命令必须支持 `--json` flag，输出机器可读的 JSON：

```bash
# 所有命令均支持
$ omnitun tunnel list --json                   # JSON 数组
$ omnitun http 8080 --json                     # 创建后输出隧道 JSON
$ omnitun status --json                        # 状态 JSON
$ omnitun logs my-api --json --lines 10        # JSON Lines (每行一个对象)
$ omnitun inspect my-api                       # 交互式，不支持 --json
```

`--json` 模式下，所有 ANSI 颜色代码禁用，stderr 输出不受影响。

### 1.13 进度条

长操作（下载、连接等待）显示 spinner 或进度条：

```bash
$ omnitun http 8080
→ Creating tunnel ...    ✓  (tun_abc123)
→ Connecting to relay ... [⠋]  (2s elapsed)
✓ Tunnel ready: https://xxxx.omnitun.io
```

```bash
$ omnitun update install
→ Downloading v2.1.1 (14.2 MB) ...
[████████░░░░░░░░░░░░░░░░] 42%  6.0 MB / 14.2 MB  12 MB/s  0:01 remaining
```

使用 `pterm` 或 `bubbletea` 实现 spinner 和进度条组件。`--json` 模式下进度条关闭。

---

## 二、多语言 SDK

### 2.1 Go SDK

**包路径**：`github.com/omnitun/omnitun-go`

```go
package main

import (
    "context"
    "fmt"
    "log"

    omnitun "github.com/omnitun/omnitun-go"
)

func main() {
    client := omnitun.NewClient(omnitun.WithToken("ot_sk_xxx"))

    // 创建隧道
    tunnel, err := client.Tunnels.Create(context.Background(), &omnitun.CreateTunnelParams{
        Name:      "my-api",
        Protocol:  omnitun.ProtocolHTTP,
        LocalPort: 8080,
        Domain:    "my-api.omnitun.io",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created: %s → %s\n", tunnel.PublicURL, tunnel.LocalAddr)

    // 启动隧道
    if err := client.Tunnels.Start(context.Background(), tunnel.ID); err != nil {
        log.Fatal(err)
    }

    // 列出隧道
    tunnels, err := client.Tunnels.List(context.Background(), &omnitun.ListTunnelsParams{
        Status: omnitun.TunnelStatusActive,
    })

    // 实时日志
    logCh, errCh := client.Tunnels.Logs(context.Background(), tunnel.ID, &omnitun.LogsParams{
        Follow: true,
    })
    for entry := range logCh {
        fmt.Printf("[%s] %s %s %d %dms\n",
            entry.Time, entry.Method, entry.Path, entry.Status, entry.Duration)
    }

    // Webhook 验证
    valid := client.Webhooks.Verify(payload, signature, "whsec_xxx")
}
```

**SDK 包结构**：

```
omnitun-go/
├── client.go              # Client struct, NewClient, Options
├── tunnels.go             # Tunnels service
│   ├── Create
│   ├── Get
│   ├── List
│   ├── Update
│   ├── Delete
│   ├── Start
│   ├── Stop
│   ├── Clone
│   ├── Logs (WebSocket)
│   └── Inspect (WebSocket)
├── domains.go             # Domains service
├── networks.go            # Networks service
├── webhooks.go            # Webhook verification
├── billing.go             # Billing service
├── users.go               # Users & Org service
├── types.go               # Shared types
├── errors.go              # Error handling
└── options.go             # Client options (base URL, timeout, etc.)
```

**安装**：

```bash
go get github.com/omnitun/omnitun-go@latest
```

### 2.2 Python SDK

**包名**：`omnitun` (PyPI)

```python
import asyncio
from omnitun import OmniTun, TunnelProtocol

async def main():
    client = OmniTun(token="ot_sk_xxx")

    # 创建并启动隧道（context manager 自动清理）
    async with client.tunnels.connect(
        name="my-api",
        protocol=TunnelProtocol.HTTP,
        local_port=8080,
    ) as tunnel:
        print(f"Tunnel ready: {tunnel.public_url}")

        # 实时日志
        async for entry in tunnel.logs(follow=True):
            print(f"[{entry.time}] {entry.method} {entry.path} {entry.status}")

asyncio.run(main())
```

```python
# 同步方式管理隧道
tunnels = client.tunnels.list(status="active")
for t in tunnels:
    print(f"{t.name}: {t.public_url}")

# 批量操作
client.tunnels.batch_stop([t.id for t in tunnels if t.name.startswith("dev-")])
```

```python
# Webhook 验证
from omnitun.webhooks import WebhookVerifier

verifier = WebhookVerifier(secret="whsec_xxx")
is_valid = verifier.verify(payload, signature)
```

**安装**：

```bash
pip install omnitun
```

**包结构**：

```
omnitun/
├── __init__.py             # OmniTun class
├── client.py               # HTTP client with retry + rate limit
├── tunnels.py              # Tunnels API
├── domains.py              # Domains API
├── networks.py             # Networks API
├── webhooks.py             # Webhook verification
├── billing.py              # Billing API
├── types.py                # Enums & dataclasses
├── errors.py               # Exception hierarchy
└── _async.py               # Async client
```

### 2.3 JavaScript / TypeScript SDK

**包名**：`@omnitun/sdk` (npm)

```typescript
import { OmniTun } from '@omnitun/sdk'

const client = new OmniTun({ token: 'ot_sk_xxx' })

// 创建隧道
const tunnel = await client.tunnels.create({
  name: 'my-api',
  protocol: 'http',
  localPort: 8080,
})
console.log(`Created: ${tunnel.publicUrl}`)

// 实时日志 (AsyncIterator)
for await (const entry of client.tunnels.logs(tunnel.id, { follow: true })) {
  console.log(`[${entry.time}] ${entry.method} ${entry.path} ${entry.status}`)
}

// 连接隧道（自动清理）
using tunnel = await client.tunnels.connect({
  name: 'dev-server',
  localPort: 3000,
})
console.log(`Ready: ${tunnel.publicUrl}`)
```

**包结构**：

```
@omnitun/sdk/
├── src/
│   ├── index.ts            # OmniTun class (exports)
│   ├── client.ts           # HTTP client
│   ├── tunnels.ts          # Tunnels API
│   ├── domains.ts          # Domains API
│   ├── networks.ts         # Networks API
│   ├── webhooks.ts         # Webhook verification
│   ├── billing.ts          # Billing API
│   ├── types.ts            # TypeScript types
│   ├── errors.ts           # Error classes
│   └── streaming.ts        # WebSocket + SSE helpers
├── package.json
├── tsconfig.json
└── README.md
```

**安装**：

```bash
npm install @omnitun/sdk
```

### 2.4 Java SDK

**Maven Central 坐标**：`io.omnitun:omnitun-java`

```java
import io.omnitun.OmniTun;
import io.omnitun.OmniTunClient;
import io.omnitun.models.Tunnel;
import io.omnitun.models.CreateTunnelRequest;

OmniTunClient client = OmniTun.builder()
    .token("ot_sk_xxx")
    .build();

Tunnel tunnel = client.tunnels().create(
    CreateTunnelRequest.builder()
        .name("my-api")
        .protocol("http")
        .localPort(8080)
        .build()
);

System.out.println("Created: " + tunnel.getPublicUrl());

// 列出隧道
List<Tunnel> tunnels = client.tunnels().list(
    ListTunnelsRequest.builder().status("active").build()
);

// 流量日志 (Reactive Streams)
client.tunnels().logs(tunnel.getId(), true)
    .subscribe(entry -> {
        System.out.printf("[%s] %s %s %d%n",
            entry.getTime(), entry.getMethod(), entry.getPath(), entry.getStatus());
    });
```

**安装 (Maven)**：

```xml
<dependency>
    <groupId>io.omnitun</groupId>
    <artifactId>omnitun-java</artifactId>
    <version>2.1.0</version>
</dependency>
```

**安装 (Gradle)**：

```groovy
implementation 'io.omnitun:omnitun-java:2.1.0'
```

### 2.5 Rust SDK

**crates.io**：`omnitun`

```rust
use omnitun::{OmniTun, CreateTunnelParams, TunnelProtocol};

#[tokio::main]
async fn main() -> Result<(), omnitun::Error> {
    let client = OmniTun::new("ot_sk_xxx");

    let tunnel = client.tunnels().create(CreateTunnelParams {
        name: "my-api".into(),
        protocol: TunnelProtocol::Http,
        local_port: 8080,
        ..Default::default()
    }).await?;

    println!("Created: {}", tunnel.public_url);

    // 实时日志
    let mut logs = client.tunnels().logs(tunnel.id, true).await?;
    while let Some(entry) = logs.next().await {
        println!("[{}] {} {} {}", entry.time, entry.method, entry.path, entry.status);
    }

    Ok(())
}
```

**安装**：

```toml
[dependencies]
omnitun = "2.1"
```

### 2.6 SDK 统一设计原则

| 原则 | 规范 |
|------|------|
| **认证** | 统一 `token` 参数 (API Key 首选的 `ot_sk_` 前缀) |
| **请求重试** | 默认重试 3 次，429/5xx 自动退避 |
| **速率限制** | 自动处理 `429` + `Retry-After` 头 |
| **WebSocket** | 支持自动重连 + 心跳保活 |
| **超时** | 默认 30s，可配置 |
| **User-Agent** | `OmniTun-{lang}/{version}` |
| **错误处理** | 统一错误类型（APIError / NetworkError / AuthError） |
| **日志** | 支持注入 logger 接口 |

---

## 三、CI/CD 集成

### 3.1 GitHub Actions

**Action 名称**：`omnitun/tunnel-action@v1`

仓库：`github.com/omnitun/tunnel-action`

```yaml
# .github/workflows/preview.yml
name: Preview Environment
on: [pull_request]

jobs:
  preview:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Start dev server
        run: npm run dev &

      - name: Create OmniTun tunnel
        id: tunnel
        uses: omnitun/tunnel-action@v1
        with:
          api-token: ${{ secrets.OMNITUN_API_KEY }}
          port: 3000
          protocol: http
          domain: preview-${{ github.event.pull_request.number }}.omnitun.io

      - name: Comment PR
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `🚀 Preview ready: ${{ steps.tunnel.outputs.url }}`
            })
```

**Action 输入参数**：

| 参数 | 必需 | 描述 | 默认值 |
|------|------|------|--------|
| `api-token` | 是 | OmniTun API Key | — |
| `port` | 是 | 本地端口 | — |
| `protocol` | 否 | 协议类型 | `http` |
| `domain` | 否 | 自定义域名 | 自动生成 |
| `timeout` | 否 | 隧道空闲超时 (秒) | `300` |
| `wait-for-health` | 否 | 等待端口可访问后才返回 | `true` |

**Action 输出**：

| 输出 | 描述 |
|------|------|
| `url` | 隧道公网 URL |
| `tunnel-id` | 隧道 ID |
| `domain` | 隧道域名 |

### 3.2 GitLab CI 模板

```yaml
# .gitlab-ci.yml
include:
  - project: 'omnitun/gitlab-templates'
    file: '/tunnel.yml'

preview:
  stage: deploy
  variables:
    OMNITUN_API_KEY: $OMNITUN_API_KEY
    OMNITUN_PORT: 3000
    OMNITUN_PROTOCOL: http
    OMNITUN_DOMAIN: preview-$CI_MERGE_REQUEST_IID.example.com
  script:
    - npm run dev &
    - omnitun http 3000 --domain $OMNITUN_DOMAIN
```

### 3.3 CircleCI Orb

```yaml
# .circleci/config.yml
version: 2.1

orbs:
  omnitun: omnitun/orb@1

jobs:
  preview:
    docker:
      - image: cimg/node:20
    steps:
      - checkout
      - run: npm run dev
      - background: true
      - omnitun/tunnel:
          port: 3000
          protocol: http
          domain: preview-$CIRCLE_PR_NUMBER.example.com
```

### 3.4 Docker 集成

**官方 Docker 镜像**：`omnitun/agent:latest`

```yaml
# docker-compose.yml
version: '3.8'
services:
  app:
    build: .
    ports:
      - '3000'

  omnitun:
    image: omnitun/agent:2.1
    command: http app:3000 --domain my-app.omnitun.io
    environment:
      OMNITUN_API_KEY: ${OMNITUN_API_KEY}
    network_mode: 'service:app'  # 共享网络栈（无需端口映射）
```

**Kubernetes Sidecar**：

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        image: my-app:latest
        ports:
        - containerPort: 3000
      - name: omnitun
        image: omnitun/agent:2.1
        args: ["http", "localhost:3000", "--domain", "my-app.omnitun.io"]
        env:
        - name: OMNITUN_API_KEY
          valueFrom:
            secretKeyRef:
              name: omnitun-secret
              key: api-key
```

---

## 四、框架插件

### 4.1 Next.js 插件

**包名**：`@omnitun/next`

```bash
npm install @omnitun/next
```

**用法**：在 `next.config.js` 中添加：

```js
// next.config.js
const { withOmniTun } = require('@omnitun/next')

module.exports = withOmniTun({
  apiKey: process.env.OMNITUN_API_KEY,
  domain: 'my-next-app.omnitun.io',
  // 仅 dev 模式启用
  enabled: process.env.NODE_ENV === 'development',
})({
  // 你的 Next.js 配置
})
```

启动时自动创建隧道：

```
$ npm run dev

▲ Next.js 14.1.0
- Local:          http://localhost:3000
- Network:        http://192.168.1.100:3000
- OmniTun:        https://my-next-app.omnitun.io  ← 自动添加
```

### 4.2 Vite 插件

**包名**：`@omnitun/vite-plugin`

```bash
npm install @omnitun/vite-plugin
```

```ts
// vite.config.ts
import { defineConfig } from 'vite'
import omnitun from '@omnitun/vite-plugin'

export default defineConfig({
  plugins: [
    omnitun({
      apiKey: process.env.OMNITUN_API_KEY,
      port: 5173,
      domain: 'my-vite-app.omnitun.io',
    }),
  ],
})
```

### 4.3 Spring Boot Starter

**包名**：`omnitun-spring-boot-starter`

```xml
<dependency>
    <groupId>io.omnitun</groupId>
    <artifactId>omnitun-spring-boot-starter</artifactId>
    <version>2.1.0</version>
</dependency>
```

```yaml
# application.yml
omnitun:
  api-key: ${OMNITUN_API_KEY}
  enabled: true
  tunnels:
    - name: my-api
      port: ${server.port}
      protocol: http
      domain: my-spring-app.omnitun.io
```

**自动配置**：
- ApplicationReadyEvent 触发时自动创建并启动隧道
- ContextClosedEvent 触发时自动停止隧道
- Actuator 端点：`/actuator/omnitun` 显示隧道状态

### 4.4 Django 插件

**包名**：`django-omnitun`

```bash
pip install django-omnitun
```

```python
# settings.py
INSTALLED_APPS = [
    ...
    'django_omnitun',
]

OMNITUN_API_KEY = os.environ['OMNITUN_API_KEY']
OMNITUN_TUNNELS = [
    {
        'name': 'my-django-app',
        'port': 8000,
        'protocol': 'http',
        'domain': 'my-django-app.omnitun.io',
    }
]
```

**管理命令**：

```bash
python manage.py omnitun_start    # 启动所有配置的隧道
python manage.py omnitun_stop     # 停止所有隧道
python manage.py omnitun_status   # 显示隧道状态
```

**信号**：`django_omnitun.signals.tunnel_started` / `tunnel_stopped` / `tunnel_error` 供应用监听。

### 4.5 框架插件统一设计原则

| 原则 | 规范 |
|------|------|
| **零配置启动** | 插件检测到 `OMNITUN_API_KEY` 环境变量后自动工作 |
| **仅开发模式** | 默认仅在 dev/profile active 时启用，生产环境需显式开启 |
| **优雅启停** | 框架启动时创建隧道，框架关闭时停止隧道 |
| **状态报告** | 控制台 / Actuator / Django admin 显示隧道 URL 和状态 |
| **复用 CLI** | 插件内部调用 SDK，不重复实现隧道逻辑 |

---

## 五、API 增强

### 5.1 API Key 作用域

当前 API Key 为全权限。需增加细粒度作用域：

```
┌─ Create API Key ─────────────────────────────┐
│ Name: [CI/CD Pipeline                        │
│                                                │
│ Scopes:                                        │
│ ☐ tunnels:read      读取隧道信息               │
│ ☐ tunnels:write     创建/修改/删除隧道         │
│ ☐ tunnels:control   启动/停止隧道              │
│ ☐ domains:read      读取域名信息               │
│ ☐ domains:write     管理自定义域名             │
│ ☐ networks:read     读取 Mesh 网络             │
│ ☐ networks:write    管理 Mesh 网络             │
│ ☐ billing:read      读取计费信息               │
│ ☐ billing:write     管理订阅                   │
│ ☐ org:read          读取组织信息               │
│ ☐ org:write         管理组织和成员             │
│ ☐ webhooks:write    管理 Webhook               │
│                                                │
│ Expiration: [30 days ▼]                       │
│                                                │
│ [Generate Key]                                 │
└────────────────────────────────────────────────┘
```

生成的 API Key 记录其 scope 到数据库。API Gateway 中间件验证 scope。

### 5.2 API 版本控制

```
/v1/tunnels  →  当前版本（稳定）
/v2/tunnels  →  下一代版本（新增功能）
/v1/  →  始终指向最新稳定版本（等同于 /v2/ 发布后）
```

版本策略：
- `/v1/` 发布 `v2` 后至少维护 12 个月
- 废弃端点返回 `Sunset: Sat, 01 Jan 2028 00:00:00 GMT` 响应头
- Dashboard 和 SDK 文档自动切换到最新版本
- `GET /v1/openapi.json` 和 `GET /v2/openapi.json` 分别返回各自规范

### 5.3 速率限制头

所有 API 响应返回标准限流头：

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 987
X-RateLimit-Reset: 1716278400
X-RateLimit-Resource: tunnels
```

| 头 | 描述 |
|----|------|
| `X-RateLimit-Limit` | 该时间窗口内最大请求数 |
| `X-RateLimit-Remaining` | 剩余可用请求数 |
| `X-RateLimit-Reset` | 窗口重置的 Unix 时间戳 |
| `X-RateLimit-Resource` | 限制的资源类别 |

超限时返回：

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
```

### 5.4 条件请求

支持 `ETag` 和 `If-None-Match` 减少带宽：

```http
GET /v1/tunnels HTTP/1.1
If-None-Match: "abc123"
```

```http
HTTP/1.1 304 Not Modified
ETag: "abc123"
```

支持范围：
- `GET /v1/tunnels` — 列表
- `GET /v1/tunnels/:id` — 详情
- `GET /v1/domains` — 域名列表
- `GET /v1/openapi.json` — API 规范

### 5.5 新增/增强端点汇总

| 方法 | 路径 | 说明 | 状态 |
|------|------|------|------|
| `GET` | `/v2/status` | 聚合状态概览 | 新增 |
| `WS` | `/v1/tunnels/:id/logs` | 实时流量日志流 | 新增 |
| `WS` | `/v1/tunnels/:id/inspect` | 交互式请求检查 | 新增 |
| `GET` | `/v1/tunnels/:id/clone` | 获取隧道可克隆配置 | 新增 |
| `POST` | `/v1/tunnels/:id/tags` | 添加标签 | 新增 |
| `DELETE` | `/v1/tunnels/:id/tags/:tag` | 移除标签 | 新增 |
| `POST` | `/v1/tunnels/batch/start` | 批量启动 | 新增 |
| `POST` | `/v1/tunnels/batch/stop` | 批量停止 | 新增 |
| `POST` | `/v1/tunnels/batch/delete` | 批量删除 | 新增 |
| `GET` | `/v1/networks` | 网络列表 | 新增 |
| `POST` | `/v1/networks` | 创建网络 | 新增 |
| `POST` | `/v1/networks/:id/join` | 加入网络 | 新增 |
| `POST` | `/v1/networks/:id/leave` | 离开网络 | 新增 |
| `GET` | `/v1/networks/:id/nodes` | 网络节点状态 | 新增 |
| `GET` | `/v1/releases` | CLI 版本列表 | 新增 |
| `GET` | `/v1/releases/latest` | 最新 CLI 版本 | 新增 |
| `GET` | `/v1/config` | CLI 配置模板 | 新增 |
| `GET` | `/v1/openapi.json` | OpenAPI spec | 已有 |
| `GET` | `/v2/openapi.json` | v2 OpenAPI spec | 新增 |
| `POST` | `/v1/api-keys` | 创建 API Key (支持 scopes) | 增强 |
| `PUT` | `/v1/api-keys/:id` | 更新 API Key scopes | 新增 |
