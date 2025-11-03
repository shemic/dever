package server

import "context"

type HandlerFunc func(*Context) error

type Server interface {
	Use(middlewares ...HandlerFunc)
	Group(path string) Server
	Get(path string, handler HandlerFunc)
	Post(path string, handler HandlerFunc)
	Put(path string, handler HandlerFunc)
	Delete(path string, handler HandlerFunc)
	Handle(method, path string, handler HandlerFunc)
	Run(addr string) error
	Shutdown(ctx context.Context) error
}
