package core

import "fmt"

// LoadAs 尝试将 Load 的结果转换为指定类型。
// 若类型不匹配将 panic，可用于生成器提供的强类型封装。
func LoadAs[T any](name string, args ...any) T {
	result := Load(name, args...)
	if result == nil {
		var zero T
		return zero
	}
	typed, ok := result.(T)
	if !ok {
		panic(fmt.Sprintf("core: service %s returns %T, not expected type", name, result))
	}
	return typed
}
