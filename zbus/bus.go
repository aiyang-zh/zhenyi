package zbus

// Subscription is an abstract subscription handle.
// Subscription 抽象的订阅句柄。
// It is independent from underlying implementation (NATS/Redis/Kafka/in-memory, etc.).
// 不关心底层实现（NATS、Redis、Kafka、本地内存等）。
type Subscription interface {
	Unsubscribe() error
}

// Handler is topic-message callback.
// Handler 主题消息处理函数。
// topic is the actual topic name.
// topic：实际的主题名称。
// data is raw bytes decoupled from transport protocol.
// data：消息内容（已解耦底层协议，只用字节流）。
type Handler func(topic string, data []byte)

// TopicBus abstracts topic-based cross-process message bus.
// TopicBus 基于 topic 的跨进程消息总线抽象。
//
// - Broadcast: broadcast one message to the specified topic.
// - Broadcast：向指定 topic 广播一条消息
// - Subscribe: subscribe to a topic; invoke handler on message receive.
// - Subscribe：订阅某个 topic，收到消息时回调 handler
//
// Actors depend only on this abstraction, not specific MQ implementation.
// Actor 只依赖这一层抽象，不关心底层是 NATS 还是其他 MQ。
type TopicBus interface {
	Broadcast(topic string, data []byte) error
	Subscribe(topic string, handler Handler) (Subscription, error)
}

// DefaultBus is the global TopicBus implementation.
// DefaultBus 默认的全局消息总线实现。
// It is injected by upper layer at startup:
// 由上层在启动时注入：
//   - 生产环境：通常使用 NATS 实现（由 znats 适配）
//   - 单机 / 测试：可以替换为 in‑memory 实现
var DefaultBus TopicBus
