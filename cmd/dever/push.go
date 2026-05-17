package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

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
	status, err := gitOutput(projectRoot, "status", "--short")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		fmt.Println("dever push: 工作区没有修改，直接执行 git push")
		return gitRun(projectRoot, "push")
	}

	fmt.Println("dever push: git status --short")
	fmt.Print(status)

	files, err := gitStatusFiles(projectRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("dever push: 没有可 add 的文件，直接执行 git push")
		return gitRun(projectRoot, "push")
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

	fmt.Println("dever push: git push")
	return gitRun(projectRoot, "push")
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
