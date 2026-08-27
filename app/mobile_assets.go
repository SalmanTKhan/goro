package app

import (
	"fmt"
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/res"
)

type localAssetAvailability struct {
	manager *res.Manager
}

func (a localAssetAvailability) PackState(name string) client.PackState {
	if strings.TrimSpace(name) == "" {
		return client.PackUnknown
	}
	return client.PackUnknown
}

func (a localAssetAvailability) RequireMap(mapName string) client.AssetRequirement {
	mapName = strings.ToLower(strings.TrimSpace(mapName))
	mapName = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(mapName, ".gnd"), ".rsw"), ".gat")
	requirement := client.AssetRequirement{MapName: mapName, Ready: true}
	for _, extension := range []string{"gnd", "gat", "rsw"} {
		name := fmt.Sprintf("data/%s.%s", mapName, extension)
		if !a.manager.HasResourceExact(name) {
			requirement.Ready = false
			requirement.Missing = append(requirement.Missing, name)
		}
	}
	return requirement
}

// RequestPack is deliberately a no-op in the platform-neutral game layer.
// Android's WorkManager owns the network request; its native mount command
// changes the manager view and makes RequireMap ready on a later frame.
func (a localAssetAvailability) RequestPack(name string) error     { return nil }
func (a localAssetAvailability) Subscribe(func(client.AssetEvent)) {}
