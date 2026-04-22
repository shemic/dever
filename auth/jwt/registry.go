package jwt

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/shemic/dever/config"
)

type Scheme struct {
	name      string
	alg       string
	secret    string
	header    string
	prefix    string
	claimKeys []string
}

type Guard struct {
	scheme      string
	prefixes    []string
	publicPaths map[string]struct{}
}

type runtimeState struct {
	schemes map[string]Scheme
	guards  []Guard
	primary string
}

type Signer struct {
	Algorithm string
	Secret    string
	ClaimKeys []string
}

var (
	stateMu sync.RWMutex
	state   = runtimeState{
		schemes: map[string]Scheme{},
	}
)

func Configure(authCfg config.Auth) error {
	next, err := buildRuntimeState(authCfg)
	if err != nil {
		return err
	}

	stateMu.Lock()
	state = next
	stateMu.Unlock()
	return nil
}

func ResolveSigner(authCfg config.Auth, preferred ...string) (Signer, error) {
	runtime, err := buildRuntimeState(authCfg)
	if err != nil {
		return Signer{}, err
	}
	for _, name := range preferred {
		if scheme, ok := runtime.schemes[strings.TrimSpace(name)]; ok {
			return Signer{
				Algorithm: scheme.alg,
				Secret:    scheme.secret,
				ClaimKeys: append([]string(nil), scheme.claimKeys...),
			}, nil
		}
	}
	if runtime.primary != "" {
		if scheme, ok := runtime.schemes[runtime.primary]; ok {
			return Signer{
				Algorithm: scheme.alg,
				Secret:    scheme.secret,
				ClaimKeys: append([]string(nil), scheme.claimKeys...),
			}, nil
		}
	}
	if len(runtime.schemes) == 1 {
		if scheme, ok := runtime.schemes[soleSchemeName(runtime.schemes)]; ok {
			return Signer{
				Algorithm: scheme.alg,
				Secret:    scheme.secret,
				ClaimKeys: append([]string(nil), scheme.claimKeys...),
			}, nil
		}
	}
	return Signer{}, fmt.Errorf("未找到可用的 jwt 签发方案")
}

func currentState() runtimeState {
	stateMu.RLock()
	defer stateMu.RUnlock()

	cloned := runtimeState{
		schemes: make(map[string]Scheme, len(state.schemes)),
		guards:  append([]Guard(nil), state.guards...),
		primary: state.primary,
	}
	for name, scheme := range state.schemes {
		cloned.schemes[name] = scheme
	}
	return cloned
}

func lookupScheme(name string) (Scheme, bool) {
	stateMu.RLock()
	defer stateMu.RUnlock()
	scheme, ok := state.schemes[strings.TrimSpace(name)]
	return scheme, ok
}

func buildRuntimeState(authCfg config.Auth) (runtimeState, error) {
	next := runtimeState{
		schemes: map[string]Scheme{},
	}

	if secret := strings.TrimSpace(authCfg.JWTSecret); secret != "" {
		next.schemes["default"] = Scheme{
			name:      "default",
			alg:       jwtlib.SigningMethodHS256.Alg(),
			secret:    secret,
			header:    "Authorization",
			prefix:    "Bearer",
			claimKeys: []string{"uid", "sub"},
		}
	}

	names := make([]string, 0, len(authCfg.JWT.Schemes))
	for name := range authCfg.JWT.Schemes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		scheme, err := resolveScheme(name, authCfg.JWT.Schemes[name])
		if err != nil {
			return runtimeState{}, err
		}
		if scheme.name == "" {
			continue
		}
		next.schemes[scheme.name] = scheme
	}

	guards, err := normalizeGuards(authCfg.JWT.Guards, next.schemes)
	if err != nil {
		return runtimeState{}, err
	}
	next.guards = guards

	if len(next.guards) == 0 {
		switch len(next.schemes) {
		case 0:
		case 1:
			next.primary = soleSchemeName(next.schemes)
		default:
			return runtimeState{}, fmt.Errorf("jwt 配置了多个 schemes，但 guards 为空")
		}
	}

	return next, nil
}

func resolveScheme(name string, raw config.JWTScheme) (Scheme, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Scheme{}, nil
	}
	if raw.Enabled != nil && !*raw.Enabled {
		return Scheme{}, nil
	}

	alg := strings.ToUpper(strings.TrimSpace(raw.Alg))
	if alg == "" {
		alg = jwtlib.SigningMethodHS256.Alg()
	}
	if alg != jwtlib.SigningMethodHS256.Alg() {
		return Scheme{}, fmt.Errorf("jwt scheme %s 暂不支持算法: %s", name, alg)
	}

	secret := strings.TrimSpace(raw.Secret)
	if env := strings.TrimSpace(raw.SecretEnv); env != "" {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			secret = value
		}
	}
	if secret == "" {
		return Scheme{}, fmt.Errorf("jwt scheme %s 缺少 secret/secretEnv", name)
	}

	header := strings.TrimSpace(raw.Header)
	if header == "" {
		header = "Authorization"
	}

	prefix := strings.TrimSpace(raw.Prefix)
	if prefix == "" {
		prefix = "Bearer"
	}

	claimKeys := normalizeClaimKeys(raw.ClaimKeys)
	if len(claimKeys) == 0 {
		claimKeys = []string{"uid", "sub"}
	}

	return Scheme{
		name:      name,
		alg:       alg,
		secret:    secret,
		header:    header,
		prefix:    prefix,
		claimKeys: claimKeys,
	}, nil
}

func normalizeGuards(raw []config.JWTGuard, schemes map[string]Scheme) ([]Guard, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	guards := make([]Guard, 0, len(raw))
	for _, item := range raw {
		name := strings.TrimSpace(item.Scheme)
		if name == "" {
			continue
		}
		if _, ok := schemes[name]; !ok {
			return nil, fmt.Errorf("jwt guard 引用了不存在的 scheme: %s", name)
		}

		prefixes := normalizePrefixes(item.Prefixes)
		guard := Guard{
			scheme:      name,
			prefixes:    prefixes,
			publicPaths: normalizePathSet(item.PublicPaths),
		}
		guards = append(guards, guard)
	}
	sort.SliceStable(guards, func(i, j int) bool {
		return longestPrefixLength(guards[i].prefixes) > longestPrefixLength(guards[j].prefixes)
	})
	return guards, nil
}

func normalizeClaimKeys(keys []string) []string {
	result := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizePrefixes(prefixes []string) []string {
	if len(prefixes) == 0 {
		return []string{""}
	}
	result := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		normalized := strings.TrimSpace(prefix)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return []string{""}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return len(result[i]) > len(result[j])
	})
	return result
}

func normalizePathSet(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func soleSchemeName(schemes map[string]Scheme) string {
	for name := range schemes {
		return name
	}
	return ""
}

func longestPrefixLength(prefixes []string) int {
	maxLen := 0
	for _, prefix := range prefixes {
		if size := len(prefix); size > maxLen {
			maxLen = size
		}
	}
	return maxLen
}
