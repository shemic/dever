package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
)

// Register 在运行时注册服务处理函数。重复注册会 panic。
func Register(name string, handler any) {
	RegisterMany(map[string]any{name: handler})
}

// RegisterMany 批量注册服务，避免重复复制注册表。
func RegisterMany(handlers map[string]any) {
	if len(handlers) == 0 {
		return
	}

	prepared := make(map[string]*binding, len(handlers))
	originals := make(map[string]string, len(handlers))

	for name, raw := range handlers {
		key := normalize(name)
		if key == "" {
			panic("core: service name cannot be empty")
		}
		if raw == nil {
			panic(fmt.Sprintf("core: handler for %s is nil", name))
		}
		if original, duplicated := originals[key]; duplicated {
			panic(fmt.Sprintf("core: duplicated service name: %s conflicts with %s", name, original))
		}
		originals[key] = name
		prepared[key] = adaptHandler(raw)
	}

	regMutex.Lock()
	defer regMutex.Unlock()

	current := registry.Load().(map[string]*binding)
	for key, original := range originals {
		if _, exists := current[key]; exists {
			panic(fmt.Sprintf("core: service already registered: %s", original))
		}
	}

	next := make(map[string]*binding, len(current)+len(prepared))
	for k, v := range current {
		next[k] = v
	}
	for key, handler := range prepared {
		next[key] = handler
	}

	registry.Store(next)
}

func adaptHandler(fn any) *binding {
	switch h := fn.(type) {
	case nil:
		panic("core: handler is nil")
	case Handler:
		return &binding{handler: h}
	case func(map[string]any) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				return h(cloneParams(params))
			},
		}
	case func(map[string]any) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				return h(cloneParams(params)), nil
			},
		}
	case func(map[string]any):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				h(cloneParams(params))
				return nil, nil
			},
		}
	case func(context.Context, map[string]any) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, clone := splitParams(params)
				return h(ctx, clone)
			},
		}
	case func(context.Context, map[string]any) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, clone := splitParams(params)
				return h(ctx, clone), nil
			},
		}
	case func(context.Context, map[string]any):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, clone := splitParams(params)
				h(ctx, clone)
				return nil, nil
			},
		}
	case func(context.Context) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, _ := splitParams(params)
				return h(ctx)
			},
		}
	case func(context.Context) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, _ := splitParams(params)
				return h(ctx), nil
			},
		}
	case func(context.Context):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, _, _ := splitParams(params)
				h(ctx)
				return nil, nil
			},
		}
	case func(*server.Context, map[string]any) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, clone := splitParams(params)
				return h(srv, clone)
			},
			fastSrv: func(srv *server.Context) (any, error) {
				return h(srv, make(map[string]any))
			},
		}
	case func(*server.Context, map[string]any) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, clone := splitParams(params)
				return h(srv, clone), nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				return h(srv, make(map[string]any)), nil
			},
		}
	case func(*server.Context, map[string]any):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, clone := splitParams(params)
				h(srv, clone)
				return nil, nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				h(srv, make(map[string]any))
				return nil, nil
			},
		}
	case func(*server.Context) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, _ := splitParams(params)
				return h(srv)
			},
			fastSrv: func(srv *server.Context) (any, error) {
				return h(srv)
			},
		}
	case func(*server.Context) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, _ := splitParams(params)
				return h(srv), nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				return h(srv), nil
			},
		}
	case func(*server.Context):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				_, srv, _ := splitParams(params)
				h(srv)
				return nil, nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				h(srv)
				return nil, nil
			},
		}
	case func(*server.Context, context.Context, map[string]any) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, clone := splitParams(params)
				return h(srv, ctx, clone)
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				return h(srv, ctx, make(map[string]any))
			},
		}
	case func(*server.Context, context.Context, map[string]any) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, clone := splitParams(params)
				return h(srv, ctx, clone), nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				return h(srv, ctx, make(map[string]any)), nil
			},
		}
	case func(*server.Context, context.Context, map[string]any):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, clone := splitParams(params)
				h(srv, ctx, clone)
				return nil, nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				h(srv, ctx, make(map[string]any))
				return nil, nil
			},
		}
	case func(*server.Context, context.Context) (any, error):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, _ := splitParams(params)
				return h(srv, ctx)
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				return h(srv, ctx)
			},
		}
	case func(*server.Context, context.Context) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, _ := splitParams(params)
				return h(srv, ctx), nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				return h(srv, ctx), nil
			},
		}
	case func(*server.Context, context.Context):
		return &binding{
			handler: func(params map[string]any) (any, error) {
				ctx, srv, _ := splitParams(params)
				h(srv, ctx)
				return nil, nil
			},
			fastSrv: func(srv *server.Context) (any, error) {
				ctx := context.Background()
				if srv != nil {
					if c := srv.Context(); c != nil {
						ctx = c
					}
				}
				h(srv, ctx)
				return nil, nil
			},
		}
	case func() (any, error):
		return &binding{
			handler: func(map[string]any) (any, error) {
				return h()
			},
		}
	case func() *orm.Model:
		return &binding{
			handler: func(map[string]any) (any, error) {
				return h(), nil
			},
			modelFn: h,
		}
	case func() any:
		return &binding{
			handler: func(map[string]any) (any, error) {
				return h(), nil
			},
		}
	case func():
		return &binding{
			handler: func(map[string]any) (any, error) {
				h()
				return nil, nil
			},
		}
	default:
		panic(fmt.Sprintf("core: unsupported handler type %T", fn))
	}
}

func normalize(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func splitParams(params map[string]any) (context.Context, *server.Context, map[string]any) {
	ctx := context.Background()
	var srv *server.Context
	if params == nil {
		return ctx, nil, map[string]any{}
	}
	clone := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "_ctx":
			if c, ok := v.(context.Context); ok && c != nil {
				ctx = c
			}
		case "_srv_ctx":
			if sc, ok := v.(*server.Context); ok && sc != nil {
				srv = sc
			}
		default:
			clone[k] = v
		}
	}
	return ctx, srv, clone
}

func cloneParams(params map[string]any) map[string]any {
	_, _, clone := splitParams(params)
	return clone
}
