# 贡献指南（CONTRIBUTING）

感谢你对 `zhenyi` 的兴趣与贡献。

## 开始之前

- 请先阅读根目录 `README.md` 与 `LICENSE`（本项目为 **AGPL-3.0 + 商业双授权**）。
- 提交 PR 即表示你同意你的贡献按本项目双授权模式进行许可（详见下文“贡献许可与 CLA”）。

## 如何提交 Issue

- **Bug**：提供可复现步骤、期望行为、实际行为、日志/堆栈、Go 版本与 OS。
- **Feature**：说明使用场景、API 设计草案、兼容性影响与替代方案。

## 如何提交 PR

- **一个 PR 解决一个问题**：避免大而杂的改动。
- **保持向后兼容**：对外 API 变更请在 PR 描述中明确，并同步文档。
- **写测试**：修 bug/加功能尽量带上对应测试用例。

### 仓库洁净（提交前必查）

公开仓库请勿包含：运行生成的 `logs/`、`*.log`、`.env`、真实密钥或仅本地使用的 PEM/配置；提交前执行 `git status`，确认无 IDE 垃圾文件误加入。详见 [SECURITY.md](SECURITY.md) 末尾「仓库洁净」。

### 本地检查（提交前）

为尽量与 CI 一致，建议优先使用仓库 `Makefile` 目标（而不是手写命令）。推荐：

- `make release-check`（最贴近 CI：docs-check + go test -count=1 + bug-check）
- 或至少 `make test-unit`（快速）/ `make test`（含 `-race`，更严格）

仅修改 `**/*.md`、`docs/**` 或根目录 `LICENSE` 时，GitHub 上的 **Run Tests** / **Bug Check** 不会触发。**Docs Link Check** 仅在变更 `**/*.md` 或 `scripts/docs-check.py` 时运行（与 `make docs-check` 同源）。若同一次提交包含代码与文档，仍会跑对应工作流。

```bash
go test ./... -count=1
go vet ./...
test -z "$(gofmt -l .)"
```

仓库也提供 `Makefile`：

```bash
make test
make test-unit
make bug-check
make docs-check
make release-check
```

### 常见失败排查

- **Go 版本不对**
  - 本项目使用 `go.mod` 声明的 Go 版本；请确保本地 Go 与 CI 一致（见根 `README.md` 的 Go 版本徽章）。
- **`make bug-check` 因工具缺失失败**
  - `bug-check` 会运行 `staticcheck`、`gosec`；本地缺失时可先安装，或使用 `make bug-check-strict` 与 CI 行为保持一致。
- **`staticcheck` 报 `unsupported version: 2` / import 标准库失败**
  - 多为本机 `staticcheck` 过旧（与 Go 1.24 不匹配）。请重装与 CI 一致版本：`go install honnef.co/go/tools/cmd/staticcheck@v0.6.0`
- **`go install gosec@latest` 失败，提示 `requires go >= 1.25.0`（或类似）**
  - 原因：某些新版本在其 **go.mod** 里声明了更高 Go 版本，当前环境的 Go 1.24 **无法编译/安装**该版本的可执行文件（不是「跑业务代码需要 1.25」）。与 CI 一致请固定版本：`go install github.com/securego/gosec/v2/cmd/gosec@v2.22.0`
- **脚本引擎/示例相关问题**
  - 单机示例（`im_single_demo`/`im_single_client`）不依赖 Etcd/NATS；多进程示例通常需要外部依赖可达（见 `docs/EXAMPLES.md`）。
- **文档链接检查失败**
  - 运行 `make docs-check` 查看断链列表，修复相对路径或调整文档位置。

## 贡献许可与 CLA

为保障本项目的开源发布与双授权一致性，所有贡献者需要同意贡献许可条款：

- 请在首次贡献前阅读并同意 `CLA.md`。
- 提交 PR 视为你确认你有权贡献该代码，并同意 `CLA.md` 中的条款。

