// Package mobilepack builds deterministic, renderer-loadable mobile resource
// packs from the same GRF closure used by the production resource manager.
// v1 optimizes deployment closure and compression; GPU mesh/texture baking is
// intentionally a later format version.
package mobilepack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/res"
)

type Result struct {
	Root                   string
	Scope                  string
	Maps                   []string
	Output                 string
	IncludedFiles          int
	SourceBytes            int64
	ArchiveBytes           int64
	SHA256                 string
	Missing                []string
	CategoryCounts         map[string]int
	CategoryBytes          map[string]int64
	OriginalDataRoot       string
	GND                    res.GNDDiagnostics
	TextureMetrics         []TextureOptimizationMetric
	TerrainBake            *TerrainBakeInfo
	OfflineContentIncluded bool
	OfflineContentSHA256   string
}

type Manifest struct {
	Format              string                      `json:"format"`
	Version             int                         `json:"version"`
	Scope               string                      `json:"scope"`
	Map                 string                      `json:"map"`
	Maps                []string                    `json:"maps,omitempty"`
	Pack                string                      `json:"pack"`
	OriginalDataRoot    string                      `json:"original_data_root"`
	OriginalGRFPattern  string                      `json:"original_grf_reference"`
	IncludedFiles       int                         `json:"included_files"`
	SourceBytes         int64                       `json:"source_bytes"`
	ArchiveBytes        int64                       `json:"archive_bytes"`
	SHA256              string                      `json:"sha256"`
	MissingDependencies []string                    `json:"missing_dependencies,omitempty"`
	CategoryCounts      map[string]int              `json:"category_counts"`
	CategoryBytes       map[string]int64            `json:"category_bytes"`
	GNDDiagnostics      res.GNDDiagnostics          `json:"gnd_diagnostics"`
	TextureMetrics      []TextureOptimizationMetric `json:"texture_optimization,omitempty"`
	RuntimeMetrics      *RuntimeMetrics             `json:"runtime_metrics,omitempty"`
	TerrainBake         *TerrainBakeInfo            `json:"terrain_bake,omitempty"`
	RuntimeNotes        []string                    `json:"runtime_notes"`
	OfflineContent      string                      `json:"offline_content,omitempty"`
}

type builder struct {
	manager          *res.Manager
	root             string
	files            map[string][]byte
	included         map[string]struct{}
	categories       map[string]string
	missing          []string
	counts           map[string]int
	bytes            map[string]int64
	gnd              res.GNDDiagnostics
	stage            string
	scope            string
	maps             []string
	textureMetrics   []TextureOptimizationMetric
	optimizeTextures bool
	bakeTerrain      bool
	parsedGND        *res.GND
	terrainBake      *TerrainBakeInfo
}

func Build(dataDir, mapName, out string) (Result, error) {
	return BuildWithOptions(dataDir, mapName, out, BuildOptions{})
}

// BuildWithOptions builds a map closure and optionally adds the deterministic
// optimized-texture sidecar. The source texture files are always retained.
func BuildWithOptions(dataDir, mapName, out string, options BuildOptions) (Result, error) {
	manager, err := newManager(dataDir, out)
	if err != nil {
		return Result{}, err
	}
	defer closeManager(manager)

	root := strings.TrimSuffix(strings.TrimSuffix(mapName, ".gnd"), ".rsw")
	b := newBuilder(manager, root, "map")
	b.optimizeTextures = options.OptimizeTextures
	b.bakeTerrain = options.BakeTerrain
	b.maps = []string{root}
	if err := b.build(); err != nil {
		return Result{}, err
	}
	if b.optimizeTextures {
		if err := b.optimizeTextureFiles(); err != nil {
			return Result{}, err
		}
	}
	if b.bakeTerrain {
		if err := b.bakeTerrainFile(); err != nil {
			return Result{}, err
		}
	}
	if options.OfflineContent != "" {
		if err := b.includeOfflineContent(options.OfflineContent); err != nil {
			return Result{}, err
		}
		b.includeOfflineNPCResources(options.OfflineContent)
		b.includeOfflineMonsterResources(options.OfflineContent)
		b.includeOfflineItemResources(options.OfflineContent)
		b.includeOfflineCombatFeedbackResources()
		b.includeOfflineAudioResources()
	}
	return b.pack(out)
}

