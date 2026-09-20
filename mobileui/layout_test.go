package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestLayoutFitsSupportedViewports(t *testing.T) {
	model := Fixture("many-status")
	viewports := []Viewport{
		{Width: 2400, Height: 1080},
		{Width: 1920, Height: 1080},
		{Width: 2560, Height: 1440},
		{Width: 1280, Height: 800},
		FoldOuterViewport(),
		{Width: 2400, Height: 1080, SafeTop: 48, SafeRight: 32, SafeBottom: 36, SafeLeft: 32},
	}
	for _, viewport := range viewports {
		layout := LayoutHUD(viewport, DefaultTokens(), model, Navigation{})
		for name, rect := range map[string]Rect{
			"player": layout.PlayerPanel, "target": layout.TargetPanel, "minimap": layout.Minimap,
			"menu": layout.Menu, "status": layout.StatusArea, "chat": layout.ChatBar, "skills": layout.SkillBar,
		} {
			if rect.W <= 0 || rect.H <= 0 {
				continue
			}
			if rect.X < layout.Safe.X || rect.Y < layout.Safe.Y || rect.Right() > layout.Safe.Right()+0.01 || rect.Bottom() > layout.Safe.Bottom()+0.01 {
				t.Fatalf("%s %+v escapes safe rect %+v at viewport %+v", name, rect, layout.Safe, viewport)
			}
		}
		targeting := LayoutHUD(viewport, DefaultTokens(), model, Navigation{Targeting: input.SkillTargetState{Mode: input.SkillTargetActor, SkillID: 100, Level: 1}})
		if targeting.CombatBanner.W == 0 || targeting.CombatCancel.W < DefaultTokens().MinTouchTarget || targeting.CombatCancel.H < DefaultTokens().MinTouchTarget {
			t.Fatalf("combat targeting controls are not touch-safe: %+v", targeting)
		}
		if targeting.CombatBanner.X < targeting.Safe.X || targeting.CombatBanner.Right() > targeting.Safe.Right() || targeting.CombatBanner.Y < targeting.Safe.Y || targeting.CombatBanner.Bottom() > targeting.Safe.Bottom() {
			t.Fatalf("combat targeting banner escapes safe rect: %+v in %+v", targeting.CombatBanner, targeting.Safe)
		}
		for i, rect := range layout.SkillSlots {
			if rect.W < DefaultTokens().MinTouchTarget || rect.H < DefaultTokens().MinTouchTarget {
				t.Fatalf("skill %d below minimum touch target: %+v", i, rect)
			}
			if i > 0 && layout.SkillSlots[i-1].Intersects(rect) {
				t.Fatalf("skill %d overlaps prior slot", i)
			}
		}
		if layout.Minimap.Intersects(layout.Menu) {
			t.Fatal("minimap overlaps menu")
		}
		if layout.TargetPanel.W > 0 && layout.TargetPanel.Intersects(layout.SkillBar) {
			t.Fatal("target panel overlaps skill bar")
		}
	}
}

func TestFoldOuterHUDAnchoring(t *testing.T) {
	layout := LayoutHUD(FoldOuterViewport(), DefaultTokens(), Fixture("normal"), Navigation{})
	if layout.SkillBar.W != 388 || layout.SkillBar.X != 1740 || layout.SkillBar.Y != 728 {
		t.Fatalf("unexpected Fold skill bar: %+v", layout.SkillBar)
	}
	if layout.Menu.X != 2188 || layout.Minimap.X != 1960 || layout.Minimap.W != 216 {
		t.Fatalf("unexpected Fold top-right controls: menu=%+v minimap=%+v", layout.Menu, layout.Minimap)
	}
	if layout.Minimap.Intersects(layout.Menu) || layout.SkillBar.W > 440 {
		t.Fatal("Fold HUD controls are not separated")
	}
	if layout.ChatButton.W < DefaultTokens().MinTouchTarget || layout.ChatBar.Bottom() > layout.Safe.Bottom() {
		t.Fatalf("Fold chat control is not touch-safe: chat=%+v button=%+v", layout.ChatBar, layout.ChatButton)
	}
	if layout.TargetPanel.Right() >= layout.SkillBar.X {
		t.Fatal("Fold target and skill controls overlap")
	}
}


