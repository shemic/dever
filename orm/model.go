package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/jmoiron/sqlx"
)

// Model 提供最小化的 CRUD 能力，并在初始化时支持基于结构体的自动建表/更新。
type Model struct {
	table        string
	primaryKey   string
	dbName       string
	driverName   string
	schema       *tableSchema
	dbOnce       sync.Once
	dbCached     *sqlx.DB
	dbErr        error
	defaultOrder string
	versionOnce  sync.Once
	hasVersion   bool
}

var (
	modelCacheMu sync.RWMutex
	modelCache   = map[string]*Model{}
)

// EnsureCachedSchemas 尝试为已加载的模型同步表结构，适用于模型早于数据库初始化加载的场景。
func EnsureCachedSchemas(ctx context.Context) error {
	if !autoMigrateEnabled() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	modelCacheMu.RLock()
	cached := make([]*Model, 0, len(modelCache))
	for _, m := range modelCache {
		cached = append(cached, m)
	}
	modelCacheMu.RUnlock()

	for _, m := range cached {
		if m == nil || m.schema == nil {
			continue
		}
		if err := m.ensureSchema(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewModel 创建模型，参数格式：
//   - table 必填，表示表名
//   - 可选参数：字符串表示数据库名称
func NewModel(table string, args ...any) (*Model, error) {
	m := &Model{
		table:      table,
		primaryKey: "id",
		dbName:     currentDefaultDatabase(),
	}

	var (
		schemaModel  any
		indexModels  []any
		seedRows     []map[string]any
		defaultOrder string
		orderSet     bool
	)

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				continue
			}
			if schemaModel != nil && !orderSet && looksLikeOrder(trimmed) {
				defaultOrder = trimmed
				orderSet = true
				continue
			}
			m.dbName = trimmed
		case *string:
			if v != nil && strings.TrimSpace(*v) != "" {
				m.dbName = *v
			}
		case []map[string]any:
			if len(v) > 0 {
				seedRows = append(seedRows, cloneSeedRows(v)...)
			}
		case SeedData:
			if len(v.Rows) > 0 {
				seedRows = append(seedRows, cloneSeedRows(v.Rows)...)
			}
		case nil:
			continue
		default:
			if isStructLike(v) {
				if schemaModel == nil {
					schemaModel = v
				} else {
					indexModels = append(indexModels, v)
				}
				continue
			}
			return nil, fmt.Errorf("orm: unsupported argument type %T for NewModel", v)
		}
	}

	if schemaModel != nil {
		options := schemaOptions{
			indexes: indexModels,
			seeds:   seedRows,
		}
		if err := registerSchemaOnce(table, schemaModel, options); err != nil {
			return nil, err
		}
	}

	m.defaultOrder = strings.TrimSpace(defaultOrder)

	schema, ok := getRegisteredSchema(table)
	if !ok {
		return nil, fmt.Errorf("orm: table %s schema not registered, please call orm.RegisterModel", table)
	}
	m.schema = schema

	if autoMigrateEnabled() {
		if err := m.ensureSchema(context.Background()); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// LoadModel 返回缓存的模型实例，若不存在则创建并缓存。
func LoadModel(table string, args ...any) (*Model, error) {
	if err := ensureDatabaseInitialized(); err != nil {
		return nil, err
	}

	key := modelCacheKey(table, args...)
	modelCacheMu.RLock()
	if cached, ok := modelCache[key]; ok {
		modelCacheMu.RUnlock()
		return cached, nil
	}
	modelCacheMu.RUnlock()

	created, err := NewModel(table, args...)
	if err != nil {
		return nil, err
	}

	modelCacheMu.Lock()
	defer modelCacheMu.Unlock()
	if cached, ok := modelCache[key]; ok {
		return cached, nil
	}
	modelCache[key] = created
	return created, nil
}

// MustLoadModel is LoadModel but panics on error.
func MustLoadModel(table string, args ...any) *Model {
	m, err := LoadModel(table, args...)
	if err != nil {
		panic(err)
	}
	return m
}

// Select 查询多条记录。
func (m *Model) Select(ctx context.Context, filters any, options map[string]any, lock ...bool) []map[string]any {
	if ctx == nil {
		ctx = context.Background()
	}
	if filters == nil {
		filters = map[string]any{}
	}
	if filters != nil {
		filters = m.normalizeFilters(filters)
	}
	var into any
	lockFlag := false
	if len(lock) > 0 {
		lockFlag = lock[0]
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
	quoter := m.identifierQuoter()

	fields := "main.*"
	joinClause := ""
	if options != nil {
		if val, ok := options["field"].(string); ok && strings.TrimSpace(val) != "" {
			fields = val
		}
		if dest, ok := options["into"]; ok {
			into = dest
		}
		if joinRaw, ok := options["join"]; ok {
			joinClause = buildJoinClause(joinRaw)
		}
	}

	tableName := quoteWith(m.table, quoter)
	query := fmt.Sprintf("SELECT %s FROM %s AS main", fields, tableName)
	if joinClause != "" {
		query += " " + joinClause
	}
	filters = m.normalizeFilters(filters)
	whereClause, args := buildWhereClauseWithQuoter(filters, quoter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	orderExpr := m.defaultOrder
	if options != nil {
		if val, ok := options["order"].(string); ok {
			trimmed := strings.TrimSpace(val)
			if trimmed != "" {
				orderExpr = trimmed
			} else {
				orderExpr = ""
			}
		}
	}
	if orderExpr != "" {
		query += " ORDER BY " + orderExpr
	}
	query = appendLimit(query, options)
	if lockFlag {
		query += " FOR UPDATE"
	}

	query = exec.rebind(query)
	if into != nil {
		panicOnError(ensureIntoDest(into))
		panicOnError(exec.selectContext(ctx, into, query, args...))
		return []map[string]any{}
	}

	rows, err := exec.queryxContext(ctx, query, args...)
	panicOnError(err)
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		record := make(map[string]any)
		panicOnError(rows.MapScan(record))
		normalizeMap(record)
		result = append(result, record)
	}
	if result == nil {
		return []map[string]any{}
	}
	return result
}

// Count 根据过滤条件统计满足条件的行数，可选传入 join 等选项（同 Select）。
func (m *Model) Count(ctx context.Context, filters any, options ...map[string]any) int64 {
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
func (m *Model) Sum(ctx context.Context, column string, filters any, options ...map[string]any) float64 {
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

// Find 查询单条记录。
func (m *Model) Find(ctx context.Context, filters any, options ...map[string]any) map[string]any {
	if ctx == nil {
		ctx = context.Background()
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
	quoter := m.identifierQuoter()

	fields := "main.*"
	joinClause := ""
	if len(options) > 0 && options[0] != nil {
		opt := options[0]
		if val, ok := opt["field"].(string); ok && strings.TrimSpace(val) != "" {
			fields = val
		}
		if joinRaw, ok := opt["join"]; ok {
			joinClause = buildJoinClause(joinRaw)
		}
	}

	tableName := quoteWith(m.table, quoter)
	query := fmt.Sprintf("SELECT %s FROM %s AS main", fields, tableName)
	if joinClause != "" {
		query += " " + joinClause
	}
	whereClause, args := buildWhereClauseWithQuoter(filters, quoter)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " LIMIT 1"
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
	normalizeMap(record)
	return record
}

// Insert 插入数据，返回自增主键（若驱动支持）。
func (m *Model) Insert(ctx context.Context, data map[string]any) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(data) > 0 {
		data = m.normalizeColumns(data)
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
	quoter := m.identifierQuoter()
	query, payload, err := buildInsertQuery(m.table, data, quoter)
	panicOnError(err)
	res, err := exec.namedExecContext(ctx, query, payload)
	panicOnError(err)
	if id, err := res.LastInsertId(); err == nil {
		return id
	}
	return 0
}

// Update 根据条件更新数据。
func (m *Model) Update(ctx context.Context, filters any, data map[string]any, optimistic ...bool) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
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
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", quoteWith(m.table, quoter), setBuilder.String(), whereClause)
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
func (m *Model) Delete(ctx context.Context, filters any) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
	quoter := m.identifierQuoter()
	filters = m.normalizeFilters(filters)
	whereClause, whereArgs := buildWhereClauseWithQuoter(filters, quoter)
	if strings.TrimSpace(whereClause) == "" {
		panic(fmt.Errorf("orm: delete %s requires filter conditions", m.table))
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", quoteWith(m.table, quoter), whereClause)
	query = exec.rebind(query)
	res, err := exec.execContext(ctx, query, whereArgs...)
	panicOnError(err)
	affected, err := res.RowsAffected()
	panicOnError(err)
	return affected
}

func (m *Model) aggregateInt(ctx context.Context, expr string, filters any, options map[string]any) int64 {
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

func (m *Model) aggregateFloat(ctx context.Context, expr string, filters any, options map[string]any) float64 {
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

func (m *Model) aggregateRow(ctx context.Context, expr string, filters any, options map[string]any) *sqlx.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	if filters == nil {
		filters = map[string]any{}
	}
	if filters != nil {
		filters = m.normalizeFilters(filters)
	}
	baseDB, err := m.db()
	panicOnError(err)
	exec := newExecutor(ctx, baseDB)
	quoter := m.identifierQuoter()

	joinClause := ""
	if options != nil {
		if joinRaw, ok := options["join"]; ok {
			joinClause = buildJoinClause(joinRaw)
		}
	}

	query := fmt.Sprintf("SELECT %s FROM %s AS main", wrapAggregateExpression(expr), quoteWith(m.table, quoter))
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

func wrapAggregateExpression(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		trimmed = "0"
	}
	return fmt.Sprintf("COALESCE(%s, 0)", trimmed)
}

func (m *Model) db() (*sqlx.DB, error) {
	m.dbOnce.Do(func() {
		m.dbCached, m.dbErr = Get(m.dbName)
		if m.dbErr == nil && m.dbCached != nil {
			m.driverName = normalizeDriver(m.dbCached.DriverName())
		}
	})
	return m.dbCached, m.dbErr
}

func (m *Model) identifierQuoter() func(string) string {
	driver := m.driverName
	return func(name string) string {
		return quoteIdentifier(driver, name)
	}
}

func (m *Model) ensureVersionColumn() bool {
	m.versionOnce.Do(func() {
		if m.schema == nil {
			return
		}
		for _, col := range m.schema.Columns {
			if strings.EqualFold(col.Name, "version") {
				m.hasVersion = true
				return
			}
		}
	})
	return m.hasVersion
}

type executor struct {
	db *sqlx.DB
	tx *sqlx.Tx
}

func newExecutor(ctx context.Context, db *sqlx.DB) executor {
	exec := executor{db: db}
	if tx := txFromContext(ctx); tx != nil {
		exec.tx = tx
	}
	return exec
}

func (e executor) rebind(query string) string {
	return e.db.Rebind(query)
}

func (e executor) selectContext(ctx context.Context, dest any, query string, args ...any) error {
	if e.tx != nil {
		return e.tx.SelectContext(ctx, dest, query, args...)
	}
	return e.db.SelectContext(ctx, dest, query, args...)
}

func (e executor) queryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error) {
	if e.tx != nil {
		return e.tx.QueryxContext(ctx, query, args...)
	}
	return e.db.QueryxContext(ctx, query, args...)
}

func (e executor) queryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row {
	if e.tx != nil {
		return e.tx.QueryRowxContext(ctx, query, args...)
	}
	return e.db.QueryRowxContext(ctx, query, args...)
}

func (e executor) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if e.tx != nil {
		return e.tx.ExecContext(ctx, query, args...)
	}
	return e.db.ExecContext(ctx, query, args...)
}

func (e executor) namedExecContext(ctx context.Context, query string, arg any) (sql.Result, error) {
	if e.tx != nil {
		return e.tx.NamedExecContext(ctx, query, arg)
	}
	return e.db.NamedExecContext(ctx, query, arg)
}

func buildWhereClause(filters any) (string, []any) {
	return buildWhereClauseWithQuoter(filters, nil)
}

func buildWhereClauseWithQuoter(filters any, quoter func(string) string) (string, []any) {
	if filters == nil {
		return "", nil
	}
	clause, args := parseCondition(filters, "AND", quoter)
	return clause, args
}

func parseCondition(filters any, glue string, quoter func(string) string) (string, []any) {
	switch v := filters.(type) {
	case map[string]any:
		return parseConditionMap(v, glue, quoter)
	case []map[string]any:
		return parseConditionSliceMap(v, glue, quoter)
	case []any:
		return parseConditionSlice(v, glue, quoter)
	default:
		return "", nil
	}
}

func parseConditionSliceMap(items []map[string]any, glue string, quoter func(string) string) (string, []any) {
	clauses := make([]string, 0, len(items))
	var args []any
	for _, item := range items {
		clause, subArgs := parseConditionMap(item, glue, quoter)
		if clause == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("(%s)", clause))
		args = append(args, subArgs...)
	}
	return joinClauses(clauses, glue), args
}

func parseConditionSlice(items []any, glue string, quoter func(string) string) (string, []any) {
	clauses := make([]string, 0, len(items))
	var args []any
	for _, item := range items {
		clause, subArgs := parseCondition(item, glue, quoter)
		if clause == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("(%s)", clause))
		args = append(args, subArgs...)
	}
	return joinClauses(clauses, glue), args
}

func parseConditionMap(m map[string]any, glue string, quoter func(string) string) (string, []any) {
	if len(m) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	clauses := make([]string, 0, len(keys))
	var args []any
	for _, key := range keys {
		lower := strings.ToLower(strings.TrimSpace(key))
		value := m[key]
		switch lower {
		case "and", "&&":
			clause, subArgs := parseCondition(value, "AND", quoter)
			if clause != "" {
				clauses = append(clauses, fmt.Sprintf("(%s)", clause))
				args = append(args, subArgs...)
			}
		case "or", "||":
			clause, subArgs := parseCondition(value, "OR", quoter)
			if clause != "" {
				clauses = append(clauses, fmt.Sprintf("(%s)", clause))
				args = append(args, subArgs...)
			}
		default:
			clause, subArgs := parseFieldCondition(key, value, quoter)
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			args = append(args, subArgs...)
		}
	}
	return joinClauses(clauses, glue), args
}

func parseFieldCondition(field string, value any, quoter func(string) string) (string, []any) {
	if err := ensureQualifiedIdentifier(field); err != nil {
		return "", nil
	}
	switch val := value.(type) {
	case map[string]any:
		if len(val) == 0 {
			return "", nil
		}
		ops := make([]string, 0, len(val))
		for op := range val {
			ops = append(ops, op)
		}
		sort.Strings(ops)
		clauses := make([]string, 0, len(ops))
		var args []any
		for _, op := range ops {
			clause, subArgs := buildComparisonClause(field, op, val[op], quoter)
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			args = append(args, subArgs...)
		}
		return joinClauses(clauses, "AND"), args
	case []any:
		clause, args := buildComparisonClause(field, "in", val, quoter)
		return clause, args
	default:
		clause, args := buildComparisonClause(field, "=", val, quoter)
		return clause, args
	}
}

func buildComparisonClause(field, op string, value any, quoter func(string) string) (string, []any) {
	op = strings.TrimSpace(strings.ToUpper(op))
	if op == "" {
		op = "="
	}
	quotedField := quoteQualified(field, quoter)
	switch op {
	case "IN", "NOT IN":
		slice, ok := valueSlice(value)
		if !ok || len(slice) == 0 {
			return "", nil
		}
		placeholders := strings.Repeat("?,", len(slice))
		placeholders = placeholders[:len(placeholders)-1]
		return fmt.Sprintf("%s %s (%s)", quotedField, op, placeholders), slice
	case "BETWEEN":
		slice, ok := valueSlice(value)
		if !ok || len(slice) != 2 {
			return "", nil
		}
		return fmt.Sprintf("%s BETWEEN ? AND ?", quotedField), slice
	case "LIKE", "NOT LIKE":
		return fmt.Sprintf("%s %s ?", quotedField, op), []any{value}
	case "IS", "IS NOT":
		valStr := strings.TrimSpace(fmt.Sprint(value))
		if strings.EqualFold(valStr, "null") || value == nil {
			return fmt.Sprintf("%s %s NULL", quotedField, op), nil
		}
		return fmt.Sprintf("%s %s %s", quotedField, op, valStr), nil
	default:
		if value == nil {
			if op == "=" {
				return fmt.Sprintf("%s IS NULL", quotedField), nil
			}
			if op == "<>" || op == "!=" {
				return fmt.Sprintf("%s IS NOT NULL", quotedField), nil
			}
		}
		return fmt.Sprintf("%s %s ?", quotedField, op), []any{value}
	}
}

func joinClauses(clauses []string, glue string) string {
	n := len(clauses)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return clauses[0]
	}
	separator := " " + glue + " "
	return strings.Join(clauses, separator)
}

func valueSlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	length := rv.Len()
	result := make([]any, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, rv.Index(i).Interface())
	}
	return result, true
}

