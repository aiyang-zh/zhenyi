package znats

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/ztime"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/zmetrics"
)

const (
	DefaultMaxRetries = 30
	DefaultRetryDelay = 200 * time.Millisecond
)

// Nats is a single NATS client wrapper with subscription tracking and topic timeouts.
// Nats 是单个 NATS 客户端封装，包含订阅跟踪与按 topic 超时配置。
type Nats struct {
	url  string
	conn *nats.Conn
	// subs uses copy-on-write readonly snapshots:
	// subs 使用 copy-on-write 的只读快照：
	// - 读：atomic.Load，无锁
	// - 写：subsMu 串行化 + 复制 map 后再 atomic.Store
	subsMu sync.Mutex
	subs   atomic.Value // subsMap

	globalTimeoutNs atomic.Int64
}

type subItem struct {
	topic   string
	sub     *nats.Subscription
	timeout time.Duration
}

type subsMap map[string]*subItem

// NatsPool is a simple pool of Nats clients for load spreading.
// NatsPool 是 Nats 客户端池（用于分摊发送负载）。
type NatsPool struct {
	clients []*Nats
	counter uint64
}

// Broadcast publishes data to topic using one client selected from pool.
// Broadcast 使用池中一个客户端向 topic 发布消息。
func (c *NatsPool) Broadcast(topic string, data []byte) error {
	if len(c.clients) == 0 {
		return zerrs.New(zerrs.ErrTypeInternal, "nats pool has no available clients")
	}
	idx := atomic.AddUint64(&c.counter, 1) % uint64(len(c.clients))
	err := c.clients[idx].Broadcast(topic, data)
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats pool broadcast failed")
	}
	return nil
}

// SubscribeCall subscribes topic with callback using the first client in pool.
// SubscribeCall 使用池中第一个客户端订阅 topic（回调方式）。
func (c *NatsPool) SubscribeCall(topic string, f func(msg *nats.Msg)) *nats.Subscription {
	if len(c.clients) == 0 {
		return nil
	}
	return c.clients[0].SubscribeCall(topic, f)
}

// Connect connects all clients in pool.
// Connect 连接池内所有客户端。
func (c *NatsPool) Connect(ctx context.Context) error {
	for i, v := range c.clients {
		if err := v.Connect(ctx); err != nil {
			return zerrs.Wrapf(err, zerrs.ErrTypeNetwork, "nats pool client[%d] connect failed", i)
		}
	}
	return nil
}

// IsConnected reports whether this client has an active NATS connection.
func (q *Nats) IsConnected() bool {
	if q == nil || q.conn == nil {
		return false
	}
	return q.conn.IsConnected()
}

// IsConnected reports whether every pool client is connected.
// IsConnected 返回池内所有客户端是否都已连接。
func (c *NatsPool) IsConnected() bool {
	if c == nil || len(c.clients) == 0 {
		return false
	}
	for _, cl := range c.clients {
		if cl == nil || !cl.IsConnected() {
			return false
		}
	}
	return true
}

// DefaultNatsClient is the global default NATS pool client created by NewDefaultNats.
// DefaultNatsClient 为全局默认 NATS 连接池客户端（由 NewDefaultNats 创建）。
var DefaultNatsClient *NatsPool

var one sync.Once

// NewDefaultNats creates global DefaultNatsClient and injects it as default zbus.TopicBus implementation.
// NewDefaultNats 创建全局 DefaultNatsClient，并注入为默认 zbus.TopicBus 实现。
func NewDefaultNats(url string, poolSize int) {
	one.Do(func() {
		if poolSize <= 0 {
			poolSize = 1
		}
		DefaultNatsClient = &NatsPool{clients: make([]*Nats, poolSize)}
		for i := 0; i < poolSize; i++ {
			natsClient := NewNats(url)
			DefaultNatsClient.clients[i] = natsClient
		}
		// Use NATS as default cross-process TopicBus implementation.
		// 使用 NATS 作为默认的跨进程 TopicBus 实现。
		zbus.DefaultBus = NewNatsBus(DefaultNatsClient)
	})
}

// DefaultURL is default NATS server URL.
// DefaultURL 默认 NATS 服务地址。
const DefaultURL = "nats://127.0.0.1:4222"

// NewNats creates a single Nats client wrapper.
// NewNats 创建单个 Nats 客户端封装。
func NewNats(url string) *Nats {
	q := &Nats{
		url: url,
	}
	q.globalTimeoutNs.Store(int64((1 * time.Second).Nanoseconds()))
	q.subs.Store(subsMap{})
	return q
}

