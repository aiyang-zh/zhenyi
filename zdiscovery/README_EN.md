# zdiscovery

**Service Discovery Module**: Provides `Discoverer` implementations for Etcd and Noop, used for cross-process Actor discovery and routing support.

## Module Positioning

- Maintains "Actor registration info -> queryable view"
- Provides candidate source for `zgate` remote routing and `zactor` cross-process calls
- In single-node mode, can use Noop implementation to keep interface consistent

## Current Implementations

| Implementation | Description |
|----------------|-------------|
| `NewEtcdDiscovery(ctx, client)` | Etcd |
| `NewNoopDiscovery()` | Single-node/testing empty implementation |

## Minimal Usage

```go
d, _ := zdiscovery.NewEtcdDiscovery(ctx, etcdClient)
group.SetDiscoverer(d)
```

## Usage Suggestions

- Production environment recommends completing registration and watch warmup at startup before opening for traffic
- Have degradation strategy for discovery failures (fallback to local routing or fast failure)
- Single-node/testing scenarios prefer `NewNoopDiscovery()` to avoid external dependencies

## Related Documentation

- Architecture: `../docs/ARCHITECTURE.md`
- Module API navigation: `../docs/MODULE_API.md`
- Gateway routing: `../zgate/README.md`
