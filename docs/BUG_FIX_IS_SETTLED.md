# Bug 修复：game_end 事件误报"尚未结算"警告

## 🐛 问题描述

### 现象
在调用 `/api/game_event` (event_type=5, game_end) 时，即使已经调用过 `/api/drawresult` 完成结算，仍然会打印警告：

```
[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算, round_id=round_1760905015873
```

### 日志分析
```
[GameEvent]  当前状态: state=settled(6), round_id=round_1760905015873
[GameEvent]  game_end: 游戏结束, round_id=round_1760905015873
[GameEvent]  校验开奖结果和结算状态: round_id=round_1760905015873
[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算
```

**矛盾点**：
- 状态显示为 `settled(6)`（已结算状态）
- 但仍然提示"尚未结算"

---

## 🔍 根本原因

### 问题定位

在 `internal/service/game_event.go` 的 `game_end` 事件处理中：

```go
// 第 145 行
round, err := model.GetRoundForUpdate(ctx, tx, in.GameRoundID)

// 第 160-163 行
if round.IsSettled == 0 {
    fmt.Printf("[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算, round_id=%s, trace_id=%s\n",
        in.GameRoundID, in.TraceID)
}
```

### 问题根源

在 `internal/model/game_round_info.go` 的 `GetRoundForUpdate` 函数中：

```go
// 第 120-123 行（修复前）
sqlStr := `SELECT id, game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
    game_draw_time, card_list, game_result, game_result_str, game_status,
    trace_id, created_at, updated_at
    FROM game_round_info WHERE game_round_id = ? FOR UPDATE`
```

**问题**：SQL 查询中**没有包含 `is_settled` 字段**！

### 为什么会出现这个问题？

1. **结算流程正确**：
   - `/api/drawresult` 调用 `model.MarkAsSettled()`
   - `MarkAsSettled()` 正确更新了数据库：`is_settled = 1, game_status = 6`

2. **查询流程有缺陷**：
   - `game_end` 事件调用 `GetRoundForUpdate()` 获取回合信息
   - SQL 查询没有包含 `is_settled` 字段
   - Go 结构体的 `IsSettled` 字段使用默认值 0
   - 导致误判为"未结算"

---

## ✅ 修复方案

### 修改文件
`internal/model/game_round_info.go`

### 修改内容

**修复前**（第 120-123 行）：
```go
sqlStr := `SELECT id, game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
    game_draw_time, card_list, game_result, game_result_str, game_status,
    trace_id, created_at, updated_at
    FROM game_round_info WHERE game_round_id = ? FOR UPDATE`
```

**修复后**（第 120-123 行）：
```go
sqlStr := `SELECT id, game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
    game_draw_time, card_list, game_result, game_result_str, game_status, is_settled,
    trace_id, created_at, updated_at
    FROM game_round_info WHERE game_round_id = ? FOR UPDATE`
```

**变更**：在 `game_status` 后面添加了 `, is_settled`

---

## 🧪 测试验证

### 测试步骤

1. **编译代码**
   ```bash
   go build -o dt-server ./cmd/server
   ```
   ✅ 编译成功，无错误

2. **启动服务器**
   ```bash
   ./dt-server
   ```

3. **完整游戏流程测试**
   ```bash
   # 1. 开始游戏
   curl -X POST http://localhost:8087/api/game_event \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "event_type": 1,
       "game_round_id": "test_round_001",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   
   # 2. 投注
   curl -X POST http://localhost:8087/api/bet \
     -H "Content-Type: application/json" \
     -d '{
       "game_round_id": "test_round_001",
       "bet_amount": "100.00",
       "play_type": 1,
       "platform": 99,
       "idempotency_key": "test_bet_001"
     }'
   
   # 3. 封盘
   curl -X POST http://localhost:8087/api/game_event \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "event_type": 2,
       "game_round_id": "test_round_001",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   
   # 4. 发牌
   curl -X POST http://localhost:8087/api/game_event \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "event_type": 3,
       "game_round_id": "test_round_001",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   
   # 5. 准备开奖
   curl -X POST http://localhost:8087/api/game_event \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "event_type": 4,
       "game_round_id": "test_round_001",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   
   # 6. 开奖结算
   curl -X POST http://localhost:8087/api/drawresult \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "game_round_id": "test_round_001",
       "card_list": "D5,T3,RDragon",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   
   # 7. 结束游戏（修复后不应再出现警告）
   curl -X POST http://localhost:8087/api/game_event \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer admin_token_xxx" \
     -d '{
       "event_type": 5,
       "game_round_id": "test_round_001",
       "game_id": "dt_game",
       "room_id": "room_001"
     }'
   ```

