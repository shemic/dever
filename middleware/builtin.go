package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	dlog "github.com/shemic/dever/log"
	"github.com/shemic/dever/observe"
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
				method, path, traceID, spanID := requestLogFields(c)
				fields := requestSourceFields(c)
				fields["trace_id"] = traceID
				fields["span_id"] = spanID
				fields["method"] = method
				fields["path"] = path
				fields["error"] = fmt.Sprintf("%v", r)
				dlog.ErrorFields("http_recover", "panic recovered", fields)
				if c != nil {
					_ = c.Error(fmt.Sprintf("%v", r), http.StatusInternalServerError)
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
		method, path, _, _ := requestLogFields(c)
		sourceFields := requestSourceFields(c)
		observeCtx := observeBaseContext(c)
		if c != nil {
			attrs := map[string]any{
				"http.method": method,
				"http.path":   path,
			}
			applyObserveRequestSourceAttrs(attrs, sourceFields)
			observeCtx, span := observe.Start(observeCtx, observe.KindRequest, method+" "+path, attrs)
			c.SetContext(observeCtx)
			start := time.Now()
			defer func() {
				if r := recover(); r != nil {
					span.RecordError(fmt.Errorf("%v", r))
					span.End()
					panic(r)
				}

				duration := time.Since(start)
				traceID := observe.TraceID(observeCtx)
				spanID := observe.SpanID(observeCtx)
				if err != nil {
					span.RecordError(err)
					fields := dlog.Fields{
						"trace_id": traceID,
						"span_id":  spanID,
						"method":   method,
						"path":     path,
						"duration": duration.String(),
						"error":    dlog.ErrorValue(err),
					}
					mergeLogFields(fields, sourceFields)
					dlog.ErrorFields("http_error", "request failed", fields)
				}
				fields := dlog.Fields{
					"trace_id": traceID,
					"span_id":  spanID,
					"method":   method,
					"path":     path,
					"duration": duration.String(),
					"error":    dlog.ErrorValue(err),
				}
				mergeLogFields(fields, sourceFields)
				dlog.AccessFields("http_access", "request completed", fields)
				span.End()
			}()
			if next != nil {
				err = next(ctx)
			}
			return err
		}

		start := time.Now()
		defer func() {
			duration := time.Since(start)
			if err != nil {
				fields := dlog.Fields{
					"method":   method,
					"path":     path,
					"duration": duration.String(),
					"error":    dlog.ErrorValue(err),
				}
				mergeLogFields(fields, sourceFields)
				dlog.ErrorFields("http_error", "request failed", fields)
			}
			fields := dlog.Fields{
				"method":   method,
				"path":     path,
				"duration": duration.String(),
				"error":    dlog.ErrorValue(err),
			}
			mergeLogFields(fields, sourceFields)
			dlog.AccessFields("http_access", "request completed", fields)
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

func observeBaseContext(c *server.Context) context.Context {
	if c == nil {
		return nil
	}
	return c.Context()
}

func requestLogFields(c *server.Context) (string, string, string, string) {
	if c == nil {
		return "", "", "", ""
	}
	ctx := c.Context()
	return c.Method(), c.Path(), observe.TraceID(ctx), observe.SpanID(ctx)
}

func requestSourceFields(c *server.Context) dlog.Fields {
	if c == nil {
		return nil
	}
	fields := dlog.Fields{}
	if origin := c.Header("Origin"); origin != "" {
		fields["origin"] = origin
	}
	if referer := c.Header("Referer"); referer != "" {
		fields["referer"] = referer
	}
	if clientPage := c.Header("X-Client-Page"); clientPage != "" {
		fields["client_page"] = clientPage
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func mergeLogFields(target dlog.Fields, extra dlog.Fields) {
	for key, value := range extra {
		target[key] = value
	}
}

func applyObserveRequestSourceAttrs(target map[string]any, fields dlog.Fields) {
	if len(fields) == 0 {
		return
	}
	if origin, ok := fields["origin"]; ok {
		target["http.origin"] = origin
	}
	if referer, ok := fields["referer"]; ok {
		target["http.referer"] = referer
	}
	if clientPage, ok := fields["client_page"]; ok {
		target["http.client_page"] = clientPage
	}
}
