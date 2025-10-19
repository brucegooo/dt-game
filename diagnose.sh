#!/bin/bash

# 诊断脚本 - 检查系统状态和常见问题

echo "🔍 Dragon vs Tiger 系统诊断"
echo "=========================================="
echo ""

# 1. 检查 MySQL 版本
echo "1️⃣ MySQL 版本:"
mysql --version
echo ""

# 2. 检查数据库连接
echo "2️⃣ MySQL 连接测试:"
if mysql -u root -e "SELECT 1" &> /dev/null; then
    echo "✅ MySQL 连接成功"
else
    echo "❌ MySQL 连接失败"
fi
echo ""

# 3. 检查数据库是否存在
echo "3️⃣ 数据库检查:"
if mysql -u root -e "USE dt_game; SELECT 1;" &> /dev/null; then
    echo "✅ 数据库 dt_game 存在"
    
    # 检查表
    echo ""
    echo "📋 数据库表:"
    mysql -u root dt_game -e "SHOW TABLES;"
else
    echo "❌ 数据库 dt_game 不存在"
fi
echo ""

# 4. 检查 Redis
echo "4️⃣ Redis 检查:"
if redis-cli ping &> /dev/null; then
    echo "✅ Redis 运行正常"
else
    echo "❌ Redis 未运行"
fi
echo ""

# 5. 检查服务端口
echo "5️⃣ 端口检查:"
if lsof -Pi :8087 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "✅ 端口 8087 正在监听"
    PID=$(lsof -ti:8087)
    echo "   进程 PID: $PID"
else
    echo "❌ 端口 8087 未监听"
fi
echo ""

# 6. 检查服务健康
echo "6️⃣ 服务健康检查:"
if curl -s http://localhost:8087/healthz > /dev/null 2>&1; then
    RESPONSE=$(curl -s http://localhost:8087/healthz)
    echo "✅ 服务健康: $RESPONSE"
else
    echo "❌ 服务未响应"
fi
echo ""

# 7. 检查最近的游戏回合
echo "7️⃣ 最近的游戏回合:"
mysql -u root dt_game -e "SELECT game_round_id, game_status, bet_start_time, bet_stop_time, game_result_str FROM game_round_info ORDER BY created_at DESC LIMIT 5;" 2>&1
echo ""

# 8. 检查用户余额
echo "8️⃣ 测试用户余额:"
mysql -u root dt_game -e "SELECT user_id, username, amount, status FROM customers WHERE user_id=100001;" 2>&1
echo ""

# 9. 检查配置文件
echo "9️⃣ 配置文件检查:"
if [ -f "config/dev.json" ]; then
    echo "✅ config/dev.json 存在"
    echo ""
    echo "配置内容:"
    cat config/dev.json | grep -v "password" | grep -v "secret"
else
    echo "❌ config/dev.json 不存在"
fi
echo ""

# 10. 测试简单的 API 调用
echo "🔟 API 测试:"
ROUND_ID="diag_$(date +%s)"
echo "测试 Round ID: $ROUND_ID"
echo ""

echo "发送 game_start 事件..."
RESPONSE=$(curl -s -X POST http://localhost:8087/api/game_event \
  -H "Content-Type: application/json" \
  -d "{
    \"game_id\": \"game_001\",
    \"room_id\": \"room_001\",
    \"game_round_id\": \"$ROUND_ID\",
    \"event_type\": 1
  }")

if echo "$RESPONSE" | grep -q "ok"; then
    echo "✅ API 调用成功"
    echo "响应: $RESPONSE"
else
    echo "❌ API 调用失败"
    echo "响应: $RESPONSE"
fi
echo ""

echo "=========================================="
echo "✅ 诊断完成"

