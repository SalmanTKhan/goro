package mobileui

import (
	"strings"

	"github.com/kivutar/goro/input"
)

type MobileProfileController struct {
	Viewport       Viewport
	Model          MobileProfileModel
	Draft          MobileProfileModel
	Open           bool
	Editor         bool
	EditingName    bool
	NewProfile     bool
	Layout         MobileProfileLayout
	Sink           input.CommandSink
	StartRequested bool
}

func NewProfileController(model MobileProfileModel, viewport Viewport, sink input.CommandSink) *MobileProfileController {
	c := &MobileProfileController{Viewport: viewport, Model: model, Sink: sink}
	c.relayout()
	return c
}

func (c *MobileProfileController) SetModel(model MobileProfileModel) {
	if c == nil {
		return
	}
	model.SexLabel = ProfileSexLabel(model.Sex)
	c.Model = model
	if !c.Editor {
		c.Draft = model
	}
	c.relayout()
}

func (c *MobileProfileController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *MobileProfileController) OpenProfile(model MobileProfileModel) {
	if c == nil {
		return
	}
	model.SexLabel = ProfileSexLabel(model.Sex)
	c.Model, c.Draft, c.Open = model, model, true
	c.StartRequested = false
	c.Editor, c.EditingName, c.NewProfile = false, false, false
	c.relayout()
}

func (c *MobileProfileController) BeginNew() {
	if c == nil {
		return
	}
	c.Draft = MobileProfileModel{Available: true, Editable: true, Name: "New Adventurer", Sex: 0, SexLabel: ProfileSexLabel(0), HairStyle: 2, Stats: [6]uint8{5, 5, 5, 5, 5, 5}}
	c.Open, c.Editor, c.NewProfile, c.EditingName = true, true, true, false
	c.relayout()
}

func (c *MobileProfileController) Close() bool {
	if c == nil || !c.Open {
		return false
	}
	c.Open, c.Editor, c.EditingName, c.NewProfile = false, false, false, false
	c.relayout()
	return true
}

// The methods below are the controller's edit vocabulary. Tap resolves a
// rectangle to one of them, and the Android host binds the same set to the
// desktop character windows' callbacks, so both input paths share one
// implementation of what an edit means.

// SetDraftName replaces the name being edited, applying the same length and
// character limits the on-screen keyboard enforces.
func (c *MobileProfileController) SetDraftName(name string) {
	if c == nil {
		return
	}
	c.Draft.Name = appendProfileName("", name)
}

// ToggleSex flips the draft between the two sexes.
func (c *MobileProfileController) ToggleSex() {
	if c == nil {
		return
	}
	c.Draft.Sex = 1 - c.Draft.Sex
	c.Draft.SexLabel = ProfileSexLabel(c.Draft.Sex)
}

// PreviousHairStyle and NextHairStyle walk the hair styles the client ships,
// wrapping at both ends.
func (c *MobileProfileController) PreviousHairStyle() {
	if c == nil {
		return
	}
	c.Draft.HairStyle--
	if c.Draft.HairStyle < profileMinHairStyle {
		c.Draft.HairStyle = profileMaxHairStyle
	}
}

func (c *MobileProfileController) NextHairStyle() {
	if c == nil {
		return
	}
	c.Draft.HairStyle++
	if c.Draft.HairStyle > profileMaxHairStyle {
		c.Draft.HairStyle = profileMinHairStyle
	}
}

// CycleHairColor advances to the next palette.
func (c *MobileProfileController) CycleHairColor() {
	if c == nil {
		return
	}
	c.Draft.HairColor = (c.Draft.HairColor + 1) % profileHairColors
}

// BumpStat moves one starter stat by delta, taking the difference from its
// paired stat so the total stays fixed. It is a no-op when either side would
// leave the legal 1..9 range.
func (c *MobileProfileController) BumpStat(index, delta int) {
	c.bumpStat(index, delta)
}

// BeginEdit opens the editor on the saved profile.
func (c *MobileProfileController) BeginEdit() {
	if c == nil || !c.Model.Editable {
		return
	}
	c.Draft, c.Editor, c.NewProfile, c.EditingName = c.Model, true, false, false
	c.relayout()
}

// CancelEdit discards the draft and returns to the read-only view.
func (c *MobileProfileController) CancelEdit() {
	if c == nil {
		return
	}
	c.Editor, c.EditingName, c.NewProfile = false, false, false
	c.Draft = c.Model
	c.relayout()
}

// SaveDraft emits the save command and, if the sink accepts it, promotes the
// draft to the saved profile. It reports whether the save was accepted.
func (c *MobileProfileController) SaveDraft() bool {
	if c == nil {
		return false
	}
	command := input.PlayerCommand{
		Kind:             input.CommandSaveOfflineProfile,
		Text:             strings.TrimSpace(c.Draft.Name),
		ProfileID:        c.Draft.ProfileID,
		ProfileSex:       c.Draft.Sex,
		ProfileHairStyle: int16(c.Draft.HairStyle),
		ProfileHairColor: uint8(c.Draft.HairColor),
		ProfileStats:     c.Draft.Stats,
		ProfileNew:       c.NewProfile,
	}
	if c.Sink == nil || !c.Sink.Emit(command) {
		return false
	}
	c.Model = c.Draft
	c.Model.Notice = "Profile saved."
	c.Editor, c.EditingName, c.NewProfile = false, false, false
	c.relayout()
	return true
}

