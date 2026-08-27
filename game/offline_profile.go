package game

import (
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

func (m *WorldMode) applyOfflineProfile(ctx client.Context, command input.PlayerCommand) bool {
	if m == nil || ctx.Offline == nil || ctx.Session == nil || ctx.World == nil || strings.TrimSpace(command.Text) == "" {
		return false
	}
	profile := session.OfflineProfile{
		ID:        command.ProfileID,
		Name:      command.Text,
		Sex:       command.ProfileSex,
		HairStyle: command.ProfileHairStyle,
		HairColor: command.ProfileHairColor,
		Stats:     command.ProfileStats,
	}
	if err := ctx.Offline.SetProfile(profile); err != nil {
		return false
	}
	if command.ProfileNew {
		return m.resetOfflineProfile(ctx, profile)
	}
	character := ctx.Session.SelectedCharacter()
	character.Name = profile.Name
	character.Hair = profile.HairStyle
	character.HairColor = profile.HairColor
	ctx.Session.Selected = character
	ctx.Session.Sex = profile.Sex
	ctx.World.Player = worldActorForSelectedCharacter(ctx)
	m.reloadPlayerSpriteView(ctx, "offline profile updated")
	return true
}

func (m *WorldMode) resetOfflineProfile(ctx client.Context, profile session.OfflineProfile) bool {
	content, ok := ctx.Offline.Content()
	if !ok {
		return false
	}
	character := session.Character{
		ID: 1, Name: profile.Name, Level: 1, JobLevel: 1, HP: 100, MaxHP: 100,
		SP: 30, MaxSP: 30, Job: 0, Hair: profile.HairStyle, HairColor: profile.HairColor,
		Money: 2500, Str: profile.Stats[0], Agi: profile.Stats[1], Vit: profile.Stats[2],
		Int: profile.Stats[3], Dex: profile.Stats[4], Luk: profile.Stats[5],
	}
	ctx.Session.SelectCharacter(character)
	ctx.Session.Sex = profile.Sex
	ctx.Session.Playing = true
	redPotion, redOK := content.Items[501]
	weapon, weaponOK := content.Items[1201]
	jellopy, jellopyOK := content.Items[909]
	if redOK && weaponOK && jellopyOK {
		ctx.Session.Inventory = session.Inventory{
			Zeny: 2500, Weight: 8*redPotion.Weight + weapon.Weight + 12*jellopy.Weight, MaxWeight: 2000,
			Items: []session.InventoryItem{
				{Index: 1, ItemID: redPotion.ID, Type: redPotion.Type, Identified: true, Amount: 8},
				{Index: 2, ItemID: weapon.ID, Type: weapon.Type, Location: db.EquipWeapon, Identified: true, Amount: 1, Equip: true, Equipped: true},
				{Index: 3, ItemID: jellopy.ID, Type: jellopy.Type, Identified: true, Amount: 12},
			},
		}
	}
	ctx.Session.Storage = session.Storage{MaxAmount: 300}
	if _, exists := content.Skills[db.SkillSMBash]; exists {
		ctx.Session.Skills, ctx.Session.Hotkeys = session.StarterSkillLoadout(content)
	}
	ctx.Session.PlayerX, ctx.Session.PlayerY, ctx.Session.PlayerDir = 78, 98, 0
	ctx.Offline.BindState(ctx.Session)
	if err := ctx.Offline.ActivateMap(ctx.Offline.MapName, ctx.Session.PlayerX, ctx.Session.PlayerY); err != nil {
		return false
	}
	ctx.World.MapName = ctx.Offline.MapName
	ctx.World.SetPlayerPosition(ctx.Session.PlayerX, ctx.Session.PlayerY, ctx.Session.PlayerDir)
	m.Enter(ctx)
	return true
}
