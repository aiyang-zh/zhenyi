package zscript

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultEngineConfig(t *testing.T) {
	cfg := DefaultEngineConfig()
	if cfg == nil {
		t.Fatalf("expected config")
	}
	if cfg.Timeout <= 0 || cfg.VMPoolSize <= 0 {
		t.Fatalf("unexpected defaults: %+v", *cfg)
	}
}

func TestStatsCollectorLifecycle(t *testing.T) {
	c := NewStatsCollector("lua")
	c.RecordCall(10*time.Millisecond, nil)
	c.RecordCall(5*time.Millisecond, errors.New("x"))
	c.RecordTimeout()
	c.RecordReload()

	s := c.GetStats()
	if s.EngineType != "lua" {
		t.Fatalf("engine type: %q", s.EngineType)
	}
	if s.CallCount != 2 || s.ErrorCount != 1 || s.TimeoutCount != 1 || s.ReloadCount != 1 {
		t.Fatalf("unexpected counts: %+v", *s)
	}
	if s.MaxDuration <= 0 || s.MinDuration <= 0 || s.AvgDuration <= 0 || s.TotalDuration <= 0 {
		t.Fatalf("unexpected durations: %+v", *s)
	}
	if s.LastReloadTime.IsZero() {
		t.Fatalf("expected last reload time")
	}

	c.Reset()
	s2 := c.GetStats()
	if s2.CallCount != 0 || s2.ErrorCount != 0 || s2.TimeoutCount != 0 || s2.ReloadCount != 0 {
		t.Fatalf("expected reset stats: %+v", *s2)
	}
	if s2.MinDuration != 0 || s2.MaxDuration != 0 || s2.TotalDuration != 0 || s2.AvgDuration != 0 {
		t.Fatalf("expected reset durations: %+v", *s2)
	}
}
