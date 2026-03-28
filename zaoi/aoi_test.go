package zaoi

import (
	"math"
	"testing"
)

// ============================================================
// Mock Entity
// ============================================================

type mockEntity struct {
	id       int64
	eType    EntityType
	pos      IVector
	viewDist float64
	node     *EntityNode
}

func newMock(id int64, x, y, viewDist float64) *mockEntity {
	return &mockEntity{id: id, eType: EntityTypeAll, pos: Vector2{X: x, Y: y}, viewDist: viewDist}
}

func newMockTyped(id int64, eType EntityType, x, y, viewDist float64) *mockEntity {
	return &mockEntity{id: id, eType: eType, pos: Vector2{X: x, Y: y}, viewDist: viewDist}
}

func (m *mockEntity) GetId() int64                { return m.id }
func (m *mockEntity) GetType() EntityType         { return m.eType }
func (m *mockEntity) GetPosition() IVector        { return m.pos }
func (m *mockEntity) SetPosition(v IVector)       { m.pos = v }
func (m *mockEntity) GetViewDistance() float64    { return m.viewDist }
func (m *mockEntity) GetAoiNode() *EntityNode     { return m.node }
func (m *mockEntity) SetAoiNode(node *EntityNode) { m.node = node }

func mustAdd(t *testing.T, aoi *StaticAoi, e IEntity) {
	t.Helper()
	if err := aoi.AddEntity(e); err != nil {
		t.Fatal(err)
	}
}
func mustUpdate(t *testing.T, aoi *StaticAoi, e IEntity, pos IVector) {
	t.Helper()
	if err := aoi.UpdateEntity(e, pos); err != nil {
		t.Fatal(err)
	}
}

// ============================================================
// Vector2
// ============================================================

func TestVector2_Getters(t *testing.T) {
	v := Vector2{X: 1.5, Y: 2.5}
	if v.GetX() != 1.5 {
		t.Errorf("GetX: got %f, want 1.5", v.GetX())
	}
	if v.GetY() != 2.5 {
		t.Errorf("GetY: got %f, want 2.5", v.GetY())
	}
}

// ============================================================
// Grid — 链表 Add/Remove
// ============================================================

func TestGrid_AddRemove(t *testing.T) {
	g := &Grid{id: 0}

	e1 := newMock(1, 0, 0, 10)
	e2 := newMock(2, 0, 0, 10)
	e3 := newMock(3, 0, 0, 10)

	g.Add(e1)
	g.Add(e2)
	g.Add(e3)

	if g.Count() != 3 {
		t.Fatalf("after 3 Adds: count=%d, want 3", g.Count())
	}

	// 移除中间
	g.Remove(e2)
	if g.Count() != 2 {
		t.Fatalf("after removing e2: count=%d, want 2", g.Count())
	}
	if e2.GetAoiNode() != nil {
		t.Error("e2 node should be nil after Remove")
	}

	// 移除头部
	g.Remove(e3)
	if g.Count() != 1 {
		t.Fatalf("after removing e3: count=%d, want 1", g.Count())
	}

	// 移除最后一个
	g.Remove(e1)
	if g.Count() != 0 {
		t.Fatalf("after removing e1: count=%d, want 0", g.Count())
	}
	if g.GetHead() != nil {
		t.Error("head should be nil in empty grid")
	}
}

func TestGrid_RemoveNotInGrid(t *testing.T) {
	g := &Grid{id: 0}
	e := newMock(1, 0, 0, 10)
	// 没有Add，直接Remove不应panic
	g.Remove(e)
	if g.Count() != 0 {
		t.Error("count should be 0")
	}
}

func TestGrid_RemoveFromDifferentGrid(t *testing.T) {
	g1 := &Grid{id: 0}
	g2 := &Grid{id: 1}

	e := newMock(1, 0, 0, 10)
	g1.Add(e)

	// 尝试从 g2 移除 e（GridID 不匹配，不应移除）
	g2.Remove(e)
	if g1.Count() != 1 {
		t.Error("entity should not be removed from g1")
	}
}

// ============================================================
// StaticAoi — 创建与边界
// ============================================================

