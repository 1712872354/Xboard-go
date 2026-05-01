# =============================================================
#  Xboard-Go 一键部署脚本 (Windows)
#  支持两种运行方式：
#    1. 独立运行（自动下载）:
#       irm https://github.com/1712872354/Xboard-go/releases/latest/download/install.ps1 | iex
#       $v="v2.0.2"; irm ... | iex  # 指定版本
#    2. 解压后运行（本地文件）:
#       解压 ZIP → cd 目录 → .\install.ps1
# =============================================================

param(
    [string]$Version = "latest"
)

$REPO = "1712872354/Xboard-go"
$RELEASES_URL = "https://github.com/$REPO/releases"
$API_URL = "https://api.github.com/repos/$REPO/releases"

function Write-OK  { Write-Host "  ✓ $args" -ForegroundColor Green }
function Write-Warn { Write-Host "  ⚠ $args" -ForegroundColor Yellow }
function Write-Err  { Write-Host "  ✗ $args" -ForegroundColor Red; exit 1 }
function Write-Step { param($n,$t) Write-Host "`n━━━ [$n/6] $t ━━━" -ForegroundColor Cyan }
function Write-Info { Write-Host "  → $args" -ForegroundColor Blue }

# =============================================================
# Step 1: 检测环境
# =============================================================
Write-Step 1 "检测运行环境"

$ARCH = $env:PROCESSOR_ARCHITECTURE
if ($ARCH -eq "AMD64") { $ARCH = "amd64" }
elseif ($ARCH -eq "ARM64") { $ARCH = "arm64" }
else { Write-Err "不支持的架构: $ARCH" }

Write-OK "系统: Windows ($ARCH)"
Write-OK "环境检测通过"

# =============================================================
# Step 2: 获取发布包
# =============================================================
Write-Step 2 "获取发布包"

$LOCAL_MODE = $false
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path 2>$null

if (Test-Path "$SCRIPT_DIR\xboard.exe") -and (Test-Path "$SCRIPT_DIR\xboard-scheduler.exe") {
    $LOCAL_MODE = $true
    $WORKDIR = $SCRIPT_DIR
    Write-OK "检测到本地文件，跳过下载"
} else {
    $WORKDIR = "C:\xboard"

    if ($Version -eq "latest") {
        Write-Info "正在获取最新版本信息..."
        $release = Invoke-RestMethod -Uri "$API_URL/latest" -UseBasicParsing
        $asset = $release.assets | Where-Object { $_.name -like "*windows-$ARCH.zip" } | Select-Object -First 1
        $DOWNLOAD_URL = $asset.browser_download_url
    } else {
        $DOWNLOAD_URL = "https://github.com/$REPO/releases/download/$Version/xboard-${Version}-windows-${ARCH}.zip"
    }

    if (-not $DOWNLOAD_URL) {
        Write-Err "未找到 Windows-$ARCH 架构的发布包`n请前往 $RELEASES_URL 手动下载"
    }

    Write-Info "下载: $DOWNLOAD_URL"

    try {
        $tmpFile = "$env:TEMP\xboard.zip"
        Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $tmpFile -UseBasicParsing
    } catch {
        Write-Err "下载失败，请检查网络或版本号是否正确"
    }

    $zipSize = (Get-Item $tmpFile).Length / 1MB
    Write-OK "下载完成 ($([math]::Round($zipSize, 1)) MB)"

    if (-not (Test-Path $WORKDIR)) { New-Item -ItemType Directory -Path $WORKDIR -Force | Out-Null }
    Expand-Archive -Path $tmpFile -DestinationPath $WORKDIR -Force
    Remove-Item $tmpFile -Force
    Write-OK "解压到 $WORKDIR"
}

Set-Location $WORKDIR

# =============================================================
# Step 3: 配置文件
# =============================================================
Write-Step 3 "配置文件"

