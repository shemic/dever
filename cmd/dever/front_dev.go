package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultFrontPluginDevPort = 5174
	frontPluginDevPortOffset  = 10000
	frontPluginDevPortRange   = 20
	frontPluginDevWait        = 8 * time.Second
	frontPluginDevOutputLimit = 32 * 1024
	maxTCPPort                = 65535
)

type frontPluginDevPortConfig struct {
	port   int
	strict bool
}

type frontPluginDevServer struct {
	cmd    *exec.Cmd
	done   chan error
	url    string
	output *frontPluginDevOutput
}

type frontPluginDevOutput struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newFrontPluginDevOutput(limit int) *frontPluginDevOutput {
	return &frontPluginDevOutput{limit: limit}
}

func (o *frontPluginDevOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.data = append(o.data, p...)
	if o.limit > 0 && len(o.data) > o.limit {
		o.data = append([]byte(nil), o.data[len(o.data)-o.limit:]...)
	}
	return len(p), nil
}

func (o *frontPluginDevOutput) String() string {
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.TrimSpace(string(o.data))
}

func startFrontPluginDevServer(projectRoot string) (*frontPluginDevServer, error) {
	if value, ok := frontPluginDevEnabledFromEnv(); ok && !value {
		return nil, nil
	}

	plugins, err := discoverRunFrontPluginSources(projectRoot)
	if err != nil {
		return nil, err
	}
	if len(plugins) == 0 {
		return nil, nil
	}

	compilerRoot, err := resolveFrontCompilerRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	if err := ensureFrontCompilerDependencies(compilerRoot); err != nil {
		return nil, err
	}

	port, err := resolveFrontPluginDevPort(projectRoot, compilerRoot, frontPluginDevPortConfigForProject(projectRoot))
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	output := newFrontPluginDevOutput(frontPluginDevOutputLimit)
	cmd := exec.Command(
		"pnpm",
		"--dir",
		compilerRoot,
		"exec",
		"vite",
		"--config",
		filepath.Join(compilerRoot, frontCompilerViteConfig),
		"--host",
		"127.0.0.1",
		"--port",
		strconv.Itoa(port),
		"--strictPort",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.Env = frontCompilerEnv(projectRoot, nil)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动前端插件源码编译服务失败: %w", err)
	}

	server := &frontPluginDevServer{
		cmd:    cmd,
		done:   make(chan error, 1),
		url:    url,
		output: output,
	}
	go func() {
		server.done <- cmd.Wait()
	}()

	processExited, err := waitForFrontPluginDevServer(url, server.done, frontPluginDevWait)
	if err != nil {
		if !processExited {
			_ = server.stop(processStopTimeout)
		}
		return nil, withFrontPluginDevOutput(err, output)
	}

	log.Printf("已启动前端插件源码编译服务（plugins=%s, port=%d）", strings.Join(plugins, ","), port)
	return server, nil
}

func withFrontPluginDevOutput(err error, output *frontPluginDevOutput) error {
	if err == nil {
		return nil
	}
	text := output.String()
	if text == "" {
		return err
	}
	return fmt.Errorf("%w\n前端插件编译输出:\n%s", err, text)
}

func (s *frontPluginDevServer) backendEnv() map[string]string {
	if s == nil || strings.TrimSpace(s.url) == "" {
		return nil
	}
	return map[string]string{
		"DEVER_FRONT_PLUGIN_DEV":     "1",
		"DEVER_FRONT_PLUGIN_DEV_URL": s.url,
	}
}

func (s *frontPluginDevServer) doneChannel() <-chan error {
	if s == nil {
		return nil
	}
	return s.done
}

func (s *frontPluginDevServer) exitError(err error) error {
	err = normalizeProcessExitError(err)
	if err == nil {
		return fmt.Errorf("前端插件源码编译服务已退出")
	}
	return withFrontPluginDevOutput(err, s.output)
}

