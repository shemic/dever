package server

// 上下文对象池
import "sync"

var ctxPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

func GetContext(raw any) *Context {
	ctx := ctxPool.Get().(*Context)
	ctx.Raw = raw
	return ctx
}

func ReleaseContext(c *Context) {
	c.Raw = nil
	ctxPool.Put(c)
}
