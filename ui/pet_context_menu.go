package ui

import (
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/input"
)

const petContextMenuWidth = 140

type PetContextActionKind uint8

const (
	PetContextActionNone PetContextActionKind = iota
	PetContextActionInfo
	PetContextActionFeed
	PetContextActionPerformance
	PetContextActionBackToEgg
	PetContextActionUnequipAccessory
)

type PetContextAction struct {
	Kind PetContextActionKind
}

type PetContextMenu struct {
	Window
	action PetContextAction
}

func (m *PetContextMenu) Open(ctx Context, x, y int) {
	m.EnsureWindow(petContextMenuWidth, m.height())
	m.titleHeight = 0
	m.ctx = ctx
	screenW, screenH := ctx.UIScreenSize()
	x = clampWindowInt(x, windowScreenMargin, maxInt(windowScreenMargin, screenW-petContextMenuWidth-windowScreenMargin))
	y = clampWindowInt(y, windowScreenMargin, maxInt(windowScreenMargin, screenH-m.height()-windowScreenMargin))
	m.OpenAt(x, y, m.widgetTree())
	m.Publish(ctx)
}

func (m *PetContextMenu) Update(ctx Context) bool {
	m.EnsureWindow(petContextMenuWidth, m.height())
	m.titleHeight = 0
	m.ctx = ctx
	if !m.IsOpen() {
		return false
	}
	if ctx.Input != nil {
		inside := pointInRect(ctx.Input.MouseX, ctx.Input.MouseY, m.x, m.y, petContextMenuWidth, m.height())
		if ctx.Input.JustPressed(input.KeyEscape) || (!inside && (ctx.Input.MouseJustPressed(input.MouseButtonLeft) || ctx.Input.MouseJustPressed(input.MouseButtonRight))) {
			m.Close()
			return true
		}
	}
	consumed := m.Window.Update(ctx)
	m.Publish(ctx)
	return consumed
}

func (m *PetContextMenu) PopAction() PetContextAction {
	action := m.action
	m.action = PetContextAction{}
	return action
}

func (m *PetContextMenu) widgetTree() widget.Widget {
	return contextMenu(
		petContextMenuWidth,
		m.button("Check Pet Status", PetContextActionInfo),
		m.button("Feed", PetContextActionFeed),
		m.button("Performance", PetContextActionPerformance),
		m.button("Unequip Accessory", PetContextActionUnequipAccessory),
		m.button("Return to Egg", PetContextActionBackToEgg),
	)
}

func (m *PetContextMenu) button(label string, action PetContextActionKind) widget.Widget {
	return contextMenuItem(label, func() {
		m.action = PetContextAction{Kind: action}
		m.Close()
	})
}

func (m *PetContextMenu) height() int {
	return contextMenuHeight(5)
}
