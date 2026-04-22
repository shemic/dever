package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	defaultWatchInterval = 800 * time.Millisecond
	processStopTimeout   = 5 * time.Second
)

type watchRunOptions struct {
	projectRoot string
	entry       string
	interval    time.Duration
	skipInit    bool
}

type watchedFileState struct {
	size    int64
	modTime int64
}

type watchedProcess struct {
	root       string
	entry      string
	binaryPath string
	cmd        *exec.Cmd
	done       chan error
}

func runWatchMode(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	entry := fs.String("entry", "main.go", "启动入口，默认 main.go")
	interval := fs.Duration("interval", defaultWatchInterval, "文件扫描间隔")
	skipInit := fs.Bool("skip-init", false, "跳过启动前与敏感变更后的 init --skip-tidy")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("run 参数解析失败: %v", err)
	}

	options := watchRunOptions{
		projectRoot: resolveProjectRoot(*projectRoot),
		entry:       strings.TrimSpace(*entry),
		interval:    *interval,
		skipInit:    *skipInit,
	}
	if options.entry == "" {
		options.entry = "main.go"
	}
	if options.interval <= 0 {
		options.interval = defaultWatchInterval
	}

	if err := runHotReload(options); err != nil {
		log.Fatalf("run 执行失败: %v", err)
	}
}

func runHotReload(options watchRunOptions) error {
	if !options.skipInit {
		fmt.Println("dever run: 启动前执行 init --skip-tidy")
		if err := runProjectInit(options.projectRoot, true); err != nil {
			return err
		}
	}

	snapshot, err := scanWatchedFiles(options.projectRoot)
	if err != nil {
		return err
	}

	process := &watchedProcess{
		root:       options.projectRoot,
		entry:      options.entry,
		binaryPath: filepath.Join(options.projectRoot, "tmp", "dever-run", "app"),
	}
	if err := process.restart("初始启动", true); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(options.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\ndever run: 正在停止...")
			return process.stop(processStopTimeout)
		case err := <-process.doneChannel():
			process.clear()
			if err != nil {
				log.Printf("运行进程已退出: %v", err)
			} else {
				log.Printf("运行进程已退出")
			}
		case <-ticker.C:
			current, err := scanWatchedFiles(options.projectRoot)
			if err != nil {
				log.Printf("扫描文件失败: %v", err)
				continue
			}

			changes := detectWatchedFileChanges(snapshot, current)
			if len(changes) == 0 {
				continue
			}

			log.Printf("检测到文件变更: %s", formatChangedPaths(changes))

			if !options.skipInit && requiresProjectInit(changes) {
				log.Printf("检测到 model/service/api 变更，执行 init --skip-tidy")
				if err := runProjectInit(options.projectRoot, true); err != nil {
					log.Printf("init 执行失败，保留当前进程: %v", err)
					snapshot = current
					continue
				}
				current, err = scanWatchedFiles(options.projectRoot)
				if err != nil {
					log.Printf("init 后重新扫描失败: %v", err)
				}
			}

			rebuild := requiresBinaryRebuild(changes) || !process.binaryExists()
			if err := process.restart("检测到文件变更", rebuild); err != nil {
				log.Printf("重启进程失败: %v", err)
			}
			snapshot = current
		}
	}
}

func (p *watchedProcess) restart(reason string, rebuild bool) error {
	if rebuild {
		if err := p.buildBinary(); err != nil {
			return err
		}
	}

	if err := p.stop(processStopTimeout); err != nil {
		return err
	}

	cmd := exec.Command(p.binaryPath)
	cmd.Dir = p.root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动子进程失败: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	p.cmd = cmd
	p.done = done
	log.Printf("已启动运行进程（pid=%d）：%s", cmd.Process.Pid, reason)
	return nil
}

