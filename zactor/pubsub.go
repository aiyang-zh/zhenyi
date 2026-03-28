package zactor

import (
	"github.com/aiyang-zh/zhenyi-base/zpub"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// LocalActorBroadcast adapts zpub events into actor commands.
// LocalActorBroadcast 将 zpub 事件适配为 Actor 命令。
type LocalActorBroadcast struct {
	a *Actor
}

// OnChange handles local pub-sub events and forwards message to actor mailbox.
// OnChange 处理本地发布订阅事件，并将消息转发到 Actor 邮箱。
func (c *LocalActorBroadcast) OnChange(e *zpub.Event) {
	msg, ok := e.Val.(*zmsg.Message)
	if !ok || msg == nil {
		return
	}
	c.a.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeMsg, Msg: msg.Retain()})
}

// pubSub subscribes local and remote topics for current actor.
// pubSub 为当前 Actor 订阅本地与远端 topic。
func (a *Actor) pubSub() {
	zpub.EventSystem.Subscribe(a.GetNameTopic(), &LocalActorBroadcast{a: a})
	zpub.EventSystem.Subscribe(a.GetBroadcastTopic(), &LocalActorBroadcast{a: a})
	zpub.EventSystem.Subscribe(a.GetTopic(), &LocalActorBroadcast{a: a})
	a.subscribeRemoteBus()
}

// subscribeRemoteBus subscribes to remote message bus for cross-process delivery.
// subscribeRemoteBus 订阅远端消息总线（跨进程）。
// The concrete implementation is determined by zbus.DefaultBus.
// 具体实现由 zbus.DefaultBus 决定（NATS / 其他 MQ / 自定义）。
func (a *Actor) subscribeRemoteBus() {
	if zbus.DefaultBus == nil {
		return
	}
	a.GetLogger().Debug("subscribe remote bus",
		zap.String("topic", a.GetTopic()),
		zap.String("nameTopic", a.GetNameTopic()),
		zap.String("broadcastTopic", a.GetBroadcastTopic()))

	add := func(sub zbus.Subscription, err error) {
		if err != nil || sub == nil {
			return
		}
		a.subsMu.Lock()
		a.subs = append(a.subs, sub)
		a.subsMu.Unlock()
	}

	add(zbus.DefaultBus.Subscribe(a.GetTopic(), a.receiveRemote))
	add(zbus.DefaultBus.Subscribe(a.GetNameTopic(), a.receiveRemote))
	add(zbus.DefaultBus.Subscribe(a.GetBroadcastTopic(), a.receiveRemote))
}
