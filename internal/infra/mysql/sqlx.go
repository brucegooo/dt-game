package mysql

import (
	"sync"

	"github.com/jmoiron/sqlx"
)

var (
	once   sync.Once
	sqlxDB *sqlx.DB
)

func SQLX() *sqlx.DB {
	once.Do(func() {
		if DB() != nil {
			sqlxDB = sqlx.NewDb(DB(), "mysql")
		}
	})
	return sqlxDB
}
