# Dragon vs Tiger Server - 启动脚本（带 RocketMQ）
# 用途：启动 RocketMQ 服务并运行应用

Write-Host "🚀 启动 Dragon vs Tiger Server（带 RocketMQ）" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan

# 1. 检查 Docker 是否运行
Write-Host ""
Write-Host "🔍 检查 Docker 状态..." -ForegroundColor Yellow

try {
    docker info | Out-Null
    Write-Host "✅ Docker 已运行" -ForegroundColor Green
} catch {
    Write-Host "❌ Docker 未运行，请先启动 Docker Desktop" -ForegroundColor Red
    exit 1
}

# 2. 启动 RocketMQ NameServer
Write-Host ""
Write-Host "📡 启动 RocketMQ NameServer..." -ForegroundColor Yellow
docker-compose up -d rocketmq-namesrv

# 等待 NameServer 启动
Write-Host "⏳ 等待 NameServer 启动..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# 检查 NameServer 是否健康
$namesrvStatus = docker-compose ps rocketmq-namesrv
if ($namesrvStatus -match "Up") {
    Write-Host "✅ RocketMQ NameServer 已启动" -ForegroundColor Green
} else {
    Write-Host "❌ RocketMQ NameServer 启动失败" -ForegroundColor Red
    docker-compose logs rocketmq-namesrv
    exit 1
}

# 3. 启动 RocketMQ Broker
Write-Host ""
Write-Host "📡 启动 RocketMQ Broker..." -ForegroundColor Yellow
docker-compose up -d rocketmq-broker

# 等待 Broker 启动
Write-Host "⏳ 等待 Broker 启动..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# 检查 Broker 是否健康
$brokerStatus = docker-compose ps rocketmq-broker
if ($brokerStatus -match "Up") {
    Write-Host "✅ RocketMQ Broker 已启动" -ForegroundColor Green
} else {
    Write-Host "❌ RocketMQ Broker 启动失败" -ForegroundColor Red
    docker-compose logs rocketmq-broker
    exit 1
}

# 4. 验证 RocketMQ 端口
Write-Host ""
Write-Host "🔍 验证 RocketMQ 端口..." -ForegroundColor Yellow

try {
    $namesrvPort = Test-NetConnection -ComputerName localhost -Port 9876 -WarningAction SilentlyContinue
    if ($namesrvPort.TcpTestSucceeded) {
        Write-Host "✅ NameServer 端口 9876 可访问" -ForegroundColor Green
    } else {
        Write-Host "⚠️  NameServer 端口 9876 不可访问（可能需要等待）" -ForegroundColor Yellow
    }
} catch {
    Write-Host "⚠️  无法测试 NameServer 端口" -ForegroundColor Yellow
}

try {
    $brokerPort = Test-NetConnection -ComputerName localhost -Port 10911 -WarningAction SilentlyContinue
    if ($brokerPort.TcpTestSucceeded) {
        Write-Host "✅ Broker 端口 10911 可访问" -ForegroundColor Green
    } else {
        Write-Host "⚠️  Broker 端口 10911 不可访问（可能需要等待）" -ForegroundColor Yellow
    }
} catch {
    Write-Host "⚠️  无法测试 Broker 端口" -ForegroundColor Yellow
}

# 5. 显示 RocketMQ 状态
Write-Host ""
Write-Host "📊 RocketMQ 服务状态：" -ForegroundColor Cyan
docker-compose ps rocketmq-namesrv rocketmq-broker

# 6. 编译应用
Write-Host ""
Write-Host "🔨 编译应用..." -ForegroundColor Yellow

$buildResult = go build -o dt-server.exe ./cmd/server 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ 编译成功" -ForegroundColor Green
} else {
    Write-Host "❌ 编译失败" -ForegroundColor Red
    Write-Host $buildResult -ForegroundColor Red
    exit 1
}

# 7. 启动应用
Write-Host ""
Write-Host "🎮 启动 Dragon vs Tiger Server..." -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "📝 提示：" -ForegroundColor Yellow
Write-Host "  - 应用将使用配置文件：config/windows.json" -ForegroundColor White
Write-Host "  - RocketMQ NameServer: 127.0.0.1:9876" -ForegroundColor White
Write-Host "  - RocketMQ Topic: dt_settle" -ForegroundColor White
Write-Host "  - 调试页面: http://localhost:8087/debug" -ForegroundColor White
Write-Host ""
Write-Host "🔍 查看 RocketMQ 日志：" -ForegroundColor Yellow
Write-Host "  docker-compose logs -f rocketmq-namesrv" -ForegroundColor White
Write-Host "  docker-compose logs -f rocketmq-broker" -ForegroundColor White
Write-Host ""
Write-Host "🛑 停止 RocketMQ：" -ForegroundColor Yellow
Write-Host "  docker-compose stop rocketmq-namesrv rocketmq-broker" -ForegroundColor White
Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

# 启动应用
.\dt-server.exe