// BuildComplete creates a deterministic pack containing the complete resource
// view visible to res.Manager: loose resource files plus every entry from the
// top-level GRF/GPF archives. Duplicate logical paths retain the same
// first-match precedence as res.Manager.
func BuildComplete(dataDir, out string) (Result, error) {
	return BuildCompleteWithOptions(dataDir, out, BuildOptions{})
}

func BuildCompleteWithOptions(dataDir, out string, options BuildOptions) (Result, error) {
	manager, err := newManager(dataDir, out)
	if err != nil {
		return Result{}, err
	}
	defer closeManager(manager)

	b := newBuilder(manager, "all", "complete")
	stage, err := createStage(out)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)
	b.stage = stage
	if err := b.buildComplete(); err != nil {
		return Result{}, err
	}
	if options.OfflineContent != "" {
		if err := b.includeOfflineContent(options.OfflineContent); err != nil {
			return Result{}, err
		}
		b.includeOfflineNPCResources(options.OfflineContent)
		b.includeOfflineMonsterResources(options.OfflineContent)
		b.includeOfflineItemResources(options.OfflineContent)
		b.includeOfflineCombatFeedbackResources()
		b.includeOfflineAudioResources()
	}
	return b.pack(out)
}

// BuildMaps creates one deterministic pack from the union of several map
// closures. Each map is resolved using the same dependency rules as Build.
func BuildMaps(dataDir string, mapNames []string, out string) (Result, error) {
	return BuildMapsWithOptions(dataDir, mapNames, out, BuildOptions{})
}

func BuildMapsWithOptions(dataDir string, mapNames []string, out string, options BuildOptions) (Result, error) {
	manager, err := newManager(dataDir, out)
	if err != nil {
		return Result{}, err
	}
	defer closeManager(manager)

	b := newBuilder(manager, "selection", "selection")
	seen := make(map[string]struct{}, len(mapNames))
	for _, mapName := range mapNames {
		root := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(mapName), ".gnd"), ".rsw")
		if root == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(root)]; ok {
			continue
		}
		seen[strings.ToLower(root)] = struct{}{}
		b.root = root
		b.maps = append(b.maps, root)
		if err := b.build(); err != nil {
			return Result{}, err
		}
	}
	if len(b.maps) == 0 {
		return Result{}, fmt.Errorf("mobile pack selection contains no maps")
	}
	b.root = "selection"
	if options.OfflineContent != "" {
		if err := b.includeOfflineContent(options.OfflineContent); err != nil {
			return Result{}, err
		}
		b.includeOfflineNPCResources(options.OfflineContent)
		b.includeOfflineMonsterResources(options.OfflineContent)
		b.includeOfflineItemResources(options.OfflineContent)
		b.includeOfflineCombatFeedbackResources()
		b.includeOfflineAudioResources()
	}
	return b.pack(out)
}

func newManager(dataDir, out string) (*res.Manager, error) {
	if dataDir == "" || out == "" {
		return nil, fmt.Errorf("mobile pack requires data directory and output path")
	}
	manager, err := res.NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resource manager: %w", err)
	}
	return manager, nil
}

func closeManager(manager *res.Manager) {
	if manager == nil {
		return
	}
	_ = manager.Close()
}

func newBuilder(manager *res.Manager, root, scope string) *builder {
	return &builder{
		manager:    manager,
		root:       root,
		files:      map[string][]byte{},
		included:   map[string]struct{}{},
		categories: map[string]string{},
		counts:     map[string]int{},
		bytes:      map[string]int64{},
		scope:      scope,
	}
}

