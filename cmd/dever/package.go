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

const defaultPackageRef = "latest"

var packageNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type packageInstallOptions struct {
	projectRoot string
	name        string
	ref         string
}

func runPackage(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "remove":
			runPackageRemoveCommand(args[1:])
			return
		case "add", "update", "sync", "doctor", "list":
			log.Fatalf("dever package %s 已废弃，请使用：dever package <name>", args[0])
		}
	}
	runPackageInstallCommand(args)
}

func printPackageUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever package - package 组件命令

Usage:
    dever package [--project-root=.] [--ref=latest]          # 更新当前项目已启用的所有 package
    dever package [--project-root=.] [--ref=latest] <name>   # 安装或更新 github.com/dever-package/<name>
    dever package remove [--project-root=.] <name>
`)
}

func runPackageInstallCommand(args []string) {
	fs := flag.NewFlagSet("package", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	ref := fs.String("ref", defaultPackageRef, "Go module ref（默认 latest，可用 main、v0.1.1 或 commit）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package 参数解析失败: %v", err)
	}
	if fs.NArg() > 1 {
		log.Fatal("package 最多只能指定一个组件名称，例如：dever package bot")
	}

	packageRef, err := normalizePackageRef(*ref)
	if err != nil {
		log.Fatalf("package 参数解析失败: %v", err)
	}
	root := resolvePackageProjectRoot(*projectRoot)
	if fs.NArg() == 0 {
		if err := runPackageInstallAll(root, packageRef); err != nil {
			log.Fatalf("package 执行失败: %v", err)
		}
		return
	}

	options := packageInstallOptions{
		projectRoot: root,
		name:        strings.TrimSpace(fs.Arg(0)),
		ref:         packageRef,
	}
	if err := runPackageInstall(options); err != nil {
		log.Fatalf("package 执行失败: %v", err)
	}
}

func runPackageRemoveCommand(args []string) {
	fs := flag.NewFlagSet("package remove", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("package remove 参数解析失败: %v", err)
	}
	if fs.NArg() != 1 {
		log.Fatal("package remove 需要一个组件名称，例如：dever package remove bot")
	}

	root := resolvePackageProjectRoot(*projectRoot)
	if err := runPackageRemove(root, strings.TrimSpace(fs.Arg(0))); err != nil {
		log.Fatalf("package remove 执行失败: %v", err)
	}
}

func runPackageInstall(options packageInstallOptions) error {
	if err := validatePackageName(options.name); err != nil {
		return err
	}
	ref, err := normalizePackageRef(options.ref)
	if err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(options.projectRoot); err != nil {
		return err
	}

	synced := map[string]struct{}{}
	if err := syncPackageWithDependencies(options.projectRoot, options.name, ref, synced, map[string]struct{}{}); err != nil {
		return err
	}

	return refreshPackageRegistrations(options.projectRoot, "dever package")
}

func runPackageInstallAll(projectRoot string, ref string) error {
	packageRef, err := normalizePackageRef(ref)
	if err != nil {
		return err
	}
	if err := ensurePackageProjectRoot(projectRoot); err != nil {
		return err
	}

	names, err := activePackageInstallNames(projectRoot)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("当前项目没有已启用的 package 组件，请先使用：dever package <name>")
	}

	synced := map[string]struct{}{}
	for _, name := range names {
		if err := syncPackageWithDependencies(projectRoot, name, packageRef, synced, map[string]struct{}{}); err != nil {
			return fmt.Errorf("同步 %s 失败: %w", name, err)
		}
	}
	fmt.Printf("dever package: 已更新 %d 个 package: %s\n", len(names), strings.Join(names, ", "))
	return refreshPackageRegistrations(projectRoot, "dever package")
}

func refreshPackageRegistrations(projectRoot, label string) error {
	if err := runProjectInit(projectRoot, true); err != nil {
		return fmt.Errorf("刷新生成文件失败: %w", err)
	}
	fmt.Printf("%s: 已刷新 routes/model/service/component 注册\n", label)
	return nil
}

func activePackageInstallNames(projectRoot string) ([]string, error) {
	moduleDir := filepath.Join(projectRoot, "module")
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 module 目录失败: %w", err)
	}

	seen := map[string]struct{}{}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}

		moduleName := strings.TrimSpace(entry.Name())
		if moduleName == "" {
			continue
		}
		importPath, ok, err := util.ReadModuleImportDirective(filepath.Join(moduleDir, moduleName, "main.go"))
		if err != nil {
			return nil, fmt.Errorf("解析 module/%s/main.go 失败: %w", moduleName, err)
		}
		if !ok || !util.IsCanonicalPackageImport(importPath) {
			continue
		}

		packageName, err := packageNameFromCanonicalImport(importPath)
		if err != nil {
			return nil, fmt.Errorf("解析 module/%s/main.go 失败: %w", moduleName, err)
		}
		if moduleName != packageName {
			return nil, fmt.Errorf("module/%s/main.go 引用 %s，但 package shim 目录应为 module/%s", moduleName, importPath, packageName)
		}
		if _, exists := seen[packageName]; exists {
			continue
		}
		seen[packageName] = struct{}{}
		names = append(names, packageName)
	}

	sort.Strings(names)
	return names, nil
}

func packageNameFromCanonicalImport(importPath string) (string, error) {
	if !util.IsCanonicalPackageImport(importPath) {
		return "", fmt.Errorf("不是标准 package 导入路径: %s", importPath)
	}
	name := strings.TrimPrefix(strings.TrimSpace(importPath), util.CanonicalPackagePrefix)
	name = strings.Trim(name, "/")
	if strings.Contains(name, "/") {
		return "", fmt.Errorf("package 导入路径不能包含多级名称: %s", importPath)
	}
	if err := validatePackageName(name); err != nil {
		return "", err
	}
	return name, nil
}

func syncPackageWithDependencies(projectRoot, name string, ref string, synced map[string]struct{}, visiting map[string]struct{}) error {
	if err := validatePackageName(name); err != nil {
		return err
	}
	if _, ok := synced[name]; ok {
		return nil
	}
	if _, ok := visiting[name]; ok {
		return fmt.Errorf("检测到 package 循环依赖: %s", name)
	}
	visiting[name] = struct{}{}
	defer delete(visiting, name)

	importPath := util.CanonicalPackageImport(name)
	root, manifest, err := ensurePackageModule(projectRoot, name, importPath, ref)
	if err != nil {
		return err
	}

	for _, dep := range sortedDependencyNames(manifest.Depends) {
		if err := syncPackageWithDependencies(projectRoot, dep, ref, synced, visiting); err != nil {
			return fmt.Errorf("同步依赖 %s 失败: %w", dep, err)
		}
	}

	changed, err := ensurePackageShim(projectRoot, name, importPath)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("dever package: 已写入 module/%s/main.go\n", name)
	}
	fmt.Printf("dever package: %s -> %s\n", importPath, root)
	synced[name] = struct{}{}
	return nil
}

func ensurePackageModule(projectRoot, name, importPath string, ref string) (string, component.Manifest, error) {
	if err := ensurePackageRequirement(projectRoot, importPath, ref); err != nil {
		return "", component.Manifest{}, err
	}

	root, err := resolvePackageSourceDir(projectRoot, importPath)
	if err != nil {
		return "", component.Manifest{}, fmt.Errorf("解析 %s 源码目录失败: %w", importPath, err)
	}

	manifest, err := readPackageManifest(root)
	if err != nil {
		return "", component.Manifest{}, err
	}
	if manifest.Name != "" && manifest.Name != name {
		return "", component.Manifest{}, fmt.Errorf("%s dever.json name 不一致: %s", importPath, manifest.Name)
	}
	return root, manifest, nil
}

func ensurePackageRequirement(projectRoot, importPath string, ref string) error {
	if target, ok, err := localReplaceTarget(projectRoot, importPath); err != nil {
		return err
	} else if ok {
		if _, exists, err := goModRequireVersion(projectRoot, importPath); err != nil {
			return err
		} else if !exists {
			if err := runGoModEditRequire(projectRoot, importPath, "v0.0.0"); err != nil {
				return err
			}
		}
		fmt.Printf("dever package: 使用本地 replace %s => %s\n", importPath, target)
		return nil
	}

	cmd := goPackageCommand("get", packageModuleQuery(importPath, ref))
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装或更新 %s 失败: %w", packageModuleQuery(importPath, ref), err)
	}
	return nil
}

func runGoModEditRequire(projectRoot, importPath, version string) error {
	cmd := exec.Command("go", "mod", "edit", "-require="+importPath+"@"+version)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("写入 go.mod require %s@%s 失败: %w", importPath, version, err)
	}
	return nil
}

func runGoModEditDropRequire(projectRoot, importPath string) error {
	cmd := exec.Command("go", "mod", "edit", "-droprequire="+importPath)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("移除 go.mod require %s 失败: %w", importPath, err)
	}
	return nil
}

func resolvePackageSourceDir(projectRoot, importPath string) (string, error) {
	cmd := goPackageCommand("list", "-f", "{{.Dir}}", importPath)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", fmt.Errorf("go list 未返回源码目录")
	}
	return filepath.Abs(root)
}

func goPackageCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("go", args...)
	cmd.Env = util.WithCanonicalPackageGoEnv(os.Environ())
	return cmd
}

func ensurePackageShim(projectRoot, name, importPath string) (bool, error) {
	moduleDir := filepath.Join(projectRoot, "module", name)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return false, err
	}

	mainPath := filepath.Join(moduleDir, "main.go")
	content := fmt.Sprintf("package %s\n\n// dever:import %s\n", name, importPath)
	current, err := os.ReadFile(mainPath)
	if err == nil {
		if normalizePackageShimContent(string(current)) == normalizePackageShimContent(content) {
			return false, nil
		}
		if !isPackageShimContent(string(current), name) {
			return false, fmt.Errorf("module/%s/main.go 已存在且不是 package shim，请手动处理", name)
		}
		if err := os.WriteFile(mainPath, []byte(content), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.WriteFile(mainPath, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func isPackageShimContent(content, packageName string) bool {
	normalized := normalizePackageShimContent(content)
	if normalized == "package "+packageName {
		return true
	}
	return strings.Contains(normalized, "dever:import")
}

func runPackageRemove(projectRoot string, name string) error {
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
	if err := runGoModEditDropRequire(projectRoot, util.CanonicalPackageImport(name)); err != nil {
		return err
	}
	fmt.Printf("dever package remove: 已移除 module/%s\n", name)

	return refreshPackageRegistrations(projectRoot, "dever package remove")
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

func normalizePackageRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return defaultPackageRef, nil
	}
	if strings.ContainsAny(ref, " \t\r\n") {
		return "", fmt.Errorf("package ref 不能包含空白字符: %s", ref)
	}
	if strings.Contains(ref, "@") {
		return "", fmt.Errorf("package ref 不要包含 @，例如使用 --ref=main")
	}
	return ref, nil
}

func packageModuleQuery(importPath, ref string) string {
	return importPath + "@" + ref
}

func ensurePackageProjectRoot(projectRoot string) error {
	if !isGoModuleRoot(projectRoot) {
		return fmt.Errorf("未找到 go.mod，请在 Dever 后端项目根目录执行或传 --project-root")
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "module"), 0o755); err != nil {
		return fmt.Errorf("创建 module 目录失败: %w", err)
	}
	return nil
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

func localReplaceTarget(projectRoot, importPath string) (string, bool, error) {
	content, err := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return "", false, err
	}

	inReplaceBlock := false
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := stripGoModLineComment(rawLine)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if inReplaceBlock {
			if line == ")" {
				inReplaceBlock = false
				continue
			}
			if target, ok := parseLocalReplaceLine(projectRoot, importPath, line); ok {
				return target, true, nil
			}
			continue
		}

		if line == "replace (" {
			inReplaceBlock = true
			continue
		}
		if strings.HasPrefix(line, "replace ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "replace "))
			if target, ok := parseLocalReplaceLine(projectRoot, importPath, line); ok {
				return target, true, nil
			}
		}
	}
	return "", false, nil
}

func stripGoModLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func parseLocalReplaceLine(projectRoot, importPath, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != importPath {
		return "", false
	}

	arrow := -1
	for index, field := range fields {
		if field == "=>" {
			arrow = index
			break
		}
	}
	if arrow <= 0 || arrow+1 >= len(fields) {
		return "", false
	}

	target := strings.Trim(fields[arrow+1], `"`)
	if !isLocalReplaceValue(target) {
		return "", false
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target), true
	}
	return filepath.Clean(filepath.Join(projectRoot, target)), true
}

func goModRequireVersion(projectRoot, importPath string) (string, bool, error) {
	content, err := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return "", false, err
	}

	inRequireBlock := false
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := stripGoModLineComment(rawLine)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if inRequireBlock {
			if line == ")" {
				inRequireBlock = false
				continue
			}
			if version, ok := parseRequireLine(importPath, line); ok {
				return version, true, nil
			}
			continue
		}

		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
			if version, ok := parseRequireLine(importPath, line); ok {
				return version, true, nil
			}
		}
	}
	return "", false, nil
}

func parseRequireLine(importPath, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != importPath {
		return "", false
	}
	return strings.Trim(fields[1], `"`), true
}

func isLocalReplaceValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "." ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "."+string(os.PathSeparator)) ||
		strings.HasPrefix(value, ".."+string(os.PathSeparator)) ||
		filepath.IsAbs(value)
}

func normalizePackageShimContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.TrimSpace(content)
}
