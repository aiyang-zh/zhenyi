# 示例总览

本文档汇总仓库内可直接运行的示例与用途。

## 1. 单机最小闭环

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

另开终端启动静态文件服务：

```bash
python3 -m http.server 8080 -d ./examples/mmo_web_demo/web
```

浏览器访问 `http://127.0.0.1:8080/`，可多开标签页联调。

## 4. 示例使用建议

- 先跑单机（`im_single_demo` + `im_single_client`）
- 再跑浏览器接入（`mmo_web_demo`）
- 再切到多进程（`im_multi_demo`）
- 最后用 `im_multi_client_load` 做压力验证

## 5. 常见问题

- 若跨进程消息不通，优先检查 `znats` / `zbus` 初始化与连接状态
- 若路由异常，检查 `ActorType`、路由注册与 discoverer 注入顺序
- 若无指标，确认是否已调用 `zmetrics.Enable(...)`
