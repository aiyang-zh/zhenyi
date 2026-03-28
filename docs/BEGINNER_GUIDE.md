# zhenyi 新手教程（Quick Start）

本文档面向首次接触 `zhenyi` 的开发者，目标是用最短路径跑通一个可工作的最小实时应用服务。

## 1. 前置准备

- Go 版本：`1.24+`
- 已安装 `git`
- 本地可用端口（示例默认 `9001`）

## 2. 获取代码与基础验证

```bash
git clone https://github.com/aiyang-zh/zhenyi.git
cd zhenyi
go test ./... -count=1
```

若测试通过，说明依赖与环境基本可用。

## 3. 跑最小示例（单机）

```bash
go run ./examples/im_single_demo
```

你将得到一个基于 `zgate + zactor` 的单机实时应用示例服务。  
如需客户端配合验证，可另开终端运行：

```bash
go run ./examples/im_single_client
```

## 4. 最小代码骨架

核心流程是：创建 Gate -> 初始化 -> 启动。

```go
cfg := zmodel.ActorConfig{
    Id: 1, Name: "gate", ActorType: 1, Index: 0, Host: "0.0.0.0", Port: 9001,
}
gate := zgate.NewServer(cfg, znet.TCP)
_ = gate.Init(ctx)
_ = gate.RunServer(ctx)
```

建议在 `Init/Run` 前完成：

- `RegisterHandle` 业务消息处理
- 可选 `SetHTTPAddr` 启用 HTTP
- 可选 `SetTLSConfig`/`SetGMTLS` 启用 TLS/GM-TLS

## 5. 跨进程最小认知

跨进程路由依赖消息总线与服务发现，常见组合是：

- `znats` 作为默认总线
- `zdiscovery`（如 etcd）作为发现层

启动前可用 `zcheck.Validate(...)` 做依赖自检，避免运行时才暴露缺失。

## 6. 下一步阅读建议

- 架构说明：[`ARCHITECTURE.md`](ARCHITECTURE.md)
- 模块 API 导航：[`MODULE_API.md`](MODULE_API.md)
- 示例总览：[`EXAMPLES.md`](EXAMPLES.md)
- 监控体系：[`MONITORING_OVERVIEW.md`](MONITORING_OVERVIEW.md)