if (-not (Test-Path "config.yaml")) {
    if (Test-Path "config.example.yaml") {
        Copy-Item "config.example.yaml" "config.yaml"
        Write-Warn "已生成 config.yaml，请编辑关键配置项"
        Write-Host ""
        Write-Info " 1. app.key       → 生成32位随机密钥"
        Write-Info " 2. app.url       → 服务对外地址"
        Write-Info " 3. database.*    → MySQL 连接信息"
        Write-Info " 4. redis.*       → Redis 连接信息"
        Write-Host ""
        $confirm = Read-Host "编辑 config.yaml 后输入 y 继续 [y/N]"
        if ($confirm -ne "y" -and $confirm -ne "Y") {
            Write-Warn "编辑 $WORKDIR\config.yaml 后重新运行本脚本"
            exit 0
        }
    } else {
        Write-Err "缺少 config.example.yaml"
    }
} else {
    Write-OK "config.yaml 已存在"
}

# =============================================================
# Step 4: 数据库迁移
# =============================================================
Write-Step 4 "数据库迁移"

$configContent = Get-Content "config.yaml" -Raw
if ($configContent -match 'key: "your-app-key-here"') {
    $newKey = -join ((65..90) + (97..122) + (48..57) + 33..47 | Get-Random -Count 32 | ForEach-Object { [char]$_ })
    $configContent = $configContent -replace 'key: "your-app-key-here"', "key: `"$newKey`""
    Set-Content -Path "config.yaml" -Value $configContent
    Write-OK "已自动生成 app.key"
}

$migrate = & ".\xboard.exe" --migrate --config config.yaml 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-OK "数据库迁移完成"
} else {
    Write-Err "迁移失败，请检查数据库配置`n$migrate"
}

# =============================================================
# Step 5: 创建管理员
# =============================================================
Write-Step 5 "管理员账号"

$seed = Read-Host "是否创建默认管理员？[Y/n]"
if ($seed -eq "" -or $seed -eq "Y" -or $seed -eq "y") {
    $result = & ".\xboard.exe" --seed --config config.yaml 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host ""
        Write-OK "管理员创建成功（请记录上方输出的邮箱和密码）"
    } else {
        Write-Warn "管理员创建失败（可能已存在，可忽略）"
    }
} else {
    Write-OK "跳过"
}

# =============================================================
# Step 6: 启动服务
# =============================================================
Write-Step 6 "启动服务"

Write-Host "  请选择启动方式:"
Write-Host "    1) 启动服务（当前窗口保持打开）"
Write-Host "    2) 跳过，稍后手动启动"
$startMode = Read-Host "  请输入 [1/2]"

if ($startMode -eq "" -or $startMode -eq "1") {
    $p1 = Start-Process -FilePath ".\xboard.exe" -ArgumentList "--config config.yaml" -NoNewWindow -PassThru
    $p2 = Start-Process -FilePath ".\xboard-scheduler.exe" -ArgumentList "--config config.yaml" -NoNewWindow -PassThru
    Write-OK "API 服务已启动 (PID: $($p1.Id))"
    Write-OK "调度器已启动 (PID: $($p2.Id))"
    Write-Warn "关闭此窗口将停止服务，建议使用 nssm 注册为 Windows 服务"
} else {
    Write-OK "跳过启动"
}

# =============================================================
# 完成
# =============================================================
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "   Xboard-Go 部署完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "   访问地址:  http://localhost:8080" -ForegroundColor Cyan
Write-Host "   管理后台:  http://localhost:8080/admin" -ForegroundColor Cyan
Write-Host ""
Write-Host "   部署目录:  $WORKDIR" -ForegroundColor Cyan
Write-Host "   配置文件:  $WORKDIR\config.yaml" -ForegroundColor Cyan
Write-Host ""
Write-Info "升级新版本时，重新运行本脚本即可:"
Write-Host "  irm https://github.com/$REPO/releases/latest/download/install.ps1 | iex"
Write-Host "========================================"
