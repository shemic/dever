package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// 查询/聚合相关方法：负责 SELECT 语句拼装与读取结果。

func (m *modelCore) selectMaps(ctx context.Context, filters any, options map[string]any, lock ...bool) []map[string]any {
	return m.selectMapsWithOptions(ctx, filters, options, true, lock...)
}

func (m *modelCore) selectMapsWithOptions(ctx context.Context, filters any, options map[string]any, normalizeKeys bool, lock ...bool) []map[string]any {
	if filters == nil {
		filters = map[string]any{}
	}
	lockFlag := false
	if len(lock) > 0 {
		lockFlag = lock[0]
	}
	ctx, exec := m.executor(ctx)
	resolved := resolveSelectOptions(options, m.defaultOrder, true)
	query, args := m.buildSelectQuery(filters, selectQueryConfig{
		fields:           resolved.fields,
		joinRaw:          resolved.joinRaw,
		order:            resolved.order,
		applyOrder:       true,
		limitOptions:     options,
		lock:             lockFlag,
		normalizeFilters: true,
	})

	query = exec.rebind(query)
	if resolved.into != nil {
		panicOnError(ensureIntoDest(resolved.into))
		panicOnError(exec.selectContext(ctx, resolved.into, query, args...))
		return []map[string]any{}
	}

	rows, err := exec.queryxContext(ctx, query, args...)
	panicOnError(err)
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		record := make(map[string]any)
		panicOnError(rows.MapScan(record))
		if normalizeKeys {
			normalizeMapWithSchema(record, m.schema)
		} else {
			normalizeMapValues(record)
		}
		result = append(result, record)
	}
	if result == nil {
		return []map[string]any{}
	}
	return result
}

// Count 根据过滤条件统计满足条件的行数，可选传入 join 等选项（同 Select）。
func (m *modelCore) Count(ctx context.Context, filters any, options ...map[string]any) int64 {
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}
	expr := "COUNT(*)"
	if opt != nil {
		if field, ok := opt["field"].(string); ok && strings.TrimSpace(field) != "" {
			expr = field
		}
	}
	return m.aggregateInt(ctx, expr, filters, opt)
}

// Sum 返回指定字段的求和结果，column 支持表达式，可结合 join/where 选项。
func (m *modelCore) Sum(ctx context.Context, column string, filters any, options ...map[string]any) float64 {
	trimmed := strings.TrimSpace(column)
	if trimmed == "" {
		panic("orm: Sum column cannot be empty")
	}
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}
	expr := fmt.Sprintf("SUM(%s)", trimmed)
	if opt != nil {
		if field, ok := opt["field"].(string); ok && strings.TrimSpace(field) != "" {
			expr = field
		}
	}
	return m.aggregateFloat(ctx, expr, filters, opt)
}

func (m *modelCore) findMap(ctx context.Context, filters any, options ...map[string]any) map[string]any {
	ctx, exec := m.executor(ctx)
	var opt map[string]any
	if len(options) > 0 && options[0] != nil {
		opt = options[0]
	}
	resolved := resolveSelectOptions(opt, m.defaultOrder, true)
	query, args := m.buildSelectQuery(filters, selectQueryConfig{
		fields:     resolved.fields,
		joinRaw:    resolved.joinRaw,
		order:      resolved.order,
		applyOrder: true,
		limitOne:   true,
	})
	query = exec.rebind(query)

	row := exec.queryRowxContext(ctx, query, args...)
	record := make(map[string]any)
	if err := row.MapScan(record); err != nil {
		err = normalizeError(err)
		if errors.Is(err, ErrNotFound) {
			return map[string]any{}
		}
		panic(err)
	}
	normalizeMapWithSchema(record, m.schema)
	return record
}

// Select 查询多条记录。
func (m *Model[T]) Select(ctx context.Context, filters any, options ...map[string]any) []*T {
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}
	records := m.modelCore.selectMaps(ctx, filters, opt)
	if len(records) == 0 {
		return []*T{}
	}
	result := make([]*T, 0, len(records))
	for _, record := range records {
		dest := new(T)
		_ = mapToStruct(record, dest)
		result = append(result, dest)
	}
	return result
}

// SelectMap 查询多条记录并返回 map 结果。
func (m *Model[T]) SelectMap(ctx context.Context, filters any, options ...map[string]any) []map[string]any {
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}
	normalizeKeys := true
	if opt != nil {
		if field, ok := opt["field"].(string); ok && strings.TrimSpace(field) != "" {
			normalizeKeys = false
		}
	}
	return m.modelCore.selectMapsWithOptions(ctx, filters, opt, normalizeKeys)
}

