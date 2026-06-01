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
			fmt.Println("dever front build: 未发现前端插件，跳过")
			return nil
		}
		return fmt.Errorf("未发现前端插件: %s", options.target)
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
			name := entry.Name()
			if target != "" && target != name {
				continue
			}
			frontRoot := filepath.Join(root, name, "front")
			pluginEntry := filepath.Join(frontRoot, "src", "plugin.ts")
			if _, err := os.Stat(pluginEntry); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			targets = append(targets, frontPluginTarget{
				name: name,
				kind: filepath.Base(root),
				root: frontRoot,
			})
		}
	}

	return targets, nil
}

func buildFrontPlugin(projectRoot string, target frontPluginTarget) error {
	compilerRoot, err := resolveFrontCompilerRoot(projectRoot)
	if err != nil {
		return err
	}
	if err := ensureFrontCompilerDependencies(compilerRoot); err != nil {
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
	cmd.Env = mergeCommandEnv(os.Environ(), map[string]string{
		"DEVER_FRONT_PLUGIN_NAME":         target.name,
		"DEVER_FRONT_PLUGIN_ROOT":         target.root,
		"DEVER_FRONT_PLUGIN_PROJECT_ROOT": projectRoot,
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s/%s 构建失败: %w", target.kind, target.name, err)
	}
	return nil
}
