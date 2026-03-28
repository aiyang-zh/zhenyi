# zgate

**Unified Gateway**: Embeds Actor, supports long connections (TCP/WS/KCP) + optional HTTP, routes client messages to backend Actors.

## Module Positioning

- Handles client connections and packet receiving
- Uniformly converts to `ActorCmd` into Actor processing chain
- Responsible for `ToClient` response delivery and routing failure fallback
- Optional HTTP and TLS/GM-TLS enablement

## Core Types

| Type | Description |
|------|-------------|
| `Server` | Gateway main body: embeds `*zactor.Actor`, holds underlying `IServer` |
| `SessionManager` | authId -> actorType -> actorId mapping (sticky routing) |

## Routing Order

1. Gate's own handler (e.g., login)
2. In-process `LocalRouter.RouteLocal` (Group routing table)
3. Cross-process `routeToRemoteActor` (NATS + discovery)
4. No route -> `sendNoRouteError` or `OnNoRoute` hook

## Minimal Usage

```go
gate := zgate.NewServer(cfg, znet.TCP)
gate.SetHTTPAddr(":8080") // optional
if err := gate.SetStandardTLS("server.crt", "server.key"); err != nil {
    return err
}
if err := gate.Init(ctx); err != nil {
    return err
}
if err := gate.RunServer(ctx); err != nil {
    return err
}
```

## Common Extension Points

- `SetRemoteRouteStrategy`: Remote primary selection (`PickOne`, fallback on failure)
- Default prefers `ziface.IGroupRemoteRouteTableView` (remote candidates read-only view) for zero-allocation routing hot path
- `OnNoRoute`: No route fallback handling
- `SetTraceHook`: Inject trace fields in receive path
- `SetHTTPAddr`: Enable HTTP service
- `SetTLSConfig` / `SetStandardTLS` / `SetGMTLS`: Encrypted access

## Related Documentation

- Overall architecture: `../docs/ARCHITECTURE.md`
- Module navigation: `../docs/MODULE_API.md`
- Monitoring and metrics: `../docs/MONITORING_METRICS.md`
