# ziface

**Interface Contract Layer**: Defines core abstractions and extension boundaries of the solution, no concrete implementations included.

## Module Positioning

- Unifies core interfaces like `Actor/Gate/Group/Discovery/Dispatcher`
- Reduces module coupling, convenient for implementation replacement and unit testing
- Provides stable boundaries for extension points (routing strategies, scripts, discovery, etc.)

## Core Interfaces (Commonly Used)

| Interface | Description |
|-----------|-------------|
| `IActor` | Actor capabilities: Push, Init, Run, SendMsg, CallActor, GetDispatcher, GetGroup, etc. |
| `IServerActor` | Extends IActor, adds RunServer (service startup phase) |
| `IGroup` | Group capabilities: AddActor, Run, RegisterRoutes, LookupActorsByMsgID, SetDiscoverer |
| `IMessage` | Business proto messages: MarshalVT/UnmarshalVT/GetMsgId |
| `IHttpServer` | HTTP: SetActor, GET/POST/Group, Run |
| `IDiscovery` | Service discovery: Register, FindAllByPrefix, Watch |
| `IDispatcher` | Message dispatch: Dispatch |
| `LocalRouter` | In-process routing: RouteLocal |
| `RemoteRouteStrategy` | Cross-process routing selection strategy |
| `IGroupRouteTableView` | In-process routing read-only view (zero-allocation) |
| `IGroupRemoteRouteTableView` | Cross-process candidates read-only view (zero-allocation) |
| `Handle` | Client message handler function |

## Usage

- Business and solution components interact through interfaces, convenient for mock and extension;
- New Actor implements `IActor` (or `IServerActor`) to integrate with Group/Gate.

## Related Documentation

- Overall architecture: `../docs/ARCHITECTURE.md`
- Module navigation: `../docs/MODULE_API.md`
- Model definitions: `../zmodel/README.md`
