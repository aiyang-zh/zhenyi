#!/usr/bin/env bash
set -euo pipefail
ZHENYI="$(cd "$(dirname "$0")/.." && pwd)"
ZDISCOVERY="$ZHENYI/zdiscovery"
cd "$ZDISCOVERY"

# 默认：仅按 discovery 的现有实现启动依赖（请自行确保本机等依赖已就绪）
# 需要 Docker 里的 Etcd 时： ZDISCOVERY_DOCKER_ETCD=1 ./scripts/run_integration.sh
COMPOSE=(docker compose -f "$ZHENYI/docker-compose.test.yml")
if [[ "${ZDISCOVERY_DOCKER_ETCD:-}" == "1" ]]; then
  COMPOSE+=(--profile docker-etcd)
fi
"${COMPOSE[@]}" up -d

echo "等待 discovery 依赖就绪（Etcd 使用本机 ${ZDISCOVERY_ETCD_ENDPOINTS:-127.0.0.1:2379}）..."
sleep 5
cd "$ZHENYI"
go mod tidy

echo ""
echo ">>> 单测 + 集成测（含覆盖率）..."
go test ./zdiscovery/... -tags=integration -count=1 -timeout=120s -short=false -v \
  -coverprofile=zdiscovery.coverage.out -covermode=atomic "$@"

echo ""
echo ">>> 基测（Benchmark）..."
go test ./zdiscovery/... -tags=integration -bench=. -benchmem -count=2 -run=^$ -timeout=60s 2>/dev/null || true

# 覆盖率摘要
COV_TOTAL=$(go tool cover -func=zdiscovery.coverage.out | grep '^total:' | awk '{print $3}')
echo ""
echo "=========================================="
echo "  zdiscovery: 单测+集成测 通过，基测见上"
echo "  语句覆盖率: ${COV_TOTAL:-N/A}（单测+集成测合并）"
echo "  各文件: go tool cover -func=zdiscovery.coverage.out"
echo "  HTML:  go tool cover -html=zdiscovery.coverage.out -o zdiscovery.coverage.html"
echo "=========================================="
cd "$ZDISCOVERY"
docker compose -f "$ZHENYI/docker-compose.test.yml" down 2>/dev/null || true

