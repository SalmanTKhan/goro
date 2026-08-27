package mobilepack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kivutar/goro/res"
)

const (
	ProjectFormat   = "goro-mobile-asset-project"
	ProjectVersion  = 1
	ArtifactFormat  = "goro-mobile-assets"
	ArtifactVersion = 2
)

// Project is the reproducible input to the mobile asset composer. It contains
// selections and rules, never copied source data.
type Project struct {
	Format         string           `json:"format"`
	Version        int              `json:"version"`
	Source         ProjectSource    `json:"source"`
	OfflineContent string           `json:"offline_content,omitempty"`
	Base           PackSpec         `json:"base"`
	Packs          []PackSpec       `json:"packs,omitempty"`
	Profiles       []ProfileSpec    `json:"profiles"`
	Containers     ContainerSpec    `json:"containers"`
	Delivery       DeliverySpec     `json:"delivery,omitempty"`
	Optimization   OptimizationSpec `json:"optimization"`
	Budgets        BudgetSpec       `json:"budgets"`
}

type ProjectSource struct {
	Root            string `json:"root"`
	InventorySHA256 string `json:"inventory_sha256,omitempty"`
}

type PackSpec struct {
	Name         string   `json:"name"`
	Presets      []string `json:"presets,omitempty"`
	StartMap     string   `json:"start_map,omitempty"`
	Maps         []string `json:"maps,omitempty"`
	Include      []string `json:"include,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Overrides    []string `json:"overrides,omitempty"`
	Priority     int      `json:"download_priority,omitempty"`
}

type ProfileSpec struct {
	Name     string   `json:"name"`
	Packs    []string `json:"packs"`
	StartMap string   `json:"start_map,omitempty"`
}

type OptimizationSpec struct {
	Mode string `json:"mode"`
}

type DeliverySpec struct {
	Enabled        bool     `json:"enabled,omitempty"`
	Formats        []string `json:"formats,omitempty"`
	BaseURL        string   `json:"base_url,omitempty"`
	Release        string   `json:"release,omitempty"`
	SigningKey     string   `json:"signing_key,omitempty"`
	PreviousOutput string   `json:"previous_output,omitempty"`
	KeyID          string   `json:"key_id,omitempty"`
}

type ContainerSpec struct {
	EmitGRF bool    `json:"emit_grf"`
	EmitPAK bool    `json:"emit_pak"`
	PAK     PAKSpec `json:"pak"`
}

type PAKSpec struct {
	ChunkSize   int      `json:"chunk_size"`
	ZstdLevel   int      `json:"zstd_level"`
	RawSuffixes []string `json:"raw_suffixes,omitempty"`
}

type BudgetSpec struct {
	BaseArchiveBytes    int64 `json:"base_archive_bytes,omitempty"`
	ProfileArchiveBytes int64 `json:"profile_archive_bytes,omitempty"`
	TotalArchiveBytes   int64 `json:"total_archive_bytes,omitempty"`
	BasePAKBytes        int64 `json:"base_pak_bytes,omitempty"`
	ProfilePAKBytes     int64 `json:"profile_pak_bytes,omitempty"`
	TotalPAKBytes       int64 `json:"total_pak_bytes,omitempty"`
}

type ResourcePlan struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Category   string `json:"category"`
	SourceSize int64  `json:"source_size"`
	Size       int64  `json:"expanded_size"`
	Required   bool   `json:"required"`
	open       func() (io.ReadCloser, error)
}

type PackPlan struct {
	Name                  string           `json:"name"`
	Role                  string           `json:"role"`
	StartMap              string           `json:"start_map,omitempty"`
	Resources             []ResourcePlan   `json:"resources"`
	SourceBytes           int64            `json:"source_bytes"`
	ExpandedBytes         int64            `json:"expanded_bytes"`
	EstimatedArchiveBytes int64            `json:"estimated_archive_bytes"`
	EstimatedGRFBytes     int64            `json:"estimated_grf_bytes"`
	EstimatedPAKBytes     int64            `json:"estimated_pak_bytes"`
	EstimatedPAKChunks    int              `json:"estimated_pak_chunks"`
	Missing               []string         `json:"missing,omitempty"`
	Warnings              []string         `json:"warnings,omitempty"`
	Errors                []string         `json:"errors,omitempty"`
	CategoryCounts        map[string]int   `json:"category_counts"`
	CategoryBytes         map[string]int64 `json:"category_bytes"`
	Dependencies          []string         `json:"dependencies,omitempty"`
	Overrides             []string         `json:"overrides,omitempty"`
	DownloadPriority      int              `json:"download_priority,omitempty"`
}

type ProjectPlan struct {
	Format            string     `json:"format"`
	Version           int        `json:"version"`
	SourceRoot        string     `json:"source_root"`
	SourceFingerprint string     `json:"source_fingerprint"`
	Packs             []PackPlan `json:"packs"`
	Warnings          []string   `json:"warnings,omitempty"`
	Errors            []string   `json:"errors,omitempty"`
	PlannedAt         time.Time  `json:"planned_at"`
}

type ArtifactManifest struct {
	Format            string            `json:"format"`
	Version           int               `json:"version"`
	SourceRoot        string            `json:"source_root"`
	SourceFingerprint string            `json:"source_fingerprint"`
	DefaultProfile    string            `json:"default_profile"`
	PreferredFormat   string            `json:"preferred_format"`
	StartMap          string            `json:"start_map"`
	Packs             []ArtifactPack    `json:"packs"`
	Profiles          []ArtifactProfile `json:"profiles"`
	Containers        ContainerSpec     `json:"containers"`
	Optimization      OptimizationSpec  `json:"optimization"`
	Warnings          []string          `json:"warnings,omitempty"`
}

type ArtifactPack struct {
	Name          string         `json:"name"`
	Role          string         `json:"role"`
	File          string         `json:"file,omitempty"`
	SHA256        string         `json:"sha256,omitempty"`
	ArchiveBytes  int64          `json:"archive_bytes,omitempty"`
	SourceBytes   int64          `json:"source_bytes,omitempty"`
	IncludedFiles int            `json:"included_files,omitempty"`
	Files         []ArtifactFile `json:"files"`
	StartMap      string         `json:"start_map,omitempty"`
}

type ArtifactFile struct {
	Format        string `json:"format"`
	File          string `json:"file"`
	SHA256        string `json:"sha256"`
	ArchiveBytes  int64  `json:"archive_bytes"`
	SourceBytes   int64  `json:"source_bytes"`
	IncludedFiles int    `json:"included_files"`
	Chunks        int    `json:"chunks,omitempty"`
}

type ArtifactProfile struct {
	Name     string   `json:"name"`
	Packs    []string `json:"packs"`
	StartMap string   `json:"start_map,omitempty"`
}

const DeliveryManifestVersion = 1

type DeliveryManifest struct {
	Format          string         `json:"format"`
	Version         int            `json:"version"`
	Release         string         `json:"release"`
	BaseURL         string         `json:"base_url,omitempty"`
	PreferredFormat string         `json:"preferred_format"`
	PublicKey       string         `json:"public_key,omitempty"`
	KeyID           string         `json:"key_id,omitempty"`
	Packs           []DeliveryPack `json:"packs"`
	Warnings        []string       `json:"warnings,omitempty"`
}

type DeliveryPack struct {
	Name             string             `json:"name"`
	Role             string             `json:"role"`
	PreferredFormat  string             `json:"preferred_format"`
	Artifacts        []DeliveryArtifact `json:"artifacts"`
	Dependencies     []string           `json:"dependencies,omitempty"`
	Overrides        []string           `json:"overrides,omitempty"`
	DownloadPriority int                `json:"download_priority"`
}

type DeliveryArtifact struct {
	Format       string `json:"format"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	ArchiveBytes int64  `json:"archive_bytes"`
	Chunks       int    `json:"chunks,omitempty"`
}

