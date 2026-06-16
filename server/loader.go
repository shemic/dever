package server

import (
	"context"
	"errors"
	"sync"
)

// 注册函数类型
type RegisterFunc func(Server)
type ShutdownFunc func(context.Context) error

// 全局注册表
var (
	mu            sync.Mutex
	registry      []RegisterFunc
	shutdownHooks []ShutdownFunc
)

// 模块调用的注册入口
func Auto(fn RegisterFunc) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, fn)
}

func OnShutdown(fn ShutdownFunc) {
	if fn == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	shutdownHooks = append(shutdownHooks, fn)
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

func RunShutdownHooks(ctx context.Context) error {
	mu.Lock()
	hooks := append([]ShutdownFunc(nil), shutdownHooks...)
	mu.Unlock()

	var errs []error
	for index := len(hooks) - 1; index >= 0; index-- {
		if err := hooks[index](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
