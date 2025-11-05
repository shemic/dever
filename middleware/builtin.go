package middleware

import (
	"fmt"
	"runtime/debug"
	"time"

	dlog "github.com/shemic/dever/log"
	"github.com/shemic/dever/server"
)

var (
	defaultRecover = Recover()
	defaultLog     = Log()
)

// Init 提供框架默认中间件组合：Recover + Log。
func Init() Middleware {
	return func(ctx any, next Next) error {
		return defaultRecover(ctx, func(nextCtx any) error {
			return defaultLog(nextCtx, next)
		})
	}
}

// Recover 捕获 panic 并返回统一错误响应。
func Recover() Middleware {
	return func(ctx any, next Next) (err error) {
		c := extractContext(ctx)
		defer func() {
			if r := recover(); r != nil {
				dlog.Error().Printf("[recover] method=%s path=%s panic=%v\n%s",
					requestMethod(c), requestPath(c), r, debug.Stack())
				if c != nil {
					_ = c.JSON(map[string]any{"error": fmt.Sprintf("panic: %v", r)})
				}
				err = nil
			}
		}()
		if next != nil {
			return next(ctx)
		}
		return nil
	}
}

// Log 输出基础请求日志，同时记录错误。
func Log() Middleware {
	return func(ctx any, next Next) (err error) {
		c := extractContext(ctx)
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			if err != nil {
				dlog.Error().Printf("[error] method=%s path=%s duration=%s err=%v",
					requestMethod(c), requestPath(c), duration, err)
			}
			dlog.Access().Printf("[access] method=%s path=%s duration=%s err=%v",
				requestMethod(c), requestPath(c), duration, err)
		}()
		if next != nil {
			err = next(ctx)
		}
		return err
	}
}

func extractContext(ctx any) *server.Context {
	if c, ok := ctx.(*server.Context); ok {
		return c
	}
	return nil
}

func requestMethod(c *server.Context) string {
	if c == nil || c.Raw == nil {
		return ""
	}
	type methoder interface{ Method() string }
	if m, ok := c.Raw.(methoder); ok {
		return m.Method()
	}
	if r, ok := c.Raw.(interface{ Request() any }); ok {
		if req := r.Request(); req != nil {
			if m, ok := req.(interface{ Method() string }); ok {
				return m.Method()
			}
		}
	}
	return ""
}

func requestPath(c *server.Context) string {
	if c == nil || c.Raw == nil {
		return ""
	}
	type router interface{ Path() string }
	if r, ok := c.Raw.(router); ok {
		return r.Path()
	}
	if a, ok := c.Raw.(interface{ OriginalURL() string }); ok {
		return a.OriginalURL()
	}
	return ""
}
