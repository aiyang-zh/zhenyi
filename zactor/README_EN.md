# zactor

**Actor Runtime Core Module**: Provides single-Actor single-mailbox message processing model, plus Group orchestration, RPC, Tick, Watchdog, and other capabilities.

## Module Positioning

- `zactor` is the execution core of `zhenyi`, responsible for Actor lifecycle and message processing main loop.
- Typical flow: After `zgate` receives packets, converts to `ActorCmd`, delivered and executed by `zactor` Handler.
- Suitable for high concurrency, low latency, and business processing scenarios requiring clear state boundaries.

## Core Capabilities

- **Actor lifecycle**: `NewActor`, `Init`, `Close`
- **Message handling**: `RegisterHandle`, `Push`, `SendMsg`
- **Async tasks**: `AsyncRun`, `AsyncRunWithMsg`
- **Periodic tasks**: `RegisterTickFn`
- **Group orchestration**: `NewGroup`, `AddActor`, `RegisterRoutes`, `Run`
- **Observability & protection**: Watchdog blocking detection, RPC circuit breaker and timeout statistics

## Minimal Usage

```go
cfg := zmodel.ActorConfig{Id: 1, Name: "im", ActorType: 2, Index: 0}
actor := zactor.NewActor(cfg)
actor.SetIActor(actor)
actor.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    // business logic
})
if err := actor.Init(ctx); err != nil {
    return err
}
```

## Usage Suggestions

- Keep Handler/Tick short; move blocking operations to `AsyncRun`
- When cross-Actor calls are involved, must use `context.Context` with timeout
- All pooled messages should be `Release`d as agreed to avoid object leaks

## Routing Fast Path Extension

- `IGroup` defaults to returning candidate replicas via `LookupActorsByMsgID` (safe to modify).
- If `ziface.IGroupRouteTableView` is implemented, the routing layer can use `LookupActorsByMsgIDView` read-only view fast path, avoiding hot path slice `make+copy` allocation.
- Constraint: Returned slice from `LookupActorsByMsgIDView` must be treated as read-only; callers must not modify; use `LookupActorsByMsgID` when mutable results are needed.

## Related Documentation

- Overall architecture: `../docs/ARCHITECTURE.md`
- Module navigation: `../docs/MODULE_API.md`
- Examples overview: `../docs/EXAMPLES.md`
- Gateway collaboration: `zgate/README.md`
