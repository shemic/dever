package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    devercmd "github.com/shemic/dever/cmd"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "routes":
		runRoutes(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever - 开发辅助命令

Usage:
    dever init [--project-root=.] [--skip-tidy]   # 执行 go mod tidy 并生成路由
    dever routes [--project-root=.]               # 仅生成路由
    dever migrate [--project-root=.] <database>   # 应用 data/table 中记录的表结构到目标数据库
`)
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	skipTidy := fs.Bool("skip-tidy", false, "跳过执行 go mod tidy")
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("init 参数解析失败: %v", err)
	}

	root := resolveProjectRoot(*projectRoot)
	if !*skipTidy {
		if err := runGoModTidy(root); err != nil {
			log.Fatalf("go mod tidy 执行失败: %v", err)
		}
	}

	if err := devercmd.GenerateRoutes(root); err != nil {
		log.Fatalf("路由生成失败: %v", err)
	}
}

func runRoutes(args []string) {
	fs := flag.NewFlagSet("routes", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("routes 参数解析失败: %v", err)
	}
	root := resolveProjectRoot(*projectRoot)
	if err := devercmd.GenerateRoutes(root); err != nil {
		log.Fatalf("路由生成失败: %v", err)
	}
}

func runMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("migrate 参数解析失败: %v", err)
	}
	if fs.NArg() < 1 {
		log.Fatal("migrate 需要指定目标数据库名称，例如：dever migrate default")
	}
	target := strings.TrimSpace(fs.Arg(0))
	if target == "" {
		log.Fatal("数据库名称不能为空")
	}

	root := resolveProjectRoot(*projectRoot)
	if err := os.Chdir(root); err != nil {
		log.Fatalf("切换到项目目录失败: %v", err)
	}
	if err := devercmd.RunMigrations(root, target); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
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
