# zaoi

**空间邻近（AOI）**：九宫格 AOI、Enter/Leave 事件。

## 核心类型

- `StaticAoi`：静态网格实现
- `IEntity`、`IAoi`、`IVector`：接口
- `Zone`、`WorldManager`：多区域场景

## 并发

本包**非并发安全**。AddEntity/RemoveEntity/UpdateEntity 等需由调用方保证单 goroutine 或外部加锁。

## 示例

见 `examples/zaoi_demo`。