type ProjectBuildResult struct {
	ManifestPath          string            `json:"manifest_path"`
	ValidationPath        string            `json:"validation_path"`
	DeliveryManifestPath  string            `json:"delivery_manifest_path,omitempty"`
	DeliverySignaturePath string            `json:"delivery_signature_path,omitempty"`
	Manifest              ArtifactManifest  `json:"manifest"`
	Delivery              *DeliveryManifest `json:"delivery,omitempty"`
	Plan                  ProjectPlan       `json:"plan"`
}

type projectSource struct {
	name       string
	source     string
	size       int64
	storedSize int64
	open       func() (io.ReadCloser, error)
}

type preparedProject struct {
	project Project
	plan    ProjectPlan
	manager *res.Manager
	closed  bool
}

// NewProject creates a useful starting project for the current offline
// Android runtime. Prontera is a default, not a required hard-coded choice.
func NewProject(dataDir, startMap string) (Project, error) {
	inventory, err := Inspect(dataDir)
	if err != nil {
		return Project{}, err
	}
	startMap = normalizeMapName(startMap)
	if startMap == "" {
		startMap = "prontera"
	}
	return Project{
		Format: ProjectFormat, Version: ProjectVersion,
		Source:       ProjectSource{Root: inventory.Root, InventorySHA256: inventory.Fingerprint},
		Base:         PackSpec{Name: "base", Presets: []string{"runtime-core", "starting-map"}, StartMap: startMap},
		Profiles:     []ProfileSpec{{Name: "default", Packs: []string{"base"}, StartMap: startMap}},
		Containers:   defaultContainerSpec(),
		Optimization: OptimizationSpec{Mode: "lossless"},
	}, nil
}

func LoadProject(filePath string) (Project, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Project{}, err
	}
	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return Project{}, fmt.Errorf("decode mobile asset project: %w", err)
	}
	project = normalizeProject(project)
	if err := validateProjectShape(project); err != nil {
		return Project{}, err
	}
	return project, nil
}

func SaveProject(filePath string, project Project) error {
	project = normalizeProject(project)
	if err := validateProjectShape(project); err != nil {
		return err
	}
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, append(data, '\n'), 0o644)
}

func validateProjectShape(project Project) error {
	if project.Format != ProjectFormat || project.Version != ProjectVersion {
		return fmt.Errorf("unsupported mobile asset project %q version %d", project.Format, project.Version)
	}
	if strings.TrimSpace(project.Source.Root) == "" {
		return fmt.Errorf("mobile asset project source root is empty")
	}
	if strings.TrimSpace(project.Base.Name) == "" {
		return fmt.Errorf("mobile asset project base pack name is empty")
	}
	if project.Optimization.Mode == "" {
		project.Optimization.Mode = "lossless"
	}
	if project.Optimization.Mode != "lossless" {
		return fmt.Errorf("unsupported mobile asset optimization mode %q", project.Optimization.Mode)
	}
	return nil
}

