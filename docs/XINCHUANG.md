# 信创适配分析与初步验证

本文档给出 `zhenyi` 在“信创”常见环境下的适配点梳理与可执行的初步验证方法。

> 说明：这里的“信创”默认指 **国产 CPU（多为 arm64）+ 国产 OS（Linux 发行版）+ 国产中间件/数据库替代** 的组合。
> 本仓库为 Go 项目，核心适配目标通常是：**无 CGO 依赖、跨架构可构建、关键路径无平台强绑定**。

## 1. 适配结论（当前仓库状态）

- **CGO 依赖**：未发现 `import "C"` / `#cgo`；在 `CGO_ENABLED=0` 下 `go test ./...` 可通过。
- **平台强绑定**：仓库内未发现 `_linux.go` / `_darwin.go` 之类平台文件；存在少量 `//go:build`（用于调试/生命周期开关），不影响信创。
- **JSON 编解码策略（信创友好默认）**：
  - 默认使用标准库 `encoding/json`（无需额外 tag，跨平台最稳）。
  - 如需启用 `sonic`（更高性能，且可能受 Go/平台兼容性影响），请在构建时显式添加：`-tags sonic`。
- **多架构 / Linux 发行版构建**：不在仓库脚本里做 `GOOS/GOARCH` 交叉编译；请在 **目标信创机（或同架构 VM）** 上直接 `go test` / `go build`，或使用 **`make openeuler-check`** 在 openEuler 官方 Go 镜像内按 **容器所在架构** 做测试与编译。
- **本机快速检查**：`make xinchuang-check` 为 `CGO_ENABLED=0 go test ./...` 与 **本机架构** 的 `go build ./...`（开发机为 macOS/Windows 时验证的是该 OS 下的可构建性，不等价于生产 Linux）。

## 2. 已知：bytedance/sonic 与部分环境（linux/amd64）

在 **启用 `-tags sonic`** 且目标为 **linux/amd64** 时，**sonic/loader** 与 Go runtime 的符号兼容性可能因工具链组合而异（例如链接阶段报错涉及 `sonic/loader` 与 `runtime`）。依赖关系在启用 sonic 时为：`examples/...` → `zhenyi-base/zserialize` → `bytedance/sonic` → `sonic/loader`。

**建议**：Go 版本以根目录 **`go.mod`** 为准；在目标架构上执行 `go test` / `go build` 做实测。可选规避：构建时 `-ldflags="-checklinkname=0"`（不推荐，属工具链层面放宽）。仓库默认使用标准库 `encoding/json`，仅显式 `-tags sonic` 才走 sonic。

## 3. 国产 OS/中间件维度建议

`zhenyi` 本身为纯 Go 解决方案层，外部依赖主要集中在“可选组件”：

- **发现**：Etcd（可替换为其他实现；或在不需要分布式时使用 Noop）。
- **总线**：NATS（可替换为其他实现；或在单机示例下完全不需要）。

建议在信创落地时把这些外部组件做成“可插拔清单”：

- 必选：无（单机闭环）
- 可选：Etcd / NATS（分布式能力）

## 4. 初步验证（可执行）

仓库提供脚本：

```bash
./scripts/xinchuang-check.sh
```

它会执行：

- `CGO_ENABLED=0 go test ./... -count=1`
- `CGO_ENABLED=0 go build ./...`（**本机** `GOOS/GOARCH`，不做交叉编译）
- 若需验证 **sonic** 路径，请在 **目标 linux/amd64 环境** 上自行执行 `CGO_ENABLED=0 go build ./... -tags sonic`，并参考上节兼容性说明。

### 4.1 openEuler 官方 Go 镜像（单测 + 示例编译）

使用 **`GOLANG_IMAGE`** 指向的 openEuler Go 镜像（默认见 `scripts/openeuler-check.sh`，须满足根目录 **`go.mod`** 的 Go 版本），在容器内执行 **`CGO_ENABLED=0 go test ./...`**，并对 **`go build ./...`** 以及 **`im_single_demo` / `im_single_client` / `im_multi_demo` / `im_multi_client_load` / `gencert`** 等示例路径做显式编译检查（不长期跑进程，仅验证可构建）。

```bash
make openeuler-check
# 或
./scripts/openeuler-check.sh
```

说明：

- 同样需要 **`../zhenyi-base`** 或 **`ZHENYI_BASE`**。
- 可通过 **`GOLANG_IMAGE`** 覆盖镜像标签；若需指定平台（多为 **amd64** 镜像），可设 **`PLATFORM=linux/amd64`**。
- 首次运行前可 **`docker pull`** 所选 **`GOLANG_IMAGE`**。

## 5. 后续建议（更严格的信创验证）

- **在真实信创 OS 上跑**：直接在目标环境执行 `make bug-check`（或容器/VM）。
- **覆盖更多架构**：如 loong64/riscv64（需评估依赖库支持）。
- **外部组件替换验证**：用你计划的国产替代组件跑 `examples/im_multi_demo` 的分布式链路。

