package orm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/shemic/dever/util"
)

// WHERE 条件解析与参数构建（仅负责条件拼装，不做查询执行）。

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
	if len(items) == 1 {
		return parseConditionMap(items[0], glue, quoter)
	}
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
	if len(items) == 1 {
		return parseCondition(items[0], glue, quoter)
	}
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
	if len(m) == 1 {
		for key, value := range m {
			lower := strings.ToLower(strings.TrimSpace(key))
			switch lower {
			case "and", "&&":
				return parseCondition(value, "AND", quoter)
			case "or", "||":
				return parseCondition(value, "OR", quoter)
			default:
				return parseFieldCondition(key, value, quoter)
			}
		}
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
		if len(val) == 1 {
			for op, item := range val {
				return buildComparisonClause(field, op, item, quoter)
			}
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
		if shouldUseInComparison(value) {
			clause, args := buildComparisonClause(field, "in", value, quoter)
			return clause, args
		}
		clause, args := buildComparisonClause(field, "=", val, quoter)
		return clause, args
	}
}

func shouldUseInComparison(value any) bool {
	if value == nil {
		return false
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false
	}

	// []byte/[]uint8 更接近单个二进制值，不应默认展开为 IN 查询。
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return false
	}

	return true
}

func buildComparisonClause(field, op string, value any, quoter func(string) string) (string, []any) {
	op = normalizeComparisonOperator(op)
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
		valStr := util.ToStringTrimmed(value)
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

func normalizeComparisonOperator(op string) string {
	switch strings.TrimSpace(strings.ToUpper(op)) {
	case "GT":
		return ">"
	case "GTE":
		return ">="
	case "LT":
		return "<"
	case "LTE":
		return "<="
	case "NE", "NEQ":
		return "!="
	case "EQ":
		return "="
	default:
		return strings.TrimSpace(strings.ToUpper(op))
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
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		return typedValueSlice(v), true
	case []int:
		return typedValueSlice(v), true
	case []int64:
		return typedValueSlice(v), true
	case []uint:
		return typedValueSlice(v), true
	case []uint64:
		return typedValueSlice(v), true
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

func typedValueSlice[T any](items []T) []any {
	result := make([]any, len(items))
	for i := range items {
		result[i] = items[i]
	}
	return result
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

func appendLimit(query string, options map[string]any) string {
	if options == nil {
		return query
	}
	if limit, ok := options["limit"].(string); ok && strings.TrimSpace(limit) != "" {
		return query + " LIMIT " + limit
	}
	page, hasPage := util.ParseInt64(options["page"])
	pageSize, hasSize := util.ParseInt64(options["pageSize"])
	if hasPage && hasSize && page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		return fmt.Sprintf("%s LIMIT %d OFFSET %d", query, pageSize, offset)
	}
	return query
}

func wrapAggregateExpression(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		trimmed = "0"
	}
	return fmt.Sprintf("COALESCE(%s, 0)", trimmed)
}
