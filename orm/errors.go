package orm

import (
	"database/sql"
	"errors"
)

// ErrNotFound 表示未找到符合条件的记录。
var ErrNotFound = errors.New("orm: record not found")

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