func normalizeProject(project Project) Project {
	if project.Format == "" {
		project.Format = ProjectFormat
	}
	if project.Version == 0 {
		project.Version = ProjectVersion
	}
	if project.Optimization.Mode == "" {
		project.Optimization.Mode = "lossless"
	}
	if !project.Containers.EmitGRF && !project.Containers.EmitPAK {
		project.Containers = defaultContainerSpec()
	}
	if project.Containers.PAK.ChunkSize == 0 {
		project.Containers.PAK.ChunkSize = res.DefaultPAKChunkSize
	}
	if project.Containers.PAK.ZstdLevel == 0 {
		project.Containers.PAK.ZstdLevel = res.DefaultPAKZstdLevel
	}
	return project
}

// NormalizeProject fills defaults for projects authored by older versions or
// submitted by the editor without optional container settings.
func NormalizeProject(project Project) Project { return normalizeProject(project) }

func defaultContainerSpec() ContainerSpec {
	return ContainerSpec{
		EmitGRF: true,
		EmitPAK: true,
		PAK: PAKSpec{
			ChunkSize: res.DefaultPAKChunkSize,
			ZstdLevel: res.DefaultPAKZstdLevel,
		},
	}
}

func projectPAKOptions(project Project) res.PAKOptions {
	return res.PAKOptions{
		ChunkSize:   project.Containers.PAK.ChunkSize,
		ZstdLevel:   project.Containers.PAK.ZstdLevel,
		RawSuffixes: append([]string(nil), project.Containers.PAK.RawSuffixes...),
	}
}

// PlanProject resolves all selections and calculates exact GRF and PAK
// estimates without writing an expanded source tree or final artifact.
func PlanProject(dataDir string, project Project) (ProjectPlan, error) {
	prepared, err := prepareProject(dataDir, project)
	if err != nil {
		return prepared.plan, err
	}
	closeManager(prepared.manager)
	prepared.closed = true
	return prepared.plan, planError(prepared.plan)
}

func prepareProject(dataDir string, project Project) (preparedProject, error) {
	project = normalizeProject(project)
	if project.Format == "" {
		project.Format = ProjectFormat
		project.Version = ProjectVersion
	}
	if project.Source.Root == "" {
		project.Source.Root = dataDir
	}
	if project.Optimization.Mode == "" {
		project.Optimization.Mode = "lossless"
	}
	if err := validateProjectShape(project); err != nil {
		return preparedProject{}, err
	}
	if strings.TrimSpace(dataDir) == "" {
		dataDir = project.Source.Root
	}
	manager, err := newManager(dataDir, "project-plan.grf")
	if err != nil {
		return preparedProject{}, err
	}
	plan := ProjectPlan{Format: ProjectFormat, Version: ProjectVersion, SourceRoot: manager.Root, PlannedAt: time.Now().UTC()}
	plan.SourceFingerprint = inventoryFingerprint(manager.Root, manager)
	if project.Source.InventorySHA256 != "" && project.Source.InventorySHA256 != plan.SourceFingerprint {
		plan.Warnings = append(plan.Warnings, "source inventory changed since the project was saved")
	}

	base, err := resolvePack(manager, project.Base, "base", project.Base.StartMap, nil, project.OfflineContent, projectPAKOptions(project))
	if err != nil {
		closeManager(manager)
		return preparedProject{}, err
	}
	plan.Packs = append(plan.Packs, base)
	baseNames := make(map[string]struct{}, len(base.Resources))
	for _, resource := range base.Resources {
		baseNames[resource.Name] = struct{}{}
	}
	for _, spec := range project.Packs {
		pack, err := resolvePack(manager, spec, "optional", spec.StartMap, baseNames, "", projectPAKOptions(project))
		if err != nil {
			closeManager(manager)
			return preparedProject{}, err
		}
		plan.Packs = append(plan.Packs, pack)
	}

	packNames := make(map[string]struct{}, len(plan.Packs))
	for _, pack := range plan.Packs {
		if _, exists := packNames[pack.Name]; exists {
			plan.Errors = append(plan.Errors, "duplicate pack name: "+pack.Name)
		}
		packNames[pack.Name] = struct{}{}
		plan.Warnings = append(plan.Warnings, pack.Warnings...)
		plan.Errors = append(plan.Errors, pack.Errors...)
	}
	owners := make(map[string]string)
	for _, pack := range plan.Packs {
		for _, resource := range pack.Resources {
			if owner, exists := owners[resource.Name]; exists && !hasAssetOverride(pack.Overrides, resource.Name) {
				plan.Errors = append(plan.Errors, fmt.Sprintf("resource %s is owned by both %s and %s; declare it in %s overrides", resource.Name, owner, pack.Name, pack.Name))
				continue
			}
			owners[resource.Name] = pack.Name
		}
	}
	for _, profile := range project.Profiles {
		profileGRFTotal := int64(0)
		profilePAKTotal := int64(0)
		seenProfilePacks := map[string]struct{}{}
		for _, name := range profile.Packs {
			if _, ok := packNames[name]; !ok {
				plan.Errors = append(plan.Errors, fmt.Sprintf("profile %s references unknown pack %s", profile.Name, name))
				continue
			}
			if _, ok := seenProfilePacks[name]; ok {
				continue
			}
			seenProfilePacks[name] = struct{}{}
			for _, pack := range plan.Packs {
				if pack.Name == name {
					profileGRFTotal += pack.EstimatedGRFBytes
					profilePAKTotal += pack.EstimatedPAKBytes
					break
				}
			}
		}
		if _, hasBase := seenProfilePacks[project.Base.Name]; !hasBase {
			plan.Errors = append(plan.Errors, fmt.Sprintf("profile %s does not include base pack", profile.Name))
		}
		if project.Budgets.ProfileArchiveBytes > 0 && profileGRFTotal > project.Budgets.ProfileArchiveBytes {
			plan.Errors = append(plan.Errors, fmt.Sprintf("profile %s GRF estimate %d exceeds budget %d", profile.Name, profileGRFTotal, project.Budgets.ProfileArchiveBytes))
		}
		if project.Budgets.ProfilePAKBytes > 0 && profilePAKTotal > project.Budgets.ProfilePAKBytes {
			plan.Errors = append(plan.Errors, fmt.Sprintf("profile %s PAK estimate %d exceeds budget %d", profile.Name, profilePAKTotal, project.Budgets.ProfilePAKBytes))
		}
	}
	if len(project.Profiles) == 0 {
		plan.Errors = append(plan.Errors, "project contains no profiles")
	}
	if project.Budgets.BaseArchiveBytes > 0 && base.EstimatedGRFBytes > project.Budgets.BaseArchiveBytes {
		plan.Errors = append(plan.Errors, fmt.Sprintf("base GRF estimate %d exceeds budget %d", base.EstimatedGRFBytes, project.Budgets.BaseArchiveBytes))
	}
	if project.Budgets.BasePAKBytes > 0 && base.EstimatedPAKBytes > project.Budgets.BasePAKBytes {
		plan.Errors = append(plan.Errors, fmt.Sprintf("base PAK estimate %d exceeds budget %d", base.EstimatedPAKBytes, project.Budgets.BasePAKBytes))
	}
	if project.Budgets.TotalArchiveBytes > 0 {
		total := int64(0)
		for _, pack := range plan.Packs {
			total += pack.EstimatedGRFBytes
		}
		if total > project.Budgets.TotalArchiveBytes {
			plan.Errors = append(plan.Errors, fmt.Sprintf("all packs GRF estimate %d exceeds budget %d", total, project.Budgets.TotalArchiveBytes))
		}
	}
	if project.Budgets.TotalPAKBytes > 0 {
		total := int64(0)
		for _, pack := range plan.Packs {
			total += pack.EstimatedPAKBytes
		}
		if total > project.Budgets.TotalPAKBytes {
			plan.Errors = append(plan.Errors, fmt.Sprintf("all packs PAK estimate %d exceeds budget %d", total, project.Budgets.TotalPAKBytes))
		}
	}
	return preparedProject{project: project, plan: plan, manager: manager}, nil
}

