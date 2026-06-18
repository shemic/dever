package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	deverSkillName       = "shemic-dever"
	deverSkillSourcePath = "skills/skills-dever"
	deverSkillRepo       = "https://github.com/shemic/skills-dever.git"
	deverSkillRepoRef    = "main"
	deverSkillStart      = "<!-- dever-skill:start -->"
	deverSkillEnd        = "<!-- dever-skill:end -->"
)

type skillInstallOptions struct {
	projectRoot string
	global      bool
	project     bool
	agents      bool
	force       bool
	repo        string
	ref         string
}

func runSkill(args []string) {
	if len(args) == 0 {
		printSkillUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "install":
		runSkillInstallCommand(args[1:])
	case "doctor":
		runSkillDoctorCommand(args[1:])
	default:
		printSkillUsage()
		os.Exit(1)
	}
}

func printSkillUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever skill - AI skill 安装和检查命令

Usage:
    dever skill install [--project-root=.] [--global=true] [--project=false] [--agents=true] [--force] [--repo=https://github.com/shemic/skills-dever.git] [--ref=main]
    dever skill doctor [--project-root=.]
`)
}

func runSkillInstallCommand(args []string) {
	fs := flag.NewFlagSet("skill install", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	global := fs.Bool("global", true, "同步到常见全局 skill 目录")
	project := fs.Bool("project", false, "同步一份项目本地 skills/skills-dever 镜像")
	agents := fs.Bool("agents", true, "写入项目 AGENTS/CLAUDE/OpenCode/Codex managed block")
	force := fs.Bool("force", false, "忽略已安装全局 skill 作为来源，按项目/Git ref 重新同步")
	repo := fs.String("repo", deverSkillRepo, "skills-dever Git 仓库地址")
	ref := fs.String("ref", deverSkillRepoRef, "skills-dever Git ref/tag/branch")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("skill install 参数解析失败: %v", err)
	}

	root := resolveProjectRoot(*projectRoot)
	if err := runSkillInstall(skillInstallOptions{
		projectRoot: root,
		global:      *global,
		project:     *project,
		agents:      *agents,
		force:       *force,
		repo:        strings.TrimSpace(*repo),
		ref:         strings.TrimSpace(*ref),
	}); err != nil {
		log.Fatalf("skill install 执行失败: %v", err)
	}
}

func runSkillDoctorCommand(args []string) {
	fs := flag.NewFlagSet("skill doctor", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("skill doctor 参数解析失败: %v", err)
	}

	root := resolveProjectRoot(*projectRoot)
	if err := runSkillDoctor(root); err != nil {
		log.Fatalf("skill doctor 执行失败: %v", err)
	}
}

func runSkillInstall(options skillInstallOptions) error {
	source, err := resolveDeverSkillSource(options)
	if err != nil {
		return err
	}
	skillRoot := resolveSkillProjectRoot(options.projectRoot)

	if options.project {
		target := filepath.Join(skillRoot, deverSkillSourcePath)
		if err := copyDeverSkill(source, target, options.force); err != nil {
			return fmt.Errorf("同步项目 skill 失败: %w", err)
		}
		fmt.Printf("dever skill install: 已同步项目 skill: %s\n", target)
	}

	if options.global {
		targets, err := globalSkillTargets()
		if err != nil {
			return err
		}
		for _, target := range targets {
			if err := copyDeverSkill(source, target, true); err != nil {
				return fmt.Errorf("同步全局 skill %s 失败: %w", target, err)
			}
			fmt.Printf("dever skill install: 已同步全局 skill: %s\n", target)
		}
	}

	if options.agents {
		block, err := readDeverAgentsBlock(source)
		if err != nil {
			return err
		}
		updated, err := installDeverAgentBlocks(skillRoot, block)
		if err != nil {
			return err
		}
		for _, file := range updated {
			fmt.Printf("dever skill install: 已更新 agent 提示: %s\n", file)
		}
	}

	return nil
}

func runSkillDoctor(projectRoot string) error {
	skillRoot := resolveSkillProjectRoot(projectRoot)
	projectSkill := filepath.Join(skillRoot, deverSkillSourcePath, "SKILL.md")
	hasProjectSkill := fileExists(projectSkill)
	if hasProjectSkill {
		fmt.Printf("dever skill doctor: 项目 skill 镜像正常: %s\n", projectSkill)
	}

	block := false
	for _, target := range agentInstructionTargets(skillRoot) {
		if hasDeverSkillBlock(target) {
			fmt.Printf("dever skill doctor: agent 提示正常: %s\n", target)
			block = true
		}
	}
	if !block {
		return fmt.Errorf("项目缺少 Dever agent managed block，请执行 dever skill install")
	}

	if err := doctorComponentSkills(projectRoot); err != nil {
		return err
	}

	global := false
	for _, target := range mustGlobalSkillTargets() {
		if fileExists(filepath.Join(target, "SKILL.md")) {
			fmt.Printf("dever skill doctor: 全局 skill 正常: %s\n", target)
			global = true
		}
	}
	if !global && !hasProjectSkill {
		return fmt.Errorf("未发现全局 skill 或项目 skill 镜像，请先安装 github.com/shemic/skills-dever")
	}
	if !global && hasProjectSkill {
		fmt.Println("dever skill doctor: 未发现全局 skill；项目本地 skill 镜像可用")
	}
	return nil
}

func doctorComponentSkills(projectRoot string) error {
	componentRoot := resolvePackageProjectRoot(projectRoot)
	if !isGoModuleRoot(componentRoot) {
		return nil
	}

	components, err := listActiveComponentSources(componentRoot)
	if err != nil {
		return fmt.Errorf("检查组件 skill 失败: %w", err)
	}

	checked := 0
	missing := make([]string, 0)
	for _, current := range components {
		manifest, err := readPackageManifest(current.root)
		if err != nil {
			return fmt.Errorf("读取 %s/%s dever.json 失败: %w", current.source, current.name, err)
		}
		for _, skill := range manifest.Skills {
			skillPath, err := componentSkillPath(current.root, skill)
			if err != nil {
				missing = append(missing, fmt.Sprintf("%s/%s: %s", current.source, current.name, err))
				continue
			}
			if !fileExists(skillPath) {
				missing = append(missing, fmt.Sprintf("%s/%s: 缺少 %s", current.source, current.name, skill))
				continue
			}
			checked++
			relative, err := filepath.Rel(componentRoot, skillPath)
			if err != nil {
				relative = skillPath
			}
			fmt.Printf("dever skill doctor: 组件 skill 正常: %s\n", filepath.ToSlash(relative))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("组件 skill 声明无效:\n- %s", strings.Join(missing, "\n- "))
	}
	if checked == 0 {
		fmt.Println("dever skill doctor: 未发现组件 skill 声明")
	}
	return nil
}

func componentSkillPath(componentRoot string, skill string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(skill)))
	if clean == "" || clean == "." {
		return "", fmt.Errorf("skill 路径不能为空")
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("skill 路径必须在组件目录内: %s", skill)
	}
	return filepath.Join(componentRoot, clean), nil
}

type deverSkillSource struct {
	root string
}

func resolveDeverSkillSource(options skillInstallOptions) (deverSkillSource, error) {
	if local, ok := findProjectDeverSkillSource(options.projectRoot); ok {
		return deverSkillSource{root: local}, nil
	}

	if !options.force {
		if global, ok := firstInstalledGlobalSkill(); ok {
			return deverSkillSource{root: global}, nil
		}
	}

	fetched, err := fetchDeverSkill(options)
	if err == nil {
		return deverSkillSource{root: fetched}, nil
	}

	if !options.force {
		if global, ok := firstInstalledGlobalSkill(); ok {
			fmt.Fprintf(os.Stderr, "dever skill install: 拉取 skills-dever 失败，使用已安装全局 skill 兜底: %s\n", global)
			return deverSkillSource{root: global}, nil
		}
	}
	return deverSkillSource{}, err
}

func findProjectDeverSkillSource(projectRoot string) (string, bool) {
	candidates := []string{
		filepath.Join(projectRoot, deverSkillSourcePath),
		filepath.Join(projectRoot, "..", deverSkillSourcePath),
		filepath.Join(projectRoot, "..", "..", deverSkillSourcePath),
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if isCompleteDeverSkillRoot(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func firstInstalledGlobalSkill() (string, bool) {
	for _, target := range mustGlobalSkillTargets() {
		if isCompleteDeverSkillRoot(target) {
			return target, true
		}
	}
	return "", false
}

func fetchDeverSkill(options skillInstallOptions) (string, error) {
	repo := strings.TrimSpace(options.repo)
	if repo == "" {
		repo = deverSkillRepo
	}
	ref := strings.TrimSpace(options.ref)
	if ref == "" {
		ref = deverSkillRepoRef
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheRoot) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("无法解析 skill 缓存目录: %w", homeErr)
		}
		cacheRoot = filepath.Join(home, ".cache")
	}

	target := filepath.Join(cacheRoot, "dever", "skills", "skills-dever")
	if _, ok := skillRootInRepo(target); ok {
		if err := gitUpdateDeverSkills(target, ref); err == nil {
			if skillRoot, ok := skillRootInRepo(target); ok {
				return skillRoot, nil
			}
		}
	}

	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := gitCloneDeverSkills(repo, ref, target); err != nil {
		return "", fmt.Errorf("拉取 skills-dever 失败，请先手动安装 %s: %w", repo, err)
	}
	skillRoot, ok := skillRootInRepo(target)
	if !ok {
		return "", fmt.Errorf("skills-dever 仓库缺少 SKILL.md 或 %s/SKILL.md", deverSkillSourcePath)
	}
	return skillRoot, nil
}

func skillRootInRepo(root string) (string, bool) {
	if isCompleteDeverSkillRoot(filepath.Join(root, deverSkillSourcePath)) {
		return filepath.Join(root, deverSkillSourcePath), true
	}
	if isCompleteDeverSkillRoot(root) {
		return root, true
	}
	return "", false
}

func isCompleteDeverSkillRoot(root string) bool {
	return fileExists(filepath.Join(root, "SKILL.md")) &&
		fileExists(filepath.Join(root, "files", "AGENTS.dever.md"))
}

func gitCloneDeverSkills(repo, ref, target string) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repo, target)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitUpdateDeverSkills(root, ref string) error {
	cmd := exec.Command("git", "fetch", "--depth", "1", "origin", ref)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "checkout", "FETCH_HEAD")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = root
	return cmd.Run()
}

func resolveSkillProjectRoot(projectRoot string) string {
	for _, candidate := range []string{
		projectRoot,
		filepath.Join(projectRoot, ".."),
		filepath.Join(projectRoot, "..", ".."),
	} {
		candidate = filepath.Clean(candidate)
		if isCompleteDeverSkillRoot(filepath.Join(candidate, deverSkillSourcePath)) {
			return candidate
		}
	}
	return projectRoot
}

func copyDeverSkill(source deverSkillSource, target string, overwrite bool) error {
	if samePath(source.root, target) {
		return nil
	}
	if overwrite {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return copyDirectoryFromDisk(source.root, target, overwrite)
}

func copyDirectoryFromDisk(sourceRoot, targetRoot string, overwrite bool) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() && (name == ".git" || name == "node_modules") {
			return filepath.SkipDir
		}
		if name == ".DS_Store" {
			return nil
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(targetRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !overwrite && fileExists(target) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	return writeReaderToFile(input, target, mode)
}

func writeReaderToFile(input io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	perm := copiedFileMode(target, mode)
	if info, err := os.Stat(target); err == nil && !info.IsDir() && info.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(target, 0o644)
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(target, perm); err != nil {
		return err
	}
	return nil
}

func copiedFileMode(target string, mode os.FileMode) os.FileMode {
	cleanTarget := filepath.ToSlash(target)
	if strings.HasSuffix(cleanTarget, ".sh") && (strings.HasPrefix(cleanTarget, "scripts/") || strings.Contains(cleanTarget, "/scripts/")) {
		return 0o755
	}
	perm := mode.Perm()
	if perm == 0 || perm&0o200 == 0 {
		return 0o644
	}
	return perm
}

func globalSkillTargets() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	targets := []string{
		filepath.Join(home, ".agents", "skills", deverSkillName),
		filepath.Join(home, ".codex", "skills", deverSkillName),
		filepath.Join(home, ".claude", "skills", deverSkillName),
	}
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		targets = append(targets, filepath.Join(codexHome, "skills", deverSkillName))
	}
	return uniquePaths(targets), nil
}

func mustGlobalSkillTargets() []string {
	targets, err := globalSkillTargets()
	if err != nil {
		return nil
	}
	return targets
}

func uniquePaths(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = filepath.Clean(strings.TrimSpace(item))
		if item == "." || item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func readDeverAgentsBlock(source deverSkillSource) (string, error) {
	path := filepath.Join(source.root, "files", "AGENTS.dever.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 Dever agent 提示模板失败: %s: %w", path, err)
	}
	return strings.TrimSpace(string(content)) + "\n", nil
}

func installDeverAgentBlocks(projectRoot, block string) ([]string, error) {
	updated := make([]string, 0)
	for _, target := range agentInstructionTargets(projectRoot) {
		changed, err := upsertManagedBlock(target, block)
		if err != nil {
			return nil, err
		}
		if changed {
			updated = append(updated, target)
		}
	}
	return updated, nil
}

func agentInstructionTargets(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(projectRoot, "CLAUDE.md"),
		filepath.Join(projectRoot, ".codex", "AGENTS.md"),
		filepath.Join(projectRoot, ".opencode", "AGENTS.md"),
	}
}

func upsertManagedBlock(path string, block string) (bool, error) {
	currentBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	current := string(currentBytes)
	next := replaceManagedBlock(current, block)
	if next == current {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}

func replaceManagedBlock(content string, block string) string {
	block = strings.TrimSpace(block) + "\n"
	start := strings.Index(content, deverSkillStart)
	end := strings.Index(content, deverSkillEnd)
	if start >= 0 && end >= start {
		end += len(deverSkillEnd)
		next := strings.TrimRight(content[:start], "\n")
		if next != "" {
			next += "\n\n"
		}
		next += block
		tail := strings.TrimLeft(content[end:], "\n")
		if tail != "" {
			next += "\n" + tail
		}
		return next
	}
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return block
	}
	return content + "\n\n" + block
}

func hasDeverSkillBlock(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(content)
	return strings.Contains(text, deverSkillStart) && strings.Contains(text, deverSkillEnd)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func samePath(left, right string) bool {
	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