func TestFoldOuterCombatHUDUsesTopCenterTargetAndThumbAction(t *testing.T) {
	layout := LayoutHUD(FoldOuterViewport(), DefaultTokens(), Fixture("monster"), Navigation{})
	if layout.TargetPanel.W <= 0 || layout.PrimaryAction.W <= 0 {
		t.Fatalf("combat HUD missing target/action: target=%+v action=%+v", layout.TargetPanel, layout.PrimaryAction)
	}
	targetCenter := layout.TargetPanel.X + layout.TargetPanel.W/2
	safeCenter := layout.Safe.X + layout.Safe.W/2
	if diff := targetCenter - safeCenter; diff < -0.01 || diff > 0.01 {
		t.Fatalf("target frame center = %.1f, safe center = %.1f", targetCenter, safeCenter)
	}
	if layout.PrimaryAction.Right() > layout.Safe.Right() || layout.PrimaryAction.Bottom() > layout.Safe.Bottom() {
		t.Fatalf("primary action escapes safe area: %+v in %+v", layout.PrimaryAction, layout.Safe)
	}
	if layout.PrimaryAction.Intersects(layout.SkillBar) {
		t.Fatalf("primary action overlaps skill bar: action=%+v skills=%+v", layout.PrimaryAction, layout.SkillBar)
	}
	if layout.TargetPanel.Intersects(layout.PlayerPanel) || layout.TargetPanel.Intersects(layout.Minimap) || layout.TargetPanel.Intersects(layout.Menu) {
		t.Fatalf("target frame overlaps top HUD: target=%+v player=%+v minimap=%+v menu=%+v", layout.TargetPanel, layout.PlayerPanel, layout.Minimap, layout.Menu)
	}
}

func TestLongTargetNameDoesNotChangeGeometry(t *testing.T) {
	short := LayoutHUD(Viewport{Width: 2400, Height: 1080}, DefaultTokens(), Fixture("monster"), Navigation{})
	long := LayoutHUD(Viewport{Width: 2400, Height: 1080}, DefaultTokens(), Fixture("long-target"), Navigation{})
	if short.TargetPanel != long.TargetPanel || short.SkillBar != long.SkillBar {
		t.Fatal("text length changed HUD geometry")
	}
}

func TestMenuLayoutFitsSafeArea(t *testing.T) {
	layout := LayoutHUD(Viewport{Width: 1280, Height: 800, SafeTop: 24, SafeRight: 24, SafeBottom: 32, SafeLeft: 24}, DefaultTokens(), Fixture("normal"), Navigation{MenuOpen: true})
	if layout.MenuPanel.X < layout.Safe.X || layout.MenuPanel.Right() > layout.Safe.Right() || layout.MenuPanel.Bottom() > layout.Safe.Bottom() {
		t.Fatalf("menu panel escapes safe area: %+v in %+v", layout.MenuPanel, layout.Safe)
	}
	if len(layout.MenuActions) != 7 {
		t.Fatalf("got %d menu actions", len(layout.MenuActions))
	}
}


func TestHUDDoesNotExposeBlankStatusControls(t *testing.T) {
	model := Fixture("normal")
	model.Statuses = []StatusEffectModel{{ID: 1}, {ID: 2}}
	layout := LayoutHUD(Viewport{Width: 1280, Height: 720}, DefaultTokens(), model, Navigation{})
	if layout.StatusArea.W != 0 || layout.StatusArea.H != 0 {
		t.Fatalf("unresolved statuses created visible HUD controls: %+v", layout.StatusArea)
	}
	if hit := layout.HitTest(layout.PlayerPanel.Right()+16, layout.PlayerPanel.Y+16); hit.Control == ControlStatus {
		t.Fatalf("unresolved status region remained a Character-screen hit target: %+v", hit)
	}
}

