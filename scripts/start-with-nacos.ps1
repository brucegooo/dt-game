# 使用 Nacos 配置中心启动应用的脚本（Windows PowerShell）
# 用法：.\scripts\start-with-nacos.ps1

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  使用 Nacos 配置中心启动应用" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# 检查 Nacos 服务器是否可用
$nacosServer = if ($env:NACOS_SERVER_ADDR) { $env:NACOS_SERVER_ADDR } else { "127.0.0.1:8848" }
Write-Host "🔍 检查 Nacos 服务器: $nacosServer" -ForegroundColor Yellow

try {
    $response = Invoke-WebRequest -Uri "http://$nacosServer/nacos/" -TimeoutSec 5 -ErrorAction Stop
    Write-Host "✅ Nacos 服务器可用" -ForegroundColor Green
} catch {
    Write-Host "❌ Nacos 服务器不可用: $nacosServer" -ForegroundColor Red
    Write-Host ""
    Write-Host "请先启动 Nacos Server：" -ForegroundColor Yellow
    Write-Host "  docker run -d --name nacos-server -e MODE=standalone -p 8848:8848 -p 9848:9848 nacos/nacos-server:latest" -ForegroundColor White
    Write-Host ""
    exit 1
}

Write-Host ""

# 设置默认环境变量
if (-not $env:NACOS_SERVER_ADDR) { $env:NACOS_SERVER_ADDR = "127.0.0.1:8848" }
if (-not $env:NACOS_DATA_ID) { $env:NACOS_DATA_ID = "dt-server.yaml" }
if (-not $env:NACOS_NAMESPACE) { $env:NACOS_NAMESPACE = "public" }
if (-not $env:NACOS_GROUP) { $env:NACOS_GROUP = "DEFAULT_GROUP" }
if (-not $env:CONFIG_FILE) { $env:CONFIG_FILE = "config/windows.json" }

Write-Host "📝 Nacos 配置：" -ForegroundColor Yellow
Write-Host "  服务器地址: $env:NACOS_SERVER_ADDR"
Write-Host "  Data ID: $env:NACOS_DATA_ID"
Write-Host "  命名空间: $env:NACOS_NAMESPACE"
Write-Host "  配置分组: $env:NACOS_GROUP"
Write-Host "  兜底配置: $env:CONFIG_FILE"
Write-Host ""

# 检查配置是否存在
Write-Host "🔍 检查 Nacos 配置是否存在..." -ForegroundColor Yellow
$configUrl = "http://$env:NACOS_SERVER_ADDR/nacos/v1/cs/configs?dataId=$env:NACOS_DATA_ID&group=$env:NACOS_GROUP&tenant=$env:NACOS_NAMESPACE"

try {
    $configResponse = Invoke-WebRequest -Uri $configUrl -TimeoutSec 5 -ErrorAction Stop
    if ($configResponse.Content -match "content") {
        Write-Host "✅ Nacos 配置存在" -ForegroundColor Green
    } else {
        Write-Host "⚠️  Nacos 配置不存在，将使用本地文件作为兜底" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "请在 Nacos 控制台中创建配置：" -ForegroundColor Yellow
        Write-Host "  1. 访问: http://$env:NACOS_SERVER_ADDR/nacos"
        Write-Host "  2. 登录: nacos / nacos"
        Write-Host "  3. 创建配置:"
        Write-Host "     - Data ID: $env:NACOS_DATA_ID"
        Write-Host "     - Group: $env:NACOS_GROUP"
        Write-Host "     - 配置内容: 参考 config/nacos-example.yaml"
        Write-Host ""
    }
} catch {
    Write-Host "⚠️  无法检查 Nacos 配置，将使用本地文件作为兜底" -ForegroundColor Yellow
    Write-Host ""
}

Write-Host ""
Write-Host "🚀 启动应用..." -ForegroundColor Green
Write-Host ""

# 启动应用
.\dt-server.exe

