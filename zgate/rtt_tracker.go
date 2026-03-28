package zgate

import (
	"sync/atomic"
	"time"
)

// cacheLineSize is set for cache-line isolation.
// cacheLineSize 现代 CPU 通常为 64 字节。
const cacheLineSize = 128

// AtomicSlot stores one tracking slot with cache-line padding.
// AtomicSlot 确保每个槽位占据独立的缓存行。
// reqKey combines sessionId and seqId for uniqueness.
// 使用 reqKey（sessionId + seqId 组合）确保唯一性。
type AtomicSlot struct {
	sendTime atomic.Int64
	reqKey   atomic.Uint64 // High32=sessionId low32, Low32=seqId / 高32位: sessionId低32位, 低32位: seqId
	_        [cacheLineSize]byte
}

// LockFreeRTTTracker tracks request RTT samples without locks.
// LockFreeRTTTracker 无锁 RTT 追踪器，用于记录请求往返时延采样。
type LockFreeRTTTracker struct {
	buffer []AtomicSlot
	mask   uint32

	// Sample storage.
	// 采样存储。
	samples    []atomic.Int64
	sampleIdx  atomic.Uint32
	maxSamples uint32
}

// NewLockFreeRTTTracker creates a lock-free RTT tracker with ring buffer and sample storage.
// NewLockFreeRTTTracker 创建无锁 RTT 追踪器（环形槽位 + 采样存储）。
func NewLockFreeRTTTracker(bufferSize, maxSamples uint32) *LockFreeRTTTracker {
	size := nextPowerOfTwo(int(bufferSize))
	return &LockFreeRTTTracker{
		buffer:     make([]AtomicSlot, size),
		mask:       uint32(size - 1),
		samples:    make([]atomic.Int64, maxSamples),
		maxSamples: maxSamples,
	}
}

// Record records request send timestamp.
// Record 记录请求发送时间。
// sessionId: session ID, seqId: sequence ID.
// sessionId: 会话ID, seqId: 序列号。
func (t *LockFreeRTTTracker) Record(sessionId uint64, seqId uint32) {
	// Compose key: high32=sessionId low32, low32=seqId.
	// 组合 key: 高32位=sessionId低32位, 低32位=seqId。
	reqKey := (sessionId << 32) | uint64(seqId)

	// Compute slot index by reqKey.
	// 使用 reqKey 计算槽位索引。
	// reqKey low 32 bits equals seqId, so seqId avoids uint64->uint32 truncation warning.
	// reqKey 的低 32 位恒等于 seqId，因此直接使用 seqId 计算索引可避免 uint64->uint32 的截断告警。
	idx := seqId & t.mask
	slot := &t.buffer[idx]

	// Release semantics: make time write visible to subsequent reads.
	// Release 语义：确保 Time 的写入对后续读可见。
	slot.sendTime.Store(time.Now().UnixNano())
	slot.reqKey.Store(reqKey)
}

// Complete finalizes request and computes RTT.
// Complete 完成请求并计算 RTT。
// sessionId: session ID, seqId: sequence ID.
// sessionId: 会话ID, seqId: 序列号。
func (t *LockFreeRTTTracker) Complete(sessionId uint64, seqId uint32) (time.Duration, bool) {
	// Compose key: high32=sessionId low32, low32=seqId.
	// 组合 key: 高32位=sessionId低32位, 低32位=seqId。
	reqKey := (sessionId << 32) | uint64(seqId)

	// Compute slot index by reqKey.
	// 使用 reqKey 计算槽位索引。
	// Same as above: idx depends only on reqKey low 32 bits (seqId).
	// 同上：idx 仅依赖 reqKey 的低 32 位（即 seqId）。
	idx := seqId & t.mask
	slot := &t.buffer[idx]

	// Double-Check Locking Pattern (Lock-Free Ver.)

	// Check 1: fast filter with full reqKey (sessionId + seqId must match).
	// Check 1: 快速过滤（比较完整的 reqKey，确保 sessionId + seqId 都匹配）。
	if slot.reqKey.Load() != reqKey {
		return 0, false
	}

	// Load Data
	start := slot.sendTime.Load()
	if start == 0 {
		return 0, false
	}

	// Check 2: verify slot data was not overwritten.
	// Check 2: 确认数据未被篡改。
	if slot.reqKey.Load() != reqKey {
		return 0, false
	}

	now := time.Now().UnixNano()
	cost := now - start

	// Sanity Check
	if cost <= 0 || cost > 10*time.Second.Nanoseconds() {
		return 0, false
	}

	// Record sample atomically to avoid races.
	// 采样记录（使用原子操作避免竞态）。
	idxVal := t.sampleIdx.Add(1) - 1
	if idxVal < t.maxSamples {
		t.samples[idxVal].Store(cost)
	}

	return time.Duration(cost), true
}

// GetAndResetSamples returns collected samples and resets state.
// GetAndResetSamples 获取数据并重置（完全线程安全）。
func (t *LockFreeRTTTracker) GetAndResetSamples() []int64 {
	// 1) Atomically swap index so subsequent writes start at 0.
	// 1. 原子交换索引，立即使后续写入从 0 开始。
	count := t.sampleIdx.Swap(0)

	if count > t.maxSamples {
		count = t.maxSamples
	}

	// 2) Atomically read data to avoid races.
	// 2. 原子读取数据（避免竞态）。
	result := make([]int64, count)
	for i := uint32(0); i < count; i++ {
		result[i] = t.samples[i].Load()
	}

	return result
}

func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 2
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
