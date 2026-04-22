package orm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/shemic/dever/util"
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
		if v, ok := toUint64(value); ok {
			out := reflect.New(target).Elem()
			out.SetUint(v)
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
	return util.ParseInt64(value)
}

func toFloat64(value any) (float64, bool) {
	return util.ParseFloat64(value)
}

func toBool(value any) (bool, bool) {
	return util.ParseBool(value)
}

func toUint64(value any) (uint64, bool) {
	return util.ParseUint64(value)
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

func assignStructField(field reflect.Value, binding fieldBinding, value any) {
	if value == nil {
		if binding.isPointer {
			field.Set(reflect.Zero(field.Type()))
		}
		return
	}
	if binding.isPointer {
		converted, ok := convertValue(value, binding.elemType)
		if !ok {
			return
		}
		ptr := reflect.New(binding.elemType)
		ptr.Elem().Set(converted)
		field.Set(ptr)
		return
	}
	converted, ok := convertValue(value, binding.targetType)
	if ok {
		field.Set(converted)
	}
}
