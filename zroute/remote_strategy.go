package zroute

import (
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// RemoteRouteKeyFunc provides a sticky-routing key.
// RemoteRouteKeyFunc 提供“粘性路由 key”。
// Returning 0 means "no key", and strategy may fallback to random/round-robin/first.
// 返回值为 0 表示“无 key”，策略可选择退化为随机/轮询/第一个候选。
type RemoteRouteKeyFunc func(msg *zmsg.Message) uint64

// DefaultRemoteRouteKey is the framework default key selector.
// DefaultRemoteRouteKey 作为框架默认：优先 SessionId，其次 RpcId。
func DefaultRemoteRouteKey(msg *zmsg.Message) uint64 {
	if msg == nil {
		return 0
	}
	if msg.SessionId != 0 {
		return msg.SessionId
	}
	if msg.RpcId != 0 {
		return msg.RpcId
	}
	return 0
}

// RemoteRouteStrategy selects the preferred candidate index from remote actors.
// RemoteRouteStrategy 负责从候选远程 Actor 中选出“首选候选”索引。
// Gate retries remaining candidates if the preferred one fails.
// Gate 在首选失败后会继续尝试其余候选（fallback）。
type RemoteRouteStrategy interface {
	PickOne(msg *zmsg.Message, candidates []zmodel.ActorConfig) int
}

// FirstCandidateStrategy preserves legacy behavior and picks index 0.
// FirstCandidateStrategy 保持旧行为：直接返回原候选顺序（Gate 侧默认就是发现顺序）。
type FirstCandidateStrategy struct{}

func (FirstCandidateStrategy) PickOne(_ *zmsg.Message, candidates []zmodel.ActorConfig) int {
	if len(candidates) == 0 {
		return -1
	}
	return 0
}

// RoundRobinStrategy performs simple round-robin selection.
// RoundRobinStrategy 在候选集上做简单轮询（无粘性，不保证同 key 落同分片）。
type RoundRobinStrategy struct {
	seq atomic.Uint64
}

func (s *RoundRobinStrategy) PickOne(_ *zmsg.Message, candidates []zmodel.ActorConfig) int {
	n := len(candidates)
	if n == 0 {
		return -1
	}
	return int(s.seq.Add(1) % uint64(n))
}

// RendezvousHashStrategy (HRW) uses consistent hashing and picks highest score.
// RendezvousHashStrategy（HRW）一致性哈希：基于 key 对候选排序，优先选择分数最高者。
// It has low disruption when instances scale up/down.
// 该算法对“实例增减”的扰动较小，适合扩缩容。
type RendezvousHashStrategy struct {
	KeyFunc RemoteRouteKeyFunc
}

func (s *RendezvousHashStrategy) PickOne(msg *zmsg.Message, candidates []zmodel.ActorConfig) int {
	n := len(candidates)
	if n == 0 {
		return -1
	}
	if n == 1 {
		return 0
	}
	keyFn := s.KeyFunc
	if keyFn == nil {
		keyFn = DefaultRemoteRouteKey
	}
	key := keyFn(msg)
	if key == 0 {
		// Fallback to first candidate when no key exists.
		// 无 key 时退化为“第一个候选优先”。
		return 0
	}
	// Scan linearly and return best-score index without allocations.
	// 线性扫描选出最高分候选，返回其下标（零分配）。
	bestIdx := -1
	var bestScore uint64
	for i, c := range candidates {
		score := hrwScore(key, uint64(c.Id), uint64(c.Process))
		if bestIdx < 0 || score > bestScore {
			bestIdx = i
			bestScore = score
		}
	}
	return bestIdx
}

func hrwScore(key uint64, actorID uint64, process uint64) uint64 {
	// Lightweight allocation-free mixer for HRW relative ranking.
	// 简单无分配混合函数：基于 64-bit 乘法和异或，足够用于 HRW 相对排序。
	// Constants are common 64-bit hash multipliers.
	// 常量取自常用 64-bit hash 乘子。
	const (
		c1 = 0x9e3779b185ebca87
		c2 = 0xc2b2ae3d27d4eb4f
	)
	x := key ^ (actorID + 0x100000001b3*process)
	x ^= x >> 33
	x *= c1
	x ^= x >> 29
	x *= c2
	x ^= x >> 32
	return x
}
