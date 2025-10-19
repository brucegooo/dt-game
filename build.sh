#!/usr/bin/env bash
# --------------------------------------------------
# Go 项目构建脚本（根据当前平台自动编译）
# 支持：自动检测系统架构 / 版本信息注入 / 输出到 build 目录
# --------------------------------------------------

set -euo pipefail

APP_NAME="dt-server"
MAIN_PKG="./cmd/server"
OUTPUT_DIR="./build"

# 自动获取版本和构建时间
VERSION=$(git describe --tags --always 2>/dev/null || echo "v0.0.0")
BUILD_TIME=$(date +"%Y-%m-%d_%H:%M:%S")

# 检测当前平台
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
OUTPUT_NAME="${APP_NAME}_${GOOS}_${GOARCH}"

# Windows 下加 .exe 后缀
[[ "$GOOS" == "windows" ]] && OUTPUT_NAME="${OUTPUT_NAME}.exe"

echo "🚀 开始构建项目: $APP_NAME"
echo "🧩 平台: ${GOOS}/${GOARCH}"
echo "🏷️ 版本: ${VERSION}"
echo "🕒 时间: ${BUILD_TIME}"
echo

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 执行构建
CGO_ENABLED=${CGO_ENABLED:-0} GOOS=$GOOS GOARCH=$GOARCH \
  go build -trimpath -ldflags "-s -w" \
  -o "${OUTPUT_DIR}/${OUTPUT_NAME}" "$MAIN_PKG"

echo
echo "✅ 构建完成: ${OUTPUT_DIR}/${OUTPUT_NAME}"