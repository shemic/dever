package orm

import (
	"context"
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
	schema       *tableSchema
	dbOnce       sync.Once
	dbCached     *sqlx.DB
	dbErr        error
	defaultOrder string
}

var (
	modelCacheMu sync.RWMutex
	modelCache   = map[string]*Model{}
)

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
	var into any
	lockFlag := false
	if len(lock) > 0 {
		lockFlag = lock[0]
	}
	db, err := m.db()
	panicOnError(err)

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

	query := fmt.Sprintf("SELECT %s FROM %s AS main", fields, m.table)
	if joinClause != "" {
		query += " " + joinClause
	}
	whereClause, args := buildWhereClause(filters)
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

	query = db.Rebind(query)
	if into != nil {
		panicOnError(ensureIntoDest(into))
		panicOnError(db.SelectContext(ctx, into, query, args...))
		return []map[string]any{}
	}

	rows, err := db.QueryxContext(ctx, query, args...)
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

// Find 查询单条记录。
func (m *Model) Find(ctx context.Context, filters any, options ...map[string]any) map[string]any {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := m.db()
	panicOnError(err)

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

	query := fmt.Sprintf("SELECT %s FROM %s AS main", fields, m.table)
	if joinClause != "" {
		query += " " + joinClause
	}
	whereClause, args := buildWhereClause(filters)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " LIMIT 1"
	query = db.Rebind(query)

	row := db.QueryRowxContext(ctx, query, args...)
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
	db, err := m.db()
	panicOnError(err)
	query, payload, err := buildInsertQuery(m.table, data)
	panicOnError(err)
	res, err := db.NamedExecContext(ctx, query, payload)
	panicOnError(err)
	if id, err := res.LastInsertId(); err == nil {
		return id
	}
	return 0
}

// Update 根据条件更新数据。
func (m *Model) Update(ctx context.Context, filters any, data map[string]any) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := m.db()
	panicOnError(err)
	if len(data) == 0 {
		panic(fmt.Errorf("orm: update %s requires at least one column", m.table))
	}
	setKeys := sortedKeys(data)
	args := make([]any, 0, len(setKeys))
	var setBuilder strings.Builder
	for i, key := range setKeys {
		panicOnError(ensureIdentifier(key))
		if i > 0 {
			setBuilder.WriteString(", ")
		}
		setBuilder.WriteString(key)
		setBuilder.WriteString(" = ?")
		args = append(args, data[key])
	}
	whereClause, whereArgs := buildWhereClause(filters)
	if strings.TrimSpace(whereClause) == "" {
		panic(fmt.Errorf("orm: update %s requires filter conditions", m.table))
	}
	args = append(args, whereArgs...)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", m.table, setBuilder.String(), whereClause)
	query = db.Rebind(query)
	res, err := db.ExecContext(ctx, query, args...)
	panicOnError(err)
	affected, err := res.RowsAffected()
	panicOnError(err)
	return affected
}

// Delete 根据条件删除数据。
func (m *Model) Delete(ctx context.Context, filters any) int64 {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := m.db()
	panicOnError(err)
	whereClause, whereArgs := buildWhereClause(filters)
	if strings.TrimSpace(whereClause) == "" {
		panic(fmt.Errorf("orm: delete %s requires filter conditions", m.table))
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", m.table, whereClause)
	query = db.Rebind(query)
	res, err := db.ExecContext(ctx, query, whereArgs...)
	panicOnError(err)
	affected, err := res.RowsAffected()
	panicOnError(err)
	return affected
}

func (m *Model) db() (*sqlx.DB, error) {
	m.dbOnce.Do(func() {
		m.dbCached, m.dbErr = Get(m.dbName)
	})
	return m.dbCached, m.dbErr
}

func buildWhereClause(filters any) (string, []any) {
	if filters == nil {
		return "", nil
	}
	clause, args := parseCondition(filters, "AND")
	return clause, args
}

func parseCondition(filters any, glue string) (string, []any) {
	switch v := filters.(type) {
	case map[string]any:
		return parseConditionMap(v, glue)
	case []map[string]any:
		return parseConditionSliceMap(v, glue)
	case []any:
		return parseConditionSlice(v, glue)
	default:
		return "", nil
	}
}

func parseConditionSliceMap(items []map[string]any, glue string) (string, []any) {
	clauses := make([]string, 0, len(items))
	var args []any
	for _, item := range items {
		clause, subArgs := parseConditionMap(item, glue)
		if clause == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("(%s)", clause))
		args = append(args, subArgs...)
	}
	return joinClauses(clauses, glue), args
}

func parseConditionSlice(items []any, glue string) (string, []any) {
	clauses := make([]string, 0, len(items))
	var args []any
	for _, item := range items {
		clause, subArgs := parseCondition(item, glue)
		if clause == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("(%s)", clause))
		args = append(args, subArgs...)
	}
	return joinClauses(clauses, glue), args
}

func parseConditionMap(m map[string]any, glue string) (string, []any) {
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
			clause, subArgs := parseCondition(value, "AND")
			if clause != "" {
				clauses = append(clauses, fmt.Sprintf("(%s)", clause))
				args = append(args, subArgs...)
			}
		case "or", "||":
			clause, subArgs := parseCondition(value, "OR")
			if clause != "" {
				clauses = append(clauses, fmt.Sprintf("(%s)", clause))
				args = append(args, subArgs...)
			}
		default:
			clause, subArgs := parseFieldCondition(key, value)
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			args = append(args, subArgs...)
		}
	}
	return joinClauses(clauses, glue), args
}

func parseFieldCondition(field string, value any) (string, []any) {
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
			clause, subArgs := buildComparisonClause(field, op, val[op])
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			args = append(args, subArgs...)
		}
		return joinClauses(clauses, "AND"), args
	case []any:
		clause, args := buildComparisonClause(field, "in", val)
		return clause, args
	default:
		clause, args := buildComparisonClause(field, "=", val)
		return clause, args
	}
}

func buildComparisonClause(field, op string, value any) (string, []any) {
	op = strings.TrimSpace(strings.ToUpper(op))
	if op == "" {
		op = "="
	}
	switch op {
	case "IN", "NOT IN":
		slice, ok := valueSlice(value)
		if !ok || len(slice) == 0 {
			return "", nil
		}
		placeholders := strings.Repeat("?,", len(slice))
		placeholders = placeholders[:len(placeholders)-1]
		return fmt.Sprintf("%s %s (%s)", field, op, placeholders), slice
	case "BETWEEN":
		slice, ok := valueSlice(value)
		if !ok || len(slice) != 2 {
			return "", nil
		}
		return fmt.Sprintf("%s BETWEEN ? AND ?", field), slice
	case "LIKE", "NOT LIKE":
		return fmt.Sprintf("%s %s ?", field, op), []any{value}
	case "IS", "IS NOT":
		valStr := strings.TrimSpace(fmt.Sprint(value))
		if strings.EqualFold(valStr, "null") || value == nil {
			return fmt.Sprintf("%s %s NULL", field, op), nil
		}
		return fmt.Sprintf("%s %s %s", field, op, valStr), nil
	default:
		if value == nil {
			if op == "=" {
				return fmt.Sprintf("%s IS NULL", field), nil
			}
			if op == "<>" || op == "!=" {
				return fmt.Sprintf("%s IS NOT NULL", field), nil
			}
		}
		return fmt.Sprintf("%s %s ?", field, op), []any{value}
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
	for k, v := range record {
		if b, ok := v.([]byte); ok {
			record[k] = string(b)
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
