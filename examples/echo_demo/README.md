# echo_demo

`echo_demo` 是一个最小可运行示例，用于理解 `zhenyi` 的基本模式：**Gate 接入** + **业务 Actor 处理消息**。

- 服务端：单进程 Gate + Echo Actor
- 客户端：`examples/echo_client`（命令行）
- 协议：默认 TCP（也可 `-conn ws`）

## 运行

在仓库根目录执行：

```bash
go run ./examples/echo_demo -conn tcp -addr 127.0.0.1:8021
```

另开终端运行客户端：

```bash
go run ./examples/echo_client -addr 127.0.0.1:8021 -text "hello zhenyi"
```
