package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Handler 定义服务层函数签名。
// params: 调用时传入的参数，约定为可变键值对。
// 返回业务数据或错误，错误需要由上层决定如何处理。
type Handler func(params map[string]any) (any, error)

var (
	registry atomic.Value // 存放 map[string]Handler，读操作无锁，注册时复制写
	regMutex sync.Mutex

	// ErrNotFound 表示服务未注册。
	ErrNotFound = errors.New("core: service not found")
	// ErrInvalidName 表示服务名称非法。
	ErrInvalidName = errors.New("core: invalid service name")
)

func init() {
	registry.Store(make(map[string]Handler))
}

// Register 在运行时注册服务处理函数。重复注册会 panic。
func Register(name string, handler Handler) {
	key := normalize(name)
	if key == "" {
		panic("core: service name cannot be empty")
	}
	if handler == nil {
		panic(fmt.Sprintf("core: handler for %s is nil", name))
	}

	regMutex.Lock()
	defer regMutex.Unlock()

	current := registry.Load().(map[string]Handler)
	if _, exists := current[key]; exists {
		panic(fmt.Sprintf("core: service already registered: %s", name))
	}

	next := make(map[string]Handler, len(current)+1)
	for k, v := range current {
		next[k] = v
	}
	next[key] = handler
	registry.Store(next)
}

// Load 查找并执行已注册的服务。未找到或执行失败返回错误。
func Load(name string, params map[string]any) (any, error) {
	key := normalize(name)
	if key == "" {
		return nil, ErrInvalidName
	}

	current := registry.Load().(map[string]Handler)
	handler, ok := current[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	if params == nil {
		params = make(map[string]any)
	}
	return handler(params)
}

func normalize(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
