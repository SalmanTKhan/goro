package mobileui

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestFoldOuterLayoutSnapshot(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fold_outer_2268x832.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Width, Height, HudSkillBarWidth, HudSkillBarX, HudSkillBarY, MinimapX, MinimapWidth, MenuX                                    int
		InventoryRailX, InventoryRailWidth, InventoryGridWidth, InventoryDetailWidth                                                  int
		EquipmentRailWidth, EquipmentSlotWidth                                                                                        int
		EconomyPanelWidth, EconomyListHeight, EconomyVisibleRows, EconomyRowHeight                                                    int
		EconomyQuantityWidth, EconomyQuantityHeight                                                                                   int
		MapPanelWidth, MapViewportWidth, MapWarpPanelWidth, MapConfirmWidth                                                           int
		LootPanelX, LootPanelY, LootPanelWidth, LootPanelHeight, LootVisibleRows                                                      int
		ChatPanelX, ChatPanelWidth, ChatComposerWidth, SettingsPanelX, SettingsPanelWidth, SettingsRows                               int
		VendingPanelX, VendingPanelWidth, VendingListWidth, VendingDetailWidth, VendingBuyWidth, VendingVisibleRows, VendingRowHeight int
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	hud := LayoutHUD(FoldOuterViewport(), DefaultTokens(), Fixture("normal"), Navigation{})
	loot := LayoutHUD(FoldOuterViewport(), DefaultTokens(), Fixture("loot-basic"), Navigation{})
	inventory := LayoutInventory(FoldOuterViewport(), DefaultInventoryTokens(), FixtureInventory("inventory-long-names"), InventoryInteractionState{Screen: ScreenInventory})
	equipment := LayoutEquipment(FoldOuterViewport(), DefaultInventoryTokens(), FixtureEquipment("equipment-full"), InventoryInteractionState{Screen: ScreenEquipment})
	chatController := NewChatController(FoldOuterViewport(), nil)
	chatController.Open(FixtureChat("chat-long"))
	settings := SettingsSurface()
	settingsLayout := LayoutSurface(FoldOuterViewport(), settings, SurfaceInteractionState{}, 0)
	shopController := NewEconomyController(FoldOuterViewport(), nil)
	shopController.OpenShop(FixtureShop("shop-long"))
	shopController.Tap(shopController.Layout.Rows[0].X+2, shopController.Layout.Rows[0].Y+2)
	vendingController := NewVendingController(FixtureVending("vending-long"), FoldOuterViewport(), nil)
	mapController := NewMapController(FixtureMap("map-basic"), FoldOuterViewport(), nil)
	if expected.Width != 2268 || expected.Height != 832 {
		t.Fatalf("fixture is not the Fold outer viewport: %+v", expected)
	}
	checks := map[string][2]int{
		"hudSkillBarWidth": {roundSnapshot(hud.SkillBar.W), expected.HudSkillBarWidth}, "hudSkillBarX": {roundSnapshot(hud.SkillBar.X), expected.HudSkillBarX}, "hudSkillBarY": {roundSnapshot(hud.SkillBar.Y), expected.HudSkillBarY},
		"minimapX": {roundSnapshot(hud.Minimap.X), expected.MinimapX}, "minimapWidth": {roundSnapshot(hud.Minimap.W), expected.MinimapWidth}, "menuX": {roundSnapshot(hud.Menu.X), expected.MenuX},
		"inventoryRailX": {roundSnapshot(inventory.Header.X), expected.InventoryRailX}, "inventoryRailWidth": {roundSnapshot(inventory.Header.W), expected.InventoryRailWidth}, "inventoryGridWidth": {roundSnapshot(inventory.GridViewport.W), expected.InventoryGridWidth}, "inventoryDetailWidth": {roundSnapshot(inventory.DetailPanel.W), expected.InventoryDetailWidth},
		"equipmentRailWidth": {roundSnapshot(equipment.Header.W), expected.EquipmentRailWidth}, "equipmentSlotWidth": {roundSnapshot(equipment.EquipmentSlots[0].Rect.W), expected.EquipmentSlotWidth},
		"economyPanelWidth": {roundSnapshot(shopController.Layout.Panel.W), expected.EconomyPanelWidth}, "economyListHeight": {roundSnapshot(shopController.Layout.ListViewport.H), expected.EconomyListHeight}, "economyVisibleRows": {len(shopController.Layout.Rows), expected.EconomyVisibleRows}, "economyRowHeight": {roundSnapshot(shopController.Layout.Rows[0].H), expected.EconomyRowHeight},
		"economyQuantityWidth": {roundSnapshot(shopController.Layout.QuantityModal.W), expected.EconomyQuantityWidth}, "economyQuantityHeight": {roundSnapshot(shopController.Layout.QuantityModal.H), expected.EconomyQuantityHeight},
		"mapPanelWidth": {roundSnapshot(mapController.Layout.WarpPanel.W), expected.MapPanelWidth}, "mapViewportWidth": {roundSnapshot(mapController.Layout.MapViewport.W), expected.MapViewportWidth}, "mapWarpPanelWidth": {roundSnapshot(mapController.Layout.WarpPanel.W), expected.MapWarpPanelWidth}, "mapConfirmWidth": {roundSnapshot(LayoutMap(FoldOuterViewport(), DefaultMapTokens(), FixtureMap("map-basic"), MapInteractionState{SelectedWarpID: 7001, ConfirmOpen: true}).ConfirmModal.W), expected.MapConfirmWidth},
		"lootPanelX": {roundSnapshot(loot.LootPanel.X), expected.LootPanelX}, "lootPanelY": {roundSnapshot(loot.LootPanel.Y), expected.LootPanelY}, "lootPanelWidth": {roundSnapshot(loot.LootPanel.W), expected.LootPanelWidth}, "lootPanelHeight": {roundSnapshot(loot.LootPanel.H), expected.LootPanelHeight}, "lootVisibleRows": {len(loot.LootRows), expected.LootVisibleRows},
		"chatPanelX": {roundSnapshot(chatController.Layout.Panel.X), expected.ChatPanelX}, "chatPanelWidth": {roundSnapshot(chatController.Layout.Panel.W), expected.ChatPanelWidth}, "chatComposerWidth": {roundSnapshot(chatController.Layout.Composer.W), expected.ChatComposerWidth}, "settingsPanelX": {roundSnapshot(settingsLayout.Panel.X), expected.SettingsPanelX}, "settingsPanelWidth": {roundSnapshot(settingsLayout.Panel.W), expected.SettingsPanelWidth}, "settingsRows": {len(settingsLayout.Rows), expected.SettingsRows},
		"vendingPanelX": {roundSnapshot(vendingController.Layout.Panel.X), expected.VendingPanelX}, "vendingPanelWidth": {roundSnapshot(vendingController.Layout.Panel.W), expected.VendingPanelWidth}, "vendingListWidth": {roundSnapshot(vendingController.Layout.ListViewport.W), expected.VendingListWidth}, "vendingDetailWidth": {roundSnapshot(vendingController.Layout.DetailPanel.W), expected.VendingDetailWidth}, "vendingBuyWidth": {roundSnapshot(vendingController.Layout.BuyButton.W), expected.VendingBuyWidth}, "vendingVisibleRows": {len(vendingController.Layout.Rows), expected.VendingVisibleRows}, "vendingRowHeight": {roundSnapshot(vendingController.Layout.Rows[0].H), expected.VendingRowHeight},
	}
	for name, values := range checks {
		if values[0] != values[1] {
			t.Errorf("%s = %d, snapshot = %d", name, values[0], values[1])
		}
	}
}

func roundSnapshot(value float32) int { return int(math.Round(float64(value))) }
