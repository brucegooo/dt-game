package helper

import (
	"regexp"
	"strings"
)

// 手机号格式校验函数，它用于验证中国大陆的 11 位手机号是否合法
func ValidateMobile(mobile string) bool {
	re := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return re.MatchString(mobile)
}

// 手机号码脱敏
func MaskPhone(mobile string) string {
	if len(mobile) != 11 {
		return "Xxxx"
	}

	return mobile[:3] + "****" + mobile[7:]
}

func MaskName(name string) string {
	if len(name) == 0 {
		return ""
	}
	runes := []rune(name)
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}
