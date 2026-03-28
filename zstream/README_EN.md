# zstream

**Business Actor Light Wrapper**: Lightweight wrapper around `zactor.Actor`, implements `IServerActor` lifecycle interface.

## Positioning

- Provides a independently startable business Actor Server (`NewServer` + `RunServer`)
- Suitable for organizing business logic by `Server` semantics without duplicating Actor lifecycle wrapper

Current implementation does not include additional streaming protocol processing logic; core capabilities come from embedded `zactor.Actor`.

## Core API

- `NewServer(actorConfig zmodel.ActorConfig) *Server`
- `RunServer(ctx context.Context) error`
