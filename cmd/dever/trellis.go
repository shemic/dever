package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	trellisPackage        = "@mindfoldhq/trellis"
	defaultTrellisVersion = "latest"
	trellisAgentStart     = "<!-- TRELLIS:START -->"
	trellisAgentEnd       = "<!-- TRELLIS:END -->"
)

const trellisAgentBlock = `<!-- TRELLIS:START -->
# Trellis

本项目使用 Trellis 保存跨会话的规范、任务和工作记录：

- .trellis/workflow.md：任务阶段和 skill 路由
- .trellis/spec/：按 package/layer 组织的工程规范
- .trellis/tasks/：复杂任务的 PRD、设计和执行上下文
- .trellis/workspace/：开发者工作日志

项目 AGENTS.md 中的显式规则优先于 Trellis 默认流程。需求明确的小改动可直接实施；需求不清、跨模块或高风险工作再创建 Trellis 任务。项目禁止 build/test 时，不得由 Trellis 验证阶段绕过该约束。未经用户明确要求，不创建 Git commit。
<!-- TRELLIS:END -->`

const trellisNoTaskBlock = `[workflow-state:no_task]
No active task. Classify the current turn before acting.
Simple conversation or a small, well-scoped task: proceed directly without asking whether to create a Trellis task.
Complex, ambiguous, cross-module, or high-risk work: ask the user for consent before creating a Trellis task and entering planning.
[/workflow-state:no_task]`

type trellisInstallOptions struct {
	projectRoot string
	project     bool
	user        string
	version     string
}

func runTrellisInstall(options trellisInstallOptions) error {
	if err := validateTrellisRuntime(); err != nil {
		return err
	}

	version := strings.TrimSpace(options.version)
	if version == "" {
		version = defaultTrellisVersion
	}
	packageSpec := trellisPackage + "@" + version
	fmt.Printf("dever skill install: 正在安装或更新 Trellis: %s\n", packageSpec)
	if err := runVisibleCommand("", "npm", "install", "--global", "--no-audit", "--no-fund", packageSpec); err != nil {
		return fmt.Errorf("npm install --global %s 失败: %w", packageSpec, err)
	}

	trellisBin, err := resolveTrellisBinary()
	if err != nil {
		return err
	}
	fmt.Printf("dever skill install: Trellis CLI 已就绪: %s\n", trellisBin)
	if !options.project {
		return nil
	}

	projectRoot, err := filepath.Abs(options.projectRoot)
	if err != nil {
		return fmt.Errorf("解析 Trellis 项目目录失败: %w", err)
	}
	if err := syncTrellisProject(trellisBin, projectRoot, options.user); err != nil {
		return err
	}
	if err := configureTrellisProject(projectRoot); err != nil {
		return err
	}
	changed, err := upsertManagedBlock(
		filepath.Join(projectRoot, "AGENTS.md"),
		trellisAgentStart,
		trellisAgentEnd,
		trellisAgentBlock,
	)
	if err != nil {
		return fmt.Errorf("写入 Trellis agent 提示失败: %w", err)
	}
	if changed {
		fmt.Printf("dever skill install: 已更新 Trellis agent 提示: %s\n", filepath.Join(projectRoot, "AGENTS.md"))
	}
	return nil
}

func validateTrellisRuntime() error {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("Trellis 需要 Node.js 18+，未找到 node；不需要 Trellis 时可使用 --trellis=false")
	}
	output, err := exec.Command(nodeBin, "--version").Output()
	if err != nil {
		return fmt.Errorf("读取 Node.js 版本失败: %w", err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output)), "v")
	majorText, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(majorText)
	if err != nil || major < 18 {
		return fmt.Errorf("Trellis 需要 Node.js 18+，当前版本为 %q", strings.TrimSpace(string(output)))
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("Trellis 需要 npm，未找到 npm；不需要 Trellis 时可使用 --trellis=false")
	}
	return nil
}

