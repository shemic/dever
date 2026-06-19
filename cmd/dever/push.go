package main

import (
	"errors"
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

const (
	deverFrameworkModule = "github.com/shemic/dever"
	deverPackagePrefix   = "github.com/dever-package/"
)

var (
	releaseTagPattern   = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	versionConstPattern = regexp.MustCompile(`(?m)^const\s+Version\s*=\s*"([^"]+)"`)
)

type pushReleaseMetadata struct {
	ModulePath string
	Version    string
	Tag        string
}

func runPush(args []string) {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	message := "edit"
	fs.StringVar(&message, "message", "edit", "提交信息")
	fs.StringVar(&message, "m", "edit", "提交信息")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("push 参数解析失败: %v", err)
	}
	if fs.NArg() > 0 {
		log.Fatal("push 不接受位置参数，请使用 --project-root 或 --message")
	}
	if strings.TrimSpace(message) == "" {
		log.Fatal("提交信息不能为空")
	}

	root := resolveProjectRoot(*projectRoot)
	if err := runGitPush(root, strings.TrimSpace(message)); err != nil {
		log.Fatalf("push 执行失败: %v", err)
	}
}

func runGitPush(projectRoot, message string) error {
	release, err := loadPushReleaseMetadata(projectRoot)
	if err != nil {
		return err
	}

	if err := commitGitChanges(projectRoot, message); err != nil {
		return err
	}
	if err := ensureReleaseTag(projectRoot, release); err != nil {
		return err
	}

	fmt.Println("dever push: git push")
	if err := gitRun(projectRoot, "push"); err != nil {
		return err
	}

	return pushReleaseTag(projectRoot, release)
}

func commitGitChanges(projectRoot, message string) error {
	status, err := gitOutput(projectRoot, "status", "--short")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		fmt.Println("dever push: 工作区没有修改，跳过 commit")
		return nil
	}

	fmt.Println("dever push: git status --short")
	fmt.Print(status)

	files, err := gitStatusFiles(projectRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("dever push: 没有可 add 的文件，跳过 commit")
		return nil
	}

	fmt.Printf("dever push: git add %d 个文件\n", len(files))
	if err := gitRun(projectRoot, append([]string{"add", "--"}, files...)...); err != nil {
		return err
	}

	staged, err := hasStagedChanges(projectRoot, files)
	if err != nil {
		return err
	}
	if staged {
		fmt.Printf("dever push: git commit -m %q\n", message)
		if err := gitRun(projectRoot, append([]string{"commit", "-m", message, "--"}, files...)...); err != nil {
			return err
		}
	} else {
		fmt.Println("dever push: git add 后没有可提交变更，跳过 commit")
	}

	return nil
}

func loadPushReleaseMetadata(projectRoot string) (pushReleaseMetadata, error) {
	content, _, err := util.ReadJSONCFile(filepath.Join(projectRoot, "dever.json"))
	if errors.Is(err, os.ErrNotExist) {
		return pushReleaseMetadata{}, nil
	}
	if err != nil {
		return pushReleaseMetadata{}, fmt.Errorf("读取 dever.json 失败: %w", err)
	}

	manifest, err := component.DecodeManifest(content)
	if err != nil {
		return pushReleaseMetadata{}, fmt.Errorf("解析 dever.json 失败: %w", err)
	}
	if manifest.Version == "" {
		return pushReleaseMetadata{}, nil
	}

	modulePath, err := util.ReadProjectModuleName(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return pushReleaseMetadata{}, nil
	}

	ok, err := isReleaseModule(modulePath, manifest.Name)
	if err != nil {
		return pushReleaseMetadata{}, err
	}
	if !ok {
		return pushReleaseMetadata{}, nil
	}

	tag, err := normalizeReleaseTag(manifest.Version)
	if err != nil {
		return pushReleaseMetadata{}, err
	}

	release := pushReleaseMetadata{
		ModulePath: modulePath,
		Version:    manifest.Version,
		Tag:        tag,
	}
	if err := validateFrameworkReleaseVersion(projectRoot, release); err != nil {
		return pushReleaseMetadata{}, err
	}
	return release, nil
}

func isReleaseModule(modulePath, manifestName string) (bool, error) {
	if modulePath == deverFrameworkModule {
		return true, nil
	}
	if !strings.HasPrefix(modulePath, deverPackagePrefix) {
		return false, nil
	}

	packageName := strings.Trim(strings.TrimPrefix(modulePath, deverPackagePrefix), "/")
	if packageName == "" || strings.Contains(packageName, "/") {
		return false, fmt.Errorf("package module 路径不合法: %s", modulePath)
	}
	if manifestName != "" && manifestName != packageName {
		return false, fmt.Errorf("go.mod module=%s 与 dever.json.name=%s 不一致", modulePath, manifestName)
	}
	return true, nil
}

