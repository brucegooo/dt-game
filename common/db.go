package common

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	g "github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

var (
	dialect = g.Dialect("mysql")
)

type Conn struct {
	Db *sqlx.DB
	Tx *sqlx.Tx
}

type QueryArg struct {
	Db      *sqlx.DB                // db connection
	Table   string                  // table
	Fields  []interface{}           // query fields
	Ex      []exp.Expression        // where conditions
	Order   []exp.OrderedExpression // order conditions
	GroupBy []interface{}           // group by fields
	Offset  uint                    // offset
	Limit   uint                    // limit
}

func EnumFields(obj interface{}) []interface{} {

	rt := reflect.TypeOf(obj)
	if rt.Kind() != reflect.Struct {
		return nil
	}

	var fields []interface{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if field := f.Tag.Get("db"); field != "" && field != "-" {
			fields = append(fields, field)
		}
	}

	return fields
}

func Insert(db Conn, table string, rows ...interface{}) (sql.Result, error) {

	if db.Db == nil && db.Tx == nil {
		return nil, fmt.Errorf("no conn")
	}
	query, _, _ := dialect.Insert(table).Rows(rows...).ToSQL()

	var (
		res sql.Result
		err error
	)

	if db.Tx != nil {
		res, err = db.Tx.Exec(query)
	} else {
		res, err = db.Db.Exec(query)
	}
	if err != nil {
		Printf("insert into %s err: %s\n", table, err.Error())
	}

	return res, err
}

func Update(db Conn, table string, record g.Record, ex ...g.Expression) (sql.Result, error) {

	if db.Db == nil && db.Tx == nil {
		return nil, fmt.Errorf("no conn")
	}
	query, _, _ := dialect.Update(table).Set(record).Where(ex...).ToSQL()

	var (
		res sql.Result
		err error
	)

	if db.Tx != nil {
		res, err = db.Tx.Exec(query)
	} else {
		res, err = db.Db.Exec(query)
	}
	if err != nil {
		Printf("update %s err: %s\n", table, err.Error())
	}

	return res, err
}

func Delete(db Conn, table string, ex ...exp.Expression) (sql.Result, error) {

	if db.Db == nil && db.Tx == nil {
		return nil, fmt.Errorf("no conn")
	}
	query, _, _ := dialect.Delete(table).Where(ex...).ToSQL()

	var (
		res sql.Result
		err error
	)

	if db.Tx != nil {
		res, err = db.Tx.Exec(query)
	} else {
		res, err = db.Db.Exec(query)
	}
	if err != nil {
		Printf("delete from %s err: %s\n", table, err.Error())
	}

	return res, err
}

func SelectOne(data interface{}, db *sqlx.DB, table string, fields []interface{}, ex ...exp.Expression) error {

	query, _, _ := dialect.Select(fields...).From(table).Where(ex...).Limit(1).ToSQL()
	err := db.Get(data, query)
	if err != nil {
		Printf("get %s err: %s\n", table, err.Error())
	}

	return err
}

// SelectOneTx 在事务中查询一条记录，可选加锁
func SelectOneTx(tx *sqlx.Tx, data interface{}, table string, fields []interface{}, ex exp.Expression, forUpdate bool) error {
	query, _, _ := dialect.Select(fields...).From(table).Where(ex).Limit(1).ToSQL()

	if forUpdate {
		query += " FOR UPDATE"
	}

	err := tx.Get(data, query)
	if err != nil {
		Printf("get %s err: %s\n", table, err.Error())
		return err
	}
	return nil
}

func SelectOneConn(data interface{}, db Conn, table string, fields []interface{}, ex ...exp.Expression) error {
	if db.Db == nil && db.Tx == nil {
		return fmt.Errorf("no conn")
	}
	query, args, _ := dialect.Select(fields...).From(table).Where(ex...).Limit(1).ToSQL()
	if db.Tx != nil {
		return db.Tx.Get(data, query, args...)
	}
	return db.Db.Get(data, query, args...)
}

func SelectAll(data interface{}, args QueryArg) error {

	if args.Db == nil {
		return fmt.Errorf("invalid db")
	}
	if args.Table == "" {
		return fmt.Errorf("invalid table")
	}
	if len(args.Fields) == 0 {
		return fmt.Errorf("invalid fields")
	}

	ds := dialect.Select(args.Fields...).From(args.Table)

	if len(args.Ex) > 0 {
		ds = ds.Where(args.Ex...)
	}

	if len(args.GroupBy) > 0 {
		ds = ds.GroupBy(args.GroupBy...)
	}

	if len(args.Order) > 0 {
		ds = ds.Order(args.Order...)
	}

	if args.Offset > 0 {
		ds = ds.Offset(args.Offset)
	}

	if args.Limit > 0 {
		ds = ds.Limit(args.Limit)
	}

	query, _, _ := ds.ToSQL()
	err := args.Db.Select(data, query)
	if err != nil {
		Printf("select %s err: %s\n", args.Table, err.Error())
	}

	return err
}

