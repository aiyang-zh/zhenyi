package zactor

import (
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// DefaultLocalRouter is a simple msgId-based local router.
// DefaultLocalRouter 是一个基于 msgId 的简单本地路由实现：
//   - It gets candidate actors from Group.LookupActorsByMsgID(msg.MsgId).
//   - 先通过 Group.LookupActorsByMsgID(msg.MsgId) 获取候选 Actor 列表；
//   - It returns directly when only one candidate exists.
//   - 若只有一个候选，直接返回；
//   - For multiple candidates, current behavior picks the first; can be extended by message/context sharding.
//   - 若有多个候选，当前版本按“首个候选”返回，后续可扩展为基于 msg/上下文的分片策略。
type DefaultLocalRouter struct{}

// NewDefaultLocalRouter creates a default local router.
// NewDefaultLocalRouter 创建默认本地路由器。
func NewDefaultLocalRouter() *DefaultLocalRouter {
	return &DefaultLocalRouter{}
}

// RouteLocal chooses one local actor for given message.
// RouteLocal 为给定消息选择一个本地 Actor。
func (r *DefaultLocalRouter) RouteLocal(group ziface.IGroup, msg *zmsg.Message) (ziface.IActor, error) {
	if group == nil || msg == nil {
		return nil, zerrs.New(zerrs.ErrTypeInternal, "RouteLocal: group or message is nil")
	}

	// Prefer optional allocation-free fast path; fallback to copy-returning legacy API.
	// 优先走可选的零分配快路径（若 group 实现支持）；否则回退到副本语义（兼容旧接口）。
	if fast, ok := any(group).(ziface.IGroupRouteTableView); ok {
		actors := fast.LookupActorsByMsgIDView(msg.MsgId)
		if len(actors) == 0 {
			return nil, zerrs.Newf(zerrs.ErrTypeNotFound, "no local actor found for msgId %d", msg.MsgId)
		}
		return actors[0], nil
	}

	actors := group.LookupActorsByMsgID(msg.MsgId)
	if len(actors) == 0 {
		return nil, zerrs.Newf(zerrs.ErrTypeNotFound, "no local actor found for msgId %d", msg.MsgId)
	}

	// TODO: extend with message-aware shard/sticky routing (session/tenant/shardKey).
	// TODO: 后续可在此按 msg 内容做分片/粘性路由（如按 session/tenant/shardKey 等字段）。
	return actors[0], nil
}
