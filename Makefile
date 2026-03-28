# zhenyi Makefile

.PHONY: test test-unit bench coverage cover-html bug-check bug-check-strict install-hooks
.PHONY: docs-check release-check
.PHONY: xinchuang-check openeuler-check

test:
	go test ./... -race -count=2

# 仅单元测试（供 pre-commit 等快速检查）
test-unit:
	go test ./...

# 基准测试
bench:
	go test ./... -bench=. -benchmem -run=^$$

# 覆盖率
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
	@go tool cover -func=coverage.out | tail -1

cover-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html"

# 代码层面 bug 检查（本地默认：工具缺失时仅警告并跳过）
bug-check:
	./scripts/bug-check.sh

# 严格模式：staticcheck/gosec 未安装则直接失败
bug-check-strict:
	STRICT_TOOLS=1 ./scripts/bug-check.sh

# 文档链接可达（快速检查：仅相对链接是否存在）
docs-check:
	python3 ./scripts/docs-check.py

# 发布前终检：文档 + 快速测试 + 全量 bug-check（最贴近 CI）
release-check: docs-check
	go test ./... -count=1
	$(MAKE) bug-check

# 信创初步验证：CGO=0 测试 + 本机架构 go build（不做交叉编译；目标机验证见 docs/XINCHUANG.md）
xinchuang-check:
	chmod +x ./scripts/xinchuang-check.sh
	./scripts/xinchuang-check.sh

# openEuler 官方 Go 镜像：单测 + 全量与示例编译（需 ../zhenyi-base）
openeuler-check:
	chmod +x ./scripts/openeuler-check.sh
	./scripts/openeuler-check.sh

# 启用 Git 钩子（提交前运行 make test-unit）
install-hooks:
	git config core.hooksPath .githooks
	@echo "已启用 .githooks，提交前将运行 make test-unit"