func TestStaticAoi_New(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)
	stats := aoi.GetGridStats()
	totalGrids := stats["total_grids"].(int)
	if totalGrids != 100 { // 10x10
		t.Errorf("expected 100 grids, got %d", totalGrids)
	}
}

func TestStaticAoi_NewPanic_InvalidGridSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for gridSize=0")
		}
	}()
	NewStaticAoi([4]float64{0, 100, 0, 100}, 0)
}

func TestStaticAoi_NewPanic_InvalidBounds(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for min >= max")
		}
	}()
	NewStaticAoi([4]float64{100, 0, 0, 100}, 10) // minX > maxX
}

// ============================================================
// StaticAoi — AddEntity / RemoveEntity
// ============================================================

func TestStaticAoi_AddRemoveEntity(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 50, 50, 20)
	if err := aoi.AddEntity(e); err != nil {
		t.Fatal(err)
	}

	if e.GetAoiNode() == nil {
		t.Fatal("entity should have AoiNode after Add")
	}

	stats := aoi.GetGridStats()
	if stats["total_entities"].(int) != 1 {
		t.Error("expected 1 entity")
	}

	aoi.RemoveEntity(e)
	if e.GetAoiNode() != nil {
		t.Error("entity node should be nil after Remove")
	}

	stats = aoi.GetGridStats()
	if stats["total_entities"].(int) != 0 {
		t.Error("expected 0 entities after Remove")
	}
}

func TestStaticAoi_AddEntity_OutOfBounds(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, -1, 50, 10) // 越界
	err := aoi.AddEntity(e)
	if err != ErrOutOfBounds {
		t.Errorf("AddEntity out of bounds: got err %v, want ErrOutOfBounds", err)
	}
	if e.GetAoiNode() != nil {
		t.Error("out-of-bounds entity should not be added")
	}
}

func TestStaticAoi_RemoveEntity_NoNode(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)
	e := newMock(1, 50, 50, 10)
	// 没有 Add，直接 Remove
	aoi.RemoveEntity(e) // 不应 panic
}

// ============================================================
// StaticAoi — UpdateEntity
// ============================================================

func TestStaticAoi_UpdateEntity_SameGrid(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 15, 15, 20)
	if err := aoi.AddEntity(e); err != nil {
		t.Fatal(err)
	}
	oldNode := e.GetAoiNode()
	oldGridID := oldNode.GridID

	// 小幅移动，留在同一格
	if err := aoi.UpdateEntity(e, Vector2{X: 16, Y: 16}); err != nil {
		t.Fatal(err)
	}
	if e.GetAoiNode().GridID != oldGridID {
		t.Error("entity should remain in same grid")
	}
}

func TestStaticAoi_UpdateEntity_DifferentGrid(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 5, 5, 20)
	if err := aoi.AddEntity(e); err != nil {
		t.Fatal(err)
	}
	oldGridID := e.GetAoiNode().GridID

	// 移到另一个格子
	if err := aoi.UpdateEntity(e, Vector2{X: 55, Y: 55}); err != nil {
		t.Fatal(err)
	}
	newGridID := e.GetAoiNode().GridID
	if newGridID == oldGridID {
		t.Error("entity should be in a different grid")
	}
}

func TestStaticAoi_UpdateEntity_OutOfBounds(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 50, 50, 10)
	if err := aoi.AddEntity(e); err != nil {
		t.Fatal(err)
	}

	// 移到界外：应返回错误且不修改状态（实体仍在原格）
	err := aoi.UpdateEntity(e, Vector2{X: 200, Y: 200})
	if err != ErrOutOfBounds {
		t.Errorf("UpdateEntity out of bounds: got err %v, want ErrOutOfBounds", err)
	}
	if e.GetAoiNode() == nil {
		t.Error("entity should remain in grid when UpdateEntity returns ErrOutOfBounds")
	}
}

// ============================================================
// StaticAoi — GetNearbyEntities (九宫格)
// ============================================================

func TestStaticAoi_GetNearbyEntities(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	center := newMock(1, 50, 50, 25) // 视距 25
	near := newMock(2, 52, 52, 10)   // 距离 ~2.83
	far := newMock(3, 90, 90, 10)    // 距离 ~56.6

	mustAdd(t, aoi, center)
	mustAdd(t, aoi, near)
	mustAdd(t, aoi, far)

	var result []IEntity
	aoi.GetNearbyEntities(center, &result)

	if len(result) != 1 {
		t.Fatalf("expected 1 nearby entity, got %d", len(result))
	}
	if result[0].GetId() != 2 {
		t.Errorf("expected entity 2, got %d", result[0].GetId())
	}
}

