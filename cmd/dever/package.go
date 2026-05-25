package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultPackageRepoBase = "https://github.com/dever-package"

var packageNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type packageAddOptions struct {
	projectRoot string
	name        string
	repoBase    string
	skipInit    bool
}

func runPackage(args []string) {
	if len(args) == 0 {
		printPackageUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runPackageAddCommand(args[1:])
	default:
		printPackageUsage()
		os.Exit(1)
	}
}

func printPackageUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever package - package 组件命令

Usage:
    dever package add [--project-root=.] [--repo-base=https://github.com/dever-package] [--skip-init] <name>
`)
}

func runPackageAddCommand(args []string) {
	fs := flag.NewFlagSet("package add", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	repoBase := fs.String("repo-base", defaultPackageRepoBase, "package GitHub 组织或仓库基础地址")
	skipInit := fs.Bool("skip-init", false, "跳过刷新 routes/model/service 注册")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package add 参数解析失败: %v", err)
	}
	if fs.NArg() != 1 {
		log.Fatal("package add 需要一个组件名称，例如：dever package add bot")
	}

	root := resolvePackageProjectRoot(*projectRoot)
	options := packageAddOptions{
		projectRoot: root,
		name:        strings.TrimSpace(fs.Arg(0)),
		repoBase:    strings.TrimSpace(*repoBase),
		skipInit:    *skipInit,
	}
	if err := runPackageAdd(options); err != nil {
		log.Fatalf("package add 执行失败: %v", err)
	}
}

func runPackageAdd(options packageAddOptions) error {
	if err := validatePackageName(options.name); err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(options.projectRoot); err != nil {
		return err
	}

	projectModule, err := readPackageProjectModuleName(filepath.Join(options.projectRoot, "go.mod"))
	if err != nil {
		return err
	}

	importPath := projectModule + "/package/" + options.name
	repoURL := buildPackageRepoURL(options.repoBase, options.name)
	packageDir := filepath.Join(options.projectRoot, "package", options.name)
	moduleDir := filepath.Join(options.projectRoot, "module", options.name)

	cloned, err := ensurePackageSource(packageDir, repoURL)
	if err != nil {
		return err
	}
	if cloned {
		fmt.Printf("dever package add: 已拉取 %s 到 %s\n", repoURL, packageDir)
	} else {
		fmt.Printf("dever package add: package/%s 已存在，跳过拉取\n", options.name)
	}

	createdShim, err := ensurePackageModuleShim(moduleDir, options.name, importPath)
	if err != nil {
		return err
	}
	if createdShim {
		fmt.Printf("dever package add: 已创建 module/%s/main.go\n", options.name)
	} else {
		fmt.Printf("dever package add: module/%s/main.go 已存在，跳过创建\n", options.name)
	}

	if !options.skipInit {
		if err := runProjectInit(options.projectRoot, true); err != nil {
			return fmt.Errorf("刷新生成文件失败: %w", err)
		}
		fmt.Println("dever package add: 已刷新 routes/model/service 注册")
	}
	return nil
}

func resolvePackageProjectRoot(rawRoot string) string {
	root := resolveProjectRoot(rawRoot)
	if isGoModuleRoot(root) {
		return root
	}
	backendRoot := filepath.Join(root, "backend")
	if isGoModuleRoot(backendRoot) {
		return backendRoot
	}
	return root
}

func isGoModuleRoot(root string) bool {
	info, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil && !info.IsDir()
}

func validatePackageName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("组件名称不能为空")
	}
	if !packageNamePattern.MatchString(name) {
		return fmt.Errorf("组件名称必须是合法 Go 包名: %s", name)
	}
	return nil
}

func ensurePackageProjectRoot(projectRoot string) error {
	if !isGoModuleRoot(projectRoot) {
		return fmt.Errorf("未找到 go.mod，请在 Dever 后端项目根目录执行或传 --project-root")
	}
	for _, dir := range []string{"package", "module"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, dir), 0o755); err != nil {
			return fmt.Errorf("创建 %s 目录失败: %w", dir, err)
		}
	}
	return nil
}

func readPackageProjectModuleName(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if moduleName == "" {
				break
			}
			return moduleName, nil
		}
	}
	return "", fmt.Errorf("go.mod 缺少 module 声明")
}

func buildPackageRepoURL(repoBase, name string) string {
	repoBase = strings.TrimRight(strings.TrimSpace(repoBase), "/")
	if repoBase == "" {
		repoBase = defaultPackageRepoBase
	}
	return repoBase + "/" + name + ".git"
}

func ensurePackageSource(packageDir, repoURL string) (bool, error) {
	info, err := os.Stat(packageDir)
	switch {
	case err == nil && !info.IsDir():
		return false, fmt.Errorf("package 目标已存在且不是目录: %s", packageDir)
	case err == nil:
		empty, err := isDirectoryEmpty(packageDir)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(packageDir), 0o755); err != nil {
			return false, err
		}
	default:
		return false, err
	}

	if err := runGitClone(repoURL, packageDir); err != nil {
		if info == nil {
			_ = os.RemoveAll(packageDir)
		}
		return false, err
	}
	return true, nil
}

func isDirectoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func runGitClone(repoURL, packageDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, packageDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("拉取 package 失败: %w", err)
	}
	return nil
}

func ensurePackageModuleShim(moduleDir, packageName, importPath string) (bool, error) {
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return false, err
	}

	mainPath := filepath.Join(moduleDir, "main.go")
	content := fmt.Sprintf("package %s\n\n// dever:import %s\n", packageName, importPath)
	current, err := os.ReadFile(mainPath)
	if err == nil {
		if normalizePackageShimContent(string(current)) == normalizePackageShimContent(content) {
			return false, nil
		}
		return false, fmt.Errorf("module/%s/main.go 已存在且不是目标 package shim，请手动处理", packageName)
	}
	if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.WriteFile(mainPath, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func normalizePackageShimContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.TrimSpace(content)
}
