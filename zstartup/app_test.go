package zstartup

import (
	"context"
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

func init() {
	// Initialize default logger for tests
	cfg := zlog.NewDefaultLoggerConfig()
	cfg.Logs = map[string]int{}
	cfg.IsConsole = false
	zlog.NewDefaultLoggerWithConfig(cfg)
}

type mockServerActor struct {
	ziface.IActor
}

func (m *mockServerActor) RunServer(ctx context.Context) error {
	return nil
}

func TestNewApp(t *testing.T) {
	ctx := context.Background()
	cfg := AppConfig{
		Process:  1,
		IsSingle: true,
	}
	app := NewApp(ctx, cfg)
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
	if app.Group == nil {
		t.Fatal("app.Group is nil")
	}
}

func TestRegisterActorFactory(t *testing.T) {
	app := NewApp(context.Background(), AppConfig{})

	err := app.RegisterActorFactory(1, func(a *App, c zmodel.ActorConfig) ziface.IServerActor {
		return &mockServerActor{zactor.NewActor(c)}
	})
	if err != nil {
		t.Fatalf("RegisterActorFactory failed: %v", err)
	}

	// Register same type again
	err = app.RegisterActorFactory(1, func(a *App, c zmodel.ActorConfig) ziface.IServerActor {
		return nil
	})
	if err == nil {
		t.Fatal("expected error when registering same type twice")
	}

	// Register nil factory
	err = app.RegisterActorFactory(2, nil)
	if err == nil {
		t.Fatal("expected error when registering nil factory")
	}
}

func TestApp_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := AppConfig{
		Process:  1,
		IsSingle: true,
		Actors: []zmodel.ActorConfig{
			{Id: 1, ActorType: 1, Name: "test"},
		},
	}
	app := NewApp(ctx, cfg)

	app.RegisterActorFactory(1, func(a *App, c zmodel.ActorConfig) ziface.IServerActor {
		return &mockServerActor{zactor.NewActor(c)}
	})

	// Trigger stop shortly after starting
	go func() {
		app.Grace.Stop()
	}()

	err := app.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestApp_InitActorsError(t *testing.T) {
	// 1. Missing factory
	app1 := NewApp(context.Background(), AppConfig{
		Actors: []zmodel.ActorConfig{{Id: 1, ActorType: 1}},
	})
	if err := app1.Run(); err == nil {
		t.Fatal("expected error for missing factory")
	}

	// 2. Factory returns nil
	app2 := NewApp(context.Background(), AppConfig{
		Actors: []zmodel.ActorConfig{{Id: 1, ActorType: 1}},
	})
	_ = app2.RegisterActorFactory(1, func(a *App, c zmodel.ActorConfig) ziface.IServerActor {
		return nil
	})
	if err := app2.Run(); err == nil {
		t.Fatal("expected error for nil from factory")
	}
}
