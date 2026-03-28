package zstream

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"testing"
)

func init() {
	// Initialize logger.
	// 初始化 logger。
	zlog.NewDefaultLogger()
}

// TestServer_New tests server creation.
// TestServer_New 测试服务器创建。
func TestServer_New(t *testing.T) {
	actorConfig := zmodel.ActorConfig{
		Id:        4001,
		ActorType: 40,
		Index:     1,
	}

	server := NewServer(actorConfig)

	if server == nil {
		t.Fatal("server should not be nil")
	}

	if server.Actor == nil {
		t.Error("Actor should not be nil")
	}

	if server.GetActorId() != 4001 {
		t.Errorf("expected actorId 4001, got %d", server.GetActorId())
	}
}

// TestServer_RunServer tests server running.
// TestServer_RunServer 测试服务器运行。
func TestServer_RunServer(t *testing.T) {
	server := NewServer(zmodel.ActorConfig{
		Id:        4002,
		ActorType: 40,
		Index:     2,
	})

	// Set init function (required, otherwise it panics).
	// 设置初始化函数（必须设置，否则会 panic）。
	server.SetInitServer(func(ctx context.Context) error {
		return nil
	})

	ctx := context.Background()
	err := server.RunServer(ctx)

	if err != nil {
		t.Errorf("RunServer should not return error: %v", err)
	}
}

// TestServer_ActorIntegration tests Actor integration.
// TestServer_ActorIntegration 测试 Actor 集成。
func TestServer_ActorIntegration(t *testing.T) {
	server := NewServer(zmodel.ActorConfig{
		Id:        4003,
		ActorType: 40,
		Index:     3,
	})

	// Verify Actor attributes.
	// 验证 Actor 属性。
	if server.GetActorType() != 40 {
		t.Errorf("expected actorType 40, got %d", server.GetActorType())
	}

	if server.GetActorId() != 4003 {
		t.Errorf("expected actorId 4003, got %d", server.GetActorId())
	}
}

// BenchmarkServer_New benchmarks server creation.
// BenchmarkServer_New 测试服务器创建性能。
func BenchmarkServer_New(b *testing.B) {
	actorConfig := zmodel.ActorConfig{
		Id:        5001,
		ActorType: 50,
		Index:     1,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		server := NewServer(actorConfig)
		_ = server
	}
}

// TODO: Add more tests after stream-server functionality is completed.
// TODO: 需要完善 StreamServer 功能后添加更多测试。
// Suggested tests:
// 建议添加的测试：
// - TestServer_StreamProcessing - 流处理测试
// - TestServer_DataPipeline - 数据管道测试
// - TestServer_Backpressure - 背压处理测试
// - BenchmarkServer_Throughput - 吞吐量测试
