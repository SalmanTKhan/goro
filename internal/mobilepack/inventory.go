package mobilepack

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kivutar/goro/res"
)

// Inventory is the lightweight source catalog used by the local pack editor.
// It indexes archive tables and map roots without extracting the source data.
type Inventory struct {
	Format         string         `json:"format"`
	Version        int            `json:"version"`
	Root           string         `json:"root"`
	Fingerprint    string         `json:"fingerprint"`
	LooseFiles     int            `json:"loose_files"`
	LooseBytes     int64          `json:"loose_bytes"`
	Packs          []ArchiveInfo  `json:"packs"`
	Archives       []ArchiveInfo  `json:"archives"`
	Maps           []MapInfo      `json:"maps"`
	CategoryCounts map[string]int `json:"category_counts"`
}

type ArchiveInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Files int    `json:"files"`
}

type MapInfo struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

type ResourceInfo struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	SourceBytes   int64  `json:"source_bytes"`
	ExpandedBytes int64  `json:"expanded_bytes"`
	Category      string `json:"category"`
}

type ResourcePage struct {
	Format  string         `json:"format"`
	Version int            `json:"version"`
	Prefix  string         `json:"prefix"`
	Offset  int            `json:"offset"`
	Limit   int            `json:"limit"`
	Total   int            `json:"total"`
	Files   []ResourceInfo `json:"files"`
}

// Inspect returns a deterministic, non-extracting inventory of a client data
// directory. It follows the same archive discovery and precedence order as
// res.Manager.
func Inspect(dataDir string) (Inventory, error) {
	if dataDir == "" {
		return Inventory{}, fmt.Errorf("mobile pack inventory requires data directory")
	}
	manager, err := res.NewManager(dataDir)
	if err != nil {
		return Inventory{}, fmt.Errorf("resource manager: %w", err)
	}
	defer closeManager(manager)

	inventory := Inventory{
		Format:         "goro-mobile-pack-inventory",
		Version:        1,
		Root:           manager.Root,
		CategoryCounts: map[string]int{},
	}
	mapSources := map[string]map[string]struct{}{}
	addMap := func(name, source string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if mapSources[key] == nil {
			mapSources[key] = map[string]struct{}{}
		}
		mapSources[key][source] = struct{}{}
	}

	err = filepath.WalkDir(manager.Root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(manager.Root, filePath)
			if err != nil {
				return err
			}
			parts := strings.Split(filepath.ToSlash(relative), "/")
			if len(parts) > 0 {
				if _, excluded := completeExcludedDirectories[strings.ToLower(parts[0])]; excluded {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(manager.Root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if !isCompleteLooseResource(name) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		inventory.LooseFiles++
		inventory.LooseBytes += info.Size()
		canonical, err := canonicalPackPath(name)
		if err != nil {
			return err
		}
		inventory.CategoryCounts[inventoryCategory(canonical)]++
		if mapName, ok := mapNameFromResource(canonical); ok {
			addMap(mapName, "loose:"+name)
		}
		return nil
	})
	if err != nil {
		return Inventory{}, fmt.Errorf("scan loose resources: %w", err)
	}

	for _, pack := range manager.Packs {
		info := ArchiveInfo{Name: filepath.Base(pack.Path()), Path: pack.Path(), Files: pack.Count()}
		inventory.Packs = append(inventory.Packs, info)
		for _, name := range pack.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				return Inventory{}, fmt.Errorf("pack %s entry %q: %w", pack.Path(), name, err)
			}
			inventory.CategoryCounts[inventoryCategory(canonical)]++
			if mapName, ok := mapNameFromResource(canonical); ok {
				addMap(mapName, "pak:"+filepath.Base(pack.Path()))
			}
		}
	}
	for _, archive := range manager.Archives {
		info := ArchiveInfo{Name: filepath.Base(archive.Path()), Path: archive.Path(), Files: archive.Count()}
		inventory.Archives = append(inventory.Archives, info)
		for _, name := range archive.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				return Inventory{}, fmt.Errorf("archive %s entry %q: %w", archive.Path(), name, err)
			}
			inventory.CategoryCounts[inventoryCategory(canonical)]++
			if mapName, ok := mapNameFromResource(canonical); ok {
				addMap(mapName, "archive:"+filepath.Base(archive.Path()))
			}
		}
	}

	mapNames := make([]string, 0, len(mapSources))
	for key := range mapSources {
		mapNames = append(mapNames, key)
	}
	sort.Strings(mapNames)
	for _, name := range mapNames {
		sources := make([]string, 0, len(mapSources[name]))
		for source := range mapSources[name] {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		inventory.Maps = append(inventory.Maps, MapInfo{Name: name, Sources: sources})
	}
	sort.Slice(inventory.Packs, func(i, j int) bool { return inventory.Packs[i].Name < inventory.Packs[j].Name })
	sort.Slice(inventory.Archives, func(i, j int) bool { return inventory.Archives[i].Name < inventory.Archives[j].Name })
	inventory.Fingerprint = inventoryFingerprint(manager.Root, manager)
	return inventory, nil
}

// ListResources returns a bounded effective resource view for the advanced
// file picker. It uses the same loose-file/archive precedence as the builder.
func ListResources(dataDir, prefix string, offset, limit int) (ResourcePage, error) {
	if dataDir == "" {
		return ResourcePage{}, fmt.Errorf("mobile resource listing requires data directory")
	}
	manager, err := res.NewManager(dataDir)
	if err != nil {
		return ResourcePage{}, fmt.Errorf("resource manager: %w", err)
	}
	defer closeManager(manager)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	prefix = strings.ToLower(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(prefix)), "./"))
	sources := effectiveSources(manager)
	filtered := make([]projectSource, 0, len(sources))
	for _, source := range sources {
		if prefix != "" && !strings.HasPrefix(source.name, prefix) {
			continue
		}
		filtered = append(filtered, source)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	files := make([]ResourceInfo, 0, end-offset)
	for _, source := range filtered[offset:end] {
		files = append(files, ResourceInfo{Name: source.name, Source: source.source, SourceBytes: source.storedSize, ExpandedBytes: source.size, Category: inventoryCategory(source.name)})
	}
	return ResourcePage{Format: "goro-mobile-resource-page", Version: 1, Prefix: prefix, Offset: offset, Limit: limit, Total: len(filtered), Files: files}, nil
}

func mapNameFromResource(name string) (string, bool) {
	if !strings.HasSuffix(name, ".gnd") {
		return "", false
	}
	name = strings.TrimSuffix(name, ".gnd")
	name = strings.TrimPrefix(name, "data/")
	if name == "" || strings.Contains(name, "/../") {
		return "", false
	}
	return name, true
}

func inventoryCategory(name string) string {
	if strings.HasSuffix(name, ".gnd") || strings.HasSuffix(name, ".rsw") || strings.HasSuffix(name, ".gat") {
		return "map"
	}
	parts := strings.Split(name, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "other"
	}
	if parts[0] == "data" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}
