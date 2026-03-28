package zaoi

import "math"

// StaticAoi is the static 9-grid AOI manager.
// StaticAoi 静态九宫格 AOI 管理器。
type StaticAoi struct {
	minX     float64 // World left bound / 世界左边界
	maxX     float64 // World right bound / 世界右边界
	minY     float64 // World bottom bound / 世界下边界
	maxY     float64 // World top bound / 世界上边界
	grids    []*Grid // 1D grid array, index = y*gridX+x / 一维网格数组，index = y*gridX + x
	gridSize float64 // Per-grid size / 单个网格尺寸
	gridX    int     // Grid count on X-axis / X轴网格数量
	gridY    int     // Grid count on Y-axis / Y轴网格数量
	onEnter  EnterLeaveCallback
	onLeave  EnterLeaveCallback
}

// NewStaticAoi creates static AOI manager.
// NewStaticAoi 创建静态AOI管理器。
// It panics on invalid config to fail fast during bootstrap.
// 参数校验失败时 panic（配置错误属于不可恢复错误，应在启动阶段暴露）。
func NewStaticAoi(bounds [4]float64, gridSize float64) *StaticAoi {
	minX, maxX, minY, maxY := bounds[0], bounds[1], bounds[2], bounds[3]

	if gridSize <= 0 {
		panic("aoi: gridSize must be positive")
	}
	if minX >= maxX || minY >= maxY {
		panic("aoi: invalid bounds (min >= max)")
	}

	gridX := int(math.Ceil((maxX - minX) / gridSize))
	gridY := int(math.Ceil((maxY - minY) / gridSize))

	if gridX <= 0 || gridY <= 0 {
		panic("aoi: grid dimensions must be positive")
	}

	total := gridX * gridY
	if total > int(InvalidGridID) {
		panic("aoi: grid count exceeds uint32 max")
	}
	grids := make([]*Grid, total)
	for i := 0; i < total; i++ {
		grids[i] = &Grid{id: uint32(i)}
	}

	return &StaticAoi{
		minX:     minX,
		maxX:     maxX,
		minY:     minY,
		maxY:     maxY,
		grids:    grids,
		gridSize: gridSize,
		gridX:    gridX,
		gridY:    gridY,
	}
}

// gridByID returns grid by GridID (inline-friendly helper).
// gridByID 通过 GridID 获取格子（内联友好）。
func (m *StaticAoi) gridByID(id uint32) *Grid {
	if id == InvalidGridID || int(id) >= len(m.grids) {
		return nil
	}
	return m.grids[id]
}

// GetGridBounds computes grid bounds by GridID (on-demand, non-hot path).
// GetGridBounds 根据 GridID 计算格子边界（按需计算，非热路径）。
func (m *StaticAoi) GetGridBounds(id uint32) [4]float64 {
	x := int(id) % m.gridX
	y := int(id) / m.gridX
	gMinX := m.minX + float64(x)*m.gridSize
	gMinY := m.minY + float64(y)*m.gridSize
	return [4]float64{gMinX, gMinX + m.gridSize, gMinY, gMinY + m.gridSize}
}

// GetGridByPos returns grid for world position.
// GetGridByPos 根据世界坐标获取所在格子。
func (m *StaticAoi) GetGridByPos(pos IVector) *Grid {
	px, py := pos.GetX(), pos.GetY()
	if px < m.minX || px >= m.maxX || py < m.minY || py >= m.maxY {
		return nil
	}

	x := int((px - m.minX) / m.gridSize)
	y := int((py - m.minY) / m.gridSize)

	// Clamp boundaries to avoid floating-point precision issues.
	// 边界钳位（防止浮点精度问题）。
	if x >= m.gridX {
		x = m.gridX - 1
	}
	if y >= m.gridY {
		y = m.gridY - 1
	}

	return m.grids[y*m.gridX+x]
}

// SetCallbacks sets Enter/Leave callbacks.
// SetCallbacks 设置 Enter/Leave 事件回调。
func (m *StaticAoi) SetCallbacks(onEnter, onLeave EnterLeaveCallback) {
	m.onEnter = onEnter
	m.onLeave = onLeave
}

// AddEntity adds entity into AOI and triggers Enter callbacks with nearby entities in 9-grid.
// AddEntity 将实体添加到 AOI，并触发与九宫格内已有实体的 Enter 事件。
// If entity position is out of bounds, it returns ErrOutOfBounds and does not mutate state.
// 若 entity 的坐标超出 NewStaticAoi 的 bounds，则不加入并返回 ErrOutOfBounds。
func (m *StaticAoi) AddEntity(entity IEntity) error {
	grid := m.GetGridByPos(entity.GetPosition())
	if grid == nil {
		return ErrOutOfBounds
	}
	grid.Add(entity)

	if m.onEnter != nil {
		m.notifyNearby(entity, m.onEnter)
	}
	return nil
}