// Find 查询单条记录。
func (m *Model[T]) Find(ctx context.Context, filters any, options ...map[string]any) *T {
	record := m.modelCore.findMap(ctx, filters, options...)
	if len(record) == 0 {
		return nil
	}
	dest := new(T)
	_ = mapToStruct(record, dest)
	return dest
}

// FindMap 查询单条记录并返回 map 结果。
func (m *Model[T]) FindMap(ctx context.Context, filters any, options ...map[string]any) map[string]any {
	record := m.modelCore.findMap(ctx, filters, options...)
	if len(record) == 0 {
		return map[string]any{}
	}
	if len(options) > 0 && options[0] != nil {
		if field, ok := options[0]["field"].(string); ok && strings.TrimSpace(field) != "" {
			normalizeMapValues(record)
			return record
		}
	}
	return record
}

func (m *modelCore) aggregateInt(ctx context.Context, expr string, filters any, options map[string]any) int64 {
	row := m.aggregateRow(ctx, expr, filters, options)
	var result sql.NullInt64
	if err := row.Scan(&result); err != nil {
		err = normalizeError(err)
		if errors.Is(err, ErrNotFound) {
			return 0
		}
		panic(err)
	}
	if result.Valid {
		return result.Int64
	}
	return 0
}

func (m *modelCore) aggregateFloat(ctx context.Context, expr string, filters any, options map[string]any) float64 {
	row := m.aggregateRow(ctx, expr, filters, options)
	var result sql.NullFloat64
	if err := row.Scan(&result); err != nil {
		err = normalizeError(err)
		if errors.Is(err, ErrNotFound) {
			return 0
		}
		panic(err)
	}
	if result.Valid {
		return result.Float64
	}
	return 0
}

func (m *modelCore) aggregateRow(ctx context.Context, expr string, filters any, options map[string]any) *observedRow {
	if filters == nil {
		filters = map[string]any{}
	}
	if filters != nil {
		filters = m.normalizeFilters(filters)
	}
	ctx, exec := m.executor(ctx)
	quoter := m.identifierQuoter()

	joinClause := ""
	if options != nil {
		if joinRaw, ok := options["join"]; ok {
			joinClause = buildJoinClause(joinRaw)
		}
	}

	query := fmt.Sprintf("SELECT %s FROM %s AS main", wrapAggregateExpression(expr), m.quotedTableName())
	if joinClause != "" {
		query += " " + joinClause
	}
	whereClause, args := buildWhereClauseWithQuoter(filters, quoter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query = exec.rebind(query)
	return exec.queryRowxContext(ctx, query, args...)
}

type resolvedSelectOptions struct {
	fields  string
	joinRaw any
	order   string
	into    any
}

func resolveSelectOptions(options map[string]any, defaultOrder string, allowOrder bool) resolvedSelectOptions {
	resolved := resolvedSelectOptions{
		fields: "main.*",
		order:  defaultOrder,
	}
	if options == nil {
		return resolved
	}
	if val, ok := options["field"].(string); ok && strings.TrimSpace(val) != "" {
		resolved.fields = val
	}
	if joinRaw, ok := options["join"]; ok {
		resolved.joinRaw = joinRaw
	}
	if allowOrder {
		if val, ok := options["order"].(string); ok {
			trimmed := strings.TrimSpace(val)
			if trimmed != "" {
				resolved.order = trimmed
			} else {
				resolved.order = ""
			}
		}
	}
	if dest, ok := options["into"]; ok {
		resolved.into = dest
	}
	return resolved
}

type selectQueryConfig struct {
	fields           string
	joinRaw          any
	order            string
	applyOrder       bool
	limitOne         bool
	lock             bool
	limitOptions     map[string]any
	normalizeFilters bool
}

func (m *modelCore) buildSelectQuery(filters any, cfg selectQueryConfig) (string, []any) {
	if cfg.fields == "" {
		cfg.fields = "main.*"
	}
	if cfg.normalizeFilters && filters != nil {
		filters = m.normalizeFilters(filters)
	}
	quoter := m.identifierQuoter()
	tableName := m.quotedTableName()
	query := fmt.Sprintf("SELECT %s FROM %s AS main", cfg.fields, tableName)
	if cfg.joinRaw != nil {
		joinClause := buildJoinClause(cfg.joinRaw)
		if joinClause != "" {
			query += " " + joinClause
		}
	}
	whereClause, args := buildWhereClauseWithQuoter(filters, quoter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	if cfg.applyOrder && cfg.order != "" {
		query += " ORDER BY " + cfg.order
	}
	if cfg.limitOne {
		query += " LIMIT 1"
	} else {
		query = appendLimit(query, cfg.limitOptions)
	}
	if cfg.lock {
		query += " FOR UPDATE"
	}
	return query, args
}
