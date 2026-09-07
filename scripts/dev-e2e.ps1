# Arrival Ready E2E 一键脚本（Windows PowerShell）
# 启动本地全栈（PG + MinIO + Go API + AI[离线适配器] + Web）供 Playwright 运行。
# 前提：ephemeral PostgreSQL（端口 54329）与 MinIO（:9000）已在运行；
#       Go/Node/Python 工具链就绪（见 AGENTS.md §6）。

param(
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot

Write-Host "[1/4] 启动 Go API :8080（本地测试身份）" -ForegroundColor Cyan
$apiEnv = @{
    DATABASE_URL                  = "postgres://postgres@127.0.0.1:54329/arrival_test?sslmode=disable"
    ARRIVAL_ENABLE_TEST_IDENTITY  = "1"
    ARRIVAL_TEST_IDENTITY_SECRET  = "local-dev-secret-0123456789abcdef"
    AI_SERVICE_URL                = "http://127.0.0.1:8100"
    S3_ENDPOINT                   = "127.0.0.1:9000"
    S3_ACCESS_KEY                 = "arrival"
    S3_SECRET_KEY                 = "arrival_dev_password"
    ENV                           = "dev"
}
Start-Process powershell -ArgumentList @(
    "-NoExit", "-Command",
    "Set-Location '$RepoRoot\services\api'; " +
    ($apiEnv.GetEnumerator() | ForEach-Object { "\$env:$($_.Key)='$($_.Value)'" }) -join "; " +
    "; go run ./cmd/api"
)

# 模型模式跟随配置：services/ai/.env 配有 MODEL_API_KEY 时走真实模型，
# 否则才退回离线确定性 fake——演示宣称与实际运行必须一致。
$aiEnv = "Set-Location '$RepoRoot\services\ai'; "
if (Test-Path "$RepoRoot\services\ai\.env") {
    $hasKey = Select-String -Path "$RepoRoot\services\ai\.env" -Pattern '^\s*MODEL_API_KEY\s*=\s*\S' -Quiet
    if ($hasKey) {
        Write-Host "[2/4] 启动 AI Service :8100（真实模型：.env 检出 MODEL_API_KEY）" -ForegroundColor Cyan
    } else {
        Write-Host "[2/4] 启动 AI Service :8100（ARRIVAL_FAKE_MODEL=1，离线确定性）" -ForegroundColor Cyan
        $aiEnv += "`$env:ARRIVAL_FAKE_MODEL='1'; "
    }
} else {
    Write-Host "[2/4] 启动 AI Service :8100（ARRIVAL_FAKE_MODEL=1，离线确定性）" -ForegroundColor Cyan
    $aiEnv += "`$env:ARRIVAL_FAKE_MODEL='1'; "
}
$aiEnv += "uv run uvicorn app.main:app --port 8100"
Start-Process powershell -ArgumentList @("-NoExit", "-Command", $aiEnv)

Write-Host "[3/4] 启动 Web :3000" -ForegroundColor Cyan
Start-Process powershell -ArgumentList @(
    "-NoExit", "-Command",
    "Set-Location '$RepoRoot\apps\web'; pnpm dev"
)

Write-Host "[4/4] 等待三端就绪…" -ForegroundColor Cyan
$deadline = (Get-Date).AddSeconds(90)
while ((Get-Date) -lt $deadline) {
    $api = $false; $ai = $false; $web = $false
    try { $api = (Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/healthz -TimeoutSec 2).StatusCode -eq 200 } catch {}
    try { $ai = (Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8100/healthz -TimeoutSec 2).StatusCode -eq 200 } catch {}
    try { $web = (Invoke-WebRequest -UseBasicParsing http://127.0.0.1:3000 -TimeoutSec 2).StatusCode -lt 500 } catch {}
    if ($api -and $ai -and $web) { break }
    Start-Sleep -Seconds 2
}
if ($api -and $ai -and $web) {
    Write-Host "全栈就绪。运行 E2E：  cd apps\web; pnpm exec playwright test" -ForegroundColor Green
    if (-not $KeepRunning) { Write-Host "（各服务在独立窗口中运行，跑完手动关窗或 Stop-Process）" }
} else {
    Write-Host "api=$api ai=$ai web=$web —— 超时，请检查各窗口日志" -ForegroundColor Red
}
