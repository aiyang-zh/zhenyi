package zactor

import (
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// StartAsyncThen is a "zero-Promise" fast path for the common pattern:
// "run work asynchronously -> write back on the Actor thread".
// StartAsyncThen 是一个“零 Promise”快路径，用于典型的「异步执行 work -> 回到 Actor 线程写回状态」场景。
//
// Usage notes:
// 使用注意：
//   - work runs in a worker-pool goroutine: MUST NOT read/write Actor private state
//     (maps/slices/counters/state machine, etc.). Only do computation or I/O decoupled from Actor state.
//   - work 在 worker pool 的协程中执行：禁止读/写 Actor 私有状态（包括 map/slice/计数器/状态机等），只能做与 Actor 状态解耦的计算或 I/O。
//   - then runs on the Actor mailbox (main) thread: it is safe to update Actor state here; keep it non-blocking
//     (no sleep, no synchronous I/O, no blocking channel ops, no long-running computation).
//   - then 在 Actor mailbox 主线程执行：这里才允许更新 Actor 状态；同时必须保持非阻塞（不要 sleep/同步 I/O/阻塞 channel/长耗时计算）。
//   - msg lifetime: when msg != nil, it will be Retain()'d internally for cross-goroutine safety and Release()'d
//     internally along the established flow; user code should NOT manually Release() it in work/then.
//   - msg 生命周期：当 msg != nil 时，内部会先 Retain() 以支持跨协程安全，并在内部按既定流程 Release()；业务侧不需要也不应在 work/then 里手动 Release()。
//   - return value only indicates whether the task was successfully submitted to the worker pool;
//     if false, then callback will not be invoked.
//   - 返回值仅表示是否成功提交到 worker pool；返回 false 时不会触发 then 回调。
func (a *Actor) StartAsyncThen(msg *zmsg.Message, work func(*zmsg.Message) (interface{}, error), then func(*Actor, interface{}, error)) bool {
	if a == nil || work == nil {
		return false
	}
	task := getAsyncTask()
	task.actor = a
	task.msg = msg
	if msg != nil {
		msg.Retain()
	}
	task.flowWork = work
	task.flowThen = then
	if err := a.workerPool.Invoke(task); err != nil {
		if msg != nil {
			msg.Release()
		}
		putAsyncTask(task)
		return false
	}
	return true
}
