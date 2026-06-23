package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shemic/dever/util"
)

const (
	defaultUpdateModule = "github.com/shemic/dever/cmd/dever"
	defaultUpdateRoot   = "github.com/shemic/dever"
)

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	binDir := fs.String("bin-dir", "", "安装目录，默认覆盖当前 PATH 命中的 dever 所在目录")
	ref := fs.String("ref", "main", "Git ref/tag/branch，例如 main、latest、v0.1.3")
	skipFramework := fs.Bool("skip-framework", false, "只更新 dever 命令，跳过当前项目 github.com/shemic/dever 依赖")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("update 参数解析失败: %v", err)
	}

	targetDir, err := resolveInstallBinDir(*binDir)
	if err != nil {
		log.Fatalf("解析安装目录失败: %v", err)
	}
	if !*skipFramework {
		root := resolveProjectRoot(*projectRoot)
		if err := runUpdateFramework(root, strings.TrimSpace(*ref)); err != nil {
			log.Fatalf("更新 Dever 框架依赖失败: %v", err)
		}
	}
	if err := runUpdateCommand(targetDir, strings.TrimSpace(*ref)); err != nil {
		log.Fatalf("update 执行失败: %v", err)
	}
}

func runUpdateCommand(targetDir, ref string) error {
	if strings.TrimSpace(ref) == "" {
		ref = "main"
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("创建安装目录失败: %w", err)
	}
	targetPath := filepath.Join(targetDir, "dever")
	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		return fmt.Errorf("安装目标已存在且是目录: %s", targetPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("检查安装目标失败: %w", err)
	}

	query := defaultUpdateModule + "@" + ref
	fmt.Printf("dever update: 正在从 GitHub 更新 %s\n", query)
	fmt.Printf("dever update: 安装目录 %s\n", targetDir)

	tempDir, err := os.MkdirTemp(targetDir, ".dever-update-*")
	if err != nil {
		return fmt.Errorf("创建临时安装目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("go", "install", query)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeCommandEnv(util.WithCanonicalPackageGoEnv(os.Environ()), map[string]string{
		"GOBIN":     tempDir,
		"GONOSUMDB": defaultUpdateRoot,
		"GOPROXY":   "direct",
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install %s 失败: %w", query, err)
	}

	tempBinary := filepath.Join(tempDir, "dever")
	if info, err := os.Stat(tempBinary); err != nil {
		return fmt.Errorf("更新产物缺失: %w", err)
	} else if info.IsDir() {
		return fmt.Errorf("更新产物是目录: %s", tempBinary)
	}
	if err := os.Chmod(tempBinary, 0o755); err != nil {
		return fmt.Errorf("设置更新产物权限失败: %w", err)
	}
	if err := os.Rename(tempBinary, targetPath); err != nil {
		return fmt.Errorf("替换 %s 失败: %w", targetPath, err)
	}

	fmt.Printf("dever update: 已更新到 %s\n", targetPath)
	if !isBinDirInPath(targetDir) {
		fmt.Printf("请将该目录加入 PATH: %s\n", targetDir)
	}
	return nil
}

func runUpdateFramework(projectRoot, ref string) error {
	root, ok, err := resolveUpdateFrameworkRoot(projectRoot)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Printf("dever update: 当前目录未找到需要更新的 %s 依赖，已跳过框架依赖更新\n", defaultUpdateRoot)
		return nil
	}
	if strings.TrimSpace(ref) == "" {
		ref = "main"
	}
	if target, ok, err := localReplaceTarget(root, defaultUpdateRoot); err != nil {
		return fmt.Errorf("检查本地 replace 失败: %w", err)
	} else if ok {
		fmt.Printf("dever update: 检测到本地 replace %s => %s，go get 会更新 require 版本，构建仍使用本地源码\n", defaultUpdateRoot, target)
	}

	query := defaultUpdateRoot + "@" + ref
	fmt.Printf("dever update: 正在更新项目 Dever 框架 %s\n", query)
	cmd := exec.Command("go", "get", query)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeCommandEnv(util.WithCanonicalPackageGoEnv(os.Environ()), map[string]string{
		"GONOSUMDB": defaultUpdateRoot,
		"GOPROXY":   "direct",
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go get %s 失败: %w", query, err)
	}
	fmt.Printf("dever update: 已更新项目 Dever 框架依赖: %s\n", root)
	return nil
}

func resolveUpdateFrameworkRoot(projectRoot string) (string, bool, error) {
	root := resolvePackageProjectRoot(projectRoot)
	if !isGoModuleRoot(root) {
		return "", false, nil
	}
	moduleName, err := util.ReadProjectModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", false, err
	}
	if moduleName == defaultUpdateRoot {
		parent := filepath.Dir(root)
		if shouldUpdateFrameworkInRoot(parent) {
			return parent, true, nil
		}
		return "", false, nil
	}
	if shouldUpdateFrameworkInRoot(root) {
		return root, true, nil
	}
	return "", false, nil
}

func shouldUpdateFrameworkInRoot(root string) bool {
	if !isGoModuleRoot(root) {
		return false
	}
	if _, ok, err := goModRequireVersion(root, defaultUpdateRoot); err == nil && ok {
		return true
	}
	if _, ok, err := localReplaceTarget(root, defaultUpdateRoot); err == nil && ok {
		return true
	}
	return false
}