func planError(plan ProjectPlan) error {
	if len(plan.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("mobile asset project has errors: %s", strings.Join(plan.Errors, "; "))
}

func resolvePack(manager *res.Manager, spec PackSpec, role, defaultMap string, baseNames map[string]struct{}, offlineContent string, pakConfig res.PAKOptions) (PackPlan, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return PackPlan{}, fmt.Errorf("%s pack name is empty", role)
	}
	if defaultMap == "" {
		defaultMap = spec.StartMap
	}
	pack := PackPlan{Name: name, Role: role, StartMap: normalizeMapName(defaultMap), CategoryCounts: map[string]int{}, CategoryBytes: map[string]int64{}, Dependencies: append([]string(nil), spec.Dependencies...), Overrides: append([]string(nil), spec.Overrides...), DownloadPriority: spec.Priority}
	selected := map[string]ResourcePlan{}
	required := map[string]bool{}
	catalogSources := effectiveSources(manager)
	catalog := make(map[string]projectSource, len(catalogSources))
	for _, source := range catalogSources {
		catalog[source.name] = source
	}
	add := func(resource ResourcePlan) {
		resource.Name = strings.ToLower(path.Clean(strings.ReplaceAll(resource.Name, "\\", "/")))
		if resource.Name == "." || resource.Name == "" {
			return
		}
		if existing, ok := selected[resource.Name]; ok {
			existing.Required = existing.Required || resource.Required
			selected[resource.Name] = existing
			return
		}
		selected[resource.Name] = resource
	}
	addMap := func(mapName string) error {
		mapName = normalizeMapName(mapName)
		if mapName == "" {
			return nil
		}
		builder := newBuilder(manager, mapName, "map")
		if err := builder.build(); err != nil {
			return err
		}
		for name, data := range builder.files {
			name = strings.ToLower(name)
			required[name] = true
			sourceName := "map-closure:" + mapName
			sourceSize := int64(len(data))
			open := func(data []byte) func() (io.ReadCloser, error) {
				copyData := append([]byte(nil), data...)
				return func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(copyData)), nil }
			}(data)
			if source, ok := catalog[name]; ok {
				sourceName = source.source
				sourceSize = source.storedSize
				open = source.open
			}
			add(ResourcePlan{Name: name, Source: sourceName, Category: inventoryCategory(name), SourceSize: sourceSize, Size: int64(len(data)), Required: true, open: open})
		}
		pack.Missing = append(pack.Missing, builder.missing...)
		if builder.gnd.CellCount == 0 {
			pack.Warnings = append(pack.Warnings, "map closure has no parsed GND diagnostics: "+mapName)
		}
		return nil
	}

	presets := append([]string(nil), spec.Presets...)
	if len(presets) == 0 && len(spec.Maps) == 0 && len(spec.Include) == 0 {
		presets = []string{"starting-map"}
	}
	for _, preset := range presets {
		preset = strings.ToLower(strings.TrimSpace(preset))
		switch {
		case preset == "starting-map":
			if err := addMap(defaultMap); err != nil {
				return PackPlan{}, err
			}
		case preset == "map":
			for _, mapName := range spec.Maps {
				if err := addMap(mapName); err != nil {
					return PackPlan{}, err
				}
			}
		case preset == "runtime-core":
			for _, pattern := range []string{"offline/**", "mobile/**", "data/clientinfo.xml", "data/sclientinfo.xml", "clientinfo.xml", "sclientinfo.xml"} {
				addCatalogPattern(catalogSources, selected, pattern, false)
			}
		case preset == "all":
			addCatalogPattern(catalogSources, selected, "**", false)
		case strings.HasPrefix(preset, "category:"):
			category := strings.TrimPrefix(preset, "category:")
			addCatalogPattern(catalogSources, selected, category+"/**", false)
		case preset != "":
			pack.Warnings = append(pack.Warnings, "unknown preset: "+preset)
		}
	}
	for _, mapName := range spec.Maps {
		if err := addMap(mapName); err != nil {
			return PackPlan{}, err
		}
	}
	for _, pattern := range spec.Include {
		addCatalogPattern(catalogSources, selected, pattern, false)
	}
	if role == "base" && strings.TrimSpace(offlineContent) != "" {
		data, err := os.ReadFile(offlineContent)
		if err != nil {
			return PackPlan{}, fmt.Errorf("read offline content %s: %w", offlineContent, err)
		}
		dataCopy := append([]byte(nil), data...)
		selected["offline/content.json"] = ResourcePlan{
			Name: "offline/content.json", Source: "offline-content:" + offlineContent,
			Category: "offline", SourceSize: int64(len(data)), Size: int64(len(data)), Required: true,
			open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(dataCopy)), nil },
		}
		// Keep the modular asset project in lockstep with the direct map
		// builder: offline actors introduce sprite dependencies beyond terrain.
		npcBuilder := newBuilder(manager, "offline-npcs", "map")
		npcBuilder.includeOfflineNPCResources(offlineContent)
		npcBuilder.includeOfflineMonsterResources(offlineContent)
		npcBuilder.includeOfflineItemResources(offlineContent)
		npcBuilder.includeOfflineCombatFeedbackResources()
		npcBuilder.includeOfflineAudioResources()
		pack.Missing = append(pack.Missing, npcBuilder.missing...)
		for name, data := range npcBuilder.files {
			name = strings.ToLower(name)
			required[name] = true
			sourceName := "offline-actor-closure"
			sourceSize := int64(len(data))
			open := func(data []byte) func() (io.ReadCloser, error) {
				copyData := append([]byte(nil), data...)
				return func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(copyData)), nil }
			}(data)
			if source, ok := catalog[name]; ok {
				sourceName = source.source
				sourceSize = source.storedSize
				open = source.open
			}
			add(ResourcePlan{Name: name, Source: sourceName, Category: inventoryCategory(name), SourceSize: sourceSize, Size: int64(len(data)), Required: true, open: open})
		}
	}
	for _, pattern := range spec.Exclude {
		for name, resource := range selected {
			if matchAssetPattern(pattern, name) {
				if required[name] || resource.Required {
					pack.Errors = append(pack.Errors, fmt.Sprintf("cannot exclude required resource %s", name))
					continue
				}
				delete(selected, name)
			}
		}
	}
	if baseNames != nil {
		for name := range baseNames {
			if !hasAssetOverride(spec.Overrides, name) {
				delete(selected, name)
			}
		}
	}
	if role == "base" && strings.Contains(strings.Join(presets, ","), "runtime-core") {
		if _, ok := selected["offline/content.json"]; !ok {
			pack.Warnings = append(pack.Warnings, "base runtime-core has no offline/content.json; offline Android startup will require --offline-content")
		}
	}
	for _, resource := range selected {
		pack.Resources = append(pack.Resources, resource)
	}
	sort.Slice(pack.Resources, func(i, j int) bool { return pack.Resources[i].Name < pack.Resources[j].Name })
	for _, resource := range pack.Resources {
		pack.SourceBytes += resource.SourceSize
		pack.ExpandedBytes += resource.Size
		pack.CategoryCounts[resource.Category]++
		pack.CategoryBytes[resource.Category] += resource.Size
	}
	sources := make([]res.GRFPackSource, 0, len(pack.Resources))
	for _, resource := range pack.Resources {
		sources = append(sources, res.GRFPackSource{Name: resource.Name, Size: resource.Size, Open: resource.open})
	}
	estimate, err := res.EstimateGRFEntries(sources)
	if err != nil {
		return PackPlan{}, fmt.Errorf("estimate pack %s: %w", pack.Name, err)
	}
	pack.EstimatedArchiveBytes = estimate.ArchiveBytes
	pack.EstimatedGRFBytes = estimate.ArchiveBytes
	pakSources := make([]res.PAKPackSource, 0, len(pack.Resources))
	for _, resource := range pack.Resources {
		pakSources = append(pakSources, res.PAKPackSource{Name: resource.Name, Size: resource.Size, Open: resource.open})
	}
	pakEstimate, err := res.EstimatePAKEntries(pakSources, pakConfig)
	if err != nil {
		return PackPlan{}, fmt.Errorf("estimate PAK pack %s: %w", pack.Name, err)
	}
	pack.EstimatedPAKBytes = pakEstimate.ArchiveBytes
	pack.EstimatedPAKChunks = pakEstimate.Chunks
	return pack, nil
}

