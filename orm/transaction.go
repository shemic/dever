package orm

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type txContextKey struct{}

// Transaction 在指定数据库连接上开启事务，并将事务句柄注入到上下文中。
// 若上下文已存在事务，则直接复用，避免重复开启。
func Transaction(ctx context.Context, fn func(context.Context) error, name ...string) error {
	if fn == nil {
		return fmt.Errorf("orm: transaction callback required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if current := txFromContext(ctx); current != nil {
		return fn(ctx)
	}
	db, err := Get(name...)
	if err != nil {
		return err
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func txFromContext(ctx context.Context) *sqlx.Tx {
	if ctx == nil {
		return nil
	}
	if tx, ok := ctx.Value(txContextKey{}).(*sqlx.Tx); ok {
		return tx
	}
	return nil
}