### 预期结果

**修复前**：
```
[GameEvent]  当前状态: state=settled(6), round_id=test_round_001
[GameEvent]  game_end: 游戏结束, round_id=test_round_001
[GameEvent]  校验开奖结果和结算状态: round_id=test_round_001
[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算  ❌
```

**修复后**：
```
[GameEvent]  当前状态: state=settled(6), round_id=test_round_001
[GameEvent]  game_end: 游戏结束, round_id=test_round_001
[GameEvent]  校验开奖结果和结算状态: round_id=test_round_001
[GameEvent]  开奖结果校验通过: round_id=test_round_001, game_result=1(dragon), card_list=D5,T3,RDragon, is_settled=1  ✅
```

---

## 📊 影响范围

### 受影响的功能
- ✅ **game_end 事件处理**（主要影响）
- ✅ **投注接口**（使用 `GetRoundForUpdate` 校验时间窗口）

### 不受影响的功能
- ✅ **结算逻辑**（使用 `GetSettlementStatusForUpdate`，已包含 `is_settled`）
- ✅ **其他游戏事件**（不依赖 `is_settled` 字段）

---

## 🎯 相关代码

### 1. 结算逻辑（正确）
`internal/service/draw.go` 第 275 行：
```go
// 标记为已结算
if err := model.MarkAsSettled(ctx, tx, in.GameRoundID); err != nil {
    return err
}
```

`internal/model/game_round_info.go` 第 95-101 行：
```go
func MarkAsSettled(ctx context.Context, exec sqlx.ExtContext, roundID string) error {
    now := time.Now().UnixMilli()
    sqlStr := "UPDATE game_round_info SET is_settled = 1, game_status = 6, updated_at = ? WHERE game_round_id = ?"
    _, err := exec.ExecContext(ctx, sqlStr, now, roundID)
    return err
}
```

### 2. 结算状态查询（正确）
`internal/model/game_round_info.go` 第 78-93 行：
```go
func GetSettlementStatusForUpdate(ctx context.Context, exec sqlx.ExtContext, roundID string) (int8, int8, error) {
    sqlStr := "SELECT game_status, is_settled FROM game_round_info WHERE game_round_id = ? FOR UPDATE"
    
    type result struct {
        GameStatus int8 `db:"game_status"`
        IsSettled  int8 `db:"is_settled"`
    }
    
    var r result
    if err := sqlx.GetContext(ctx, exec, &r, sqlStr, roundID); err != nil {
        return 0, 0, err
    }
    return r.GameStatus, r.IsSettled, nil
}
```

### 3. game_end 事件校验（已修复）
`internal/service/game_event.go` 第 159-163 行：
```go
// 检查是否已经结算
if round.IsSettled == 0 {
    fmt.Printf("[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算, round_id=%s, trace_id=%s\n",
        in.GameRoundID, in.TraceID)
}
```

---

## 📝 总结

### 问题
`GetRoundForUpdate` 函数的 SQL 查询缺少 `is_settled` 字段，导致 `game_end` 事件误判结算状态。

### 修复
在 SQL 查询中添加 `is_settled` 字段。

### 影响
- ✅ 修复了 `game_end` 事件的误报警告
- ✅ 确保投注接口能正确读取结算状态
- ✅ 不影响现有结算逻辑

### 状态
- ✅ 代码已修复
- ✅ 编译成功
- ⚠️  待测试验证

---

**修复时间**: 2025-10-20  
**修复文件**: `internal/model/game_round_info.go`  
**修复行数**: 第 121 行  
**优先级**: HIGH  
**类型**: Bug Fix