func (b *builder) pack(out string) (Result, error) {
	stage := b.stage
	if stage == "" {
		var err error
		stage, err = createStage(out)
		if err != nil {
			return Result{}, err
		}
		defer os.RemoveAll(stage)
		for name, data := range b.files {
			if err := writeStageFile(stage, name, data); err != nil {
				return Result{}, err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return Result{}, err
	}
	stats, err := res.PackGRF(out, stage)
	if err != nil {
		return Result{}, err
	}
	archive, err := os.ReadFile(out)
	if err != nil {
		return Result{}, err
	}
	hash := sha256.Sum256(archive)
	missing := append([]string(nil), b.missing...)
	sort.Strings(missing)
	contentHash := ""
	contentIncluded := false
	if content, ok := b.files["offline/content.json"]; ok {
		digest := sha256.Sum256(content)
		contentHash = hex.EncodeToString(digest[:])
		contentIncluded = true
	}
	return Result{Root: b.root, Scope: b.scope, Maps: append([]string(nil), b.maps...), Output: out, IncludedFiles: stats.Files, SourceBytes: stats.Bytes, ArchiveBytes: int64(len(archive)), SHA256: hex.EncodeToString(hash[:]), Missing: missing, CategoryCounts: b.counts, CategoryBytes: b.bytes, OriginalDataRoot: b.manager.Root, GND: b.gnd, TextureMetrics: append([]TextureOptimizationMetric(nil), b.textureMetrics...), TerrainBake: b.terrainBake, OfflineContentIncluded: contentIncluded, OfflineContentSHA256: contentHash}, nil
}

func createStage(out string) (string, error) {
	directory := filepath.Dir(out)
	if directory == "" {
		directory = "."
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(directory, ".goro-mobile-pack-")
}

func WriteManifest(path string, result Result) error {
	missing := append([]string(nil), result.Missing...)
	manifest := Manifest{
		Format: "goro-mobile-pack", Version: 3, Scope: result.Scope, Map: result.Root, Maps: append([]string(nil), result.Maps...), Pack: filepath.Base(result.Output),
		OriginalDataRoot: result.OriginalDataRoot, OriginalGRFPattern: originalReference(result.Scope),
		IncludedFiles: result.IncludedFiles, SourceBytes: result.SourceBytes, ArchiveBytes: result.ArchiveBytes,
		SHA256: result.SHA256, MissingDependencies: missing, CategoryCounts: result.CategoryCounts, CategoryBytes: result.CategoryBytes,
		GNDDiagnostics: result.GND,
		TextureMetrics: result.TextureMetrics,
		TerrainBake:    result.TerrainBake,
		OfflineContent: func() string {
			if result.OfflineContentIncluded {
				return result.OfflineContentSHA256
			}
			return ""
		}(),
		RuntimeNotes: runtimeNotes(result.Scope),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func originalReference(scope string) string {
	if scope == "complete" {
		return "all loose resource files + all entries from top-level .grf/.gpf archives"
	}
	return "data/<map>.gnd + data/<map>.rsw + data/<map>.gat"
}

func runtimeNotes(scope string) []string {
	notes := []string{"v3 is a deterministic GRF pack; runtime loading is compatible with the existing GRF loader.", "GPU mesh baking and ASTC/ETC2 transcoding are measured separately before a future format revision."}
	if scope == "complete" {
		notes = append(notes, "complete scope preserves the effective res.Manager resource precedence and omits client executables, libraries, patch containers, and runtime state directories.")
	}
	return notes
}

func (b *builder) build() error {
	for _, ext := range []string{"gnd", "rsw", "gat"} {
		name := b.root + "." + ext
		if _, err := b.include("data/"+name, []string{"data/" + name, name}); err != nil {
			b.missing = append(b.missing, name)
			if ext == "gnd" {
				return fmt.Errorf("map root missing: %s", name)
			}
		}
	}
	if data := b.files["data/"+b.root+".gnd"]; data != nil {
		gnd, err := res.ParseGND(data)
		if err != nil {
			return fmt.Errorf("parse GND: %w", err)
		}
		b.parsedGND = gnd
		b.gnd = res.AnalyzeGND(gnd, b.manager)
		for _, name := range gnd.Textures {
			b.includeTexture(name)
		}
	}
	if data := b.files["data/"+b.root+".rsw"]; data != nil {
		rsw, err := res.ParseRSW(data)
		if err != nil {
			return fmt.Errorf("parse RSW: %w", err)
		}
		for _, model := range rsw.Models {
			modelName := strings.TrimSpace(model.Filename)
			if modelName == "" {
				continue
			}
			modelData, err := b.include(res.NormalizeResourcePath("data/model/"+modelName), res.RSMModelCandidates(modelName))
			if err != nil {
				b.missing = append(b.missing, modelName)
				continue
			}
			rsm, err := res.ParseRSM(modelData)
			if err != nil {
				b.missing = append(b.missing, modelName+" (parse)")
				continue
			}
			for _, texture := range rsm.Textures {
				b.includeTexture(texture)
			}
		}
	}
	b.includeOfflineActorFixtures()
	return nil
}

func (b *builder) buildComplete() error {
	if err := b.includeCompleteLooseFiles(); err != nil {
		return err
	}
	for _, pack := range b.manager.Packs {
		for _, name := range pack.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				return fmt.Errorf("pack %s entry %q: %w", pack.Path(), name, err)
			}
			if _, ok := b.included[canonical]; ok {
				continue
			}
			data, err := pack.ReadFile(name)
			if err != nil {
				return fmt.Errorf("read %s from %s: %w", name, pack.Path(), err)
			}
			if err := b.includeCompleteData(canonical, data); err != nil {
				return fmt.Errorf("stage %s from %s: %w", name, pack.Path(), err)
			}
		}
	}
	for _, archive := range b.manager.Archives {
		for _, name := range archive.Names() {
			canonical, err := canonicalPackPath(name)
			if err != nil {
				return fmt.Errorf("archive %s entry %q: %w", archive.Path(), name, err)
			}
			if _, ok := b.included[canonical]; ok {
				continue
			}
			data, err := archive.ReadFile(name)
			if err != nil {
				return fmt.Errorf("read %s from %s: %w", name, archive.Path(), err)
			}
			if err := b.includeCompleteData(canonical, data); err != nil {
				return fmt.Errorf("stage %s from %s: %w", name, archive.Path(), err)
			}
		}
	}
	return nil
}

var completeExcludedDirectories = map[string]struct{}{
	"cache":          {},
	"crash-dumps":    {},
	"dlls":           {},
	"extracted data": {},
	"licenses":       {},
	"log":            {},
	"patchclient":    {},
	"recordings":     {},
	"screenshots":    {},
	"thirdparty":     {},
}

func (b *builder) includeCompleteLooseFiles() error {
	return filepath.WalkDir(b.manager.Root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(b.manager.Root, filePath)
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
		relative, err := filepath.Rel(b.manager.Root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if !isCompleteLooseResource(name) {
			return nil
		}
		canonical, err := canonicalPackPath(name)
		if err != nil {
			return fmt.Errorf("loose file %q: %w", name, err)
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		return b.includeCompleteData(canonical, data)
	})
}

func isCompleteLooseResource(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".grf", ".gpf", ".rgz", ".thor", ".exe", ".dll", ".asi", ".m3d", ".log", ".dmp":
		return false
	}
	return true
}

func canonicalPackPath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimLeft(name, "/")
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || (len(clean) > 1 && clean[1] == ':') {
		return "", fmt.Errorf("unsafe resource path")
	}
	return strings.ToLower(clean), nil
}

func (b *builder) includeCompleteData(canonical string, data []byte) error {
	if _, ok := b.included[canonical]; ok {
		return nil
	}
	b.included[canonical] = struct{}{}
	b.recordCategory(canonical, int64(len(data)))
	return writeStageFile(b.stage, canonical, data)
}

func writeStageFile(stage, name string, data []byte) error {
	filePath := filepath.Join(stage, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0o644)
}

func (b *builder) includeOfflineActorFixtures() {
	for _, extension := range []string{"act", "spr"} {
		bodyCandidates := res.PlayerBodyResourceCandidates(0, 0, extension)
		if _, err := b.include(slash(bodyCandidates[0]), bodyCandidates); err != nil {
			b.missing = append(b.missing, "offline player body "+extension)
		}
		headCandidates := res.PlayerHeadResourceCandidates(0, 1, 0, extension)
		if _, err := b.include(slash(headCandidates[0]), headCandidates); err != nil {
			b.missing = append(b.missing, "offline player head "+extension)
		}
	}
	imfCandidates := res.PlayerIMFResourceCandidates(0, 0)
	if _, err := b.include(slash(imfCandidates[0]), imfCandidates); err != nil {
		b.missing = append(b.missing, "offline player imf")
	}
	for _, extension := range []string{"act", "spr"} {
		b.include("data/sprite/shadow."+extension, []string{"data\\sprite\\shadow." + extension, "data/sprite/shadow." + extension})
	}
	resourceName, ok := b.manager.NonPCResourceName(1002)
	if !ok {
		b.missing = append(b.missing, "offline actor resource name 1002")
		return
	}
	for _, extension := range []string{"act", "spr"} {
		candidates := res.NonPCSpriteResourceCandidates(1002, resourceName, extension)
		if _, err := b.include(slash(candidates[0]), candidates); err != nil {
			b.missing = append(b.missing, "offline actor "+extension)
		}
	}
}

// includeOfflineNPCResources closes the resource dependency introduced by the
// generated offline content. The map closure can render terrain and the
// starter player without this step, but NPC actors need both the identity
// tables and the sprite pair selected by each NPC's actual sprite/job ID.
// Keep this data-driven: do not pack every NPC sprite when a fixture only
// references a small deterministic map slice.
func (b *builder) includeOfflineNPCResources(sourcePath string) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return
	}
	var content struct {
		Maps map[string]struct {
			NPCs []struct {
				Sprite int16 `json:"sprite"`
			} `json:"npcs"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return
	}

	for _, candidates := range res.NonPCResourceNameTableCandidates() {
		if len(candidates) == 0 {
			continue
		}
		// The generated DB already covers the legacy NPC IDs used by the
		// current content. Preserve a table when the source client provides
		// one, but do not report it as a missing dependency: older clients
		// legitimately rely on the compiled fallback table.
		_, _ = b.include(slash(candidates[0]), candidates)
	}

	jobs := make(map[int]struct{})
	for _, world := range content.Maps {
		for _, npc := range world.NPCs {
			if npc.Sprite > 0 {
				jobs[int(npc.Sprite)] = struct{}{}
			}
		}
	}
	for job := range jobs {
		resourceName, ok := b.manager.NonPCResourceName(job)
		if !ok {
			b.missing = append(b.missing, fmt.Sprintf("offline NPC resource name %d", job))
			continue
		}
		if res.IsGR2ResourceName(resourceName) {
			// The current offline Prontera slice uses legacy NPC sprites. Keep
			// GR2 NPCs visible in the manifest as a future closure extension
			// instead of treating them as legacy ACT/SPR files.
			b.missing = append(b.missing, fmt.Sprintf("offline NPC GR2 resource %d (%s)", job, resourceName))
			continue
		}
		for _, extension := range []string{"act", "spr"} {
			candidates := res.NonPCSpriteResourceCandidates(job, resourceName, extension)
			if len(candidates) == 0 {
				b.missing = append(b.missing, fmt.Sprintf("offline NPC %d %s candidates", job, extension))
				continue
			}
			if _, err := b.include(slash(candidates[0]), candidates); err != nil {
				b.missing = append(b.missing, fmt.Sprintf("offline NPC %d %s", job, extension))
			}
		}
	}
}

// includeOfflineMonsterResources closes the sprite dependency introduced by
// every generated offline spawn. The starter actor closure intentionally keeps
// a Poring available for the small HUD fixture, but a multi-map content pack
// can reference additional monsters. Resolve those IDs from the same client
// identity tables/fallback database used by the renderer and include only the
// ACT/SPR pairs the content actually names.
func (b *builder) includeOfflineMonsterResources(sourcePath string) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return
	}
	var content struct {
		Maps map[string]struct {
			Spawns []struct {
				MonsterID uint16 `json:"monster_id"`
			} `json:"spawns"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return
	}

	monsterIDs := make(map[int]struct{})
	for _, world := range content.Maps {
		for _, spawn := range world.Spawns {
			if spawn.MonsterID > 0 {
				monsterIDs[int(spawn.MonsterID)] = struct{}{}
			}
		}
	}
	for monsterID := range monsterIDs {
		resourceName, ok := b.manager.NonPCResourceName(monsterID)
		if !ok {
			b.missing = append(b.missing, fmt.Sprintf("offline monster resource name %d", monsterID))
			continue
		}
		if res.IsGR2ResourceName(resourceName) {
			b.missing = append(b.missing, fmt.Sprintf("offline monster GR2 resource %d (%s)", monsterID, resourceName))
			continue
		}
		for _, extension := range []string{"act", "spr"} {
			candidates := res.NonPCSpriteResourceCandidates(monsterID, resourceName, extension)
			if len(candidates) == 0 {
				b.missing = append(b.missing, fmt.Sprintf("offline monster %d %s candidates", monsterID, extension))
				continue
			}
			if _, err := b.include(slash(candidates[0]), candidates); err != nil {
				b.missing = append(b.missing, fmt.Sprintf("offline monster %d %s", monsterID, extension))
			}
		}
	}
}

// includeOfflineCombatFeedbackResources closes the renderer resources used by
// the existing combat presentation path. Without this small closure, offline
// packs fall back to the compact debug font for damage numbers and critical
// messages even though the normal client data contains the authored sprites.
func (b *builder) includeOfflineCombatFeedbackResources() {
	resources := []struct {
		canonical string
		label     string
	}{
		{canonical: "data/sprite/이팩트/숫자.act", label: "offline damage number act"},
		{canonical: "data/sprite/이팩트/숫자.spr", label: "offline damage number spr"},
		{canonical: "data/sprite/이팩트/msg.act", label: "offline damage message act"},
		{canonical: "data/sprite/이팩트/msg.spr", label: "offline damage message spr"},
	}
	for _, resource := range resources {
		candidates := []string{strings.ReplaceAll(resource.canonical, "/", "\\"), resource.canonical}
		if _, err := b.include(resource.canonical, candidates); err != nil {
			b.missing = append(b.missing, resource.label)
		}
	}
}

// includeOfflineItemResources closes the item presentation dependency
// introduced by the offline inventory, shops, and monster drops. The runtime
// still owns item identity and mutations; this only packages the metadata,
// icons, and ACT/SPR resources that the renderer can project.
func (b *builder) includeOfflineItemResources(sourcePath string) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return
	}
	var content struct {
		Maps map[string]struct {
			Spawns []struct {
				MonsterID int `json:"monster_id"`
			} `json:"spawns"`
			Shops []struct {
				Items []struct {
					ItemID int `json:"item_id"`
				} `json:"items"`
			} `json:"shops"`
		} `json:"maps"`
		Monsters map[string]struct {
			Drops []struct {
				ItemID int `json:"item_id"`
			} `json:"drops"`
		} `json:"monsters"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return
	}

	itemIDs := map[int]struct{}{
		501:  {}, // starter Red Potion
		909:  {}, // starter Jellopy
		1201: {}, // starter Knife
	}
	selectedMaps := make(map[string]struct{}, len(b.maps))
	for _, mapName := range b.maps {
		selectedMaps[strings.ToLower(strings.TrimSpace(mapName))] = struct{}{}
	}
	for mapName, world := range content.Maps {
		if b.scope != "complete" && len(selectedMaps) > 0 {
			if _, ok := selectedMaps[strings.ToLower(mapName)]; !ok {
				continue
			}
		}
		for _, spawn := range world.Spawns {
			monster, ok := content.Monsters[fmt.Sprint(spawn.MonsterID)]
			if !ok {
				continue
			}
			for _, drop := range monster.Drops {
				if drop.ItemID > 0 {
					itemIDs[drop.ItemID] = struct{}{}
				}
			}
		}
		for _, shop := range world.Shops {
			for _, item := range shop.Items {
				if item.ItemID > 0 {
					itemIDs[item.ItemID] = struct{}{}
				}
			}
		}
	}

	// These files are small but are required for ItemResourceName and the
	// inventory detail projection to behave like the normal resource manager.
	for _, candidates := range res.ItemMetadataResourceCandidates() {
		if len(candidates) == 0 {
			continue
		}
		_, _ = b.include(slash(candidates[0]), candidates)
	}

	ids := make([]int, 0, len(itemIDs))
	for itemID := range itemIDs {
		ids = append(ids, itemID)
	}
	sort.Ints(ids)
	for _, itemID := range ids {
		resourceName, ok := b.manager.ItemResourceName(itemID, true)
		if !ok || strings.TrimSpace(resourceName) == "" {
			b.missing = append(b.missing, fmt.Sprintf("offline item resource name %d", itemID))
			continue
		}
		for _, extension := range []string{"act", "spr"} {
			candidates := res.ItemSpriteResourceCandidates(resourceName, extension)
			if len(candidates) == 0 {
				b.missing = append(b.missing, fmt.Sprintf("offline item %d %s candidates", itemID, extension))
				continue
			}
			if _, err := b.include(slash(candidates[0]), candidates); err != nil {
				b.missing = append(b.missing, fmt.Sprintf("offline item %d %s", itemID, extension))
			}
		}
		// Icons are an enhancement: the renderer falls back to the item ACT/SPR
		// billboard when a particular client has no icon texture.
		iconCandidates := res.ItemIconTextureCandidates(resourceName)
		if len(iconCandidates) > 0 {
			_, _ = b.include(slash(iconCandidates[0]), iconCandidates)
		}
	}
}

// includeOfflineAudioResources packages the compact authored audio closure
// used by the offline loop: Prontera BGM, Bash/level-up feedback, and the
// starter knife attack/hit plus normal enemy hit sounds.
func (b *builder) includeOfflineAudioResources() {
	bgmCandidates := []string{"BGM\\01.mp3", "BGM/01.mp3", "bgm\\01.mp3", "bgm/01.mp3"}
	if _, err := b.include("BGM/01.mp3", bgmCandidates); err != nil {
		b.missing = append(b.missing, "offline BGM 01")
	}
	for _, candidates := range [][]string{
		{"data\\mp3nametable.txt", "data/mp3nametable.txt", "mp3nametable.txt"},
	} {
		if len(candidates) > 0 {
			_, _ = b.include(slash(candidates[0]), candidates)
		}
	}

	sounds := []string{"effect\\ef_bash.wav", "levelup.wav"}
	sounds = append(sounds, db.EnemyHitNormalSounds()...)
	sounds = append(sounds, db.WeaponAttackSounds(db.WeaponShortsword)...)
	sounds = append(sounds, db.WeaponHitSounds(db.WeaponShortsword)...)
	seen := make(map[string]struct{}, len(sounds))
	for _, sound := range sounds {
		sound = strings.TrimSpace(strings.ReplaceAll(sound, "/", "\\"))
		if sound == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(sound)]; ok {
			continue
		}
		seen[strings.ToLower(sound)] = struct{}{}
		name := strings.TrimSuffix(sound, filepath.Ext(sound))
		canonical := "data/wav/" + strings.ReplaceAll(name+filepath.Ext(sound), "\\", "/")
		candidates := []string{sound, strings.ReplaceAll(sound, "\\", "/"), "wav\\" + sound, "wav/" + strings.ReplaceAll(sound, "\\", "/"), "data\\wav\\" + sound, "data/wav/" + strings.ReplaceAll(sound, "\\", "/")}
		if _, err := b.include(canonical, candidates); err != nil {
			b.missing = append(b.missing, "offline SFX "+sound)
		}
	}
}

func (b *builder) includeTexture(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	canonical := res.NormalizeResourcePath("data/texture/" + name)
	if _, err := b.include(canonical, res.GroundTextureCandidates(name)); err != nil {
		b.missing = append(b.missing, name)
	}
}

func (b *builder) include(canonical string, candidates []string) ([]byte, error) {
	if data, ok := b.files[canonical]; ok {
		return data, nil
	}
	var data []byte
	var err error
	for _, candidate := range candidates {
		data, err = b.manager.ReadFile(candidate)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("resource not found: %s", strings.Join(candidates, ", "))
	}
	b.files[canonical] = data
	b.included[canonical] = struct{}{}
	b.recordCategory(canonical, int64(len(data)))
	return data, nil
}

func (b *builder) includeOfflineContent(sourcePath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read offline content %q: %w", sourcePath, err)
	}
	canonical := "offline/content.json"
	if _, exists := b.included[canonical]; exists {
		return fmt.Errorf("offline content path already included")
	}
	b.files[canonical] = data
	b.included[canonical] = struct{}{}
	b.recordCategory(canonical, int64(len(data)))
	if b.stage != "" {
		if err := writeStageFile(b.stage, canonical, data); err != nil {
			return fmt.Errorf("stage offline content: %w", err)
		}
	}
	return nil
}

func (b *builder) recordCategory(canonical string, size int64) {
	category := "other"
	parts := strings.Split(canonical, "/")
	if len(parts) > 0 {
		category = parts[0]
		if parts[0] == "data" && len(parts) > 1 {
			category = parts[1]
		}
	}
	if strings.HasSuffix(canonical, ".gnd") || strings.HasSuffix(canonical, ".rsw") || strings.HasSuffix(canonical, ".gat") {
		category = "map"
	}
	b.categories[canonical] = category
	b.counts[category]++
	b.bytes[category] += size
}

func slash(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "\\", "/"), "//", "/")
}
