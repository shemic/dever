package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	frontcompiler "github.com/shemic/dever/compiler/front"
)

const (
	frontCompilerRootEnv        = "DEVER_FRONT_COMPILER_ROOT"
	frontPackageRootEnv         = "DEVER_FRONT_PACKAGE_ROOT"
	frontCompilerCacheDir       = "tmp/dever/compiler/front"
	frontCompilerFingerprint    = ".dever-front-compiler-fingerprint"
	frontPluginProjectRootEnv   = "DEVER_FRONT_PLUGIN_PROJECT_ROOT"
	frontPluginNameEnv          = "DEVER_FRONT_PLUGIN_NAME"
	frontPluginRootEnv          = "DEVER_FRONT_PLUGIN_ROOT"
	frontCompilerPackageJSON    = "package.json"
	frontCompilerViteConfig     = "vite.config.ts"
	frontPackageSDKRelativePath = "sdk/src/index.ts"
)

func resolveFrontCompilerRoot(projectRoot string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(frontCompilerRootEnv)); value != "" {
		return validateFrontCompilerRoot(value)
	}
	return prepareEmbeddedFrontCompiler(projectRoot)
}

func validateFrontCompilerRoot(rawRoot string) (string, error) {
	compilerRoot, err := filepath.Abs(strings.TrimSpace(rawRoot))
	if err != nil {
		return "", err
	}
	if !hasFrontCompilerConfig(compilerRoot) {
		return "", fmt.Errorf("%s 指向的前端插件编译器不可用: %s", frontCompilerRootEnv, compilerRoot)
	}
	return compilerRoot, nil
}

func prepareEmbeddedFrontCompiler(projectRoot string) (string, error) {
	compilerRoot := filepath.Join(projectRoot, frontCompilerCacheDir)
	fingerprint, err := embeddedFrontCompilerFingerprint()
	if err != nil {
		return "", err
	}

	if cachedFrontCompilerReady(compilerRoot, fingerprint) {
		return compilerRoot, nil
	}

	if err := os.RemoveAll(compilerRoot); err != nil {
		return "", fmt.Errorf("清理前端插件编译器缓存失败: %w", err)
	}
	if err := os.MkdirAll(compilerRoot, 0o755); err != nil {
		return "", fmt.Errorf("创建前端插件编译器缓存目录失败: %w", err)
	}
	if err := extractFrontCompilerFS(compilerRoot); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(compilerRoot, frontCompilerFingerprint), []byte(fingerprint), 0o644); err != nil {
		return "", fmt.Errorf("写入前端插件编译器缓存标记失败: %w", err)
	}
	if !hasFrontCompilerConfig(compilerRoot) {
		return "", fmt.Errorf("内置前端插件编译器缺少必要配置")
	}
	return compilerRoot, nil
}

func cachedFrontCompilerReady(compilerRoot, fingerprint string) bool {
	content, err := os.ReadFile(filepath.Join(compilerRoot, frontCompilerFingerprint))
	if err != nil || strings.TrimSpace(string(content)) != fingerprint {
		return false
	}
	return hasFrontCompilerConfig(compilerRoot)
}

func embeddedFrontCompilerFingerprint() (string, error) {
	hash := sha256.New()
	if err := fs.WalkDir(frontcompiler.FS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		file, err := frontcompiler.FS.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()

		hash.Write([]byte(name))
		hash.Write([]byte{0})
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		hash.Write([]byte{0})
		return nil
	}); err != nil {
		return "", fmt.Errorf("计算前端插件编译器指纹失败: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractFrontCompilerFS(compilerRoot string) error {
	return fs.WalkDir(frontcompiler.FS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}

		target := filepath.Join(compilerRoot, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyEmbeddedFrontCompilerFile(name, target, entry)
	})
}

func copyEmbeddedFrontCompilerFile(name, target string, entry fs.DirEntry) error {
	source, err := frontcompiler.FS.Open(name)
	if err != nil {
		return fmt.Errorf("读取前端插件编译器文件失败: %s: %w", name, err)
	}
	defer source.Close()

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("读取前端插件编译器文件信息失败: %s: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("创建前端插件编译器文件目录失败: %w", err)
	}

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("写入前端插件编译器文件失败: %s: %w", target, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, source); err != nil {
		return fmt.Errorf("复制前端插件编译器文件失败: %s: %w", target, err)
	}
	return nil
}

func hasFrontCompilerConfig(compilerRoot string) bool {
	for _, file := range []string{frontCompilerPackageJSON, frontCompilerViteConfig} {
		info, err := os.Stat(filepath.Join(compilerRoot, file))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func ensureFrontCompilerDependencies(compilerRoot string) error {
	if frontCompilerDependenciesReady(compilerRoot) {
		return nil
	}

	log.Printf("dever front: 安装前端插件编译器依赖: %s", compilerRoot)
	cmd := exec.Command("pnpm", "--dir", compilerRoot, "install", "--lockfile=false")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装前端插件编译器依赖失败: %w", err)
	}
	return nil
}

func frontCompilerDependenciesReady(compilerRoot string) bool {
	for _, file := range []string{
		filepath.Join("node_modules", ".bin", "vite"),
		filepath.Join("node_modules", "@vitejs", "plugin-react-swc"),
	} {
		info, err := os.Stat(filepath.Join(compilerRoot, file))
		if err != nil || info.IsDir() && filepath.Base(file) == "vite" {
			return false
		}
	}
	return true
}

func frontCompilerEnv(projectRoot string, overrides map[string]string) []string {
	env := map[string]string{
		frontPluginProjectRootEnv: projectRoot,
	}
	if frontPackageRoot := resolveFrontPackageRoot(projectRoot); frontPackageRoot != "" {
		env[frontPackageRootEnv] = frontPackageRoot
	}
	for key, value := range overrides {
		if strings.TrimSpace(key) == "" {
			continue
		}
		env[key] = value
	}
	return mergeCommandEnv(os.Environ(), env)
}

func resolveFrontPackageRoot(projectRoot string) string {
	if value := strings.TrimSpace(os.Getenv(frontPackageRootEnv)); value != "" {
		if root := usableFrontPackageRoot(value); root != "" {
			return root
		}
	}

	for _, candidate := range []string{
		filepath.Join(projectRoot, "package", "front"),
		filepath.Join(projectRoot, "backend", "package", "front"),
	} {
		if root := usableFrontPackageRoot(candidate); root != "" {
			return root
		}
	}
	return ""
}

func usableFrontPackageRoot(rawRoot string) string {
	root, err := filepath.Abs(strings.TrimSpace(rawRoot))
	if err != nil {
		return ""
	}
	info, err := os.Stat(filepath.Join(root, frontPackageSDKRelativePath))
	if err != nil || info.IsDir() {
		return ""
	}
	return root
}
