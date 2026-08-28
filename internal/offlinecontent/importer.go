// Package offlinecontent converts the selected, data-only part of an
// rAthena checkout into the JSON contract consumed by session. It is a build
// time importer: the client never parses rAthena YAML or executes scripts.
package offlinecontent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/session"
	"gopkg.in/yaml.v3"
)

const (
	DefaultProfile   = "pre-re"
	DefaultPacketver = 20080910
)

type Options struct {
	RathenaRoot string
	Maps        []string
	Profile     string
	Packetver   int
}

type Result struct {
	Content  session.OfflineContent
	Warnings []string
}

type yamlImport struct {
	Path string `yaml:"Path"`
}

type yamlDocument struct {
	Header struct {
		Imports []yamlImport `yaml:"Imports"`
	} `yaml:"Header"`
	Body   []map[string]any `yaml:"Body"`
	Footer struct {
		Imports []yamlImport `yaml:"Imports"`
	} `yaml:"Footer"`
}

type sourceReader struct {
	root  string
	files map[string]string
	bytes map[string][]byte
}

func Build(options Options) (session.OfflineContent, error) {
	root := filepath.Clean(strings.TrimSpace(options.RathenaRoot))
	if root == "." || root == "" {
		return session.OfflineContent{}, errors.New("rathena root is required")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return session.OfflineContent{}, fmt.Errorf("rathena root %q: %w", root, err)
	}
	maps := normalizeMaps(options.Maps)
	if len(maps) == 0 {
		return session.OfflineContent{}, errors.New("at least one map is required")
	}
	profile := options.Profile
	if profile == "" {
		profile = DefaultProfile
	}
	packetver := options.Packetver
	if packetver == 0 {
		packetver = DefaultPacketver
	}

	r := &sourceReader{root: root, files: map[string]string{}, bytes: map[string][]byte{}}
	items, err := r.loadDatabase("db/"+profile+"/item_db.yml", "item_db.yml")
	if err != nil {
		return session.OfflineContent{}, fmt.Errorf("items: %w", err)
	}
	monsters, err := r.loadDatabase("db/"+profile+"/mob_db.yml", "mob_db.yml")
	if err != nil {
		return session.OfflineContent{}, fmt.Errorf("monsters: %w", err)
	}
	skills, err := r.loadDatabase("db/"+profile+"/skill_db.yml", "skill_db.yml")
	if err != nil {
		return session.OfflineContent{}, fmt.Errorf("skills: %w", err)
	}

	content := session.OfflineContent{
		Format:   session.OfflineContentFormat,
		Version:  session.OfflineContentVersion,
		Source:   session.OfflineContentSource{Emulator: "rAthena", Profile: profile, Packetver: packetver, Commit: gitCommit(root), Files: r.files},
		Maps:     map[string]session.OfflineMap{},
		Items:    map[uint16]session.OfflineItem{},
		Monsters: map[uint16]session.OfflineMonsterDef{},
		Skills:   map[uint16]session.OfflineSkillDef{},
	}
	warnings := []string{}
	for _, raw := range items {
		item, itemWarnings, err := parseItem(raw)
		if err != nil {
			return session.OfflineContent{}, fmt.Errorf("item: %w", err)
		}
		content.Items[item.ID] = item
		warnings = append(warnings, itemWarnings...)
	}
	for _, raw := range monsters {
		monster, err := parseMonster(raw, content.Items)
		if err != nil {
			// rAthena uses TREASURE_BOX1 as a scripted container rather than a
			// combat monster. It has zero HP by design and must not become a
			// local combat definition. Keep all other malformed records fatal.
			aegis := strings.ToUpper(strings.TrimSpace(stringValue(raw["AegisName"])))
			if strings.Contains(aegis, "TREASURE_BOX") &&
				(strings.Contains(err.Error(), "malformed Hp") || strings.Contains(err.Error(), "invalid core stats")) {
				warnings = append(warnings, fmt.Sprintf("monster skipped: %v", err))
				continue
			}
			return session.OfflineContent{}, fmt.Errorf("monster: %w", err)
		}
		content.Monsters[monster.ID] = monster
	}
	for _, raw := range skills {
		skill, err := parseSkill(raw)
		if err != nil {
			return session.OfflineContent{}, fmt.Errorf("skill: %w", err)
		}
		content.Skills[skill.ID] = skill
	}

	declarations, scriptWarnings, err := r.parseScripts(maps)
	if err != nil {
		return session.OfflineContent{}, err
	}
	warnings = append(warnings, scriptWarnings...)
	if err := applyDeclarations(&content, declarations); err != nil {
		return session.OfflineContent{}, err
	}
	for _, mapName := range maps {
		if _, ok := content.Maps[mapName]; !ok {
			content.Maps[mapName] = session.OfflineMap{Name: mapName}
		}
	}
	filterSelectedMapWarps(&content, maps, &warnings)
	content.Warnings = uniqueSorted(warnings)
	content.Source.Fingerprint = fingerprint(content.Source.Commit, profile, packetver, maps, r.bytes)
	session.NormalizeOfflineContent(&content)
	return content, nil
}