func (s *frontPluginDevServer) stop(timeout time.Duration) error {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	processGroupID := -s.cmd.Process.Pid
	if err := syscall.Kill(processGroupID, syscall.SIGTERM); err != nil && !isMissingProcessError(err) {
		_ = syscall.Kill(processGroupID, syscall.SIGKILL)
		return nil
	}

	select {
	case err := <-s.done:
		return normalizeStoppedProcessExitError(err)
	case <-time.After(timeout):
		_ = syscall.Kill(processGroupID, syscall.SIGKILL)
		<-s.done
		return nil
	}
}

func discoverRunFrontPluginSources(projectRoot string) ([]string, error) {
	names := map[string]struct{}{}
	for _, root := range frontPluginSourceRoots(projectRoot) {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginName := entry.Name()
			pluginEntry := filepath.Join(root, pluginName, "front", "src", "plugin.ts")
			if info, err := os.Stat(pluginEntry); err == nil && !info.IsDir() {
				names[pluginName] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func frontPluginSourceRoots(projectRoot string) []string {
	return uniqueExistingParentDirs([]string{
		filepath.Join(projectRoot, "package"),
		filepath.Join(projectRoot, "module"),
		filepath.Join(projectRoot, "backend", "package"),
		filepath.Join(projectRoot, "backend", "module"),
	})
}

func uniqueExistingParentDirs(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		abs, err := filepath.Abs(item)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		result = append(result, abs)
	}
	return result
}

func frontPluginDevPortConfigForProject(projectRoot string) frontPluginDevPortConfig {
	if config, ok := frontPluginDevPortFromEnv(); ok {
		return config
	}
	return frontPluginDevPortConfig{
		port: derivedFrontPluginDevPort(projectRoot),
	}
}

func frontPluginDevPortFromEnv() (frontPluginDevPortConfig, bool) {
	value := strings.TrimSpace(os.Getenv("DEVER_FRONT_PLUGIN_DEV_PORT"))
	if value == "" {
		return frontPluginDevPortConfig{}, false
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return frontPluginDevPortConfig{port: defaultFrontPluginDevPort}, true
	}
	return frontPluginDevPortConfig{port: port, strict: true}, true
}

func derivedFrontPluginDevPort(projectRoot string) int {
	httpPort, err := loadRunListenPort(projectRoot)
	if err != nil || httpPort <= 0 {
		return defaultFrontPluginDevPort
	}

	port := httpPort + frontPluginDevPortOffset
	if port > 0 && port <= maxTCPPort {
		return port
	}
	return defaultFrontPluginDevPort
}

func frontPluginDevEnabledFromEnv() (bool, bool) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("DEVER_FRONT_PLUGIN_DEV")))
	switch value {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func resolveFrontPluginDevPort(projectRoot, compilerRoot string, config frontPluginDevPortConfig) (int, error) {
	if config.port <= 0 {
		config.port = defaultFrontPluginDevPort
	}

	attempts := frontPluginDevPortRange
	if config.strict {
		attempts = 1
	}

	for offset := 0; offset < attempts; offset++ {
		port := config.port + offset
		free, err := releaseOwnedFrontPluginDevPort(projectRoot, compilerRoot, port)
		if err != nil {
			return 0, err
		}
		if free {
			return port, nil
		}
	}

	if config.strict {
		return 0, fmt.Errorf("前端插件源码编译端口 %d 已被占用", config.port)
	}
	return 0, fmt.Errorf("前端插件源码编译端口 %d-%d 均已被占用", config.port, config.port+attempts-1)
}

func releaseOwnedFrontPluginDevPort(projectRoot, compilerRoot string, port int) (bool, error) {
	pids, err := listeningPIDsOnPort(port)
	if err != nil {
		return false, fmt.Errorf("检查前端插件源码编译端口 %d 占用失败: %w", port, err)
	}
	if len(pids) == 0 {
		return true, nil
	}

	ownedPIDs := make([]int, 0, len(pids))
	foreignPIDs := make([]int, 0, len(pids))
	for _, pid := range pids {
		if isProjectFrontPluginDevProcess(pid, projectRoot, compilerRoot) {
			ownedPIDs = append(ownedPIDs, pid)
			continue
		}
		foreignPIDs = append(foreignPIDs, pid)
	}

	handledSupervisors := map[int]bool{}
	for _, pid := range ownedPIDs {
		if supervisorPID := projectRunAncestorPID(pid); supervisorPID > 0 {
			if handledSupervisors[supervisorPID] {
				continue
			}
			handledSupervisors[supervisorPID] = true
			log.Printf("检测到旧 dever run 监督进程持有前端插件端口 %d，正在关闭 pid=%d", port, supervisorPID)
			if err := terminateProcess(supervisorPID, processStopTimeout); err != nil {
				return false, fmt.Errorf("关闭旧 dever run 监督进程 pid=%d 失败: %w", supervisorPID, err)
			}
			continue
		}

		log.Printf("检测到旧前端插件源码编译服务占用端口 %d，正在关闭 pid=%d", port, pid)
		if err := terminateFrontPluginDevProcess(pid, processStopTimeout); err != nil {
			return false, fmt.Errorf("关闭旧前端插件源码编译服务 pid=%d 失败: %w", pid, err)
		}
	}
	if len(foreignPIDs) > 0 {
		return false, nil
	}
	if len(ownedPIDs) == 0 {
		return false, nil
	}
	if err := waitPortReleased(port, processStopTimeout); err != nil {
		return false, err
	}
	return true, nil
}

func terminateFrontPluginDevProcess(pid int, timeout time.Duration) error {
	pgid, err := syscall.Getpgid(pid)
	currentPGID, currentErr := syscall.Getpgid(os.Getpid())
	if err != nil || pgid <= 0 || (currentErr == nil && pgid == currentPGID) {
		return terminateProcess(pid, timeout)
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !isMissingProcessError(err) {
		return terminateProcess(pid, timeout)
	}
	if waitProcessExit(pid, timeout) {
		return nil
	}

	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	if waitProcessExit(pid, timeout) {
		return nil
	}
	return fmt.Errorf("进程未退出")
}

func isProjectFrontPluginDevProcess(pid int, projectRoot, compilerRoot string) bool {
	if sameProcessPath(procEnvValue(pid, frontPluginProjectRootEnv), projectRoot) {
		return true
	}

	if !sameProcessPath(readProcLink(pid, "cwd"), projectRoot) {
		return false
	}
	cmdline := readProcCmdline(pid)
	if !strings.Contains(cmdline, "vite") {
		return false
	}
	return strings.Contains(filepath.ToSlash(cmdline), filepath.ToSlash(compilerRoot))
}

func projectRunAncestorPID(pid int) int {
	currentPID := os.Getpid()
	supervisorPID := 0
	for parentPID := procParentPID(pid); parentPID > 1; parentPID = procParentPID(parentPID) {
		if parentPID == currentPID {
			return 0
		}
		if isDeverRunSupervisorProcess(parentPID) {
			supervisorPID = parentPID
		}
	}
	return supervisorPID
}

func procEnvValue(pid int, name string) string {
	if pid <= 0 || strings.TrimSpace(name) == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "environ"))
	if err != nil || len(data) == 0 {
		return ""
	}
	prefix := name + "="
	for _, item := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

func readProcCmdline(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", " "))
}

func waitForFrontPluginDevServer(url string, done <-chan error, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			if exitErr := normalizeProcessExitError(err); exitErr != nil {
				return true, fmt.Errorf("前端插件源码编译服务提前退出: %w", exitErr)
			}
			return true, fmt.Errorf("前端插件源码编译服务提前退出")
		default:
		}

		if frontPluginDevServerReady(url) {
			return false, nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return false, fmt.Errorf("前端插件源码编译服务启动超时: %s", url)
}

func frontPluginDevServerReady(url string) bool {
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url + "/@vite/client")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}
