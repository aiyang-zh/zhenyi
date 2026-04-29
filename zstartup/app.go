// Package zstartup provides lightweight Group-based bootstrapping.
// Package zstartup 提供基于 Group 的轻量启动。
// NewApp + Run is for full configuration/lifecycle control, while
// RunSimple(SimpleConfig) is a quick single-process entry with signal-based shutdown.
// NewApp + Run 用于完整配置与生命周期控制，RunSimple(SimpleConfig) 用于单进程、默认参数、信号退出的简易入口。
package zstartup

import (
	"context"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"sync"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zgrace"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"go.uber.org/zap"
)

type ActorFactory func(a *App, c zmodel.ActorConfig) ziface.IServerActor

// App is a lightweight starter that creates Group, mounts Actors, and runs lifecycle.
// App 是轻量启动器：负责创建 Group、挂载 Actors，并启动 Run。
// Use RunSimple for the single-process + default-params + signal-exit path.
// 若只需单进程 + 默认参数 + 信号退出，建议使用 RunSimple。
type App struct {
	Ctx            context.Context
	Grace          *zgrace.Grace
	Group          ziface.IGroup
	stopCancel     context.CancelFunc
	shutdownOnce   sync.Once
	actorFactories map[uint32]ActorFactory
	AppConfig
}

// AppConfig describes which actors to start in current process and how to create the underlying Group.
// AppConfig 描述“本进程要启动哪些 Actor”以及 Group 的创建参数。
type AppConfig struct {
	Process  uint
	IsSingle bool
	ConnType znet.ConnProtocol
	Actors   []zmodel.ActorConfig // Actors to start in this process / 本进程需要启动的所有 Actor

}

// NewApp creates an App with a new Group.
// NewApp 创建 App，并初始化一个新的 Group。
func NewApp(ctx context.Context, cfg AppConfig) *App {
	runCtx, cancel := context.WithCancel(ctx)
	g := zactor.NewGroup(cfg.Process, cfg.IsSingle)
	app := &App{
		Ctx:            runCtx,
		stopCancel:     cancel,
		Group:          g,
		AppConfig:      cfg,
		Grace:          zgrace.New(),
		actorFactories: make(map[uint32]ActorFactory),
	}
	app.Grace.SetContext(runCtx)
	// Enable signal forwarding early to avoid startup window races.
	// 进程启动早期即开始接收退出信号，避免在 Group.Run 前收到 SIGTERM 时被直接终止。
	app.Grace.EnableSignalNotify()
	return app
}

// Run initializes actors, runs Group, then waits for graceful shutdown completion.
// Run 初始化 Actor，启动 Group，然后等待优雅退出流程结束。
func (a *App) Run() error {
	err := a.initActors()
	if err != nil {
		return err
	}
	a.Grace.Register(func(shutdownCtx context.Context) { a.shutdown("app shutdown") })
	err = a.Group.Run(a.Ctx)
	if err != nil {
		// If startup fails, Grace.Wait() won't run, so proactively do best-effort cleanup here.
		a.shutdown("app startup failed")
		return err
	}
	a.Grace.Wait()
	return nil
}

func (a *App) shutdown(reason string) {
	a.shutdownOnce.Do(func() {
		if a.stopCancel != nil {
			a.stopCancel()
		}
		if err := a.Group.Close(a.Ctx); err != nil {
			zlog.Warn(reason+": group close returned error", zap.Error(err))
		}
	})
}

func (a *App) initActors() error {
	for _, c := range a.Actors {
		f, ok := a.actorFactories[c.ActorType]
		if !ok {
			return fmt.Errorf("InitActors: no ActorFactory registered for type=%d (id=%d name=%s)", c.ActorType, c.Id, c.Name)
		}
		server := f(a, c)
		if server == nil {
			return fmt.Errorf("InitActors: factory returned nil for type=%d (id=%d name=%s)", c.ActorType, c.Id, c.Name)
		}
		// 1. 先 AddActor，使 actor 获得 Group
		a.Group.AddActor(server)
		// 2. 再注册路由，此时 actor.GetMsgList() 已由 RegisterHandle 填充
		if msgList := server.GetMsgList(); len(msgList) > 0 {
			ids := make([]int32, 0, len(msgList))
			for id := range msgList {
				ids = append(ids, id)
			}
			a.Group.RegisterRoutes(server, ids)
		}
		server.GetLogger().Info("app init actor success", zap.Uint64("actorId", server.GetActorId()))
	}
	return nil
}

// RegisterActorFactory registers factory for an ActorType.
// RegisterActorFactory 为指定 ActorType 注册创建工厂。
// The factory MUST be registered before Run(), and MUST NOT return nil.
// 工厂必须在 Run() 之前注册，且返回值不得为 nil。
func (a *App) RegisterActorFactory(actorType uint32, f ActorFactory) error {
	if f == nil {
		return zerrs.New(zerrs.ErrTypeValidation, "ActorFactory cannot be nil")
	}
	if _, exists := a.actorFactories[actorType]; exists {
		return zerrs.Newf(zerrs.ErrTypeValidation, "ActorFactory already registered for type=%d", actorType)
	}
	a.actorFactories[actorType] = f
	return nil
}