func addCatalogPattern(sources []projectSource, selected map[string]ResourcePlan, pattern string, required bool) {
	for _, source := range sources {
		if !matchAssetPattern(pattern, source.name) {
			continue
		}
		resource := ResourcePlan{Name: source.name, Source: source.source, Category: inventoryCategory(source.name), SourceSize: source.storedSize, Size: source.size, Required: required, open: source.open}
		if existing, ok := selected[source.name]; ok {
			resource.Required = existing.Required || resource.Required
		}
		selected[source.name] = resource
	}
}

func effectiveSources(manager *res.Manager) []projectSource {
	byName := map[string]projectSource{}
	_ = filepath.WalkDir(manager.Root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !entry.Type().IsRegular() {
			return walkErr
		}
		relative, err := filepath.Rel(manager.Root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if !isCompleteLooseResource(name) || !isAllowedCompletePath(name) {
			return nil
		}
		canonical, err := canonicalPackPath(name)
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		filePathCopy := filePath
		byName[canonical] = projectSource{name: canonical, source: "loose:" + name, size: info.Size(), storedSize: info.Size(), open: func() (io.ReadCloser, error) { return os.Open(filePathCopy) }}
		return nil
	})
	for _, pack := range manager.Packs {
		for _, name := range pack.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				continue
			}
			if _, exists := byName[canonical]; exists {
				continue
			}
			packCopy := pack
			nameCopy := name
			entrySize := int64(0)
			storedSize := int64(0)
			if entry, ok := pack.Entry(name); ok {
				entrySize = int64(entry.UncompressedSize)
				for _, chunk := range entry.Chunks {
					storedSize += int64(chunk.CompressedSize)
				}
			}
			byName[canonical] = projectSource{name: canonical, source: "pak:" + filepath.Base(pack.Path()), size: entrySize, storedSize: storedSize, open: func() (io.ReadCloser, error) {
				reader, _, err := packCopy.OpenFile(nameCopy)
				return reader, err
			}}
		}
	}
	for _, archive := range manager.Archives {
		for _, name := range archive.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				continue
			}
			if _, exists := byName[canonical]; exists {
				continue
			}
			archiveCopy := archive
			nameCopy := name
			entrySize := int64(0)
			if entry, ok := archive.Entry(name); ok {
				entrySize = int64(entry.RealSize)
			}
			storedSize := entrySize
			if entry, ok := archive.Entry(name); ok {
				storedSize = int64(entry.PackedSize)
			}
			byName[canonical] = projectSource{name: canonical, source: "archive:" + filepath.Base(archive.Path()), size: entrySize, storedSize: storedSize, open: func() (io.ReadCloser, error) {
				reader, _, err := archiveCopy.OpenFile(nameCopy)
				return reader, err
			}}
		}
	}
	result := make([]projectSource, 0, len(byName))
	for _, source := range byName {
		result = append(result, source)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func isAllowedCompletePath(name string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(name)), "/")
	return len(parts) == 0 || func() bool {
		_, excluded := completeExcludedDirectories[parts[0]]
		return !excluded
	}()
}

