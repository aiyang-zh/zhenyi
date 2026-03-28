// Package zaoi provides grid-based AOI (Area of Interest) and Enter/Leave events.
// Package zaoi 提供九宫格 AOI（Area of Interest）与 Enter/Leave 事件。
//
// Core types: StaticAoi (static grid) and IEntity/IAoi/IVector interfaces.
// 核心类型：StaticAoi（静态网格）、IEntity/IAoi/IVector 接口；
// 可选：Zone、WorldManager 用于多区域场景。
//
// Concurrency: this package is not thread-safe.
// 并发：本包非并发安全。AddEntity/RemoveEntity/UpdateEntity 等需由调用方保证
// 单 goroutine 调用，或在外部加锁后使用。
package zaoi