// SetGlobalTimeout sets default request timeout for all topics.
// SetGlobalTimeout 设置所有 topic 的默认请求超时。
func (q *Nats) SetGlobalTimeout(t time.Duration) {
	q.globalTimeoutNs.Store(int64(t.Nanoseconds()))
}

// SetTopicTimeout sets request timeout for one topic (copy-on-write).
// SetTopicTimeout 设置单个 topic 的请求超时（copy-on-write）。
func (q *Nats) SetTopicTimeout(topic string, t time.Duration) {
	q.subsMu.Lock()
	defer q.subsMu.Unlock()

	m := q.loadSubs()
	item, ok := m[topic]
	if !ok {
		return
	}
	// copy-on-write: keep subItem immutable to avoid concurrent read/write on same object.
	// copy-on-write：subItem 也保持不可变，避免并发读写同一对象。
	next := make(subsMap, len(m))
	for k, v := range m {
		next[k] = v
	}
	next[topic] = &subItem{topic: item.topic, sub: item.sub, timeout: t}
	q.subs.Store(next)
}

func (q *Nats) getTimeout(topic string) time.Duration {
	if item, ok := q.loadSubs()[topic]; ok {
		return item.timeout
	}
	return time.Duration(q.globalTimeoutNs.Load())
}

// Connect connects to NATS server with retry until success or ctx done.
// Connect 连接 NATS 服务端（重试直到成功或 ctx 结束）。
func (q *Nats) Connect(ctx context.Context) error {
	var lastErr error
	timer := time.NewTimer(0)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()

	for attempt := 1; attempt <= DefaultMaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return zerrs.Wrap(ctx.Err(), zerrs.ErrTypeNetwork, "nats connect cancelled")
		default:
		}

		conn, err := nats.Connect(q.url)
		if err != nil {
			lastErr = err
			zlog.Warn("Failed to connect to NATS, retrying...",
				zap.String("url", q.url),
				zap.Int("attempt", attempt),
				zap.Int("maxRetries", DefaultMaxRetries),
				zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats connection failed")))

			// Reuse one timer to avoid allocation on each retry.
			// 避免每次重试都分配新的 timer：复用一个 timer。
			// Delay is still computed from ztime.ServerNow() as baseline.
			// 这里用标准库 timer，但延迟的计算以 ztime.ServerNow() 为基准（支持时间偏移）。
			deadline := ztime.ServerNow().Add(DefaultRetryDelay)
			wait := time.Until(deadline)
			if wait < 0 {
				wait = 0
			}
			timer.Reset(wait)

			select {
			case <-ctx.Done():
				return zerrs.Wrap(ctx.Err(), zerrs.ErrTypeNetwork, "nats connect cancelled during retry")
			case <-timer.C:
			}
			continue
		}

		q.conn = conn
		zlog.Info("Successfully connected to NATS", zap.String("url", q.url))
		return nil
	}
	return zerrs.Wrapf(lastErr, zerrs.ErrTypeNetwork, "nats connect failed after %d attempts", DefaultMaxRetries)
}

// Close closes underlying NATS connection.
// Close 关闭底层 NATS 连接。
func (q *Nats) Close() {
	if q.conn != nil {
		q.conn.Close()
	}
}

// GetSub returns subscription of topic if exists.
// GetSub 返回指定 topic 的订阅（若存在）。
func (q *Nats) GetSub(topic string) *nats.Subscription {
	if item, ok := q.loadSubs()[topic]; ok {
		return item.sub
	}
	return nil
}

func (q *Nats) loadSubs() subsMap {
	if q == nil {
		return subsMap{}
	}
	if v := q.subs.Load(); v != nil {
		return v.(subsMap)
	}
	// Should not happen (NewNats stores initial map), but keep fallback to avoid panic.
	// 理论上不会发生（NewNats 会 Store），但兜底避免 panic。
	return subsMap{}
}

var errNoConn = zerrs.New(zerrs.ErrTypeNetwork, "nats: connection not established")

