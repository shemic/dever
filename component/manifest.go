package component

import (
	"encoding/json"
	"fmt"
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
	Page    string              `json:"page"`
	API     string              `json:"api"`
	Public  []string            `json:"public"`
	Config  ManifestSiteConfig  `json:"config"`
	Setting ManifestSiteSetting `json:"setting"`
	Access  ManifestSiteAccess  `json:"access"`
	Entry   string              `json:"entry"`
	Auth    []AuthSeed          `json:"auth"`
}

type ManifestSiteConfig struct {
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Logo        string `json:"logo"`
	Favicon     string `json:"favicon"`
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
	Shell      string   `json:"shell"`
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
	if err := validateManifestFrontSites(content); err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := util.UnmarshalJSONC(content, &manifest); err != nil {
		return Manifest{}, err
	}
	return normalizeManifest(manifest), nil
}

func validateManifestFrontSites(content []byte) error {
	normalized, err := util.NormalizeJSONC(content)
	if err != nil {
		return err
	}
	var raw struct {
		Front struct {
			Sites map[string]map[string]json.RawMessage `json:"sites"`
		} `json:"front"`
	}
	if err := json.Unmarshal(normalized, &raw); err != nil {
		return err
	}
	for siteKey, site := range raw.Front.Sites {
		for key := range site {
			switch key {
			case "api", "page", "config", "setting", "access", "entry", "public", "auth":
				if key == "config" {
					if err := validateManifestSiteConfig(siteKey, site[key]); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("front.sites.%s 不允许字段 %q；站点展示配置请写入 config", siteKey, key)
			}
		}
	}
	return nil
}

func validateManifestSiteConfig(siteKey string, content json.RawMessage) error {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(content, &config); err != nil {
		return err
	}
	for key := range config {
		switch key {
		case "name", "subtitle", "description", "url", "logo", "favicon":
		default:
			return fmt.Errorf("front.sites.%s.config 不允许字段 %q", siteKey, key)
		}
	}
	return nil
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
			site.Page = strings.Trim(strings.TrimSpace(site.Page), "/")
			site.API = strings.Trim(strings.TrimSpace(site.API), "/")
			site.Entry = strings.Trim(strings.TrimSpace(site.Entry), "/")
			site.Public = normalizeStringList(site.Public)
			site.Config.Name = strings.TrimSpace(site.Config.Name)
			site.Config.Subtitle = strings.TrimSpace(site.Config.Subtitle)
			site.Config.Description = strings.TrimSpace(site.Config.Description)
			site.Config.URL = strings.TrimSpace(site.Config.URL)
			site.Config.Logo = strings.TrimSpace(site.Config.Logo)
			site.Config.Favicon = strings.TrimSpace(site.Config.Favicon)
			site.Setting.Appearance.Theme = strings.TrimSpace(site.Setting.Appearance.Theme)
			site.Setting.Appearance.Sidebar = strings.TrimSpace(site.Setting.Appearance.Sidebar)
			site.Setting.Appearance.Layout = strings.TrimSpace(site.Setting.Appearance.Layout)
			site.Setting.Appearance.Direction = strings.TrimSpace(site.Setting.Appearance.Direction)
			site.Setting.Runtime.Skin = strings.TrimSpace(site.Setting.Runtime.Skin)
			site.Setting.Runtime.RouterMode = strings.TrimSpace(site.Setting.Runtime.RouterMode)
			site.Setting.Runtime.Shell = strings.ToLower(strings.TrimSpace(site.Setting.Runtime.Shell))
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
