package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultDaemonName = "default"
	daemonDir         = "tmp/dever/daemon"
)

type daemonOptions struct {
	projectRoot string
	name        string
	command     []string
}

type daemonMetadata struct {
	Name        string   `json:"name"`
	PID         int      `json:"pid"`
	ProjectRoot string   `json:"project_root"`
	Command     []string `json:"command"`
	LogPath     string   `json:"log_path"`
	StartedAt   string   `json:"started_at"`
}

type daemonPaths struct {
	dir      string
	pidPath  string
	metaPath string
	logPath  string
}

func runDaemon(args []string) {
	if len(args) == 0 {
		printDaemonUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "start":
		options, err := parseDaemonCommand("daemon start", args[1:], true)
		exitOnDaemonError(err)
		exitOnDaemonError(startDaemon(options))
	case "stop":
		options, err := parseDaemonCommand("daemon stop", args[1:], false)
		exitOnDaemonError(err)
		exitOnDaemonError(stopDaemon(options, true))
	case "restart":
		options, err := parseDaemonCommand("daemon restart", args[1:], true)
		exitOnDaemonError(err)
		exitOnDaemonError(restartDaemon(options))
	case "status":
		options, err := parseDaemonCommand("daemon status", args[1:], false)
		exitOnDaemonError(err)
		exitOnDaemonError(statusDaemon(options))
	case "logs":
		options, follow, err := parseDaemonLogsCommand(args[1:])
		exitOnDaemonError(err)
		exitOnDaemonError(logsDaemon(options, follow))
	default:
		printDaemonUsage()
		os.Exit(1)
	}
}

func printDaemonUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever daemon - 后台运行命令

Usage:
    dever daemon start [--project-root=.] [--name=default] -- <command...>
    dever daemon stop [--project-root=.] [--name=default]
    dever daemon restart [--project-root=.] [--name=default] [-- <command...>]
    dever daemon status [--project-root=.] [--name=default]
    dever daemon logs [--project-root=.] [--name=default] [-f]
