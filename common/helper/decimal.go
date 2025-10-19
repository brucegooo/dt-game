package helper

import (
	"github.com/shopspring/decimal"
)

var (
	OneDecimal = decimal.NewFromInt(1)
)

/*
* @Description: decimal对像四舍五入到2位小数
* @Author: awen
* @Date: 2021/10/17 10:08
* @LastEditTime: 2025/08/28 16:00
* @LastEditors: bruce
* @Fixed: 修复截断BUG，改为四舍五入
 */
func TrimDecimal(val decimal.Decimal) string {
	// 直接使用 StringFixed(2) 进行四舍五入到2位小数
	// 这样可以避免截断导致的精度丢失问题
	return val.StringFixed(2)
}
