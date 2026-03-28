package znats

import (
	"github.com/nats-io/nats.go"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi/zbus"
)

// natsSubscription 适配 NATS 的订阅到 zbus.Subscription 接口
type natsSubscription struct {
	sub *nats.Subscription
}

func (s *natsSubscription) Unsubscribe() error {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.Unsubscribe()
}

// NatsBus adapts NatsPool to zbus.TopicBus.
// NatsBus 使用 NatsPool 作为底层实现的 TopicBus 适配器。
type NatsBus struct {
	pool *NatsPool
}

// NewNatsBus creates a TopicBus implementation backed by NatsPool.
// NewNatsBus 基于 NatsPool 创建一个 TopicBus 实现。
func NewNatsBus(pool *NatsPool) *NatsBus {
	if pool == nil {
		return nil
	}
	return &NatsBus{pool: pool}
}

// Broadcast implements zbus.TopicBus.Broadcast.
// Broadcast 实现 zbus.TopicBus 的 Broadcast。
func (b *NatsBus) Broadcast(topic string, data []byte) error {
	if b == nil || b.pool == nil {
		return zerrs.New(zerrs.ErrTypeNetwork, "nats bus not initialized")
	}
	return b.pool.Broadcast(topic, data)
}

// Subscribe implements zbus.TopicBus.Subscribe.
// Subscribe 实现 zbus.TopicBus 的 Subscribe。
func (b *NatsBus) Subscribe(topic string, handler zbus.Handler) (zbus.Subscription, error) {
	if b == nil || b.pool == nil {
		return nil, zerrs.New(zerrs.ErrTypeNetwork, "nats bus not initialized")
	}
	if handler == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "nats bus handler cannot be nil")
	}

	sub := b.pool.SubscribeCall(topic, func(msg *nats.Msg) {
		if msg == nil {
			return
		}
		handler(msg.Subject, msg.Data)
	})
	if sub == nil {
		return nil, zerrs.New(zerrs.ErrTypeNetwork, "nats subscribe failed")
	}
	return &natsSubscription{sub: sub}, nil
}
