package app

import (
	"testing"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/session"
	worldstate "github.com/kivutar/goro/world"
)

func TestEnrichMobileStatusesDerivesWeightThresholds(t *testing.T) {
	tests := []struct {
		name       string
		weight     int
		maxWeight  int
		wantStatus uint16
	}{
		{name: "50 percent", weight: 5000, maxWeight: 10000, wantStatus: db.StatusWeightover50},
		{name: "90 percent", weight: 9000, maxWeight: 10000, wantStatus: db.StatusWeightover90},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &session.Session{}
			s.Inventory.Weight = test.weight
			s.Inventory.MaxWeight = test.maxWeight
			model := mobileui.MobileHUDModel{}

			enrichMobileStatuses(&model, s)

			if len(model.Statuses) != 1 || model.Statuses[0].ID != test.wantStatus {
				t.Fatalf("weight status = %+v, want %d", model.Statuses, test.wantStatus)
			}
			if model.Statuses[0].IconKey == "" || model.Statuses[0].Name == "" {
				t.Fatalf("weight status lacks retail presentation metadata: %+v", model.Statuses[0])
			}
			if model.Statuses[0].Beneficial {
				t.Fatalf("weight status marked beneficial: %+v", model.Statuses[0])
			}
		})
	}
}

func TestMobileHUDProjectsWorldLootWithoutOfflineAuthority(t *testing.T) {
	s := &session.Session{PlayerX: 10, PlayerY: 10}
	g := &Game{
		session: s,
		world: &worldstate.World{
			Player: worldstate.Actor{ID: 1, X: 10, Y: 10},
			Items: map[uint32]worldstate.FloorItem{
				50001: {ID: 50001, ItemID: 909, Identified: true, X: 11, Y: 10, Amount: 2},
			},
		},
	}

	model := g.MobileHUDModel()
	if len(model.Loot) != 1 {
		t.Fatalf("loot count = %d, want one: %+v", len(model.Loot), model.Loot)
	}
	loot := model.Loot[0]
	if loot.DropID != 50001 || loot.ItemID != 909 || loot.Quantity != 2 || loot.Distance != 1 {
		t.Fatalf("unexpected projected loot: %+v", loot)
	}
	if !loot.PickupReady {
		t.Fatalf("adjacent drop was not marked pickup-ready: %+v", loot)
	}
	if len(model.Minimap.Markers) != 1 || model.Minimap.Markers[0].Kind != mobileui.MinimapMarkerItem {
		t.Fatalf("item marker not projected: %+v", model.Minimap.Markers)
	}
}
