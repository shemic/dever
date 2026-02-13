package orm

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// 字段名/结构归一化相关函数（不负责值类型转换）。

var (
	fieldIndexCache sync.Map
	stringSlicePool sync.Pool
)

func normalizeMap(record map[string]any) {
	if len(record) == 0 {
		return
	}
	keys := getStringSlice(len(record))
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
	putStringSlice(keys)
}

func normalizeMapValues(record map[string]any) {
	if len(record) == 0 {
		return
	}
	keys := getStringSlice(len(record))
	for k := range record {
		keys = append(keys, k)
	}
	for _, k := range keys {
		if b, ok := record[k].([]byte); ok {
			record[k] = string(b)
		}
	}
	putStringSlice(keys)
}

func normalizeMapWithSchema(record map[string]any, schema *tableSchema) {
	normalizeMap(record)
	if schema == nil || len(schema.Columns) == 0 {
		return
	}
	keys := getStringSlice(len(record))
	for k := range record {
		keys = append(keys, k)
	}
	for _, k := range keys {
		col, lookupKey, ok := schema.resolveColumnDefWithAlias(k)
		if !ok {
			continue
		}
		normalized := normalizeValueByType(record[k], col.Type)
		record[k] = normalized
		if lookupKey != k {
			if _, exists := record[lookupKey]; !exists {
				record[lookupKey] = normalized
			}
		}
	}
	putStringSlice(keys)
}

func (m *modelCore) normalizeColumns(data map[string]any) map[string]any {
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

func (m *modelCore) normalizeFilters(filters any) any {
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

func mapToStruct(record map[string]any, dest any) error {
	if dest == nil {
		return fmt.Errorf("orm: dest cannot be nil")
	}
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return fmt.Errorf("orm: dest must be a non-nil pointer")
	}
	elem := val.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("orm: dest must point to struct")
	}
	fieldMap := cachedFieldIndexMap(elem.Type())
	if len(fieldMap) == 0 {
		return nil
	}
	for key, raw := range record {
		index, ok := resolveFieldIndex(fieldMap, key)
		if !ok {
			continue
		}
		field := elem.Field(index)
		if !field.CanSet() {
			continue
		}
		assignStructField(field, raw)
	}
	return nil
}

func buildFieldColumnIndex(t reflect.Type) map[string]int {
	index := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		column := fieldColumnName(field)
		if column == "" {
			continue
		}
		index[normalizeColumnKey(column)] = i
	}
	return index
}

func cachedFieldIndexMap(t reflect.Type) map[string]int {
	if cached, ok := fieldIndexCache.Load(t); ok {
		return cached.(map[string]int)
	}
	index := buildFieldColumnIndex(t)
	fieldIndexCache.Store(t, index)
	return index
}

func resolveFieldIndex(fieldMap map[string]int, key string) (int, bool) {
	normalized := normalizeColumnKey(key)
	if index, ok := fieldMap[normalized]; ok {
		return index, true
	}
	if idx := strings.LastIndex(key, "."); idx != -1 && idx+1 < len(key) {
		base := key[idx+1:]
		normalized = normalizeColumnKey(base)
		index, ok := fieldMap[normalized]
		return index, ok
	}
	return 0, false
}

func fieldColumnName(field reflect.StructField) string {
	if dbTag := strings.TrimSpace(field.Tag.Get("db")); dbTag != "" && dbTag != "-" {
		return dbTag
	}
	tagOptions := parseDormTag(field.Tag.Get("dorm"))
	if tagExists(tagOptions, "-") {
		return ""
	}
	if col := firstNonEmpty(tagOptions["column"]); col != "" {
		return col
	}
	return toSnake(field.Name)
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

func getStringSlice(size int) []string {
	if pooled := stringSlicePool.Get(); pooled != nil {
		if keys, ok := pooled.([]string); ok {
			if cap(keys) >= size {
				return keys[:0]
			}
		}
	}
	return make([]string, 0, size)
}

func putStringSlice(keys []string) {
	if keys == nil {
		return
	}
	if cap(keys) > 4096 {
		return
	}
	stringSlicePool.Put(keys[:0])
}
