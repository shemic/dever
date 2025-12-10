package load

import (
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
			panic("load: service name cannot be empty")
		}
		if raw == nil {
			panic(fmt.Sprintf("load: handler for %s is nil", name))
		}
		if original, duplicated := originals[key]; duplicated {
			panic(fmt.Sprintf("load: duplicated service name: %s conflicts with %s", name, original))
		}
		originals[key] = name
		prepared[key] = adaptHandler(raw)
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

func adaptHandler(fn any) *binding {
	switch h := fn.(type) {
	case nil:
		panic("load: handler is nil")
	case Handler:
		return &binding{handler: h}
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
		panic(fmt.Sprintf("load: unsupported handler type %T", fn))
	}
}

func normalize(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
