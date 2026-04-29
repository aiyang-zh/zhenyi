package zactor

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zid"
	"github.com/aiyang-zh/zhenyi-base/zpub"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// SendActor sends one async message to target actor.
// SendActor 向目标 Actor 发送一条异步消息。
func (a *Actor) SendActor(actorId uint64, msg ziface.IMessage) {
	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(msg, m); err != nil {
		a.GetLogger().Error("Failed to serialize message for SendActor",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", msg.GetMsgId()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		return
	}
	m.MsgId = msg.GetMsgId()
	m.TarActor = actorId
	m.SrcActor = a.GetActorId()
	if isTraceEnabled() {
		traceGenerateIDs(m)
	} else {
		m.TraceIdHi = zid.NextFast()
		m.SpanId = a.GetActorId()
	}
	a.SendMsg(m.Retain())
}

// SendActorReply sends RPC reply for one request message.
// SendActorReply 为请求消息发送 RPC 回包。
func (a *Actor) SendActorReply(msg *zmsg.Message, reply ziface.IMessage) {
	if msg.RpcId == 0 {
		return
	}
	if reply == nil {
		a.GetLogger().Error("Reply message is nil in SendActorReply",
			zap.Uint64("targetActorId", msg.SrcActor),
			zap.Uint64("rpcId", msg.RpcId),
			zap.Error(zerrs.New(zerrs.ErrTypeValidation, "reply message cannot be nil")))
		// Write a unified internal-error placeholder instead of forcing caller timeout dependence.
		// 为避免调用方长期依赖超时，这里设置统一的内部错误占位。
		placeholder := zmsg.GetMessage()
		placeholder.RpcId = msg.RpcId
		placeholder.IsResponse = true
		placeholder.Data = placeholder.Data[:0]
		a.SetReply(placeholder)
		placeholder.Release()
		return
	}
	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(reply, m); err != nil {
		a.GetLogger().Error("Failed to serialize reply message",
			zap.Uint64("targetActorId", msg.SrcActor),
			zap.Int32("msgId", reply.GetMsgId()),
			zap.Uint64("rpcId", msg.RpcId),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		// Same fallback: write internal-error placeholder for explicit failure signaling.
		// 同样，为调用方写入内部错误占位，防止只能通过超时感知失败。
		placeholder := zmsg.GetMessage()
		placeholder.RpcId = msg.RpcId
		placeholder.IsResponse = true
		placeholder.Data = placeholder.Data[:0]
		a.SetReply(placeholder)
		placeholder.Release()
		return
	}
	m.MsgId = reply.GetMsgId()
	m.TarActor = msg.SrcActor
	m.SrcActor = a.GetActorId()
	m.IsResponse = true
	m.RpcId = msg.RpcId
	m.TraceIdHi = msg.TraceIdHi
	m.TraceIdLo = msg.TraceIdLo
	m.SpanId = a.GetActorId()
	m.SeqId = msg.SeqId
	a.SendMsg(m.Retain())
}

func (a *Actor) getCircuitBreaker(actorId uint64) *circuitBreaker {
	cb, ok := a.circuitBreakers[actorId]
	if !ok {
		cb = newCircuitBreaker()
		a.circuitBreakers[actorId] = cb
	}
	return cb
}

// CallActor sends RPC request with circuit-breaker protection.
// CallActor 发送 RPC 请求（带熔断保护）。
func (a *Actor) CallActor(actorId uint64, request ziface.IMessage, reply ziface.IMessage, timeout time.Duration) ziface.RpcReply {
	res := ziface.RpcReply{
		Code: ziface.ErrCode_Success,
		Msg:  "ok",
		Data: nil,
	}

	cb := a.getCircuitBreaker(actorId)
	if !cb.allow() {
		zmetrics.RPCCBTripped.Inc()
		a.GetLogger().Warn("RPC circuit breaker open, fast-fail",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", request.GetMsgId()))
		res.Code = ziface.ErrorCode_RpcErr
		res.Msg = "circuit breaker open"
		return res
	}
	zmetrics.RPCSent.Inc()
	rpcStart := time.Now()

	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(request, m); err != nil {
		a.GetLogger().Error("Failed to serialize RPC request",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", request.GetMsgId()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		res.Code = ziface.ErrorCode_Serialize
		res.Msg = "Serialize err"
		return res
	}
	rpcId, err := a.AddSender()
	if err != nil {
		a.GetLogger().Error("Failed to allocate RPC ID",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", request.GetMsgId()),
			zap.Error(err))
		res.Code = ziface.ErrorCode_RpcErr
		res.Msg = "rpc id err"
		return res
	}
	m.MsgId = request.GetMsgId()
	m.TarActor = actorId
	m.SrcActor = a.GetActorId()
	m.RpcId = rpcId
	if isTraceEnabled() {
		traceGenerateIDs(m)
	} else {
		m.TraceIdHi = zid.NextFast()
		m.SpanId = a.GetActorId()
	}
	a.SendMsg(m.Retain())
	data, ok := a.GetReply(rpcId, timeout)
	if data != nil {
		defer data.Release()
	}
	rpcCost := time.Since(rpcStart)
	zmetrics.RPCLatency.ObserveDuration(rpcCost)
	if !ok {
		cb.recordFailure()
		zmetrics.RPCTimeout.Inc()
		a.GetLogger().Warn("RPC call timeout",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", request.GetMsgId()),
			zap.Uint64("rpcId", rpcId),
			zap.Duration("timeout", timeout))
		res.Code = ziface.ErrorCode_RpcTimeOut
		res.Msg = "timeout"
		return res
	}

	cb.recordSuccess()
	zmetrics.RPCSuccess.Inc()

	err = reply.UnmarshalVT(data.Data)
	if err != nil {
		a.GetLogger().Error("Failed to deserialize RPC reply",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", reply.GetMsgId()),
			zap.Uint64("rpcId", rpcId),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf unmarshal failed")))
		res.Code = ziface.ErrorCode_DeSerialize
		res.Msg = "DeSerialize err"
		return res
	}
	res.Data = reply
	return res
}

// CallActorAll concurrently issues multi-way CallActor and blocks until all return (or each times out).
// It uses the same workerPool (ants) as AsyncRun, bounded by Actor.WorkSize, to avoid unbounded goroutines.
//
// Note: DO NOT call this in the current Actor mailbox thread and block-wait, otherwise callbacks may need
// mailbox delivery and can deadlock. Use non-mailbox goroutines (e.g., inside AsyncRun) instead.
//
// CallActorAll 并发 CallActor；适用于同一调用方需同时等待多路 Worker 回包的场景（如 DAG 同层）。
// 实现走与 AsyncRun 相同的 workerPool（ants），受 Actor.WorkSize 限制、共享 panic 回收，避免无界 go。
//
// 注意：不要在本 Actor 的邮箱处理线程上调用并阻塞等待（例如在某条入站消息的 Handler 里直接 CallActorAll + 等齐），
// 否则子任务完成后的 AsyncRun 回调需经邮箱投递，可能造成自锁。应在 AsyncRun 工作协程或其它非邮箱线程上调用。
func (a *Actor) CallActorAll(specs []ziface.RpcCallSpec, timeout time.Duration) []ziface.RpcReply {
	if len(specs) == 0 {
		return nil
	}
	out := make([]ziface.RpcReply, len(specs))
	var wg sync.WaitGroup
	for i := range specs {
		wg.Add(1)
		idx := i
		ok := a.AsyncRunResult(
			func() interface{} {
				out[idx] = a.CallActor(specs[idx].ActorID, specs[idx].Request, specs[idx].Reply, timeout)
				return nil
			},
			func(interface{}) { wg.Done() },
		)
		if !ok {
			// Submission failed (worker pool closed/full), complete this slot immediately.
			out[idx] = ziface.RpcReply{Code: ziface.ErrorCode_RpcErr, Msg: "async task submit failed"}
			wg.Done()
		}
	}
	wg.Wait()
	return out
}

// SendToClientByUserId sends one response-style message to a client via gate actor ID.
// SendToClientByUserId 通过网关 Actor ID 向客户端发送一条响应消息。
func (a *Actor) SendToClientByUserId(actorId uint64, userId uint64, clientMsg ziface.IMessage) {
	msgId := clientMsg.GetMsgId()
	if actorId == 0 {
		a.GetLogger().Error("Invalid actor ID for SendMsgToClient",
			zap.Uint64("actorId", actorId),
			zap.Int32("msgId", msgId),
			zap.Uint64("userId", userId),
			zap.Error(zerrs.Newf(zerrs.ErrTypeValidation, "actorId must be positive, got %d", actorId)))
		return
	}
	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(clientMsg, m); err != nil {
		a.GetLogger().Error("Failed to serialize client message",
			zap.Uint64("actorId", actorId),
			zap.Int32("msgId", clientMsg.GetMsgId()),
			zap.Uint64("userId", userId),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		return
	}
	m.MsgId = msgId
	m.IsResponse = true
	m.ToClient = true
	m.TarActor = actorId
	m.SrcActor = a.GetActorId()
	if isTraceEnabled() {
		traceGenerateIDs(m)
	} else {
		m.TraceIdHi = zid.NextFast()
		m.SpanId = uint64(a.GetActorId())
	}
	a.SendMsg(m.Retain())
}

// SendToClient sends one client response using source metadata from request envelope.
// SendToClient 基于请求信封元数据向客户端发送一条响应消息。
func (a *Actor) SendToClient(msg *zmsg.Message, clientMsg ziface.IMessage) {
	start := time.Now()
	msgId := clientMsg.GetMsgId()
	actorId := msg.SrcActor
	seqId := msg.SeqId
	if actorId == 0 {
		a.GetLogger().Error("Invalid actor ID for SendMsgToClient",
			zap.Uint64("actorId", actorId),
			zap.Int32("msgId", msgId),
			zap.Uint64("sessionId", msg.SessionId),
			zap.Error(zerrs.Newf(zerrs.ErrTypeValidation, "actorId must be non-zero, got %d", actorId)))
		return
	}
	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(clientMsg, m); err != nil {
		a.GetLogger().Error("Failed to serialize client message",
			zap.Uint64("actorId", actorId),
			zap.Int32("msgId", clientMsg.GetMsgId()),
			zap.Uint64("sessionId", msg.SessionId),
			zap.Uint32("seqId", seqId),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		return
	}
	afterMarshal := time.Now()
	marshalCost := afterMarshal.Sub(start)
	m.MsgId = msgId
	m.IsResponse = true
	m.ToClient = true
	m.TarActor = actorId
	m.SeqId = seqId
	m.SrcActor = a.GetActorId()
	m.TraceIdHi = msg.TraceIdHi
	m.TraceIdLo = msg.TraceIdLo
	m.SpanId = a.GetActorId()
	m.SessionId = msg.SessionId
	a.SendMsg(m.Retain())
	afterSend := time.Now()
	sendCost := afterSend.Sub(afterMarshal)
	totalCost := afterSend.Sub(start)
	if totalCost > zmodel.GetFrameworkTuning().SlowLogThreshold {
		a.GetLogger().Warn("Slow SendToClient path",
			zap.Uint64("actorId", a.GetActorId()),
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", msgId),
			zap.Uint64("sessionId", msg.SessionId),
			zap.Uint32("seqId", seqId),
			zap.Duration("cost", totalCost),
			zap.Duration("marshalCost", marshalCost),
			zap.Duration("sendMsgCost", sendCost))
	}
}

// SendMsg delivers message to local actor or remote bus by target actor ID.
// SendMsg 按目标 Actor ID 将消息投递到本地 Actor 或远端总线。
func (a *Actor) SendMsg(msg *zmsg.Message) {
	defer msg.Release()
	actorId := msg.TarActor
	if actorId <= 0 {
		a.GetLogger().Error("Invalid target actor ID in SendMsg",
			zap.Uint64("targetActorId", actorId),
			zap.Int32("msgId", msg.MsgId),
			zap.Uint64("srcActorId", a.GetActorId()),
			zap.Error(zerrs.Newf(zerrs.ErrTypeValidation, "target actorId must be positive, got %d", actorId)))
		return
	}
	msg.SpanId = a.GetActorId()
	actor := a.GetActorById(actorId)
	if actor != nil { // Direct enqueue when target actor is in current process / 在同一个进程直接将消息发送到 actor 队列
		// Push selects proper path by message type automatically.
		// Push 会自动判断消息类型并选择合适的发送方式。
		// Response messages go through asyncSendChan and worker pool.
		// 响应消息会进入 asyncSendChan，由 worker pool 处理。
		actor.Push(zmodel.ActorCmd{
			Type: zmodel.CmdTypeMsg,
			Msg:  msg.Retain(),
		})
		return
	}
	// Lookup target actor from other processes.
	// 查询其他进程 actor。
	modelActor, ok := a.GetOtherActorById(actorId)
	if ok {
		msg.Retain()
		a.AsyncRun(func() interface{} {
			defer msg.Release()
			mBuf, err := msg.MarshalPooled()
			if err != nil {
				a.GetLogger().Error("Failed to serialize message for remote actor",
					zap.Uint64("targetActorId", actorId),
					zap.Int32("msgId", msg.MsgId),
					zap.String("topic", modelActor.GetTopic()),
					zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
				return nil
			}
			err = a.sendRemote(modelActor.GetTopic(), mBuf.B)
			mBuf.Release() // NATS 已拷贝，立即归还
			if err != nil {
				a.GetLogger().Error("Failed to send message via NATS",
					zap.Uint64("targetActorId", actorId),
					zap.Int32("msgId", msg.MsgId),
					zap.String("topic", modelActor.GetTopic()),
					zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats publish failed")))
			}
			return nil
		}, nil)
		return
	}
	a.GetLogger().Error("Target actor not found",
		zap.Uint64("targetActorId", actorId),
		zap.Int32("msgId", msg.MsgId),
		zap.Uint64("srcActorId", a.GetActorId()),
		zap.Error(zerrs.Newf(zerrs.ErrTypeNotFound, "actor %d not found in local or remote", actorId)))
}

func (a *Actor) sendRemote(topic string, data []byte) error {
	if zbus.DefaultBus == nil {
		return zerrs.New(zerrs.ErrTypeNetwork, "remote bus is not configured")
	}
	return zbus.DefaultBus.Broadcast(topic, data)
}

func (a *Actor) broadcast(topic string, data []byte) error {
	return a.sendRemote(topic, data)
}

// Broadcast publishes one message to a topic.
// Broadcast 向指定 topic 广播一条消息。
func (a *Actor) Broadcast(topic string, msg ziface.IMessage) error {
	m := zmsg.GetMessage()
	defer m.Release()
	if err := ziface.MarshalVTToMsg(msg, m); err != nil {
		a.GetLogger().Error("Failed to serialize broadcast message",
			zap.String("topic", topic),
			zap.Int32("msgId", msg.GetMsgId()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		return err
	}
	m.MsgId = msg.GetMsgId()
	m.SrcActor = a.GetActorId()
	if isTraceEnabled() {
		traceGenerateIDs(m)
	} else {
		m.TraceIdHi = zid.NextFast()
		m.SpanId = a.GetActorId()
	}
	if a.IsSingle() {
		// ✅ 本地模式直接发布，无需信封序列化
		zpub.EventSystem.Publish(&zpub.Event{
			Topic: topic,
			Val:   m,
		})
	} else {
		mBuf, err := m.MarshalPooled()
		if err != nil {
			a.GetLogger().Error("Failed to serialize message envelope for broadcast",
				zap.String("topic", topic),
				zap.Int32("msgId", msg.GetMsgId()),
				zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "message envelope marshal failed")))
			return err
		}
		err = a.broadcast(topic, mBuf.B)
		mBuf.Release() // NATS 已拷贝，立即归还
		if err != nil {
			a.GetLogger().Error("Failed to broadcast message via NATS",
				zap.String("topic", topic),
				zap.Int32("msgId", msg.GetMsgId()),
				zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "nats broadcast failed")))
			return err
		}
	}
	return nil
}

// BatchSendToClients 将同一条下行 payload 送达多个 Gate 连接（payload 仅 MarshalVT 一次，每条投递复制 Data，避免异步队列间共享切片）。
// actorUserMap：key 为 Gate Actor ID（与 SendToClient 中 TarActor 一致），value 为 channel/session id 列表（与 SessionId 一致）。
// origin 非 nil 时复用其 TraceIdHi/TraceIdLo（与触发请求对齐）；推送式下行 SeqId 固定为 0。
func (a *Actor) BatchSendToClients(origin *zmsg.Message, actorUserMap map[uint64][]int64, clientMsg ziface.IMessage) {
	if len(actorUserMap) == 0 {
		return
	}
	body, err := clientMsg.MarshalVT()
	if err != nil {
		a.GetLogger().Error("Failed to serialize message for batch send to clients",
			zap.Int32("msgId", clientMsg.GetMsgId()),
			zap.Int("actorCount", len(actorUserMap)),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "protobuf marshal failed")))
		return
	}
	msgId := clientMsg.GetMsgId()
	srcActor := a.GetActorId()
	for actorId, sessions := range actorUserMap {
		if actorId <= 0 || len(sessions) == 0 {
			continue
		}
		for _, sid := range sessions {
			m := zmsg.GetMessage()
			m.MsgId = msgId
			m.Data = append([]byte(nil), body...)
			m.IsResponse = true
			m.ToClient = true
			m.TarActor = actorId
			m.SrcActor = srcActor
			m.SessionId = uint64(sid)
			m.SeqId = 0
			if origin != nil {
				m.TraceIdHi = origin.TraceIdHi
				m.TraceIdLo = origin.TraceIdLo
			} else if isTraceEnabled() {
				traceGenerateIDs(m)
			} else {
				m.TraceIdHi = zid.NextFast()
			}
			m.SpanId = a.GetActorId()
			a.SendMsg(m.Retain())
			m.Release()
		}
	}
}