func TestStaticAoi_GetNearbyEntities_ExcludesSelf(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 50, 50, 100) // 大视距
	mustAdd(t, aoi, e)

	var result []IEntity
	aoi.GetNearbyEntities(e, &result)

	if len(result) != 0 {
		t.Errorf("should not include self, got %d", len(result))
	}
}

func TestStaticAoi_GetNearbyEntities_NoNode(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e := newMock(1, 50, 50, 10)
	// 不 Add，直接查
	var result []IEntity
	aoi.GetNearbyEntities(e, &result)
	if len(result) != 0 {
		t.Error("expected empty result for entity without node")
	}
}

func TestStaticAoi_GetNearbyEntities_CornerGrid(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	// 放在角落 (0,0) 网格
	corner := newMock(1, 1, 1, 50)
	nearby := newMock(2, 5, 5, 10)
	mustAdd(t, aoi, corner)
	mustAdd(t, aoi, nearby)

	var result []IEntity
	aoi.GetNearbyEntities(corner, &result)
	if len(result) != 1 {
		t.Fatalf("corner: expected 1 nearby, got %d", len(result))
	}
}

func TestStaticAoi_GetNearbyEntities_BufferReuse(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	center := newMock(1, 50, 50, 100)
	e2 := newMock(2, 51, 51, 10)
	mustAdd(t, aoi, center)
	mustAdd(t, aoi, e2)

	result := make([]IEntity, 0, 128)
	aoi.GetNearbyEntities(center, &result)
	if len(result) != 1 {
		t.Fatalf("first query: expected 1, got %d", len(result))
	}

	// 第二次查询复用同一 buffer
	aoi.GetNearbyEntities(center, &result)
	if len(result) != 1 {
		t.Fatalf("second query should reset buffer: expected 1, got %d", len(result))
	}
}

// ============================================================
// GetGridByPos
// ============================================================

func TestStaticAoi_GetGridByPos(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	g := aoi.GetGridByPos(Vector2{X: 0, Y: 0})
	if g == nil || g.GetId() != 0 {
		t.Errorf("(0,0) should be grid 0, got %v", g)
	}

	g = aoi.GetGridByPos(Vector2{X: 99.9, Y: 99.9})
	if g == nil {
		t.Fatal("(99.9, 99.9) should be in grid")
	}

	// 边界外
	g = aoi.GetGridByPos(Vector2{X: -1, Y: 0})
	if g != nil {
		t.Error("(-1, 0) should return nil")
	}

	g = aoi.GetGridByPos(Vector2{X: 100, Y: 0})
	if g != nil {
		t.Error("(100, 0) should return nil (x >= maxX)")
	}
}

// ============================================================
// GetGridBounds
// ============================================================

func TestStaticAoi_GetGridBounds(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	// Grid 0 => (0, 10, 0, 10)
	bounds := aoi.GetGridBounds(0)
	if bounds[0] != 0 || bounds[1] != 10 || bounds[2] != 0 || bounds[3] != 10 {
		t.Errorf("grid 0 bounds: %v", bounds)
	}

	// Grid 11 = row 1 col 1 => (10, 20, 10, 20)
	bounds = aoi.GetGridBounds(11) // y=1, x=1 in 10-wide grid
	if bounds[0] != 10 || bounds[1] != 20 || bounds[2] != 10 || bounds[3] != 20 {
		t.Errorf("grid 11 bounds: %v", bounds)
	}
}

// ============================================================
// Zone & WorldManager
// ============================================================

func TestWorldManager_AddZone(t *testing.T) {
	wm := NewWorldManager()
	aoiMgr := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	zone := &Zone{Id: 1, Name: "main", Type: ZoneTypeStatic, IAoi: aoiMgr}
	wm.AddZone(zone)

	if _, ok := wm.Zones[1]; !ok {
		t.Error("zone 1 should be added")
	}
}

