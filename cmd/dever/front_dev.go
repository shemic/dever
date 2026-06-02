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
	frontPluginDevWait        = 8 * time.Second
	frontPluginDevOutputLimit = 32 * 1024
)

type frontPluginDevServer struct {
	cmd  *exec.Cmd
	done chan error
	url  string
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

	port := frontPluginDevPort()
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	output := newFrontPluginDevOutput(frontPluginDevOutputLimit)
	cmd := exec.Command(
		"pnpm",
		"--dir",
		compilerRoot,
		"exec",
		"vite",
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
		cmd:  cmd,
		done: make(chan error, 1),
		url:  url,
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

	log.Printf("已启动前端插件源码编译服务（plugins=%s）", strings.Join(plugins, ","))
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
		return normalizeProcessExitError(err)
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

func frontPluginDevPort() int {
	value := strings.TrimSpace(os.Getenv("DEVER_FRONT_PLUGIN_DEV_PORT"))
	if value == "" {
		return defaultFrontPluginDevPort
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return defaultFrontPluginDevPort
	}
	return port
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
