# zroute

**Routing Strategy Module**: Defines local routing and remote primary selection strategies, used by `zgate` routing chain.

## Module Positioning

- Constrains "how to select local Actor / how to sort remote candidates"
- Decouples strategy from gateway main logic, convenient for scenario-based replacement
- Default strategies work out of the box; complex scenarios can customize interface implementation

## Core Types

| Type | Description |
|------|-------------|
| `LocalRouter` | In-process routing by msgId, implements `RouteLocal` |
| `RemoteRouteStrategy` | Cross-process primary selection, implements `PickOne` (zero-allocation) |
| `FirstCandidateStrategy` | Default: keep discovery order |
| `RoundRobinStrategy` | Round-robin |
| `RendezvousHashStrategy` | HRW consistent hashing (expansion/shrink friendly) |

## Minimal Usage

```go
gate.SetRemoteRouteStrategy(&zroute.RendezvousHashStrategy{})
```

## Usage Suggestions

- Simple scenarios can use default `FirstCandidateStrategy`
- Multi-instance with desire for stable session affinity, prefer `RendezvousHashStrategy`
- When customizing strategy, only do "selection" responsibility, do not couple network I/O in strategy

## Related Documentation

- Architecture: `../docs/ARCHITECTURE.md`
- Module navigation: `../docs/MODULE_API.md`
- Gateway collaboration: `../zgate/README.md`
