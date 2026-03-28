package zstream

import (
	"context"

	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

var _ ziface.IServerActor = (*Server)(nil)

// Server is a lightweight wrapper around zactor.Actor that exposes an IServerActor-style lifecycle.
// Server 是对 zactor.Actor 的轻量包装，对外提供类似 IServerActor 的生命周期入口。
type Server struct {
	*zactor.Actor
}

// NewServer creates a new Server with the given ActorConfig and sets itself as IActor.
// NewServer 基于给定的 ActorConfig 创建 Server，并将自身设置为 IActor。
func NewServer(actorConfig zmodel.ActorConfig) *Server {
	s := &Server{
		Actor: zactor.NewActor(actorConfig),
	}
	s.SetIActor(s)
	return s
}

// RunServer runs the server initialization lifecycle (InitServer hook).
// RunServer 执行服务器初始化生命周期（InitServer 钩子）。
func (s *Server) RunServer(ctx context.Context) error {
	return s.CallInitServer(ctx)
}