// AddStarterPoring adds the deterministic close-range encounter used by the
// offline mobile fixture. It is an explicit build-time fixture overlay, not a
// new gameplay rule: the monster definition still comes from the imported
// rAthena database and the runtime remains authoritative for combat.
func AddStarterPoring(content *session.OfflineContent, mapName string, x, y int) error {
	if content == nil {
		return errors.New("offline content is required")
	}
	mapName = strings.ToLower(strings.TrimSpace(mapName))
	world, ok := content.Maps[mapName]
	if !ok {
		return fmt.Errorf("offline content has no map %q", mapName)
	}
	monster, ok := content.Monsters[1002]
	if !ok {
		return errors.New("offline content has no Poring monster definition (1002)")
	}
	for _, spawn := range world.Spawns {
		if spawn.MonsterID == monster.ID && spawn.X == x && spawn.Y == y && spawn.Count == 1 {
			return nil
		}
	}
	world.Spawns = append(world.Spawns, session.OfflineSpawn{
		ID:           stableID(fmt.Sprintf("fixture|starter-poring|%s|%d|%d", mapName, x, y)),
		MonsterID:    monster.ID,
		Name:         monster.Name,
		X:            x,
		Y:            y,
		Count:        1,
		RespawnMinMS: 7200000,
		RespawnMaxMS: 3600000,
	})
	content.Maps[mapName] = world
	// Include the fixture overlay in the content identity so saves cannot
	// silently reuse a world seeded from a different encounter layout.
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s\nfixture:starter-poring\nmap:%s\nx:%d\ny:%d\n", content.Source.Fingerprint, mapName, x, y)
	content.Source.Fingerprint = hex.EncodeToString(hash.Sum(nil))
	return nil
}

// filterSelectedMapWarps keeps the generated pack self-contained. A selected
// map can have source declarations that lead to maps intentionally omitted
// from this pack; retaining those visible-but-unloadable edges would make the
// runtime reject an otherwise valid pack. The omission is explicit and
// deterministic so a broader map selection can restore the source edge.
func filterSelectedMapWarps(content *session.OfflineContent, maps []string, warnings *[]string) {
	selected := make(map[string]struct{}, len(maps))
	for _, mapName := range maps {
		selected[strings.ToLower(strings.TrimSpace(mapName))] = struct{}{}
	}
	for mapName, world := range content.Maps {
		kept := world.Warps[:0]
		for _, warp := range world.Warps {
			destination := strings.ToLower(strings.TrimSpace(warp.Map))
			if _, ok := selected[destination]; !ok {
				*warnings = append(*warnings, fmt.Sprintf("map %s warp %q omitted: destination map %q is outside selected pack", mapName, warp.Name, destination))
				continue
			}
			warp.Map = destination
			kept = append(kept, warp)
		}
		world.Warps = kept
		content.Maps[mapName] = world
	}
}

func normalizeMaps(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(value, ".gat"), ".rsw")))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (r *sourceReader) read(rel string, required bool) ([]byte, error) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return nil, fmt.Errorf("source path escapes rathena root: %s", rel)
	}
	if data, ok := r.bytes[rel]; ok {
		return data, nil
	}
	data, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	r.bytes[rel] = data
	hash := sha256.Sum256(data)
	r.files[rel] = hex.EncodeToString(hash[:])
	return data, nil
}

