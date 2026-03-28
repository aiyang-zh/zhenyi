package zdiscovery

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/aiyang-zh/zhenyi/zmodel"
)

func etcdEndpoints() []string {
	s := os.Getenv("ZDISCOVERY_ETCD_ENDPOINTS")
	if s == "" {
		s = "127.0.0.1:2379"
	}
	out := strings.Split(s, ",")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func dialEtcd(tb testing.TB) *clientv3.Client {
	tb.Helper()
	var cli *clientv3.Client
	var err error
	for i := 0; i < 40; i++ {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   etcdEndpoints(),
			DialTimeout: 3 * time.Second,
		})
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, err = cli.Get(ctx, "/")
			cancel()
			if err == nil {
				return cli
			}
			_ = cli.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	tb.Fatalf("etcd 不可用: %v", err)
	return nil
}

const testEtcdActorType uint32 = 877001

func TestIntegration_EtcdDiscovery_Flow(t *testing.T) {
	cli := dialEtcd(t)
	defer cli.Close()

	ctx := context.Background()
	_, _ = cli.Delete(ctx, "/servers/877001/", clientv3.WithPrefix())
	_, err := cli.Put(ctx, "/servers/877001/999", "not-valid-json")
	require.NoError(t, err)

	dctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := NewEtcdDiscovery(dctx, cli)
	require.NoError(t, err)
	require.NotNil(t, d)
	defer func() { _ = d.CloseAll() }()

	time.Sleep(300 * time.Millisecond)

	a1 := zmodel.ActorConfig{Id: 1, ActorType: testEtcdActorType, Name: "a1", Addr: "127.0.0.1:1001", Process: 1, Index: 0}
	a2 := zmodel.ActorConfig{Id: 2, ActorType: testEtcdActorType, Name: "a2", Addr: "127.0.0.1:1002", Process: 1, Index: 0}
	require.NoError(t, d.Register(a1))
	require.NoError(t, d.Register(a2))

	key := "/servers/877001"
	all := d.FindAllByPrefix(key)
	require.Len(t, all, 2)

	poll1 := d.FindPoll(key)
	poll2 := d.FindPoll(key)
	poll3 := d.FindPoll(key)
	ids := map[uint64]bool{poll1.Id: true, poll2.Id: true, poll3.Id: true}
	assert.True(t, ids[1] && ids[2], "轮询应覆盖两台")

	mod := d.FindMod(testEtcdActorType, 0)
	assert.Contains(t, []uint64{1, 2}, mod.Id)
	mod2 := d.FindMod(testEtcdActorType, uint64(len(all)))
	assert.NotNil(t, mod2.Id)

	r := d.FindRandom(key)
	assert.Contains(t, []uint64{1, 2}, r.Id)

	full := d.FindAllByPrefix("/servers")
	assert.GreaterOrEqual(t, len(full), 2)

	a1updated := a1
	a1updated.Name = "a1-updated"
	require.NoError(t, d.Register(a1updated))
	got := d.FindPoll(key)
	for i := 0; i < 6 && got.Name != "a1-updated"; i++ {
		time.Sleep(100 * time.Millisecond)
		got = d.FindPoll(key)
	}
	assert.Equal(t, "a1-updated", got.Name)

	require.NoError(t, d.Unregister(a2))
	time.Sleep(200 * time.Millisecond)
	one := d.FindAllByPrefix(key)
	require.Len(t, one, 1)
	assert.Equal(t, uint64(1), one[0].ActorConfig.Id)

	ch := d.Watch()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
	}

	require.NoError(t, d.CloseAll())
	cancel()
	time.Sleep(200 * time.Millisecond)
}

func TestIntegration_EtcdDiscovery_InvalidPrefix(t *testing.T) {
	cli := dialEtcd(t)
	defer cli.Close()
	d, err := NewEtcdDiscovery(context.Background(), cli)
	require.NoError(t, err)
	defer func() { _ = d.CloseAll() }()

	assert.Nil(t, d.FindAllByPrefix("/bad"))
	assert.Nil(t, d.FindAllByPrefix("/servers/notint"))
	assert.Equal(t, zmodel.ActorConfig{}, d.FindPoll("/servers/999999"))
	assert.Equal(t, zmodel.ActorConfig{}, d.FindMod(999998, 1))
}
