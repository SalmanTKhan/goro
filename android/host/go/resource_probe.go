package main

import (
	"fmt"
	"strings"

	"github.com/kivutar/goro/res"
)

type resourceProbe struct {
	Packs            int
	Archives         int
	Source           string
	Width            int
	Height           int
	Textures         int
	TexturesResolved int
	TexturesMissing  int
	Diagnostics      res.GNDDiagnostics
}

func probeGoroResources(root, mapName string) (resourceProbe, error) {
	manager, err := res.NewManager(root)
	if err != nil {
		return resourceProbe{}, err
	}
	base := strings.TrimSuffix(strings.TrimSuffix(mapName, ".gnd"), ".gat")
	for _, candidate := range []string{"data/" + base + ".gnd", "data\\" + base + ".gnd", base + ".gnd"} {
		data, readErr := manager.ReadFile(candidate)
		if readErr != nil {
			continue
		}
		gnd, parseErr := res.ParseGND(data)
		if parseErr != nil {
			return resourceProbe{}, fmt.Errorf("parse %s: %w", candidate, parseErr)
		}
		probe := resourceProbe{Packs: len(manager.Packs), Archives: len(manager.Archives), Source: candidate, Width: gnd.Width, Height: gnd.Height, Textures: len(gnd.Textures), Diagnostics: res.AnalyzeGND(gnd, manager)}
		probe.TexturesResolved = probe.Diagnostics.TexturesResolved
		probe.TexturesMissing = probe.Diagnostics.TexturesMissing
		return probe, nil
	}
	return resourceProbe{Packs: len(manager.Packs), Archives: len(manager.Archives)}, fmt.Errorf("map %q not found under %s", mapName, root)
}