func normalizeReleaseTag(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", nil
	}
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	if !releaseTagPattern.MatchString(tag) {
		return "", fmt.Errorf("dever.json.version=%q 不能转换成 Go module tag，期望格式如 0.1.0 或 v0.1.0", version)
	}
	return tag, nil
}

func validateFrameworkReleaseVersion(projectRoot string, release pushReleaseMetadata) error {
	if release.ModulePath != deverFrameworkModule {
		return nil
	}

	content, err := os.ReadFile(filepath.Join(projectRoot, "version.go"))
	if err != nil {
		return fmt.Errorf("读取 version.go 失败: %w", err)
	}
	matches := versionConstPattern.FindSubmatch(content)
	if len(matches) < 2 {
		return fmt.Errorf("version.go 缺少 Version 常量")
	}
	version := strings.TrimSpace(string(matches[1]))
	if version != release.Tag {
		return fmt.Errorf("version.go Version=%q 与 dever.json.version=%q（%s）不一致", version, release.Version, release.Tag)
	}
	return nil
}

func ensureReleaseTag(projectRoot string, release pushReleaseMetadata) error {
	if release.Tag == "" {
		return nil
	}

	head, err := gitOutput(projectRoot, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	head = strings.TrimSpace(head)
	if head == "" {
		return fmt.Errorf("无法获取当前 HEAD")
	}

	tagCommit, exists, err := gitOptionalOutput(projectRoot, "rev-parse", "-q", "--verify", "refs/tags/"+release.Tag+"^{}")
	if err != nil {
		return err
	}
	if exists {
		tagCommit = strings.TrimSpace(tagCommit)
		if tagCommit != head {
			return fmt.Errorf("tag %s 已存在但不在当前 HEAD，请先更新 dever.json.version 或手动处理 tag", release.Tag)
		}
		fmt.Printf("dever push: tag %s 已在当前 HEAD，跳过 git tag\n", release.Tag)
	} else {
		fmt.Printf("dever push: git tag %s（来自 dever.json.version=%s）\n", release.Tag, release.Version)
		if err := gitRun(projectRoot, "tag", release.Tag); err != nil {
			return err
		}
	}
	return nil
}

func pushReleaseTag(projectRoot string, release pushReleaseMetadata) error {
	if release.Tag == "" {
		return nil
	}
	remote := currentPushRemote(projectRoot)
	fmt.Printf("dever push: git push %s %s\n", remote, release.Tag)
	return gitRun(projectRoot, "push", remote, release.Tag)
}

func currentPushRemote(projectRoot string) string {
	branch, err := gitOutput(projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "origin"
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		return "origin"
	}

	remote, ok, err := gitOptionalOutput(projectRoot, "config", "--get", "branch."+branch+".remote")
	if err != nil || !ok {
		return "origin"
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "origin"
	}
	return remote
}

func gitStatusFiles(projectRoot string) ([]string, error) {
	status, err := gitOutput(projectRoot, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	if status == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	entries := strings.Split(status, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 {
			return nil, fmt.Errorf("无法解析 git status 输出: %q", entry)
		}

		state := entry[:2]
		if state == "!!" {
			continue
		}
		addGitPath(seen, entry[3:])

		if strings.ContainsAny(state, "RC") && i+1 < len(entries) {
			i++
			addGitPath(seen, entries[i])
		}
	}

	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

func addGitPath(seen map[string]struct{}, path string) {
	if path == "" {
		return
	}
	seen[path] = struct{}{}
}

func hasStagedChanges(projectRoot string, files []string) (bool, error) {
	args := append([]string{"diff", "--cached", "--quiet", "--"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, gitCommandError(args, err)
	}
	return false, nil
}

func gitOptionalOutput(projectRoot string, args ...string) (string, bool, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, gitCommandError(args, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))))
	}
	return string(output), true, nil
}

func gitOutput(projectRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", gitCommandError(args, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))))
	}
	return string(output), nil
}

func gitRun(projectRoot string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return gitCommandError(args, err)
	}
	return nil
}

func gitCommandError(args []string, err error) error {
	return fmt.Errorf("git %s 执行失败: %w", strings.Join(args, " "), err)
}
