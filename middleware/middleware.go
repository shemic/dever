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
	mu         sync.RWMutex
	global     []Middleware
	routes     = map[string][]Middleware{}
	chainCache = map[string][]Middleware{}
)

// UseGlobal 注册全局中间件，按注册顺序执行。
func UseGlobal(middlewares ...Middleware) {
	if len(middlewares) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	global = append(global, middlewares...)
	clear(chainCache)
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
	clear(chainCache)
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
	handlers := collect(method, path)
	return runChain(ctx, handlers, final)
}

// Compile 在注册阶段构建固定执行链，避免请求期重复拼接 next 闭包。
func Compile(method, path string, final ContextFunc) ContextFunc {
	return compileChain(collect(method, path), final)
}

func compileChain(handlers []Middleware, final ContextFunc) ContextFunc {
	if len(handlers) == 0 {
		if final == nil {
			return func(any) error { return nil }
		}
		return final
	}

	next := final
	if next == nil {
		next = func(any) error { return nil }
	}

	for index := len(handlers) - 1; index >= 0; index-- {
		current := handlers[index]
		downstream := next
		next = func(ctx any) error {
			return current(ctx, func(nextCtx any) error {
				return downstream(nextCtx)
			})
		}
	}

	return next
}

func runChain(ctx any, handlers []Middleware, final ContextFunc) error {
	var invoke func(index int, currentCtx any) error
	invoke = func(index int, currentCtx any) error {
		if index >= len(handlers) {
			if final != nil {
				return final(currentCtx)
			}
			return nil
		}
		current := handlers[index]
		return current(currentCtx, func(nextCtx any) error {
			return invoke(index+1, nextCtx)
		})
	}
	return invoke(0, ctx)
}

func collect(method, path string) []Middleware {
	key := routeKey(method, path)
	mu.RLock()
	if cached, ok := chainCache[key]; ok {
		mu.RUnlock()
		return cached
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if cached, ok := chainCache[key]; ok {
		return cached
	}

	size := len(global)
	if route := routes[key]; len(route) > 0 {
		size += len(route)
	}
	handlers := make([]Middleware, 0, size)
	if len(global) > 0 {
		handlers = append(handlers, global...)
	}
	if route := routes[key]; len(route) > 0 {
		handlers = append(handlers, route...)
	}
	chainCache[key] = handlers
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