func TestWorldManager_AddZone_Dynamic_Stored(t *testing.T) {
	wm := NewWorldManager()
	zone := &Zone{Id: 2, Name: "dynamic", Type: ZoneTypeDynamic}
	wm.AddZone(zone)

	if _, ok := wm.DynamicZones[2]; !ok {
		t.Error("dynamic zones should be stored in DynamicZones")
	}
}

// ============================================================
// GetGridStats
// ============================================================

func TestStaticAoi_GetGridStats(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	for i := 0; i < 10; i++ {
		e := newMock(int64(i), 50+float64(i)*0.1, 50+float64(i)*0.1, 10)
		mustAdd(t, aoi, e)
	}

	stats := aoi.GetGridStats()
	total := stats["total_entities"].(int)
	if total != 10 {
		t.Errorf("total_entities: got %d, want 10", total)
	}
	maxPer := stats["max_entities_per_grid"].(int)
	if maxPer != 10 {
		t.Errorf("all in same grid, max should be 10, got %d", maxPer)
	}
}

// ============================================================
// 大量实体测试
// ============================================================

func TestStaticAoi_MassEntities(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50) // 20x20=400 grids

	entities := make([]*mockEntity, 1000)
	for i := range entities {
		x := float64(i%30)*30 + 1
		y := float64(i/30)*30 + 1
		if x >= 1000 {
			x = 999
		}
		if y >= 1000 {
			y = 999
		}
		entities[i] = newMock(int64(i), x, y, 100)
		_ = aoi.AddEntity(entities[i])
	}

	stats := aoi.GetGridStats()
	if stats["total_entities"].(int) != 1000 {
		t.Fatalf("expected 1000 entities, got %d", stats["total_entities"].(int))
	}

	// 查询附近
	var result []IEntity
	aoi.GetNearbyEntities(entities[0], &result)
	// 至少应该有一些附近实体
	if len(result) == 0 {
		t.Error("expected some nearby entities")
	}

	// 全部移除
	for _, e := range entities {
		aoi.RemoveEntity(e)
	}
	stats = aoi.GetGridStats()
	if stats["total_entities"].(int) != 0 {
		t.Error("all entities should be removed")
	}
}

// ============================================================
// Benchmarks
// ============================================================

func setupBenchAoi(entityCount int) (*StaticAoi, []*mockEntity) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	entities := make([]*mockEntity, entityCount)
	for i := range entities {
		x := float64(i%100)*10 + 0.5
		y := float64(i/100)*10 + 0.5
		if x >= 1000 {
			x = 999
		}
		if y >= 1000 {
			y = 999
		}
		entities[i] = newMock(int64(i), x, y, 80)
		_ = aoi.AddEntity(entities[i])
	}
	return aoi, entities
}

func BenchmarkStaticAoi_AddEntity(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	entities := make([]*mockEntity, b.N)
	for i := range entities {
		entities[i] = newMock(int64(i), 500, 500, 10)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = aoi.AddEntity(entities[i])
	}
}

func BenchmarkStaticAoi_RemoveEntity(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	entities := make([]*mockEntity, b.N)
	for i := range entities {
		entities[i] = newMock(int64(i), 500, 500, 10)
		_ = aoi.AddEntity(entities[i])
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.RemoveEntity(entities[i])
	}
}

func BenchmarkStaticAoi_UpdateEntity_SameGrid(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	e := newMock(1, 500, 500, 10)
	_ = aoi.AddEntity(e)
	pos := &Vector2{X: 500, Y: 500}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pos.X = 500 + float64(i%5)*0.1
		_ = aoi.UpdateEntity(e, pos)
	}
}

func BenchmarkStaticAoi_UpdateEntity_CrossGrid(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	e := newMock(1, 500, 500, 10)
	_ = aoi.AddEntity(e)
	pos := &Vector2{X: 500, Y: 500}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 在两个格子之间来回
		if i%2 == 0 {
			pos.X, pos.Y = 500, 500
		} else {
			pos.X, pos.Y = 555, 555
		}
		_ = aoi.UpdateEntity(e, pos)
	}
}

func BenchmarkStaticAoi_GetNearbyEntities_100(b *testing.B) {
	aoi, entities := setupBenchAoi(100)
	var result []IEntity

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.GetNearbyEntities(entities[0], &result)
	}
}

