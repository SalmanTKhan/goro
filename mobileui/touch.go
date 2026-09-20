package mobileui

import "github.com/kivutar/goro/input"

// DefaultTouchSlop is the logical distance a pointer may drift before a tap
// becomes a drag. Real fingers move a few pixels even when the player intends
// to tap; treating any non-zero motion as a drag made compact HUD controls and
// list rows unnecessarily difficult to activate.
const DefaultTouchSlop float32 = 12

// TouchOwner identifies the screen-level consumer that owns a drag. Once a
// drag is claimed, sibling screens and the world cannot also scroll from the
// same pointer stream.
type TouchOwner uint8

const (
	TouchUnclaimed TouchOwner = iota
	TouchHUD
	TouchCharacter
	TouchSkills
	TouchInventory
	TouchEquipment
	TouchMap
	TouchEconomy
	TouchDialog
	TouchSocial
	TouchProfile
	TouchStartup
	TouchOnline
)

type TouchSession struct {
	Owner  TouchOwner
	ID     input.TouchID
	Active bool
	Start  input.TouchPoint
	Last   input.TouchPoint
	Moved  bool
}

func (s *TouchSession) Begin(point input.TouchPoint, owner TouchOwner, modal bool) bool {
	if s == nil || s.Active {
		return false
	}
	// A modal surface must explicitly claim the stream. This prevents a
	// background list from moving while a quantity/confirmation sheet is open.
	if modal && owner == TouchUnclaimed {
		return false
	}
	*s = TouchSession{Owner: owner, ID: point.ID, Active: true, Start: point, Last: point}
	return true
}

func (s *TouchSession) Move(point input.TouchPoint) (dx, dy float32, owned bool) {
	if s == nil || !s.Active || s.ID != point.ID {
		return 0, 0, false
	}
	previous := s.Last
	s.Last = point
	if !s.Moved {
		dx = float32(point.X - s.Start.X)
		dy = float32(point.Y - s.Start.Y)
		if dx*dx+dy*dy < DefaultTouchSlop*DefaultTouchSlop {
			return 0, 0, true
		}
		s.Moved = true
		return dx, dy, true
	}
	return float32(point.X - previous.X), float32(point.Y - previous.Y), true
}

func (s *TouchSession) End(id input.TouchID) (owner TouchOwner, moved bool) {
	if s == nil || !s.Active || s.ID != id {
		return TouchUnclaimed, false
	}
	owner, moved = s.Owner, s.Moved
	*s = TouchSession{}
	return owner, moved
}

func (s *TouchSession) Cancel() {
	if s != nil {
		*s = TouchSession{}
	}
}
