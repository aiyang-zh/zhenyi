package zgate

import (
	"errors"
)

// SessionManager manages authId -> actorType -> actorId mappings.
// SessionManager 管理 authId → actorType → actorId 的映射。
//
// Thread-safety contract: methods are called only in GateServer Actor Run goroutine.
// 线程安全约定：所有方法仅在 GateServer Actor 的 Run goroutine 中调用（单线程）。
// It is mailbox-driven and lock-free by design; do not call from AsyncRun/Worker goroutines.
// 通过 Actor mailbox 消息驱动，因此无需加锁。禁止从 AsyncRun/Worker goroutine 直接调用。
type SessionManager struct {
	info map[int64]map[int32]int32
}

// NewSessionManager creates a session manager with empty mappings.
// NewSessionManager 创建 SessionManager，并初始化内部映射表。
func NewSessionManager() *SessionManager {
	return &SessionManager{
		info: make(map[int64]map[int32]int32),
	}
}

// GetSessionActorId returns actorId by authId and actorType, or 0 if not found.
// GetSessionActorId 按 authId 与 actorType 查询 actorId；未找到返回 0。
func (m *SessionManager) GetSessionActorId(authId int64, actorType int32) int32 {
	info1, ok := m.info[authId]
	if !ok {
		return 0
	}
	if actorId, ok1 := info1[actorType]; ok1 {
		return actorId
	}
	return 0
}

// SetSessionActorId upserts mappings for authId.
// SetSessionActorId 为指定 authId 写入/更新 actorType->actorId 映射。
func (m *SessionManager) SetSessionActorId(authId int64, info map[int32]int32) error {
	if len(info) == 0 {
		return errors.New("not actor")
	}
	_, ok := m.info[authId]
	if !ok {
		m.info[authId] = make(map[int32]int32)
	}
	for k, v := range info {
		m.info[authId][k] = v
	}
	return nil
}

// DelSessionActorId deletes all mappings for authId.
// DelSessionActorId 删除指定 authId 的全部映射。
func (m *SessionManager) DelSessionActorId(authId int64) {
	delete(m.info, authId)
}

// GetAllActor returns actorType->actorId mappings for authId.
// GetAllActor 返回指定 authId 的 actorType->actorId 映射表。
func (m *SessionManager) GetAllActor(authId int64) map[int32]int32 {
	info, ok := m.info[authId]
	if !ok {
		return nil
	}
	return info
}
