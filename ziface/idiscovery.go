package ziface

import (
	"github.com/aiyang-zh/zhenyi/zmodel"
)

// Discoverer defines service discovery contract for actor registration and lookup.
// Discoverer 定义服务发现契约：Actor 注册、查找与变更订阅。
type Discoverer interface {
	// FindRandom selects an Actor by key using random strategy.
	// FindRandom 按 key 随机选择一个 Actor。
	FindRandom(key string) zmodel.ActorConfig
	// FindPoll selects an Actor by key using round-robin strategy.
	// FindPoll 按 key 轮询选择一个 Actor。
	FindPoll(key string) zmodel.ActorConfig
	// FindMod selects an Actor by user ID modulo (sticky routing).
	// FindMod 按用户 ID 取模选择 Actor（常用于会话粘性）。
	FindMod(actorType uint32, userId uint64) zmodel.ActorConfig
	// Unregister removes an Actor registration.
	// Unregister 注销一个 Actor 注册信息。
	Unregister(c zmodel.ActorConfig) error
	// Register adds an Actor registration into discovery.
	// Register 注册一个 Actor 到发现系统。
	Register(c zmodel.ActorConfig) error
	// Watch returns a change stream of Actor configs.
	// Watch 返回 Actor 配置变更流。
	Watch() chan zmodel.ActorConfig
	// FindAllByPrefix returns all registrations by key prefix.
	// FindAllByPrefix 按前缀查询所有注册项。
	FindAllByPrefix(key string) []zmodel.ActorServerRegister
}
