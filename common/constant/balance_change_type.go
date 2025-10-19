package constant

// 账变类型常量定义
const ()

// 账变类型描述映射
var BalanceChangeTypeDesc = map[int]string{}

// GetBalanceChangeTypeDesc 获取账变类型描述
func GetBalanceChangeTypeDesc(changeType int) string {
	if desc, exists := BalanceChangeTypeDesc[changeType]; exists {
		return desc
	}
	return "未知类型"
}

// IsValidBalanceChangeType 验证账变类型是否有效
func IsValidBalanceChangeType(changeType int) bool {
	_, exists := BalanceChangeTypeDesc[changeType]
	return exists
}

// 常用账变类型分组
var (
	// 收入类型
	IncomeTypes = []int{}

	// 支出类型
	ExpenseTypes = []int{}

	// 奖励类型
	RewardTypes = []int{}
)

// IsIncomeType 判断是否为收入类型
func IsIncomeType(changeType int) bool {
	for _, t := range IncomeTypes {
		if t == changeType {
			return true
		}
	}
	return false
}

// IsExpenseType 判断是否为支出类型
func IsExpenseType(changeType int) bool {
	for _, t := range ExpenseTypes {
		if t == changeType {
			return true
		}
	}
	return false
}

// IsRewardType 判断是否为奖励类型
func IsRewardType(changeType int) bool {
	for _, t := range RewardTypes {
		if t == changeType {
			return true
		}
	}
	return false
}
