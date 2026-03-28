package zdiscovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aiyang-zh/zhenyi/zmodel"
)

func TestNewEtcdDiscovery_NilClient(t *testing.T) {
	d, err := NewEtcdDiscovery(context.Background(), nil)
	assert.Nil(t, d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestPtrToMap(t *testing.T) {
	assert.Nil(t, ptrToMap(nil))
	m := map[uint32][]zmodel.ActorConfig{1: {{Id: 1}}}
	assert.Equal(t, m, ptrToMap(&m))
}

func TestPtrToRegistered(t *testing.T) {
	assert.Nil(t, ptrToRegistered(nil))
	m := map[string]zmodel.ActorConfig{"/servers/1/1": {Id: 1}}
	assert.Equal(t, m, ptrToRegistered(&m))
}

func TestCloneCache(t *testing.T) {
	assert.Nil(t, cloneCache(nil))
	src := map[uint32][]zmodel.ActorConfig{
		1: {{Id: 1, ActorType: 1}, {Id: 2, ActorType: 1}},
	}
	cp := cloneCache(src)
	require.Len(t, cp[1], 2)
	cp[1][0].Id = 99
	assert.Equal(t, uint64(1), src[1][0].Id, "源 map 不应被修改")
}

func TestCloneRegistered(t *testing.T) {
	assert.Nil(t, cloneRegistered(nil))
	src := map[string]zmodel.ActorConfig{"k": {Id: 1}}
	cp := cloneRegistered(src)
	cp["k"] = zmodel.ActorConfig{Id: 2}
	assert.Equal(t, uint64(1), src["k"].Id)
}

func TestParseKeyToActorType(t *testing.T) {
	typ, ok := parseKeyToActorType("/servers/1")
	assert.True(t, ok)
	assert.Equal(t, uint32(1), typ)
	typ, ok = parseKeyToActorType("servers/877001")
	assert.True(t, ok)
	assert.Equal(t, uint32(877001), typ)
	_, ok = parseKeyToActorType("/servers")
	assert.False(t, ok)
	_, ok = parseKeyToActorType("/servers/")
	assert.False(t, ok)
	_, ok = parseKeyToActorType("/bad/1")
	assert.False(t, ok)
}
