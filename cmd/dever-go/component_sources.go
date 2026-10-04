package main

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/shemic/dever/util"
)

type activeComponentSource struct {
	name       string
	source     string
	root       string
	importPath string
	editable   bool
	external   bool
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
			name:       source.Name,
			source:     activeComponentSourceType(source),
			root:       source.Root,
			importPath: source.Import,
			editable:   source.Editable,
			external:   source.External,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result, nil
}

func activeComponentSourceType(source util.ModuleSource) string {
	switch source.Kind {
	case util.ModuleSourceKindPackage:
		return util.ModuleSourceKindPackage
	case util.ModuleSourceKindModule:
		return util.ModuleSourceKindModule
	default:
		return util.ModuleSourceKindModule
	}
}
