#!/bin/bash

# Dragon vs Tiger 游戏服务器 - 本地启动脚本
# 不依赖 Docker 和 etcd，直接从本地配置文件读取

set -e

echo "🎮 Dragon vs Tiger 游戏服务器 - 本地启动"
echo "=========================================="
echo ""

# 检查 Go
echo "🔍 检查 Go..."
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go 1.24.3+"
    exit 1
fi
GO_VERSION=$(go version)
echo "✅ $GO_VERSION"
echo ""

# 检查 MySQL
echo "🔍 检查 MySQL..."
if ! mysql -u root -e "SELECT 1" &> /dev/null; then
    echo "❌ MySQL 连接失败，请确保 MySQL 正在运行"
    echo "   提示：可以运行 'mysql.server start' 启动 MySQL"
    exit 1
fi
echo "✅ MySQL 连接成功"
echo ""

# 检查数据库
echo "🔍 检查数据库..."
if ! mysql -u root -e "USE dt_game; SELECT 1;" &> /dev/null; then
    echo "⚠️  数据库 dt_game 不存在，正在初始化..."
    if [ -f "init.sql" ]; then
        mysql -u root < init.sql
        echo "✅ 数据库初始化完成"
    else
        echo "❌ 找不到 init.sql 文件"
        exit 1
    fi
else
    echo "✅ 数据库 dt_game 已存在"
fi
echo ""

# 检查 Redis
echo "🔍 检查 Redis..."
if ! redis-cli ping &> /dev/null; then
    echo "❌ Redis 连接失败，请确保 Redis 正在运行"
    echo "   提示：可以运行 'redis-server' 启动 Redis"
    exit 1
fi
echo "✅ Redis 连接成功"
echo ""

# 检查配置文件
echo "🔍 检查配置文件..."
if [ ! -f "config/dev.json" ]; then
    echo "❌ 配置文件 config/dev.json 不存在"
    exit 1
fi
echo "✅ 配置文件存在"
echo ""

# 检查端口占用
echo "🔍 检查端口 8087..."
if lsof -Pi :8087 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "⚠️  端口 8087 已被占用"
    read -p "是否结束占用进程？(y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        lsof -ti:8087 | xargs kill -9 2>/dev/null || true
        echo "✅ 已结束占用进程"
        sleep 1
    else
        echo "❌ 请手动结束占用进程后再启动"
        exit 1
    fi
else
    echo "✅ 端口 8087 可用"
fi
echo ""

# 启动服务
echo "🚀 启动服务..."
echo "配置文件: config/dev.json"
echo "服务地址: http://localhost:8087"
echo ""
echo "按 Ctrl+C 停止服务"
echo "=========================================="
echo ""

export CONFIG_FILE=config/dev.json
go run ./cmd/server

