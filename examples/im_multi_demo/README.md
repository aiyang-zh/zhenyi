# im_multi_demo

基于 `im_single_demo` 的多进程版本：
- `process=1` 只启动 Gate（对外监听客户端连接）
- `process=2` 只启动 IM Actor（处理 join/leave/send）
- Gate 与 IM 通过 NATS + Etcd 进行跨进程路由
- 内置一个 Actor RPC 示例：Gate 在 `login` 时通过 `AsyncRunWithMsg + CallActor(IM)` 异步查询 IM 节点信息，并把 `imNode` 回给客户端（避免阻塞 Gate 主线程）

## 前置依赖

需要本机有可用的：
- NATS（默认 `nats://127.0.0.1:4222`）
- Etcd（默认 `127.0.0.1:2379`）

## 启动方式

在 `zhenyi/examples/im_multi_demo` 目录下分别启动两个进程：

```bash
# 进程1：Gate
go run ./examples/im_multi_demo --process=1 --addr=127.0.0.1:8001 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379
```

```bash
# 进程2：IM
go run ./examples/im_multi_demo --process=2 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379
```

客户端可以继续使用 `im_single_client` 连接 `127.0.0.1:8001`。

## 压测相关参数

- `-codec`：消息编解码（`json` 或 `msgpack`，默认 `json`）
- `-benchMode`：压测模式（`business` 或 `framework`，默认 `business`）

示例（推荐框架基线压测）：

```bash
# Gate
go run ./examples/im_multi_demo --process=1 --addr=127.0.0.1:8001 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379 --codec=msgpack --benchMode=framework
```

```bash
# IM
go run ./examples/im_multi_demo --process=2 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379 --codec=msgpack --benchMode=framework
```
