package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	moduleImportCommentPattern = regexp.MustCompile(`(?m)^\s*//\s*dever:import(?:\s*=\s*|\s+)(\S+)\s*$`)
	moduleImportValuePattern   = regexp.MustCompile(`(?m)^\s*(?:const|var)\s+\w+\s*=\s*"([^"]+)"\s*$`)
)

type ModuleSource struct {
	Name   string
	Root   string
	Import string
}

func ListModuleSources(projectRoot string) ([]ModuleSource, error) {
	rootPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("解析项目根目录失败: %w", err)
	}

	projectModule, err := readProjectModuleName(filepath.Join(rootPath, "go.mod"))
	if err != nil {
		return nil, err
	}

	moduleRoot := filepath.Join(rootPath, "module")
	entries, err := os.ReadDir(moduleRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	sources := make([]ModuleSource, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}

		source, err := ResolveModuleSource(rootPath, projectModule, name)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Name < sources[j].Name
	})
	return sources, nil
}

func ResolveModuleSource(projectRoot, projectModule, moduleName string) (ModuleSource, error) {
	rootPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return ModuleSource{}, fmt.Errorf("解析项目根目录失败: %w", err)
	}

	moduleName = strings.TrimSpace(moduleName)
	if moduleName == "" {
		return ModuleSource{}, fmt.Errorf("模块名不能为空")
	}

	source := ModuleSource{
		Name:   moduleName,
		Root:   filepath.Join(rootPath, "module", moduleName),
		Import: strings.TrimSpace(projectModule) + "/module/" + moduleName,
	}

	redirectImport, ok, err := readModuleImportDirective(filepath.Join(source.Root, "main.go"))
	if err != nil {
		return ModuleSource{}, fmt.Errorf("解析 module/%s/main.go 失败: %w", moduleName, err)
	}
	if !ok {
		return source, nil
	}

	resolvedRoot, err := resolveModuleImportPath(rootPath, redirectImport)
	if err != nil {
		return ModuleSource{}, fmt.Errorf("解析 module/%s 真实模块失败: %w", moduleName, err)
	}

	source.Root = resolvedRoot
	source.Import = redirectImport
	return source, nil
}

func readModuleImportDirective(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	if matches := moduleImportCommentPattern.FindSubmatch(content); len(matches) == 2 {
		return strings.TrimSpace(string(matches[1])), true, nil
	}
	if matches := moduleImportValuePattern.FindSubmatch(content); len(matches) == 2 {
		return strings.TrimSpace(string(matches[1])), true, nil
	}
	return "", false, nil
}

func resolveModuleImportPath(projectRoot, importPath string) (string, error) {
	cleanImport := strings.TrimSpace(importPath)
	if cleanImport == "" {
		return "", fmt.Errorf("模块导入路径不能为空")
	}

	// 通过 go list 解析真实源码目录，这里会自动遵循 go.mod 的 replace。
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", cleanImport)
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	resolvedRoot := strings.TrimSpace(string(output))
	if resolvedRoot == "" {
		return "", fmt.Errorf("go list 未返回源码目录")
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", err
	}
	return resolvedRoot, nil
}

func readProjectModuleName(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "module ")), nil
		}
	}
	return "", fmt.Errorf("go.mod 缺少 module 声明")
}
