# 第三方依赖与许可证提示（Third-Party Licenses）

本文件用于帮助使用者快速定位 `zhenyi` 的第三方依赖来源与许可证信息。
**最终以各依赖仓库/发布包的 LICENSE/NOTICE 为准**（本文件不构成法律意见）。

## 依赖来源

- **Go modules**：依赖声明在 `go.mod`/`go.sum` 中。
- **间接依赖**：Go 会通过依赖传递标记为 `// indirect` 的模块。
- **`replace`（如需核对）**：根目录 `go.mod` 底部对 `google.golang.org/grpc` 有一行版本对齐，用于消解传递依赖中的旧版 gRPC 与 etcd 所用版本的冲突；**不改变各模块自身许可证**。

## Direct 依赖（`go.mod` 首段 `require`，与版本号同步维护）

下列版本应与 `go.mod` 保持一致（发布前请复核）：

- `github.com/aiyang-zh/zhenyi-base` — `v1.1.0`
- `github.com/d5/tengo/v2` — `v2.17.0`
- `github.com/dop251/goja` — `v0.0.0-20260311135729-065cd970411c`
- `github.com/emmansun/gmsm` — `v0.41.1`
- `github.com/nats-io/nats-server/v2` — `v2.12.4`（主要用于集成测试 / 本地 NATS 场景）
- `github.com/nats-io/nats.go` — `v1.49.0`
- `github.com/panjf2000/ants/v2` — `v2.11.6`
- `github.com/pelletier/go-toml/v2` — `v2.2.4`
- `github.com/stretchr/testify` — `v1.11.1`
- `github.com/yuin/gopher-lua` — `v1.1.1`
- `go.etcd.io/etcd/client/v3` — `v3.6.8`
- `go.starlark.net` — `v0.0.0-20260210143700-b62fd896b91b`
- `go.uber.org/zap` — `v1.27.1`
- `google.golang.org/protobuf` — `v1.36.11`

## 如何核对许可证（推荐做法）

在本仓库根目录执行（会下载依赖；输出为一次性核对材料，无需提交到仓库）：

```bash
go mod download
go list -m -json all
```

然后到每个 module 的源代码目录中查看其 `LICENSE`/`NOTICE`/`COPYING` 等文件。

**维护约定**：升级 `go.mod` 中 direct 依赖版本时，请同步更新本节列表中的版本号。