func matchAssetPattern(pattern, name string) bool {
	pattern = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/")))
	name = strings.ToLower(strings.TrimPrefix(path.Clean(strings.ReplaceAll(name, "\\", "/")), "./"))
	if pattern == "" {
		return false
	}
	if pattern == "**" || pattern == name {
		return true
	}
	pattern = strings.TrimPrefix(path.Clean(pattern), "./")
	pattern = strings.TrimSuffix(pattern, "/")
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return name == prefix || strings.HasPrefix(name, prefix+"/")
	}
	matched, err := path.Match(pattern, name)
	return err == nil && matched
}

func hasAssetOverride(overrides []string, name string) bool {
	for _, override := range overrides {
		if matchAssetPattern(override, name) {
			return true
		}
	}
	return false
}

func normalizeMapName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".gnd"), ".rsw")
	return strings.TrimSuffix(name, ".gat")
}

type DeliveryBuildOptions struct {
	DeliveryEnabled        bool
	DeliveryFormats        []res.DeliveryFormat
	DeliveryBaseURL        string
	DeliveryRelease        string
	DeliverySigningKey     string
	DeliveryPreviousOutput string
}

// BuildProject creates all packs and the Android-consumable artifact manifest.
// Delivery metadata is opt-in here so existing local/editor builds remain
// byte-for-byte compatible; production builds can enable it through the
// project or BuildProjectWithOptions.
func BuildProject(dataDir string, project Project, outputDir string) (ProjectBuildResult, error) {
	return BuildProjectWithOptions(dataDir, project, outputDir, DeliveryBuildOptions{})
}

