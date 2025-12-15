package util

import "fmt"

// GetString 读取 map 中的字符串值，支持默认值。
func GetString(m map[string]interface{}, key, def string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}

	switch vv := v.(type) {
	case string:
		if vv == "" {
			return def
		}
		return vv
	case []byte:
		if len(vv) == 0 {
			return def
		}
		return string(vv)
	default:
		s := fmt.Sprint(v)
		if s == "" {
			return def
		}
		return s
	}
}
