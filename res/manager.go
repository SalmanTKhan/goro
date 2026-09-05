package res

import (
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Manager struct {
	Root       string
	ClientInfo ClientInfo
	FoundFiles []string
	Packs      []*PAK
	Archives   []*GRF
	Overlays   []AssetOverlay
	overlays   []mountedOverlay

	// PreferOptimizedTextures is enabled when a deterministic mobile pack
	// contains the optional decoded-texture closure. The source resources
	// remain available as the fallback path.
	PreferOptimizedTextures bool

	accessoryNames           map[int]string
	accessoryNamesLoaded     bool
	itemMetadata             map[int]ItemMetadata
	itemMetadataLoaded       bool
	nonPCResourceNames       map[int]string
	nonPCResourceNamesLoaded bool
	indoorRswNames           map[string]struct{}
	indoorRswNamesLoaded     bool
	cameraViewPoints         map[string]CameraViewPoint
	cameraViewPointsLoaded   bool
	msgStrings               map[int]string
	msgStringsLoaded         bool
	fogParameters            map[string]FogParameter
	fogParametersLoaded      bool
	skillResourceNames       map[int]string
	skillResourceNamesLoaded bool
	skillMaxLevels           map[int]int
	skillMaxLevelsLoaded     bool
	skillDisplayNames        map[int]string
	skillDescriptions        map[int][]string
	skillMetadataLoaded      bool
	skillTreePositions       map[int]map[int]int
	skillTreePositionsLoaded bool
	songTalks                map[SongTalkKind][]string
	songTalksLoaded          map[SongTalkKind]bool
	petTalks                 map[string]map[string]map[string][]string
	petTalksLoaded           bool
}

type CameraViewPoint struct {
	MinDistance      int
	DistanceScope    int
	InitialDistance  int
	MinLongitude     int
	MaxLongitude     int
	InitialLongitude int
	MaxLatitude      int
	MinLatitude      int
	InitialLatitude  int
}

func (v CameraViewPoint) LocksLongitude() bool {
	return v.MinLongitude == v.MaxLongitude
}

type FogParameter struct {
	Near   float64
	Far    float64
	Color  color.RGBA
	Factor float64
}

func NewManager(root string) (*Manager, error) {
	if root == "" {
		return nil, errors.New("empty root")
	}

	m := &Manager{Root: filepath.Clean(root)}
	m.scanKnownFiles()
	if _, err := m.ReadFile("mobile/optimized/manifest.json"); err == nil {
		m.PreferOptimizedTextures = true
	}
	m.ClientInfo = ClientInfo{
		Connections: []Connection{
			{Display: "Local rAthena", Address: "127.0.0.1", Port: 6900, Version: 55, LangType: 0},
		},
	}

	if source, data, ok := m.ReadFirst(clientInfoCandidates); ok {
		info, err := ParseClientInfo(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", source, err)
		}
		if len(info.Connections) > 0 {
			m.ClientInfo = info
		}
	}

	return m, nil
}

func (m *Manager) Find(name string) (string, bool) {
	normalized := normalizePath(name)
	lookup := pakLookupName(normalized)
	for _, overlay := range m.overlays {
		if overlayTombstones(overlay.config.Tombstones, lookup, true) {
			return "", false
		}
		if overlay.root != "" {
			if filePath, ok := overlayLooseFile(overlay.root, lookup); ok {
				return filePath, true
			}
		}
	}
	candidates := []string{
		filepath.Join(m.Root, normalized),
		filepath.Join(m.Root, strings.ReplaceAll(normalized, "\\", string(filepath.Separator))),
		filepath.Join(m.Root, strings.ReplaceAll(normalized, "/", string(filepath.Separator))),
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func (m *Manager) ReadFile(name string) ([]byte, error) {
	if data, found, masked, err := m.readOverlay(name, false); err != nil {
		return nil, err
	} else if found {
		return data, nil
	} else if masked {
		return nil, fmt.Errorf("resource masked by overlay: %s", name)
	}
	path, ok := m.Find(name)
	if ok {
		return os.ReadFile(path)
	}

	for _, pack := range m.Packs {
		data, err := pack.ReadFile(name)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, ErrPAKNotFound) {
			for _, match := range pack.NamesWithSuffix(name) {
				data, err := pack.ReadFile(match)
				if err == nil {
					return data, nil
				}
				if !errors.Is(err, ErrPAKNotFound) {
					return nil, err
				}
			}
			continue
		}
		return nil, err
	}
	for _, archive := range m.Archives {
		data, err := archive.ReadFile(name)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, ErrGRFNotFound) {
			for _, match := range archive.NamesWithSuffix(name) {
				data, err := archive.ReadFile(match)
				if err == nil {
					return data, nil
				}
				if !errors.Is(err, ErrGRFNotFound) {
					return nil, err
				}
			}
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

func (m *Manager) ReadFileExact(name string) ([]byte, error) {
	if data, found, masked, err := m.readOverlay(name, true); err != nil {
		return nil, err
	} else if found {
		return data, nil
	} else if masked {
		return nil, fmt.Errorf("resource masked by overlay: %s", name)
	}
	path, ok := m.Find(name)
	if ok {
		return os.ReadFile(path)
	}

	for _, pack := range m.Packs {
		data, err := pack.ReadFile(name)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, ErrPAKNotFound) {
			return nil, err
		}
	}
	for _, archive := range m.Archives {
		data, err := archive.ReadFile(name)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, ErrGRFNotFound) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

// HasResourceExact checks the active layered view without reading resource
// bytes. It intentionally does not use legacy suffix lookup.
func (m *Manager) HasResourceExact(name string) bool {
	if m == nil {
		return false
	}
	if _, found, masked, err := m.readOverlay(name, true); err == nil {
		if found {
			return true
		}
		if masked {
			return false
		}
	}
	if _, ok := m.Find(name); ok {
		return true
	}
	for _, pack := range m.Packs {
		if pack.Has(name) {
			return true
		}
	}
	for _, archive := range m.Archives {
		if archive != nil && archive.Has(name) {
			return true
		}
	}
	return false
}

// HasFileExact reports whether a resource exists at exactly name. Unlike
// ReadFile, it does not use the legacy suffix fallback for archive entries.
func (m *Manager) HasFileExact(name string) bool {
	if m == nil {
		return false
	}
	if _, ok := m.Find(name); ok {
		return true
	}
	for _, archive := range m.Archives {
		if archive != nil && archive.Has(name) {
			return true
		}
	}
	return false
}

func (m *Manager) FindFirst(names []string) (string, bool) {
	for _, name := range names {
		if path, ok := m.Find(name); ok {
			return path, true
		}
	}
	return "", false
}

func (m *Manager) ReadFirst(names []string) (string, []byte, bool) {
	for _, name := range names {
		data, err := m.ReadFile(name)
		if err == nil {
			if path, ok := m.Find(name); ok {
				return path, data, true
			}
			return name, data, true
		}
	}
	return "", nil, false
}

func (m *Manager) IsIndoorMap(mapName string) bool {
	m.loadIndoorRswNames()
	_, ok := m.indoorRswNames[normalizeRswNameForCameraTable(mapName)]
	return ok
}

func (m *Manager) CameraViewPoint(mapName string) (CameraViewPoint, bool) {
	m.loadCameraViewPoints()
	parameter, ok := m.cameraViewPoints[normalizeRswNameForCameraTable(mapName)]
	return parameter, ok
}

func (m *Manager) FogParameter(mapName string) (FogParameter, bool) {
	m.loadFogParameters()
	parameter, ok := m.fogParameters[normalizeRswNameForCameraTable(mapName)]
	return parameter, ok
}

func (m *Manager) loadIndoorRswNames() {
	if m.indoorRswNamesLoaded {
		return
	}
	m.indoorRswNamesLoaded = true
	m.indoorRswNames = make(map[string]struct{})
	data, err := m.ReadFile("data\\indoorrswtable.txt")
	if err != nil {
		data, err = m.ReadFile("data/indoorrswtable.txt")
	}
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if strings.HasPrefix(line, "//") {
			continue
		}
		if index := strings.IndexByte(line, '#'); index >= 0 {
			line = line[:index]
		}
		key := normalizeRswNameForCameraTable(line)
		if key != "" {
			m.indoorRswNames[key] = struct{}{}
		}
	}
}

func (m *Manager) loadCameraViewPoints() {
	if m.cameraViewPointsLoaded {
		return
	}
	m.cameraViewPointsLoaded = true
	m.cameraViewPoints = make(map[string]CameraViewPoint)
	data, err := m.ReadFile("data\\viewpointtable.txt")
	if err != nil {
		data, err = m.ReadFile("data/viewpointtable.txt")
	}
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Split(line, "#")
		if len(fields) < 10 {
			continue
		}
		key := normalizeRswNameForCameraTable(fields[0])
		if key == "" {
			continue
		}
		values := [9]int{}
		ok := true
		for i := range values {
			value, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				ok = false
				break
			}
			values[i] = value
		}
		if !ok {
			continue
		}
		m.cameraViewPoints[key] = CameraViewPoint{
			MinDistance:      values[0],
			DistanceScope:    values[1],
			InitialDistance:  values[2],
			MinLongitude:     values[3],
			MaxLongitude:     values[4],
			InitialLongitude: values[5],
			MaxLatitude:      values[6],
			MinLatitude:      values[7],
			InitialLatitude:  values[8],
		}
	}
}

func normalizeRswNameForCameraTable(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	if index := strings.LastIndexAny(name, `\/`); index >= 0 {
		name = name[index+1:]
	}
	name = strings.TrimRight(name, "#")
	switch {
	case strings.HasSuffix(name, ".gat"):
		name = strings.TrimSuffix(name, ".gat") + ".rsw"
	case !strings.Contains(name, "."):
		name += ".rsw"
	}
	return name
}

func (m *Manager) loadFogParameters() {
	if m.fogParametersLoaded {
		return
	}
	m.fogParametersLoaded = true
	m.fogParameters = make(map[string]FogParameter)
	data, err := m.ReadFile("data\\fogparametertable.txt")
	if err != nil {
		data, err = m.ReadFile("data/fogparametertable.txt")
	}
	if err != nil {
		return
	}

	tokens := strings.Split(string(data), "#")
	for index := 0; index+4 < len(tokens); index += 5 {
		key := normalizeRswNameForCameraTable(tokens[index])
		if key == "" {
			continue
		}
		near, nearOK := parseFogFloat(tokens[index+1])
		far, farOK := parseFogFloat(tokens[index+2])
		fogColor, colorOK := parseFogColor(tokens[index+3])
		factor, factorOK := parseFogFloat(tokens[index+4])
		if !nearOK || !farOK || !colorOK || !factorOK {
			continue
		}
		m.fogParameters[key] = FogParameter{
			Near:   near,
			Far:    far,
			Color:  fogColor,
			Factor: factor,
		}
	}
}

func parseFogFloat(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return value, err == nil
}

func parseFogColor(raw string) (color.RGBA, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return color.RGBA{}, false
	}
	value, err := strconv.ParseUint(raw, 0, 32)
	if err != nil {
		value, err = strconv.ParseUint(strings.TrimPrefix(strings.ToLower(raw), "0x"), 16, 32)
	}
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{
		R: uint8((value >> 16) & 0xff),
		G: uint8((value >> 8) & 0xff),
		B: uint8(value & 0xff),
		A: 255,
	}, true
}

