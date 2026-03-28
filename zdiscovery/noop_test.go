package zdiscovery

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/stretchr/testify/assert"
)

func TestNoopDiscovery(t *testing.T) {
	d := NewNoopDiscovery()

	cfg := zmodel.ActorConfig{Id: 1, ActorType: 1, Name: "test", Addr: "0.0.0.0:9001"}
	assert.NoError(t, d.Register(cfg))
	assert.NoError(t, d.Unregister(cfg))

	assert.Empty(t, d.FindRandom("/servers/1"))
	assert.Empty(t, d.FindPoll("/servers/1"))
	assert.Empty(t, d.FindMod(1, 123))
	assert.Nil(t, d.FindAllByPrefix("/servers"))
	assert.NotNil(t, d.Watch())
}