func (m *Model) normalizeColumns(data map[string]any) map[string]any {
	if len(data) == 0 || m.schema == nil {
		return data
	}
	m.schema.ensureLookup()
	normalized := make(map[string]any, len(data))
	for key, val := range data {
		if col, ok := m.schema.resolveColumn(key); ok {
			normalized[col] = val
		} else {
			normalized[key] = val
		}
	}
	return normalized
}

func (m *Model) normalizeFilters(filters any) any {
	if filters == nil || m.schema == nil {
		return filters
	}
	switch val := filters.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for key, v := range val {
			lower := strings.ToLower(strings.TrimSpace(key))
			switch lower {
			case "and", "or", "&&", "||":
				result[key] = m.normalizeFilters(v)
				continue
			}
			newKey := key
			if !strings.Contains(key, ".") {
				if col, ok := m.schema.resolveColumn(key); ok {
					newKey = col
				}
			}
			result[newKey] = v
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, 0, len(val))
		for _, item := range val {
			if item == nil {
				continue
			}
			if normalized, ok := m.normalizeFilters(item).(map[string]any); ok {
				result = append(result, normalized)
			} else {
				result = append(result, item)
			}
		}
		return result
	case []any:
		result := make([]any, 0, len(val))
		for _, item := range val {
			result = append(result, m.normalizeFilters(item))
		}
		return result
	default:
		return filters
	}
}

