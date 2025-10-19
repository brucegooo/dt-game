package state

import "fmt"

// State 游戏状态
const (
	StateInit     = "init"     // 初始化/未开始
	StateBetting  = "betting"  // 下注中(game_start~game_stop)
	StateSealed   = "sealed"   // 已封盘(game_stop)
	StateDealt    = "dealt"    // 已发牌(new_card)
	StateDrawn    = "drawn"    // 已开奖(game_draw)
	StateSettled  = "settled"  // 已结算(drawresult)
	StateFinished = "finished" // 已结束(game_end)
)

// Event 游戏事件
const (
	EvtGameStart = "game_start"
	EvtGameStop  = "game_stop"
	EvtNewCard   = "new_card"
	EvtGameDraw  = "game_draw"
	EvtGameEnd   = "game_end"
)

// NextState 根据当前状态与事件计算下一个状态，非法转换报错
func NextState(cur, evt string) (string, error) {
	switch cur {
	case StateInit:
		if evt == EvtGameStart {
			return StateBetting, nil
		}
	case StateBetting:
		if evt == EvtGameStop {
			return StateSealed, nil
		}
	case StateSealed:
		if evt == EvtNewCard {
			return StateDealt, nil
		}
	case StateDealt:
		if evt == EvtGameDraw {
			return StateDrawn, nil
		}
	case StateDrawn:
		if evt == EvtGameEnd {
			return StateFinished, nil
		}
	case StateSettled:
		if evt == EvtGameEnd {
			return StateFinished, nil
		}
	}
	return cur, fmt.Errorf("invalid transition: %s --%s--> ?", cur, evt)
}
