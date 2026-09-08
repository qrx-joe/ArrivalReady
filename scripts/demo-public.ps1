# Arrival Ready 公网演示一键脚本（本机全栈 + Cloudflare 快速隧道）。
#
# 面向路演/评委场景：在本机跑通完整三端（真模型 AI），并把 Web/API/MinIO
# 通过 cloudflared 快速隧道暴露为三个临时公网 URL（无需 Cloudflare 账号）。
#
# 前提：
#   - PostgreSQL（127.0.0.1:54329，库 arrival_test）与 MinIO（:9000，
#     账号 arrival / arrival_dev_password）已在运行（compose 或本机进程均可）；
#   - Go 1.27 / Node 24 + pnpm / Python 3.13 + uv / cloudflared 在 PATH；
#   - services/ai/.env 已配置 MODEL_API_KEY / MODEL_BASE_URL / MODEL_ID（真模型）。
#
# 用法：powershell -ExecutionPolicy Bypass -File scripts\demo-public.ps1
# 输出：%TEMP%\arrival-demo\demo-urls.txt（三个公网 URL）与 logs\*.log。
#
# 注意：快速隧道 URL 在 cloudflared 重启后会变化，需重跑本脚本并重新分享链接；
#       Web 是生产构建，NEXT_PUBLIC_API_URL 在构建时烘焙，因此每次运行都会重新构建。

$ErrorActionPreference = "Stop"
$Repo = Split-Path -Parent $PSScriptRoot
$Run  = Join-Path $env:TEMP "arrival-demo"
$Logs = Join-Path $Run "logs"
New-Item -ItemType Directory -Force -Path $Logs | Out-Null

function Start-Detached {
    # 独立进程启动（不依赖当前终端/会话存活），输出重定向（覆盖）到日志文件。
    param([string]$Name, [string]$WorkDir, [string]$Command)
    $ps = "`$ErrorActionPreference='Continue'; Set-Location '$WorkDir'; $Command *> '$Logs\$Name.log'"
    Start-Process -WindowStyle Hidden -FilePath "powershell" -ArgumentList @("-NoProfile", "-Command", $ps) | Out-Null
    Write-Host "[demo] started $Name"
}

function Read-TunnelUrl {
    # 从本轮新建的日志中读取隧道 URL（旧日志已在步骤 0 删除，无历史干扰）。
    param([string]$Name)
    $log = Join-Path $Logs "$Name.log"
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path $log) {
            $m = Select-String -Path $log -Pattern "https://[a-z0-9-]+\.trycloudflare\.com" -ErrorAction SilentlyContinue | Select-Object -Last 1
            if ($m) { return $m.Matches[0].Value }
        }
        Start-Sleep -Seconds 2
    }
    throw "$Name tunnel URL not found in $log (see $log)"
}

# 0) 清理上一轮演示遗留：
#    a) 按监听端口停 Web(3000)/API(8080)/AI(8100)；
#    b) 停本脚本启动过的 cloudflared 隧道；
#    c) 按命令行找到本脚本的包装进程，taskkill /T 连子树一起结束
#       （覆盖还在构建中、尚未监听端口的 web-build 残留）；
#    d) 删除旧日志，避免 URL 读取竞态。
$ports = Get-NetTCPConnection -LocalPort 3000, 8080, 8100 -State Listen -ErrorAction SilentlyContinue
$ports | Select-Object -ExpandProperty OwningProcess -Unique |
    ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
Get-CimInstance Win32_Process -Filter "Name='cloudflared.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like "*--url http://127.0.0.1*" } |
    ForEach-Object { taskkill /F /T /PID $_.ProcessId 2>$null }
Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like "*arrival-demo*" -and $_.CommandLine -like "*logs*" } |
    ForEach-Object { taskkill /F /T /PID $_.ProcessId 2>$null }
