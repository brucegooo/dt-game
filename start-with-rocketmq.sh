#!/bin/bash

# Dragon vs Tiger Server - 启动脚本（带 RocketMQ）
# 用途：启动 RocketMQ 服务并运行应用

set -e

echo "🚀 启动 Dragon vs Tiger Server（带 RocketMQ）"
echo "================================================"

# 1. 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker 未运行，请先启动 Docker"
    exit 1
fi

echo "✅ Docker 已运行"

# 2. 启动 RocketMQ NameServer
echo ""
echo "📡 启动 RocketMQ NameServer..."
docker-compose up -d rocketmq-namesrv

# 等待 NameServer 启动
echo "⏳ 等待 NameServer 启动..."
sleep 5

# 检查 NameServer 是否健康
if docker-compose ps rocketmq-namesrv | grep -q "Up"; then
    echo "✅ RocketMQ NameServer 已启动"
else
    echo "❌ RocketMQ NameServer 启动失败"
    docker-compose logs rocketmq-namesrv
    exit 1
fi

# 3. 启动 RocketMQ Broker
echo ""
echo "📡 启动 RocketMQ Broker..."
docker-compose up -d rocketmq-broker

# 等待 Broker 启动
echo "⏳ 等待 Broker 启动..."
sleep 10

# 检查 Broker 是否健康
if docker-compose ps rocketmq-broker | grep -q "Up"; then
    echo "✅ RocketMQ Broker 已启动"
else
    echo "❌ RocketMQ Broker 启动失败"
    docker-compose logs rocketmq-broker
    exit 1
fi

# 4. 验证 RocketMQ 端口
echo ""
echo "🔍 验证 RocketMQ 端口..."

if nc -z localhost 9876 2>/dev/null; then
    echo "✅ NameServer 端口 9876 可访问"
else
    echo "⚠️  NameServer 端口 9876 不可访问（可能需要等待）"
fi

if nc -z localhost 10911 2>/dev/null; then
    echo "✅ Broker 端口 10911 可访问"
else
    echo "⚠️  Broker 端口 10911 不可访问（可能需要等待）"
fi

# 5. 显示 RocketMQ 状态
echo ""
echo "📊 RocketMQ 服务状态："
docker-compose ps rocketmq-namesrv rocketmq-broker

# 6. 编译应用
echo ""
echo "🔨 编译应用..."
if go build -o dt-server ./cmd/server; then
    echo "✅ 编译成功"
else
    echo "❌ 编译失败"
    exit 1
fi

# 7. 启动应用
echo ""
echo "🎮 启动 Dragon vs Tiger Server..."
echo "================================================"
echo ""
echo "📝 提示："
echo "  - 应用将使用配置文件：config/dev.json 或 config/windows.json"
echo "  - RocketMQ NameServer: 127.0.0.1:9876"
echo "  - RocketMQ Topic: dt_settle"
echo "  - 调试页面: http://localhost:8087/debug"
echo ""
echo "🔍 查看 RocketMQ 日志："
echo "  docker-compose logs -f rocketmq-namesrv"
echo "  docker-compose logs -f rocketmq-broker"
echo ""
echo "🛑 停止 RocketMQ："
echo "  docker-compose stop rocketmq-namesrv rocketmq-broker"
echo ""
echo "================================================"
echo ""

# 启动应用
./dt-server

