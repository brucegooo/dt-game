package common

import (
	"time"
)

// 获取某天的0点0分0秒的时间戳
func GetDataTimeUnix(input time.Time) int64 {
	location, _ := time.LoadLocation("Asia/Shanghai")

	year, month, day := input.Date()
	midnightToday := time.Date(year, month, day, 0, 0, 0, 0, location)
	midnightTomorrow := midnightToday.Unix()

	return midnightTomorrow
}

// 获取当天 00:00:00 和 第二天 00:00:00
func GetTodayRange(t time.Time) (start, end int64) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	year, month, day := t.In(loc).Date()

	startTime := time.Date(year, month, day, 0, 0, 0, 0, loc)
	endTime := startTime.AddDate(0, 0, 1) // +1 天

	return startTime.Unix(), endTime.Unix()
}

// 获取当周周一 00:00:00 和 周日 00:00:00
func GetWeekRange(t time.Time) (start, end int64) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t = t.In(loc)

	// 获取当前是周几（周日是0，周一是1 ... 周六是6）
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // 让周日变成 7，方便计算
	}

	// 计算周一
	year, month, day := t.Date()
	monday := time.Date(year, month, day, 0, 0, 0, 0, loc).AddDate(0, 0, -(weekday - 1))
	// 周日 = 周一 + 7天
	sunday := monday.AddDate(0, 0, 7)

	return monday.Unix(), sunday.Unix()
}

// 获取当月第一天 00:00:00 和 下个月第一天 00:00:00
func GetMonthRange(t time.Time) (start, end int64) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t = t.In(loc)

	year, month, _ := t.Date()
	// 月初
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	// 下个月月初
	nextMonth := firstDay.AddDate(0, 1, 0)

	return firstDay.Unix(), nextMonth.Unix()
}
