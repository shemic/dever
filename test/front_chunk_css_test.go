package integration_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFrontPluginInjectsLateChunkCSSMetadata(t *testing.T) {
	source := readTestFile(
		t,
		filepath.Join(testFrameworkRoot(t), "compiler", "front", "vite.config.ts"),
	)
	auditSource := readTestFile(
		t,
		filepath.Join(
			testFrameworkRoot(t),
			"compiler",
			"front",
			"src",
			"bundle-audit.ts",
		),
	)

	required := []string{
		"function injectChunkCSSLoader(",
		"const injected = injectChunkCSSLoader(",
		"generateBundle: {",
		`order: "post"`,
		"chunk.code = injected",
	}
	for _, fragment := range required {
		if !strings.Contains(source, fragment) {
			t.Fatalf("split front plugin CSS loading is missing %q", fragment)
		}
	}

	if count := strings.Count(source, "await window.DeverFront.ensureStyles"); count != 1 {
		t.Fatalf("chunk CSS loader must have one shared implementation, got %d", count)
	}
	cssPluginIndex := strings.Index(source, "? [pluginChunkCSSPlugin()]")
	auditPluginIndex := strings.Index(source, "bundleAuditPlugin({")
	if cssPluginIndex == -1 || auditPluginIndex == -1 || cssPluginIndex >= auditPluginIndex {
		t.Fatal("late chunk CSS injection must be registered before bundle audit")
	}

	for _, fragment := range []string{`enforce: "post"`, `order: "post"`} {
		if !strings.Contains(auditSource, fragment) {
			t.Fatalf("bundle audit must run after late chunk CSS injection: missing %q", fragment)
		}
	}
}
