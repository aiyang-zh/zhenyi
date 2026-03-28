# zstream

**业务 Actor 轻封装**：对 `zactor.Actor` 的轻量包装，实现 `IServerActor` 生命周期接口。

## 定位

- 提供一个可独立启动的业务 Actor Server（`NewServer` + `RunServer`）
- 适合把业务逻辑按 `Server` 语义组织，而不重复封装 Actor 生命周期

当前实现不包含额外流式协议处理逻辑，核心能力来自嵌入的 `zactor.Actor`。

## 核心 API

- `NewServer(actorConfig zmodel.ActorConfig) *Server`
- `RunServer(ctx context.Context) error`