`)
}

func exitOnDaemonError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "daemon 执行失败: %v\n", err)
	os.Exit(1)
}

func parseDaemonCommand(name string, args []string, includeCommand bool) (daemonOptions, error) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	daemonName := fs.String("name", defaultDaemonName, "后台进程名称")
	if err := fs.Parse(args); err != nil {
		return daemonOptions{}, err
	}

	options, err := normalizeDaemonOptions(*projectRoot, *daemonName)
	if err != nil {
		return daemonOptions{}, err
	}
	if includeCommand {
		options.command = trimDaemonCommand(fs.Args())
	}
	return options, nil
}

func parseDaemonLogsCommand(args []string) (daemonOptions, bool, error) {
	fs := flag.NewFlagSet("daemon logs", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	daemonName := fs.String("name", defaultDaemonName, "后台进程名称")
	follow := fs.Bool("f", false, "持续跟随日志输出")
	if err := fs.Parse(args); err != nil {
		return daemonOptions{}, false, err
	}
	if fs.NArg() > 0 {
		return daemonOptions{}, false, fmt.Errorf("logs 不接受额外参数")
	}

	options, err := normalizeDaemonOptions(*projectRoot, *daemonName)
	return options, *follow, err
}

func normalizeDaemonOptions(projectRoot, name string) (daemonOptions, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		normalizedName = defaultDaemonName
	}
	if !validDaemonName(normalizedName) {
		return daemonOptions{}, fmt.Errorf("后台进程名称只能包含字母、数字、点、下划线和中划线: %s", name)
	}

	return daemonOptions{
		projectRoot: resolveProjectRoot(projectRoot),
		name:        normalizedName,
	}, nil
}

func trimDaemonCommand(args []string) []string {
	command := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		command = append(command, arg)
	}
	return command
}

func validDaemonName(name string) bool {
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '.', char == '_', char == '-':
		default:
			return false
		}
	}
	return true
}

func startDaemon(options daemonOptions) error {
	if len(options.command) == 0 {
		return fmt.Errorf("start 需要指定要后台运行的命令，例如：dever daemon start -- dever run")
	}

	paths := daemonFilePaths(options)
	if err := os.MkdirAll(paths.dir, 0o755); err != nil {
		return fmt.Errorf("创建后台进程目录失败: %w", err)
	}

	if pid := readPIDFile(paths.pidPath); processExists(pid) {
		return fmt.Errorf("后台进程 %s 已在运行 pid=%d；请先 stop 或使用 restart", options.name, pid)
	}

	logFile, err := os.OpenFile(paths.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer logFile.Close()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("打开空输入失败: %w", err)
	}
	defer devNull.Close()

	writeDaemonLogHeader(logFile, options)

	cmd := exec.Command(options.command[0], options.command[1:]...)
	cmd.Dir = options.projectRoot
	cmd.Env = mergeCommandEnv(os.Environ(), map[string]string{
		callerDirEnv: options.projectRoot,
	})
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动后台命令失败: %w", err)
	}

	metadata := daemonMetadata{
		Name:        options.name,
		PID:         cmd.Process.Pid,
		ProjectRoot: options.projectRoot,
		Command:     append([]string(nil), options.command...),
		LogPath:     paths.logPath,
		StartedAt:   time.Now().Format(time.RFC3339),
	}
	if err := saveDaemonMetadata(paths, metadata); err != nil {
		_ = os.Remove(paths.pidPath)
		_ = os.Remove(paths.metaPath)
		_ = terminateProcess(cmd.Process.Pid, processStopTimeout)
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "dever daemon: 释放后台进程句柄失败: %v\n", err)
	}

	fmt.Printf("dever daemon: 已启动 %s pid=%d\n", options.name, metadata.PID)
	fmt.Printf("dever daemon: 命令 %s\n", formatDaemonCommand(options.command))
	fmt.Printf("dever daemon: 日志 %s\n", paths.logPath)
	return nil
}

func stopDaemon(options daemonOptions, requireExisting bool) error {
	paths := daemonFilePaths(options)
	metadata, _ := loadDaemonMetadata(paths)
	pid := readPIDFile(paths.pidPath)
	if pid <= 0 && metadata.PID > 0 {
		pid = metadata.PID
	}

	if pid <= 0 {
		if requireExisting {
			return fmt.Errorf("后台进程 %s 未启动", options.name)
		}
		return nil
	}
	if !processExists(pid) {
		_ = os.Remove(paths.pidPath)
		if requireExisting {
			fmt.Printf("dever daemon: %s 已停止，清理失效 pid=%d\n", options.name, pid)
		}
		return nil
	}

	if err := terminateProcess(pid, processStopTimeout); err != nil {
		return fmt.Errorf("停止后台进程 %s pid=%d 失败: %w", options.name, pid, err)
	}
	_ = os.Remove(paths.pidPath)
	fmt.Printf("dever daemon: 已停止 %s pid=%d\n", options.name, pid)
	return nil
}

func restartDaemon(options daemonOptions) error {
	if len(options.command) == 0 {
		metadata, err := loadDaemonMetadata(daemonFilePaths(options))
		if err != nil {
			return fmt.Errorf("restart 未指定命令，且没有上次运行记录；请使用：dever daemon restart -- <command...>")
		}
		options.command = append([]string(nil), metadata.Command...)
	}
	if len(options.command) == 0 {
		return fmt.Errorf("restart 需要指定要后台运行的命令")
	}

	if err := stopDaemon(options, false); err != nil {
		return err
	}
	return startDaemon(options)
}

func statusDaemon(options daemonOptions) error {
	paths := daemonFilePaths(options)
	metadata, metaErr := loadDaemonMetadata(paths)
	pid := readPIDFile(paths.pidPath)
	if pid <= 0 && metadata.PID > 0 {
		pid = metadata.PID
	}

	if pid > 0 && processExists(pid) {
		fmt.Printf("dever daemon: %s 正在运行 pid=%d\n", options.name, pid)
		if metaErr == nil {
			printDaemonMetadata(metadata)
		}
		return nil
	}

	if pid > 0 {
		_ = os.Remove(paths.pidPath)
	}
	fmt.Printf("dever daemon: %s 未运行\n", options.name)
	if metaErr == nil {
		fmt.Printf("dever daemon: 上次命令 %s\n", formatDaemonCommand(metadata.Command))
		fmt.Printf("dever daemon: 日志 %s\n", metadata.LogPath)
	}
	return nil
}

func logsDaemon(options daemonOptions, follow bool) error {
	paths := daemonFilePaths(options)
	if _, err := os.Stat(paths.logPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("后台进程 %s 暂无日志: %s", options.name, paths.logPath)
		}
		return err
	}
	return streamDaemonLog(paths.logPath, follow)
}

func daemonFilePaths(options daemonOptions) daemonPaths {
	dir := filepath.Join(options.projectRoot, daemonDir)
	base := options.name
	return daemonPaths{
		dir:      dir,
		pidPath:  filepath.Join(dir, base+".pid"),
		metaPath: filepath.Join(dir, base+".json"),
		logPath:  filepath.Join(dir, base+".log"),
	}
}

func writeDaemonLogHeader(writer io.Writer, options daemonOptions) {
	fmt.Fprintf(writer, "\n[%s] dever daemon start %s\n", time.Now().Format(time.RFC3339), options.name)
	fmt.Fprintf(writer, "cwd: %s\n", options.projectRoot)
	fmt.Fprintf(writer, "cmd: %s\n\n", formatDaemonCommand(options.command))
}

func saveDaemonMetadata(paths daemonPaths, metadata daemonMetadata) error {
	if err := os.WriteFile(paths.pidPath, []byte(fmt.Sprintf("%d\n", metadata.PID)), 0o644); err != nil {
		return fmt.Errorf("写入 pid 文件失败: %w", err)
	}

	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("生成后台进程元数据失败: %w", err)
	}
	if err := os.WriteFile(paths.metaPath, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入后台进程元数据失败: %w", err)
	}
	return nil
}

func loadDaemonMetadata(paths daemonPaths) (daemonMetadata, error) {
	content, err := os.ReadFile(paths.metaPath)
	if err != nil {
		return daemonMetadata{}, err
	}
	var metadata daemonMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return daemonMetadata{}, err
	}
	return metadata, nil
}

func printDaemonMetadata(metadata daemonMetadata) {
	if metadata.StartedAt != "" {
		fmt.Printf("dever daemon: 启动时间 %s\n", metadata.StartedAt)
	}
	if len(metadata.Command) > 0 {
		fmt.Printf("dever daemon: 命令 %s\n", formatDaemonCommand(metadata.Command))
	}
	if metadata.LogPath != "" {
		fmt.Printf("dever daemon: 日志 %s\n", metadata.LogPath)
	}
}

func formatDaemonCommand(command []string) string {
	if len(command) == 0 {
		return ""
	}
	return strings.Join(command, " ")
}

func streamDaemonLog(path string, follow bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(os.Stdout, file); err != nil {
		return err
	}
	if !follow {
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := io.Copy(os.Stdout, file); err != nil {
				return err
			}
		}
	}
}
