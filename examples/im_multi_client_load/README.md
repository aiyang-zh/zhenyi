# im_multi_client_load

聊天室并发压测客户端示例（多连接自动登录/进房/发消息）。

## 启动

在克隆后的 **zhenyi 仓库根目录**执行：

```bash
go run ./examples/im_multi_client_load
```

## 常用参数

```bash
go run ./examples/im_multi_client_load \
  -addr 127.0.0.1:8001 \
  -room lobby \
  -clients 100 \
  -intervalMs 200 \
  -durationS 43200 \
  -prefix bot \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgSend 4 \
  -codec msgpack \
  -benchMode framework
```

- `-addr`：Gate 地址
- `-room`：压测房间名
- `-clients`：并发连接数
- `-intervalMs`：每个客户端发消息间隔（毫秒）
- `-durationS`：压测持续时长（秒）
- `-prefix`：昵称前缀（示例：`bot_1`、`bot_2`）
- `-msgLogin`：登录请求消息 ID
- `-msgJoin`：进房请求消息 ID
- `-msgSend`：发送房间消息请求 ID
- `-codec`：消息编解码（`json` 或 `msgpack`，默认 `json`）
- `-benchMode`：压测模式（`business` 或 `framework`，默认 `business`）

## 框架压测推荐命令

目标：尽量压框架链路能力，减少业务逻辑干扰。

### 1) 服务端（两个进程都用同一 codec/benchMode）

```bash
# 进程1：Gate
go run ./examples/im_multi_demo \
  -process 1 \
  -addr 127.0.0.1:8001 \
  -nats nats://127.0.0.1:4222 \
  -etcd 127.0.0.1:2379 \
  -codec msgpack \
  -benchMode framework
```

```bash
# 进程2：IM
go run ./examples/im_multi_demo \
  -process 2 \
  -nats nats://127.0.0.1:4222 \
  -etcd 127.0.0.1:2379 \
  -codec msgpack \
  -benchMode framework
```

### 2) 客户端（你的 500 并发 12 小时参数）

```bash
go run ./examples/im_multi_client_load \
  -addr 127.0.0.1:8001 \
  -room lobby \
  -clients 500 \
  -intervalMs 200 \
  -durationS 43200 \
  -prefix bot \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgSend 4 \
  -codec msgpack \
  -benchMode framework
```

## 对照实验命令（建议）

固定并发参数后，按下面四组依次跑，比较 `Gate Monitor` 中 `RTT_P99` 与 slow warn 数量：

1. `-codec json -benchMode business`
2. `-codec msgpack -benchMode business`
3. `-codec json -benchMode framework`
4. `-codec msgpack -benchMode framework`

## 输出指标

程序会周期输出：

- `sent`：累计发送消息数
- `recv`：累计接收消息数
- `avg_send_qps`：平均发送 QPS

用于快速验证跨 Actor 聊天室路由与广播链路的吞吐表现。
