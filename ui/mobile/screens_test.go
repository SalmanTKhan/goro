package mobile

import (
	"testing"

	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
)

// screenCase is one screen builder exercised at every qualification viewport.
type screenCase struct {
	name  string
	build func(Kit, mobileui.Viewport) *Canvas
	safe  func(mobileui.Viewport) mobileui.Rect
}

func screenCases() []screenCase {
	return []screenCase{
		{
			name: "inventory",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m, l, s := inventoryFixture(nil, vp)
				return k.InventoryTree(m, l, s)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "equipment",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureEquipment("equipment-full")
				s := mobileui.InventoryInteractionState{Screen: mobileui.ScreenEquipment}
				return k.EquipmentTree(m, mobileui.LayoutEquipment(vp, mobileui.DefaultInventoryTokens(), m, s), s)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "character",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureCharacter("character-rich")
				return k.CharacterTree(m, mobileui.LayoutCharacter(vp, m))
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "social",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureSocial("social-basic")
				st := mobileui.SocialInteractionState{Tab: mobileui.SocialTabFriends}
				return k.SocialTree(m, mobileui.LayoutSocial(vp, mobileui.DefaultSocialTokens(), m, st), st)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "trade",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureTrade("trade-basic")
				st := mobileui.TradeInteractionState{}
				return k.TradeTree(m, mobileui.LayoutTrade(vp, m, st), st)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "vending",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureVending("vending-basic")
				return k.VendingTree(m, mobileui.LayoutVending(vp, len(m.Items)), -1, mobileui.VendingQuantityState{})
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "chat",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureChat("chat-basic")
				ctrl := mobileui.NewChatController(vp, nil)
				ctrl.Open(m)
				return k.ChatTree(m, ctrl.Layout, "")
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "profile",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureProfile("profile-basic")
				return k.ProfileTree(m, mobileui.LayoutMobileProfile(vp, m, false, false))
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "startup",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.StartupModel{Title: "Goro", Subtitle: "A Ragnarok client", Action: "Start offline"}
				return k.StartupTree(m, mobileui.LayoutStartup(vp))
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "shop",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureShop("shop-long")
				l := mobileui.LayoutShop(vp, len(m.Items), mobileui.ShopBuyTab)
				return k.ShopTree(m, l, mobileui.ShopBuyTab, mobileui.EconomyQuantityState{})
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "storage",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureStorage("storage-basic")
				l := mobileui.LayoutStorage(vp, len(m.Items))
				return k.StorageTree(m, l, mobileui.EconomyQuantityState{})
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "map",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureMap("map-basic")
				st := mobileui.MapInteractionState{}
				return k.MapTree(m, mobileui.LayoutMap(vp, mobileui.DefaultMapTokens(), m, st), st)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "hud",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.Fixture("monster")
				nav := mobileui.Navigation{Screen: mobileui.ScreenWorldHUD}
				return k.HUDTree(m, mobileui.LayoutHUD(vp, mobileui.DefaultTokens(), m, nav), nav)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "dialog",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureDialog("menu")
				return k.DialogTree(m, mobileui.LayoutDialog(vp, m))
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "settings",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.SettingsSurfaceForSettings(input.DefaultMobileSettings())
				st := mobileui.SurfaceInteractionState{}
				return k.SurfaceTree(m, mobileui.LayoutSurface(vp, m, st, 0), st)
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
		{
			name: "skills",
			build: func(k Kit, vp mobileui.Viewport) *Canvas {
				m := mobileui.FixtureSkills("skills-rich")
				return k.SkillsTree(m, mobileui.LayoutSkills(vp, m, 0))
			},
			safe: func(vp mobileui.Viewport) mobileui.Rect { return vp.SafeRect() },
		},
	}
}

func TestEveryScreenDrawsSomethingAtEveryViewport(t *testing.T) {
	k := testKit()
	for _, sc := range screenCases() {
		for _, vp := range qualViewports() {
			t.Run(sc.name+"/"+vp.Name, func(t *testing.T) {
				c := sc.build(k, vp.VP)
				if len(c.Children()) == 0 {
					t.Fatal("screen produced an empty tree")
				}
				canvas := layoutAndDraw(t, c)
				if len(canvas.RoundRects) == 0 && len(canvas.Rects) == 0 {
					t.Error("screen drew no surfaces")
				}
				if len(canvas.StyledTexts) == 0 && len(canvas.Texts) == 0 {
					t.Error("screen drew no text")
				}
			})
		}
	}
}

func TestEveryScreenStaysInsideTheSafeArea(t *testing.T) {
	k := testKit()
	for _, sc := range screenCases() {
		for _, vp := range qualViewports() {
			t.Run(sc.name+"/"+vp.Name, func(t *testing.T) {
				safe := sc.safe(vp.VP)
				for i, child := range sc.build(k, vp.VP).children {
					r := child.rect
					if r.Min.X < safe.X-0.5 || r.Min.Y < safe.Y-0.5 ||
						r.Max.X > safe.X+safe.W+0.5 || r.Max.Y > safe.Y+safe.H+0.5 {
						t.Errorf("child %d at %v escapes the safe area %+v", i, r, safe)
					}
				}
			})
		}
	}
}

func TestEquipmentSpritesLandOnFilledSlots(t *testing.T) {
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := mobileui.FixtureEquipment("equipment-full")
	state := mobileui.InventoryInteractionState{Screen: mobileui.ScreenEquipment}
	layout := mobileui.LayoutEquipment(vp, mobileui.DefaultInventoryTokens(), model, state)

	placements := EquipmentIconRects(model, layout)
	if len(placements) == 0 {
		t.Fatal("a fully equipped fixture produced no sprite placements")
	}
	for _, p := range placements {
		if !p.Rect.Intersects(layout.PaperDoll) {
			t.Errorf("sprite for %q sits outside the paper doll", p.Item.DisplayName)
		}
	}
}

func TestSkillSpritesAreSquareAndInsideTheList(t *testing.T) {
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := mobileui.FixtureSkills("skills-rich")
	layout := mobileui.LayoutSkills(vp, model, 0)

	placements := SkillIconRects(model, layout)
	if len(placements) == 0 {
		t.Fatal("no skill sprite placements")
	}
	for _, p := range placements {
		if p.Rect.W != p.Rect.H {
			t.Errorf("skill icon %v is not square", p.Rect)
		}
		if !p.Rect.Intersects(layout.ListViewport) {
			t.Errorf("skill icon %v is outside the list viewport", p.Rect)
		}
	}
}

func TestStatRaiseButtonsMeetTheTouchTarget(t *testing.T) {
	// The status screen's "+" controls are the smallest thing on it; they still
	// have to be hittable with a finger.
	k := testKit()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := mobileui.FixtureCharacter("character-rich")
	c := k.CharacterTree(model, mobileui.LayoutCharacter(vp, model))

	minTouch := k.Theme.Metrics.MinTouchTarget
	found := 0
	for _, child := range c.children {
		if child.rect.Width() == minTouch {
			found++
			if child.rect.Height() < minTouch-0.5 {
				t.Errorf("raise button %v is shorter than the %v touch target", child.rect, minTouch)
			}
		}
	}
	if model.StatPoints > 0 && found == 0 {
		t.Error("stat points are available but no raise buttons were placed")
	}
}

func TestSkillDetailReportsWhyASkillIsUnusable(t *testing.T) {
	k := testKit()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := mobileui.FixtureSkills("skills-rich")
	if len(model.Skills) == 0 {
		t.Skip("fixture has no skills")
	}
	model.Skills[0].DisabledReason = "Not enough SP"
	model.Selection.SelectedIndex = 0
	// Portrait only lays out a detail panel once something is selected.
	model.Selection.HasSelection = true

	canvas := layoutAndDraw(t, k.SkillsTree(model, mobileui.LayoutSkills(vp, model, 0)))
	for _, line := range drawnLines(canvas) {
		if line == "Not enough SP" {
			return
		}
	}
	t.Error("the disabled reason was not shown in the skill detail")
}
