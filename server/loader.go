package server

import "sync"

// 注册函数类型
type RegisterFunc func(Server)

// 全局注册表
var (
	mu       sync.Mutex
	registry []RegisterFunc
)

// 模块调用的注册入口
func Auto(fn RegisterFunc) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, fn)
}

// 框架启动时执行
func LoadAll(s Server) {
	mu.Lock()
	fns := append([]RegisterFunc(nil), registry...)
	mu.Unlock()
	for _, fn := range fns {
		fn(s)
	}
}
