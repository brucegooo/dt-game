#!/bin/bash

# 使用 Nacos 配置中心启动应用的脚本
# 用法：./scripts/start-with-nacos.sh

set -e

echo "=========================================="
echo "  使用 Nacos 配置中心启动应用"
echo "=========================================="
echo ""

# 检查 Nacos 服务器是否可用
NACOS_SERVER="${NACOS_SERVER_ADDR:-127.0.0.1:8848}"
echo "🔍 检查 Nacos 服务器: $NACOS_SERVER"

if curl -s "http://$NACOS_SERVER/nacos/" > /dev/null 2>&1; then
    echo "✅ Nacos 服务器可用"
else
    echo "❌ Nacos 服务器不可用: $NACOS_SERVER"
    echo ""
    echo "请先启动 Nacos Server："
    echo "  docker run -d --name nacos-server -e MODE=standalone -p 8848:8848 -p 9848:9848 nacos/nacos-server:latest"
    echo ""
    exit 1
fi

echo ""

# 设置默认环境变量
export NACOS_SERVER_ADDR="${NACOS_SERVER_ADDR:-127.0.0.1:8848}"
export NACOS_DATA_ID="${NACOS_DATA_ID:-dt-server.yaml}"
export NACOS_NAMESPACE="${NACOS_NAMESPACE:-public}"
export NACOS_GROUP="${NACOS_GROUP:-DEFAULT_GROUP}"
export CONFIG_FILE="${CONFIG_FILE:-config/dev.json}"

echo "📝 Nacos 配置："
echo "  服务器地址: $NACOS_SERVER_ADDR"
echo "  Data ID: $NACOS_DATA_ID"
echo "  命名空间: $NACOS_NAMESPACE"
echo "  配置分组: $NACOS_GROUP"
echo "  兜底配置: $CONFIG_FILE"
echo ""

# 检查配置是否存在
echo "🔍 检查 Nacos 配置是否存在..."
if curl -s "http://$NACOS_SERVER/nacos/v1/cs/configs?dataId=$NACOS_DATA_ID&group=$NACOS_GROUP&tenant=$NACOS_NAMESPACE" | grep -q "content"; then
    echo "✅ Nacos 配置存在"
else
    echo "⚠️  Nacos 配置不存在，将使用本地文件作为兜底"
    echo ""
    echo "请在 Nacos 控制台中创建配置："
    echo "  1. 访问: http://$NACOS_SERVER/nacos"
    echo "  2. 登录: nacos / nacos"
    echo "  3. 创建配置:"
    echo "     - Data ID: $NACOS_DATA_ID"
    echo "     - Group: $NACOS_GROUP"
    echo "     - 配置内容: 参考 config/nacos-example.yaml"
    echo ""
fi

echo ""
echo "🚀 启动应用..."
echo ""

# 启动应用
./dt-server

