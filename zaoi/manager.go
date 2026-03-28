package zaoi

// WorldManager manages multiple zones in a world.
// WorldManager 管理一个世界中的多个区域（Zone）。
type WorldManager struct {
	Zones map[int]*Zone

	// DynamicZones 保存 ZoneTypeDynamic 区域（当前不参与 AOI 计算，但不再无效丢弃）。
	DynamicZones map[int]*Zone
}

// NewWorldManager creates a world manager with empty zone maps.
// NewWorldManager 创建 WorldManager，并初始化内部 zone 映射表。
func NewWorldManager() *WorldManager {
	return &WorldManager{
		Zones:        make(map[int]*Zone),
		DynamicZones: make(map[int]*Zone),
	}
}

// AddZone adds a zone into world.
// AddZone 将区域加入世界。
// Currently ZoneTypeStatic is stored in Zones, and ZoneTypeDynamic is stored in DynamicZones.
// 当前 ZoneTypeStatic 写入 Zones；ZoneTypeDynamic 写入 DynamicZones。
func (w *WorldManager) AddZone(zone *Zone) {
	if w == nil || zone == nil {
		return
	}
	switch zone.Type {
	case ZoneTypeDynamic:
		w.DynamicZones[zone.Id] = zone
	case ZoneTypeStatic:
		w.Zones[zone.Id] = zone
	}
}
