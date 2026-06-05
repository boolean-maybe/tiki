package gridlayout

// HideFields returns a copy of spec in which every anchor naming a field in
// names — both its value anchor and any matching `.caption` anchor — is
// removed, and every grid cell those anchors cover (including row-span `^`
// and column-span `--` continuations) is replaced with EmptyCell. The result
// is identical to a layout authored with `_` at those positions. The input
// spec is not mutated.
//
// Only plain field anchors (AnchorField) are hidden; single-field composite
// anchors that happen to name a hidden field are left intact (out of scope —
// this transform is used only for list-type fields, which never render as
// composites).
func HideFields(spec GridSpec, names []string) GridSpec {
	if len(names) == 0 {
		return spec
	}
	hide := make(map[string]struct{}, len(names))
	for _, n := range names {
		hide[n] = struct{}{}
	}

	// deep-copy Cells so we never mutate the caller's spec.
	cells := make([][]Cell, len(spec.Cells))
	for r := range spec.Cells {
		cells[r] = append([]Cell(nil), spec.Cells[r]...)
	}

	kept := make([]Anchor, 0, len(spec.Anchors))
	for _, a := range spec.Anchors {
		if a.Kind == AnchorField {
			if _, drop := hide[a.Name]; drop {
				blankAnchorRegion(cells, a)
				continue
			}
		}
		kept = append(kept, a)
	}

	out := spec
	out.Anchors = kept
	out.Cells = cells
	return out
}

// blankAnchorRegion sets every cell covered by the anchor's span to EmptyCell.
func blankAnchorRegion(cells [][]Cell, a Anchor) {
	for r := a.Row; r < a.Row+a.RowSpan && r < len(cells); r++ {
		for c := a.Col; c < a.Col+a.ColSpan && c < len(cells[r]); c++ {
			cells[r][c] = EmptyCell{}
		}
	}
}
