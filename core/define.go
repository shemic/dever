package core

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
)

// Handler 定义服务层函数签名。
// params: 调用时传入的参数，约定为可变键值对。
// 返回业务数据或错误，错误需要由上层决定如何处理。
type Handler func(params map[string]any) (any, error)

type binding struct {
	handler Handler

	fastSrv func(*server.Context) (any, error)

	modelFn     func() *orm.Model
	modelOnce   sync.Once
	modelCached *orm.Model
}

var (
	registry atomic.Value // 存放 map[string]*binding，读操作无锁，注册时复制写
	regMutex sync.Mutex

	// ErrNotFound 表示服务未注册。
	ErrNotFound = errors.New("core: service not found")
	// ErrInvalidName 表示服务名称非法。
	ErrInvalidName = errors.New("core: invalid service name")
)

func init() {
	registry.Store(make(map[string]*binding))
}
