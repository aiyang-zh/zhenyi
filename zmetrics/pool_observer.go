package zmetrics

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/ziface"
)

// PoolObserver exports pool events into zmetrics / Prometheus text (label: pool), same source as zmetrics.Enable /metrics.
// PoolObserver 将对象池事件导出为 zmetrics / Prometheus text（label: pool），与 zmetrics.Enable 的 /metrics 同源。
//
// Metric naming convention:
// 指标名约定：
//   - zhenyi_zpool_get_total{pool="..."}
//   - zhenyi_zpool_put_total{pool="..."}
//   - zhenyi_zpool_new_total{pool="..."}
//   - zhenyi_zpool_put_nil_total{pool="..."}
//   - zhenyi_zpool_outstanding{pool="..."} (gauge)
type PoolObserver struct{}

type poolObEntry struct {
	getTotal    atomic.Int64
	putTotal    atomic.Int64
	newTotal    atomic.Int64
	putNilTotal atomic.Int64
	outstanding atomic.Int64
}

var (
	poolMetricsSingleton PoolObserver
	poolObMap            sync.Map // string -> *poolObEntry
)

var _ ziface.IPoolObserver = (*PoolObserver)(nil)

// GlobalPoolObserver returns process-wide singleton implementing ziface.IPoolObserver.
// GlobalPoolObserver 返回进程内单例，实现 ziface.IPoolObserver；可传给 zpoolobs.SetObserver。
func GlobalPoolObserver() ziface.IPoolObserver {
	return &poolMetricsSingleton
}

func (p *PoolObserver) entry(name string) *poolObEntry {
	if name == "" {
		name = "(unnamed)"
	}
	if v, ok := poolObMap.Load(name); ok {
		return v.(*poolObEntry)
	}
	e := &poolObEntry{}
	actual, _ := poolObMap.LoadOrStore(name, e)
	return actual.(*poolObEntry)
}

func (p *PoolObserver) OnPoolCreate(name string) { _ = p.entry(name) }
func (p *PoolObserver) OnNew(name string)        { p.entry(name).newTotal.Add(1) }
func (p *PoolObserver) OnGet(name string) {
	e := p.entry(name)
	e.getTotal.Add(1)
	e.outstanding.Add(1)
}
func (p *PoolObserver) OnPut(name string) {
	e := p.entry(name)
	e.putTotal.Add(1)
	e.outstanding.Add(-1)
}
func (p *PoolObserver) OnPutNil(name string) { p.entry(name).putNilTotal.Add(1) }

func escapePromPoolLabel(s string) string {
	if !strings.ContainsAny(s, `"\`) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// WritePoolPrometheus outputs zpool metrics with pool label; called from Registry.WritePrometheus.
// WritePoolPrometheus 输出带 pool 标签的 zpool 指标；由 Registry.WritePrometheus 调用。
func WritePoolPrometheus(b *strings.Builder) {
	var names []string
	poolObMap.Range(func(key, _ any) bool {
		names = append(names, key.(string))
		return true
	})
	if len(names) == 0 {
		return
	}
	sort.Strings(names)

	const (
		mGet = "zhenyi_zpool_get_total"
		mPut = "zhenyi_zpool_put_total"
		mNew = "zhenyi_zpool_new_total"
		mNil = "zhenyi_zpool_put_nil_total"
		mOut = "zhenyi_zpool_outstanding"
	)

	writeMeta := func(metric, typ, help string) {
		b.WriteString("# HELP ")
		b.WriteString(metric)
		b.WriteByte(' ')
		b.WriteString(help)
		b.WriteByte('\n')
		b.WriteString("# TYPE ")
		b.WriteString(metric)
		b.WriteByte(' ')
		b.WriteString(typ)
		b.WriteByte('\n')
	}
	writeMeta(mGet, "counter", "zpool Get total")
	writeMeta(mPut, "counter", "zpool Put total")
	writeMeta(mNew, "counter", "zpool New (sync.Pool.New) total")
	writeMeta(mNil, "counter", "zpool Put(nil) total")
	writeMeta(mOut, "gauge", "zpool outstanding (Get-Put)")

	for _, name := range names {
		v, ok := poolObMap.Load(name)
		if !ok {
			continue
		}
		e := v.(*poolObEntry)
		lbl := escapePromPoolLabel(name)
		writeLabeled := func(metric string, val int64) {
			b.WriteString(metric)
			b.WriteString(`{pool="`)
			b.WriteString(lbl)
			b.WriteString(`"} `)
			appendInt(b, val)
			b.WriteByte('\n')
		}
		writeLabeled(mGet, e.getTotal.Load())
		writeLabeled(mPut, e.putTotal.Load())
		writeLabeled(mNew, e.newTotal.Load())
		writeLabeled(mNil, e.putNilTotal.Load())
		writeLabeled(mOut, e.outstanding.Load())
	}
}
