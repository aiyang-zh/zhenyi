package zaoi

// ZoneType indicates zone kind.
// ZoneType 表示区域类型。
type ZoneType int

const (
	ZoneTypeStatic  ZoneType = iota // Static zone / 静态区域
	ZoneTypeDynamic                 // Dynamic zone / 动态区域
)

// Zone describes one world zone and its AOI implementation.
// Zone 描述一个世界区域及其 AOI 实现。
type Zone struct {
	Id     int
	Name   string
	Type   ZoneType
	Bounds [4]float64 // [minX, maxX, minY, maxY]
	IAoi   IAoi
}
