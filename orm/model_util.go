package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx"
)

// 工具函数和辅助方法（执行器、反射、通用辅助）。

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
	driver := m.driverName
	return func(name string) string {
		return quoteIdentifier(driver, name)
	}
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

func quoteWith(ident string, quoter func(string) string) string {
	if quoter == nil {
		return ident
	}
	return quoter(ident)
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
