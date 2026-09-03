package game

import (
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/network"
)

func (m *WorldMode) applyAutoSpellList(ctx client.Context, list network.AutoSpellList) {
	if len(list.SkillIDs) == 0 {
		m.sendAutoSpellSelection(ctx, 0)
		return
	}
	m.ui.autoSpellWindow.OpenList(ctx, list, m)
	glog.Debugf("auto spell choices skills=%v", list.SkillIDs)
}

func (m *WorldMode) updateAutoSpellWindow(ctx client.Context) bool {
	consumed := m.ui.autoSpellWindow.Update(ctx)
	if skillID, ok := m.ui.autoSpellWindow.PopSelection(); ok {
		m.sendAutoSpellSelection(ctx, skillID)
		return true
	}
	return consumed
}

func (m *WorldMode) sendAutoSpellSelection(ctx client.Context, skillID uint16) {
	if ctx.Network == nil {
		m.ui.console.AddErrorMessage("Auto Spell selection failed: not connected.")
		return
	}
	if err := ctx.Network.SendSelectAutoSpell(skillID); err != nil {
		m.ui.console.AddErrorMessage("Auto Spell selection failed.")
		return
	}
	glog.Debugf("auto spell selected skill=%d", skillID)
}