func BuildProjectWithOptions(dataDir string, project Project, outputDir string, options DeliveryBuildOptions) (ProjectBuildResult, error) {
	prepared, err := prepareProject(dataDir, project)
	if err != nil {
		return ProjectBuildResult{}, err
	}
	defer closeManager(prepared.manager)
	if err := planError(prepared.plan); err != nil {
		return ProjectBuildResult{Plan: prepared.plan}, err
	}
	project = prepared.project
	if err := os.MkdirAll(filepath.Join(outputDir, "packs"), 0o755); err != nil {
		return ProjectBuildResult{}, err
	}
	preferredFormat := "grf"
	if project.Containers.EmitPAK {
		preferredFormat = "pak"
	}
	manifest := ArtifactManifest{Format: ArtifactFormat, Version: ArtifactVersion, SourceRoot: prepared.plan.SourceRoot, SourceFingerprint: prepared.plan.SourceFingerprint, PreferredFormat: preferredFormat, Containers: project.Containers, Optimization: project.Optimization}
	for index, pack := range prepared.plan.Packs {
		artifact := ArtifactPack{Name: pack.Name, Role: pack.Role, SourceBytes: pack.SourceBytes, IncludedFiles: len(pack.Resources), StartMap: pack.StartMap}
		if project.Containers.EmitPAK {
			fileName := fmt.Sprintf("%02d-%s.pak", index, safePackName(pack.Name))
			relative := filepath.ToSlash(filepath.Join("packs", fileName))
			outputPath := filepath.Join(outputDir, filepath.FromSlash(relative))
			stats, err := packPAKAtomic(outputPath, pakSources(pack), projectPAKOptions(project))
			if err != nil {
				return ProjectBuildResult{}, fmt.Errorf("build PAK pack %s: %w", pack.Name, err)
			}
			if err := validateArtifactPAK(outputPath, pack); err != nil {
				return ProjectBuildResult{}, fmt.Errorf("validate PAK pack %s: %w", pack.Name, err)
			}
			hash, archiveBytes, err := hashFile(outputPath)
			if err != nil {
				return ProjectBuildResult{}, err
			}
			artifact.Files = append(artifact.Files, ArtifactFile{Format: "pak", File: relative, SHA256: hash, ArchiveBytes: archiveBytes, SourceBytes: pack.SourceBytes, IncludedFiles: stats.Files, Chunks: stats.Chunks})
		}
		if project.Containers.EmitGRF {
			fileName := fmt.Sprintf("%02d-%s.grf", index, safePackName(pack.Name))
			relative := filepath.ToSlash(filepath.Join("packs", fileName))
			outputPath := filepath.Join(outputDir, filepath.FromSlash(relative))
			stats, err := packGRFAtomic(outputPath, grfSources(pack))
			if err != nil {
				return ProjectBuildResult{}, fmt.Errorf("build GRF pack %s: %w", pack.Name, err)
			}
			if err := validateArtifactPack(outputPath, pack); err != nil {
				return ProjectBuildResult{}, fmt.Errorf("validate GRF pack %s: %w", pack.Name, err)
			}
			hash, archiveBytes, err := hashFile(outputPath)
			if err != nil {
				return ProjectBuildResult{}, err
			}
			artifact.Files = append(artifact.Files, ArtifactFile{Format: "grf", File: relative, SHA256: hash, ArchiveBytes: archiveBytes, SourceBytes: pack.SourceBytes, IncludedFiles: stats.Files})
			artifact.File, artifact.SHA256, artifact.ArchiveBytes = relative, hash, archiveBytes
		}
		manifest.Packs = append(manifest.Packs, artifact)
	}
	for _, profile := range project.Profiles {
		startMap := profile.StartMap
		if startMap == "" {
			startMap = project.Base.StartMap
		}
		manifest.Profiles = append(manifest.Profiles, ArtifactProfile{Name: profile.Name, Packs: append([]string(nil), profile.Packs...), StartMap: normalizeMapName(startMap)})
	}
	if len(manifest.Profiles) > 0 {
		manifest.DefaultProfile = manifest.Profiles[0].Name
		manifest.StartMap = manifest.Profiles[0].StartMap
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ProjectBuildResult{}, err
	}
	manifestPath := filepath.Join(outputDir, "mobile-assets.json")
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o644); err != nil {
		return ProjectBuildResult{}, err
	}
	validation := prepared.plan
	validationData, err := json.MarshalIndent(validation, "", "  ")
	if err != nil {
		return ProjectBuildResult{}, err
	}
	validationPath := filepath.Join(outputDir, "validation.json")
	if err := os.WriteFile(validationPath, append(validationData, '\n'), 0o644); err != nil {
		return ProjectBuildResult{}, err
	}
	result := ProjectBuildResult{ManifestPath: manifestPath, ValidationPath: validationPath, Manifest: manifest, Plan: prepared.plan}
	delivery, deliveryPath, signaturePath, err := buildDelivery(outputDir, project, prepared.plan, manifest, options)
	if err != nil {
		return ProjectBuildResult{}, err
	}
	result.Delivery = delivery
	result.DeliveryManifestPath = deliveryPath
	result.DeliverySignaturePath = signaturePath
	return result, nil
}

