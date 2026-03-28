package zmetrics

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zpoolobs"
)

func TestPoolObserver_WritePrometheus(t *testing.T) {
	zpoolobs.SetObserver(GlobalPoolObserver())
	t.Cleanup(func() { zpoolobs.SetObserver(nil) })

	// 注意：PoolObserver 的指标聚合是进程内单例（map 不会因为 SetObserver(nil) 被清空）。
	// 因此这里使用唯一 pool 名，避免在 -count 或并发测试下出现历史数据累计。
	name := fmt.Sprintf("test.pool.zmetrics.write.%d", time.Now().UnixNano())
	p := zpoolobs.NewObservedPool(name, func() int { return 0 })
	_ = p.Get()
	p.Put(0)

	var b strings.Builder
	WritePoolPrometheus(&b)
	out := b.String()
	wantGet := `zhenyi_zpool_get_total{pool="` + name + `"} 1`
	if !strings.Contains(out, wantGet) {
		t.Fatalf("missing get counter %q in:\n%s", wantGet, out)
	}
	wantOut := `zhenyi_zpool_outstanding{pool="` + name + `"} 0`
	if !strings.Contains(out, wantOut) {
		t.Fatalf("missing outstanding %q in:\n%s", wantOut, out)
	}
}
