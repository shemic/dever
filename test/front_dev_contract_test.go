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
