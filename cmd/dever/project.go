package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	devercmd "github.com/shemic/dever/cmd"
)

func runProjectInit(projectRoot string, skipTidy bool) error {
	if !skipTidy {
		if err := runGoModTidy(projectRoot); err != nil {
			return fmt.Errorf("go mod tidy 执行失败: %w", err)
		}
	}

	if err := devercmd.GenerateRoutes(projectRoot); err != nil {
		return fmt.Errorf("路由生成失败: %w", err)
	}
	if err := devercmd.GenerateServices(projectRoot); err != nil {
		return fmt.Errorf("service 生成失败: %w", err)
	}
	if err := devercmd.GenerateModels(projectRoot); err != nil {
		return fmt.Errorf("model 生成失败: %w", err)
	}
	return nil
}

func runGoModTidy(projectRoot string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = projectRoot
	return cmd.Run()
}

func resolveProjectRoot(root string) string {
	if root == "" {
		root = "."
	}
	wd, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("无法解析项目根目录: %v", err)
	}
	return wd
}

type goBuildSpec struct {
	dir      string
	target   string
	output   string
	env      map[string]string
	trimPath bool
	buildVCS string
	ldflags  string
	progress string
}

func runGoBuild(spec goBuildSpec) error {
	if strings.TrimSpace(spec.output) == "" {
		return fmt.Errorf("构建输出路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(spec.output), 0o755); err != nil {
		return fmt.Errorf("创建构建输出目录失败: %w", err)
	}

	args := []string{"build"}
	if spec.trimPath {
		args = append(args, "-trimpath")
	}
	if buildVCS := strings.TrimSpace(spec.buildVCS); buildVCS != "" {
		args = append(args, "-buildvcs="+buildVCS)
	}
	if ldflags := strings.TrimSpace(spec.ldflags); ldflags != "" {
		args = append(args, "-ldflags="+ldflags)
	}
	args = append(args, "-o", spec.output)

	target := strings.TrimSpace(spec.target)
	if target == "" {
		target = "."
	}
	args = append(args, target)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = spec.dir
	cmd.Env = mergeCommandEnv(os.Environ(), spec.env)

	stopProgress := startBuildProgressReporter(strings.TrimSpace(spec.progress))
	defer stopProgress()

	return cmd.Run()
}

func startBuildProgressReporter(label string) func() {
	if label == "" {
		return func() {}
	}

	const spinnerInterval = 200 * time.Millisecond

	spinnerFrames := []string{"-", "\\", "|", "/"}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()

		startedAt := time.Now()
		isTTY := isTerminalFile(os.Stderr)
		lastPrintedSecond := -1
		frameIndex := 0
		for {
			select {
			case <-done:
				if isTTY {
					clearTerminalLine(os.Stderr)
				}
				return
			case <-ticker.C:
				elapsed := time.Since(startedAt)
				if isTTY {
					fmt.Fprintf(
						os.Stderr,
						"\r%s: 构建中... %s %s",
						label,
						spinnerFrames[frameIndex%len(spinnerFrames)],
						formatBuildElapsed(elapsed),
					)
					frameIndex++
					continue
				}

				currentSecond := int(elapsed / time.Second)
				if currentSecond <= lastPrintedSecond {
					continue
				}
				lastPrintedSecond = currentSecond
				fmt.Fprintf(os.Stderr, "%s: 构建中... %ds\n", label, currentSecond)
			}
		}
	}()

	return func() {
		close(done)
	}
}

func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func clearTerminalLine(file *os.File) {
	if file == nil {
		return
	}
	fmt.Fprintf(file, "\r%s\r", strings.Repeat(" ", 64))
}

func formatBuildElapsed(elapsed time.Duration) string {
	if elapsed < time.Second {
		return elapsed.Truncate(100 * time.Millisecond).String()
	}
	if elapsed < 10*time.Second {
		return elapsed.Truncate(100 * time.Millisecond).String()
	}
	return elapsed.Truncate(time.Second).String()
}

func mergeCommandEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}

	merged := make([]string, 0, len(base)+len(overrides))
	seen := make(map[string]struct{}, len(base)+len(overrides))

	for _, entry := range base {
		key := entry
		if index := strings.Index(entry, "="); index >= 0 {
			key = entry[:index]
		}
		if value, ok := overrides[key]; ok {
			merged = append(merged, key+"="+value)
			seen[key] = struct{}{}
			continue
		}
		merged = append(merged, entry)
		seen[key] = struct{}{}
	}

	for key, value := range overrides {
		if _, ok := seen[key]; ok {
			continue
		}
		merged = append(merged, key+"="+value)
	}

	return merged
}