func BenchmarkStaticAoi_GetNearbyEntities_1000(b *testing.B) {
	aoi, entities := setupBenchAoi(1000)
	var result []IEntity

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.GetNearbyEntities(entities[0], &result)
	}
}

func BenchmarkStaticAoi_GetNearbyEntities_5000(b *testing.B) {
	aoi, entities := setupBenchAoi(5000)
	var result []IEntity

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.GetNearbyEntities(entities[0], &result)
	}
}

func BenchmarkStaticAoi_GetGridByPos(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	pos := &Vector2{X: 500, Y: 500}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.GetGridByPos(pos)
	}
}

// Benchmark: AddEntity + RemoveEntity 循环（衡量池化节点复用率）
func BenchmarkStaticAoi_AddRemoveCycle(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	e := newMock(1, 500, 500, 10)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = aoi.AddEntity(e)
		aoi.RemoveEntity(e)
	}
}

// ============================================================
// Enter/Leave 事件
// ============================================================

func TestStaticAoi_EnterLeave_OnAdd(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	var enters []int64
	aoi.SetCallbacks(
		func(self, other IEntity) { enters = append(enters, self.GetId()*100+other.GetId()) },
		nil,
	)

	e1 := newMock(1, 50, 50, 25)
	mustAdd(t, aoi, e1)
	if len(enters) != 0 {
		t.Error("no Enter when adding to empty AOI")
	}

	e2 := newMock(2, 52, 52, 25)
	mustAdd(t, aoi, e2)

	// e2 进入 → 双向通知: (e2,e1) 和 (e1,e2)
	if len(enters) != 2 {
		t.Fatalf("expected 2 Enter events, got %d: %v", len(enters), enters)
	}
}

func TestStaticAoi_EnterLeave_OnRemove(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	var leaves []int64
	aoi.SetCallbacks(
		nil,
		func(self, other IEntity) { leaves = append(leaves, self.GetId()*100+other.GetId()) },
	)

	e1 := newMock(1, 50, 50, 25)
	e2 := newMock(2, 52, 52, 25)
	mustAdd(t, aoi, e1)
	mustAdd(t, aoi, e2)

	aoi.RemoveEntity(e2)

	// e2 离开 → 双向通知: (e2,e1) 和 (e1,e2)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 Leave events, got %d: %v", len(leaves), leaves)
	}
}

func TestStaticAoi_EnterLeave_CrossGridMove(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	var enters, leaves []int64
	aoi.SetCallbacks(
		func(self, other IEntity) { enters = append(enters, self.GetId()*100+other.GetId()) },
		func(self, other IEntity) { leaves = append(leaves, self.GetId()*100+other.GetId()) },
	)

	// e1 在 (5,5) → 格子(0,0), e2 在 (15,5) → 格子(1,0)
	// e3 在 (85,5) → 格子(8,0) — 远离 e1
	e1 := newMock(1, 5, 5, 25)
	e2 := newMock(2, 15, 5, 25)
	e3 := newMock(3, 85, 5, 25)
	mustAdd(t, aoi, e1)
	mustAdd(t, aoi, e2)
	mustAdd(t, aoi, e3)

	enters = enters[:0]
	leaves = leaves[:0]

	// e1 从 (5,5) 移到 (75,5) → 离开 e2 的九宫格，进入 e3 的九宫格
	mustUpdate(t, aoi, e1, Vector2{X: 75, Y: 5})

	if len(leaves) == 0 {
		t.Error("expected Leave events when moving away from e2")
	}
	if len(enters) == 0 {
		t.Error("expected Enter events when moving near e3")
	}
}

func TestStaticAoi_EnterLeave_SameGridMove_NoEvents(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	eventCount := 0
	aoi.SetCallbacks(
		func(self, other IEntity) { eventCount++ },
		func(self, other IEntity) { eventCount++ },
	)

	e1 := newMock(1, 50, 50, 25)
	e2 := newMock(2, 52, 52, 25)
	mustAdd(t, aoi, e1)
	mustAdd(t, aoi, e2)
	eventCount = 0

	// 同格内小幅移动，不触发事件
	mustUpdate(t, aoi, e1, Vector2{X: 51, Y: 51})
	if eventCount != 0 {
		t.Errorf("same-grid move should trigger 0 events, got %d", eventCount)
	}
}

