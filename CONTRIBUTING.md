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

请勿提交：运行生成的 `logs/`、`*.log`、`.env`、真实密钥或仅本地使用的 PEM/配置；提交前执行 `git status`，确认无 IDE 垃圾文件误加入。详见 [SECURITY.md](SECURITY.md) 末尾「仓库洁净」。

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

- **Go 版本与 CI 不一致**：以根目录 `go.mod` 与 `README.md` 徽章为准。
- **`make bug-check` 缺工具**：`bug-check` 会跑 `staticcheck`、`gosec`；与 CI 一致可安装：`go install honnef.co/go/tools/cmd/staticcheck@v0.6.0`、`go install github.com/securego/gosec/v2/cmd/gosec@v2.22.0`（须用与项目相同主版本的 Go 编译，确保 `$(go env GOPATH)/bin` 在 `PATH` 中）。
- **`staticcheck` 报错**：用当前项目的 Go 重装上述 `staticcheck` 版本。
- **脚本引擎/示例**：单机示例（`im_single_demo`/`im_single_client`）不依赖 Etcd/NATS；多进程示例见 `docs/EXAMPLES.md`。
- **文档链接检查失败**：`make docs-check` 查看断链并修复。

## 贡献许可与 CLA

为保障许可与双授权一致性，所有贡献者需要同意贡献许可条款：

- 请在首次贡献前阅读并同意 `CLA.md`。
- 提交 PR 视为你确认你有权贡献该代码，并同意 `CLA.md` 中的条款。