func (m *Manager) scanKnownFiles() {
	for _, name := range append(clientInfoCandidates, "data.grf", "rdata.grf", "fdata.grf", "event.grf") {
		if path, ok := m.Find(name); ok {
			m.FoundFiles = append(m.FoundFiles, path)
		}
	}

	archivePaths := make([]string, 0)
	packPaths := make([]string, 0)
	seenArchives := make(map[string]struct{})
	seenPacks := make(map[string]struct{})
	_ = filepath.WalkDir(m.Root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.EqualFold(entry.Name(), ".goro-overlays") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		key := strings.ToLower(filepath.Clean(filePath))
		switch ext {
		case ".pak":
			if _, exists := seenPacks[key]; !exists {
				seenPacks[key] = struct{}{}
				packPaths = append(packPaths, filePath)
			}
		case ".grf", ".gpf":
			if _, exists := seenArchives[key]; !exists {
				seenArchives[key] = struct{}{}
				archivePaths = append(archivePaths, filePath)
			}
		}
		return nil
	})
	sort.Strings(packPaths)
	for _, path := range packPaths {
		pack, err := OpenPAK(path)
		if err != nil {
			continue
		}
		m.FoundFiles = append(m.FoundFiles, path)
		m.Packs = append(m.Packs, pack)
	}
	sort.SliceStable(archivePaths, func(i, j int) bool {
		return archivePriority(archivePaths[i]) < archivePriority(archivePaths[j])
	})
	for _, path := range archivePaths {
		archive, err := OpenGRF(path)
		if err != nil {
			continue
		}
		m.Archives = append(m.Archives, archive)
	}
}

func archivePriority(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch name {
	case "data.grf":
		return "z-data.grf:" + strings.ToLower(path)
	case "rdata.grf":
		return "y-rdata.grf:" + strings.ToLower(path)
	case "fdata.grf":
		return "x-fdata.grf:" + strings.ToLower(path)
	default:
		return name + ":" + strings.ToLower(path)
	}
}

func normalizePath(name string) string {
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, ".\\")
	return filepath.Clean(name)
}

var clientInfoCandidates = []string{
	"data/clientinfo.xml",
	"data/sclientinfo.xml",
	"clientinfo.xml",
	"sclientinfo.xml",
	"System/clientinfo.xml",
	"System/sclientinfo.xml",
}
