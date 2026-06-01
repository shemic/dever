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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shemic/dever/config"
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
	listenPort int
	env        map[string]string
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

	frontDev, err := startFrontPluginDevServer(options.projectRoot)
	if err != nil {
		return err
	}
	if frontDev != nil {
		defer frontDev.stop(processStopTimeout)
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
	if frontDev != nil {
		process.env = frontDev.backendEnv()
	}
	if listenPort, err := loadRunListenPort(options.projectRoot); err != nil {
		log.Printf("读取监听端口失败，跳过旧进程清理: %v", err)
	} else {
		process.listenPort = listenPort
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
	if err := p.stopStalePortOwners(processStopTimeout); err != nil {
		return err
	}

	cmd := exec.Command(p.binaryPath)
	cmd.Dir = p.root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = mergeCommandEnv(os.Environ(), p.env)
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

func (p *watchedProcess) stopStalePortOwners(timeout time.Duration) error {
	if p == nil || p.listenPort <= 0 {
		return nil
	}
	pids, err := listeningPIDsOnPort(p.listenPort)
	if err != nil {
		return fmt.Errorf("检查端口 %d 占用失败: %w", p.listenPort, err)
	}
	if len(pids) == 0 {
		return nil
	}

	ownedPIDs := make([]int, 0, len(pids))
	foreignPIDs := make([]int, 0)
	for _, pid := range pids {
		if pid == os.Getpid() {
			continue
		}
		if isProjectRunProcess(pid, p.root, p.binaryPath) {
			ownedPIDs = append(ownedPIDs, pid)
			continue
		}
		foreignPIDs = append(foreignPIDs, pid)
	}
	if len(foreignPIDs) > 0 {
		return fmt.Errorf("端口 %d 已被非当前项目 dever run 进程占用: %s", p.listenPort, formatPIDs(foreignPIDs))
	}
	for _, pid := range ownedPIDs {
		log.Printf("检测到旧运行进程占用端口 %d，正在关闭 pid=%d", p.listenPort, pid)
		if err := terminateProcess(pid, timeout); err != nil {
			return fmt.Errorf("关闭旧运行进程 pid=%d 失败: %w", pid, err)
		}
	}
	if err := waitPortReleased(p.listenPort, timeout); err != nil {
		return err
	}
	return nil
}

func loadRunListenPort(projectRoot string) (int, error) {
	cfg, err := config.Load(filepath.Join(projectRoot, config.DefaultPath))
	if err != nil {
		return 0, err
	}
	return cfg.HTTP.Port, nil
}

func listeningPIDsOnPort(port int) ([]int, error) {
	if port <= 0 {
		return nil, nil
	}
	inodes, err := listeningSocketInodes(port)
	if err != nil {
		return nil, err
	}
	if len(inodes) == 0 {
		return nil, nil
	}
	return pidsBySocketInode(inodes)
}

func listeningSocketInodes(port int) (map[string]bool, error) {
	result := map[string]bool{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[0] == "sl" || fields[3] != "0A" {
				continue
			}
			if tcpLinePort(fields[1]) != port {
				continue
			}
			result[fields[9]] = true
		}
	}
	return result, nil
}

func tcpLinePort(localAddress string) int {
	_, portHex, ok := strings.Cut(localAddress, ":")
	if !ok {
		return 0
	}
	value, err := strconv.ParseInt(portHex, 16, 32)
	if err != nil {
		return 0
	}
	return int(value)
}

func pidsBySocketInode(inodes map[string]bool) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	pidSet := map[int]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if inodes[inode] {
				pidSet[pid] = true
				break
			}
		}
	}

	pids := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids, nil
}

func isProjectRunProcess(pid int, projectRoot string, binaryPath string) bool {
	if sameProcessPath(readProcLink(pid, "exe"), binaryPath) {
		return true
	}
	if sameProcessPath(firstProcCmdlineArg(pid), binaryPath) {
		return true
	}
	return sameProcessPath(readProcLink(pid, "cwd"), projectRoot)
}

func readProcLink(pid int, name string) string {
	target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), name))
	if err != nil {
		return ""
	}
	return target
}

func firstProcCmdlineArg(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || len(data) == 0 {
		return ""
	}
	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func sameProcessPath(left string, right string) bool {
	left = normalizeProcessPath(left)
	right = normalizeProcessPath(right)
	return left != "" && right != "" && left == right
}

func normalizeProcessPath(path string) string {
	path = strings.TrimSpace(strings.TrimSuffix(path, " (deleted)"))
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func terminateProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	signalProcessGroupOrPID(pid, syscall.SIGTERM)
	if waitProcessExit(pid, timeout) {
		return nil
	}
	signalProcessGroupOrPID(pid, syscall.SIGKILL)
	if waitProcessExit(pid, timeout) {
		return nil
	}
	return fmt.Errorf("进程未退出")
}

func signalProcessGroupOrPID(pid int, signal syscall.Signal) {
	if err := syscall.Kill(-pid, signal); err != nil && !isMissingProcessError(err) {
		_ = syscall.Kill(pid, signal)
	}
}

func waitProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !processExists(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || !isMissingProcessError(err)
}

func waitPortReleased(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pids, err := listeningPIDsOnPort(port)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("端口 %d 仍被占用: %s", port, formatPIDs(pids))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func formatPIDs(pids []int) string {
	if len(pids) == 0 {
		return ""
	}
	values := make([]string, 0, len(pids))
	for _, pid := range pids {
		values = append(values, strconv.Itoa(pid))
	}
	return strings.Join(values, ",")
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
