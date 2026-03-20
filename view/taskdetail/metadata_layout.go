package taskdetail

// metadata_layout.go contains the pure responsive layout algorithm for the
// task detail metadata box. It has no tview or config dependencies — just
// integers in, plan out — so it can be tested and swapped independently.

const maxLeftSideGap = 8
const minBridgeGap = 3
const maxBridgeGap = 12

// Layout algorithm overview:
//
// Sections are divided into two groups: core (Status, People, Due) and optional
// (Tags, DependsOn, Blocks). Optional sections live on the right side and are
// shed when the terminal is too narrow, in order: Tags → Blocks → DependsOn.
//
// Gap distribution works differently for left vs right:
//   - Left-side gaps expand equally, each capped at maxLeftSideGap.
//   - A "bridge gap" separates the last core section from the first optional
//     section, bounded by minBridgeGap..maxBridgeGap.
//   - Right-side sections expand their widths to absorb remaining space.
//
// When only core sections remain (no right side), gaps are capped equally at
// maxLeftSideGap and any leftover is unallocated — sections stay left-aligned
// rather than stretching across the full width.

// SectionID identifies a metadata section in left-to-right display order.
type SectionID int

const (
	SectionStatusGroup SectionID = iota // status, type, priority, points
	SectionPeopleGroup                  // assignee, author, created, updated
	SectionDueGroup                     // due, recurrence
	SectionTags
	SectionDependsOn
	SectionBlocks
)

// rightSideSections are hidden in this order when space is tight.
var rightSideHideOrder = []SectionID{SectionTags, SectionBlocks, SectionDependsOn}

// SectionInput describes one candidate section for the layout.
type SectionInput struct {
	ID         SectionID
	Width      int  // minimum width this section needs
	HasContent bool // false = skip entirely (optional section with no data)
}

// PlannedSection is a section that made it into the final layout.
type PlannedSection struct {
	ID    SectionID
	Width int
}

// LayoutPlan is the output of the layout algorithm.
type LayoutPlan struct {
	Sections []PlannedSection
	Gaps     []int // len = len(Sections) - 1; gap[i] is between Sections[i] and Sections[i+1]
}

// isRightSide returns true for sections 3-5 (Tags, DependsOn, Blocks).
func isRightSide(id SectionID) bool {
	return id >= SectionTags
}

// CalculateMetadataLayout computes which sections to show and how to distribute
// horizontal space among them.
//
// Algorithm:
//  1. Include only sections that have content.
//  2. Check whether all included sections + 1-char minimum gaps fit.
//  3. If not, hide right-side sections in order: Tags → Blocks → DependsOn.
//  4. Distribute remaining free space evenly across gaps.
//  5. Any remainder goes to the "bridge gap" (between last left-side and first
//     right-side section). If no right-side sections remain, remainder goes to
//     the last gap.
func CalculateMetadataLayout(availableWidth int, sections []SectionInput) LayoutPlan {
	// step 1: filter to sections that have content
	active := filterActive(sections)
	if len(active) == 0 {
		return LayoutPlan{}
	}

	// step 2-3: shed right-side sections until everything fits
	active = shedUntilFit(active, availableWidth)
	if len(active) == 0 {
		return LayoutPlan{}
	}
	if len(active) == 1 {
		return LayoutPlan{
			Sections: []PlannedSection{{ID: active[0].ID, Width: active[0].Width}},
		}
	}

	// step 4-5: distribute free space
	return distributeSpace(active, availableWidth)
}

// filterActive keeps only sections whose HasContent is true.
func filterActive(sections []SectionInput) []SectionInput {
	var out []SectionInput
	for _, s := range sections {
		if s.HasContent {
			out = append(out, s)
		}
	}
	return out
}

// totalMinWidth returns the minimum width required for sections + 1-char gaps.
func totalMinWidth(sections []SectionInput) int {
	w := 0
	for _, s := range sections {
		w += s.Width
	}
	if len(sections) > 1 {
		w += len(sections) - 1 // 1-char gap between each
	}
	return w
}

