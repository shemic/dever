package middleware

import (
	"fmt"
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
				dlog.Error().Printf("[recover] method=%s path=%s panic=%v",
					c.Method(), c.Path(), r)
				if c != nil {
					_ = c.Error(fmt.Sprintf("%v", r))
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
					c.Method(), c.Path(), duration, err)
			}
			dlog.Access().Printf("[access] method=%s path=%s duration=%s err=%v",
				c.Method(), c.Path(), duration, err)
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
