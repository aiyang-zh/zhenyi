# 示例总览

本文档汇总仓库内可直接运行的示例与用途。

## 1. 单机最小闭环

### `examples/echo_demo`

- 作用：最小可运行示例（Gate 接入 + 业务 Actor 收发一条消息）
- 适用：5 分钟跑通链路、排查环境问题、理解消息处理模型

运行服务端：

```bash
go run ./examples/echo_demo -conn tcp -addr 127.0.0.1:8021
```

另开终端运行客户端：

```bash
go run ./examples/echo_client -addr 127.0.0.1:8021 -text "hello zhenyi"
```

### `examples/im_single_demo`

- 作用：单机 Gate + Actor 运行示例
- 适用：快速验证本地环境与消息链路
- 常用开关：
  - `--reactor`：开启 TCP reactor 读模型（仅 TCP 且无 TLS/GM-TLS）
  - `--sharedSendWorker`：开启共享写 worker 模式（兼容非 reactor / reactor）

运行：

```bash
go run ./examples/im_single_demo
```

共享写 + reactor 示例：

```bash
go run ./examples/im_single_demo --reactor --sharedSendWorker
```

### `examples/im_single_demo_bench`

- 作用：单机压测/调优示例（连接规模、QPS、尾延迟）
- 适用：框架路径压测、reactor/shared-send 调参、pprof/pyroscope 观测
- 说明：该示例偏压测，不等同于完整聊天室语义

运行：

```bash
go run ./examples/im_single_demo_bench --reactor --sharedSendWorker --benchMode framework --codec msgpack
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
- 常用开关（Gate 进程）：**`--reactor`**、**`--sharedSendWorker`**（含义同 `im_single_demo`）

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

## 3. 浏览器 MMO 示例

### `examples/mmo_web_demo`

- 作用：最小 MMO 场景（位置同步 + 简易战斗与重生 + **`zaoi` AOI 过滤广播**）示例
- 组成：Go 服务端 + HTML/JS WebSocket 客户端
- 适用：快速验证浏览器接入、房间内状态广播

运行服务端：

```bash
go run ./examples/mmo_web_demo -conn ws -addr 127.0.0.1:8001
```

浏览器打开：

`http://127.0.0.1:8080/mmo_web_demo/web/`

说明：

- 示例默认会同时启动静态页面服务（`-web 127.0.0.1:8080`），不依赖 Python。
- 静态服务默认以 `./examples` 作为根目录（`-webRoot` 可改），以便复用公共前端 SDK：`/_shared/web/zhenyi-ws-sdk.js`。

## 4. 示例使用建议

- 先跑单机（`echo_demo` 或 `im_single_demo` + `im_single_client`）
- 再跑浏览器接入（`mmo_web_demo`）
- 再切到多进程（`im_multi_demo`）
- 最后用 `im_multi_client_load` 做压力验证

## 5. 常见问题

- 若跨进程消息不通，优先检查 `znats` / `zbus` 初始化与连接状态
- 若路由异常，检查 `ActorType`、路由注册与 discoverer 注入顺序
- 若无指标，确认是否已调用 `zmetrics.Enable(...)`
