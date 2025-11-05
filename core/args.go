package core

// Arg 表示通过 Load 传入的额外键值参数。
type Arg struct {
	Key   string
	Value any
}

// Param 构造一个额外参数，便于在 Load 可变参中直接填写。
func Param(key string, value any) Arg {
	return Arg{Key: key, Value: value}
}
