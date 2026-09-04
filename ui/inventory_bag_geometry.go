package ui

// InventoryBagGeometry reports the desktop inventory window's fixed geometry.
//
// The desktop window is built from unexported constants; tooling that needs to
// reason about it — notably the mobile preview's desktop/mobile comparison —
// would otherwise have to duplicate those numbers and silently drift from them.
// Exposing them keeps one source of truth.
type InventoryBagGeometry struct {
	// Width and Height are the whole window including its title bar.
	Width, Height int
	// TitleHeight is the chrome above the content area.
	TitleHeight int
	// TabRail is the width of the vertical Item/Equip/Etc tab strip.
	TabRail int
	// TabWidth and TabHeight size one vertical tab.
	TabWidth, TabHeight int
	// Cell is the edge of one grid cell; Icon is the item sprite drawn inside it.
	Cell, Icon int
	// Columns and Rows are the fixed grid the desktop always shows, whether or
	// not the bag is full.
	Columns, Rows int
}

// DesktopInventoryBagGeometry returns the live values the desktop window uses.
func DesktopInventoryBagGeometry() InventoryBagGeometry {
	return InventoryBagGeometry{
		Width:       inventoryBagWidth,
		Height:      inventoryBagHeight,
		TitleHeight: ROWindowTitleHeight,
		TabRail:     inventoryBagTabRail,
		TabWidth:    inventoryBagTabW,
		TabHeight:   inventoryBagTabH,
		Cell:        inventoryBagCell,
		Icon:        inventoryBagIcon,
		Columns:     inventoryBagCols,
		Rows:        inventoryBagRows,
	}
}
