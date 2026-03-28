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

## 2. Known Blocking Point: Go 1.24.0 + bytedance/sonic (linux/amd64)

When using **Go 1.24.0** and **enabling `-tags sonic`** to build on `linux/amd64`, link errors may occur:

- `link: github.com/bytedance/sonic/loader: invalid reference to runtime.lastmoduledatap`

Dependency chain (when `-tags sonic` enabled):

- `examples/...` → `github.com/aiyang-zh/zhenyi-base/zserialize` → `github.com/bytedance/sonic` → `github.com/bytedance/sonic/loader`

This issue is **sonic/loader's compatibility with Go runtime internal symbols**, unrelated to business logic.

### Avoidance/Fix Suggestions (Choose One)

- **Recommended**: Use **Go 1.24.1+** (Go restored related linkname compatibility after 1.24.1).
- **Alternative (not recommended)**: Use `-ldflags="-checklinkname=0"` at build time (relaxes linkname check, toolchain-level workaround).
- **Engineering solution (already implemented)**: Use standard library `encoding/json` by default; only enable sonic with explicit `-tags sonic`, thus avoiding default build path being affected by sonic's platform/Go version compatibility.

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

Uses **`openeuler/go:1.24.1-oe2403lts`**: runs **`CGO_ENABLED=0 go test ./...`**, **`go build ./...`**, and explicit **`go build`** on **`im_single_demo`**, **`im_single_client`**, **`im_multi_demo`**, **`im_multi_client_load`**, **`gencert`** (compile-only, no long-running servers).

```bash
make openeuler-check
# or
./scripts/openeuler-check.sh
```

Notes: same **`ZHENYI_BASE`** requirement; override with **`GOLANG_IMAGE`** / **`PLATFORM`** (often **`linux/amd64`**); **`docker pull openeuler/go:1.24.1-oe2403lts`** before first run.

## 5. Future Suggestions (Stricter Xinchuang Validation)

- **Run on real Xinchuang OS**: Execute `make bug-check` directly on target environment (or container/VM).
- **Cover more architectures**: Like loong64/riscv64 (need to evaluate dependency library support).
- **External component replacement verification**: Run the distributed flow of `examples/im_multi_demo` with your planned domestic alternative components.
