package integration_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFrontPluginDevUsesEditableSourcesOnly(t *testing.T) {
	source := readTestFile(t, filepath.Join(testFrameworkRoot(t), "cmd", "dever", "front_dev.go"))

	if strings.Contains(source, "current.editable || !hasFrontPluginDist(current.root)") {
		t.Fatal("dever run still falls back to serving external package source when dist is missing")
	}

	required := []string{"if !current.editable {"}
	for _, fragment := range required {
		if !strings.Contains(source, fragment) {
			t.Fatalf("dever run source selection is missing %q", fragment)
		}
	}
}

func TestRunFrontPluginDevExportsSourceSelectionAndVersion(t *testing.T) {
	source := readTestFile(t, filepath.Join(testFrameworkRoot(t), "cmd", "dever", "front_dev.go"))

	required := []string{
		"frontPluginDevNamesEnv",
		"frontPluginDevVersionEnv",
		"sourceNames: append([]string(nil), plugins.names...)",
		"version:     newFrontPluginDevVersion()",
		"frontPluginDevNamesEnv:",
		"strings.Join(s.sourceNames, \",\")",
		"frontPluginDevVersionEnv:",
		"s.version",
	}
	for _, fragment := range required {
		if !strings.Contains(source, fragment) {
			t.Fatalf("dever run front plugin contract is missing %q", fragment)
		}
	}
}

func TestRunRestartsBackendAfterFrontPluginDevServerRestart(t *testing.T) {
	source := readTestFile(t, filepath.Join(testFrameworkRoot(t), "cmd", "dever", "run.go"))
	if !strings.Contains(source, "process.restart(\"刷新前端插件源码编译环境\", false)") {
		t.Fatal("dever run must restart the backend process to propagate a new front plugin dev session")
	}
}
