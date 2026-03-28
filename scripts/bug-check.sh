#!/usr/bin/env bash

set -euo pipefail

# Usage:
#   ./scripts/bug-check.sh
#   RACE=0 STATICCHECK=0 GOSEC=0 ./scripts/bug-check.sh
#   STRICT_TOOLS=1 ./scripts/bug-check.sh
#
# Flags via env:
#   RACE=1|0         run `go test -race ./...` (default: 1)
#   COVER=1|0        run coverage pass (default: 1)
#   VET=1|0          run go vet (default: 1)
#   STATICCHECK=1|0  run staticcheck if available (default: 1)
#   GOSEC=1|0        run gosec if available (default: 1)
#   STRICT_TOOLS=1   fail when staticcheck/gosec missing (default: 0)

RACE="${RACE:-1}"
COVER="${COVER:-1}"
VET="${VET:-1}"
STATICCHECK="${STATICCHECK:-1}"
GOSEC="${GOSEC:-1}"
STRICT_TOOLS="${STRICT_TOOLS:-0}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_step() {
  printf "${BLUE}==> %s${NC}\n" "$1"
}

log_ok() {
  printf "${GREEN}✔ %s${NC}\n" "$1"
}

log_warn() {
  printf "${YELLOW}⚠ %s${NC}\n" "$1"
}

log_err() {
  printf "${RED}✘ %s${NC}\n" "$1"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_err "缺少命令: $cmd"
    exit 1
  fi
}

run_optional_tool() {
  local tool="$1"
  local run_cmd="$2"
  if command -v "$tool" >/dev/null 2>&1; then
    log_step "Run $tool"
    eval "$run_cmd"
    log_ok "$tool 通过"
  else
    if [[ "$STRICT_TOOLS" == "1" ]]; then
      log_err "未安装 $tool，且 STRICT_TOOLS=1"
      exit 1
    fi
    log_warn "未安装 $tool，已跳过（设置 STRICT_TOOLS=1 可强制失败）"
  fi
}

list_cover_packages() {
  # 仅覆盖“有 _test.go 的非 examples 包”，避免将 demo/main 包与无测试包纳入
  # 导致某些本地 Go 工具链出现 covdata 相关报错。
  go list -f '{{.ImportPath}} {{len .TestGoFiles}}' ./... \
    | awk '$2 > 0 && $1 !~ /\/examples\// {print $1}'
}

main() {
  require_cmd go

  log_step "Go version"
  go version

  log_step "Run unit tests"
  go test ./...
  log_ok "go test 通过"

  if [[ "$RACE" == "1" ]]; then
    log_step "Run race tests"
    go test -race ./...
    log_ok "race 通过"
  else
    log_warn "RACE=0，跳过 race 测试"
  fi

  if [[ "$COVER" == "1" ]]; then
    log_step "Run coverage pass"
    cover_pkgs=()
    while IFS= read -r pkg; do
      [[ -n "$pkg" ]] && cover_pkgs+=("$pkg")
    done < <(list_cover_packages)
    if [[ "${#cover_pkgs[@]}" -eq 0 ]]; then
      log_warn "未发现可覆盖率测试包（已跳过）"
    else
      go test -cover "${cover_pkgs[@]}"
      log_ok "coverage 通过（${#cover_pkgs[@]} 包）"
    fi
  else
    log_warn "COVER=0，跳过 coverage 测试"
  fi

  if [[ "$VET" == "1" ]]; then
    log_step "Run go vet"
    go vet ./...
    log_ok "go vet 通过"
  else
    log_warn "VET=0，跳过 go vet"
  fi

  if [[ "$STATICCHECK" == "1" ]]; then
    run_optional_tool "staticcheck" "staticcheck ./..."
  else
    log_warn "STATICCHECK=0，跳过 staticcheck"
  fi

  if [[ "$GOSEC" == "1" ]]; then
    run_optional_tool "gosec" "gosec -exclude=G115 ./..."
  else
    log_warn "GOSEC=0，跳过 gosec"
  fi

  log_ok "Bug check 全部完成"
}

main "$@"

