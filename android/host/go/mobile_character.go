//go:build android && cgo

package main

import (
	"image"
	"math"

	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/session"
	gameui "github.com/kivutar/goro/ui"
	"github.com/kivutar/goro/ui/rotheme"
)

// characterWindows runs the desktop character select and create windows inside
// the mobile presentation.
//
// Every other mobile screen is built from the ui/mobile widget layer, but these
// two are the places where the desktop windows carry markedly more than a
// mobile redesign did: the slot page with its appearance previews, and the
// hexagonal stat graph. Rather than reimplement that art, this host runs the
// real windows at their authored desktop size and scales the rasterized result
// up to the phone. Touch targets grow with the art, so a 36px stat button
// becomes comfortably tappable without any layout being duplicated.
//
// The windows are pure data-in/callbacks-out (ui/character_select_window.go,
// ui/character_create_window.go); nothing here depends on LoginMode, which is
// where the online client drives them from.
type characterWindows struct {
	ui  *uiapp.App
	win *androidUIWindow
	// manager composes the published windows into the app's root. A bare
	// Window.Widget() is an overlay with no event routing of its own — the
	// manager is what hit-tests a pointer against the open windows — so the
	// windows are published here exactly as they are on the desktop.
	manager *gameui.Manager

	selectWindow *gameui.CharacterSelectWindow
	createWindow *gameui.CharacterCreateWindow

	baked *render.Image
	// natW/natH are the surface the active window is rasterized onto: the
	// window's authored size plus the margin its placement leaves.
	natW, natH int
	// scale, offX and offY map that surface onto the viewport, and invert to
	// map a touch back into it.
	scale      float32
	offX, offY float32

	// preview caches the baked appearance sprite. Rebaking it would reload the
	// humanoid sprite set, which is far too expensive to do per frame.
	preview    image.Image
	previewKey string

	// selected is the slot the select window highlights. The window activates a
	// slot only on a second tap, the first selecting it, so this has to be real
	// state rather than pinned to the profile's slot.
	selected int

	editor bool
	key    string
	dirty  bool
}

const (
	// characterPreviewW and characterPreviewH match the slot art the select
	// window reserves, which the create window's preview panel also fits.
	characterPreviewW = 139
	characterPreviewH = 144
	// characterSlotCount is one page of slots. Offline storage holds a single
	// profile, so the extra slots exist only as the route into the editor.
	characterSlotCount = 3
)

func newCharacterWindows() *characterWindows {
	theme := rotheme.Default.AsTheme()
	// Transparent ground: the window floats over the world like a desktop one.
	theme.Colors.Background = widget.RGBA8(0, 0, 0, 0)
	win := &androidUIWindow{width: 1, height: 1}
	h := &characterWindows{
		ui: uiapp.New(
			uiapp.WithWindowProvider(win),
			uiapp.WithTheme(theme),
			uiapp.WithRenderMode(uiapp.RenderModeHostManaged),
		),
		win:     win,
		manager: gameui.NewManager(),
		dirty:   true,
	}
	h.manager.SetUIApp(h)
	return h
}

// characterWindows is the client.UIApp its own manager publishes into, which is
// how the composed root reaches the widget app.
func (h *characterWindows) SetUIRoot(root widget.Widget) {
	h.ui.SetRoot(root)
	h.dirty = true
}

func (h *characterWindows) Frame() { h.ui.Frame() }

// WidgetContext is what ui.Window mounts a replaced content tree against. Its
// absence is silent and total: SetContent skips MountTree, the new tree never
// gets bounds, and the next raster comes out empty.
func (h *characterWindows) WidgetContext() widget.Context { return h.ui.Window().Context() }

func (h *characterWindows) Cursor() widget.CursorType    { return h.ui.Window().Context().Cursor() }
func (h *characterWindows) HoveredWidget() widget.Widget { return h.ui.Window().HoveredWidget() }

// active reports whether the character screen owns the display.
func (h *characterWindows) active(p *mobilePresentation) bool {
	return h != nil && p != nil && p.profileController != nil && p.profileController.Open
}

