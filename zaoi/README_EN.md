# zaoi

**Spatial Proximity (AOI)**: 9-grid AOI, Enter/Leave events.

## Core Types

- `StaticAoi`: Static grid implementation
- `IEntity`, `IAoi`, `IVector`: Interfaces
- `Zone`, `WorldManager`: Multi-zone scenarios

## Concurrency

This package is **not concurrency-safe**. AddEntity/RemoveEntity/UpdateEntity need to be guaranteed single goroutine or externally locked by callers.

## Examples

See `examples/zaoi_demo`.
