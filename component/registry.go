package component

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

const (
	defaultManifestPath = "dever.json"
	defaultPagePrefix   = "."
	defaultFrontPrefix  = "."
)

type Definition struct {
	Name         string
	Source       string
	ImportPath   string
	ManifestFS   fs.FS
	ManifestPath string
	PageFS       fs.FS
	PagePrefix   string
	FrontFS      fs.FS
	FrontPrefix  string
	DiskDir      string
}

type Component struct {
	Name        string
	Source      string
	ImportPath  string
	Manifest    Manifest
	PageFS      fs.FS
	PagePrefix  string
	FrontFS     fs.FS
	FrontPrefix string
	DiskDir     string
}

var registry struct {
	mu         sync.RWMutex
	components []Component
	byName     map[string]Component
}

func Register(def Definition) {
	component, err := buildComponent(def)
	if err != nil {
		panic(err)
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	for index, current := range registry.components {
		if current.Name == component.Name && current.Source == component.Source {
			registry.components[index] = component
			sortComponents(registry.components)
			reindexComponents()
			return
		}
	}
	registry.components = append(registry.components, component)
	sortComponents(registry.components)
	reindexComponents()
}

func Active() []Component {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	result := make([]Component, len(registry.components))
	copy(result, registry.components)
	return result
}

func Find(name string) (Component, bool) {
	name = cleanName(name)
	if name == "" {
		return Component{}, false
	}

	registry.mu.RLock()
	defer registry.mu.RUnlock()

	component, ok := registry.byName[name]
	if !ok {
		return Component{}, false
	}
	return component, true
}

func buildComponent(def Definition) (Component, error) {
	name := cleanName(def.Name)
	if name == "" || def.ManifestFS == nil {
		return Component{}, fmt.Errorf("component: invalid definition for %q", def.Name)
	}
	manifestPath := strings.Trim(strings.TrimSpace(def.ManifestPath), "/")
	if manifestPath == "" {
		manifestPath = defaultManifestPath
	}
	content, err := fs.ReadFile(def.ManifestFS, manifestPath)
	if err != nil {
		return Component{}, fmt.Errorf("component %s: read manifest failed: %w", name, err)
	}
	manifest, err := DecodeManifest(content)
	if err != nil {
		return Component{}, fmt.Errorf("component %s: decode manifest failed: %w", name, err)
	}
	if manifest.Name == "" {
		manifest.Name = name
	}

	return Component{
		Name:        name,
		Source:      normalizeSource(def.Source),
		ImportPath:  strings.TrimSpace(def.ImportPath),
		Manifest:    manifest,
		PageFS:      def.PageFS,
		PagePrefix:  cleanPrefix(def.PagePrefix, defaultPagePrefix),
		FrontFS:     def.FrontFS,
		FrontPrefix: cleanPrefix(def.FrontPrefix, defaultFrontPrefix),
		DiskDir:     strings.TrimSpace(def.DiskDir),
	}, nil
}

func normalizeSource(value string) string {
	switch strings.TrimSpace(value) {
	case SourceModule:
		return SourceModule
	default:
		return SourcePackage
	}
}

func cleanPrefix(value string, fallback string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." {
		return fallback
	}
	return value
}

func sortComponents(items []Component) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		return items[i].Name < items[j].Name
	})
}

func reindexComponents() {
	byName := make(map[string]Component, len(registry.components))
	for _, component := range registry.components {
		if _, exists := byName[component.Name]; !exists {
			byName[component.Name] = component
		}
	}
	registry.byName = byName
}
