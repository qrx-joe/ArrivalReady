# Arrival Ready 本地开发脚本（Windows 原生入口，与 Makefile 等价）。
# 用法：
#   .\scripts\dev.ps1 infra   # 启动 PostgreSQL + MinIO
#   .\scripts\dev.ps1 down    # 停止（保留数据卷）
#   .\scripts\dev.ps1 all     # infra + 三个服务各开一个终端窗口
#   .\scripts\dev.ps1 ci      # 本地复现 CI 离线步骤

param(
    [Parameter(Position = 0)]
    [ValidateSet("infra", "down", "all", "ci", "help")]
    [string]$Target = "help"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot

function Invoke-Infra {
    Push-Location $RepoRoot
    docker compose up -d
    Write-Host "等待依赖健康检查通过：docker compose ps" -ForegroundColor Cyan
    Pop-Location
}

function Invoke-Down {
    Push-Location $RepoRoot
    # 注意：绝不使用 down -v——数据卷受保护（执行方案 B04 回滚约束）。
    docker compose down
    Pop-Location
}

function Start-ServiceWindow {
    param([string]$Title, [string]$WorkDir, [string]$Command)
    Start-Process powershell -ArgumentList @(
        "-NoExit", "-Command",
        "Set-Location '$WorkDir'; Write-Host '$Title' -ForegroundColor Cyan; $Command"
    )
}

switch ($Target) {
    "infra" { Invoke-Infra }
    "down" { Invoke-Down }
    "all" {
        Invoke-Infra
        Start-ServiceWindow "Go API :8080" "$RepoRoot\services\api" "go run ./cmd/api"
        Start-ServiceWindow "AI Service :8100" "$RepoRoot\services\ai" "uv run uvicorn app.main:app --port 8100 --reload"
        Start-ServiceWindow "Web :3000" "$RepoRoot\apps\web" "pnpm dev"
    }
    "ci" {
        # 原生执行，不依赖 make（Windows 默认无 make，见 R-15 前提记录）。
        Push-Location $RepoRoot
        python scripts/validate_rules.py --fixtures standards/irrs/0.1.0/fixtures
        if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
        python scripts/validate_rules.py standards/irrs/0.1.0/rules.yaml
        if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
        python scripts/validate_contracts.py --fixtures contracts/fixtures
        if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
        python scripts/validate_contracts.py --openapi contracts/openapi/arrivalready.yaml
        if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
        Push-Location services/api
        go vet ./...; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        go test -p 1 ./...; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        go build ./...; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        Pop-Location
        Push-Location services/ai
        uv run ruff check .; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        uv run mypy app; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        uv run pytest -q; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        Pop-Location
        Push-Location apps/web
        pnpm lint; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        pnpm typecheck; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        pnpm test; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        pnpm build; if ($LASTEXITCODE -ne 0) { Pop-Location; Pop-Location; exit 1 }
        Pop-Location
        Write-Host "全部离线检查通过" -ForegroundColor Green
        Pop-Location
    }
    "help" {
        Write-Host "用法: .\scripts\dev.ps1 {infra|down|all|ci|help}"
    }
}