func resolveTrellisBinary() (string, error) {
	if path, err := exec.LookPath("trellis"); err == nil {
		return path, nil
	}
	output, err := exec.Command("npm", "prefix", "--global").Output()
	if err != nil {
		return "", fmt.Errorf("Trellis 已安装但无法解析 npm 全局目录: %w", err)
	}
	name := "trellis"
	if runtime.GOOS == "windows" {
		name = "trellis.cmd"
	}
	prefix := strings.TrimSpace(string(output))
	for _, candidate := range []string{
		filepath.Join(prefix, "bin", name),
		filepath.Join(prefix, name),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Trellis 安装完成但未找到命令，请将 npm 全局 bin 目录加入 PATH")
}

func syncTrellisProject(trellisBin, projectRoot, configuredUser string) error {
	trellisRoot := filepath.Join(projectRoot, ".trellis")
	if info, err := os.Stat(trellisRoot); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("项目 Trellis 路径不是目录: %s", trellisRoot)
		}
		fmt.Printf("dever skill install: 正在更新项目 Trellis: %s\n", projectRoot)
		if err := runVisibleCommand(projectRoot, trellisBin, "update", "--skip-all"); err != nil {
			return fmt.Errorf("更新项目 Trellis 失败；如版本要求迁移，请手动执行 trellis update --migrate --skip-all: %w", err)
		}
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("检查项目 Trellis 目录失败: %w", err)
	}

	userName := resolveTrellisUser(projectRoot, configuredUser)
	args := []string{"init", "--codex", "--yes", "--skip-existing"}
	if userName != "" {
		args = append(args, "--user", userName)
	}
	fmt.Printf("dever skill install: 正在初始化项目 Trellis: %s\n", projectRoot)
	if err := runVisibleCommand(projectRoot, trellisBin, args...); err != nil {
		return fmt.Errorf("初始化项目 Trellis 失败: %w", err)
	}
	if info, err := os.Stat(trellisRoot); err != nil || !info.IsDir() {
		return fmt.Errorf("Trellis 初始化完成但项目缺少目录: %s", trellisRoot)
	}
	return nil
}

func resolveTrellisUser(projectRoot, configured string) string {
	if value := strings.TrimSpace(configured); value != "" {
		return value
	}
	for _, command := range [][]string{
		{"git", "config", "user.name"},
		{"git", "config", "--global", "user.name"},
	} {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Dir = projectRoot
		if output, err := cmd.Output(); err == nil {
			if value := strings.TrimSpace(string(output)); value != "" {
				return value
			}
		}
	}
	for _, name := range []string{"USER", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return "dever"
}

func disableTrellisAutoCommit(projectRoot string) error {
	configPath := filepath.Join(projectRoot, ".trellis", "config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取 Trellis 配置失败: %s: %w", configPath, err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	changed := false
	for index, line := range lines {
		if !strings.HasPrefix(line, "session_auto_commit:") {
			continue
		}
		found = true
		if strings.TrimSpace(line) != "session_auto_commit: false" {
			lines[index] = "session_auto_commit: false"
			changed = true
		}
	}
	if !found {
		lines = append(lines, "", "# Dever keeps Git changes under explicit user control.", "session_auto_commit: false")
		changed = true
	}
	if !changed {
		return nil
	}
	next := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	if err := os.WriteFile(configPath, []byte(next), 0o644); err != nil {
		return fmt.Errorf("关闭 Trellis 自动提交失败: %w", err)
	}
	fmt.Printf("dever skill install: 已关闭 Trellis 自动提交: %s\n", configPath)
	return nil
}

func configureTrellisProject(projectRoot string) error {
	if err := disableTrellisAutoCommit(projectRoot); err != nil {
		return err
	}
	if err := relaxTrellisSmallTaskGate(projectRoot); err != nil {
		return err
	}
	return nil
}

func relaxTrellisSmallTaskGate(projectRoot string) error {
	workflowPath := filepath.Join(projectRoot, ".trellis", "workflow.md")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		return fmt.Errorf("读取 Trellis workflow 失败: %s: %w", workflowPath, err)
	}

	current := string(content)
	next, err := replaceTrellisNoTaskBlock(current)
	if err != nil {
		return fmt.Errorf("配置 Dever 轻量 Trellis workflow 失败: %w", err)
	}
	next = strings.Replace(
		next,
		"- Simple conversation or small task: ask only whether this turn should create a Trellis task. If the user says no, skip Trellis for this session.",
		"- Simple conversation or small, well-scoped task: proceed directly without creating a Trellis task or asking for task-creation consent.",
		1,
	)
	if next == current {
		return nil
	}
	if err := os.WriteFile(workflowPath, []byte(next), 0o644); err != nil {
		return fmt.Errorf("写入 Dever 轻量 Trellis workflow 失败: %w", err)
	}
	fmt.Printf("dever skill install: 已启用 Dever 轻量 Trellis workflow: %s\n", workflowPath)
	return nil
}

func replaceTrellisNoTaskBlock(content string) (string, error) {
	startMarker := "[workflow-state:no_task]"
	endMarker := "[/workflow-state:no_task]"
	startToken := "\n" + startMarker + "\n"
	start := strings.Index(content, startToken)
	if start < 0 {
		return "", fmt.Errorf("缺少独立的 %s 区块", startMarker)
	}
	blockStart := start + 1
	endToken := "\n" + endMarker
	endOffset := strings.Index(content[blockStart:], endToken)
	if endOffset < 0 {
		return "", fmt.Errorf("缺少 %s", endMarker)
	}
	blockEnd := blockStart + endOffset + len(endToken)
	return content[:blockStart] + trellisNoTaskBlock + content[blockEnd:], nil
}

func runVisibleCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
