package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackageUpdateRepairsMissingLocalReplaceRequirement(t *testing.T) {
	deverBinary := buildTestDeverBinary(t)
	tempRoot := t.TempDir()

	packageRepos := filepath.Join(tempRoot, "repos")
	frontRepo := filepath.Join(packageRepos, "front")
	writeTestPackage(t, frontRepo, "front")
	runTestCommand(t, frontRepo, nil, "git", "init", "--quiet")
	runTestCommand(t, frontRepo, nil, "git", "config", "user.email", "dever-test@example.com")
	runTestCommand(t, frontRepo, nil, "git", "config", "user.name", "Dever Test")
	runTestCommand(t, frontRepo, nil, "git", "add", ".")
	runTestCommand(t, frontRepo, nil, "git", "commit", "--quiet", "-m", "test package")
	runTestCommand(t, frontRepo, nil, "git", "tag", "v0.0.1")

	projectRoot := filepath.Join(tempRoot, "project")
	writeTestFile(t, filepath.Join(projectRoot, "go.mod"), `module my

go 1.26.3

replace github.com/dever-package/crm => ./package/crm
`)
	writeTestFile(t, filepath.Join(projectRoot, "module", "crm", "main.go"), `package crm

// dever:import github.com/dever-package/crm
`)
	writeTestPackage(t, filepath.Join(projectRoot, "package", "crm"), "crm")

	testEnv := append(os.Environ(),
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"GOPROXY=direct",
		"GOSUMDB=off",
		"GOPATH="+filepath.Join(tempRoot, "gopath"),
		"GOMODCACHE="+filepath.Join(tempRoot, "gopath", "pkg", "mod"),
		"GOCACHE="+filepath.Join(tempRoot, "gocache"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=file",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=url.file://"+filepath.ToSlash(packageRepos)+"/.insteadOf",
		"GIT_CONFIG_VALUE_0=https://github.com/dever-package/",
	)
	output, err := runTestCommandOutput(
		projectRoot,
		testEnv,
		deverBinary,
		"package",
		"--ref=v0.0.1",
		"front",
	)
	if err != nil {
		t.Fatalf("dever package front failed: %v\n%s", err, output)
	}

	goMod := readTestFile(t, filepath.Join(projectRoot, "go.mod"))
	if !strings.Contains(goMod, "github.com/dever-package/crm v0.0.0") {
		t.Fatalf("missing local crm requirement was not repaired:\n%s", goMod)
	}
}

func buildTestDeverBinary(t *testing.T) string {
	t.Helper()
	tempRoot := t.TempDir()
	deverBinary := filepath.Join(tempRoot, "dever")
	runTestCommand(t, testFrameworkRoot(t), nil, "go", "build", "-o", deverBinary, "./cmd/dever")
	return deverBinary
}

func testFrameworkRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))
}

func writeTestPackage(t *testing.T, root, name string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module github.com/dever-package/"+name+"\n\ngo 1.26.3\n")
	writeTestFile(t, filepath.Join(root, "dever.json"), `{"name":"`+name+`","version":"0.0.1"}`+"\n")
	writeTestFile(t, filepath.Join(root, "component.go"), `package `+name+`

import "embed"

//go:embed dever.json
var ManifestFS embed.FS
`)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func runTestCommand(t *testing.T, dir string, env []string, name string, args ...string) {
	t.Helper()
	output, err := runTestCommandOutput(dir, env, name, args...)
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func runTestCommandOutput(dir string, env []string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	if env != nil {
		command.Env = env
	}
	output, err := command.CombinedOutput()
	return string(output), err
}
