package load

import (
	"fmt"
	"reflect"
	"strings"

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
			panic("load: service name cannot be empty")
		}
		if raw == nil {
			panic(fmt.Sprintf("load: handler for %s is nil", name))
		}
		if original, duplicated := originals[key]; duplicated {
			panic(fmt.Sprintf("load: duplicated service name: %s conflicts with %s", name, original))
		}
		originals[key] = name
		prepared[key] = adaptHandler(name, raw)
	}

	regMutex.Lock()
	defer regMutex.Unlock()

	current := registry.Load().(map[string]*binding)
	for key, original := range originals {
		if _, exists := current[key]; exists {
			panic(fmt.Sprintf("load: service already registered: %s", original))
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

func adaptHandler(name string, fn any) *binding {
	isModelBinding := isModelBindingName(name)
	switch h := fn.(type) {
	case nil:
		panic("load: handler is nil")
	case Handler:
		return &binding{handler: h}
	case func() (any, error):
		binding := &binding{
			fastZero: h,
			handler: func(map[string]any) (any, error) {
				return h()
			},
		}
		return binding
	case func() any:
		binding := &binding{
			fastZero: func() (any, error) {
				return h(), nil
			},
			handler: func(map[string]any) (any, error) {
				return h(), nil
			},
		}
		if isModelBinding {
			binding.modelFn = h
		}
		return binding
	case func() error:
		return &binding{
			fastZero: func() (any, error) {
				return nil, h()
			},
			handler: func(map[string]any) (any, error) {
				return nil, h()
			},
		}
	case func():
		return &binding{
			fastZero: func() (any, error) {
				h()
				return nil, nil
			},
			handler: func(map[string]any) (any, error) {
				h()
				return nil, nil
			},
		}
	case func(*server.Context) (any, error):
		return &binding{
			fastSrv: h,
			handler: func(params map[string]any) (any, error) {
				var ctx *server.Context
				if v, ok := params["_srv_ctx"].(*server.Context); ok {
					ctx = v
				}
				return h(ctx)
			},
		}
	case func(*server.Context) any:
		return &binding{
			fastSrv: func(ctx *server.Context) (any, error) {
				return h(ctx), nil
			},
			handler: func(params map[string]any) (any, error) {
				var ctx *server.Context
				if v, ok := params["_srv_ctx"].(*server.Context); ok {
					ctx = v
				}
				return h(ctx), nil
			},
		}
	case func(*server.Context) error:
		return &binding{
			fastSrv: func(ctx *server.Context) (any, error) {
				return nil, h(ctx)
			},
			handler: func(params map[string]any) (any, error) {
				var ctx *server.Context
				if v, ok := params["_srv_ctx"].(*server.Context); ok {
					ctx = v
				}
				return nil, h(ctx)
			},
		}
	case func(*server.Context, []any) any:
		return &binding{
			handler: func(params map[string]any) (any, error) {
				var ctx *server.Context
				if v, ok := params["_srv_ctx"].(*server.Context); ok {
					ctx = v
				}
				var arr []any
				if v, ok := params["_params"].([]any); ok {
					arr = v
				}
				return h(ctx, arr), nil
			},
			provider: h,
		}
	default:
		if v := reflect.ValueOf(fn); v.Kind() == reflect.Func && v.Type().NumIn() == 0 && v.Type().NumOut() == 1 {
			binding := &binding{
				fastZero: func() (any, error) {
					return v.Call(nil)[0].Interface(), nil
				},
				handler: func(map[string]any) (any, error) {
					return v.Call(nil)[0].Interface(), nil
				},
			}
			if isModelBinding {
				binding.modelFn = func() any {
					return v.Call(nil)[0].Interface()
				}
			}
			return binding
		}
		panic(fmt.Sprintf("load: unsupported handler type %T", fn))
	}
}

func isModelBindingName(name string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), "model")
}

func normalize(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	for i := 0; i < len(trimmed); i++ {
		b := trimmed[i]
		if b >= 'A' && b <= 'Z' {
			return strings.ToLower(trimmed)
		}
	}
	return trimmed
}
