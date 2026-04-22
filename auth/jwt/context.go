package jwt

import (
	"context"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/shemic/dever/util"
)

type authContextKey struct{}

type authState struct {
	activeScheme string
	claims       map[string]jwtlib.MapClaims
}

func ActiveScheme(ctx context.Context) string {
	return authStateFrom(ctx).activeScheme
}

func Claims(ctx context.Context, scheme ...string) jwtlib.MapClaims {
	state := authStateFrom(ctx)
	name := state.activeScheme
	if len(scheme) > 0 && scheme[0] != "" {
		name = scheme[0]
	}
	if name == "" || state.claims == nil {
		return nil
	}
	claims, ok := state.claims[name]
	if !ok {
		return nil
	}
	return cloneClaims(claims)
}

func ActiveString(ctx context.Context, keys ...string) (string, bool) {
	state := authStateFrom(ctx)
	return claimString(state, state.activeScheme, keys...)
}

func ActiveInt64(ctx context.Context, keys ...string) (int64, bool) {
	state := authStateFrom(ctx)
	return claimInt64(state, state.activeScheme, keys...)
}

func String(ctx context.Context, scheme string, keys ...string) (string, bool) {
	return claimString(authStateFrom(ctx), scheme, keys...)
}

func Int64(ctx context.Context, scheme string, keys ...string) (int64, bool) {
	return claimInt64(authStateFrom(ctx), scheme, keys...)
}

func authStateFrom(ctx context.Context) authState {
	if ctx == nil {
		return authState{}
	}
	if value, ok := ctx.Value(authContextKey{}).(authState); ok {
		return value
	}
	return authState{}
}

func withClaims(ctx context.Context, scheme string, claims jwtlib.MapClaims) context.Context {
	state := authStateFrom(ctx)
	next := authState{
		activeScheme: scheme,
		claims:       make(map[string]jwtlib.MapClaims, len(state.claims)+1),
	}
	for name, item := range state.claims {
		next.claims[name] = cloneClaims(item)
	}
	next.claims[scheme] = cloneClaims(claims)
	return context.WithValue(ctx, authContextKey{}, next)
}

func claimString(state authState, scheme string, keys ...string) (string, bool) {
	scheme = normalizeSchemeName(state, scheme)
	if scheme == "" || state.claims == nil {
		return "", false
	}
	claims, ok := state.claims[scheme]
	if !ok {
		return "", false
	}
	keys = resolveClaimKeys(scheme, keys)
	for _, key := range keys {
		if value, ok := claims[key]; ok {
			text := util.ToKeyString(value)
			if text != "" {
				return text, true
			}
		}
	}
	return "", false
}

func claimInt64(state authState, scheme string, keys ...string) (int64, bool) {
	value, ok := claimString(state, scheme, keys...)
	if !ok {
		return 0, false
	}
	return util.ParseInt64(value)
}

func normalizeSchemeName(state authState, scheme string) string {
	if scheme = util.ToStringTrimmed(scheme); scheme != "" {
		return scheme
	}
	return state.activeScheme
}

func resolveClaimKeys(scheme string, keys []string) []string {
	if len(keys) > 0 {
		return keys
	}
	if item, ok := lookupScheme(scheme); ok {
		return append([]string(nil), item.claimKeys...)
	}
	return nil
}

func cloneClaims(claims jwtlib.MapClaims) jwtlib.MapClaims {
	if len(claims) == 0 {
		return nil
	}
	cloned := make(jwtlib.MapClaims, len(claims))
	for key, value := range claims {
		cloned[key] = value
	}
	return cloned
}
