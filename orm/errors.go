package orm

import (
	"database/sql"
	"errors"
)

// ErrNotFound 表示未找到符合条件的记录。
var ErrNotFound = errors.New("orm: record not found")

// ErrVersionConflict 表示乐观锁版本冲突。
var ErrVersionConflict = errors.New("orm: version conflict")

// IsVersionConflict 判断错误是否为乐观锁冲突。
func IsVersionConflict(err error) bool {
	return errors.Is(err, ErrVersionConflict)
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
