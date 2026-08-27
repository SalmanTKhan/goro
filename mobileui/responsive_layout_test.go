package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestPortraitRoutesStayInsideSafeRail(t *testing.T) {
	viewports := []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 1080, Height: 2400, SafeTop: 48, SafeBottom: 48},
	}
	for _, viewport := range viewports {
		safe := viewport.SafeRect()
		hud := LayoutHUD(viewport, DefaultTokens(), Fixture("many-status"), Navigation{MenuOpen: true})
		for name, rect := range map[string]Rect{
			"hud-player": hud.PlayerPanel, "hud-menu": hud.Menu, "hud-minimap": hud.Minimap,
			"hud-status": hud.StatusArea, "hud-chat": hud.ChatBar, "hud-skills": hud.SkillBar,
			"hud-menu-panel": hud.MenuPanel,
		} {
			assertInside(t, name, rect, safe)
		}
		if hud.Minimap.Intersects(hud.Menu) || hud.StatusArea.Intersects(hud.Minimap) {
			t.Fatalf("portrait HUD controls overlap at %+v: minimap=%+v menu=%+v status=%+v", viewport, hud.Minimap, hud.Menu, hud.StatusArea)
		}
		for _, rect := range append(append([]Rect{}, hud.SkillSlots...), hud.MenuActionsRects()...) {
			if rect.W < 48 || rect.H < 48 {
				t.Fatalf("touch target below 48px at %+v: %+v", viewport, rect)
			}
			assertInside(t, "hud-touch", rect, safe)
		}

		character := LayoutCharacter(viewport, FixtureCharacter("character-rich"))
		assertInside(t, "character-panel", character.Panel, safe)
		assertInside(t, "character-content", character.ContentViewport, character.Panel)
		for name, rect := range map[string]Rect{"vitals": character.Vitals, "progress": character.Progress, "stats": character.Stats, "combat": character.Combat, "skills": character.SkillsButton} {
			if rect.W > 0 && rect.X != character.ContentViewport.X {
				t.Fatalf("portrait %s is not full width: %+v", name, rect)
			}
		}

		skills := LayoutSkills(viewport, FixtureSkills("skills-many"), 0)
		assertInside(t, "skills-panel", skills.Panel, safe)
		assertInside(t, "skills-list", skills.ListViewport, skills.Panel)
		if skills.Detail.W > 0 {
			assertInside(t, "skills-detail", skills.Detail, skills.Panel)
		}

		inventoryModel := FixtureInventory("inventory-long-names")
		inventoryState := InventoryInteractionState{Screen: ScreenInventory}
		inventory := LayoutInventory(viewport, DefaultInventoryTokens(), inventoryModel, inventoryState)
		assertInside(t, "inventory-header", inventory.Header, safe)
		assertInside(t, "inventory-grid", inventory.GridViewport, safe)
		if inventory.GridViewport.W > safe.W || inventory.GridViewport.Right() > safe.Right() {
			t.Fatalf("portrait inventory overflows horizontally: %+v safe=%+v", inventory.GridViewport, safe)
		}
		for _, cell := range inventory.Cells {
			assertInside(t, "inventory-cell", cell.Rect, inventory.GridViewport)
			assertTouchTarget(t, "inventory-cell", cell.Rect)
		}

		equipment := LayoutEquipment(viewport, DefaultInventoryTokens(), FixtureEquipment("equipment-full"), InventoryInteractionState{Screen: ScreenEquipment})
		assertInside(t, "equipment-header", equipment.Header, safe)
		assertInside(t, "equipment-paper-doll", equipment.PaperDoll, safe)
		for _, slot := range equipment.PaperDollSlots {
			assertInside(t, "paper-doll-slot", slot.Rect, equipment.PaperDoll)
		}
		for _, slot := range equipment.EquipmentSlots {
			assertInside(t, "equipment-slot", slot.Rect, equipment.EquipmentViewport)
			assertTouchTarget(t, "equipment-slot", slot.Rect)
		}

		mapLayout := LayoutMap(viewport, DefaultMapTokens(), FixtureMap("map-many"), MapInteractionState{})
		for name, rect := range map[string]Rect{"map-header": mapLayout.Header, "map-preview": mapLayout.MapViewport, "map-list": mapLayout.WarpPanel} {
			assertInside(t, name, rect, safe)
		}
		for _, row := range mapLayout.WarpRows {
			assertInside(t, "map-row", row.Rect, mapLayout.WarpListViewport)
			assertTouchTarget(t, "map-row", row.Rect)
		}

		dialog := LayoutDialog(viewport, FixtureDialog("dialog-long"))
		assertInside(t, "dialog-panel", dialog.Panel, safe)
		for _, option := range dialog.Options {
			assertInside(t, "dialog-option", option, dialog.Panel)
			assertTouchTarget(t, "dialog-option", option)
		}
		shop := LayoutShopScrolled(viewport, 20, ShopBuyTab, 0)
		storage := LayoutStorageScrolled(viewport, 20, 0)
		assertInside(t, "shop-panel", shop.Panel, safe)
		assertInside(t, "shop-list", shop.ListViewport, shop.Panel)
		assertInside(t, "storage-panel", storage.Panel, safe)
		assertInside(t, "storage-list", storage.ListViewport, storage.Panel)
	}
}

