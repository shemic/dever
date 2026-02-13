package orm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// 值类型转换相关函数（配合查询结果映射）。

func convertValue(value any, target reflect.Type) (reflect.Value, bool) {
	if value == nil {
		return reflect.Zero(target), true
	}
	val := reflect.ValueOf(value)
	if val.Type().AssignableTo(target) {
		return val, true
	}
	if val.Type().ConvertibleTo(target) {
		return val.Convert(target), true
	}
	switch target.Kind() {
	case reflect.String:
		switch v := value.(type) {
		case []byte:
			return reflect.ValueOf(string(v)), true
		case map[string]any, []any:
			encoded, err := json.Marshal(value)
			if err != nil {
				return reflect.Value{}, false
			}
			return reflect.ValueOf(string(encoded)), true
		default:
			return reflect.ValueOf(fmt.Sprint(v)), true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, ok := toInt64(value); ok {
			out := reflect.New(target).Elem()
			out.SetInt(v)
			return out, true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, ok := toInt64(value); ok && v >= 0 {
			out := reflect.New(target).Elem()
			out.SetUint(uint64(v))
			return out, true
		}
	case reflect.Float32, reflect.Float64:
		if v, ok := toFloat64(value); ok {
			out := reflect.New(target).Elem()
			out.SetFloat(v)
			return out, true
		}
	case reflect.Bool:
		if v, ok := toBool(value); ok {
			out := reflect.New(target).Elem()
			out.SetBool(v)
			return out, true
		}
	}
	return reflect.Value{}, false
}

func normalizeValueByType(value any, sqlType string) any {
	if value == nil {
		return nil
	}
	typeName := strings.ToUpper(strings.TrimSpace(sqlType))
	switch {
	case strings.Contains(typeName, "INT"):
		if v, ok := toInt64(value); ok {
			return v
		}
	case strings.Contains(typeName, "BOOL"):
		if v, ok := toBool(value); ok {
			return v
		}
	case strings.Contains(typeName, "JSON"):
		if v, ok := toJSONValue(value); ok {
			return v
		}
	case strings.Contains(typeName, "DOUBLE"), strings.Contains(typeName, "FLOAT"), strings.Contains(typeName, "DECIMAL"):
		if v, ok := toFloat64(value); ok {
			return v
		}
	}
	return value
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(v))
		if trimmed == "true" || trimmed == "1" {
			return true, true
		}
		if trimmed == "false" || trimmed == "0" {
			return false, true
		}
	}
	return false, false
}

func toJSONValue(value any) (any, bool) {
	switch v := value.(type) {
	case map[string]any, []any:
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return map[string]any{}, true
		}
		var out any
		if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
			return out, true
		}
	case []byte:
		if len(v) == 0 {
			return map[string]any{}, true
		}
		var out any
		if err := json.Unmarshal(v, &out); err == nil {
			return out, true
		}
	}
	return value, false
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

func assignStructField(field reflect.Value, value any) {
	if value == nil {
		if field.Kind() == reflect.Pointer {
			field.Set(reflect.Zero(field.Type()))
		}
		return
	}
	if field.Kind() == reflect.Pointer {
		elemType := field.Type().Elem()
		converted, ok := convertValue(value, elemType)
		if !ok {
			return
		}
		ptr := reflect.New(elemType)
		ptr.Elem().Set(converted)
		field.Set(ptr)
		return
	}
	converted, ok := convertValue(value, field.Type())
	if ok {
		field.Set(converted)
	}
}