// windowContext is the context the windows resolve their placement against.
// Sizing it to the surface puts the window at the placement margin, so the
// raster is exactly the window plus a uniform border.
func (h *characterWindows) windowContext() client.Context {
	return client.Context{UIWidth: h.natW, UIHeight: h.natH, UIManager: h.manager, UIApp: h}
}

// sync rebuilds the window matching the controller's current mode.
func (h *characterWindows) sync(p *mobilePresentation) {
	c := p.profileController
	margin := gameui.CharacterWindowScreenMargin()
	if c.Editor {
		w, hgt := gameui.CharacterCreateWindowSize()
		h.natW, h.natH = w+2*margin, hgt+2*margin
		ctx := h.windowContext()
		opts := h.createOptions(p)
		if h.createWindow == nil {
			h.createWindow = gameui.NewCharacterCreateWindow(ctx, opts, h.createCallbacks(p))
		} else {
			h.createWindow.SetOptions(ctx, opts)
		}
		if !h.editor {
			// Only one of the two is ever on screen.
			h.selectWindow.Unpublish(ctx)
		}
		h.createWindow.Publish(ctx)
		h.editor = true
		return
	}
	w, hgt := gameui.CharacterSelectWindowSize()
	h.natW, h.natH = w+2*margin, hgt+2*margin
	ctx := h.windowContext()
	opts := h.selectOptions(p)
	if h.selectWindow == nil {
		h.selectWindow = gameui.NewCharacterSelectWindow(ctx, opts, h.selectCallbacks(p))
	} else {
		h.selectWindow.SetOptions(ctx, opts)
	}
	if h.editor {
		h.createWindow.Unpublish(ctx)
	}
	h.selectWindow.Publish(ctx)
	h.editor = false
}

// previewImage bakes the appearance sprite for the model being shown, reusing
// the last bake while the appearance is unchanged.
func (h *characterWindows) previewImage(p *mobilePresentation, model mobileui.MobileProfileModel) image.Image {
	key := profileAppearanceKey(model)
	// The key is recorded even when the bake fails, so an appearance whose
	// sprite set is missing is attempted once rather than reloading every frame.
	if key == h.previewKey {
		return h.preview
	}
	h.preview = p.game.MobileProfilePreviewImage(model, characterPreviewW, characterPreviewH)
	h.previewKey = key
	return h.preview
}

