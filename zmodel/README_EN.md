# zmodel

**Message and Model Layer**: Defines Actor configuration, command models, and solution tuning parameters, serving as shared data contracts across modules.

## Module Positioning

- Provides unified model definitions for modules like `zactor`, `zgate`, `zstartup`
- Focuses on "structure and configuration", does not carry business execution logic
- Cooperates with `zmsg` to define data carrier in message processing chain

## Core Types (Commonly Used)

| Type | Description |
|------|-------------|
| `ActorConfig` | Actor configuration: Id, Process, Name, ActorType, Host, Port, WorkSize, ModeConfig, etc. |
| `ActorCmd` | Enqueue unit: Type(Msg/Tick/SafeFn/TickFn/Client), Msg, TickFn, Fn |
| `CmdType` | CmdTypeMsg, CmdTypeTick, CmdTypeSafeFn, CmdTypeTickFn, CmdTypeClient |
| `TickFnItem` | Periodic callback registration item |
| `FrameworkTuning` | Runtime tuning: Actor pool size, batching, slow log threshold, RTT slots, etc. |
| `ActorModeConfig` | Execution mode: sequential/concurrent, pool size, max batch |

## Minimal Usage

```go
cfg := zmodel.ActorConfig{
    Id: 1, Name: "gate", ActorType: 1, Index: 0, Host: "0.0.0.0", Port: 9001,
}
gate := zgate.NewServer(cfg, znet.TCP)
```

## Usage Suggestions

- `ActorConfig` is used for instance declaration, do not frequently rewrite at runtime
- `ActorCmd` is mainly a queue unit for solution internals; business should use it indirectly via public APIs
- Tuning parameters recommended to be set uniformly early in process startup to avoid frequent adjustments during runtime

## Related Documentation

- Overall architecture: `../docs/ARCHITECTURE.md`
- Module navigation: `../docs/MODULE_API.md`
- Message model companion: `../zmsg/README.md`
