# Xinchuang Adaptation Analysis and Initial Validation

This document provides `zhenyi` adaptation points in common "Xinchuang" environments and executable initial validation methods.

> Note: "Xinchuang" here defaults to **domestic CPU (mostly arm64) + domestic OS (Linux distribution) + domestic middleware/database alternatives** combination.
> This repository is a Go project; core adaptation targets are typically: **no CGO dependency, cross-architecture buildable, no platform strong binding in critical paths**.

## 1. Adaptation Conclusion (Current Repository State)

- **CGO dependency**: No `import "C"` / `#cgo` found; `go test ./...` passes under `CGO_ENABLED=0`.
- **Platform strong binding**: No platform files like `_linux.go` / `_darwin.go` found in repository; small amount of `//go:build` exists (for debug/lifecycle switches), doesn't affect Xinchuang.
- **JSON codec strategy (Xinchuang-friendly default)**:
  - Uses standard library `encoding/json` by default (no extra tags, most stable across platforms).
  - If you need `sonic` (higher performance, may be affected by Go/platform compatibility), explicitly add at build time: `-tags sonic`.
- **Multi-arch / Linux distro builds**: Repository scripts do **not** set `GOOS/GOARCH` for cross-compilation; run `go test` / `go build` on **target hardware** (or same-arch VM), or use **`make openeuler-check`** inside the openEuler Go image at the **container's architecture**.
- **Quick local check**: `make xinchuang-check` runs `CGO_ENABLED=0 go test ./...` and **native** `go build ./...` (on macOS/Windows this validates that OS only, not production Linux).

## 2. Known: bytedance/sonic in Some Environments (linux/amd64)

With **`-tags sonic`** on **`linux/amd64`**, **sonic/loader** vs Go runtime symbol compatibility may vary by toolchain. Dependency chain when sonic is enabled: `examples/...` → `zhenyi-base/zserialize` → `bytedance/sonic` → `sonic/loader`.

**Suggestion**: Follow root **`go.mod`** for the Go version; validate with `go test` / `go build` on the target architecture. Optional workaround: `-ldflags="-checklinkname=0"` (not recommended). The repo defaults to `encoding/json`; sonic is only used when `-tags sonic` is set.

## 3. Domestic OS/Middleware Dimension Suggestions

`zhenyi` itself is a pure Go solution layer; external dependencies mainly focus on "optional components":

- **Discovery**: Etcd (can be replaced with other implementations; or use Noop when distributed is not needed).
- **Bus**: NATS (can be replaced with other implementations; or completely not needed for single-node examples).

Suggested to make these external components a "pluggable list" for Xinchuang deployment:

- Required: none (single-node closed loop)
- Optional: Etcd / NATS (distributed capability)

## 4. Initial Validation (Executable)

Repository provides script:

```bash
./scripts/xinchuang-check.sh
```

It executes:

- `CGO_ENABLED=0 go test ./... -count=1`
- `CGO_ENABLED=0 go build ./...` (**host** `GOOS/GOARCH`, no cross-compilation)
- For **sonic** path verification, run `CGO_ENABLED=0 go build ./... -tags sonic` on a **target linux/amd64** environment and refer to compatibility notes above.

### 4.1 openEuler official Go image (tests + example builds)

Uses the **`GOLANG_IMAGE`** openEuler Go image (default in `scripts/openeuler-check.sh`; must satisfy root **`go.mod`**): runs **`CGO_ENABLED=0 go test ./...`**, **`go build ./...`**, and explicit **`go build`** on **`im_single_demo`**, **`im_single_client`**, **`im_multi_demo`**, **`im_multi_client_load`**, **`gencert`** (compile-only, no long-running servers).

```bash
make openeuler-check
# or
./scripts/openeuler-check.sh
```

Notes: same **`ZHENYI_BASE`** requirement; override with **`GOLANG_IMAGE`** / **`PLATFORM`** (often **`linux/amd64`**); **`docker pull`** your chosen **`GOLANG_IMAGE`** before first run.

## 5. Future Suggestions (Stricter Xinchuang Validation)

- **Run on real Xinchuang OS**: Execute `make bug-check` directly on target environment (or container/VM).
- **Cover more architectures**: Like loong64/riscv64 (need to evaluate dependency library support).
- **External component replacement verification**: Run the distributed flow of `examples/im_multi_demo` with your planned domestic alternative components.