// profileAppearanceKey names everything the sprite depends on.
func profileAppearanceKey(model mobileui.MobileProfileModel) string {
	return string(rune(model.Sex)) + "|" + itoa(model.HairStyle) + "|" + itoa(model.HairColor)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	var digits [12]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

// selectOptions projects the single offline profile onto the select window's
// slot model. Offline storage holds exactly one profile (session.OfflineProfile),
// so at most slot 0 is ever occupied.
func (h *characterWindows) selectOptions(p *mobilePresentation) gameui.CharacterSelectWindowOptions {
	c := p.profileController
	opts := gameui.CharacterSelectWindowOptions{SelectedSlot: h.selected, MaxSlots: characterSlotCount}
	if !c.Model.Available {
		return opts
	}
	opts.Characters = []session.Character{{
		Slot:      0,
		Name:      c.Model.Name,
		Hair:      int16(c.Model.HairStyle),
		HairColor: uint8(c.Model.HairColor),
		Str:       c.Model.Stats[0],
		Agi:       c.Model.Stats[1],
		Vit:       c.Model.Stats[2],
		Int:       c.Model.Stats[3],
		Dex:       c.Model.Stats[4],
		Luk:       c.Model.Stats[5],
		Level:     1,
		JobLevel:  1,
	}}
	if preview := h.previewImage(p, c.Model); preview != nil {
		opts.PreviewImages = map[int]image.Image{0: preview}
	}
	return opts
}

func (h *characterWindows) selectCallbacks(p *mobilePresentation) gameui.CharacterSelectWindowCallbacks {
	c := p.profileController
	// enter starts the world with the saved profile.
	enter := func() {
		if c.Model.Available {
			c.RequestStart()
		}
	}
	// edit opens the create window. Offline storage holds a single profile
	// (session.OfflineProfile), so making a character always writes over the one
	// that exists; seeding the draft from it means the current appearance and
	// stats carry into the editor rather than being reset. That makes the empty
	// slots' Create the offline client's edit path, which matters because the
	// select window disables Make while an occupied slot is selected.
	edit := func() {
		if c.Model.Available {
			c.BeginEdit()
			return
		}
		c.BeginNew()
	}
	// occupied reports whether a slot holds the offline profile. Only slot 0
	// ever does.
	occupied := func(slot int) bool { return slot == 0 && c.Model.Available }
	return gameui.CharacterSelectWindowCallbacks{
		OnSelectSlot: func(slot int) {
			h.selected = clampSlot(slot)
			h.dirty = true
		},
		OnActivateSlot: func(slot int) {
			h.selected = clampSlot(slot)
			if occupied(h.selected) {
				enter()
				return
			}
			edit()
		},
		OnPreviousSlot: func() {
			h.selected = clampSlot(h.selected - 1)
			h.dirty = true
		},
		OnNextSlot: func() {
			h.selected = clampSlot(h.selected + 1)
			h.dirty = true
		},
		OnMake: edit,
		OnOK: func() {
			if occupied(h.selected) {
				enter()
				return
			}
			edit()
		},
		// Cancel is the screen's escape. The mobile presentation treats leaving
		// the character screen as entering the already-loaded world, which is
		// what its BACK action has always done.
		OnCancel: enter,
		// Offline profiles have no delete path — there is no packet and no
		// second slot to fall back to — so the button reports rather than lies.
		OnDelete: func() {
			c.Model.Notice = "Offline profiles cannot be deleted."
		},
	}
}

func (h *characterWindows) createOptions(p *mobilePresentation) gameui.CharacterCreateWindowOptions {
	c := p.profileController
	opts := gameui.CharacterCreateWindowOptions{
		Name:     c.Draft.Name,
		Stats:    c.Draft.Stats,
		SexLabel: mobileui.ProfileSexLabel(c.Draft.Sex),
	}
	opts.Preview = h.previewImage(p, c.Draft)
	return opts
}

func (h *characterWindows) createCallbacks(p *mobilePresentation) gameui.CharacterCreateWindowCallbacks {
	c := p.profileController
	return gameui.CharacterCreateWindowCallbacks{
		OnNameChange: c.SetDraftName,
		OnSubmit: func() {
			if c.SaveDraft() {
				// The editor is usually reached from an empty slot, but what it
				// saves is the profile in slot 0. Select that, so returning to
				// the slots shows the character just made rather than the empty
				// slot it was started from.
				h.selected = 0
				return
			}
			// The editor stays open on a rejected save, which is the only signal
			// this window can give: it has no notice surface of its own. Log it
			// so a rejection is diagnosable rather than looking like a dead
			// button.
			androidLog("stage=mobile-character save rejected")
		},
		OnCancel:    c.CancelEdit,
		OnHairPrev:  c.PreviousHairStyle,
		OnHairNext:  c.NextHairStyle,
		OnHairColor: c.CycleHairColor,
		// The stat graph's buttons add a point; the controller takes the
		// matching point off the paired stat, which is the offline rule.
		OnStat: func(stat int) { c.BumpStat(stat, 1) },
		// Offline owns the appearance outright, so unlike the online client the
		// sex is editable here.
		OnToggleSex: c.ToggleSex,
	}
}

// stateKey covers everything the rendered window depends on, so the raster is
// rebuilt exactly when it changes.
func (h *characterWindows) stateKey(p *mobilePresentation) string {
	c := p.profileController
	model := c.Model
	if c.Editor {
		model = c.Draft
	}
	key := "s"
	if c.Editor {
		key = "e"
	}
	key += "|" + itoa(h.selected) + "|" + model.Name + "|" + profileAppearanceKey(model) + "|" + model.Notice
	for _, stat := range model.Stats {
		key += "|" + itoa(int(stat))
	}
	if model.Available {
		key += "|a"
	}
	return key
}

// draw renders the active character window and blits it scaled into the
// viewport. It reports false when the character screen is not showing.
func (h *characterWindows) draw(p *mobilePresentation, frame *render.Frame) bool {
	if !h.active(p) || frame == nil {
		return false
	}
	vpW, vpH := int(p.viewport.Width), int(p.viewport.Height)
	if vpW <= 0 || vpH <= 0 {
		return false
	}
	h.sync(p)
	if key := h.stateKey(p); key != h.key {
		h.key, h.dirty = key, true
	}
	h.win.width, h.win.height = h.natW, h.natH
	if h.dirty || h.baked == nil {
		// The manager has already pushed the composed root through SetUIRoot;
		// Frame runs the layout pass that gives it bounds. Without it the raster
		// succeeds but is empty.
		h.ui.Frame()
		image, drawn, err := render.RasterizeUI(h.ui, h.natW, h.natH, h.baked)
		if err != nil {
			androidLog("stage=mobile-character raster-error=" + err.Error())
			return false
		}
		if drawn {
			h.baked = image
		}
		h.ui.Window().ClearAnimationFrame()
		h.dirty = false
	}
	if h.baked == nil {
		return false
	}

	// Fit the authored window into the viewport, never past 1:1 per axis being
	// exceeded by the other, and centre what is left over.
	h.scale = float32(math.Min(float64(vpW)/float64(h.natW), float64(vpH)/float64(h.natH)))
	if h.scale <= 0 {
		return false
	}
	h.offX = (float32(vpW) - float32(h.natW)*h.scale) / 2
	h.offY = (float32(vpH) - float32(h.natH)*h.scale) / 2

	// A scrim, so the world behind the window does not compete with it.
	render.DrawRect(frame, 0, 0, float64(vpW), float64(vpH), mobileColors().shadow)

	var opts render.DrawImageOptions
	opts.GeoM.Scale(float64(h.scale), float64(h.scale))
	opts.GeoM.Translate(float64(h.offX), float64(h.offY))
	opts.Filter = render.FilterNearest
	frame.DrawImage(h.baked, &opts)
	return true
}

// windowPoint maps a viewport point into the rasterized window surface.
func (h *characterWindows) windowPoint(x, y int) (float32, float32, bool) {
	if h == nil || h.scale <= 0 {
		return 0, 0, false
	}
	wx := (float32(x) - h.offX) / h.scale
	wy := (float32(y) - h.offY) / h.scale
	return wx, wy, wx >= 0 && wy >= 0 && wx < float32(h.natW) && wy < float32(h.natH)
}

// tap delivers a tap to the active window. The mobile presentation resolves a
// touch to a single tap, so the press and release are synthesized together;
// that is enough to drive buttons and to focus the name field.
func (h *characterWindows) tap(p *mobilePresentation, x, y int) bool {
	if !h.active(p) {
		return false
	}
	wx, wy, inside := h.windowPoint(x, y)
	if !inside {
		return false
	}
	at := geometry.Pt(wx, wy)
	// The move first, so the widget under the finger resolves as hovered before
	// it is pressed. Laying out between these would be wrong: the raster in Draw
	// runs the frame, and consuming it here leaves that pass nothing to draw.
	for _, e := range []*event.MouseEvent{
		event.NewMouseEvent(event.MouseMove, event.ButtonNone, 0, at, at, 0),
		event.NewMouseEvent(event.MousePress, event.ButtonLeft, event.ButtonStateLeft, at, at, 0),
		event.NewMouseEvent(event.MouseRelease, event.ButtonLeft, 0, at, at, 0),
	} {
		h.ui.HandleEvent(e)
	}
	h.dirty = true
	return true
}

// clampSlot keeps a slot on the single page the offline client shows.
func clampSlot(slot int) int {
	if slot < 0 {
		return 0
	}
	if slot >= characterSlotCount {
		return characterSlotCount - 1
	}
	return slot
}

// Invalidate forces the next draw to re-rasterize.
func (h *characterWindows) Invalidate() {
	if h != nil {
		h.dirty = true
	}
}

// TextInputActive reports whether the window has a focused text field, so the
// host can raise the platform IME instead of the mobile on-screen keyboard.
func (h *characterWindows) TextInputActive() bool {
	if h == nil || h.ui == nil {
		return false
	}
	_, ok := h.ui.Window().Context().FocusedWidget().(*textfield.Widget)
	return ok
}
