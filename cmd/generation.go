package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/shemic/dever/util"
)

type ProjectSources struct {
	Root          string
	Module        string
	ModuleSources []util.ModuleSource
}

type goSourceFile struct {
	FullPath string
	RelPath  string
}

func LoadProjectSources(projectRoot string) (ProjectSources, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	rootPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return ProjectSources{}, fmt.Errorf("解析项目根目录失败: %w", err)
	}

	moduleName, err := util.ReadProjectModuleName(filepath.Join(rootPath, "go.mod"))
	if err != nil {
		return ProjectSources{}, err
	}

	moduleSources, err := util.ListModuleSourcesForModule(rootPath, moduleName)
	if err != nil {
		return ProjectSources{}, fmt.Errorf("读取模块目录失败: %w", err)
	}

	return ProjectSources{
		Root:          rootPath,
		Module:        moduleName,
		ModuleSources: moduleSources,
	}, nil
}

func walkGoFilesUnder(source util.ModuleSource, subDir string, visit func(goSourceFile) error) error {
	subDir = strings.Trim(filepath.ToSlash(strings.TrimSpace(subDir)), "/")
	if subDir == "" {
		return nil
	}

	rootDir := filepath.Join(source.Root, filepath.FromSlash(subDir))
	info, err := os.Stat(rootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(rootDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry == nil || entry.IsDir() || !isGoSourceFile(entry.Name()) {
			return nil
		}
		relPath, err := filepath.Rel(source.Root, path)
		if err != nil {
			return nil
		}
		return visit(goSourceFile{
			FullPath: path,
			RelPath:  relPath,
		})
	})
}

func isGoSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}
