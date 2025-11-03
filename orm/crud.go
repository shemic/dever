package orm

import (
	"fmt"
	"sort"
	"strings"
)

func buildInsertQuery(table string, data map[string]any) (string, map[string]any, error) {
	if err := ensureIdentifier(table); err != nil {
		return "", nil, err
	}
	if len(data) == 0 {
		return "", nil, fmt.Errorf("orm: insert %s requires at least one column", table)
	}
	keys := sortedKeys(data)
	for _, key := range keys {
		if err := ensureIdentifier(key); err != nil {
			return "", nil, err
		}
	}
	var colBuilder, placeholderBuilder strings.Builder
	for i, key := range keys {
		if i > 0 {
			colBuilder.WriteString(", ")
			placeholderBuilder.WriteString(", ")
		}
		colBuilder.WriteString(key)
		placeholderBuilder.WriteByte(':')
		placeholderBuilder.WriteString(key)
	}
	var queryBuilder strings.Builder
	queryBuilder.Grow(len(table) + colBuilder.Len() + placeholderBuilder.Len() + 32)
	queryBuilder.WriteString("INSERT INTO ")
	queryBuilder.WriteString(table)
	queryBuilder.WriteString(" (")
	queryBuilder.WriteString(colBuilder.String())
	queryBuilder.WriteString(") VALUES (")
	queryBuilder.WriteString(placeholderBuilder.String())
	queryBuilder.WriteByte(')')
	return queryBuilder.String(), data, nil
}

func buildUpdateQueryWithFilters(table string, data map[string]any, filters map[string]any) (string, []any, error) {
	if err := ensureIdentifier(table); err != nil {
		return "", nil, err
	}
	if len(data) == 0 {
		return "", nil, fmt.Errorf("orm: update %s requires at least one column", table)
	}
	setKeys := sortedKeys(data)
	args := make([]any, 0, len(setKeys))
	var setBuilder strings.Builder
	for i, key := range setKeys {
		if err := ensureIdentifier(key); err != nil {
			return "", nil, err
		}
		if i > 0 {
			setBuilder.WriteString(", ")
		}
		setBuilder.WriteString(key)
		setBuilder.WriteString(" = ?")
		args = append(args, data[key])
	}
	whereClause, whereArgs := buildWhereClause(filters)
	if whereClause == "" {
		return "", nil, fmt.Errorf("orm: update %s requires filter conditions", table)
	}
	args = append(args, whereArgs...)
	var queryBuilder strings.Builder
	queryBuilder.Grow(len(table) + setBuilder.Len() + len(whereClause) + 16)
	queryBuilder.WriteString("UPDATE ")
	queryBuilder.WriteString(table)
	queryBuilder.WriteString(" SET ")
	queryBuilder.WriteString(setBuilder.String())
	queryBuilder.WriteString(" WHERE ")
	queryBuilder.WriteString(whereClause)
	return queryBuilder.String(), args, nil
}

func buildDeleteQuery(table string, filters map[string]any) (string, []any, error) {
	if err := ensureIdentifier(table); err != nil {
		return "", nil, err
	}
	whereClause, args := buildWhereClause(filters)
	if whereClause == "" {
		return "", nil, fmt.Errorf("orm: delete %s requires filter conditions", table)
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause)
	return query, args, nil
}

func ensureIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("orm: identifier cannot be empty")
	}
	for i, r := range name {
		if !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return fmt.Errorf("orm: identifier %q contains invalid characters", name)
		}
		if i == 0 && (r >= '0' && r <= '9') {
			return fmt.Errorf("orm: identifier %q cannot start with digit", name)
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	if stableFiltersEnabled() && len(keys) > 1 {
		sort.Strings(keys)
	}
	return keys
}
