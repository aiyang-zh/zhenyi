# 示例总览

本文档汇总仓库内可直接运行的示例与用途。

## 1. 单机最小闭环

### `examples/im_single_demo`

- 作用：单机 Gate + Actor 运行示例
- 适用：快速验证本地环境与消息链路

运行：

```bash
go run ./examples/im_single_demo
```

### `examples/im_single_client`

- 作用：与单机 demo 配套的最小客户端
- 适用：验证服务端收发与回包

运行：

```bash
go run ./examples/im_single_client
```

## 2. 多进程示例

### `examples/im_multi_demo`

- 作用：多进程 Actor/Gate 协作示例
- 依赖：通常需要 Etcd + NATS
- 适用：验证远程路由、跨进程消息

运行前建议先确认外部依赖可达，再启动：

```bash
go run ./examples/im_multi_demo
```

### `examples/im_multi_client_load`

- 作用：多客户端压测/并发请求示例
- 适用：基础吞吐与稳定性验证

运行：

```bash
go run ./examples/im_multi_client_load
```

## 3. 示例使用建议

- 先跑单机（`im_single_demo` + `im_single_client`）
- 再切到多进程（`im_multi_demo`）
- 最后用 `im_multi_client_load` 做压力验证

## 4. 常见问题

- 若跨进程消息不通，优先检查 `znats` / `zbus` 初始化与连接状态
- 若路由异常，检查 `ActorType`、路由注册与 discoverer 注入顺序
- 若无指标，确认是否已调用 `zmetrics.Enable(...)`
