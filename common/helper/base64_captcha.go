package helper

import (
	"image/color"
	"strings"

	"github.com/mojocn/base64Captcha"
)

var store = base64Captcha.DefaultMemStore

// 自定义字符源（去除易混淆字符）
const captchaChars = "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"

// CreateCaptcha 生成验证码（长度4，字母+数字）
func CreateCaptcha() (id, b64s string, err error) {
	driver := base64Captcha.NewDriverString(
		80,
		300, // 宽度增大一点，避免字符拥挤
		4,
		0,
		4,
		captchaChars,
		&color.RGBA{255, 255, 255, 255},
		nil,
		nil,
	)

	captcha := base64Captcha.NewCaptcha(driver, store)
	// var content string
	// id, b64s, content, err = captcha.Generate()
	// fmt.Println("验证码内容:", content, "长度:", len(content))
	id, b64s, _, err = captcha.Generate()
	return
}

// VerifyCaptcha 校验验证码（忽略大小写 + 自动清除）
func VerifyCaptcha(id, val string) bool {
	val = strings.TrimSpace(strings.ToLower(val))
	return store.Verify(id, val, true)
}