func (r *sourceReader) loadDatabase(primary, overrideName string) ([]map[string]any, error) {
	visited := map[string]struct{}{}
	var records []map[string]any
	var visit func(string, bool) error
	visit = func(rel string, required bool) error {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if _, ok := visited[rel]; ok {
			return nil
		}
		data, err := r.read(rel, required)
		if err != nil || data == nil {
			return err
		}
		visited[rel] = struct{}{}
		var document yamlDocument
		if err := yaml.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		for _, record := range document.Body {
			records = append(records, normalizeMap(record))
		}
		imports := append(append([]yamlImport(nil), document.Header.Imports...), document.Footer.Imports...)
		for _, imported := range imports {
			if strings.TrimSpace(imported.Path) == "" {
				return fmt.Errorf("malformed empty import in %s", rel)
			}
			importPath := imported.Path
			if !strings.Contains(importPath, "/") && !strings.Contains(importPath, "\\") {
				importPath = filepath.ToSlash(filepath.Join(filepath.Dir(rel), importPath))
			}
			if err := visit(importPath, true); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(primary, true); err != nil {
		return nil, err
	}
	// db/import is optional, but when present it has rAthena's documented
	// override precedence over the selected pre-renewal database.
	if err := visit(filepath.ToSlash(filepath.Join("db/import", overrideName)), false); err != nil {
		return nil, err
	}
	byID := map[int]map[string]any{}
	order := []int{}
	for _, record := range records {
		id, ok := intValue(record["Id"])
		if !ok || id <= 0 || id > 65535 {
			return nil, fmt.Errorf("malformed record id in %s", primary)
		}
		if previous, exists := byID[id]; exists {
			byID[id] = mergeMaps(previous, record)
			continue
		}
		byID[id] = record
		order = append(order, id)
	}
	sort.Ints(order)
	result := make([]map[string]any, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s contains no records", primary)
	}
	return result, nil
}

func parseItem(raw map[string]any) (session.OfflineItem, []string, error) {
	id, ok := intValue(raw["Id"])
	if !ok || id <= 0 || id > 65535 {
		return session.OfflineItem{}, nil, fmt.Errorf("invalid id")
	}
	aegis := stringValue(raw["AegisName"])
	name := stringValue(raw["Name"])
	if aegis == "" || name == "" {
		return session.OfflineItem{}, nil, fmt.Errorf("item %d has no AegisName or Name", id)
	}
	buy, _ := int64Value(raw["Buy"])
	sell, _ := int64Value(raw["Sell"])
	for _, field := range []string{"Buy", "Sell", "Weight"} {
		if value, exists := raw[field]; exists {
			if field == "Weight" {
				if _, ok := intValue(value); !ok {
					return session.OfflineItem{}, nil, fmt.Errorf("item %d has malformed %s", id, field)
				}
			} else if _, ok := int64Value(value); !ok {
				return session.OfflineItem{}, nil, fmt.Errorf("item %d has malformed %s", id, field)
			}
		}
	}
	if buy <= 0 && sell > 0 {
		buy = sell * 2
	}
	if sell <= 0 && buy > 0 {
		sell = buy / 2
	}
	item := session.OfflineItem{ID: uint16(id), AegisName: aegis, Name: name, Type: itemType(stringValue(raw["Type"])), Buy: buy, Sell: sell}
	item.Weight, _ = intValue(raw["Weight"])
	item.Locations = locationMask(raw["Locations"])
	if stack, ok := mapValue(raw["Stack"]); ok {
		item.StackAmount, _ = intValue(stack["Amount"])
	}
	if trade, ok := mapValue(raw["Trade"]); ok {
		item.NoDrop = boolValue(trade["NoDrop"])
		item.NoSell = boolValue(trade["NoSell"])
		item.NoStorage = boolValue(trade["NoStorage"])
	}
	warnings := []string{}
	if script := strings.TrimSpace(stringValue(raw["Script"])); script != "" {
		effect, supported := parseUseEffect(script)
		if supported {
			item.UseEffect = effect
			item.UseSupported = true
		} else {
			warnings = append(warnings, fmt.Sprintf("item %d (%s): unsupported Script disabled", id, aegis))
		}
	}
	return item, warnings, nil
}

func parseMonster(raw map[string]any, items map[uint16]session.OfflineItem) (session.OfflineMonsterDef, error) {
	id, ok := intValue(raw["Id"])
	if !ok || id <= 0 || id > 65535 {
		return session.OfflineMonsterDef{}, fmt.Errorf("invalid id")
	}
	aegis, name := stringValue(raw["AegisName"]), stringValue(raw["Name"])
	if aegis == "" || name == "" {
		return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has no AegisName or Name", id)
	}
	attackMin, _ := intValue(raw["Attack"])
	attackMax, _ := intValue(raw["Attack2"])
	for _, field := range []string{"Attack", "Attack2"} {
		if value, exists := raw[field]; exists {
			if _, ok := intValue(value); !ok {
				return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has malformed %s", id, field)
			}
		}
	}
	if attackMax <= 0 {
		attackMax = attackMin
	}
	monster := session.OfflineMonsterDef{ID: uint16(id), AegisName: aegis, Name: name, Level: 1, HP: 1}
	if err := setOptionalInt(&monster.Level, raw, "Level", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt(&monster.HP, raw, "Hp", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	monster.AttackMin, monster.AttackMax = attackMin, attackMax
	if err := setOptionalInt(&monster.AttackRange, raw, "AttackRange", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt(&monster.Defense, raw, "Defense", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt(&monster.WalkSpeed, raw, "WalkSpeed", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt(&monster.AttackDelay, raw, "AttackDelay", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt64(&monster.BaseEXP, raw, "BaseExp", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if err := setOptionalInt64(&monster.JobEXP, raw, "JobExp", id, "monster"); err != nil {
		return session.OfflineMonsterDef{}, err
	}
	if monster.HP <= 0 || monster.AttackMin < 0 || monster.AttackMax < monster.AttackMin || monster.Defense < 0 {
		return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has invalid core stats", id)
	}
	if drops, ok := sliceValue(raw["Drops"]); ok {
		for _, rawDrop := range drops {
			drop, ok := mapValue(rawDrop)
			if !ok {
				return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has malformed drop", id)
			}
			itemID, ok := resolveItem(drop["Item"], items)
			if !ok {
				return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has unknown drop item %q", id, stringValue(drop["Item"]))
			}
			rate, ok := intValue(drop["Rate"])
			if !ok || rate < 0 || rate > 10000 {
				return session.OfflineMonsterDef{}, fmt.Errorf("monster %d has invalid drop rate", id)
			}
			monster.Drops = append(monster.Drops, session.OfflineDropTable{ItemID: itemID, Type: items[itemID].Type, Amount: 1, Rate: rate})
		}
	}
	return monster, nil
}

func parseSkill(raw map[string]any) (session.OfflineSkillDef, error) {
	id, ok := intValue(raw["Id"])
	if !ok || id <= 0 || id > 65535 {
		return session.OfflineSkillDef{}, fmt.Errorf("invalid id")
	}
	name := stringValue(raw["Name"])
	if name == "" {
		return session.OfflineSkillDef{}, fmt.Errorf("skill %d has no Name", id)
	}
	skill := session.OfflineSkillDef{ID: uint16(id), Name: name, Target: stringValue(raw["TargetType"]), MaxLevel: 1}
	if err := setOptionalInt(&skill.MaxLevel, raw, "MaxLevel", id, "skill"); err != nil {
		return session.OfflineSkillDef{}, err
	}
	skill.Range = firstNumber(raw["Range"])
	// rAthena's -1 range means the skill uses its default/melee range. The
	// offline contract stores an effective non-negative range instead.
	if skill.Range < 0 {
		skill.Range = 0
	}
	if raw["Range"] != nil {
		if _, ok := firstNumberOK(raw["Range"]); !ok {
			return session.OfflineSkillDef{}, fmt.Errorf("skill %d has malformed Range", id)
		}
	}
	if requires, ok := mapValue(raw["Requires"]); ok {
		skill.SPCost = firstNumber(requires["SpCost"])
		if requires["SpCost"] != nil {
			if _, ok := firstNumberOK(requires["SpCost"]); !ok {
				return session.OfflineSkillDef{}, fmt.Errorf("skill %d has malformed SpCost", id)
			}
		}
	}
	if raw["Cooldown"] != nil {
		var ok bool
		skill.CooldownMS, ok = durationMSOK(firstValue(raw["Cooldown"]))
		if !ok {
			return session.OfflineSkillDef{}, fmt.Errorf("skill %d has malformed Cooldown", id)
		}
	}
	if raw["AfterCastActDelay"] != nil {
		var ok bool
		skill.AfterCastDelayMS, ok = durationMSOK(firstValue(raw["AfterCastActDelay"]))
		if !ok {
			return session.OfflineSkillDef{}, fmt.Errorf("skill %d has malformed AfterCastActDelay", id)
		}
	}
	if skill.MaxLevel < 0 || skill.Range < 0 || skill.SPCost < 0 || skill.CooldownMS < 0 {
		return session.OfflineSkillDef{}, fmt.Errorf("skill %d has invalid values", id)
	}
	return skill, nil
}

type declaration struct {
	kind    string
	mapName string
	line    string
	file    string
	lineNo  int
	shop    *session.OfflineShopDef
	npc     *session.OfflineNPC
	spawn   *session.OfflineSpawn
	warp    *session.OfflineWarp
}

var (
	shopPattern    = regexp.MustCompile(`^([^,\s]+),(-?\d+),(-?\d+),(-?\d+)\s+shop\s+(.+?)\s+(-?\d+),(.+)$`)
	monsterPattern = regexp.MustCompile(`^([^,\s]+),(-?\d+),(-?\d+)(?:,(\d+),(\d+))?\s+monster\s+(.+?)\s+(\d+),(\d+)(?:,(\d+),(\d+))?.*$`)
	warpPattern    = regexp.MustCompile(`^([^,\s]+),(-?\d+),(-?\d+),(-?\d+)\s+warp\s+(.+?)\s+(\d+),(\d+),([^,\s]+),(-?\d+),(-?\d+).*$`)
	npcPattern     = regexp.MustCompile(`^([^,\s]+),(-?\d+),(-?\d+),(-?\d+)\s+script(?:\([^)]*\))?\s+(.+?)\s+(-?\d+),.*$`)
)

func (r *sourceReader) parseScripts(maps []string) ([]declaration, []string, error) {
	files, err := r.scriptFiles()
	if err != nil {
		return nil, nil, err
	}
	wanted := map[string]struct{}{}
	for _, mapName := range maps {
		wanted[mapName] = struct{}{}
	}
	var declarations []declaration
	warnings := []string{}
	for _, file := range files {
		data, err := r.read(file, true)
		if err != nil {
			return nil, nil, err
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for lineNo, rawLine := range lines {
			line := strings.TrimSpace(stripScriptComment(rawLine))
			line = strings.ReplaceAll(line, `\t`, "\t")
			if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
				continue
			}
			mapName := strings.ToLower(strings.TrimSpace(strings.Split(line, ",")[0]))
			if _, ok := wanted[mapName]; !ok {
				continue
			}
			if match := shopPattern.FindStringSubmatch(line); match != nil {
				shop, npc, warning, err := parseShopDeclaration(match, file, lineNo+1, r)
				if err != nil {
					return nil, nil, err
				}
				if warning != "" {
					warnings = append(warnings, warning)
				}
				declarations = append(declarations, declaration{kind: "shop", mapName: mapName, line: line, file: file, lineNo: lineNo + 1, shop: shop, npc: npc})
				continue
			}
			if match := monsterPattern.FindStringSubmatch(line); match != nil {
				x, _ := strconv.Atoi(match[2])
				y, _ := strconv.Atoi(match[3])
				radius := 0
				if match[4] != "" {
					xRadius, _ := strconv.Atoi(match[4])
					yRadius, _ := strconv.Atoi(match[5])
					radius = xRadius
					if yRadius > radius {
						radius = yRadius
					}
				}
				count, _ := strconv.Atoi(match[8])
				if count <= 0 {
					return nil, nil, fmt.Errorf("%s:%d: monster spawn count must be positive", file, lineNo+1)
				}
				spawn := &session.OfflineSpawn{ID: stableID("spawn|" + file + "|" + strconv.Itoa(lineNo+1) + "|" + line), MonsterID: uint16(mustInt(match[7])), Name: strings.TrimSpace(match[6]), X: x, Y: y, Radius: radius, Count: count}
				if match[9] != "" {
					spawn.RespawnMinMS, _ = strconv.Atoi(match[9])
					spawn.RespawnMaxMS, _ = strconv.Atoi(match[10])
				}
				declarations = append(declarations, declaration{kind: "spawn", mapName: mapName, line: line, file: file, lineNo: lineNo + 1, spawn: spawn})
				continue
			}
			if match := warpPattern.FindStringSubmatch(line); match != nil {
				width, _ := strconv.Atoi(match[6])
				height, _ := strconv.Atoi(match[7])
				warp := &session.OfflineWarp{ID: stableID("warp|" + file + "|" + strconv.Itoa(lineNo+1) + "|" + line), Name: strings.TrimSpace(match[5]), X: mustInt(match[2]), Y: mustInt(match[3]), Width: width, Height: height, Map: strings.ToLower(match[8]), DestX: mustInt(match[9]), DestY: mustInt(match[10])}
				declarations = append(declarations, declaration{kind: "warp", mapName: mapName, line: line, file: file, lineNo: lineNo + 1, warp: warp})
				continue
			}
			if match := npcPattern.FindStringSubmatch(line); match != nil {
				declarations = append(declarations, declaration{kind: "npc", mapName: mapName, line: line, file: file, lineNo: lineNo + 1, npc: &session.OfflineNPC{ID: stableID("npc|" + file + "|" + strconv.Itoa(lineNo+1) + "|" + line), Name: strings.TrimSpace(match[5]), Sprite: int16(mustInt(match[6])), X: mustInt(match[2]), Y: mustInt(match[3]), Dir: mustInt(match[4])}})
				continue
			}
			if strings.Contains(line, " shop ") || strings.Contains(line, " monster ") || strings.Contains(line, " warp ") {
				warnings = append(warnings, fmt.Sprintf("%s:%d: unsupported declaration ignored", file, lineNo+1))
			}
		}
	}
	return declarations, warnings, nil
}

func (r *sourceReader) scriptFiles() ([]string, error) {
	roots := []string{"npc/pre-re/scripts_main.conf", "npc/re/scripts_main.conf"}
	var root string
	for _, candidate := range roots {
		if data, _ := r.read(candidate, false); data != nil {
			root = candidate
			break
		}
	}
	if root == "" {
		return nil, errors.New("rathena script entrypoint not found (expected npc/pre-re/scripts_main.conf)")
	}
	visited := map[string]struct{}{}
	var result []string
	var visit func(string) error
	visit = func(rel string) error {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if _, ok := visited[rel]; ok {
			return nil
		}
		data, err := r.read(rel, true)
		if err != nil {
			return err
		}
		visited[rel] = struct{}{}
		for _, rawLine := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			line := strings.TrimSpace(stripScriptComment(rawLine))
			fields := strings.Fields(line)
			if len(fields) < 2 || (fields[0] != "npc:" && fields[0] != "import:") {
				continue
			}
			pathName := strings.Trim(strings.Join(fields[1:], " "), "\"'")
			if pathName == "" {
				continue
			}
			if err := visit(pathName); err != nil {
				return err
			}
		}
		if strings.HasSuffix(strings.ToLower(rel), ".txt") {
			result = append(result, rel)
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func parseShopDeclaration(match []string, file string, lineNo int, r *sourceReader) (*session.OfflineShopDef, *session.OfflineNPC, string, error) {
	npcID := stableID("npc|" + file + "|" + strconv.Itoa(lineNo) + "|" + match[0])
	shopID := stableID("shop|" + file + "|" + strconv.Itoa(lineNo) + "|" + match[0])
	npc := &session.OfflineNPC{ID: npcID, Name: strings.TrimSpace(match[5]), Sprite: int16(mustInt(match[6])), X: mustInt(match[2]), Y: mustInt(match[3]), Dir: mustInt(match[4]), ShopID: shopID}
	shop := &session.OfflineShopDef{ID: shopID, NPCID: npcID, Name: strings.TrimSpace(match[5])}
	warnings := []string{}
	for _, token := range strings.Split(match[7], ",") {
		parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
		if len(parts) != 2 {
			return nil, nil, "", fmt.Errorf("%s:%d: malformed shop item %q", file, lineNo, token)
		}
		itemID, ok := resolveItemName(parts[0], r)
		if !ok {
			if parsed, parsedErr := strconv.Atoi(parts[0]); parsedErr == nil && parsed > 0 && parsed <= 65535 {
				itemID = uint16(parsed)
				warnings = append(warnings, fmt.Sprintf("%s:%d: shop item %d is not present in item_db", file, lineNo, itemID))
			} else {
				return nil, nil, "", fmt.Errorf("%s:%d: unknown shop item %q", file, lineNo, parts[0])
			}
		}
		price, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, nil, "", fmt.Errorf("%s:%d: invalid shop price %q", file, lineNo, parts[1])
		}
		shop.Items = append(shop.Items, session.OfflineShopItemDef{ItemID: itemID, Price: price, Stock: 0})
	}
	return shop, npc, strings.Join(warnings, "; "), nil
}

func applyDeclarations(content *session.OfflineContent, declarations []declaration) error {
	for _, item := range declarations {
		world := content.Maps[item.mapName]
		world.Name = item.mapName
		switch item.kind {
		case "shop":
			for i := range item.shop.Items {
				shopItem := &item.shop.Items[i]
				definition, ok := content.Items[shopItem.ItemID]
				if !ok {
					return fmt.Errorf("%s:%d: shop references unknown item %d", item.file, item.lineNo, shopItem.ItemID)
				}
				if shopItem.Price == -1 {
					shopItem.Price = definition.Buy
				}
				if shopItem.Price < 0 {
					return fmt.Errorf("%s:%d: shop item %d has unresolved price", item.file, item.lineNo, shopItem.ItemID)
				}
			}
			world.Shops = append(world.Shops, *item.shop)
			world.NPCs = append(world.NPCs, *item.npc)
		case "npc":
			world.NPCs = append(world.NPCs, *item.npc)
		case "spawn":
			if _, ok := content.Monsters[item.spawn.MonsterID]; !ok {
				return fmt.Errorf("%s:%d: spawn references unknown monster %d", item.file, item.lineNo, item.spawn.MonsterID)
			}
			world.Spawns = append(world.Spawns, *item.spawn)
		case "warp":
			world.Warps = append(world.Warps, *item.warp)
		}
		content.Maps[item.mapName] = world
	}
	return nil
}

func resolveItemName(value any, r *sourceReader) (uint16, bool) {
	if id, ok := intValue(value); ok && id > 0 && id <= 65535 {
		return uint16(id), true
	}
	name := strings.ToLower(strings.TrimSpace(stringValue(value)))
	if name == "" {
		return 0, false
	}
	// The importer has already parsed item records, but this helper only has a
	// source reader. Keep a small name index populated while reading item YAML.
	for _, rel := range sortedStringKeysBytes(r.bytes) {
		if !strings.Contains(rel, "item_db") {
			continue
		}
		var document yamlDocument
		if yaml.Unmarshal(r.bytes[rel], &document) != nil {
			continue
		}
		for _, record := range document.Body {
			if strings.EqualFold(stringValue(record["AegisName"]), name) {
				if id, ok := intValue(record["Id"]); ok && id > 0 && id <= 65535 {
					return uint16(id), true
				}
			}
		}
	}
	return 0, false
}

func resolveItem(value any, items map[uint16]session.OfflineItem) (uint16, bool) {
	if id, ok := intValue(value); ok && id > 0 && id <= 65535 {
		_, exists := items[uint16(id)]
		return uint16(id), exists
	}
	name := strings.TrimSpace(stringValue(value))
	for id, item := range items {
		if strings.EqualFold(item.AegisName, name) {
			return id, true
		}
	}
	return 0, false
}

func parseUseEffect(script string) (session.OfflineUseEffect, bool) {
	effect := session.OfflineUseEffect{}
	matched := false
	pattern := regexp.MustCompile(`(?i)\b(itemheal|percentheal)\s+(rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)\s*,\s*(rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)`)
	for _, command := range pattern.FindAllStringSubmatch(script, -1) {
		hpMin, hpMax, hpOK := effectRange(command[2])
		spMin, spMax, spOK := effectRange(command[3])
		if !hpOK || !spOK {
			return session.OfflineUseEffect{}, false
		}
		if strings.EqualFold(command[1], "itemheal") {
			effect.HP, effect.SP = hpMin, spMin
			effect.HPMin, effect.HPMax, effect.SPMin, effect.SPMax = hpMin, hpMax, spMin, spMax
		} else {
			effect.HPPercent, effect.SPPercent = hpMin, spMin
			effect.HPPercentMin, effect.HPPercentMax, effect.SPPercentMin, effect.SPPercentMax = hpMin, hpMax, spMin, spMax
		}
		matched = true
	}
	if !matched {
		return session.OfflineUseEffect{}, false
	}
	clean := regexp.MustCompile(`(?i)itemheal\s+(?:rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)\s*,\s*(?:rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)|percentheal\s+(?:rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)\s*,\s*(?:rand\(\s*-?\d+\s*,\s*-?\d+\s*\)|-?\d+)|[{};\s]+`).ReplaceAllString(script, "")
	return effect, clean == ""
}

func effectRange(value string) (int, int, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "rand(") && strings.HasSuffix(value, ")") {
		parts := strings.Split(strings.TrimSuffix(value[5:], ")"), ",")
		if len(parts) != 2 {
			return 0, 0, false
		}
		minimum, minErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		maximum, maxErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		return minimum, maximum, minErr == nil && maxErr == nil && maximum >= minimum
	}
	amount, err := strconv.Atoi(value)
	return amount, amount, err == nil
}

func stripScriptComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func itemType(value string) uint8 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "healing":
		return db.ItemTypeHealing
	case "usable":
		return db.ItemTypeUsable
	case "etc":
		return db.ItemTypeEtc
	case "armor":
		return db.ItemTypeArmor
	case "weapon":
		return db.ItemTypeWeapon
	case "card":
		return db.ItemTypeCard
	case "petegg":
		return db.ItemTypePetEgg
	case "petarmor":
		return db.ItemTypePetArmor
	case "ammo":
		return db.ItemTypeAmmo
	case "delayconsume":
		return db.ItemTypeDelayConsume
	case "shadowgear":
		return db.ItemTypeShadowGear
	case "cash":
		return db.ItemTypeCash
	default:
		return db.ItemTypeEtc
	}
}

func locationMask(value any) uint16 {
	locations, ok := mapValue(value)
	if !ok {
		return 0
	}
	var mask uint16
	for name, enabled := range locations {
		if !boolValue(enabled) {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(name, "_", "")) {
		case "headtop":
			mask |= db.EquipHeadTop
		case "headmid":
			mask |= db.EquipHeadMid
		case "headlow":
			mask |= db.EquipHeadBottom
		case "armor":
			mask |= db.EquipArmor
		case "righthand", "bothhand", "bothhands":
			mask |= db.EquipWeapon
		case "lefthand":
			mask |= db.EquipShield
		case "garment":
			mask |= db.EquipGarment
		case "shoes":
			mask |= db.EquipShoes
		case "rightaccessory":
			mask |= db.EquipAccessory1
		case "leftaccessory":
			mask |= db.EquipAccessory2
		case "ammo":
			mask |= db.EquipAmmo
		}
	}
	return mask
}

func durationMSOK(value any) (int, bool) {
	if value == nil {
		return 0, true
	}
	if mapped, ok := mapValue(value); ok {
		for _, key := range []string{"Time", "Amount", "Value"} {
			if candidate, exists := mapped[key]; exists {
				return durationMSOK(candidate)
			}
		}
		return 0, false
	}
	if number, ok := intValue(value); ok {
		return number, true
	}
	parsed, err := time.ParseDuration(stringValue(value))
	if err != nil {
		return 0, false
	}
	return int(parsed / time.Millisecond), true
}

func firstNumber(value any) int {
	result, _ := firstNumberOK(value)
	return result
}

func firstNumberOK(value any) (int, bool) {
	value = firstValue(value)
	if mapped, ok := mapValue(value); ok {
		for _, key := range []string{"Amount", "Time", "Value", "Size", "Area"} {
			if candidate, exists := mapped[key]; exists {
				return firstNumberOK(candidate)
			}
		}
		return 0, false
	}
	return intValue(value)
}

func firstValue(value any) any {
	if values, ok := sliceValue(value); ok && len(values) > 0 {
		return values[0]
	}
	return value
}

func mapValue(value any) (map[string]any, bool) {
	switch values := value.(type) {
	case map[string]any:
		return values, true
	case map[any]any:
		result := map[string]any{}
		for key, value := range values {
			result[stringValue(key)] = value
		}
		return result, true
	default:
		return nil, false
	}
}

func sliceValue(value any) ([]any, bool) {
	values, ok := value.([]any)
	return values, ok
}

func normalizeMap(value map[string]any) map[string]any {
	result := map[string]any{}
	for key, entry := range value {
		result[key] = normalizeValue(entry)
	}
	return result
}

func normalizeValue(value any) any {
	if mapped, ok := mapValue(value); ok {
		return normalizeMap(mapped)
	}
	if values, ok := sliceValue(value); ok {
		result := make([]any, len(values))
		for i, entry := range values {
			result[i] = normalizeValue(entry)
		}
		return result
	}
	return value
}

func mergeMaps(base, overlay map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		if baseMap, ok := mapValue(result[key]); ok {
			if overlayMap, overlayOK := mapValue(value); overlayOK {
				result[key] = mergeMaps(baseMap, overlayMap)
				continue
			}
		}
		result[key] = value
	}
	return result
}

func intValue(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int8:
		return int(number), true
	case int16:
		return int(number), true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case uint:
		return int(number), true
	case uint8:
		return int(number), true
	case uint16:
		return int(number), true
	case uint32:
		return int(number), true
	case uint64:
		return int(number), true
	case float64:
		return int(number), number == float64(int(number))
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(number))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func int64Value(value any) (int64, bool) {
	if number, ok := intValue(value); ok {
		return int64(number), true
	}
	if value, ok := value.(int64); ok {
		return value, true
	}
	return 0, false
}

func setOptionalInt(target *int, values map[string]any, key string, id int, kind string) error {
	value, exists := values[key]
	if !exists || value == nil {
		return nil
	}
	parsed, ok := intValue(firstValue(value))
	if !ok {
		return fmt.Errorf("%s %d has malformed %s", kind, id, key)
	}
	*target = parsed
	return nil
}

func setOptionalInt64(target *int64, values map[string]any, key string, id int, kind string) error {
	value, exists := values[key]
	if !exists || value == nil {
		return nil
	}
	parsed, ok := int64Value(firstValue(value))
	if !ok {
		return fmt.Errorf("%s %d has malformed %s", kind, id, key)
	}
	*target = parsed
	return nil
}

func stringValue(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func boolValue(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(value)
		return parsed
	default:
		return false
	}
}

func stableID(key string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	id := hash.Sum32()
	if id == 0 {
		return 1
	}
	return id
}

func mustInt(value string) int { number, _ := strconv.Atoi(value); return number }

func gitCommit(root string) string {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		return "uncommitted"
	}
	return strings.TrimSpace(string(output))
}

func fingerprint(commit, profile string, packetver int, maps []string, files map[string][]byte) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s\n%s\n%d\n", commit, profile, packetver)
	for _, mapName := range maps {
		_, _ = fmt.Fprintf(hash, "map:%s\n", mapName)
	}
	for _, name := range sortedStringKeysBytes(files) {
		_, _ = fmt.Fprintf(hash, "file:%s\n", name)
		_, _ = hash.Write(files[name])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func sortedStringKeysBytes(values map[string][]byte) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
