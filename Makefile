# Arrival Ready 本地开发入口（TECH_SPEC §23）。
# Windows 协作环境优先用 scripts/dev.ps1；本 Makefile 在 Git Bash / WSL 下可用。
# 约定：未实现的入口不定义占位目标——`migrate`、`eval` 随 B05/B08 落地后加入。

SHELL := /bin/bash
GOPROXY ?= https://goproxy.cn,direct
export GOPROXY

.PHONY: help infra down api ai web lint test typecheck contracts ci

help: ## 显示可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

infra: ## 启动 PostgreSQL + MinIO（compose，带健康检查）
	docker compose up -d
	@echo "等待依赖健康检查通过：docker compose ps"

down: ## 停止基础设施（保留数据卷；绝不使用 down -v）
	docker compose down

api: ## 前台运行 Go API (:8080)
	cd services/api && go run ./cmd/api

ai: ## 前台运行 AI Service (:8100)
	cd services/ai && uv run uvicorn app.main:app --port 8100 --reload

web: ## 前台运行 Web (:3000)
	cd apps/web && pnpm dev

lint: ## 三端静态检查（gofmt/vet + ruff/mypy + biome）
	cd services/api && go vet ./... && test -z "$$(gofmt -l .)"
	cd services/ai && uv run ruff check . && uv run mypy app
	cd apps/web && pnpm lint

typecheck: ## 三端类型检查（go build + mypy + tsc）
	cd services/api && go build ./...
	cd services/ai && uv run mypy app
	cd apps/web && pnpm typecheck

test: ## 三端测试（go test + pytest + vitest）
	cd services/api && go test ./...
	cd services/ai && uv run pytest -q
	cd apps/web && pnpm test

contracts: ## 契约正反例离线校验（无模型 key、无数据库）
	python scripts/validate_rules.py --fixtures standards/irrs/0.1.0/fixtures
	python scripts/validate_rules.py standards/irrs/0.1.0/rules.yaml
	python scripts/validate_contracts.py --fixtures contracts/fixtures
	python scripts/validate_contracts.py --openapi contracts/openapi/arrivalready.yaml

ci: ## 本地复现 CI 全部离线步骤（等价 .github/workflows/ci.yml）
	$(MAKE) contracts lint typecheck test
