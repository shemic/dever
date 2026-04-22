package jwt

import (
	"errors"
	"net/http"
	"strings"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/shemic/dever/middleware"
	"github.com/shemic/dever/server"
)

type Options struct {
	Allow          func(*server.Context) bool
	AllowMissing   func(*server.Context) bool
	OnUnauthorized func(*server.Context, string) error
	PublicPaths    []string
}

func UseConfigured(options Options) middleware.ContextFunc {
	return func(ctx any) error {
		c, ok := ctx.(*server.Context)
		if !ok || c == nil {
			return nil
		}
		if shouldBypass(c, options.Allow) {
			return nil
		}

		runtime := currentState()
		if len(runtime.guards) > 0 {
			guard, ok := matchGuard(runtime.guards, c.Path())
			if !ok || guard.isPublic(c.Path()) {
				return nil
			}
			scheme, ok := runtime.schemes[guard.scheme]
			if !ok {
				return unauthorized(c, options, "未配置认证方案")
			}
			return verify(c, scheme, options)
		}

		if isPublicPath(c.Path(), options.PublicPaths) {
			return nil
		}
		if runtime.primary == "" {
			return nil
		}
		scheme, ok := runtime.schemes[runtime.primary]
		if !ok {
			return unauthorized(c, options, "未配置认证方案")
		}
		return verify(c, scheme, options)
	}
}

func Require(name string, options Options) middleware.ContextFunc {
	return func(ctx any) error {
		c, ok := ctx.(*server.Context)
		if !ok || c == nil {
			return nil
		}
		if shouldBypass(c, options.Allow) || isPublicPath(c.Path(), options.PublicPaths) {
			return nil
		}
		scheme, ok := lookupScheme(name)
		if !ok {
			return unauthorized(c, options, "未配置认证方案")
		}
		return verify(c, scheme, options)
	}
}

func verify(c *server.Context, scheme Scheme, options Options) error {
	tokenText := extractToken(c, scheme.header, scheme.prefix)
	if tokenText == "" {
		if options.AllowMissing != nil && options.AllowMissing(c) {
			return nil
		}
		return unauthorized(c, options, "缺少认证信息")
	}

	claims, err := parseToken(tokenText, scheme)
	if err != nil {
		return unauthorized(c, options, "无效的令牌")
	}

	ctx := withClaims(c.Context(), scheme.name, claims)
	c.SetContext(ctx)
	return nil
}

func parseToken(tokenText string, scheme Scheme) (jwtlib.MapClaims, error) {
	claims := jwtlib.MapClaims{}
	token, err := jwtlib.ParseWithClaims(tokenText, claims, func(token *jwtlib.Token) (any, error) {
		if token.Method == nil || token.Method.Alg() != scheme.alg {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(scheme.secret), nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, errors.New("无效的令牌")
	}
	return cloneClaims(claims), nil
}

func extractToken(c *server.Context, header, prefix string) string {
	if c == nil {
		return ""
	}
	value := c.Header(header)
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return value
	}
	lower := strings.ToLower(value)
	expected := strings.ToLower(prefix) + " "
	if strings.HasPrefix(lower, expected) {
		return strings.TrimSpace(value[len(expected):])
	}
	return value
}

func unauthorized(c *server.Context, options Options, msg string) error {
	if options.OnUnauthorized != nil {
		return options.OnUnauthorized(c, msg)
	}
	if c != nil {
		_ = c.Error(msg, http.StatusUnauthorized)
	}
	panic(server.Abort{Err: errors.New(msg)})
}

func shouldBypass(c *server.Context, allow func(*server.Context) bool) bool {
	if c == nil {
		return true
	}
	if c.Method() == http.MethodOptions {
		return true
	}
	return allow != nil && allow(c)
}

func matchGuard(guards []Guard, path string) (Guard, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Guard{}, false
	}
	for _, guard := range guards {
		for _, prefix := range guard.prefixes {
			if prefix == "" || strings.HasPrefix(path, prefix) {
				return guard, true
			}
		}
	}
	return Guard{}, false
}

func isPublicPath(path string, publicPaths []string) bool {
	if path == "" || len(publicPaths) == 0 {
		return false
	}
	path = strings.TrimSpace(path)
	for _, item := range publicPaths {
		if path == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

func (g Guard) isPublic(path string) bool {
	if len(g.publicPaths) == 0 {
		return false
	}
	_, ok := g.publicPaths[strings.TrimSpace(path)]
	return ok
}
