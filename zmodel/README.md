# zmodel

**消息与模型层**：定义 Actor 配置、命令模型与方案调优参数，作为各模块共享的数据契约。

## 模块定位

- 为 `zactor`、`zgate`、`zstartup` 等模块提供统一模型定义
- 聚焦“结构与配置”，不承载业务执行逻辑
- 与 `zmsg` 协作定义消息处理链路中的数据载体

## 核心类型（常用）

| 类型 | 说明 |
|------|------|
| `ActorConfig` | Actor 配置：Id、Process、Name、ActorType、Host、Port、WorkSize、ModeConfig 等 |
| `ActorCmd` | 入队单元：Type(Msg/Tick/SafeFn/TickFn/Client)、Msg、TickFn、Fn |
| `CmdType` | CmdTypeMsg、CmdTypeTick、CmdTypeSafeFn、CmdTypeTickFn、CmdTypeClient |
| `TickFnItem` | 定时回调注册项 |
| `FrameworkTuning` | 运行时调优：Actor 池大小、批处理、慢日志阈值、RTT 槽位等 |
| `ActorModeConfig` | 执行模式：顺序/并发、池大小、最大批次 |

## 最小用法

```go
cfg := zmodel.ActorConfig{
    Id: 1, Name: "gate", ActorType: 1, Index: 0, Host: "0.0.0.0", Port: 9001,
}
gate := zgate.NewServer(cfg, znet.TCP)
```

## 使用建议

- `ActorConfig` 用于声明实例，不要在运行期频繁改写
- `ActorCmd` 主要是方案内部队列单元，业务尽量通过公开 API 间接使用
- 调优参数建议在进程启动早期统一设置，避免运行中频繁调整

## 相关文档

- 总体架构：`../docs/ARCHITECTURE.md`
- 模块 API 导航：`../docs/MODULE_API.md`
- 消息模型配套：`../zmsg/README.md`
