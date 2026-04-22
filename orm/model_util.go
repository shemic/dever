package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/shemic/dever/observe"
	"github.com/shemic/dever/util"
)

// 工具函数和辅助方法（执行器、反射、通用辅助）。

type executor struct {
	db *sqlx.DB
	tx *sqlx.Tx
}

type observedRow struct {
	row  *sqlx.Row
	span observe.Span
	once sync.Once
}

func newExecutor(ctx context.Context, db *sqlx.DB) executor {
	exec := executor{db: db}
	if tx := txFromContext(ctx); tx != nil {
		exec.tx = tx
	}
	return exec
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func (m *modelCore) executor(ctx context.Context) (context.Context, executor) {
	ctx = normalizeContext(ctx)
	baseDB, err := m.db()
	panicOnError(err)
	return ctx, newExecutor(ctx, baseDB)
}

func (e executor) rebind(query string) string {
	return e.db.Rebind(query)
}

func (e executor) selectContext(ctx context.Context, dest any, query string, args ...any) (err error) {
	ctx, span := startDBObserve(ctx, "select", query)
	defer func() {
		finishDBObserve(span, err)
	}()
	if e.tx != nil {
		err = e.tx.SelectContext(ctx, dest, query, args...)
		return err
	}
	err = e.db.SelectContext(ctx, dest, query, args...)
	return err
}

func (e executor) queryxContext(ctx context.Context, query string, args ...any) (rows *sqlx.Rows, err error) {
	ctx, span := startDBObserve(ctx, "query", query)
	defer func() {
		finishDBObserve(span, err)
	}()
	if e.tx != nil {
		rows, err = e.tx.QueryxContext(ctx, query, args...)
		return rows, err
	}
	rows, err = e.db.QueryxContext(ctx, query, args...)
	return rows, err
}

func (e executor) queryRowxContext(ctx context.Context, query string, args ...any) *observedRow {
	ctx, span := startDBObserve(ctx, "query_row", query)
	var row *sqlx.Row
	if e.tx != nil {
		row = e.tx.QueryRowxContext(ctx, query, args...)
	} else {
		row = e.db.QueryRowxContext(ctx, query, args...)
	}
	return &observedRow{row: row, span: span}
}

func (e executor) execContext(ctx context.Context, query string, args ...any) (result sql.Result, err error) {
	ctx, span := startDBObserve(ctx, "exec", query)
	defer func() {
		finishDBObserve(span, err)
	}()
	if e.tx != nil {
		result, err = e.tx.ExecContext(ctx, query, args...)
		return result, err
	}
	result, err = e.db.ExecContext(ctx, query, args...)
	return result, err
}

func (e executor) namedExecContext(ctx context.Context, query string, arg any) (result sql.Result, err error) {
	ctx, span := startDBObserve(ctx, "named_exec", query)
	defer func() {
		finishDBObserve(span, err)
	}()
	if e.tx != nil {
		result, err = e.tx.NamedExecContext(ctx, query, arg)
		return result, err
	}
	result, err = e.db.NamedExecContext(ctx, query, arg)
	return result, err
}

func (m *modelCore) db() (*sqlx.DB, error) {
	m.dbOnce.Do(func() {
		m.dbCached, m.dbErr = Get(m.dbName)
		if m.dbErr == nil && m.dbCached != nil {
			m.driverName = normalizeDriver(m.dbCached.DriverName())
		}
	})
	return m.dbCached, m.dbErr
}

func (m *modelCore) identifierQuoter() func(string) string {
	m.quoteOnce.Do(func() {
		driver := m.driverName
		m.quotedTable = quoteIdentifier(driver, m.table)
	})
	driver := m.driverName
	return func(name string) string {
		return quoteIdentifier(driver, name)
	}
}

func (m *modelCore) quotedTableName() string {
	m.quoteOnce.Do(func() {
		driver := m.driverName
		m.quotedTable = quoteIdentifier(driver, m.table)
	})
	if strings.TrimSpace(m.quotedTable) != "" {
		return m.quotedTable
	}
	return m.table
}

func (m *modelCore) ensureVersionColumn() bool {
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

func hasSchemaType[T any](args []any) bool {
	schemaType := reflect.TypeOf((*T)(nil)).Elem()
	for _, arg := range args {
		if arg == nil {
			continue
		}
		t := reflect.TypeOf(arg)
		if t == schemaType {
			return true
		}
		if t.Kind() == reflect.Pointer && t.Elem() == schemaType {
			return true
		}
	}
	return false
}

func applyTablePrefix(table, dbName string) string {
	prefix := getDatabasePrefix(dbName)
	if prefix == "" {
		return table
	}
	parts := strings.Split(table, ".")
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, prefix+"_") {
		return table
	}
	parts[len(parts)-1] = prefix + "_" + last
	return strings.Join(parts, ".")
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
	switch v := raw.(type) {
	case []any:
		var builder strings.Builder
		written := 0
		for idx, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			written = appendJoinClause(&builder, written, idx, m)
		}
		return builder.String()
	case []map[string]any:
		var builder strings.Builder
		written := 0
		for idx := range v {
			written = appendJoinClause(&builder, written, idx, v[idx])
		}
		return builder.String()
	default:
		return ""
	}
}