func appendLimit(query string, options map[string]any) string {
	if options == nil {
		return query
	}
	if limit, ok := options["limit"].(string); ok && strings.TrimSpace(limit) != "" {
		return query + " LIMIT " + limit
	}
	page, hasPage := getInt(options["page"])
	pageSize, hasSize := getInt(options["pageSize"])
	if hasPage && hasSize && page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		return fmt.Sprintf("%s LIMIT %d OFFSET %d", query, pageSize, offset)
	}
	return query
}

func getInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int8:
		return int(val), true
	case int16:
		return int(val), true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	default:
		return 0, false
	}
}

func normalizeMap(record map[string]any) {
	if len(record) == 0 {
		return
	}
	keys := make([]string, 0, len(record))
	for k := range record {
		keys = append(keys, k)
	}
	for _, k := range keys {
		if b, ok := record[k].([]byte); ok {
			record[k] = string(b)
		}
	}
	for _, k := range keys {
		snake := toSnake(k)
		if snake == "" || snake == k {
			continue
		}
		if _, exists := record[snake]; !exists {
			record[snake] = record[k]
		}
	}
}

func isStructLike(v any) bool {
	if v == nil {
		return false
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func ensureIntoDest(dest any) error {
	if dest == nil {
		return fmt.Errorf("orm: into destination must not be nil")
	}
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return fmt.Errorf("orm: into destination must be a non-nil pointer")
	}
	return nil
}

func looksLikeOrder(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if strings.ContainsAny(expr, " \t,") {
		return true
	}
	return strings.Contains(expr, ".")
}

func buildJoinClause(raw any) string {
	defs := parseJoinDefinitions(raw)
	if len(defs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(defs))
	for _, def := range defs {
		parts = append(parts, fmt.Sprintf("%s %s AS %s ON %s", def.JoinType, def.Table, def.Alias, def.On))
	}
	return strings.Join(parts, " ")
}

type joinDefinition struct {
	Table    string
	Alias    string
	JoinType string
	On       string
}

func parseJoinDefinitions(raw any) []joinDefinition {
	if raw == nil {
		return nil
	}
	var items []any
	switch v := raw.(type) {
	case []map[string]any:
		items = make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
	case []any:
		items = v
	default:
		return nil
	}

	defs := make([]joinDefinition, 0, len(items))
	for idx, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		table := firstNonEmptyString(m, "table", "name")
		table = strings.TrimSpace(table)
		if table == "" || ensureIdentifier(table) != nil {
			continue
		}
		joinType := strings.TrimSpace(firstNonEmptyString(m, "type"))
		if joinType == "" {
			joinType = "LEFT JOIN"
		} else {
			joinType = strings.ToUpper(joinType)
			if !strings.Contains(joinType, "JOIN") {
				joinType += " JOIN"
			}
		}
		onClause := strings.TrimSpace(firstNonEmptyString(m, "on"))
		if onClause == "" {
			continue
		}
		alias := fmt.Sprintf("t%d", idx)
		defs = append(defs, joinDefinition{
			Table:    table,
			Alias:    alias,
			JoinType: joinType,
			On:       onClause,
		})
	}
	return defs
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str, ok := val.(string); ok {
				if strings.TrimSpace(str) != "" {
					return str
				}
			}
		}
	}
	return ""
}

