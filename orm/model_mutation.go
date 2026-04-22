package orm

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// 数据修改相关方法（Insert/Update/Delete）。

// Insert 插入数据，返回自增主键（若驱动支持）。
func (m *modelCore) Insert(ctx context.Context, data map[string]any) int64 {
	if len(data) > 0 {
		data = m.normalizeColumns(data)
	}
	ctx, exec := m.executor(ctx)
	quoter := m.identifierQuoter()
	query, payload, err := buildInsertQuery(m.table, data, quoter)
	panicOnError(err)
	if m.driverName == "postgres" {
		panicOnError(ensureIdentifier(m.primaryKey))
		query = query + " RETURNING " + quoteWith(m.primaryKey, quoter)
		namedQuery, args, err := sqlx.Named(query, payload)
		panicOnError(err)
		namedQuery = exec.rebind(namedQuery)
		var id int64
		panicOnError(exec.queryRowxContext(ctx, namedQuery, args...).Scan(&id))
		return id
	}
	res, err := exec.namedExecContext(ctx, query, payload)
	panicOnError(err)
	if id, err := res.LastInsertId(); err == nil {
		return id
	}
	return 0
}

// Update 根据条件更新数据。
func (m *modelCore) Update(ctx context.Context, filters any, data map[string]any, optimistic ...bool) int64 {
	ctx, exec := m.executor(ctx)
	quoter := m.identifierQuoter()

	useOptimistic := len(optimistic) > 0 && optimistic[0]
	if len(data) == 0 && !useOptimistic {
		panic(fmt.Errorf("orm: update %s requires at least one column", m.table))
	}

	var updates map[string]any
	if data != nil {
		updates = data
	}
	if useOptimistic {
		if !m.ensureVersionColumn() {
			panic(fmt.Errorf("orm: table %s missing version column for optimistic lock", m.table))
		}
		if len(data) > 0 {
			needCopy := false
			for key := range data {
				if strings.EqualFold(key, "version") {
					needCopy = true
					break
				}
			}
			if needCopy {
				updates = make(map[string]any, len(data)-1)
				for k, v := range data {
					if strings.EqualFold(k, "version") {
						continue
					}
					updates[k] = v
				}
			}
		}
	}
	if len(updates) == 0 && !useOptimistic {
		panic(fmt.Errorf("orm: update %s requires at least one column", m.table))
	}
	if len(updates) > 0 {
		updates = m.normalizeColumns(updates)
	}

	setKeys := sortedKeys(updates)
	args := make([]any, 0, len(setKeys))
	var setBuilder strings.Builder
	for i, key := range setKeys {
		panicOnError(ensureIdentifier(key))
		if i > 0 {
			setBuilder.WriteString(", ")
		}
		setBuilder.WriteString(quoteWith(key, quoter))
		setBuilder.WriteString(" = ?")
		args = append(args, updates[key])
	}
	if useOptimistic {
		if setBuilder.Len() > 0 {
			setBuilder.WriteString(", ")
		}
		setBuilder.WriteString("version = version + 1")
	}
	filters = m.normalizeFilters(filters)
	whereClause, whereArgs := buildWhereClauseWithQuoter(filters, quoter)
	if strings.TrimSpace(whereClause) == "" {
		panic(fmt.Errorf("orm: update %s requires filter conditions", m.table))
	}
	args = append(args, whereArgs...)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", m.quotedTableName(), setBuilder.String(), whereClause)
	query = exec.rebind(query)
	res, err := exec.execContext(ctx, query, args...)
	panicOnError(err)
	affected, err := res.RowsAffected()
	panicOnError(err)
	if useOptimistic && affected == 0 {
		panic(ErrVersionConflict)
	}
	return affected
}

// Delete 根据条件删除数据。
func (m *modelCore) Delete(ctx context.Context, filters any) int64 {
	ctx, exec := m.executor(ctx)
	quoter := m.identifierQuoter()
	filters = m.normalizeFilters(filters)
	whereClause, whereArgs := buildWhereClauseWithQuoter(filters, quoter)
	if strings.TrimSpace(whereClause) == "" {
		panic(fmt.Errorf("orm: delete %s requires filter conditions", m.table))
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", m.quotedTableName(), whereClause)
	query = exec.rebind(query)
	res, err := exec.execContext(ctx, query, whereArgs...)
	panicOnError(err)
	affected, err := res.RowsAffected()
	panicOnError(err)
	return affected
}
