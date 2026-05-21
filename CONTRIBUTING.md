# 贡献指南

感谢你对 OmniTun 的关注！本文档将帮助你了解贡献流程和规范。

## 代码风格

### Go

项目使用 [golangci-lint](https://golangci-lint.run/) 统一代码风格，配置文件位于 `.golangci.yml`。提交前请确保通过 lint 检查：

```bash
make lint
```

关键规范：
- 使用 `gofmt` 格式化代码（由 `goimports` linter 强制）
- 错误必须处理，不得忽略（`errcheck`）
- 禁止使用 `context.Background()` 直接作为顶层 context（`noctx`）
- 必须使用参数化查询，防止 SQL 注入（`sqlclosecheck`）
- 变量、函数、类型命名遵循 Go 社区惯例（`revive`）

### TypeScript / React

前端代码使用 ESLint 检查，配置在 `web/package.json` 中：

```bash
cd web && npm run lint
```

关键规范：
- 遵循 `@typescript-eslint` 推荐规则
- React Hooks 规则由 `eslint-plugin-react-hooks` 强制
- 禁止未使用的 disable 指令

## Commit 规范

采用 [Conventional Commits](https://www.conventionalcommits.org/) 格式提交变更：

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Type 类型

| Type       | 用途                         |
| ---------- | ---------------------------- |
| `feat`     | 新功能                       |
| `fix`      | Bug 修复                     |
| `docs`     | 文档变更                     |
| `test`     | 测试相关（新增/修改/删除）   |
| `refactor` | 代码重构（无功能或 Bug 变更） |
| `chore`    | 构建/工具/依赖变更           |
| `perf`     | 性能优化                     |
| `style`    | 代码格式变更（仅空白等）     |

### Scope 作用域

常用 scope：`server`、`client`、`relay`、`auth`、`tunnel`、`gateway`、`protocol`、`web`、`deploy`、`docs`

### 示例

```
feat(tunnel): add TCP tunnel support with multiplexing

fix(auth): resolve JWT token refresh race condition

docs(web): update Dashboard API integration guide

test(tunnel): add integration tests for tunnel lifecycle

refactor(relay): extract stream multiplexing to internal package
```

## Pull Request 流程

1. **Fork** 主仓库并克隆到本地
2. 从 `main` 分支创建功能分支：`git checkout -b feat/my-feature`
3. 编码、测试、Lint 全部通过后提交
4. 推送到你的 Fork：`git push origin feat/my-feature`
5. 在 GitHub 上创建 Pull Request，填写 PR 描述
6. CI 自动运行 lint / test / build 检查
7. 至少一位维护者 Code Review 通过后合并

### PR 描述模板

```markdown
## 变更摘要
<!-- 简要描述此 PR 做了什么 -->

## 关联 Issue
<!-- 关联的 Issue 编号，如 Fixes #42 -->

## 变更类型
- [ ] 新功能 (feat)
- [ ] Bug 修复 (fix)
- [ ] 文档 (docs)
- [ ] 测试 (test)
- [ ] 重构 (refactor)
- [ ] 其他

## 测试
<!-- 描述你运行了哪些测试以及如何验证变更 -->

## 截图/录屏（如适用）
<!-- 前端变更附上截图或录屏 -->
```

## 测试要求

- **新增功能必须包含相应的单元测试**，Go 测试覆盖率不应低于变更前水平
- 修改核心模块（auth / tunnel / relay / protocol）时，需要补充或更新相关测试
- 集成测试位于 `tests/` 目录，涉及跨模块交互的变更请同步更新
- 运行测试确认通过：

```bash
make test            # Go 单元测试（含 race 检测 + 覆盖率）
cd web && npm run test  # 前端测试
```

## Issue 模板

### Bug Report

```markdown
**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Run '...'
2. Click on '...'
3. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Environment:**
 - OS: [e.g. macOS 15, Ubuntu 24.04]
 - Go version: [e.g. 1.23.4]
 - Node version: [e.g. 20.11.0]
 - Browser (if frontend): [e.g. Chrome 130]

**Logs**
```
Paste relevant logs here. Set LOG_LEVEL=debug for detailed output.
```
```

### Feature Request

```markdown
**Is your feature request related to a problem?**
A clear and concise description of the problem.

**Describe the solution you'd like**
A clear and concise description of what you want to happen.

**Describe alternatives you've considered**
Any alternative solutions or workarounds.

**Additional context**
Add any other context or screenshots about the feature request here.
```

## 行为守则

本项目遵循 [Contributor Covenant](https://www.contributor-covenant.org/) 行为守则。请对社区成员保持尊重和友善。
