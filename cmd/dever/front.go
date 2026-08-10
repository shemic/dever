package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type frontBuildOptions struct {
	projectRoot string
	target      string
}

type frontPluginTarget struct {
	name string
	kind string
	root string
}

type frontPluginPublishPaths struct {
	current  string
	next     string
	previous string
}

func runFront(args []string) {
	if len(args) == 0 {
		printFrontUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "build":
		runFrontBuildCommand(args[1:])
	default:
		printFrontUsage()
		os.Exit(1)
	}
}

func printFrontUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever front - 前端插件命令

Usage:
    dever front build [--project-root=.] [name]
`)
}

func runFrontBuildCommand(args []string) {
	fs := flag.NewFlagSet("front build", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "front build 参数解析失败: %v\n", err)
		os.Exit(1)
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "front build 最多只接受一个插件名称，例如：dever front build bot")
		os.Exit(1)
	}

	target := ""
	if fs.NArg() == 1 {
		target = fs.Arg(0)
	}

	if err := runFrontBuild(frontBuildOptions{
		projectRoot: resolveProjectRoot(*projectRoot),
		target:      target,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "front build 执行失败: %v\n", err)
		os.Exit(1)
	}
}

func runFrontBuild(options frontBuildOptions) error {
	targets, err := discoverFrontPluginTargets(options.projectRoot, options.target)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if strings.TrimSpace(options.target) == "" {
			fmt.Println("dever front build: 未发现需要构建的本地前端插件，跳过")
			return nil
		}
		fmt.Printf("dever front build: %s 无需本地构建\n", options.target)
		return nil
	}

	for _, target := range targets {
		if err := buildFrontPlugin(options.projectRoot, target); err != nil {
			return err
		}
	}
	return nil
}

func discoverFrontPluginTargets(projectRoot, rawTarget string) ([]frontPluginTarget, error) {
	target := strings.TrimSpace(rawTarget)

	var targets []frontPluginTarget
	components, err := listActiveComponentSources(projectRoot)
	if err != nil {
		return nil, err
	}
	matchedTarget := false
	for _, current := range components {
		if target != "" && target != current.name {
			continue
		}
		matchedTarget = true
		hasSource := hasFrontPluginSource(current.root)
		hasDist := hasFrontPluginDist(current.root)
		if !hasSource {
			continue
		}
		if !current.editable {
			if hasDist {
				fmt.Printf("dever front build: %s/%s 已有 dist，跳过外部 package 源码构建\n", current.source, current.name)
				continue
			}
			return nil, fmt.Errorf("%s/%s 是外部 Go module package，存在 front/src/plugin.ts 但缺少 front/dist/manifest.json；请在 package 发布前构建 dist", current.source, current.name)
		}
		targets = append(targets, frontPluginTarget{
			name: current.name,
			kind: current.source,
			root: filepath.Join(current.root, "front"),
		})
	}

	if target != "" && !matchedTarget {
		return nil, fmt.Errorf("未发现组件: %s", target)
	}
	return targets, nil
}

func hasFrontPluginDist(componentRoot string) bool {
	info, err := os.Stat(frontPluginDistManifestPath(componentRoot))
	return err == nil && !info.IsDir()
}

func hasFrontPluginSource(componentRoot string) bool {
	info, err := os.Stat(frontPluginSourceEntryPath(componentRoot))
	return err == nil && !info.IsDir()
}

func frontPluginDistManifestPath(componentRoot string) string {
	return filepath.Join(componentRoot, "front", "dist", "manifest.json")
}

func frontPluginSourceEntryPath(componentRoot string) string {
	return filepath.Join(componentRoot, "front", "src", "plugin.ts")
}

func buildFrontPlugin(projectRoot string, target frontPluginTarget) error {
	compilerRoot, err := resolveFrontCompilerRoot(projectRoot)
	if err != nil {
		return err
	}
	if err := ensureFrontCompilerDependencies(projectRoot, compilerRoot); err != nil {
		return err
	}
	publishPaths, err := newFrontPluginPublishPaths(target.root)
	if err != nil {
		return err
	}
	if err := ensureFrontPluginPublishPathsAvailable(publishPaths); err != nil {
		return err
	}

	fmt.Printf("dever front build: 构建 %s/%s\n", target.kind, target.name)
	cmd := exec.Command(
		"pnpm",
		"--dir",
		compilerRoot,
		"exec",
		"vite",
		"build",
		"--config",
		"vite.config.ts",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = frontCompilerEnv(projectRoot, map[string]string{
		frontPluginNameEnv:       target.name,
		frontPluginRootEnv:       filepath.Dir(publishPaths.current),
		frontPluginRootsEnv:      "",
		frontPluginOutputRootEnv: publishPaths.next,
	})
	if err := cmd.Run(); err != nil {
		if cleanupErr := os.RemoveAll(publishPaths.next); cleanupErr != nil {
			return fmt.Errorf("%s/%s 构建失败: %w；清理 staging 失败: %v", target.kind, target.name, err, cleanupErr)
		}
		return fmt.Errorf("%s/%s 构建失败: %w", target.kind, target.name, err)
	}
	if err := validateBuiltFrontPlugin(publishPaths.next); err != nil {
		if cleanupErr := os.RemoveAll(publishPaths.next); cleanupErr != nil {
			return fmt.Errorf("%s/%s 构建产物无效: %w；清理 staging 失败: %v", target.kind, target.name, err, cleanupErr)
		}
		return fmt.Errorf("%s/%s 构建产物无效: %w", target.kind, target.name, err)
	}
	if err := publishBuiltFrontPlugin(publishPaths); err != nil {
		if cleanupErr := os.RemoveAll(publishPaths.next); cleanupErr != nil {
			return fmt.Errorf("%s/%s 发布失败: %w；清理 staging 失败: %v", target.kind, target.name, err, cleanupErr)
		}
		return fmt.Errorf("%s/%s 发布失败: %w", target.kind, target.name, err)
	}
	return nil
}

func newFrontPluginPublishPaths(pluginRoot string) (frontPluginPublishPaths, error) {
	root, err := filepath.Abs(strings.TrimSpace(pluginRoot))
	if err != nil {
		return frontPluginPublishPaths{}, fmt.Errorf("解析前端插件目录失败: %w", err)
	}
	if root == "" || root == string(filepath.Separator) {
		return frontPluginPublishPaths{}, fmt.Errorf("前端插件目录无效: %q", pluginRoot)
	}
	info, err := os.Stat(root)
	if err != nil {
		return frontPluginPublishPaths{}, fmt.Errorf("读取前端插件目录失败: %s: %w", root, err)
	}
	if !info.IsDir() {
		return frontPluginPublishPaths{}, fmt.Errorf("前端插件根路径不是目录: %s", root)
	}
	suffix := fmt.Sprintf("%d", os.Getpid())
	paths := frontPluginPublishPaths{
		current:  filepath.Join(root, "dist"),
		next:     filepath.Join(root, ".dist-next-"+suffix),
		previous: filepath.Join(root, ".dist-prev-"+suffix),
	}
	for _, candidate := range []string{paths.current, paths.next, paths.previous} {
		if filepath.Dir(candidate) != root {
			return frontPluginPublishPaths{}, fmt.Errorf("前端插件发布目录越界: %s", candidate)
		}
	}
	return paths, nil
}

func ensureFrontPluginPublishPathsAvailable(paths frontPluginPublishPaths) error {
	for _, candidate := range []string{paths.next, paths.previous} {
		if _, err := os.Lstat(candidate); err == nil {
			return fmt.Errorf("前端插件发布临时目录已存在，请先人工确认后处理: %s", candidate)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("检查前端插件发布临时目录失败: %s: %w", candidate, err)
		}
	}
	return nil
}

func validateBuiltFrontPlugin(stagingRoot string) error {
	manifest := filepath.Join(stagingRoot, "manifest.json")
	info, err := os.Stat(manifest)
	if err != nil {
		return fmt.Errorf("缺少 manifest.json: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("manifest.json 不能是目录: %s", manifest)
	}
	return nil
}

func publishBuiltFrontPlugin(paths frontPluginPublishPaths) error {
	hadCurrent, err := pathExists(paths.current)
	if err != nil {
		return err
	}
	if hadCurrent {
		if err := os.Rename(paths.current, paths.previous); err != nil {
			return fmt.Errorf("保留当前 dist 失败: %w", err)
		}
	}
	if err := os.Rename(paths.next, paths.current); err != nil {
		if hadCurrent {
			if restoreErr := os.Rename(paths.previous, paths.current); restoreErr != nil {
				return fmt.Errorf("切换新 dist 失败: %w；恢复旧 dist 失败: %v", err, restoreErr)
			}
		}
		return fmt.Errorf("切换新 dist 失败: %w", err)
	}
	if hadCurrent {
		if err := os.RemoveAll(paths.previous); err != nil {
			fmt.Fprintf(os.Stderr, "dever front build: 新 dist 已发布，但清理旧目录失败，请人工处理 %s: %v\n", paths.previous, err)
		}
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("检查路径失败: %s: %w", path, err)
}
