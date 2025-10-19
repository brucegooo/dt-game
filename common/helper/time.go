package helper

import (
	"fmt"
	"log"
	"strings"
	"time"
)

func TimeStringToStamp(startTime, endTime string) (sTime, eTime string) {

	//日期转化为时间戳
	timeLayout := "2006-01-02 15:04:05"  //转化所需模板
	loc, _ := time.LoadLocation("Local") //获取时区
	start, _ := time.ParseInLocation(timeLayout, startTime, loc)
	end, _ := time.ParseInLocation(timeLayout, endTime, loc)

	return fmt.Sprint(start.Unix()), fmt.Sprint(end.Unix())
}

func StrToTime(value string) time.Time {

	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05 -0700 MST",
		"2006/01/02 15:04:05 -0700",
		"2006/01/02 15:04:05",
		"2006-01-02 -0700 MST",
		"2006-01-02 -0700",
		"2006-01-02",
		"2006/01/02 -0700 MST",
		"2006/01/02 -0700",
		"2006/01/02",
		"2006-01-02 15:04:05 -0700 -0700",
		"2006/01/02 15:04:05 -0700 -0700",
		"2006-01-02 -0700 -0700",
		"2006/01/02 -0700 -0700",
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC3339Nano,
		time.Kitchen,
		time.Stamp,
		time.StampMilli,
		time.StampMicro,
		time.StampNano,
	}

	var t time.Time
	var err error
	loc, _ := time.LoadLocation("Asia/Shanghai")

	for _, layout := range layouts {
		t, err = time.ParseInLocation(layout, value, loc)
		if err == nil {
			return t
		}
	}
	return t
}

// Unix 时间戳转为日期格式
func TimeUnixToStr(t int64) string {

	return time.Unix(t, 0).Format("2006-01-02 15:04:05")
}

func TimeToStrByLayout(t int64, layout string) string {

	return time.Unix(t, 0).Format(layout)
}

// 今日0点
func GetTodayZeroTimestamp() int64 {
	now := time.Now()
	zero := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return zero.Unix()
}

// ParseTimeRange returns (startTime, endTime) in Unix seconds.

func ParseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	layout := "2006-01-02 15:04:05"
	if len(value) == 10 { // 只有日期
		value += " 00:00:00"
	}
	t, err := time.ParseInLocation(layout, value, time.Local)
	if err != nil {
		log.Printf("[WARN] time parse failed: %s, err: %v", value, err)
		return time.Time{}
	}
	return t
}

// ParseTimeRange 统一时间范围解析
func ParseTimeRange(startStr, endStr string) (int64, int64) {
	now := time.Now()
	var startTime, endTime time.Time

	if startStr != "" {
		startTime = ParseTime(startStr)
	} else {
		startTime = now.Add(-72 * time.Hour) // 默认 3 天前
	}

	if endStr != "" {
		endTime = ParseTime(endStr)
		// 若只传日期（10位），自动补 23:59:59
		if len(strings.TrimSpace(endStr)) == 10 {
			endTime = endTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	} else {
		endTime = now // 默认当前时间
	}

	return startTime.Unix(), endTime.Unix()
}

// FormatTimestampToYMDHMS 将秒级时间戳格式化为 yyyy-MM-dd HH:mm:ss
func FormatTimestampToYMDHMS(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

// FormatUnixStringToYMDHMS 将字符串形式的时间戳（秒或毫秒）转为 yyyy-MM-dd HH:mm:ss
func FormatUnixStringToYMDHMS(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 尝试解析为整数
	if len(s) >= 13 { // 可能是毫秒
		// 尝试取前10位作为秒
		return s[:10]
	}
	return s
}
