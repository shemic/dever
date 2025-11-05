package core

import (
	"context"
	"fmt"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
)

// LoadService 查找并执行已注册的服务，出现错误会直接 panic。
func LoadService(name string, args ...any) any {
	binding := mustBinding(name)
	if len(args) == 1 {
		if srv, ok := args[0].(*server.Context); ok && binding.fastSrv != nil {
			result, err := binding.fastSrv(srv)
			if err != nil {
				panic(err)
			}
			return result
		}
	}
	return invokeBinding(binding, args...)
}

// Load 保留向后兼容的调用入口。
func Load(name string, args ...any) any {
	return invoke(name, args...)
}

// LoadModel 加载注册的模型函数并断言返回 *orm.Model。
func LoadModel(name string, args ...any) *orm.Model {
	binding := mustBinding(name)
	if len(args) == 0 && binding.modelFn != nil {
		binding.modelOnce.Do(func() {
			binding.modelCached = binding.modelFn()
		})
		if binding.modelCached == nil {
			panic(fmt.Sprintf("core: model %s returned nil", name))
		}
		return binding.modelCached
	}

	result := invokeBinding(binding, args...)
	if result == nil {
		panic(fmt.Sprintf("core: model %s returned nil", name))
	}
	model, ok := result.(*orm.Model)
	if !ok {
		panic(fmt.Sprintf("core: model %s returns %T, want *orm.Model", name, result))
	}
	return model
}

func invoke(name string, args ...any) any {
	binding := mustBinding(name)
	return invokeBinding(binding, args...)
}

func invokeBinding(binding *binding, args ...any) any {
	payload := assemblePayload(args...)
	result, err := binding.handler(payload)
	if err != nil {
		panic(err)
	}
	return result
}

func mustBinding(name string) *binding {
	key := normalize(name)
	if key == "" {
		panic(ErrInvalidName)
	}
	current := registry.Load().(map[string]*binding)
	handler, ok := current[key]
	if !ok {
		panic(fmt.Errorf("%w: %s", ErrNotFound, name))
	}
	return handler
}

func assemblePayload(args ...any) map[string]any {
	var (
		ctx context.Context
		srv *server.Context
		payload map[string]any
	)

	for _, arg := range args {
		switch v := arg.(type) {
		case nil:
			continue
		case context.Context:
			ctx = v
		case *server.Context:
			srv = v
		case map[string]any:
			if len(v) == 0 {
				continue
			}
			if payload == nil {
				payload = make(map[string]any, len(v)+4)
			}
			for k, val := range v {
				payload[k] = val
			}
		case Arg:
			if v.Key == "" {
				panic("core: argument key cannot be empty")
			}
			if payload == nil {
				payload = make(map[string]any, 8)
			}
			payload[v.Key] = v.Value
		default:
			panic(fmt.Sprintf("core: unsupported load argument type %T", arg))
		}
	}

	if payload == nil {
		payload = make(map[string]any)
	}
	if srv != nil {
		payload["_srv_ctx"] = srv
		if ctx == nil {
			ctx = srv.Context()
		}
	}
	if ctx != nil {
		payload["_ctx"] = ctx
	}
	return payload
}