// shedUntilFit removes right-side sections in hide order until the remaining
// sections fit within availableWidth.
func shedUntilFit(active []SectionInput, availableWidth int) []SectionInput {
	for _, hideID := range rightSideHideOrder {
		if totalMinWidth(active) <= availableWidth {
			return active
		}
		active = removeSection(active, hideID)
	}
	return active
}

// removeSection returns a new slice with the given section ID removed.
func removeSection(sections []SectionInput, id SectionID) []SectionInput {
	var out []SectionInput
	for _, s := range sections {
		if s.ID != id {
			out = append(out, s)
		}
	}
	return out
}

// distributeSpace assigns widths and gaps given sections that are known to fit.
//
//  1. All sections start at their declared min width, all gaps at 1.
//  2. Remaining free space is split: left-side gaps expand (capped at
//     maxLeftSideGap); then right-side sections expand equally.
//  3. Any leftover goes to the bridge gap. When no right-side sections exist,
//     leftover is unallocated (sections stay left-aligned).
func distributeSpace(active []SectionInput, availableWidth int) LayoutPlan {
	numGaps := len(active) - 1
	bridgeIdx := findBridgeGap(active)

	planned := make([]PlannedSection, len(active))
	for i, s := range active {
		planned[i] = PlannedSection{ID: s.ID, Width: s.Width}
	}

	gaps := make([]int, numGaps)
	for i := range gaps {
		gaps[i] = 1
	}

	// classify sections
	var rightIndices []int
	for i, s := range active {
		if isRightSide(s.ID) {
			rightIndices = append(rightIndices, i)
		}
	}

	totalUsed := func() int {
		w := 0
		for _, p := range planned {
			w += p.Width
		}
		for _, g := range gaps {
			w += g
		}
		return w
	}

	free := availableWidth - totalUsed()
	if free <= 0 {
		return LayoutPlan{Sections: planned, Gaps: gaps}
	}

	// step 0: reserve minimum bridge gap before left-gap expansion
	if bridgeIdx >= 0 && free >= (minBridgeGap-1) {
		gaps[bridgeIdx] = minBridgeGap
		free = availableWidth - totalUsed()
	}

	// step 1: expand left-side gaps equally (each capped at maxLeftSideGap)
	leftGapCount := bridgeIdx
	if bridgeIdx < 0 {
		leftGapCount = numGaps
	}
	if leftGapCount > 0 {
		perGap := min(free/leftGapCount, maxLeftSideGap-1)
		for i := 0; i < leftGapCount; i++ {
			gaps[i] += perGap
		}
		free = availableWidth - totalUsed()
	}

	// step 2: expand right-side sections equally with remaining space
	if len(rightIndices) > 0 && free > 0 {
		perRight := free / len(rightIndices)
		remainder := free % len(rightIndices)
		for j, ri := range rightIndices {
			planned[ri].Width += perRight
			if j == len(rightIndices)-1 {
				planned[ri].Width += remainder
			}
		}
		free = availableWidth - totalUsed()
	}

	// step 3: any rounding leftover goes to bridge gap; when no right-side
	// sections exist, leftover is unallocated (sections stay left-aligned)
	if free > 0 && bridgeIdx >= 0 {
		gaps[bridgeIdx] += free
	}

	// step 4: cap bridge gap — overflow goes to last section width
	if bridgeIdx >= 0 && gaps[bridgeIdx] > maxBridgeGap {
		overflow := gaps[bridgeIdx] - maxBridgeGap
		gaps[bridgeIdx] = maxBridgeGap
		planned[len(planned)-1].Width += overflow
	}

	return LayoutPlan{Sections: planned, Gaps: gaps}
}

// findBridgeGap returns the gap index between the last left-side section and
// the first right-side section, or -1 if no right-side sections exist.
func findBridgeGap(active []SectionInput) int {
	for i := 0; i < len(active)-1; i++ {
		if !isRightSide(active[i].ID) && isRightSide(active[i+1].ID) {
			return i
		}
	}
	return -1
}
