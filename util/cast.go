package util

import (
	"strconv"
	"strings"
)

func ParseInt64(value any) (int64, bool) {
	switch current := value.(type) {
	case int:
		return int64(current), true
	case int8:
		return int64(current), true
	case int16:
		return int64(current), true
	case int32:
		return int64(current), true
	case int64:
		return current, true
	case uint:
		return int64(current), true
	case uint8:
		return int64(current), true
	case uint16:
		return int64(current), true
	case uint32:
		return int64(current), true
	case uint64:
		if current > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(current), true
	case float32:
		return int64(current), true
	case float64:
		return int64(current), true
	case string:
		number, err := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
		if err == nil {
			return number, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func ToInt64(value any) int64 {
	if number, ok := ParseInt64(value); ok {
		return number
	}
	return 0
}

func ToIntDefault(value any, fallback int) int {
	number := ToInt64(value)
	if number == 0 {
		switch current := value.(type) {
		case int, int8, int16, int32, int64:
			return 0
		case uint, uint8, uint16, uint32, uint64:
			return 0
		case float32, float64:
			return 0
		case string:
			if strings.TrimSpace(current) == "" {
				return fallback
			}
			if _, ok := ParseInt64(current); !ok {
				return fallback
			}
			return 0
		default:
			return fallback
		}
	}
	return int(number)
}

func ParseUint64(value any) (uint64, bool) {
	switch current := value.(type) {
	case uint:
		return uint64(current), true
	case uint8:
		return uint64(current), true
	case uint16:
		return uint64(current), true
	case uint32:
		return uint64(current), true
	case uint64:
		return current, true
	case int:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case int8:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case int16:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case int32:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case int64:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case float32:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case float64:
		if current < 0 {
			return 0, false
		}
		return uint64(current), true
	case string:
		number, err := strconv.ParseUint(strings.TrimSpace(current), 10, 64)
		if err == nil {
			return number, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func ToUint64(value any) uint64 {
	if number, ok := ParseUint64(value); ok {
		return number
	}
	return 0
}

func ParseBool(value any) (bool, bool) {
	switch current := value.(type) {
	case bool:
		return current, true
	case int:
		return current != 0, true
	case int8:
		return current != 0, true
	case int16:
		return current != 0, true
	case int32:
		return current != 0, true
	case int64:
		return current != 0, true
	case uint:
		return current != 0, true
	case uint8:
		return current != 0, true
	case uint16:
		return current != 0, true
	case uint32:
		return current != 0, true
	case uint64:
		return current != 0, true
	case float32:
		return current != 0, true
	case float64:
		return current != 0, true
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(current))
		switch trimmed {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func ParseFloat64(value any) (float64, bool) {
	switch current := value.(type) {
	case float64:
		return current, true
	case float32:
		return float64(current), true
	case int:
		return float64(current), true
	case int8:
		return float64(current), true
	case int16:
		return float64(current), true
	case int32:
		return float64(current), true
	case int64:
		return float64(current), true
	case uint:
		return float64(current), true
	case uint8:
		return float64(current), true
	case uint16:
		return float64(current), true
	case uint32:
		return float64(current), true
	case uint64:
		return float64(current), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(current), 64)
		if err == nil {
			return number, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func ToBool(value any) bool {
	result, _ := ParseBool(value)
	return result
}

func ToKeyString(value any) string {
	switch current := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(current)
	case []byte:
		return strings.TrimSpace(string(current))
	case bool:
		return strconv.FormatBool(current)
	case int:
		return strconv.Itoa(current)
	case int8:
		return strconv.FormatInt(int64(current), 10)
	case int16:
		return strconv.FormatInt(int64(current), 10)
	case int32:
		return strconv.FormatInt(int64(current), 10)
	case int64:
		return strconv.FormatInt(current, 10)
	case uint:
		return strconv.FormatUint(uint64(current), 10)
	case uint8:
		return strconv.FormatUint(uint64(current), 10)
	case uint16:
		return strconv.FormatUint(uint64(current), 10)
	case uint32:
		return strconv.FormatUint(uint64(current), 10)
	case uint64:
		return strconv.FormatUint(current, 10)
	case float32:
		return strconv.FormatInt(int64(current), 10)
	case float64:
		return strconv.FormatInt(int64(current), 10)
	default:
		return ToStringTrimmed(value)
	}
}

func UniqueUint64s(items []uint64) []uint64 {
	if len(items) <= 1 {
		return items
	}

	result := make([]uint64, 0, len(items))
	seen := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
