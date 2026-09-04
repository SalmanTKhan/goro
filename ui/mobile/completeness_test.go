package mobile

import (
	"strings"
	"testing"

	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
)

// TestNoContentIsSilentlyDropped is the vertical counterpart to
// TestNoLabelIsTruncated. The same root cause — panels sized in fixed pixels
// for the old bitmap text — also manifests as a card that simply stops early:
// the status screen rendered three of six stats and gave no sign the other
// three existed.
//
// Screens that scroll are allowed to show a subset, so this checks the cards
// with fixed, fully-visible content at a tall portrait viewport, where
// everything has room.
func TestNoContentIsSilentlyDropped(t *testing.T) {
	k := testKit()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}

	t.Run("status stats and combat", func(t *testing.T) {
		model := mobileui.FixtureCharacter("character-rich")
		if len(model.Stats) == 0 {
			t.Skip("fixture has no stats")
		}
		drawn := drawnLines(layoutAndDraw(t, k.CharacterTree(model, mobileui.LayoutCharacter(vp, model))))
		for _, stat := range model.Stats {
			if !containsLine(drawn, stat.Label) {
				t.Errorf("stat %q was not drawn; the card stopped early", stat.Label)
			}
		}
		// The derived-combat card has a fixed set of figures.
		for _, label := range []string{"ATK", "DEF", "MDEF", "HIT", "FLEE", "CRIT", "ASPD"} {
			if !containsLine(drawn, label) {
				t.Errorf("combat figure %q was not drawn", label)
			}
		}
	})

	t.Run("skills list", func(t *testing.T) {
		model := mobileui.FixtureSkills("skills-rich")
		if len(model.Skills) == 0 {
			t.Skip("fixture has no skills")
		}
		layout := mobileui.LayoutSkills(vp, model, 0)
		if len(layout.Rows) < len(model.Skills) {
			t.Skipf("layout scrolls: %d rows for %d skills", len(layout.Rows), len(model.Skills))
		}
		drawn := drawnLines(layoutAndDraw(t, k.SkillsTree(model, layout)))
		for _, skill := range model.Skills {
			if !containsLine(drawn, skill.Name) {
				t.Errorf("skill %q was not drawn", skill.Name)
			}
		}
	})

	t.Run("settings rows", func(t *testing.T) {
		model := mobileui.SettingsSurfaceForSettings(input.DefaultMobileSettings())
		state := mobileui.SurfaceInteractionState{}
		layout := mobileui.LayoutSurface(vp, model, state, 0)
		if len(layout.Rows) < len(model.Items) {
			t.Skipf("layout scrolls: %d rows for %d items", len(layout.Rows), len(model.Items))
		}
		drawn := drawnLines(layoutAndDraw(t, k.SurfaceTree(model, layout, state)))
		for _, item := range model.Items {
			if item.Label == "" {
				continue
			}
			if !containsLine(drawn, item.Label) {
				t.Errorf("settings row %q was not drawn", item.Label)
			}
			// Help text must survive too. A row sized only for its label drops
			// the detail entirely rather than truncating it, which no
			// truncation check can see.
			if item.Detail != "" && !containsAnyLine(drawn, firstWords(item.Detail, 3)) {
				t.Errorf("help text for %q was not drawn: %q", item.Label, item.Detail)
			}
		}
	})

	t.Run("equipment slots", func(t *testing.T) {
		model := mobileui.FixtureEquipment("equipment-full")
		state := mobileui.InventoryInteractionState{Screen: mobileui.ScreenEquipment}
		layout := mobileui.LayoutEquipment(vp, mobileui.DefaultInventoryTokens(), model, state)
		// Every model slot must get a rectangle; an equipped item that has no
		// slot on the doll is unreachable.
		if len(layout.PaperDollSlots) < len(model.Slots) {
			t.Errorf("paper doll has %d slots for %d model slots", len(layout.PaperDollSlots), len(model.Slots))
		}
	})
}

// firstWords returns the leading n words of s, which is enough to identify a
// wrapped paragraph's opening line without depending on where it wraps.
func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
}

// containsAnyLine reports whether any drawn line starts with want.
func containsAnyLine(lines []string, want string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, want) {
			return true
		}
	}
	return false
}

// containsLine reports whether any drawn line is, or begins with, want. A line
// may carry a suffix (a level, a count) alongside the name being checked.
func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want || strings.HasPrefix(line, want) {
			return true
		}
	}
	return false
}
