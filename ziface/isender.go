package ziface

import (
	"time"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// ISender manages RPC slots and reply matching.
// ISender 管理 RPC 槽位分配与回包匹配。
type ISender interface {
	// AddSender allocates a new RPC slot ID.
	// AddSender 申请一个新的 RPC 槽位 ID。
	AddSender() (uint64, error)
	// SetReply writes a response message into its RPC slot.
	// SetReply 将响应消息写入对应 RPC 槽位。
	SetReply(data *zmsg.Message)
	// GetReply waits for and returns the response of rpcId before timeout.
	// GetReply 在超时前等待并获取指定 rpcId 的响应。
	GetReply(rpcId uint64, timeout time.Duration) (*zmsg.Message, bool)
}
