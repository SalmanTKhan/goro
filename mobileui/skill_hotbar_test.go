package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

func TestVisibleSkillCapByViewport(t *testing.T) {
	cases := []struct {
		name     string
		viewport Viewport
		want     int
	}{
		{"tall portrait 1080x2340", Viewport{Width: 1080, Height: 2340}, 6},
		{"tall portrait 1080x2400", Viewport{Width: 1080, Height: 2400}, 6},
		{"narrow portrait", Viewport{Width: 540, Height: 1200}, 2},
		{"landscape", Viewport{Width: 2340, Height: 1080}, 4},
		{"short portrait (near square)", Viewport{Width: 1080, Height: 1400}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layout := LayoutHUD(tc.viewport, DefaultTokens(), Fixture("skills-paged"), Navigation{})
			if layout.SkillsPerPage != tc.want {
				t.Fatalf("SkillsPerPage = %d, want %d", layout.SkillsPerPage, tc.want)
			}
			if len(layout.SkillSlots) != tc.want {
				t.Fatalf("visible SkillSlots = %d, want %d", len(layout.SkillSlots), tc.want)
			}
			for i, slot := range layout.SkillSlots {
				if slot.W < DefaultTokens().MinTouchTarget || slot.H < DefaultTokens().MinTouchTarget {
					t.Fatalf("slot %d below min touch target: %+v", i, slot)
				}
				if !layout.Safe.Contains(slot.X, slot.Y) || !layout.Safe.Contains(slot.Right(), slot.Bottom()) {
					t.Fatalf("slot %d escapes safe area: %+v (safe %+v)", i, slot, layout.Safe)
				}
			}
		})
	}
}

func TestSkillPagingButtonsAppearOnlyOnOverflow(t *testing.T) {
	viewport := Viewport{Width: 1080, Height: 2340} // perPage = 6

	few := LayoutHUD(viewport, DefaultTokens(), Fixture("skills-many"), Navigation{}) // 5 skills
	if few.SkillPagePrev.W != 0 || few.SkillPageNext.W != 0 {
		t.Fatalf("page buttons present without overflow: prev %+v next %+v", few.SkillPagePrev, few.SkillPageNext)
	}

	many := LayoutHUD(viewport, DefaultTokens(), Fixture("skills-paged"), Navigation{}) // 9 skills
	if many.SkillPagePrev.W == 0 || many.SkillPageNext.W == 0 {
		t.Fatalf("page buttons missing with overflow: prev %+v next %+v", many.SkillPagePrev, many.SkillPageNext)
	}
	if hit := many.HitTest(many.SkillPageNext.X+1, many.SkillPageNext.Y+1); hit.Control != ControlSkillPageNext {
		t.Fatalf("HitTest over next button = %v, want ControlSkillPageNext", hit.Control)
	}
	if hit := many.HitTest(many.SkillPagePrev.X+1, many.SkillPagePrev.Y+1); hit.Control != ControlSkillPagePrev {
		t.Fatalf("HitTest over prev button = %v, want ControlSkillPagePrev", hit.Control)
	}
}

func TestControllerSkillPagingAdvancesAndClamps(t *testing.T) {
	viewport := Viewport{Width: 1080, Height: 2340} // perPage = 6
	c := NewController(Fixture("skills-paged"), viewport, nil)               // 9 skills -> 2 pages
	c.Layout = LayoutHUD(viewport, c.tokens, c.Model, c.Navigation)

	next := c.Layout.SkillPageNext
	if !c.Tap(next.X+1, next.Y+1) {
		t.Fatal("tap on next page not consumed")
	}
	if c.Navigation.SkillPage != 1 {
		t.Fatalf("SkillPage = %d after one next, want 1", c.Navigation.SkillPage)
	}
	if c.Layout.SkillStart != 6 {
		t.Fatalf("SkillStart = %d on page 1, want 6", c.Layout.SkillStart)
	}
	// Already on the last page: another next is a no-op.
	c.Tap(c.Layout.SkillPageNext.X+1, c.Layout.SkillPageNext.Y+1)
	if c.Navigation.SkillPage != 1 {
		t.Fatalf("SkillPage = %d, want clamp at 1", c.Navigation.SkillPage)
	}
	prev := c.Layout.SkillPagePrev
	c.Tap(prev.X+1, prev.Y+1)
	if c.Navigation.SkillPage != 0 {
		t.Fatalf("SkillPage = %d after prev, want 0", c.Navigation.SkillPage)
	}
}

