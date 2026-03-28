package zdiscovery

import (
	"context"
	"testing"
)

func BenchmarkEtcd_FindPoll(b *testing.B) {
	cli := dialEtcd(b)
	defer cli.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d, err := NewEtcdDiscovery(ctx, cli)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = d.CloseAll() }()
	key := "/servers/877001"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = d.FindPoll(key)
	}
}