func (p *watchedProcess) binaryExists() bool {
	if p == nil || strings.TrimSpace(p.binaryPath) == "" {
		return false
	}
	info, err := os.Stat(p.binaryPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (p *watchedProcess) buildBinary() error {
	if err := runGoBuild(goBuildSpec{
		dir:    p.root,
		target: p.entry,
		output: p.binaryPath,
	}); err != nil {
		return fmt.Errorf("构建运行二进制失败: %w", err)
	}
	return nil
}

func (p *watchedProcess) stop(timeout time.Duration) error {
	if p.cmd == nil || p.cmd.Process == nil {
		p.clear()
		return nil
	}

	cmd := p.cmd
	done := p.done
	p.clear()

	processGroupID := -cmd.Process.Pid
	if err := syscall.Kill(processGroupID, syscall.SIGTERM); err != nil && !isMissingProcessError(err) {
		_ = syscall.Kill(processGroupID, syscall.SIGKILL)
		return nil
	}

	select {
	case err := <-done:
		return normalizeProcessExitError(err)
	case <-time.After(timeout):
		_ = syscall.Kill(processGroupID, syscall.SIGKILL)
		<-done
		return nil
	}
}

func (p *watchedProcess) doneChannel() <-chan error {
	if p.done == nil {
		return nil
	}
	return p.done
}

func (p *watchedProcess) clear() {
	p.cmd = nil
	p.done = nil
}

func normalizeProcessExitError(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return err
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func isMissingProcessError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone)
}

func scanWatchedFiles(projectRoot string) (map[string]watchedFileState, error) {
	result := make(map[string]watchedFileState)
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(projectRoot, entry.Name())
		relative := normalizeWatchPath(projectRoot, fullPath)
		if entry.IsDir() {
			if !shouldScanRootDir(relative) || shouldSkipWatchDir(relative) {
				continue
			}
			if err := walkWatchedPath(projectRoot, fullPath, result); err != nil {
				return nil, err
			}
			continue
		}
		if !shouldWatchFile(relative) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result[relative] = watchedFileState{
			size:    info.Size(),
			modTime: info.ModTime().UnixNano(),
		}
	}

	return result, nil
}

func walkWatchedPath(projectRoot, rootPath string, result map[string]watchedFileState) error {
	return filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative := normalizeWatchPath(projectRoot, path)
		if entry.IsDir() {
			if relative != "" && shouldSkipWatchDir(relative) {
				return filepath.SkipDir
			}
			return nil
		}

		if !shouldWatchFile(relative) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[relative] = watchedFileState{
			size:    info.Size(),
			modTime: info.ModTime().UnixNano(),
		}
		return nil
	})
}

func shouldScanRootDir(relativePath string) bool {
	switch relativePath {
	case "config", "data", "dever", "middleware", "module", "package":
		return true
	default:
		return false
	}
}

func detectWatchedFileChanges(
	previous map[string]watchedFileState,
	current map[string]watchedFileState,
) []string {
	changes := make([]string, 0)

	for path, currentState := range current {
		previousState, ok := previous[path]
		if !ok || previousState != currentState {
			changes = append(changes, path)
		}
	}
	for path := range previous {
		if _, ok := current[path]; !ok {
			changes = append(changes, path)
		}
	}

	sort.Strings(changes)
	return changes
}

func shouldSkipWatchDir(relativePath string) bool {
	if relativePath == "" {
		return false
	}

	switch relativePath {
	case ".git", ".idea", ".vscode", "node_modules", "tmp", "vendor", "data/log":
		return true
	default:
		return false
	}
}

func shouldWatchFile(relativePath string) bool {
	if relativePath == "" {
		return false
	}

	switch {
	case relativePath == "data/router.go":
		return false
	case relativePath == "data/load/model.go":
		return false
	case relativePath == "data/load/service.go":
		return false
	case strings.HasPrefix(relativePath, "data/migrations/"):
		return false
	}

	ext := strings.ToLower(filepath.Ext(relativePath))
	switch ext {
	case ".go", ".json", ".jsonc", ".yaml", ".yml", ".toml", ".mod", ".sum":
		return true
	default:
		return false
	}
}

func requiresProjectInit(paths []string) bool {
	for _, path := range paths {
		if !isGeneratedSourcePath(path) {
			continue
		}
		if strings.HasSuffix(path, ".go") &&
			(strings.Contains(path, "/api/") || strings.Contains(path, "/service/") || strings.Contains(path, "/model/")) {
			return true
		}
	}
	return false
}

func requiresBinaryRebuild(paths []string) bool {
	for _, path := range paths {
		if strings.HasPrefix(path, "package/") {
			return true
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go", ".mod", ".sum":
			return true
		}
	}
	return false
}

func isGeneratedSourcePath(path string) bool {
	return strings.HasPrefix(path, "module/") || strings.HasPrefix(path, "package/")
}

func formatChangedPaths(paths []string) string {
	if len(paths) <= 4 {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s 等 %d 个文件", strings.Join(paths[:4], ", "), len(paths))
}

func normalizeWatchPath(projectRoot, current string) string {
	relative, err := filepath.Rel(projectRoot, current)
	if err != nil {
		return filepath.ToSlash(current)
	}
	if relative == "." {
		return ""
	}
	return filepath.ToSlash(relative)
}