func TestWorldUtilityControlsAreTouchSafeAndDoNotCoverSkills(t *testing.T) {
	for _, viewport := range []Viewport{
		FoldOuterViewport(),
		{Width: 1280, Height: 800},
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
	} {
		layout := LayoutHUD(viewport, DefaultTokens(), Fixture("loot-basic"), Navigation{})
		controls := map[string]Rect{
			"sit": layout.SitAction,
			"loot": layout.LootAction,
			"emote": layout.EmoteAction,
		}
		for name, rect := range controls {
			if rect.W < DefaultTokens().MinTouchTarget || rect.H < DefaultTokens().MinTouchTarget {
				t.Fatalf("%s control below touch target viewport=%+v rect=%+v", name, viewport, rect)
			}
			if rect.X < layout.Safe.X || rect.Y < layout.Safe.Y || rect.Right() > layout.Safe.Right()+0.01 || rect.Bottom() > layout.Safe.Bottom()+0.01 {
				t.Fatalf("%s control escaped safe area viewport=%+v rect=%+v safe=%+v", name, viewport, rect, layout.Safe)
			}
			if rect.Intersects(layout.SkillBar) {
				t.Fatalf("%s control overlaps skill bar viewport=%+v control=%+v skills=%+v", name, viewport, rect, layout.SkillBar)
			}
		}
		if layout.SitAction.Intersects(layout.LootAction) || layout.LootAction.Intersects(layout.EmoteAction) || layout.SitAction.Intersects(layout.EmoteAction) {
			t.Fatalf("world utility controls overlap viewport=%+v sit=%+v loot=%+v emote=%+v", viewport, layout.SitAction, layout.LootAction, layout.EmoteAction)
		}
	}
}

func TestWorldUtilitiesRespectCombatAndMenuOwnership(t *testing.T) {
	model := Fixture("monster")
	layout := LayoutHUD(FoldOuterViewport(), DefaultTokens(), model, Navigation{})
	for name, rect := range map[string]Rect{"sit": layout.SitAction, "loot": layout.LootAction, "emote": layout.EmoteAction} {
		if rect.Intersects(layout.PrimaryAction) {
			t.Fatalf("%s overlaps primary combat action: utility=%+v primary=%+v", name, rect, layout.PrimaryAction)
		}
	}

	targeting := LayoutHUD(FoldOuterViewport(), DefaultTokens(), model, Navigation{
		Targeting: input.SkillTargetState{Mode: input.SkillTargetActor, SkillID: 100, Level: 1},
	})
	if targeting.SitAction.W != 0 || targeting.LootAction.W != 0 || targeting.EmoteAction.W != 0 {
		t.Fatalf("world utilities remained visible while targeting: sit=%+v loot=%+v emote=%+v", targeting.SitAction, targeting.LootAction, targeting.EmoteAction)
	}

	menu := LayoutHUD(FoldOuterViewport(), DefaultTokens(), model, Navigation{MenuOpen: true})
	if menu.SitAction.W != 0 || menu.LootAction.W != 0 || menu.EmoteAction.W != 0 {
		t.Fatalf("world utilities remained visible behind menu: sit=%+v loot=%+v emote=%+v", menu.SitAction, menu.LootAction, menu.EmoteAction)
	}
}

func TestQuickEmotePanelFitsSafeArea(t *testing.T) {
	model := Fixture("normal")
	for i := 0; i < 12; i++ {
		model.Emotes = append(model.Emotes, EmoteModel{ID: uint8(i), Label: "emote"})
	}
	for _, viewport := range []Viewport{
		FoldOuterViewport(),
		{Width: 1280, Height: 800, SafeTop: 24, SafeRight: 24, SafeBottom: 32, SafeLeft: 24},
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
	} {
		layout := LayoutHUD(viewport, DefaultTokens(), model, Navigation{EmoteOpen: true})
		if layout.EmotePanel.W <= 0 || len(layout.EmoteRows) != len(model.Emotes) {
			t.Fatalf("emote layout missing viewport=%+v panel=%+v rows=%d", viewport, layout.EmotePanel, len(layout.EmoteRows))
		}
		if layout.EmotePanel.X < layout.Safe.X || layout.EmotePanel.Y < layout.Safe.Y ||
			layout.EmotePanel.Right() > layout.Safe.Right()+0.01 || layout.EmotePanel.Bottom() > layout.Safe.Bottom()+0.01 {
			t.Fatalf("emote panel escaped safe area viewport=%+v panel=%+v safe=%+v", viewport, layout.EmotePanel, layout.Safe)
		}
		for i, row := range layout.EmoteRows {
			if row.W < DefaultTokens().MinTouchTarget || row.H < DefaultTokens().MinTouchTarget {
				t.Fatalf("emote %d below touch target viewport=%+v row=%+v", i, viewport, row)
			}
			if row.X < layout.EmotePanel.X || row.Y < layout.EmotePanel.Y ||
				row.Right() > layout.EmotePanel.Right()+0.01 || row.Bottom() > layout.EmotePanel.Bottom()+0.01 {
				t.Fatalf("emote %d escaped panel viewport=%+v row=%+v panel=%+v", i, viewport, row, layout.EmotePanel)
			}
		}
	}
}