func ensureQualifiedIdentifier(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("orm: identifier cannot be empty")
	}
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		for _, part := range parts {
			if err := ensureIdentifier(part); err != nil {
				return err
			}
		}
		return nil
	}
	return ensureIdentifier(name)
}

func quoteWith(ident string, quoter func(string) string) string {
	if quoter == nil {
		return ident
	}
	return quoter(ident)
}

func quoteQualified(ident string, quoter func(string) string) string {
	if quoter == nil {
		return ident
	}
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		parts[i] = quoter(part)
	}
	return strings.Join(parts, ".")
}

func modelCacheKey(table string, args ...any) string {
	var builder strings.Builder
	builder.WriteString(strings.ToLower(strings.TrimSpace(table)))
	builder.WriteByte('|')
	for i, arg := range args {
		if i > 0 {
			builder.WriteByte(';')
		}
		switch v := arg.(type) {
		case nil:
			builder.WriteString("nil")
		case string:
			builder.WriteString("s:")
			builder.WriteString(strings.TrimSpace(v))
		case *string:
			builder.WriteString("ps:")
			if v != nil {
				builder.WriteString(strings.TrimSpace(*v))
			}
		case []map[string]any:
			builder.WriteString("seeds:")
			for _, row := range v {
				builder.WriteString(fmt.Sprintf("%v", row))
			}
		case SeedData:
			builder.WriteString("seeddata:")
			for _, row := range v.Rows {
				builder.WriteString(fmt.Sprintf("%v", row))
			}
		default:
			builder.WriteString(fmt.Sprintf("%T:%v", v, v))
		}
	}
	return builder.String()
}

func toSnake(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	var builder strings.Builder
	builder.Grow(len(runes) * 2)
	for i, r := range runes {
		nextUpperToLower := false
		if i+1 < len(runes) {
			next := runes[i+1]
			nextUpperToLower = unicode.IsUpper(r) && unicode.IsLower(next)
		}
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextUpperToLower) || unicode.IsDigit(prev) {
					builder.WriteByte('_')
				}
			}
			builder.WriteRune(unicode.ToLower(r))
			continue
		}
		if i > 0 && unicode.IsDigit(r) && unicode.IsLetter(runes[i-1]) {
			builder.WriteByte('_')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
