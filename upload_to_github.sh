#!/bin/bash

echo "=========================================="
echo "上传项目到 GitHub"
echo "=========================================="
echo ""

# 检查是否已经初始化 git
if [ ! -d ".git" ]; then
    echo "1. 初始化 Git 仓库..."
    git init
    echo "✅ Git 仓库初始化完成"
else
    echo "1. Git 仓库已存在"
fi
echo ""

# 添加所有文件
echo "2. 添加所有文件..."
git add .
echo "✅ 文件添加完成"
echo ""

# 创建提交
echo "3. 创建提交..."
git commit -m "Initial commit: Dragon vs Tiger game server

Features:
- Core game APIs: /api/game_event, /api/bet, /api/drawresult
- Multi-platform authentication (B2B2C model)
- Settlement idempotency protection (3-layer)
- Production security fixes (amount validation, result validation, distributed lock)
- Log optimization (51% reduction)
- Comprehensive documentation
- MySQL 8.0 + Redis 7 + RocketMQ support"

if [ $? -eq 0 ]; then
    echo "✅ 提交创建完成"
else
    echo "⚠️  提交可能已存在或没有变更"
fi
echo ""

# 设置主分支
echo "4. 设置主分支为 main..."
git branch -M main
echo "✅ 主分支设置完成"
echo ""

# 添加远程仓库
echo "5. 添加远程仓库..."
git remote add origin https://github.com/brucegooo/dt-game.git 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ 远程仓库添加完成"
else
    echo "⚠️  远程仓库可能已存在"
    git remote set-url origin https://github.com/brucegooo/dt-game.git
    echo "✅ 远程仓库 URL 已更新"
fi
echo ""

# 推送到 GitHub
echo "6. 推送到 GitHub..."
echo "   (这可能需要您输入 GitHub 用户名和密码/token)"
echo ""
git push -u origin main

if [ $? -eq 0 ]; then
    echo ""
    echo "=========================================="
    echo "✅ 上传成功！"
    echo "=========================================="
    echo ""
    echo "您的项目已成功上传到："
    echo "https://github.com/brucegooo/dt-game"
    echo ""
else
    echo ""
    echo "=========================================="
    echo "❌ 上传失败"
    echo "=========================================="
    echo ""
    echo "可能的原因："
    echo "1. 需要 GitHub 认证（用户名和 token）"
    echo "2. 远程仓库不存在或无权限"
    echo "3. 网络连接问题"
    echo ""
    echo "建议："
    echo "1. 确保您已在 GitHub 上创建了 dt-game 仓库"
    echo "2. 使用 Personal Access Token 代替密码"
    echo "3. 或者使用 SSH 方式：git remote set-url origin git@github.com:brucegooo/dt-game.git"
    echo ""
fi