func TestProjectSessionUsesExplicitHotkeys(t *testing.T) {
	s := &session.Session{}
	s.Vitals.SP, s.Vitals.MaxSP = 100, 100
	s.Skills.List = []session.Skill{
		{ID: 5, Type: 1, Level: 3, MaxLevel: 10, SPCost: 8, Name: "SM_BASH"},
		{ID: 7, Type: 4, Level: 1, MaxLevel: 10, SPCost: 30, Name: "SM_MAGNUM"},
	}
	s.Hotkeys = session.Hotkeys{Loaded: true, Version: 1, Slots: []session.HotkeySlot{
		{Type: 1, ID: 7, Level: 2},
	}}

	model := ProjectSession(s, TargetHUDModel{})
	if len(model.Skills) != 1 {
		t.Fatalf("expected 1 hotbar slot from explicit layout, got %d", len(model.Skills))
	}
	got := model.Skills[0]
	if got.SkillID != 7 || got.Level != 2 {
		t.Fatalf("wrong slot projected: %+v", got)
	}
	if got.TargetMode != input.SkillTargetIdle {
		t.Fatalf("SM_MAGNUM (self) TargetMode = %v, want idle", got.TargetMode)
	}
	if got.IconKey != "skill-7" {
		t.Fatalf("IconKey = %q, want skill-7", got.IconKey)
	}
}

func TestProjectSessionFallsBackToLearnedSkills(t *testing.T) {
	s := &session.Session{}
	s.Vitals.SP, s.Vitals.MaxSP = 5, 100 // low SP: Bash unusable, Provoke usable
	s.Skills.List = []session.Skill{
		{ID: 5, Type: 1, Level: 1, MaxLevel: 10, SPCost: 8, Name: "SM_BASH"},
		{ID: 6, Type: 1, Level: 1, MaxLevel: 10, SPCost: 4, Name: "SM_PROVOKE"},
		{ID: 8, Type: 4, Level: 1, MaxLevel: 10, SPCost: 10, Name: "SM_ENDURE"},
	}
	// No hotkey layout at all.

	model := ProjectSession(s, TargetHUDModel{})
	if len(model.Skills) != 3 {
		t.Fatalf("expected 3 fallback slots, got %d", len(model.Skills))
	}
	if model.Skills[0].SkillID != 5 || model.Skills[0].Usable {
		t.Fatalf("slot 0 should be unusable Bash: %+v", model.Skills[0])
	}
	if !model.Skills[1].Usable {
		t.Fatalf("slot 1 (Provoke) should be usable: %+v", model.Skills[1])
	}
	if model.Skills[0].TargetMode != input.SkillTargetActor {
		t.Fatalf("Bash TargetMode = %v, want actor", model.Skills[0].TargetMode)
	}
}

func TestStarterSkillLoadoutBuildsHotbar(t *testing.T) {
	content := session.OfflineContent{Skills: map[uint16]session.OfflineSkillDef{
		5: {ID: 5, Name: "SM_BASH", MaxLevel: 10, SPCost: 8},
		6: {ID: 6, Name: "SM_PROVOKE", MaxLevel: 10, SPCost: 4, Range: 9},
		7: {ID: 7, Name: "SM_MAGNUM", MaxLevel: 10, SPCost: 30},
		8: {ID: 8, Name: "SM_ENDURE", MaxLevel: 10, SPCost: 10},
	}}
	skills, hotkeys := session.StarterSkillLoadout(content)
	if len(skills.List) != 4 || len(hotkeys.Slots) != 4 {
		t.Fatalf("expected 4 skills + 4 hotkey slots, got %d / %d", len(skills.List), len(hotkeys.Slots))
	}
	if !hotkeys.Loaded {
		t.Fatal("hotkeys should be marked Loaded")
	}

	s := &session.Session{Skills: skills, Hotkeys: hotkeys}
	s.Vitals.SP, s.Vitals.MaxSP = 100, 100
	model := ProjectSession(s, TargetHUDModel{})
	if len(model.Skills) != 4 {
		t.Fatalf("hotbar projected %d skills, want 4", len(model.Skills))
	}
}

func TestStarterSkillLoadoutSkipsMissingSkills(t *testing.T) {
	content := session.OfflineContent{Skills: map[uint16]session.OfflineSkillDef{
		5: {ID: 5, Name: "SM_BASH", MaxLevel: 10, SPCost: 8},
	}}
	skills, hotkeys := session.StarterSkillLoadout(content)
	if len(skills.List) != 1 || len(hotkeys.Slots) != 1 {
		t.Fatalf("partial pack should yield 1/1, got %d / %d", len(skills.List), len(hotkeys.Slots))
	}
}
