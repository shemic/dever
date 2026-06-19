package util

import "strings"

const CanonicalPackageGoEnvPattern = CanonicalPackagePrefix + "*"

// WithCanonicalPackageGoEnv 让 github.com/dever-package/* 通过直连解析。
func WithCanonicalPackageGoEnv(env []string) []string {
	env = appendGoEnvPattern(env, "GOPRIVATE", CanonicalPackageGoEnvPattern)
	env = appendGoEnvPattern(env, "GONOSUMDB", CanonicalPackageGoEnvPattern)
	env = appendGoEnvPattern(env, "GONOPROXY", CanonicalPackageGoEnvPattern)
	return env
}

func appendGoEnvPattern(env []string, key, pattern string) []string {
	prefix := key + "="
	for i, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		value := strings.TrimPrefix(entry, prefix)
		if hasGoEnvPattern(value, pattern) {
			return env
		}
		if strings.TrimSpace(value) == "" {
			env[i] = prefix + pattern
			return env
		}
		env[i] = prefix + value + "," + pattern
		return env
	}
	return append(env, prefix+pattern)
}

func hasGoEnvPattern(value, pattern string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) == pattern {
			return true
		}
	}
	return false
}