func TestStaticAoi_NoCallbacks_NoEvents(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	e1 := newMock(1, 50, 50, 25)
	e2 := newMock(2, 52, 52, 25)
	mustAdd(t, aoi, e1)
	mustAdd(t, aoi, e2)
	_ = aoi.UpdateEntity(e1, Vector2{X: 80, Y: 80})
	aoi.RemoveEntity(e2)
	// 不设回调，不 panic
}

// ============================================================
// 类型过滤
// ============================================================

const (
	TypePlayer EntityType = 1
	TypeNPC    EntityType = 2
	TypeItem   EntityType = 3
)

func TestStaticAoi_GetNearbyEntitiesByType(t *testing.T) {
	aoi := NewStaticAoi([4]float64{0, 100, 0, 100}, 10)

	player := newMockTyped(1, TypePlayer, 50, 50, 25)
	npc := newMockTyped(2, TypeNPC, 52, 52, 25)
	item := newMockTyped(3, TypeItem, 51, 51, 25)

	mustAdd(t, aoi, player)
	mustAdd(t, aoi, npc)
	mustAdd(t, aoi, item)

	var result []IEntity

	// 查所有类型
	aoi.GetNearbyEntitiesByType(player, EntityTypeAll, &result)
	if len(result) != 2 {
		t.Fatalf("TypeAll: expected 2, got %d", len(result))
	}

	// 只查 NPC
	aoi.GetNearbyEntitiesByType(player, TypeNPC, &result)
	if len(result) != 1 || result[0].GetId() != 2 {
		t.Fatalf("TypeNPC: expected [npc], got %v", result)
	}

	// 只查 Item
	aoi.GetNearbyEntitiesByType(player, TypeItem, &result)
	if len(result) != 1 || result[0].GetId() != 3 {
		t.Fatalf("TypeItem: expected [item], got %v", result)
	}

	// 查不存在的类型
	aoi.GetNearbyEntitiesByType(player, EntityType(99), &result)
	if len(result) != 0 {
		t.Fatalf("Type99: expected 0, got %d", len(result))
	}
}

// ============================================================
// Enter/Leave + 类型过滤 Benchmarks
// ============================================================

func BenchmarkStaticAoi_EnterLeave_AddRemove(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	aoi.SetCallbacks(func(s, o IEntity) {}, func(s, o IEntity) {})

	// 预置 100 个实体
	for i := 0; i < 100; i++ {
		e := newMock(int64(i), 500+float64(i%10), 500+float64(i/10), 80)
		_ = aoi.AddEntity(e)
	}

	e := newMock(9999, 505, 505, 80)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = aoi.AddEntity(e)
		aoi.RemoveEntity(e)
	}
}

func BenchmarkStaticAoi_EnterLeave_CrossGrid(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	aoi.SetCallbacks(func(s, o IEntity) {}, func(s, o IEntity) {})

	for i := 0; i < 100; i++ {
		e := newMock(int64(i), 500+float64(i%10), 500+float64(i/10), 80)
		_ = aoi.AddEntity(e)
	}

	e := newMock(9999, 500, 500, 80)
	_ = aoi.AddEntity(e)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			_ = aoi.UpdateEntity(e, Vector2{X: 500, Y: 500})
		} else {
			_ = aoi.UpdateEntity(e, Vector2{X: 555, Y: 555})
		}
	}
}

func BenchmarkStaticAoi_GetNearbyByType(b *testing.B) {
	aoi := NewStaticAoi([4]float64{0, 1000, 0, 1000}, 50)
	entities := make([]*mockEntity, 1000)
	for i := range entities {
		eType := EntityType(i%3 + 1) // 1=Player, 2=NPC, 3=Item
		x := float64(i%100)*10 + 0.5
		y := float64(i/100)*10 + 0.5
		if x >= 1000 {
			x = 999
		}
		if y >= 1000 {
			y = 999
		}
		entities[i] = newMockTyped(int64(i), eType, x, y, 80)
		_ = aoi.AddEntity(entities[i])
	}

	var result []IEntity
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aoi.GetNearbyEntitiesByType(entities[0], TypePlayer, &result)
	}
}

// 确保未使用的导入不报错
var _ = math.MaxFloat64
