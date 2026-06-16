package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shemic/dever/util"
)

type activeComponentSource struct {
	name   string
	source string
	root   string
}

func listActiveComponentSources(projectRoot string) ([]activeComponentSource, error) {
	moduleSources, err := util.ListModuleSources(projectRoot)
	if err != nil {
		return nil, err
	}

	result := make([]activeComponentSource, 0, len(moduleSources))
	for _, source := range moduleSources {
		if _, err := os.Stat(filepath.Join(source.Root, "dever.json")); err != nil {
			continue
		}
		result = append(result, activeComponentSource{
			name:   source.Name,
			source: activeComponentSourceType(projectRoot, source.Root),
			root:   source.Root,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result, nil
}

func activeComponentSourceType(projectRoot, sourceRoot string) string {
	packageRoot := filepath.Join(projectRoot, "package") + string(os.PathSeparator)
	if strings.HasPrefix(sourceRoot+string(os.PathSeparator), packageRoot) {
		return "package"
	}
	return "module"
}
