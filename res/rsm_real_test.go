package res

import (
	"fmt"
	"os"
	"testing"
)

func TestRSMRealArchiveWhenConfigured(t *testing.T) {
	grf, name := realDataArchiveFile(t, "data\\model\\¿öÇÁ¿¡·ºº£ÀÌÅÍ.rsm")
	data, err := grf.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	rsm, err := ParseRSM(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(rsm.Nodes) == 0 {
		t.Fatalf("parsed %s has no nodes", name)
	}
	t.Logf("parsed %s version=%d.%d textures=%d nodes=%d", name, rsm.VersionMajor, rsm.VersionMinor, len(rsm.Textures), len(rsm.Nodes))
}

func TestRSMRealArchiveFromRSWWhenConfigured(t *testing.T) {
	manager := realDataManager(t)
	rswName := "geffen_in.rsw"
	rswData, err := manager.ReadFile(rswName)
	if err != nil {
		t.Fatalf("read %s: %v", rswName, err)
	}
	rsw, err := ParseRSW(rswData)
	if err != nil {
		t.Fatalf("parse %s: %v", rswName, err)
	}

	seen := make(map[string]struct{})
	parsed := 0
	for _, model := range rsw.Models {
		if model.Filename == "" {
			continue
		}
		if _, ok := seen[model.Filename]; ok {
			continue
		}
		seen[model.Filename] = struct{}{}

		var data []byte
		var source string
		for _, candidate := range RSMModelCandidates(model.Filename) {
			data, err = manager.ReadFile(candidate)
			if err == nil {
				source = candidate
				break
			}
		}
		if data == nil {
			continue
		}
		rsm, err := ParseRSM(data)
		if err != nil {
			t.Fatalf("parse model %s from %s: %v", model.Filename, source, err)
		}
		if len(rsm.Nodes) == 0 {
			t.Fatalf("model %s from %s has no nodes", model.Filename, source)
		}
		parsed++
		if parsed >= 8 {
			break
		}
	}
	if parsed == 0 {
		t.Fatalf("parsed no models from %s (%d placements)", rswName, len(rsw.Models))
	}
	t.Logf("parsed %d unique RSM models from %s", parsed, rswName)
}

func TestRSMRealMapTextureResolutionWhenConfigured(t *testing.T) {
	manager := realDataManager(t)
	rswData, err := manager.ReadFile("prontera.rsw")
	if err != nil {
		rswData, err = manager.ReadFile("data/prontera.rsw")
	}
	if err != nil {
		t.Fatalf("read prontera.rsw: %v", err)
	}
	rsw, err := ParseRSW(rswData)
	if err != nil {
		t.Fatalf("parse prontera.rsw: %v", err)
	}
	seenModels := make(map[string]struct{})
	seenTextures := make(map[string]struct{})
	missing := make(map[string]struct{})
	parsedModels := 0
	for _, placement := range rsw.Models {
		if placement.Filename == "" {
			continue
		}
		if _, ok := seenModels[placement.Filename]; ok {
			continue
		}
		seenModels[placement.Filename] = struct{}{}
		var modelData []byte
		for _, candidate := range RSMModelCandidates(placement.Filename) {
			modelData, err = manager.ReadFile(candidate)
			if err == nil {
				break
			}
		}
		if modelData == nil {
			continue
		}
		rsm, parseErr := ParseRSM(modelData)
		if parseErr != nil {
			t.Fatalf("parse model %s: %v", placement.Filename, parseErr)
		}
		parsedModels++
		for _, texture := range rsm.Textures {
			if texture == "" {
				continue
			}
			if _, ok := seenTextures[texture]; ok {
				continue
			}
			seenTextures[texture] = struct{}{}
			found := false
			if _, _, _, readErr := LoadImageDetailed(manager, GroundTextureCandidates(texture)); readErr == nil {
				found = true
			}
			if !found {
				missing[texture] = struct{}{}
			}
		}
	}
	if len(missing) > 0 {
		values := make([]string, 0, len(missing))
		for texture := range missing {
			values = append(values, texture)
		}
		t.Logf("prontera RSM texture resolution: models=%d unique-textures=%d missing=%d", parsedModels, len(seenTextures), len(missing))
		for index, texture := range values {
			if index >= 32 {
				break
			}
			t.Logf("missing RSM texture[%d] %s", index, fmt.Sprintf("%q", texture))
		}
		t.Fatalf("prontera RSM texture dependencies unresolved: %d", len(missing))
	}
	t.Logf("prontera RSM texture resolution: models=%d unique-textures=%d missing=0", parsedModels, len(seenTextures))
}

func TestRSMPackedMapFaceTextureResolutionWhenConfigured(t *testing.T) {
	packPath := os.Getenv("GORO_MOBILE_PACK")
	if packPath == "" {
		t.Skip("GORO_MOBILE_PACK is not configured")
	}
	archive, err := OpenGRF(packPath)
	if err != nil {
		t.Fatalf("open mobile pack: %v", err)
	}
	defer archive.Close()
	manager := &Manager{Archives: []*GRF{archive}, PreferOptimizedTextures: true}
	rswData, err := manager.ReadFile("data/prontera.rsw")
	if err != nil {
		t.Fatalf("read packed prontera.rsw: %v", err)
	}
	rsw, err := ParseRSW(rswData)
	if err != nil {
		t.Fatalf("parse packed prontera.rsw: %v", err)
	}
	missing := make(map[string]struct{})
	seenModels := make(map[string]struct{})
	faces := 0
	for _, placement := range rsw.Models {
		if placement.Filename == "" {
			continue
		}
		if _, ok := seenModels[placement.Filename]; ok {
			continue
		}
		seenModels[placement.Filename] = struct{}{}
		var modelData []byte
		for _, candidate := range RSMModelCandidates(placement.Filename) {
			modelData, err = manager.ReadFile(candidate)
			if err == nil {
				break
			}
		}
		if modelData == nil {
			continue
		}
		rsm, parseErr := ParseRSM(modelData)
		if parseErr != nil {
			t.Fatalf("parse packed model %s: %v", placement.Filename, parseErr)
		}
		for nodeIndex := range rsm.Nodes {
			node := &rsm.Nodes[nodeIndex]
			for _, face := range node.Faces {
				if int(face.TextureID) >= len(node.TextureRefs) {
					continue
				}
				textureIndex := node.TextureRefs[face.TextureID].Index
				if textureIndex < 0 || int(textureIndex) >= len(rsm.Textures) {
					continue
				}
				name := rsm.Textures[textureIndex]
				if name == "" {
					continue
				}
				faces++
				if _, _, _, loadErr := LoadImageDetailed(manager, GroundTextureCandidates(name)); loadErr != nil {
					missing[name] = struct{}{}
				}
			}
		}
	}
	if len(missing) > 0 {
		for name := range missing {
			t.Logf("packed missing face texture %q", name)
		}
		t.Fatalf("packed RSM face texture dependencies unresolved: %d faces=%d", len(missing), faces)
	}
	t.Logf("packed RSM face texture resolution: models=%d faces=%d missing=0", len(seenModels), faces)
}
