package ziface

import (
	"context"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// Handle defines a message handler signature without return value.
// Handle 定义无返回值的消息处理函数签名。
type Handle func(ctx context.Context, message *zmsg.Message)

// RpcHandle defines a message handler signature with reply return value.
// RpcHandle 定义带回复返回值的消息处理函数签名。
type RpcHandle func(ctx context.Context, message *zmsg.Message) IMessage
