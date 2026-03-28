package zgate

import (
	"testing"
)

// ============================================================
// SessionManager 单元测试
// ============================================================

func TestSessionManager_New(t *testing.T) {
	m := NewSessionManager()
	if m.info == nil {
		t.Error("info map should be initialized")
	}
}

func TestSessionManager_SetAndGet(t *testing.T) {
	m := NewSessionManager()
	err := m.SetSessionActorId(1001, map[int32]int32{1: 100, 2: 200})
	if err != nil {
		t.Fatalf("SetSessionActorId error: %v", err)
	}

	if m.GetSessionActorId(1001, 1) != 100 {
		t.Errorf("expected 100, got %d", m.GetSessionActorId(1001, 1))
	}
	if m.GetSessionActorId(1001, 2) != 200 {
		t.Errorf("expected 200, got %d", m.GetSessionActorId(1001, 2))
	}
}

func TestSessionManager_GetNotExist(t *testing.T) {
	m := NewSessionManager()

	if m.GetSessionActorId(9999, 1) != 0 {
		t.Error("non-existent authId should return 0")
	}

	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100})
	if m.GetSessionActorId(1001, 999) != 0 {
		t.Error("non-existent actorType should return 0")
	}
}

func TestSessionManager_SetNilInfo(t *testing.T) {
	m := NewSessionManager()
	err := m.SetSessionActorId(1001, nil)
	if err == nil {
		t.Error("expected error for nil info")
	}
}

func TestSessionManager_SetEmptyInfo(t *testing.T) {
	m := NewSessionManager()
	err := m.SetSessionActorId(1001, map[int32]int32{})
	if err == nil {
		t.Error("expected error for empty info")
	}
}

func TestSessionManager_SetOverwrite(t *testing.T) {
	m := NewSessionManager()
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100})
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 200})

	if m.GetSessionActorId(1001, 1) != 200 {
		t.Errorf("expected 200 (overwritten), got %d", m.GetSessionActorId(1001, 1))
	}
}

func TestSessionManager_SetMerge(t *testing.T) {
	m := NewSessionManager()
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100})
	_ = m.SetSessionActorId(1001, map[int32]int32{2: 200})

	if m.GetSessionActorId(1001, 1) != 100 {
		t.Error("existing actorType should be preserved")
	}
	if m.GetSessionActorId(1001, 2) != 200 {
		t.Error("new actorType should be added")
	}
}

func TestSessionManager_Del(t *testing.T) {
	m := NewSessionManager()
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100})
	m.DelSessionActorId(1001)

	if m.GetSessionActorId(1001, 1) != 0 {
		t.Error("deleted session should return 0")
	}
}

func TestSessionManager_DelNotExist(t *testing.T) {
	m := NewSessionManager()
	m.DelSessionActorId(9999)
}

func TestSessionManager_GetAllActor(t *testing.T) {
	m := NewSessionManager()
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100, 2: 200, 3: 300})

	all := m.GetAllActor(1001)
	if len(all) != 3 {
		t.Errorf("expected 3 actors, got %d", len(all))
	}
	if all[1] != 100 || all[2] != 200 || all[3] != 300 {
		t.Errorf("unexpected actor ids: %v", all)
	}
}

func TestSessionManager_GetAllActorNotExist(t *testing.T) {
	m := NewSessionManager()
	all := m.GetAllActor(9999)
	if all != nil {
		t.Error("non-existent authId should return nil")
	}
}

func TestSessionManager_MultipleUsers(t *testing.T) {
	m := NewSessionManager()
	_ = m.SetSessionActorId(1001, map[int32]int32{1: 100})
	_ = m.SetSessionActorId(1002, map[int32]int32{1: 200})

	if m.GetSessionActorId(1001, 1) != 100 {
		t.Error("authId 1001 should have actorId 100")
	}
	if m.GetSessionActorId(1002, 1) != 200 {
		t.Error("authId 1002 should have actorId 200")
	}

	m.DelSessionActorId(1001)
	if m.GetSessionActorId(1002, 1) != 200 {
		t.Error("authId 1002 should not be affected by deleting 1001")
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkSessionManager_Get(b *testing.B) {
	m := NewSessionManager()
	for i := int64(0); i < 1000; i++ {
		_ = m.SetSessionActorId(i, map[int32]int32{1: int32(i)})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.GetSessionActorId(int64(i%1000), 1)
	}
}

func BenchmarkSessionManager_Set(b *testing.B) {
	m := NewSessionManager()
	info := map[int32]int32{1: 100, 2: 200}
	// 预先创建 authId 的内层 map，避免把“首次创建用户映射”的分配算进热路径
	_ = m.SetSessionActorId(1, info)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.SetSessionActorId(1, info)
	}
}