Remove-Item (Join-Path $Logs "*.log") -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# 1) 三条快速隧道（统一 http2 协议，QUIC 在部分网络下有 SRV/DNS 瞬断问题）。
Start-Detached "tunnel-web"   $Repo "cloudflared tunnel --url http://127.0.0.1:3000 --protocol http2"
Start-Detached "tunnel-api"   $Repo "cloudflared tunnel --url http://127.0.0.1:8080 --protocol http2"
Start-Detached "tunnel-minio" $Repo "cloudflared tunnel --url http://127.0.0.1:9000 --protocol http2"
$WebUrl   = Read-TunnelUrl "tunnel-web"
$ApiUrl   = Read-TunnelUrl "tunnel-api"
$MinioUrl = Read-TunnelUrl "tunnel-minio"
Write-Host "[demo] web=$WebUrl api=$ApiUrl minio=$MinioUrl"

# 2) AI 服务（真模型，凭据由 services/ai/.env 提供；勿设 ARRIVAL_FAKE_MODEL）。
Start-Detached "ai" "$Repo\services\ai" "uv run uvicorn app.main:app --host 127.0.0.1 --port 8100"

# 3) Go API：S3 与 CORS 指向公网隧道（浏览器直传预签名 URL、跨域直连 API 都要走公网域名）。
$apiExe = Join-Path $Run "arrival-api.exe"
Push-Location "$Repo\services\api"
$env:GOPROXY = "https://goproxy.cn,direct"
go build -o $apiExe ./cmd/api
Pop-Location
# 单引号 here-string + 占位符：避免外层提前展开 $env: 变量。
$apiEnv = @'
$env:DATABASE_URL='postgres://postgres@127.0.0.1:54329/arrival_test?sslmode=disable';
$env:ARRIVAL_ENABLE_TEST_IDENTITY='1';
$env:ARRIVAL_TEST_IDENTITY_SECRET='local-dev-secret-0123456789abcdef';
$env:AI_SERVICE_URL='http://127.0.0.1:8100';
$env:S3_ENDPOINT='{0}';
$env:S3_USE_SSL='1';
$env:S3_ACCESS_KEY='arrival';
$env:S3_SECRET_KEY='arrival_dev_password';
$env:WEB_ORIGIN='{1}';
$env:ENV='dev';
$env:PORT='8080';
'@ -f ($MinioUrl -replace 'https://', ''), $WebUrl
Start-Detached "api" "$Repo\services\api" "$apiEnv & '$apiExe'"

# 4) Web 生产构建（NEXT_PUBLIC_API_URL 构建期烘焙进浏览器代码）+ 启动。
Start-Detached "web-build" "$Repo\apps\web" "`$env:NEXT_PUBLIC_API_URL='$ApiUrl/api/v1'; pnpm build; if (`$LASTEXITCODE -eq 0) { pnpm start }"

# 5) 等待全链路就绪并写出链接清单。
$deadline = (Get-Date).AddSeconds(300)
$ready = $false
while ((Get-Date) -lt $deadline) {
    try {
        $a = Invoke-WebRequest -UseBasicParsing "$ApiUrl/healthz" -TimeoutSec 5
        $w = Invoke-WebRequest -UseBasicParsing $WebUrl -TimeoutSec 10
        if ($a.StatusCode -eq 200 -and $w.StatusCode -lt 500) { $ready = $true; break }
    } catch {}
    Start-Sleep -Seconds 5
}
$urls = @"
Arrival Ready 公网演示（生成于 $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")）
  演示入口(Web) : $WebUrl
  API          : $ApiUrl
  MinIO(S3)    : $MinioUrl
登录：打开演示入口 -> 点击「开发者登录（本地测试身份）」。
演示数据：全聚德·前门店 / 中国国家博物馆 / 前门中轴线文创（含已评分审计与发现项）。
注意：本机休眠/关机或 cloudflared 退出都会使链接失效；重跑本脚本会得到新链接。
"@
$urls | Out-File -Encoding utf8 (Join-Path $Run "demo-urls.txt")
$urls | Write-Host
if (-not $ready) { Write-Warning "部分服务未在超时内就绪，请查看 $Logs 下各日志"; exit 1 }
Write-Host "[demo] 全链路就绪。" -ForegroundColor Green
