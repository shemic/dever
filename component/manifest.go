package component

import (
	"strings"

	"github.com/shemic/dever/util"
)

const (
	SourceModule  = "module"
	SourcePackage = "package"
)

type Manifest struct {
	Name            string                  `json:"name"`
	Version         string                  `json:"version"`
	Description     string                  `json:"description"`
	Depends         map[string]string       `json:"depends"`
	OptionalDepends map[string]string       `json:"optionalDepends"`
	Public          []string                `json:"public"`
	Sites           map[string]ManifestSite `json:"sites"`
}

type ManifestSite struct {
	Auth     []AuthSeed `json:"auth"`
	Entry    string     `json:"entry"`
	Public   []string   `json:"public"`
	APIRoots []string   `json:"apiRoots"`
}

type AuthSeed struct {
	Key      string            `json:"key"`
	ID       string            `json:"id"`
	Path     string            `json:"path"`
	Name     string            `json:"name"`
	Icon     string            `json:"icon"`
	Parent   string            `json:"parent"`
	Type     int               `json:"type"`
	Sort     int               `json:"sort"`
	Query    map[string]string `json:"query"`
	Children []AuthSeed        `json:"children"`
}

func DecodeManifest(content []byte) (Manifest, error) {
	var manifest Manifest
	if err := util.UnmarshalJSONC(content, &manifest); err != nil {
		return Manifest{}, err
	}
	return normalizeManifest(manifest), nil
}

func normalizeManifest(manifest Manifest) Manifest {
	manifest.Name = cleanName(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Depends = normalizeStringMap(manifest.Depends)
	manifest.OptionalDepends = normalizeStringMap(manifest.OptionalDepends)
	manifest.Public = normalizeStringList(manifest.Public)

	if len(manifest.Sites) > 0 {
		sites := make(map[string]ManifestSite, len(manifest.Sites))
		for key, site := range manifest.Sites {
			siteKey := strings.Trim(strings.TrimSpace(key), "/")
			if siteKey == "" {
				continue
			}
			site.Entry = strings.Trim(strings.TrimSpace(site.Entry), "/")
			site.Public = normalizeStringList(site.Public)
			site.APIRoots = normalizeStringList(site.APIRoots)
			sites[siteKey] = site
		}
		manifest.Sites = sites
	}
	return manifest
}

func normalizeStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]string, len(items))
	for key, value := range items {
		key = cleanName(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cleanName(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "." || value == ".." {
		return ""
	}
	return value
}
