# znats

**NATS Bus Adapter Module**: Implements `zbus.TopicBus`, provides default bus access and connection pool capability.

## Module Positioning

- Adapts NATS capability to `zbus` abstraction for Gate/Actor reuse
- Supports default client initialization (`DefaultNatsClient`)
- Provides infrastructure for cross-process message routing

## Minimal Usage

```go
znats.NewDefaultNats(natsURL, poolSize)
if err := znats.DefaultNatsClient.Connect(ctx); err != nil {
    return err
}
// NewDefaultNats sets DefaultNatsClient and injects default zbus
```

## Usage Suggestions

- At startup, complete `NewDefaultNats + Connect` first, then start Gate/Group
- Production environment recommends explicit connection parameter and reconnection strategy configuration
- Combined with `zcheck`, can early discover "bus not initialized/not connected" issues

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- Global variables and startup checks: `../docs/GLOBALS_AND_HOOKS.md`
