package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillInstallDoesNotManageTrellis(t *testing.T) {
	deverBinary := buildTestDeverBinary(t)
	frameworkRoot := testFrameworkRoot(t)

	help, err := runTestCommandOutput(frameworkRoot, nil, deverBinary, "skill", "install", "--help")
	if err != nil {
		t.Fatalf("dever skill install --help failed: %v\n%s", err, help)
	}
	if strings.Contains(strings.ToLower(help), "trellis") {
		t.Fatalf("skill install help still exposes Trellis support:\n%s", help)
	}

	output, err := runTestCommandOutput(frameworkRoot, nil, deverBinary, "skill", "install", "--trellis=false")
	if err == nil {
		t.Fatalf("removed --trellis flag was still accepted:\n%s", output)
	}
	if !strings.Contains(output, "flag provided but not defined: -trellis") {
		t.Fatalf("removed --trellis flag returned an unexpected error:\n%s", output)
	}

	skillRepo := filepath.Join(t.TempDir(), "skills-dever")
	writeTestFile(t, filepath.Join(skillRepo, "SKILL.md"), "---\nname: shemic-dever\n---\n")
	writeTestFile(t, filepath.Join(skillRepo, "files", "AGENTS.dever.md"), "<!-- dever-skill:start -->\ntest\n<!-- dever-skill:end -->\n")
	runTestCommand(t, skillRepo, nil, "git", "init", "--quiet", "--initial-branch=main")
	runTestCommand(t, skillRepo, nil, "git", "config", "user.email", "dever-test@example.com")
	runTestCommand(t, skillRepo, nil, "git", "config", "user.name", "Dever Test")
	runTestCommand(t, skillRepo, nil, "git", "add", ".")
	runTestCommand(t, skillRepo, nil, "git", "commit", "--quiet", "-m", "test skill")

	projectRoot := filepath.Join(t.TempDir(), "project")
	sentinelPath := filepath.Join(projectRoot, ".trellis", "sentinel")
	writeTestFile(t, sentinelPath, "unchanged\n")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("resolve git: %v", err)
	}
	commandBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(commandBin, 0o755); err != nil {
		t.Fatalf("create command bin: %v", err)
	}
	if err := os.Symlink(gitPath, filepath.Join(commandBin, "git")); err != nil {
		t.Fatalf("link git: %v", err)
	}
	testEnv := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PATH=") {
			testEnv = append(testEnv, entry)
		}
	}
	testEnv = append(testEnv, "PATH="+commandBin)

	output, err = runTestCommandOutput(
		frameworkRoot,
		testEnv,
		deverBinary,
		"skill",
		"install",
		"--project-root="+projectRoot,
		"--global=false",
		"--project=false",
		"--agents=false",
		"--repo="+skillRepo,
		"--ref=main",
	)
	if err != nil {
		t.Fatalf("skill install unexpectedly required a Trellis runtime: %v\n%s", err, output)
	}
	if sentinel := readTestFile(t, sentinelPath); sentinel != "unchanged\n" {
		t.Fatalf("skill install changed the Trellis project: %q", sentinel)
	}
}
