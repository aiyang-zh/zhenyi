package zmetrics

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi/zmonitor"
)

// monitorExportMgr is written by RegisterMonitorManager / (*Server).SetMonitorManager.
// monitorExportMgr 由 RegisterMonitorManager / (*Server).SetMonitorManager 写入；
// Registry.WritePrometheus 末尾调用 WriteMonitorSnapshotPrometheus。
var monitorExportMgr atomic.Pointer[zmonitor.Manager]

// RegisterMonitorManager registers zmonitor.Manager to append zhenyi_monitor_snapshot metrics.
// RegisterMonitorManager 注册 zmonitor.Manager，使 /metrics 追加 zhenyi_monitor_snapshot 系列。
// It writes the same global pointer as (*Server).SetMonitorManager and can be called standalone.
// 与 (*Server).SetMonitorManager 写入同一全局指针；未使用 zmetrics.Server 时也可直接调用。
func RegisterMonitorManager(m *zmonitor.Manager) {
	monitorExportMgr.Store(m)
}

// WriteMonitorSnapshotPrometheus 将 Manager 内各组件 MonitorData.Metrics 中的数值导出为单一 gauge 族（field 标签区分字段）。
func WriteMonitorSnapshotPrometheus(b *strings.Builder) {
	m := monitorExportMgr.Load()
	if m == nil {
		return
	}
	all := m.GetAll()
	if len(all) == 0 {
		return
	}

	type row struct {
		kind, id, name, field string
		val                   float64
	}
	var rows []row
	kindCount := make(map[string]int)
	for _, d := range all {
		kind := d.Type
		if kind == "" {
			kind = "unknown"
		}
		kindCount[kind]++
		id := d.ID
		if id == "" {
			id = "(no-id)"
		}
		name := d.Name
		if name == "" {
			name = "(no-name)"
		}
		for k, v := range d.Metrics {
			if !monitorFieldKeyOK(k) {
				continue
			}
			fv, ok := monitorScalarToFloat(v)
			if !ok {
				continue
			}
			rows = append(rows, row{
				kind:  kind,
				id:    id,
				name:  name,
				field: k,
				val:   fv,
			})
		}
	}

	if len(rows) == 0 && len(kindCount) == 0 {
		return
	}

	b.WriteString("# HELP zhenyi_monitor_snapshot zmonitor.Manager snapshot gauges (numeric Metrics fields only)\n")
	b.WriteString("# TYPE zhenyi_monitor_snapshot gauge\n")

	// Export registered component count by kind for low-cost overview.
	// 注册组件数量（按 kind），低成本总览
	for k, n := range kindCount {
		b.WriteString("zhenyi_monitor_registry_components{kind=\"")
		b.WriteString(escapePromLabelValue(k))
		b.WriteString("\"} ")
		appendInt(b, int64(n))
		b.WriteByte('\n')
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].kind != rows[j].kind {
			return rows[i].kind < rows[j].kind
		}
		if rows[i].id != rows[j].id {
			return rows[i].id < rows[j].id
		}
		return rows[i].field < rows[j].field
	})

	for _, r := range rows {
		b.WriteString("zhenyi_monitor_snapshot{kind=\"")
		b.WriteString(escapePromLabelValue(r.kind))
		b.WriteString("\",id=\"")
		b.WriteString(escapePromLabelValue(r.id))
		b.WriteString("\",name=\"")
		b.WriteString(escapePromLabelValue(r.name))
		b.WriteString("\",field=\"")
		b.WriteString(escapePromLabelValue(r.field))
		b.WriteString("\"} ")
		appendFloat(b, r.val)
		b.WriteByte('\n')
	}
}

func monitorFieldKeyOK(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func monitorScalarToFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}

func escapePromLabelValue(s string) string {
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