// RemoveEntity removes entity from AOI and triggers Leave callbacks with nearby entities in 9-grid.
// RemoveEntity 从 AOI 移除实体，并触发与九宫格内实体的 Leave 事件。
func (m *StaticAoi) RemoveEntity(entity IEntity) {
	node := entity.GetAoiNode()
	if node == nil {
		return
	}
	grid := m.gridByID(node.GridID)
	if grid == nil {
		return
	}

	if m.onLeave != nil {
		m.notifyNearby(entity, m.onLeave)
	}

	grid.Remove(entity)
}

// UpdateEntity updates entity position; it triggers Enter/Leave on cross-grid moves.
// UpdateEntity 更新实体位置，跨格移动时触发 Enter/Leave 事件。
// If newPos is out of bounds, it returns ErrOutOfBounds and does not mutate any state.
// 若 newPos 超出 bounds，则不修改任何状态并返回 ErrOutOfBounds，业务可校验坐标后重试。
func (m *StaticAoi) UpdateEntity(entity IEntity, newPos IVector) error {
	newGrid := m.GetGridByPos(newPos)
	if newGrid == nil {
		return ErrOutOfBounds
	}

	node := entity.GetAoiNode()
	var oldGrid *Grid
	if node != nil {
		oldGrid = m.gridByID(node.GridID)
	}

	if oldGrid != nil && oldGrid.id == newGrid.id {
		entity.SetPosition(newPos)
		return nil
	}

	hasCallbacks := m.onEnter != nil || m.onLeave != nil

	if hasCallbacks && oldGrid != nil {
		m.handleCrossGridMove(entity, oldGrid, newGrid, newPos)
		return nil
	}

	if oldGrid != nil {
		if m.onLeave != nil {
			m.notifyNearby(entity, m.onLeave)
		}
		oldGrid.Remove(entity)
	}
	entity.SetPosition(newPos)
	newGrid.Add(entity)
	if m.onEnter != nil {
		m.notifyNearby(entity, m.onEnter)
	}
	return nil
}

// handleCrossGridMove 跨格移动时精确计算 Enter/Leave
// Leave 使用旧位置判断距离（"之前看得见"），Enter 使用新位置（"现在看得见"）
func (m *StaticAoi) handleCrossGridMove(entity IEntity, oldGrid, newGrid *Grid, newPos IVector) {
	oldStartX, oldEndX, oldStartY, oldEndY := m.nineGridRange(oldGrid.id)
	newStartX, newEndX, newStartY, newEndY := m.nineGridRange(newGrid.id)

	selfId := entity.GetId()
	viewDistSq := entity.GetViewDistance() * entity.GetViewDistance()

	// Leave: 用旧位置算距离
	if m.onLeave != nil {
		oldX, oldY := entity.GetPosition().GetX(), entity.GetPosition().GetY()
		for y := oldStartY; y <= oldEndY; y++ {
			for x := oldStartX; x <= oldEndX; x++ {
				if x >= newStartX && x <= newEndX && y >= newStartY && y <= newEndY {
					continue
				}
				m.notifyGridEntities(entity, selfId, oldX, oldY, viewDistSq, y*m.gridX+x, m.onLeave)
			}
		}
	}

	// 执行格子切换 + 更新位置
	oldGrid.Remove(entity)
	entity.SetPosition(newPos)
	newGrid.Add(entity)

	// Enter: 用新位置算距离
	if m.onEnter != nil {
		newX, newY := newPos.GetX(), newPos.GetY()
		for y := newStartY; y <= newEndY; y++ {
			for x := newStartX; x <= newEndX; x++ {
				if x >= oldStartX && x <= oldEndX && y >= oldStartY && y <= oldEndY {
					continue
				}
				m.notifyGridEntities(entity, selfId, newX, newY, viewDistSq, y*m.gridX+x, m.onEnter)
			}
		}
	}
}

// nineGridRange 计算九宫格的 X/Y 范围（含边界钳位）
func (m *StaticAoi) nineGridRange(gridID uint32) (startX, endX, startY, endY int) {
	gid := int(gridID)
	yIdx := gid / m.gridX
	xIdx := gid % m.gridX

	startX = xIdx - 1
	if startX < 0 {
		startX = 0
	}
	endX = xIdx + 1
	if endX >= m.gridX {
		endX = m.gridX - 1
	}
	startY = yIdx - 1
	if startY < 0 {
		startY = 0
	}
	endY = yIdx + 1
	if endY >= m.gridY {
		endY = m.gridY - 1
	}
	return
}

