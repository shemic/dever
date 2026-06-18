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
    dever skill install [--project-root=.] [--global=true] [--project=false] [--agents=true] [--repo=https://github.com/shemic/skills-dever.git] [--ref=main]
    dever skill doctor [--project-root=.]
`)
}

func runSkillInstallCommand(args []string) {
	fs := flag.NewFlagSet("skill install", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	global := fs.Bool("global", true, "同步到常见全局 skill 目录")
	project := fs.Bool("project", false, "同步一份项目本地 skills/skills-dever 镜像")
	agents := fs.Bool("agents", true, "写入项目 AGENTS.md/CLAUDE.md managed block")
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
	defer source.cleanup()
	projectRoot := options.projectRoot

	if options.project {
		target := filepath.Join(projectRoot, deverSkillSourcePath)
		if err := installDeverSkillCopy(source, target); err != nil {
			return fmt.Errorf("同步项目 skill 失败: %w", err)
		}
		fmt.Printf("dever skill install: 已同步项目 skill: %s\n", target)
	}

	if options.global {
		primary, err := primaryGlobalSkillTarget()
		if err != nil {
			return err
		}
		if err := installDeverSkillCopy(source, primary); err != nil {
			return fmt.Errorf("同步主全局 skill %s 失败: %w", primary, err)
		}
		fmt.Printf("dever skill install: 已同步主全局 skill: %s\n", primary)

		references, err := globalSkillReferenceTargets(primary)
		if err != nil {
			return err
		}
		for _, target := range references {
			if err := installSkillReference(primary, target); err != nil {
				return fmt.Errorf("创建全局 skill 引用 %s 失败: %w", target, err)
			}
			fmt.Printf("dever skill install: 已创建全局 skill 引用: %s -> %s\n", target, primary)
		}
	}

	if options.agents {
		block, err := readDeverAgentsBlock(source)
		if err != nil {
			return err
		}
		updated, err := installDeverAgentBlocks(projectRoot, block)
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
	block := false
	for _, target := range agentInstructionTargets(projectRoot) {
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

	primary, err := primaryGlobalSkillTarget()
	if err != nil {
		return err
	}
	if !isCompleteDeverSkillRoot(primary) {
		return fmt.Errorf("主全局 skill 缺失或不完整，请执行 dever skill install: %s", primary)
	}
	fmt.Printf("dever skill doctor: 主全局 skill 正常: %s\n", primary)

	references, err := globalSkillReferenceTargets(primary)
	if err != nil {
		return err
	}
	for _, target := range references {
		if !isSymlinkTo(target, primary) {
			return fmt.Errorf("全局 skill 引用缺失或不是有效 symlink: %s -> %s", target, primary)
		}
		fmt.Printf("dever skill doctor: 全局 skill 引用正常: %s -> %s\n", target, primary)
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
	root     string
	tempRoot string
}

func resolveDeverSkillSource(options skillInstallOptions) (deverSkillSource, error) {
	return fetchDeverSkill(options)
}

func fetchDeverSkill(options skillInstallOptions) (deverSkillSource, error) {
	repo := strings.TrimSpace(options.repo)
	if repo == "" {
		repo = deverSkillRepo
	}
	ref := strings.TrimSpace(options.ref)
	if ref == "" {
		ref = deverSkillRepoRef
	}

	tempRoot, err := os.MkdirTemp("", "dever-skills-")
	if err != nil {
		return deverSkillSource{}, fmt.Errorf("创建临时 skill 目录失败: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tempRoot)
		}
	}()

	target := filepath.Join(tempRoot, "repo")
	if err := gitCloneDeverSkills(repo, ref, target); err != nil {
		return deverSkillSource{}, fmt.Errorf("拉取 skills-dever 失败: %s: %w", repo, err)
	}
	skillRoot, ok := skillRootInRepo(target)
	if !ok {
		return deverSkillSource{}, fmt.Errorf("skills-dever 仓库缺少 SKILL.md 或 %s/SKILL.md", deverSkillSourcePath)
	}
	cleanup = false
	return deverSkillSource{root: skillRoot, tempRoot: tempRoot}, nil
}

func (source deverSkillSource) cleanup() {
	if strings.TrimSpace(source.tempRoot) != "" {
		_ = os.RemoveAll(source.tempRoot)
	}
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

func installDeverSkillCopy(source deverSkillSource, target string) error {
	if samePath(source.root, target) {
		return nil
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(target)+".tmp-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temp)
		}
	}()

	if err := copyDirectoryFromDisk(source.root, temp, true); err != nil {
		return err
	}
	if !isCompleteDeverSkillRoot(temp) {
		return fmt.Errorf("临时 skill 目录不完整: %s", temp)
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		return err
	}
	cleanup = false
	return nil
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

func primaryGlobalSkillTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents", "skills", deverSkillName), nil
}

func globalSkillReferenceTargets(primary string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	primary = filepath.Clean(primary)
	targets := []string{
		filepath.Join(home, ".codex", "skills", deverSkillName),
		filepath.Join(home, ".claude", "skills", deverSkillName),
		filepath.Join(home, ".opencode", "skills", deverSkillName),
		filepath.Join(home, ".trae", "skills", deverSkillName),
		filepath.Join(home, ".qoder", "skills", deverSkillName),
		filepath.Join(home, ".codebuddy", "skills", deverSkillName),
	}
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		targets = append(targets, filepath.Join(codexHome, "skills", deverSkillName))
	}
	return uniquePathsExcept(targets, primary), nil
}

func uniquePathsExcept(items []string, excluded string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = filepath.Clean(strings.TrimSpace(item))
		if item == "." || item == "" {
			continue
		}
		if samePath(item, excluded) {
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

func installSkillReference(source, target string) error {
	if samePath(source, target) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Symlink(source, target)
}

func isSymlinkTo(path, target string) bool {
	link, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	return samePath(link, target)
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
	targets := []struct {
		path  string
		block string
	}{
		{path: filepath.Join(projectRoot, "AGENTS.md"), block: block},
		{path: filepath.Join(projectRoot, "CLAUDE.md"), block: claudeAgentImportBlock()},
	}
	for _, target := range targets {
		changed, err := upsertManagedBlock(target.path, target.block)
		if err != nil {
			return nil, err
		}
		if changed {
			updated = append(updated, target.path)
		}
	}
	return updated, nil
}

func agentInstructionTargets(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(projectRoot, "CLAUDE.md"),
	}
}

func claudeAgentImportBlock() string {
	return deverSkillStart + "\n@AGENTS.md\n" + deverSkillEnd + "\n"
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
