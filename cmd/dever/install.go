package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	binDir := fs.String("bin-dir", "", "安装目录，默认自动选择用户 bin")
	skipSkills := fs.Bool("skip-skills", false, "跳过安装/同步 shemic-dever AI skill")
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

	if !*skipSkills {
		if err := runSkillInstall(skillInstallOptions{
			projectRoot: root,
			global:      true,
			project:     false,
			agents:      true,
		}); err != nil {
			log.Fatalf("安装 dever skill 失败: %v", err)
		}
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
	content, err := buildLauncherScript(root)
	if err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0o755)
}

func buildLauncherScript(root string) (string, error) {
	sourceRoot, err := resolveLauncherSourceRoot(root)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
set -e
%scaller_dir="${PWD}"
source_root=%s
cd "$source_root"
export %s="$caller_dir"
command_name="${1:-}"
cli_label="dever"
if [ -n "$command_name" ]; then
  cli_label="dever ${command_name}"
fi
cli_bin="$source_root/tmp/dever-cli/dever"

needs_cli_build() {
  if [ ! -x "$cli_bin" ]; then
    return 0
  fi
  local newer_file
  newer_file="$(find "$source_root" \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "$cli_bin" -print -quit)"
  [ -n "$newer_file" ]
}

show_cli_build_progress() {
  local build_pid="$1"
  local label="$2"
  local started_at
  started_at="$(date +%%s)"
  local frames=("-" "\\" "|" "/")
  local frame_index=0
  local last_printed=-1
  while kill -0 "$build_pid" 2>/dev/null; do
    local now
    now="$(date +%%s)"
    local elapsed=$((now - started_at))
    if [ -t 2 ]; then
      printf "\r%%s: 编译 Dever CLI 中... %%s %%ss" "$label" "${frames[$((frame_index %% 4))]}" "$elapsed" >&2
      frame_index=$((frame_index + 1))
    elif [ "$elapsed" -ne "$last_printed" ] && [ $((elapsed %% 2)) -eq 0 ]; then
      echo "$label: 编译 Dever CLI 中... ${elapsed}s" >&2
      last_printed="$elapsed"
    fi
    sleep 0.2
  done
}

build_cli() {
  mkdir -p "$(dirname "$cli_bin")"
  echo "$cli_label: 正在编译 Dever CLI，清理 Go cache 后首次编译可能较慢..." >&2
  go build -o "$cli_bin" ./cmd/dever &
  local build_pid=$!
  show_cli_build_progress "$build_pid" "$cli_label" &
  local progress_pid=$!
  set +e
  wait "$build_pid"
  local status=$?
  kill "$progress_pid" 2>/dev/null
  wait "$progress_pid" 2>/dev/null
  set -e
  if [ -t 2 ]; then
    printf "\r%%*s\r" 96 "" >&2
  fi
  if [ "$status" -ne 0 ]; then
    return "$status"
  fi
  echo "$cli_label: Dever CLI 编译完成" >&2
}

if needs_cli_build; then
  build_cli
fi

exec "$cli_bin" "$@"
`, launcherPathExport(), quoteShellPath(filepath.ToSlash(sourceRoot)), callerDirEnv), nil
}

func resolveLauncherSourceRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	candidates := []string{
		root,
		filepath.Join(root, "dever"),
	}
	for _, candidate := range candidates {
		if isDeverCommandRoot(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 dever 命令源码，请在项目根目录或 dever 源码目录执行 install: %s", root)
}

func isDeverCommandRoot(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "dever", "main.go")); err != nil {
		return false
	}
	return true
}

func launcherPathExport() string {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return ""
	}
	goDir := strings.TrimSpace(filepath.Dir(goPath))
	if goDir == "" {
		return ""
	}
	return fmt.Sprintf("export PATH=%s:\"$PATH\"\n", quoteShellPath(filepath.ToSlash(goDir)))
}

func quoteShellPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'"'"'`) + "'"
}
