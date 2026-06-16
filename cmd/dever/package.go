package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shemic/dever/component"
	"github.com/shemic/dever/util"
)

const defaultPackageRepoBase = "https://github.com/dever-package"

var packageNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type packageAddOptions struct {
	projectRoot string
	name        string
	repoBase    string
	skipInit    bool
	noDeps      bool
}

type packageUpdateOptions struct {
	projectRoot string
	name        string
	repoBase    string
	skipInit    bool
	force       bool
}

func runPackage(args []string) {
	if len(args) == 0 {
		printPackageUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runPackageAddCommand(args[1:])
	case "update":
		runPackageUpdateCommand(args[1:])
	case "remove":
		runPackageRemoveCommand(args[1:])
	case "list":
		runPackageListCommand(args[1:])
	case "sync":
		runPackageSyncCommand(args[1:])
	case "doctor":
		runPackageDoctorCommand(args[1:])
	default:
		printPackageUsage()
		os.Exit(1)
	}
}

func printPackageUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever package - package 组件命令

Usage:
    dever package add [--project-root=.] [--repo-base=https://github.com/dever-package] [--skip-init] [--no-deps] <name>
    dever package update [--project-root=.] [--repo-base=https://github.com/dever-package] [--skip-init] [--force] <name>
    dever package remove [--project-root=.] [--skip-init] <name>
    dever package list [--project-root=.]
    dever package sync [--project-root=.] [--repo-base=https://github.com/dever-package] [--skip-init]
    dever package doctor [--project-root=.]
`)
}

func runPackageAddCommand(args []string) {
	fs := flag.NewFlagSet("package add", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	repoBase := fs.String("repo-base", defaultPackageRepoBase, "package GitHub 组织或仓库基础地址")
	skipInit := fs.Bool("skip-init", false, "跳过刷新 routes/model/service 注册")
	noDeps := fs.Bool("no-deps", false, "不自动安装依赖")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package add 参数解析失败: %v", err)
	}
	if fs.NArg() != 1 {
		log.Fatal("package add 需要一个组件名称，例如：dever package add bot")
	}

	root := resolvePackageProjectRoot(*projectRoot)
	options := packageAddOptions{
		projectRoot: root,
		name:        strings.TrimSpace(fs.Arg(0)),
		repoBase:    strings.TrimSpace(*repoBase),
		skipInit:    *skipInit,
		noDeps:      *noDeps,
	}
	if err := runPackageAdd(options); err != nil {
		log.Fatalf("package add 执行失败: %v", err)
	}
}

func runPackageUpdateCommand(args []string) {
	fs := flag.NewFlagSet("package update", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	repoBase := fs.String("repo-base", defaultPackageRepoBase, "package GitHub 组织或仓库基础地址")
	skipInit := fs.Bool("skip-init", false, "跳过刷新 routes/model/service 注册")
	force := fs.Bool("force", false, "删除本地 package 后重新拉取")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package update 参数解析失败: %v", err)
	}
	if fs.NArg() != 1 {
		log.Fatal("package update 需要一个组件名称，例如：dever package update bot")
	}

	root := resolvePackageProjectRoot(*projectRoot)
	options := packageUpdateOptions{
		projectRoot: root,
		name:        strings.TrimSpace(fs.Arg(0)),
		repoBase:    strings.TrimSpace(*repoBase),
		skipInit:    *skipInit,
		force:       *force,
	}
	if err := runPackageUpdate(options); err != nil {
		log.Fatalf("package update 执行失败: %v", err)
	}
}

func runPackageRemoveCommand(args []string) {
	fs := flag.NewFlagSet("package remove", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	skipInit := fs.Bool("skip-init", false, "跳过刷新 routes/model/service 注册")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package remove 参数解析失败: %v", err)
	}
	if fs.NArg() != 1 {
		log.Fatal("package remove 需要一个组件名称，例如：dever package remove bot")
	}
	root := resolvePackageProjectRoot(*projectRoot)
	if err := runPackageRemove(root, strings.TrimSpace(fs.Arg(0)), *skipInit); err != nil {
		log.Fatalf("package remove 执行失败: %v", err)
	}
}

func runPackageListCommand(args []string) {
	fs := flag.NewFlagSet("package list", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package list 参数解析失败: %v", err)
	}
	root := resolvePackageProjectRoot(*projectRoot)
	if err := printPackageList(root); err != nil {
		log.Fatalf("package list 执行失败: %v", err)
	}
}

func runPackageSyncCommand(args []string) {
	fs := flag.NewFlagSet("package sync", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	repoBase := fs.String("repo-base", defaultPackageRepoBase, "package GitHub 组织或仓库基础地址")
	skipInit := fs.Bool("skip-init", false, "跳过刷新 routes/model/service 注册")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package sync 参数解析失败: %v", err)
	}
	root := resolvePackageProjectRoot(*projectRoot)
	if err := runPackageSync(root, strings.TrimSpace(*repoBase), *skipInit); err != nil {
		log.Fatalf("package sync 执行失败: %v", err)
	}
}

func runPackageDoctorCommand(args []string) {
	fs := flag.NewFlagSet("package doctor", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package doctor 参数解析失败: %v", err)
	}
	root := resolvePackageProjectRoot(*projectRoot)
	if err := runPackageDoctor(root); err != nil {
		log.Fatalf("package doctor 执行失败: %v", err)
	}
}

func runPackageAdd(options packageAddOptions) error {
	if err := validatePackageName(options.name); err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(options.projectRoot); err != nil {
		return err
	}

	installed := map[string]struct{}{}
	if err := installPackageWithDependencies(options, options.name, installed, map[string]struct{}{}); err != nil {
		return err
	}

	if !options.skipInit {
		if err := runProjectInit(options.projectRoot, true); err != nil {
			return fmt.Errorf("刷新生成文件失败: %w", err)
		}
		fmt.Println("dever package add: 已刷新 routes/model/service/component 注册")
	}
	return nil
}

func installPackageWithDependencies(options packageAddOptions, name string, installed map[string]struct{}, visiting map[string]struct{}) error {
	if err := validatePackageName(name); err != nil {
		return err
	}
	if _, ok := installed[name]; ok {
		return nil
	}
	if _, ok := visiting[name]; ok {
		return fmt.Errorf("检测到 package 循环依赖: %s", name)
	}
	visiting[name] = struct{}{}
	defer delete(visiting, name)

	manifest, err := ensurePackageInstalled(options.projectRoot, options.repoBase, name)
	if err != nil {
		return err
	}
	if !options.noDeps {
		deps := sortedDependencyNames(manifest.Depends)
		for _, dep := range deps {
			if err := installPackageWithDependencies(options, dep, installed, visiting); err != nil {
				return fmt.Errorf("安装依赖 %s 失败: %w", dep, err)
			}
		}
	}

	if err := ensurePackageShim(options.projectRoot, name); err != nil {
		return err
	}
	installed[name] = struct{}{}
	return nil
}

func ensurePackageInstalled(projectRoot, repoBase, name string) (component.Manifest, error) {
	repoURL := buildPackageRepoURL(repoBase, name)
	packageDir := filepath.Join(projectRoot, "package", name)
	cloned, err := ensurePackageSource(packageDir, repoURL)
	if err != nil {
		return component.Manifest{}, err
	}
	if cloned {
		fmt.Printf("dever package add: 已拉取 %s 到 %s\n", repoURL, packageDir)
	} else {
		fmt.Printf("dever package add: package/%s 已存在，跳过拉取\n", name)
	}
	manifest, err := readPackageManifest(packageDir)
	if err != nil {
		return component.Manifest{}, err
	}
	if manifest.Name != "" && manifest.Name != name {
		return component.Manifest{}, fmt.Errorf("package/%s dever.json name 不一致: %s", name, manifest.Name)
	}
	return manifest, nil
}

func ensurePackageShim(projectRoot, name string) error {
	projectModule, err := readPackageProjectModuleName(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return err
	}
	importPath := projectModule + "/package/" + name
	moduleDir := filepath.Join(projectRoot, "module", name)
	createdShim, err := ensurePackageModuleShim(moduleDir, name, importPath)
	if err != nil {
		return err
	}
	if createdShim {
		fmt.Printf("dever package add: 已创建 module/%s/main.go\n", name)
	} else {
		fmt.Printf("dever package add: module/%s/main.go 已存在，跳过创建\n", name)
	}
	return nil
}

func runPackageUpdate(options packageUpdateOptions) error {
	if err := validatePackageName(options.name); err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(options.projectRoot); err != nil {
		return err
	}

	projectModule, err := readPackageProjectModuleName(filepath.Join(options.projectRoot, "go.mod"))
	if err != nil {
		return err
	}

	importPath := projectModule + "/package/" + options.name
	repoURL := buildPackageRepoURL(options.repoBase, options.name)
	packageDir := filepath.Join(options.projectRoot, "package", options.name)
	moduleDir := filepath.Join(options.projectRoot, "module", options.name)

	if options.force {
		if err := forceReplacePackageSource(packageDir, repoURL); err != nil {
			return err
		}
		fmt.Printf("dever package update: 已重新拉取 %s 到 %s\n", repoURL, packageDir)
	} else {
		if err := updatePackageSource(packageDir); err != nil {
			return err
		}
		fmt.Printf("dever package update: 已更新 package/%s\n", options.name)
	}

	createdShim, err := ensurePackageModuleShim(moduleDir, options.name, importPath)
	if err != nil {
		return err
	}
	if createdShim {
		fmt.Printf("dever package update: 已创建 module/%s/main.go\n", options.name)
	}

	if !options.skipInit {
		if err := runProjectInit(options.projectRoot, true); err != nil {
			return fmt.Errorf("刷新生成文件失败: %w", err)
		}
		fmt.Println("dever package update: 已刷新 routes/model/service/component 注册")
	}
	return nil
}

func runPackageRemove(projectRoot string, name string, skipInit bool) error {
	if err := validatePackageName(name); err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(projectRoot); err != nil {
		return err
	}
	dependents, err := packageDependents(projectRoot, name)
	if err != nil {
		return err
	}
	if len(dependents) > 0 {
		return fmt.Errorf("package/%s 正被这些组件依赖: %s", name, strings.Join(dependents, ", "))
	}

	moduleDir := filepath.Join(projectRoot, "module", name)
	if err := os.RemoveAll(moduleDir); err != nil {
		return fmt.Errorf("删除 module/%s 失败: %w", name, err)
	}
	fmt.Printf("dever package remove: 已移除 module/%s\n", name)

	if !skipInit {
		if err := runProjectInit(projectRoot, true); err != nil {
			return fmt.Errorf("刷新生成文件失败: %w", err)
		}
		fmt.Println("dever package remove: 已刷新 routes/model/service/component 注册")
	}
	return nil
}

func printPackageList(projectRoot string) error {
	components, err := listActiveComponentSources(projectRoot)
	if err != nil {
		return err
	}
	if len(components) == 0 {
		fmt.Println("未发现 active component")
		return nil
	}
	for _, current := range components {
		manifest, err := readPackageManifest(current.root)
		version := ""
		if err == nil {
			version = manifest.Version
		}
		if version == "" {
			version = "-"
		}
		fmt.Printf("%s/%s %s\n", current.source, current.name, version)
	}
	return nil
}

func runPackageSync(projectRoot, repoBase string, skipInit bool) error {
	if err := ensurePackageProjectRoot(projectRoot); err != nil {
		return err
	}
	missing, err := missingDependencies(projectRoot)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		fmt.Println("dever package sync: 依赖完整")
		return nil
	}
	options := packageAddOptions{
		projectRoot: projectRoot,
		repoBase:    repoBase,
		skipInit:    true,
	}
	installed := map[string]struct{}{}
	for _, name := range missing {
		if err := installPackageWithDependencies(options, name, installed, map[string]struct{}{}); err != nil {
			return err
		}
	}
	if !skipInit {
		if err := runProjectInit(projectRoot, true); err != nil {
			return fmt.Errorf("刷新生成文件失败: %w", err)
		}
	}
	return nil
}

func runPackageDoctor(projectRoot string) error {
	components, err := listActiveComponentSources(projectRoot)
	if err != nil {
		return err
	}
	if len(components) == 0 {
		return fmt.Errorf("未发现 active component")
	}
	missing, err := missingDependencies(projectRoot)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少依赖 package: %s", strings.Join(missing, ", "))
	}
	fmt.Printf("dever package doctor: %d 个 active component 正常\n", len(components))
	return nil
}

func resolvePackageProjectRoot(rawRoot string) string {
	root := resolveProjectRoot(rawRoot)
	if isGoModuleRoot(root) {
		return root
	}
	backendRoot := filepath.Join(root, "backend")
	if isGoModuleRoot(backendRoot) {
		return backendRoot
	}
	return root
}

func isGoModuleRoot(root string) bool {
	info, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil && !info.IsDir()
}

func validatePackageName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("组件名称不能为空")
	}
	if !packageNamePattern.MatchString(name) {
		return fmt.Errorf("组件名称必须是合法 Go 包名: %s", name)
	}
	return nil
}

func ensurePackageProjectRoot(projectRoot string) error {
	if !isGoModuleRoot(projectRoot) {
		return fmt.Errorf("未找到 go.mod，请在 Dever 后端项目根目录执行或传 --project-root")
	}
	for _, dir := range []string{"package", "module"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, dir), 0o755); err != nil {
			return fmt.Errorf("创建 %s 目录失败: %w", dir, err)
		}
	}
	return nil
}

func readPackageProjectModuleName(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if moduleName == "" {
				break
			}
			return moduleName, nil
		}
	}
	return "", fmt.Errorf("go.mod 缺少 module 声明")
}

func buildPackageRepoURL(repoBase, name string) string {
	repoBase = strings.TrimRight(strings.TrimSpace(repoBase), "/")
	if repoBase == "" {
		repoBase = defaultPackageRepoBase
	}
	return repoBase + "/" + name + ".git"
}

func readPackageManifest(root string) (component.Manifest, error) {
	content, _, err := util.ReadJSONCFile(filepath.Join(root, "dever.json"))
	if err != nil {
		return component.Manifest{}, fmt.Errorf("读取 %s 失败: %w", filepath.Join(root, "dever.json"), err)
	}
	manifest, err := component.DecodeManifest(content)
	if err != nil {
		return component.Manifest{}, err
	}
	if manifest.Name == "" {
		manifest.Name = filepath.Base(root)
	}
	return manifest, nil
}

func sortedDependencyNames(deps map[string]string) []string {
	names := make([]string, 0, len(deps))
	for name := range deps {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func packageDependents(projectRoot string, target string) ([]string, error) {
	components, err := listActiveComponentSources(projectRoot)
	if err != nil {
		return nil, err
	}
	dependents := make([]string, 0)
	for _, current := range components {
		if current.name == target {
			continue
		}
		manifest, err := readPackageManifest(current.root)
		if err != nil {
			return nil, err
		}
		if _, ok := manifest.Depends[target]; ok {
			dependents = append(dependents, current.name)
		}
	}
	sort.Strings(dependents)
	return dependents, nil
}

func missingDependencies(projectRoot string) ([]string, error) {
	components, err := listActiveComponentSources(projectRoot)
	if err != nil {
		return nil, err
	}
	active := make(map[string]struct{}, len(components))
	for _, current := range components {
		active[current.name] = struct{}{}
	}

	missing := map[string]struct{}{}
	for _, current := range components {
		manifest, err := readPackageManifest(current.root)
		if err != nil {
			return nil, err
		}
		for dep := range manifest.Depends {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if _, ok := active[dep]; ok {
				continue
			}
			missing[dep] = struct{}{}
		}
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func ensurePackageSource(packageDir, repoURL string) (bool, error) {
	info, err := os.Stat(packageDir)
	switch {
	case err == nil && !info.IsDir():
		return false, fmt.Errorf("package 目标已存在且不是目录: %s", packageDir)
	case err == nil:
		empty, err := isDirectoryEmpty(packageDir)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(packageDir), 0o755); err != nil {
			return false, err
		}
	default:
		return false, err
	}

	if err := runGitClone(repoURL, packageDir); err != nil {
		if info == nil {
			_ = os.RemoveAll(packageDir)
		}
		return false, err
	}
	return true, nil
}

func isDirectoryEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func runGitClone(repoURL, packageDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, packageDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("拉取 package 失败: %w", err)
	}
	return nil
}

func forceReplacePackageSource(packageDir, repoURL string) error {
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("删除本地 package 失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(packageDir), 0o755); err != nil {
		return err
	}
	return runGitClone(repoURL, packageDir)
}

func updatePackageSource(packageDir string) error {
	info, err := os.Stat(packageDir)
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("package 未安装: %s，请先执行 dever package add", packageDir)
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("package 目标不是目录: %s", packageDir)
	}

	if !isGitWorkTree(packageDir) {
		return fmt.Errorf("package 不是 git 仓库，无法自动更新: %s", packageDir)
	}
	clean, err := isGitWorkTreeClean(packageDir)
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("package/%s 有本地改动，请先提交/处理改动，或使用 --force 重新拉取", filepath.Base(packageDir))
	}
	return runGitPullFastForward(packageDir)
}

func isGitWorkTree(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func isGitWorkTreeClean(dir string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "", nil
}

func runGitPullFastForward(dir string) error {
	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("更新 package 失败: %w", err)
	}
	return nil
}

func ensurePackageModuleShim(moduleDir, packageName, importPath string) (bool, error) {
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return false, err
	}

	mainPath := filepath.Join(moduleDir, "main.go")
	content := fmt.Sprintf("package %s\n\n// dever:import %s\n", packageName, importPath)
	current, err := os.ReadFile(mainPath)
	if err == nil {
		if normalizePackageShimContent(string(current)) == normalizePackageShimContent(content) {
			return false, nil
		}
		return false, fmt.Errorf("module/%s/main.go 已存在且不是目标 package shim，请手动处理", packageName)
	}
	if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.WriteFile(mainPath, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func normalizePackageShimContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.TrimSpace(content)
}
