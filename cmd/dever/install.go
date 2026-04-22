package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	binDir := fs.String("bin-dir", "", "安装目录，默认自动选择用户 bin")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("install 参数解析失败: %v", err)
	}

	root := resolveProjectRoot(*projectRoot)
	targetDir, err := resolveInstallBinDir(*binDir)
	if err != nil {
		log.Fatalf("解析安装目录失败: %v", err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		log.Fatalf("创建安装目录失败: %v", err)
	}

	targetPath := filepath.Join(targetDir, "dever")
	if err := writeInstallLauncher(root, targetPath); err != nil {
		log.Fatalf("安装 dever 失败: %v", err)
	}
	fmt.Printf("dever 已安装到: %s\n", targetPath)

	if !isBinDirInPath(targetDir) {
		fmt.Printf("请将该目录加入 PATH: %s\n", targetDir)
	}
}

func resolveInstallBinDir(configured string) (string, error) {
	if strings.TrimSpace(configured) != "" {
		return filepath.Abs(strings.TrimSpace(configured))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
	}

	for _, candidate := range candidates {
		if isBinDirInPath(candidate) {
			return candidate, nil
		}
	}

	return candidates[0], nil
}

func isBinDirInPath(target string) bool {
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}

	for _, current := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.TrimSpace(current) == "" {
			continue
		}
		currentAbs, err := filepath.Abs(current)
		if err != nil {
			continue
		}
		if currentAbs == targetAbs {
			return true
		}
	}
	return false
}

func writeInstallLauncher(root, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(targetPath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("安装目标已存在且是目录: %s", targetPath)
		}
		if err := os.Remove(targetPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	content := buildLauncherScript(root)
	if err := os.WriteFile(targetPath, []byte(content), 0o755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0o755)
}

func buildLauncherScript(root string) string {
	sourceDir := filepath.ToSlash(filepath.Join(root, "dever", "cmd", "dever"))
	return fmt.Sprintf("#!/usr/bin/env bash\nset -e\nexec go run %s/*.go \"$@\"\n", quoteShellPath(sourceDir))
}

func quoteShellPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'"'"'`) + "'"
}