func validateArtifactPack(filePath string, plan PackPlan) error {
	archive, err := res.OpenGRF(filePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if archive.Count() != len(plan.Resources) {
		return fmt.Errorf("entry count %d does not match plan %d", archive.Count(), len(plan.Resources))
	}
	for _, resource := range plan.Resources {
		data, err := archive.ReadFile(resource.Name)
		if err != nil {
			return fmt.Errorf("read %s: %w", resource.Name, err)
		}
		if int64(len(data)) != resource.Size {
			return fmt.Errorf("resource %s size %d does not match plan %d", resource.Name, len(data), resource.Size)
		}
	}
	return nil
}

func validateArtifactPAK(filePath string, plan PackPlan) error {
	archive, err := res.OpenPAK(filePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if archive.Count() != len(plan.Resources) {
		return fmt.Errorf("entry count %d does not match plan %d", archive.Count(), len(plan.Resources))
	}
	for _, resource := range plan.Resources {
		data, err := archive.ReadFile(resource.Name)
		if err != nil {
			return fmt.Errorf("read %s: %w", resource.Name, err)
		}
		if int64(len(data)) != resource.Size {
			return fmt.Errorf("resource %s size %d does not match plan %d", resource.Name, len(data), resource.Size)
		}
		entry, ok := archive.Entry(resource.Name)
		if !ok {
			return fmt.Errorf("resource %s is missing from index", resource.Name)
		}
		if sha256.Sum256(data) != entry.SHA256 {
			return fmt.Errorf("resource %s hash mismatch", resource.Name)
		}
	}
	return nil
}

func grfSources(pack PackPlan) []res.GRFPackSource {
	sources := make([]res.GRFPackSource, 0, len(pack.Resources))
	for _, resource := range pack.Resources {
		sources = append(sources, res.GRFPackSource{Name: resource.Name, Size: resource.Size, Open: resource.open})
	}
	return sources
}

func pakSources(pack PackPlan) []res.PAKPackSource {
	sources := make([]res.PAKPackSource, 0, len(pack.Resources))
	for _, resource := range pack.Resources {
		sources = append(sources, res.PAKPackSource{Name: resource.Name, Size: resource.Size, Open: resource.open})
	}
	return sources
}

func packPAKAtomic(outputPath string, sources []res.PAKPackSource, options res.PAKOptions) (res.PAKPackStats, error) {
	temp, err := os.CreateTemp(filepath.Dir(outputPath), ".mobile-pack-*.tmp")
	if err != nil {
		return res.PAKPackStats{}, err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return res.PAKPackStats{}, err
	}
	defer os.Remove(tempPath)
	stats, err := res.PackPAKEntries(tempPath, sources, options)
	if err != nil {
		return res.PAKPackStats{}, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return res.PAKPackStats{}, err
	}
	return stats, nil
}

func hashFile(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	bytesWritten, err := io.Copy(digest, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), bytesWritten, nil
}

func packGRFAtomic(outputPath string, sources []res.GRFPackSource) (res.GRFPackStats, error) {
	temp, err := os.CreateTemp(filepath.Dir(outputPath), ".mobile-pack-*.tmp")
	if err != nil {
		return res.GRFPackStats{}, err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return res.GRFPackStats{}, err
	}
	defer os.Remove(tempPath)
	stats, err := res.PackGRFEntries(tempPath, sources)
	if err != nil {
		return res.GRFPackStats{}, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return res.GRFPackStats{}, err
	}
	return stats, nil
}

func safePackName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out strings.Builder
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			out.WriteRune(char)
		} else {
			out.WriteByte('-')
		}
	}
	if out.Len() == 0 {
		return "pack"
	}
	return strings.Trim(out.String(), "-")
}

func inventoryFingerprint(root string, manager *res.Manager) string {
	parts := []string{strings.ToLower(filepath.Clean(root))}
	_ = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !entry.Type().IsRegular() {
			return walkErr
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if !isAllowedCompletePath(name) || !isCompleteLooseResource(name) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("loose:%s:%d:%d", strings.ToLower(name), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	for _, pack := range manager.Packs {
		info, _ := os.Stat(pack.Path())
		stamp := int64(0)
		size := int64(0)
		if info != nil {
			stamp, size = info.ModTime().UnixNano(), info.Size()
		}
		parts = append(parts, fmt.Sprintf("pack:%s:%d:%d", strings.ToLower(filepath.Base(pack.Path())), size, stamp))
		for _, name := range pack.Names() {
			parts = append(parts, "pack-entry:"+strings.ToLower(filepath.Base(pack.Path()))+":"+name)
		}
	}
	for _, archive := range manager.Archives {
		info, _ := os.Stat(archive.Path())
		stamp := int64(0)
		size := int64(0)
		if info != nil {
			stamp, size = info.ModTime().UnixNano(), info.Size()
		}
		parts = append(parts, fmt.Sprintf("archive:%s:%d:%d", strings.ToLower(filepath.Base(archive.Path())), size, stamp))
		for _, name := range archive.Names() {
			parts = append(parts, "entry:"+strings.ToLower(filepath.Base(archive.Path()))+":"+name)
		}
	}
	sort.Strings(parts)
	hash := sha256.New()
	for _, part := range parts {
		_, _ = io.WriteString(hash, part+"\n")
	}
	return hex.EncodeToString(hash.Sum(nil))
}
