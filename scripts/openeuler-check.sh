#!/usr/bin/env bash
# zhenyi + zhenyi-base 在 openEuler 官方 Go 镜像中的适配验证（单测 + 全模块与示例编译）。
# 依赖：与根目录 go.mod 中 replace ../zhenyi-base 一致，需并列挂载 zhenyi-base 源码。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZHENYI_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ZHENYI_BASE="${ZHENYI_BASE:-$ZHENYI_ROOT/../zhenyi-base}"
# Default: Docker Hub openeuler/go tag "1.25.0-oe2403sp1" (Go 1.25.0, oe 24.03 SP1, amd64+arm64). Newer: e.g. 1.25.4-oe2403sp2.
GOLANG_IMAGE="${GOLANG_IMAGE:-openeuler/go:1.25.0-oe2403sp1}"
# openEuler 镜像多为 amd64；Apple Silicon 上需 QEMU 或省略 --platform（由 Docker 处理）
PLATFORM="${PLATFORM:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf '%b\n' "${BLUE}==>${NC} $*"; }

if ! command -v docker >/dev/null 2>&1; then
  echo -e "${RED}未找到 docker${NC}" >&2
  exit 1
fi

if [[ ! -d "$ZHENYI_BASE" ]]; then
  echo -e "${RED}未找到 zhenyi-base：${ZHENYI_BASE}${NC}" >&2
  echo "请将 zhenyi-base 与本仓库并列克隆，或设置 ZHENYI_BASE=/path/to/zhenyi-base" >&2
  exit 1
fi

echo ""
echo "=== zhenyi openEuler 适配验证 ==="
echo "镜像: ${GOLANG_IMAGE}"
echo "zhenyi: ${ZHENYI_ROOT}"
echo "zhenyi-base: ${ZHENYI_BASE}"
echo ""

DOCKER_ARGS=(--rm)
if [[ -n "${PLATFORM}" ]]; then
  DOCKER_ARGS+=(--platform "${PLATFORM}")
fi
# 有终端时用 -it，便于本地 Ctrl+C；CI 无 TTY 时去掉 -t
if [[ -t 0 ]] && [[ -t 1 ]]; then
  DOCKER_ARGS=(-it "${DOCKER_ARGS[@]}")
fi

docker run "${DOCKER_ARGS[@]}" \
  -v "${ZHENYI_ROOT}:/src/zhenyi" \
  -v "${ZHENYI_BASE}:/src/zhenyi-base" \
  -w /src/zhenyi \
  -e CGO_ENABLED=0 \
  "${GOLANG_IMAGE}" \
  bash -ceu '
    set -euo pipefail
    echo "Go: $(go version)"
    echo ""
    echo "==> 单测 CGO_ENABLED=0 go test ./..."
    go test ./... -count=1
    echo ""
    echo "==> 全量编译 go build ./..."
    go build ./...
    echo ""
    echo "==> 示例/工具显式编译（im_single / im_multi / gencert）"
    go build -o /dev/null ./examples/im_single_demo/...
    go build -o /dev/null ./examples/im_single_client/...
    go build -o /dev/null ./examples/im_multi_demo/...
    go build -o /dev/null ./examples/im_multi_client_load/...
    go build -o /dev/null ./examples/im_single_demo/gencert/...
    echo ""
    echo "=== openEuler 镜像内验证通过 ==="
  '

echo -e "${GREEN}完成。${NC}"
