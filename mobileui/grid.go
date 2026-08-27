package mobileui

// GridSpec is the shared, renderer-neutral sizing contract for touch grids.
type GridSpec struct {
	MinCellWidth, MaxCellWidth, MinRowHeight float32
	HorizontalGap, VerticalGap, OuterPadding float32
	PreferredColumns, MinColumns, MaxColumns int
	AspectRatio                              float32
	FillWidth                                bool
}

type GridLayout struct {
	Cells         []Rect
	Columns, Rows int
	Content       Rect
	Scroll        ScrollState
	CellWidth     float32
	CellHeight    float32
	HorizontalGap float32
	VerticalGap   float32
	OuterPadding  float32
}

func LayoutGrid(viewport Rect, itemCount int, spec GridSpec) GridLayout {
	if viewport.W <= 0 || viewport.H <= 0 || itemCount < 0 {
		return GridLayout{}
	}
	minW := maxf(1, spec.MinCellWidth)
	maxW := maxf(minW, spec.MaxCellWidth)
	gapX, gapY := maxf(0, spec.HorizontalGap), maxf(0, spec.VerticalGap)
	innerW := maxf(0, viewport.W-2*spec.OuterPadding)
	cols := maxInt(1, int((innerW+gapX)/(minW+gapX)))
	if spec.PreferredColumns > 0 {
		cols = maxInt(cols, spec.PreferredColumns)
	}
	if spec.MinColumns > 0 {
		cols = maxInt(cols, spec.MinColumns)
	}
	if spec.MaxColumns > 0 {
		cols = minInt(cols, spec.MaxColumns)
	}
	cellW := (innerW - float32(cols-1)*gapX) / float32(cols)
	if !spec.FillWidth && cellW > maxW {
		cellW = maxW
	}
	cellW = maxf(minW, cellW)
	cellH := maxf(spec.MinRowHeight, cellW/maxf(0.01, spec.AspectRatio))
	rows := (itemCount + cols - 1) / cols
	contentW := float32(cols)*cellW + float32(cols-1)*gapX
	contentH := maxf(0, float32(rows)*cellH+float32(rows-1)*gapY)
	content := Rect{viewport.X + spec.OuterPadding, viewport.Y + spec.OuterPadding, contentW, contentH}
	result := GridLayout{
		Columns: cols, Rows: rows, Content: content,
		Scroll:    ScrollState{ViewportExtent: viewport.H, ContentExtent: contentH, RowExtent: cellH + gapY},
		CellWidth: cellW, CellHeight: cellH, HorizontalGap: gapX, VerticalGap: gapY, OuterPadding: spec.OuterPadding,
	}
	for i := 0; i < itemCount; i++ {
		col, row := i%cols, i/cols
		result.Cells = append(result.Cells, Rect{content.X + float32(col)*(cellW+gapX), content.Y + float32(row)*(cellH+gapY), cellW, cellH})
	}
	return result
}
