package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBuildOS      = "linux"
	defaultBuildArch    = "amd64"
	defaultBuildBinary  = "server"
	defaultBuildCGO     = false
	releaseBuildLDFlags = "-s -w -buildid="
)

type releaseBuildOptions struct {
	projectRoot string
	target      string
	output      string
	goos        string
	goarch      string
	cgoEnabled  bool
}

type releaseBuildTarget struct {
	packageArg string
	outputName string
}

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	output := fs.String("output", "", "输出文件路径，默认自动推导")
	fs.StringVar(output, "o", "", "输出文件路径，默认自动推导")
	goos := fs.String("os", defaultBuildOS, "目标操作系统")
	goarch := fs.String("arch", defaultBuildArch, "目标架构")
	cgoEnabled := fs.Bool("cgo", defaultBuildCGO, "是否启用 CGO")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("build 参数解析失败: %v", err)
	}
	if fs.NArg() > 1 {
		log.Fatal("build 最多只接受一个构建目标，例如：dever build 或 dever build cmd/workflow-worker")
	}

	target := ""
	if fs.NArg() == 1 {
		target = fs.Arg(0)
	}

	options := releaseBuildOptions{
		projectRoot: resolveProjectRoot(*projectRoot),
		target:      target,
		output:      strings.TrimSpace(*output),
		goos:        normalizeBuildValue(*goos, defaultBuildOS),
		goarch:      normalizeBuildValue(*goarch, defaultBuildArch),
		cgoEnabled:  *cgoEnabled,
	}

	if err := runReleaseBuild(options); err != nil {
		log.Fatalf("build 执行失败: %v", err)
	}
}

func runReleaseBuild(options releaseBuildOptions) error {
	startedAt := time.Now()
	target, err := resolveReleaseBuildTarget(options.projectRoot, options.target)
	if err != nil {
		return err
	}

	outputPath, err := resolveReleaseBuildOutput(options.projectRoot, options.output, target.outputName, options.goos)
	if err != nil {
		return err
	}

	fmt.Printf("dever build: 目标 %s\n", target.packageArg)
	fmt.Printf("dever build: 输出 %s\n", outputPath)
	fmt.Printf(
		"dever build: 环境 GOOS=%s GOARCH=%s CGO_ENABLED=%s\n",
		options.goos,
		options.goarch,
		boolToEnvValue(options.cgoEnabled),
	)
	fmt.Println("dever build: 开始构建...")

	if err := runGoBuild(goBuildSpec{
		dir:    options.projectRoot,
		target: target.packageArg,
		output: outputPath,
		env: map[string]string{
			"CGO_ENABLED": boolToEnvValue(options.cgoEnabled),
			"GOOS":        options.goos,
			"GOARCH":      options.goarch,
		},
		trimPath: true,
		buildVCS: "false",
		ldflags:  releaseBuildLDFlags,
		progress: "dever build",
	}); err != nil {
		return fmt.Errorf("打包 %s 失败: %w", target.packageArg, err)
	}

	fmt.Printf("dever build: 打包完成 %s (耗时 %s)\n", outputPath, time.Since(startedAt))
	return nil
}

func resolveReleaseBuildTarget(projectRoot, rawTarget string) (releaseBuildTarget, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		if err := ensureMainEntry(projectRoot); err != nil {
			return releaseBuildTarget{}, err
		}
		return releaseBuildTarget{
			packageArg: ".",
			outputName: defaultBuildBinary,
		}, nil
	}

	relativePath, err := normalizeReleaseBuildTarget(projectRoot, target)
	if err != nil {
		return releaseBuildTarget{}, err
	}
	if relativePath == "." {
		if err := ensureMainEntry(projectRoot); err != nil {
			return releaseBuildTarget{}, err
		}
		return releaseBuildTarget{
			packageArg: ".",
			outputName: defaultBuildBinary,
		}, nil
	}

	fullPath := filepath.Join(projectRoot, relativePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return releaseBuildTarget{}, fmt.Errorf("构建目标不存在: %s", rawTarget)
	}

	switch {
	case info.IsDir():
		if err := ensureMainEntry(fullPath); err != nil {
			return releaseBuildTarget{}, fmt.Errorf("构建目标 %s 无法打包: %w", rawTarget, err)
		}
		return releaseBuildTarget{
			packageArg: filepathToPackageArg(relativePath),
			outputName: filepath.Base(relativePath),
		}, nil
	case filepath.Base(relativePath) == "main.go":
		parent := filepath.Dir(relativePath)
		if parent == "." {
			if err := ensureMainEntry(projectRoot); err != nil {
				return releaseBuildTarget{}, err
			}
			return releaseBuildTarget{
				packageArg: ".",
				outputName: defaultBuildBinary,
			}, nil
		}
		if err := ensureMainEntry(filepath.Join(projectRoot, parent)); err != nil {
			return releaseBuildTarget{}, fmt.Errorf("构建目标 %s 无法打包: %w", rawTarget, err)
		}
		return releaseBuildTarget{
			packageArg: filepathToPackageArg(parent),
			outputName: filepath.Base(parent),
		}, nil
	default:
		return releaseBuildTarget{}, fmt.Errorf("构建目标必须是目录或 main.go: %s", rawTarget)
	}
}

func normalizeReleaseBuildTarget(projectRoot, rawTarget string) (string, error) {
	if strings.TrimSpace(rawTarget) == "" {
		return ".", nil
	}

	cleanTarget := filepath.Clean(strings.TrimSpace(rawTarget))
	if !filepath.IsAbs(cleanTarget) {
		if cleanTarget == "." {
			return ".", nil
		}
		if strings.HasPrefix(cleanTarget, ".."+string(filepath.Separator)) || cleanTarget == ".." {
			return "", fmt.Errorf("构建目标必须位于项目目录内: %s", rawTarget)
		}
		return cleanTarget, nil
	}

	relative, err := filepath.Rel(projectRoot, cleanTarget)
	if err != nil {
		return "", fmt.Errorf("解析构建目标失败: %w", err)
	}
	relative = filepath.Clean(relative)
	if strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", fmt.Errorf("构建目标必须位于项目目录内: %s", rawTarget)
	}
	return relative, nil
}

func ensureMainEntry(targetDir string) error {
	entryPath := filepath.Join(targetDir, "main.go")
	info, err := os.Stat(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("未找到入口文件: %s", entryPath)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("入口文件路径是目录: %s", entryPath)
	}
	return nil
}

func resolveReleaseBuildOutput(projectRoot, configuredOutput, defaultName, goos string) (string, error) {
	if strings.TrimSpace(configuredOutput) != "" {
		outputPath := strings.TrimSpace(configuredOutput)
		if !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(projectRoot, outputPath)
		}
		return filepath.Abs(outputPath)
	}

	outputName := defaultName
	if strings.EqualFold(goos, "windows") && !strings.HasSuffix(strings.ToLower(outputName), ".exe") {
		outputName += ".exe"
	}
	return filepath.Join(projectRoot, outputName), nil
}

func filepathToPackageArg(path string) string {
	clean := filepath.Clean(path)
	if clean == "." {
		return "."
	}
	return "./" + filepath.ToSlash(clean)
}

func boolToEnvValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func normalizeBuildValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