// notifyNearby 通知九宫格内所有视距内实体（双向通知）
func (m *StaticAoi) notifyNearby(entity IEntity, callback EnterLeaveCallback) {
	node := entity.GetAoiNode()
	if node == nil {
		return
	}
	selfId := entity.GetId()
	selfX, selfY := entity.GetPosition().GetX(), entity.GetPosition().GetY()
	viewDistSq := entity.GetViewDistance() * entity.GetViewDistance()

	startX, endX, startY, endY := m.nineGridRange(node.GridID)
	for y := startY; y <= endY; y++ {
		base := y * m.gridX
		for x := startX; x <= endX; x++ {
			m.notifyGridEntities(entity, selfId, selfX, selfY, viewDistSq, base+x, callback)
		}
	}
}

// notifyGridEntities 通知单个格子内视距内的实体（双向：self→other 和 other→self）
func (m *StaticAoi) notifyGridEntities(entity IEntity, selfId int64, selfX, selfY, viewDistSq float64, gridIdx int, callback EnterLeaveCallback) {
	curr := m.grids[gridIdx].head
	for curr != nil {
		if curr.Entity.GetId() != selfId {
			ePos := curr.Entity.GetPosition()
			dx := ePos.GetX() - selfX
			if dxSq := dx * dx; dxSq <= viewDistSq {
				dy := ePos.GetY() - selfY
				if dxSq+dy*dy <= viewDistSq {
					callback(entity, curr.Entity)
					otherViewSq := curr.Entity.GetViewDistance() * curr.Entity.GetViewDistance()
					if dxSq+dy*dy <= otherViewSq {
						callback(curr.Entity, entity)
					}
				}
			}
		}
		curr = curr.Next
	}
}

// GetNearbyEntities collects nearby entities in 9-grid into resultBuf (caller-provided, zero-allocation).
// GetNearbyEntities 获取九宫格内的附近实体（零分配：调用方传入 resultBuf）。
func (m *StaticAoi) GetNearbyEntities(entity IEntity, resultBuf *[]IEntity) {
	m.GetNearbyEntitiesByType(entity, EntityTypeAll, resultBuf)
}

// GetNearbyEntitiesByType collects nearby entities of given type in 9-grid into resultBuf (zero-allocation).
// GetNearbyEntitiesByType 获取九宫格内指定类型的附近实体（零分配）。
// When entityType is EntityTypeAll(0), it is equivalent to GetNearbyEntities.
// entityType 为 EntityTypeAll(0) 时等同于 GetNearbyEntities。
func (m *StaticAoi) GetNearbyEntitiesByType(entity IEntity, entityType EntityType, resultBuf *[]IEntity) {
	*resultBuf = (*resultBuf)[:0]

	node := entity.GetAoiNode()
	if node == nil {
		return
	}

	selfId := entity.GetId()
	selfX, selfY := entity.GetPosition().GetX(), entity.GetPosition().GetY()
	viewDistSq := entity.GetViewDistance() * entity.GetViewDistance()

	startX, endX, startY, endY := m.nineGridRange(node.GridID)
	filterAll := entityType == EntityTypeAll

	for y := startY; y <= endY; y++ {
		base := y * m.gridX
		for x := startX; x <= endX; x++ {
			curr := m.grids[base+x].head
			for curr != nil {
				if curr.Entity.GetId() != selfId && (filterAll || curr.Entity.GetType() == entityType) {
					ePos := curr.Entity.GetPosition()
					dx := ePos.GetX() - selfX
					if dxSq := dx * dx; dxSq <= viewDistSq {
						dy := ePos.GetY() - selfY
						if dxSq+dy*dy <= viewDistSq {
							*resultBuf = append(*resultBuf, curr.Entity)
						}
					}
				}
				curr = curr.Next
			}
		}
	}
}

// GetGridStats returns grid statistics for debugging.
// GetGridStats 获取网格统计信息（调试用）。
// When there is no entity, min_entities_per_grid equals math.MaxInt32 (meaning no grid has entities).
// 当没有任何实体时，min_entities_per_grid 为 math.MaxInt32（表示无格子有实体）。
func (m *StaticAoi) GetGridStats() map[string]interface{} {
	totalEntities := 0
	minPerGrid := math.MaxInt32
	maxPerGrid := 0

	for _, grid := range m.grids {
		count := grid.Count()
		totalEntities += count
		if count < minPerGrid {
			minPerGrid = count
		}
		if count > maxPerGrid {
			maxPerGrid = count
		}
	}

	totalGrids := m.gridX * m.gridY
	return map[string]interface{}{
		"total_entities":        totalEntities,
		"total_grids":           totalGrids,
		"grid_size":             m.gridSize,
		"min_entities_per_grid": minPerGrid,
		"max_entities_per_grid": maxPerGrid,
		"avg_entities_per_grid": float64(totalEntities) / float64(totalGrids),
	}
}
