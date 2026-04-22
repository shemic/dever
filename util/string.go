package util

import (
	"fmt"
	"strings"
	"unicode"
)

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

func ToString(value any) string {
	switch current := value.(type) {
	case nil:
		return ""
	case string:
		return current
	case []byte:
		return string(current)
	default:
		return fmt.Sprint(value)
	}
}

func ToStringTrimmed(value any) string {
	return strings.TrimSpace(ToString(value))
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ToSnake(name string) string {
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
