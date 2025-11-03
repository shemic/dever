package middleware

import (
	"net/http"
	"strings"
	"sync"
)

// Next 表示继续执行下一个中间件或最终处理函数。
type Next func(ctx any) error

// Middleware 表示一个通用的中间件，接收上下文 ctx，并决定是否调用 next。
type Middleware func(ctx any, next Next) error

// ContextFunc 是更简洁的中间件定义，仅依赖上下文；结束后自动进入下一个环节。
type ContextFunc func(ctx any) error

var (
	mu     sync.RWMutex
	global []Middleware
	routes = map[string][]Middleware{}
)

// UseGlobal 注册全局中间件，按注册顺序执行。
func UseGlobal(middlewares ...Middleware) {
	if len(middlewares) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	global = append(global, middlewares...)
}

// UseGlobalFunc 使用仅依赖上下文的中间件。
func UseGlobalFunc(funcs ...ContextFunc) {
	if len(funcs) == 0 {
		return
	}
	middlewares := make([]Middleware, 0, len(funcs))
	for _, fn := range funcs {
		if fn == nil {
			continue
		}
		middlewares = append(middlewares, wrapContextFunc(fn))
	}
	UseGlobal(middlewares...)
}

// UseRoute 为指定 method + path 注册中间件。
func UseRoute(method, path string, middlewares ...Middleware) {
	if len(middlewares) == 0 {
		return
	}
	key := routeKey(method, path)
	mu.Lock()
	defer mu.Unlock()
	routes[key] = append(routes[key], middlewares...)
}

// UseRouteFunc 使用仅依赖上下文的中间件。
func UseRouteFunc(method, path string, funcs ...ContextFunc) {
	if len(funcs) == 0 {
		return
	}
	middlewares := make([]Middleware, 0, len(funcs))
	for _, fn := range funcs {
		if fn == nil {
			continue
		}
		middlewares = append(middlewares, wrapContextFunc(fn))
	}
	UseRoute(method, path, middlewares...)
}

// Execute 依次执行全局、路由级中间件，最后调用 final。
func Execute(ctx any, method, path string, final ContextFunc) error {
	handlers := collect(method, path, final)
	return runChain(ctx, handlers)
}

func runChain(ctx any, handlers []Middleware) error {
	var invoke func(index int, currentCtx any) error
	invoke = func(index int, currentCtx any) error {
		if index >= len(handlers) {
			return nil
		}
		current := handlers[index]
		return current(currentCtx, func(nextCtx any) error {
			return invoke(index+1, nextCtx)
		})
	}
	return invoke(0, ctx)
}

func collect(method, path string, final ContextFunc) []Middleware {
	mu.RLock()
	defer mu.RUnlock()

	key := routeKey(method, path)
	size := len(global)
	if route := routes[key]; len(route) > 0 {
		size += len(route)
	}
	if final != nil {
		size++
	}

	handlers := make([]Middleware, 0, size)
	if len(global) > 0 {
		handlers = append(handlers, global...)
	}
	if route := routes[key]; len(route) > 0 {
		handlers = append(handlers, route...)
	}
	if final != nil {
		handlers = append(handlers, wrapContextFunc(final))
	}
	return handlers
}

func wrapContextFunc(fn ContextFunc) Middleware {
	return func(ctx any, next Next) error {
		if err := fn(ctx); err != nil {
			return err
		}
		if next != nil {
			return next(ctx)
		}
		return nil
	}
}

func routeKey(method, path string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	return method + " " + path
}