func TestResponsiveScrollStatesClampAtBothEnds(t *testing.T) {
	viewport := Viewport{Width: 390, Height: 844}
	character := NewCharacterSkillsController(FixtureCharacter("character-rich"), FixtureSkills("skills-many"), viewport)
	character.ScrollBy(10000)
	if character.CharacterOffset != character.CharacterLayout.ContentExtent-character.CharacterLayout.ContentViewport.H {
		t.Fatalf("character scroll did not clamp: offset=%v layout=%+v", character.CharacterOffset, character.CharacterLayout)
	}
	character.ScrollBy(-10000)
	if character.CharacterOffset != 0 {
		t.Fatalf("character scroll did not clamp at zero: %v", character.CharacterOffset)
	}

	inventory := NewInventoryController(FixtureInventory("inventory-long-names"), viewport, nil)
	inventory.ScrollBy(10000)
	if inventory.State.Scroll.Offset != inventory.State.Scroll.MaxOffset() {
		t.Fatalf("inventory scroll did not clamp: %+v", inventory.State.Scroll)
	}
	inventory.ScrollBy(-10000)
	if inventory.State.Scroll.Offset != 0 {
		t.Fatalf("inventory scroll did not clamp at zero: %+v", inventory.State.Scroll)
	}

	mapController := NewMapController(FixtureMap("map-many"), viewport, nil)
	mapController.ScrollBy(10000)
	if mapController.State.ScrollOffset != MapScrollExtent(mapController.Model, mapController.Layout, mapController.Tokens).MaxOffset() {
		t.Fatalf("map scroll did not clamp: %+v", mapController.State)
	}
}

func TestNavigationStackAndTouchOwnership(t *testing.T) {
	navigation := Navigation{}
	navigation.Open(ScreenInventory)
	navigation.OpenLayer(ScreenInventory, NavigationDetailSheet, "item:4")
	navigation.OpenLayer(ScreenInventory, NavigationConfirmation, "drop:4")
	if len(navigation.Stack) != 3 || navigation.Top().Layer != NavigationConfirmation {
		t.Fatalf("unexpected navigation stack: %+v", navigation.Stack)
	}
	if !navigation.Back() || navigation.Top().Layer != NavigationDetailSheet || navigation.Screen != ScreenInventory {
		t.Fatalf("confirmation did not pop first: %+v", navigation)
	}
	if !navigation.Back() || navigation.Screen != ScreenInventory || navigation.Top().Layer != NavigationFullScreen {
		t.Fatalf("detail did not pop second: %+v", navigation)
	}
	if !navigation.Back() || navigation.Screen != ScreenWorldHUD {
		t.Fatalf("full screen did not return to HUD: %+v", navigation)
	}

	var session TouchSession
	if !session.Begin(input.TouchPoint{ID: 7, X: 20, Y: 80}, TouchInventory, false) {
		t.Fatal("touch session did not claim inventory stream")
	}
	if _, dy, owned := session.Move(input.TouchPoint{ID: 7, X: 21, Y: 42}); !owned || dy != -38 {
		t.Fatalf("touch stream move mismatch: dy=%v owned=%v", dy, owned)
	}
	if owner, moved := session.End(7); owner != TouchInventory || !moved {
		t.Fatalf("touch session did not release correctly: owner=%v moved=%v", owner, moved)
	}
}

func assertInside(t *testing.T, name string, rect, parent Rect) {
	t.Helper()
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	if rect.X < parent.X-0.01 || rect.Y < parent.Y-0.01 || rect.Right() > parent.Right()+0.01 || rect.Bottom() > parent.Bottom()+0.01 {
		t.Fatalf("%s escapes parent: rect=%+v parent=%+v", name, rect, parent)
	}
}

func assertTouchTarget(t *testing.T, name string, rect Rect) {
	t.Helper()
	if rect.W < 48 || rect.H < 48 {
		t.Fatalf("%s below 48px touch target: %+v", name, rect)
	}
}

func (l HUDLayout) MenuActionsRects() []Rect {
	result := make([]Rect, 0, len(l.MenuActions))
	for _, action := range l.MenuActions {
		result = append(result, action.Rect)
	}
	return result
}
