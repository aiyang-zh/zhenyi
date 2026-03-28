# zbus

**Message Bus Abstraction Module**: Defines cross-process Topic broadcast/subscribe contract, not bound to specific middleware implementation.

## Module Positioning

- Provides unified `TopicBus` interface, isolating upper layer from underlying middleware differences
- Provides `DefaultBus` global for Gate/Actor quick access
- Allows business to replace with in-memory implementation in test scenarios

## Core Interface

```go
type TopicBus interface {
    Broadcast(topic string, data []byte) error
    Subscribe(topic string, handler Handler) (Subscription, error)
}
```

## Usage Suggestions

- Production environment typically injects `znats` implementation into `zbus.DefaultBus`
- Single-node/testing can replace with in-memory implementation
- zactor cross-process sending depends on `DefaultBus`, needs injection before startup

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- NATS adapter: `../znats/README.md`
