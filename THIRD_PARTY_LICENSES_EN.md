# Third-Party Dependencies and License Notice

This file is provided to help users quickly locate third-party dependency sources and license information used by `zhenyi`.
**For legal purposes, always refer to each dependency's own LICENSE/NOTICE** (this file is not legal advice).

## Dependency Sources

- **Go modules**: Dependencies are declared in `go.mod`/`go.sum`.
- **Indirect dependencies**: Go may bring in transitive modules marked as `// indirect`.
- **`replace` (for audits)**: The root `go.mod` pins `google.golang.org/grpc` to align transitive pulls with etcd expectations. This **does not** change third-party license terms.

## Direct Dependencies (first `require` block in `go.mod`; keep in sync)

These versions should match `go.mod` (re-check before release):

- `github.com/aiyang-zh/zhenyi-base` — `v1.1.0`
- `github.com/d5/tengo/v2` — `v2.17.0`
- `github.com/dop251/goja` — `v0.0.0-20260311135729-065cd970411c`
- `github.com/emmansun/gmsm` — `v0.41.1`
- `github.com/nats-io/nats-server/v2` — `v2.12.4` (primarily integration tests / embedded NATS workflows)
- `github.com/nats-io/nats.go` — `v1.49.0`
- `github.com/panjf2000/ants/v2` — `v2.11.6`
- `github.com/pelletier/go-toml/v2` — `v2.2.4`
- `github.com/stretchr/testify` — `v1.11.1`
- `github.com/yuin/gopher-lua` — `v1.1.1`
- `go.etcd.io/etcd/client/v3` — `v3.6.8`
- `go.starlark.net` — `v0.0.0-20260210143700-b62fd896b91b`
- `go.uber.org/zap` — `v1.27.1`
- `google.golang.org/protobuf` — `v1.36.11`

## How to Verify Licenses (Recommended)

From the repository root (this will download modules; the output is for one-off verification and does not need to be committed):

```bash
go mod download
go list -m -json all
```

Then inspect each module's source directory for `LICENSE`/`NOTICE`/`COPYING`, etc.

**Maintenance**: When bumping direct dependencies in `go.mod`, update the version numbers in this section as well.
