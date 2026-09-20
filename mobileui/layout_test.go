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
	if layout.SkillBar.W != 388 || layout.SkillBar.X != 1864 || layout.SkillBar.Y != 728 {
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