func TestStatusArtworkGetsConcreteIconSlots(t *testing.T) {
	model := Fixture("many-status")
	layout := LayoutHUD(FoldOuterViewport(), DefaultTokens(), model, Navigation{})
	if layout.StatusArea.W <= 0 || len(layout.StatusSlots) == 0 {
		t.Fatalf("status artwork did not produce HUD slots: area=%+v slots=%d", layout.StatusArea, len(layout.StatusSlots))
	}
	for i, slot := range layout.StatusSlots {
		if slot.W <= 0 || slot.H <= 0 || !layout.StatusArea.Contains(slot.X, slot.Y) || slot.Right() > layout.StatusArea.Right()+0.01 {
			t.Fatalf("status slot %d escaped status area: slot=%+v area=%+v", i, slot, layout.StatusArea)
		}
	}
}


func TestCombatDockDoesNotShiftWhenTargetAppears(t *testing.T) {
	for _, viewport := range []Viewport{
		FoldOuterViewport(),
		{Width: 840, Height: 2289, SafeTop: 48, SafeBottom: 96},
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
	} {
		idleModel := Fixture("normal")
		targetModel := idleModel
		monster := Fixture("monster")
		targetModel.Target = monster.Target

		idle := LayoutHUD(viewport, DefaultTokens(), idleModel, Navigation{})
		targeted := LayoutHUD(viewport, DefaultTokens(), targetModel, Navigation{})
		if idle.SkillBar != targeted.SkillBar {
			t.Fatalf("skill bar shifted on target acquisition viewport=%+v idle=%+v target=%+v", viewport, idle.SkillBar, targeted.SkillBar)
		}
		if idle.LootAction != targeted.LootAction || idle.SitAction != targeted.SitAction || idle.EmoteAction != targeted.EmoteAction {
			t.Fatalf("utility controls shifted on target acquisition viewport=%+v idle=%+v target=%+v", viewport, idle, targeted)
		}
		if targeted.PrimaryAction.W < DefaultTokens().MinTouchTarget || targeted.PrimaryAction.H < DefaultTokens().MinTouchTarget {
			t.Fatalf("targeted primary action is not touch safe: %+v", targeted.PrimaryAction)
		}
	}
}

func TestLootIsSeparatedFromSit(t *testing.T) {
	for _, viewport := range []Viewport{
		FoldOuterViewport(),
		{Width: 840, Height: 2289, SafeTop: 48, SafeBottom: 96},
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
	} {
		layout := LayoutHUD(viewport, DefaultTokens(), Fixture("loot-basic"), Navigation{})
		if layout.LootAction.Intersects(layout.SitAction) {
			t.Fatalf("loot and sit overlap viewport=%+v loot=%+v sit=%+v", viewport, layout.LootAction, layout.SitAction)
		}
		dx := layout.LootAction.X - layout.SitAction.Right()
		if dx < 0 {
			dx = layout.SitAction.X - layout.LootAction.Right()
		}
		dy := layout.LootAction.Y - layout.SitAction.Bottom()
		if dy < 0 {
			dy = layout.SitAction.Y - layout.LootAction.Bottom()
		}
		if dx < 24 && dy < 24 {
			t.Fatalf("loot and sit are too close for distinct thumb actions viewport=%+v loot=%+v sit=%+v", viewport, layout.LootAction, layout.SitAction)
		}
	}
}
