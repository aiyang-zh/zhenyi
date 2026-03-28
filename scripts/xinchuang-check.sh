#!/usr/bin/env bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_step() { printf "${BLUE}==> %s${NC}\n" "$1"; }
log_ok() { printf "${GREEN}✔ %s${NC}\n" "$1"; }
log_err() { printf "${RED}✘ %s${NC}\n" "$1"; }

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_err "缺少命令: $cmd"
    exit 1
  fi
}

go_version() {
  go version | awk '{print $3}' | sed 's/^go//'
}

main() {
  require_cmd go

  log_step "Go version"
  echo "go$(go_version)"

  log_step "CGO=0 unit tests"
  CGO_ENABLED=0 go test ./... -count=1
  log_ok "CGO=0 go test 通过"

  log_step "Native build (CGO=0, 本机 GOOS/GOARCH，不做交叉编译)"
  CGO_ENABLED=0 go build ./...
  log_ok "本机架构 go build 通过"
}

main "$@"
