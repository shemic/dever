package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shemic/dever/middleware"
	"github.com/shemic/dever/server"
)

func init() {
	server.RegisterJSONAdapter(func(raw any, status int, data any) (bool, error) {
		ctx, ok := raw.(*fiber.Ctx)
		if !ok {
			return false, nil
		}
		ctx.Status(status)
		return true, ctx.JSON(data)
	})
}

// fiberServer 基于 Fiber 实现统一的 Server 接口，负责路由与中间件注册。
type fiberServer struct {
	app    *fiber.App
	router fiber.Router
}

// New 创建一个 Fiber 版本的 Server，并加载通过 server.Auto 注册的路由配置。
func New() server.Server {
	return NewWithConfig(fiber.Config{})
}

// NewWithConfig 创建一个自定义配置的 Fiber Server，并加载通过 server.Auto 注册的路由配置。
func NewWithConfig(conf fiber.Config) server.Server {
	app := fiber.New(conf)
	srv := &fiberServer{
		app:    app,
		router: app,
	}
	if !(conf.Prefork && !fiber.IsChild()) {
		server.LoadAll(srv)
	}
	return srv
}

func (s *fiberServer) Get(path string, handler server.HandlerFunc) {
	s.router.Get(path, wrap(handler, http.MethodGet, path))
}

func (s *fiberServer) Post(path string, handler server.HandlerFunc) {
	s.router.Post(path, wrap(handler, http.MethodPost, path))
}

func (s *fiberServer) Put(path string, handler server.HandlerFunc) {
	s.router.Put(path, wrap(handler, http.MethodPut, path))
}

func (s *fiberServer) Delete(path string, handler server.HandlerFunc) {
	s.router.Delete(path, wrap(handler, http.MethodDelete, path))
}

func (s *fiberServer) Handle(method, path string, handler server.HandlerFunc) {
	s.router.Add(method, path, wrap(handler, method, path))
}

func (s *fiberServer) Group(prefix string) server.Server {
	group := s.router.Group(prefix)
	return &fiberServer{
		app:    s.app,
		router: group,
	}
}

func (s *fiberServer) Use(middlewares ...server.HandlerFunc) {
	for _, m := range middlewares {
		s.router.Use(wrap(m, "", ""))
	}
}

func (s *fiberServer) Run(addr string) error {
	return s.app.Listen(addr)
}

// wrap 将统一的 HandlerFunc 适配为 Fiber 的处理函数，并复用 Context 对象池。
func wrap(fn server.HandlerFunc, method, path string) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		ctx := server.GetContext(c)
		defer server.ReleaseContext(ctx)
		defer func() {
			if r := recover(); r != nil {
				if abort, ok := r.(server.Abort); ok {
					_ = abort // response 已在 Context 中输出
					err = nil
					return
				}
				panic(r)
			}
		}()
		return middleware.Execute(ctx, method, path, func(_ any) error {
			return fn(ctx)
		})
	}
}

func (s *fiberServer) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return s.app.Shutdown()
	}

	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return ctx.Err()
		}
		if graceful, ok := interface{}(s.app).(interface {
			ShutdownWithTimeout(time.Duration) error
		}); ok {
			return awaitWithContext(ctx, func() error {
				return graceful.ShutdownWithTimeout(timeout)
			})
		}
	}
	return awaitWithContext(ctx, s.app.Shutdown)
}

func awaitWithContext(ctx context.Context, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
