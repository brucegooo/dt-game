#!/usr/bin/env bash

set -euo pipefail

# 环境变量（仅在未设置时给出本地合理默认）
: "${ETCD_ENDPOINTS:=127.0.0.1:2379}"
if [[ -z "${ETCD_CONFIG_KEY:-}" ]]; then
  echo "❌ 未设置 ETCD_CONFIG_KEY，例如：/dt-server/config/dev"
  echo "   例如本地开发可临时： export ETCD_CONFIG_KEY=/dt-server/config/dev"
  exit 1
fi
# RocketMQ 客户端日志目录（避免默认写 /logs 导致只读文件系统报错）
export MQ_CLIENT_LOG_DIR=${MQ_CLIENT_LOG_DIR:-./logs}
export ROCKETMQ_CLIENT_LOG_ROOT=${ROCKETMQ_CLIENT_LOG_ROOT:-./logs}
export ROCKETMQ_CLIENT_LOG_PATH=${ROCKETMQ_CLIENT_LOG_PATH:-./logs}
# 尝试将日志输出到控制台（若客户端版本支持）
export MQ_CONSOLE_APPENDER_ENABLED=${MQ_CONSOLE_APPENDER_ENABLED:-true}
export ROCKETMQ_CONSOLE_APPENDER_ENABLED=${ROCKETMQ_CONSOLE_APPENDER_ENABLED:-true}
# 创建本地日志目录（若上述变量生效时使用）
mkdir -p "$MQ_CLIENT_LOG_DIR" "$ROCKETMQ_CLIENT_LOG_ROOT" "$ROCKETMQ_CLIENT_LOG_PATH" 2>/dev/null || true


# 项目名
APP_NAME="dt-server"
BUILD_DIR="./build"
OS=$(go env GOOS)
ARCH=$(go env GOARCH)
ext=""; if [[ "$OS" == "windows" ]]; then ext=".exe"; fi
BIN_FILE="${BUILD_DIR}/${APP_NAME}_${OS}_${ARCH}${ext}"

# 如果没有可执行文件则编译
if [ ! -f "$BIN_FILE" ]; then
  echo "⚙️ 未检测到编译产物，正在执行构建..."
  ./build.sh
fi

echo "🚀 启动 ${APP_NAME} 服务..."
echo "🌐 ETCD_ENDPOINTS=$ETCD_ENDPOINTS"
echo "🗝️  ETCD_CONFIG_KEY=$ETCD_CONFIG_KEY"
echo

exec "$BIN_FILE"