package mysql

import "database/sql"

// UseDB: 注入外部初始化好的 *sql.DB（例如 common.InitDB 返回的句柄）
func UseDB(d *sql.DB) {
	if d == nil {
		return
	}
	db = d
}

// 全局 *sql.DB 句柄（由 UseDB 注入）
var db *sql.DB

// DB 返回全局 *sql.DB 句柄
func DB() *sql.DB { return db }
