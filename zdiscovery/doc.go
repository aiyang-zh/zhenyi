// Package zdiscovery provides Etcd and Noop implementations of ziface.Discoverer.
// Package zdiscovery 提供 ziface.Discoverer 的 Etcd 与 Noop 实现，
// 用于 zhenyi Actor 框架的分布式服务发现。
//
// Usage examples:
// 使用示例：
//
//	// Etcd
//	cli, _ := clientv3.New(clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}})
//	d, err := zdiscovery.NewEtcdDiscovery(ctx, cli)
//	g.SetDiscoverer(d)
//
//	// 单机/测试（空实现）
//	d := zdiscovery.NewNoopDiscovery()
//	g.SetDiscoverer(d)
package zdiscovery
