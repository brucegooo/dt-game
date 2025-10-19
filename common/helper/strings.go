package helper

import (
	"database/sql"
	"dt-server/common"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// BuildFullURL 根据 host 和相对路径拼接完整 URL
// - 如果 path 为空，返回空字符串
// - 如果 path 已经是 http/https 开头，原样返回
// - 否则使用 host 和 path 进行拼接，避免重复斜杠
func BuildFullURL(host, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	p := strings.TrimSpace(path)
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	h := strings.TrimRight(strings.TrimSpace(host), "/")
	p = strings.TrimLeft(p, "/")
	if h == "" {
		return p
	}
	return h + "/" + p
}

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
}

func PrintError(ctx *fasthttp.RequestCtx, code int, error error, errs ...error) {

	if ctx == nil {
		return
	}

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")

	rspData := fmt.Sprintf("%s 错误ID: %d", error, ctx.ID())
	res := Response{
		Code: code,
		Data: rspData,
	}
	if len(errs) > 0 && errs[0] != nil {
		logrus.Error(ctx, errs[0].Error(), zap.ByteString("query", ctx.Request.RequestURI()), zap.ByteString("body", ctx.Request.Body()))
	}
	bytes, err := common.JsonMarshal(res)
	if err != nil {
		ctx.SetBodyString(err.Error())
		return
	}

	ctx.SetBody(bytes)
}

func Print(ctx *fasthttp.RequestCtx, code int, data interface{}, errors ...error) {

	if ctx == nil {
		return
	}

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")
	if code != 200 && len(errors) > 0 && errors[0] != nil {
		fmt.Println(errors[0].Error())
		data = ErrCodeMsg[code]
	}
	// 统一时间字段转换：将所有 created_at/updated_at 转成 yyyy-MM-dd HH:mm:ss
	processed := data
	if data != nil {
		if b, err2 := common.JsonMarshal(data); err2 == nil {
			var generic interface{}
			if err3 := common.JsonUnmarshal(b, &generic); err3 == nil {
				processed = transformTimeFieldsGeneric(generic)
			}
		}
	}

	res := Response{
		Code: code,
		Data: processed,
	}

	bytes, err := common.JsonMarshal(res)
	if err != nil {
		ctx.SetBodyString(err.Error())
		return
	}

	ctx.SetBody(bytes)
}

func PrintJson(ctx *fasthttp.RequestCtx, code int, data string, errors ...error) {

	if ctx == nil {
		return
	}

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")
	if len(errors) > 0 && errors[0] != nil {
		fmt.Errorf(errors[0].Error())
	}

	if code != 200 {
		if _, ok := ErrCodeMsg[code]; ok {
			data = ErrCodeMsg[code]
		} else {
			data = "系统错误"
		}
	}
	builder := strings.Builder{}

	builder.WriteString(`{"code":"`)
	builder.WriteByte(byte(code))
	builder.WriteString(`","data":`)
	builder.WriteString(data)
	builder.WriteString("}")

	ctx.SetBodyString(builder.String())
}

// 判断字符是否为数字
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// 判断字符是否为英文字符
func isAlpha(r rune) bool {

	if r >= 'A' && r <= 'Z' {
		return true
	} else if r >= 'a' && r <= 'z' {
		return true
	}
	return false
}

// 判断字符串是不是数字
func CtypeDigit(s string) bool {

	if s == "" {
		return false
	}
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
	}
	return true
}

// 判断字符串是不是字母+数字
func CtypeAlnum(s string) bool {

	if s == "" {
		return false
	}
	for _, r := range s {
		if !isDigit(r) && !isAlpha(r) {
			return false
		}
	}
	return true
}

func strReplace(str string, original []string, replacement []string) string {

	for i, replace := range original {
		r := strings.NewReplacer(replace, replacement[i])
		str = r.Replace(str)
	}

	return str
}

var regexJoinWhitespaces = regexp.MustCompile(`[　\s]+`)

func FilterInjection(s string) string {
	empty := []rune(" ")
	r := []rune(s) // 转成unicode
	for index, c := range r {
		if unicode.IsSpace(c) {
			r[index] = empty[0]
		}
	}
	s = string(r)
	s = strings.TrimSpace(regexJoinWhitespaces.ReplaceAllString(s, " "))
	original := []string{"<", ">", "\"", " ", "'", "\\", "\t", "\n", " "}
	replacement := []string{"&lt;", "&gt;", "&quot;", "&nbsp;", "&apos;", "", "&nbsp;", "<br/>", "&nbsp;"}

	return strReplace(s, original, replacement)
}

func IsEmptyString(str string) bool {

	s := strings.TrimSpace(str)
	if len(s) == 0 {
		return true
	}

	return false
}
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(input string, hashed string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(input))
	return err == nil
}

// transformTimeFieldsGeneric 将通用结构中的 created_at/updated_at 字段格式化为 yyyy-MM-dd HH:mm:ss
// 仅浅层遍历 map[string]any 和 []any，避免深层性能和行为变化
func transformTimeFieldsGeneric(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			lower := strings.ToLower(k)
			if lower == "created_at" || lower == "updated_at" || lower == "received_at" || lower == "applied_at" {
				// 针对时间字段做转换
				switch t := v2.(type) {
				case float64:
					sec := int64(t)
					// 判断是否为毫秒
					if sec >= 1000000000000 { // >= 1e12 视作毫秒
						sec = sec / 1000
					}
					m[k] = TimeUnixToStr(sec)
				case int64:
					m[k] = TimeUnixToStr(t)
				case int:
					m[k] = TimeUnixToStr(int64(t))
				case string:
					st := strings.TrimSpace(t)
					// 纯数字则按时间戳处理
					numeric := true
					for i := 0; i < len(st); i++ {
						c := st[i]
						if c < '0' || c > '9' {
							numeric = false
							break
						}
					}
					if numeric && len(st) >= 10 {
						m[k] = TimeUnixToStr(parseUnixLike(st))
					} else {
						m[k] = t
					}
				default:
					m[k] = v2
				}
			} else {
				// 其他字段递归处理，确保深层也会转换
				m[k] = transformTimeFieldsGeneric(v2)
			}
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, it := range val {
			arr[i] = transformTimeFieldsGeneric(it)
		}
		return arr
	default:
		return v
	}
}

// parseUnixLike 尝试从字符串解析秒级时间戳（支持毫秒长度，取前10位）
func parseUnixLike(s string) int64 {
	st := strings.TrimSpace(s)
	if len(st) >= 13 {
		st = st[:10]
	}
	var sec int64
	for i := 0; i < len(st); i++ {
		c := st[i]
		if c < '0' || c > '9' {
			return 0
		}
	}
	// 简单的十进制解析
	for i := 0; i < len(st); i++ {
		sec = sec*10 + int64(st[i]-'0')
	}
	return sec
}

func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// GenerateSerialNumber 生成用户订单号
func GenerateSerialNumber(userID int) string {
	return fmt.Sprintf("%d%d%s", time.Now().Unix(), userID, TimeToStrByLayout(time.Now().Unix(), "20060102150405"))
}