func Count(db *sqlx.DB, table string, ex ...exp.Expression) (int, error) {

	var count int
	query, _, _ := dialect.Select(g.COUNT("*")).From(table).Where(ex...).ToSQL()
	err := db.Get(&count, query)
	if err != nil {
		Printf("count %s err: %s\n", table, err.Error())
	}

	return count, err
}

func CountInfo(db *sqlx.DB, table, name string, ex ...exp.Expression) (int64, error) {

	var count int64
	query, _, _ := dialect.Select(g.COUNT(name)).From(table).Where(ex...).ToSQL()
	err := db.Get(&count, query)
	if err != nil {
		Printf("count %s err: %s\n", table, err.Error())
	}

	return count, err
}

func SumInfo(db *sqlx.DB, table, name string, ex ...exp.Expression) (float64, error) {

	var sum float64
	query, _, _ := dialect.Select(g.COALESCE(g.SUM(name), 0)).From(table).Where(ex...).ToSQL()
	err := db.Get(&sum, query)
	if err != nil {
		Printf("count %s err: %s\n", table, err.Error())
	}

	return sum, err
}

// ---- 带 Context/args 的增强版通用方法（不破坏兼容，按需逐步替换调用）----

// InsertCtx：在 sqlx.ExtContext 上执行 INSERT，保持 goqu 生成的占位符与 args
func InsertCtx(ctx context.Context, exec sqlx.ExtContext, table string, rows ...interface{}) (sql.Result, error) {
	query, args, err := dialect.Insert(table).Rows(rows...).ToSQL()
	if err != nil {
		return nil, err
	}
	return exec.ExecContext(ctx, query, args...)
}

// UpdateCtx：在 sqlx.ExtContext 上执行 UPDATE
func UpdateCtx(ctx context.Context, exec sqlx.ExtContext, table string, record g.Record, ex ...g.Expression) (sql.Result, error) {
	query, args, err := dialect.Update(table).Set(record).Where(ex...).ToSQL()
	if err != nil {
		return nil, err
	}
	return exec.ExecContext(ctx, query, args...)
}

// DeleteCtx：在 sqlx.ExtContext 上执行 DELETE
func DeleteCtx(ctx context.Context, exec sqlx.ExtContext, table string, ex ...exp.Expression) (sql.Result, error) {
	query, args, err := dialect.Delete(table).Where(ex...).ToSQL()
	if err != nil {
		return nil, err
	}
	return exec.ExecContext(ctx, query, args...)
}

// SelectOneExtCtx：在 sqlx.ExtContext 上查询单条记录
func SelectOneExtCtx(ctx context.Context, exec sqlx.ExtContext, data interface{}, table string, fields []interface{}, ex ...exp.Expression) error {
	query, args, err := dialect.Select(fields...).From(table).Where(ex...).Limit(1).ToSQL()
	if err != nil {
		return err
	}
	return sqlx.GetContext(ctx, exec, data, query, args...)
}

// SelectOneTxCtx：在事务中查询单条记录，可选 FOR UPDATE
func SelectOneTxCtx(ctx context.Context, tx *sqlx.Tx, data interface{}, table string, fields []interface{}, ex exp.Expression, forUpdate bool) error {
	query, args, err := dialect.Select(fields...).From(table).Where(ex).Limit(1).ToSQL()
	if err != nil {
		return err
	}
	if forUpdate {
		query += " FOR UPDATE"
	}
	return tx.GetContext(ctx, data, query, args...)
}

// SelectAllCtx：查询多条记录（使用 *sqlx.DB）
func SelectAllCtx(ctx context.Context, data interface{}, args QueryArg) error {
	if args.Db == nil {
		return fmt.Errorf("invalid db")
	}
	if args.Table == "" {
		return fmt.Errorf("invalid table")
	}
	if len(args.Fields) == 0 {
		return fmt.Errorf("invalid fields")
	}
	ds := dialect.Select(args.Fields...).From(args.Table)
	if len(args.Ex) > 0 {
		ds = ds.Where(args.Ex...)
	}
	if len(args.GroupBy) > 0 {
		ds = ds.GroupBy(args.GroupBy...)
	}
	if len(args.Order) > 0 {
		ds = ds.Order(args.Order...)
	}
	if args.Offset > 0 {
		ds = ds.Offset(args.Offset)
	}
	if args.Limit > 0 {
		ds = ds.Limit(args.Limit)
	}
	query, qargs, _ := ds.ToSQL()
	return args.Db.SelectContext(ctx, data, query, qargs...)
}