// RequestStart asks the host to enter the world with the saved profile.
func (c *MobileProfileController) RequestStart() {
	if c == nil {
		return
	}
	c.StartRequested = true
	c.Close()
}

func (c *MobileProfileController) ConsumeTouch(point input.TouchPoint) bool {
	_ = point
	return c != nil && c.Open
}

func (c *MobileProfileController) Tap(x, y float32) bool {
	if c == nil || !c.Open {
		return false
	}
	if c.EditingName {
		for _, key := range c.Layout.Keys {
			if key.Rect.Contains(x, y) {
				if key.Key == "DONE" {
					c.EditingName = false
					c.relayout()
					return true
				}
				c.applyKey(key.Key)
				c.relayout()
				return true
			}
		}
		if c.Layout.NameField.Contains(x, y) {
			return true
		}
		if c.Layout.BackButton.Contains(x, y) {
			c.EditingName = false
			c.relayout()
			return true
		}
		return true
	}
	if c.Layout.BackButton.Contains(x, y) {
		if c.Editor {
			c.CancelEdit()
			return true
		}
		c.Close()
		return true
	}
	if !c.Editor {
		if c.Layout.ContinueButton.Contains(x, y) {
			c.RequestStart()
			return true
		}
		if c.Layout.EditButton.Contains(x, y) && c.Model.Editable {
			c.BeginEdit()
			return true
		}
		if c.Layout.NewButton.Contains(x, y) && c.Model.Editable {
			c.BeginNew()
			return true
		}
		return true
	}
	if c.Layout.NameField.Contains(x, y) {
		c.EditingName = true
		c.relayout()
		return true
	}
	if c.Layout.SexButton.Contains(x, y) {
		c.ToggleSex()
		return true
	}
	if c.Layout.HairPrev.Contains(x, y) {
		c.PreviousHairStyle()
		return true
	}
	if c.Layout.HairNext.Contains(x, y) {
		c.NextHairStyle()
		return true
	}
	if c.Layout.HairColor.Contains(x, y) {
		c.CycleHairColor()
		return true
	}
	for i := range c.Draft.Stats {
		if c.Layout.StatPlus[i].Contains(x, y) {
			c.bumpStat(i, 1)
			return true
		}
		if c.Layout.StatMinus[i].Contains(x, y) {
			c.bumpStat(i, -1)
			return true
		}
	}
	if c.Layout.SaveButton.Contains(x, y) {
		c.SaveDraft()
		return true
	}
	if c.Layout.CancelButton.Contains(x, y) {
		c.CancelEdit()
		return true
	}
	return true
}

func (c *MobileProfileController) Back() bool {
	if c == nil || !c.Open {
		return false
	}
	if c.EditingName {
		c.EditingName = false
		c.relayout()
		return true
	}
	if c.Editor {
		c.CancelEdit()
		return true
	}
	return c.Close()
}

func (c *MobileProfileController) applyKey(key string) {
	switch key {
	case "SPACE":
		c.Draft.Name = appendProfileName(c.Draft.Name, " ")
	case "⌫":
		runes := []rune(c.Draft.Name)
		if len(runes) > 0 {
			c.Draft.Name = string(runes[:len(runes)-1])
		}
	case "CLEAR":
		c.Draft.Name = ""
	default:
		c.Draft.Name = appendProfileName(c.Draft.Name, key)
	}
}

func appendProfileName(current, value string) string {
	for _, r := range value {
		if r < 32 || r == 127 {
			continue
		}
		next := current + string(r)
		if len([]byte(next)) > 23 {
			return current
		}
		current = next
	}
	return current
}

func (c *MobileProfileController) bumpStat(index, delta int) {
	if c == nil || index < 0 || index >= len(c.Draft.Stats) || delta == 0 {
		return
	}
	pair := [6]int{3, 4, 5, 0, 1, 2}[index]
	if delta > 0 && c.Draft.Stats[index] < 9 && c.Draft.Stats[pair] > 1 {
		c.Draft.Stats[index]++
		c.Draft.Stats[pair]--
	} else if delta < 0 && c.Draft.Stats[index] > 1 && c.Draft.Stats[pair] < 9 {
		c.Draft.Stats[index]--
		c.Draft.Stats[pair]++
	}
}

func (c *MobileProfileController) relayout() {
	if c == nil {
		return
	}
	if c.Editor {
		c.Layout = LayoutMobileProfile(c.Viewport, c.Draft, true, c.EditingName)
	} else {
		c.Layout = LayoutMobileProfile(c.Viewport, c.Model, false, false)
	}
}
