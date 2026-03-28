package zdiscovery

import (
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

var _ ziface.Discoverer = (*NoopDiscovery)(nil)

// NoopDiscovery is a no-op implementation used in single-node or testing scenarios.
// NoopDiscovery 空实现，用于单机或测试场景。
// Register/Unregister are no-ops, Find* return zero-value, Watch returns a never-closed empty channel.
// Register/Unregister 无操作，Find* 返回零值，Watch 返回不会关闭的空 channel。
func NewNoopDiscovery() *NoopDiscovery {
	return &NoopDiscovery{
		ch: make(chan zmodel.ActorConfig),
	}
}

// NoopDiscovery implements ziface.Discoverer with no remote state.
// NoopDiscovery 实现 ziface.Discoverer，但不维护任何远端状态。
type NoopDiscovery struct {
	ch chan zmodel.ActorConfig
}

func (n *NoopDiscovery) FindRandom(_ string) zmodel.ActorConfig                { return zmodel.ActorConfig{} }
func (n *NoopDiscovery) FindPoll(_ string) zmodel.ActorConfig                  { return zmodel.ActorConfig{} }
func (n *NoopDiscovery) FindMod(_ uint32, _ uint64) zmodel.ActorConfig         { return zmodel.ActorConfig{} }
func (n *NoopDiscovery) Register(_ zmodel.ActorConfig) error                   { return nil }
func (n *NoopDiscovery) Unregister(_ zmodel.ActorConfig) error                 { return nil }
func (n *NoopDiscovery) Watch() chan zmodel.ActorConfig                        { return n.ch }
func (n *NoopDiscovery) FindAllByPrefix(_ string) []zmodel.ActorServerRegister { return nil }
