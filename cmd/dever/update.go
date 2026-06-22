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

const defaultUpdateModule = "github.com/shemic/dever/cmd/dever"

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	binDir := fs.String("bin-dir", "", "安装目录，默认覆盖当前 PATH 命中的 dever 所在目录")
	ref := fs.String("ref", "latest", "Git ref/tag/branch，例如 latest、main、v0.1.3")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("update 参数解析失败: %v", err)
	}

	targetDir, err := resolveInstallBinDir(*binDir)
	if err != nil {
		log.Fatalf("解析安装目录失败: %v", err)
	}
	if err := runUpdateCommand(targetDir, strings.TrimSpace(*ref)); err != nil {
		log.Fatalf("update 执行失败: %v", err)
	}
}

func runUpdateCommand(targetDir, ref string) error {
	if strings.TrimSpace(ref) == "" {
		ref = "latest"
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

	cmd := exec.Command("go", "install", query)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeCommandEnv(util.WithCanonicalPackageGoEnv(os.Environ()), map[string]string{
		"GOBIN": targetDir,
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install %s 失败: %w", query, err)
	}

	fmt.Printf("dever update: 已更新到 %s\n", targetPath)
	if !isBinDirInPath(targetDir) {
		fmt.Printf("请将该目录加入 PATH: %s\n", targetDir)
	}
	return nil
}
