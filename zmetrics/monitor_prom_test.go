package zmetrics

import (
	"strings"
	"testing"

	"github.com/aiyang-zh/zhenyi/zmonitor"
)

type promSnapTestMon struct{}

func (promSnapTestMon) GetMonitorData() zmonitor.MonitorData {
	return zmonitor.MonitorData{
		Type: "gate",
		ID:   "g1",
		Name: "main",
		Metrics: map[string]interface{}{
			"connections": int64(7),
		},
	}
}

func TestWriteMonitorSnapshotPrometheus(t *testing.T) {
	t.Cleanup(func() { RegisterMonitorManager(nil) })

	mgr := zmonitor.NewManager()
	mgr.Register("g1", promSnapTestMon{})
	RegisterMonitorManager(mgr)

	var b strings.Builder
	WriteMonitorSnapshotPrometheus(&b)
	out := b.String()
	if !strings.Contains(out, `zhenyi_monitor_registry_components{kind="gate"} 1`) {
		t.Fatalf("missing registry_components line:\n%s", out)
	}
	if !strings.Contains(out, `zhenyi_monitor_snapshot{kind="gate",id="g1",name="main",field="connections"} 7`) {
		t.Fatalf("missing snapshot gauge:\n%s", out)
	}
}
