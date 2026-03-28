package zaoi

import "errors"

// ErrOutOfBounds indicates the position is outside bounds configured by NewStaticAoi.
// ErrOutOfBounds 表示坐标超出 NewStaticAoi 的 bounds，AddEntity 或 UpdateEntity 未执行。
var ErrOutOfBounds = errors.New("zaoi: position out of bounds")

// EntityType represents entity category for filtering queries.
// EntityType 实体类型（用于查询时过滤）。
type EntityType int32

// EntityTypeAll matches all entity types.
// EntityTypeAll 匹配所有实体类型。
const EntityTypeAll EntityType = 0

// IEntity is an AOI entity interface.
// IEntity AOI 实体接口。
type IEntity interface {
	GetId() int64
	GetType() EntityType
	GetPosition() IVector
	SetPosition(IVector)
	GetViewDistance() float64

	GetAoiNode() *EntityNode
	SetAoiNode(node *EntityNode)
}

// EnterLeaveCallback is invoked on Enter/Leave events.
// EnterLeaveCallback Enter/Leave 事件回调。
// self: view owner, other: entity entering/leaving the view.
// self：视野拥有者，other：进入/离开视野的实体。
type EnterLeaveCallback func(self, other IEntity)

// IAoi is an AOI manager interface.
// IAoi AOI 管理器接口。
type IAoi interface {
	AddEntity(entity IEntity) error
	RemoveEntity(entity IEntity)
	UpdateEntity(entity IEntity, newPos IVector) error
	GetNearbyEntities(entity IEntity, resultBuf *[]IEntity)
	GetNearbyEntitiesByType(entity IEntity, entityType EntityType, resultBuf *[]IEntity)
	SetCallbacks(onEnter, onLeave EnterLeaveCallback)
	GetGridStats() map[string]interface{}
}

// IVector is a 2D vector interface.
// IVector 二维向量接口。
type IVector interface {
	GetX() float64
	GetY() float64
}