// SubscribeChan subscribes with channel delivery.
// SubscribeChan 订阅。
func (q *Nats) SubscribeChan(topic string, ch chan *nats.Msg) *nats.Subscription {
	if q.conn == nil {
		zlog.Error("nats conn is nil", zap.String("topic", topic))
		return nil
	}
	sub, err := q.conn.ChanSubscribe(topic, ch)
	if err != nil {
		zlog.Error("Failed to subscribe to NATS channel",
			zap.String("topic", topic),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats channel subscribe failed")))
		return nil
	}
	q.upsertSub(topic, sub, time.Duration(q.globalTimeoutNs.Load()))
	return sub
}

// Subscribe performs synchronous subscription.
// Subscribe 订阅。
func (q *Nats) Subscribe(topic string) *nats.Subscription {
	if q.conn == nil {
		zlog.Error("nats conn is nil", zap.String("topic", topic))
		return nil
	}
	sub, err := q.conn.SubscribeSync(topic)
	if err != nil {
		zlog.Error("Failed to subscribe to NATS sync",
			zap.String("topic", topic),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats sync subscribe failed")))
		return nil
	}
	q.upsertSub(topic, sub, time.Duration(q.globalTimeoutNs.Load()))
	return sub
}

// SubscribeCall subscribes with callback handler.
// SubscribeCall 订阅。
func (q *Nats) SubscribeCall(topic string, f func(msg *nats.Msg)) *nats.Subscription {
	if q.conn == nil {
		zlog.Error("nats conn is nil", zap.String("topic", topic))
		return nil
	}
	sub, err := q.conn.Subscribe(topic, f)
	if err != nil {
		zlog.Error("Failed to subscribe to NATS with callback",
			zap.String("topic", topic),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats callback subscribe failed")))
		return nil
	}
	q.upsertSub(topic, sub, time.Duration(q.globalTimeoutNs.Load()))
	return sub
}

// SubscribeQueue subscribes in queue-group mode.
// SubscribeQueue 订阅队列模式。
func (q *Nats) SubscribeQueue(topic, queue string) *nats.Subscription {
	if q.conn == nil {
		zlog.Error("nats conn is nil", zap.String("topic", topic))
		return nil
	}
	sub, err := q.conn.QueueSubscribeSync(topic, queue)
	if err != nil {
		zlog.Error("Failed to subscribe to NATS queue",
			zap.String("topic", topic),
			zap.String("queue", queue),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats queue subscribe failed")))
		return nil
	}
	q.upsertSub(topic, sub, time.Duration(q.globalTimeoutNs.Load()))
	return sub
}

// UnSubscribe unsubscribes topics.
// UnSubscribe 取消订阅。
func (q *Nats) UnSubscribe(topics ...string) []string {
	var errTopic []string
	q.subsMu.Lock()
	defer q.subsMu.Unlock()

	cur := q.loadSubs()
	next := make(subsMap, len(cur))
	for k, v := range cur {
		next[k] = v
	}
	for _, topic := range topics {
		if item, ok := next[topic]; ok {
			err := item.sub.Unsubscribe()
			if err != nil {
				zlog.Warn("Failed to unsubscribe from NATS topic",
					zap.String("topic", topic),
					zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats unsubscribe failed")))
				errTopic = append(errTopic, topic)
				continue
			}
			delete(next, topic)
		}
	}
	q.subs.Store(next)
	return errTopic
}

func (q *Nats) upsertSub(topic string, sub *nats.Subscription, timeout time.Duration) {
	q.subsMu.Lock()
	defer q.subsMu.Unlock()

	cur := q.loadSubs()
	next := make(subsMap, len(cur)+1)
	for k, v := range cur {
		next[k] = v
	}
	next[topic] = &subItem{topic: topic, sub: sub, timeout: timeout}
	q.subs.Store(next)
}

// Broadcast publishes a message to topic.
// Broadcast 广播。
func (q *Nats) Broadcast(topic string, data []byte) error {
	if q.conn == nil {
		return errNoConn
	}
	zmetrics.NatsPublishTotal.Inc()
	err := q.conn.Publish(topic, data)
	if err != nil {
		zmetrics.NatsPublishErrors.Inc()
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats broadcast failed")
	}
	return nil
}

// Request sends request and waits for reply.
// Request 请求回复。
func (q *Nats) Request(topic string, data []byte) (*nats.Msg, error) {
	if q.conn == nil {
		return nil, errNoConn
	}
	zmetrics.NatsRequestTotal.Inc()
	start := time.Now()
	msg, err := q.conn.Request(topic, data, q.getTimeout(topic))
	zmetrics.NatsRequestLatency.ObserveDuration(time.Since(start))
	if err != nil {
		zmetrics.NatsRequestErrors.Inc()
		return nil, zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats request failed")
	}
	return msg, nil
}
