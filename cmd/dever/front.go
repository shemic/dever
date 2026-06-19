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
		frontPluginNameEnv: target.name,
		frontPluginRootEnv: target.root,
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s/%s 构建失败: %w", target.kind, target.name, err)
	}
	return nil
}
