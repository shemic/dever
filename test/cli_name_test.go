package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launcher fixtures require POSIX shell commands")
	}
	deverBinary := buildTestDeverBinary(t)

	t.Run("help", func(t *testing.T) {
		help, err := runTestCommandOutput(t.TempDir(), []string{}, deverBinary, "--help")
		if err != nil {
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 1 {
				t.Fatalf("read CLI help: %v\n%s", err, help)
			}
		}
		for _, usage := range []string{"dever-go -", "dever-go install", "dever-go update"} {
			if !strings.Contains(help, usage) {
				t.Errorf("help missing %q:\n%s", usage, help)
			}
		}
		for _, line := range strings.Split(help, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "dever ") {
				t.Errorf("help still advertises the old command: %s", line)
			}
		}
	})

	for _, commandName := range []string{"install", "update"} {
		t.Run(commandName, func(t *testing.T) {
			for _, directoryMode := range []string{"explicit_bin", "path_bin"} {
				t.Run(directoryMode, func(t *testing.T) {
					tempRoot := t.TempDir()
					legacyDir := filepath.Join(tempRoot, "legacy bin")
					binDir := filepath.Join(tempRoot, "go bin")
					legacyScript := "#!/bin/sh\nexit 71\n"
					for _, directory := range []string{legacyDir, binDir} {
						writeCLINameExecutable(t, filepath.Join(directory, "dever"), legacyScript)
					}
					commandPath := filepath.Join(binDir, "dever-go")
					writeCLINameExecutable(t, commandPath, "#!/bin/sh\nexit 72\n")
					testEnv := cliNameTestEnvironment(t, tempRoot, legacyDir, binDir)
					arguments := []string{commandName}
					if directoryMode == "explicit_bin" {
						arguments = append(arguments, "--bin-dir="+binDir)
					}

					if commandName == "install" {
						projectRoot := filepath.Join(tempRoot, "project with spaces")
						frameworkRoot := filepath.Join(projectRoot, "dever")
						writeTestFile(t, filepath.Join(frameworkRoot, "go.mod"), "module github.com/shemic/dever\n")
						writeTestFile(t, filepath.Join(frameworkRoot, "cmd", "dever-go", "main.go"), "package main\n")
						arguments = append(arguments, "--skip-skills", "--project-root="+projectRoot)
						runTestCommand(t, tempRoot, testEnv, deverBinary, arguments...)

						callerRoot := filepath.Join(tempRoot, "caller with spaces")
						writeTestFile(t, filepath.Join(callerRoot, "go.mod"), "module my\n")
						launcherEnv := append(testEnv, "PWD="+callerRoot)
						runTestCommand(t, callerRoot, launcherEnv, commandPath, "model", "--project-root=relative project", "argument with spaces", "")
						compiledPath := filepath.Join(frameworkRoot, "tmp", "dever-cli", "dever-go")
						assertCLINameFile(t, filepath.Join(tempRoot, "go-call"), strings.Join([]string{
							frameworkRoot, "build", "-o", compiledPath, "./cmd/dever-go", "",
						}, "\n"))
						assertCLINameFile(t, filepath.Join(tempRoot, "cli-call"), strings.Join([]string{
							frameworkRoot, callerRoot, "model", "--project-root=relative project", "argument with spaces", "", "",
						}, "\n"))
					} else {
						arguments = append(arguments, "--skip-framework", "--ref=rename-test")
						runTestCommand(t, tempRoot, testEnv, deverBinary, arguments...)
						assertCLINameFile(t, filepath.Join(tempRoot, "go-call"), strings.Join([]string{
							tempRoot, "install", "github.com/shemic/dever/cmd/dever-go@rename-test", "",
						}, "\n"))
						runTestCommand(t, tempRoot, testEnv, commandPath, "--help")
						installedBinary := readTestFile(t, commandPath)
						failedEnv := append(testEnv, "CLI_TEST_FAIL_INSTALL=1")
						output, err := runTestCommandOutput(tempRoot, failedEnv, deverBinary, arguments...)
						if err == nil {
							t.Fatalf("update succeeded despite failed go install:\n%s", output)
						}
						if !strings.Contains(output, "exit status 23") {
							t.Fatalf("update failed before the simulated install failure: %v\n%s", err, output)
						}
						assertCLINameFile(t, commandPath, installedBinary)
						runTestCommand(t, tempRoot, testEnv, commandPath, "--help")
					}

					for _, directory := range []string{legacyDir, binDir} {
						assertCLINameFile(t, filepath.Join(directory, "dever"), legacyScript)
					}
					if _, err := os.Stat(filepath.Join(legacyDir, "dever-go")); !os.IsNotExist(err) {
						t.Fatalf("unexpected command in legacy PATH directory: %v", err)
					}
				})
			}
		})
	}
}

func cliNameTestEnvironment(t *testing.T, tempRoot string, pathDirs ...string) []string {
	t.Helper()
	toolsDir := filepath.Join(tempRoot, "tools")
	writeCLINameExecutable(t, filepath.Join(toolsDir, "go"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$PWD" "$@" > "$CLI_TEST_ROOT/go-call"
case "$1" in
  build)
    test "$#" -eq 4
    test "$2" = -o
    target="$3"
    ;;
  install)
    test "$#" -eq 2
    package="${2%@*}"
    target="$GOBIN/${package##*/}"
    if [ "${CLI_TEST_FAIL_INSTALL:-0}" = 1 ]; then
      printf 'partial download\n' > "$target"
      exit 23
    fi
    ;;
  *)
    exit 24
    ;;
esac
cp "$CLI_TEST_ROOT/fake-cli" "$target"
chmod +x "$target"
`)
	writeCLINameExecutable(t, filepath.Join(tempRoot, "fake-cli"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$PWD" "${DEVER_CALLER_DIR:-}" "$@" > "$CLI_TEST_ROOT/cli-call"
`)
	for _, commandName := range []string{"bash", "mkdir", "dirname", "find", "date", "sleep", "cp", "chmod"} {
		commandPath, err := exec.LookPath(commandName)
		if err != nil {
			t.Fatalf("resolve fixture command %s: %v", commandName, err)
		}
		if err := os.Symlink(commandPath, filepath.Join(toolsDir, commandName)); err != nil {
			t.Fatalf("link fixture command %s: %v", commandName, err)
		}
	}
	pathDirs = append(pathDirs, toolsDir)
	return []string{
		"PATH=" + strings.Join(pathDirs, string(os.PathListSeparator)),
		"CLI_TEST_ROOT=" + tempRoot,
	}
}

func writeCLINameExecutable(t *testing.T, path, content string) {
	t.Helper()
	writeTestFile(t, path, content)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("make fixture executable %s: %v", path, err)
	}
}

func assertCLINameFile(t *testing.T, path, expected string) {
	t.Helper()
	if actual := readTestFile(t, path); actual != expected {
		t.Fatalf("unexpected %s content:\ngot:  %q\nwant: %q", path, actual, expected)
	}
}
