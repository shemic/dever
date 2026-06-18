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
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	Depends         map[string]string `json:"depends"`
	OptionalDepends map[string]string `json:"optionalDepends"`
	Front           ManifestFront     `json:"front"`
	Skills          []string          `json:"skills"`
}

type ManifestFront struct {
	Public []string                `json:"public"`
	Sites  map[string]ManifestSite `json:"sites"`
}

type ManifestSite struct {
	Name        string              `json:"name"`
	Subtitle    string              `json:"subtitle"`
	Description string              `json:"description"`
	URL         string              `json:"url"`
	Page        string              `json:"page"`
	API         string              `json:"api"`
	Public      []string            `json:"public"`
	Assets      ManifestSiteAssets  `json:"assets"`
	Setting     ManifestSiteSetting `json:"setting"`
	Access      ManifestSiteAccess  `json:"access"`
	Entry       string              `json:"entry"`
	Auth        []AuthSeed          `json:"auth"`
}

type ManifestSiteAssets struct {
	Logo    string `json:"logo"`
	Favicon string `json:"favicon"`
}

type ManifestSiteSetting struct {
	Appearance ManifestAppearanceSetting `json:"appearance"`
	Runtime    ManifestRuntimeSetting    `json:"runtime"`
}

type ManifestAppearanceSetting struct {
	Theme     string `json:"theme"`
	Sidebar   string `json:"sidebar"`
	Layout    string `json:"layout"`
	Direction string `json:"direction"`
}

type ManifestRuntimeSetting struct {
	Skin       string   `json:"skin"`
	RouterMode string   `json:"routerMode"`
	Plugins    []string `json:"plugins,omitempty"`
}

type ManifestSiteAccess struct {
	Mode         string `json:"mode"`
	AuthProvider string `json:"authProvider"`
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
	manifest.Front.Public = normalizeStringList(manifest.Front.Public)
	manifest.Skills = normalizeStringList(manifest.Skills)

	if len(manifest.Front.Sites) > 0 {
		sites := make(map[string]ManifestSite, len(manifest.Front.Sites))
		for key, site := range manifest.Front.Sites {
			siteKey := strings.Trim(strings.TrimSpace(key), "/")
			if siteKey == "" {
				continue
			}
			site.Name = strings.TrimSpace(site.Name)
			site.Subtitle = strings.TrimSpace(site.Subtitle)
			site.Description = strings.TrimSpace(site.Description)
			site.URL = strings.TrimSpace(site.URL)
			site.Page = strings.Trim(strings.TrimSpace(site.Page), "/")
			site.API = strings.Trim(strings.TrimSpace(site.API), "/")
			site.Entry = strings.Trim(strings.TrimSpace(site.Entry), "/")
			site.Public = normalizeStringList(site.Public)
			site.Assets.Logo = strings.TrimSpace(site.Assets.Logo)
			site.Assets.Favicon = strings.TrimSpace(site.Assets.Favicon)
			site.Setting.Appearance.Theme = strings.TrimSpace(site.Setting.Appearance.Theme)
			site.Setting.Appearance.Sidebar = strings.TrimSpace(site.Setting.Appearance.Sidebar)
			site.Setting.Appearance.Layout = strings.TrimSpace(site.Setting.Appearance.Layout)
			site.Setting.Appearance.Direction = strings.TrimSpace(site.Setting.Appearance.Direction)
			site.Setting.Runtime.Skin = strings.TrimSpace(site.Setting.Runtime.Skin)
			site.Setting.Runtime.RouterMode = strings.TrimSpace(site.Setting.Runtime.RouterMode)
			site.Setting.Runtime.Plugins = normalizeStringList(site.Setting.Runtime.Plugins)
			site.Access.Mode = strings.ToLower(strings.TrimSpace(site.Access.Mode))
			site.Access.AuthProvider = strings.Trim(strings.TrimSpace(site.Access.AuthProvider), "/")
			sites[siteKey] = site
		}
		manifest.Front.Sites = sites
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