func normalizeJoinType(raw string) string {
	joinType := strings.TrimSpace(raw)
	if joinType == "" {
		return "LEFT JOIN"
	}
	joinType = strings.ToUpper(joinType)
	if !strings.Contains(joinType, "JOIN") {
		joinType += " JOIN"
	}
	return joinType
}

func appendJoinClause(builder *strings.Builder, written, idx int, m map[string]any) int {
	table := strings.TrimSpace(firstNonEmptyString(m, "table", "name"))
	if table == "" || ensureIdentifier(table) != nil {
		return written
	}
	onClause := strings.TrimSpace(firstNonEmptyString(m, "on"))
	if onClause == "" {
		return written
	}
	joinType := normalizeJoinType(firstNonEmptyString(m, "type"))
	if written > 0 {
		builder.WriteByte(' ')
	}
	builder.WriteString(joinType)
	builder.WriteByte(' ')
	builder.WriteString(table)
	builder.WriteString(" AS t")
	builder.WriteString(strconv.Itoa(idx))
	builder.WriteString(" ON ")
	builder.WriteString(onClause)
	return written + 1
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str := util.ToStringTrimmed(val); str != "" {
				return str
			}
		}
	}
	return ""
}

func startDBObserve(ctx context.Context, operation, query string) (context.Context, observe.Span) {
	return observe.Start(ctx, observe.KindDB, operation, map[string]any{
		"db.operation": operation,
		"db.statement": normalizeSQLStatement(query),
	})
}

func finishDBObserve(span observe.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

func normalizeObserveDBError(err error) error {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (r *observedRow) Scan(dest ...any) error {
	if r == nil || r.row == nil {
		return sql.ErrNoRows
	}
	err := r.row.Scan(dest...)
	r.finish(err)
	return err
}

func (r *observedRow) MapScan(dest map[string]any) error {
	if r == nil || r.row == nil {
		return sql.ErrNoRows
	}
	err := r.row.MapScan(dest)
	r.finish(err)
	return err
}

func (r *observedRow) finish(err error) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		finishDBObserve(r.span, normalizeObserveDBError(err))
	})
}

func normalizeSQLStatement(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func quoteWith(ident string, quoter func(string) string) string {
	if quoter == nil {
		return ident
	}
	return quoter(ident)
}

func modelCacheKey(table string, args ...any) string {
	normalizedTable := strings.ToLower(strings.TrimSpace(table))
	if len(args) == 0 {
		return normalizedTable
	}

	var builder strings.Builder
	builder.WriteString(normalizedTable)
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
