package taskdetail

import (
	"testing"
)

// allSixSections returns the standard 6-section input with all content present.
// Widths mirror realistic values: left-side=30, tags=10, dep/blk=30.
func allSixSections() []SectionInput {
	return []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: true},
		{ID: SectionPeopleGroup, Width: 30, HasContent: true},
		{ID: SectionDueGroup, Width: 30, HasContent: true},
		{ID: SectionTags, Width: 10, HasContent: true},
		{ID: SectionDependsOn, Width: 30, HasContent: true},
		{ID: SectionBlocks, Width: 30, HasContent: true},
	}
}

func TestAllSectionsFit(t *testing.T) {
	// 30+30+30+10+30+30 = 160 min widths + 5 gaps = 165 min
	// give 190: left gaps expand to 8 each, right sections expand with remaining
	plan := CalculateMetadataLayout(190, allSixSections())

	if len(plan.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(plan.Sections))
	}
	if len(plan.Gaps) != 5 {
		t.Fatalf("expected 5 gaps, got %d", len(plan.Gaps))
	}

	// left-side gaps capped at 8, bridge+right gaps stay at 1
	expectedGaps := []int{8, 8, 1, 1, 1}
	for i, g := range plan.Gaps {
		if g != expectedGaps[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expectedGaps[i])
		}
	}

	// right-side sections expand: (190-90-19)/3 = 27, remainder distributed
	verifyTotalWidth(t, plan, 190)
}

func TestAllSectionsFit_RemainderInBridgeGap(t *testing.T) {
	// 160 widths + 5 gaps min = 165; give 178
	// left gaps: free=13, perGap=13/2=6, gaps=[7,7], then right sections expand
	plan := CalculateMetadataLayout(178, allSixSections())

	if len(plan.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(plan.Sections))
	}

	expectedGaps := []int{7, 7, 1, 1, 1}
	for i, g := range plan.Gaps {
		if g != expectedGaps[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expectedGaps[i])
		}
	}
	verifyTotalWidth(t, plan, 178)
}

func TestTagsHiddenFirst(t *testing.T) {
	// min with all 6: 160 + 5 = 165. With Tags removed: 150 + 4 = 154
	// give 160 — doesn't fit all 6 (165), but fits without Tags (154)
	plan := CalculateMetadataLayout(160, allSixSections())

	for _, s := range plan.Sections {
		if s.ID == SectionTags {
			t.Error("Tags should be hidden but is present")
		}
	}
	if len(plan.Sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(plan.Sections))
	}
	verifyTotalWidth(t, plan, 160)
}

func TestTagsAndBlocksHidden(t *testing.T) {
	// without Tags+Blocks: 30+30+30+30 = 120 + 3 gaps = 123
	// give 130 — doesn't fit 5 sections (154 without Tags), but fits 4 (123)
	plan := CalculateMetadataLayout(130, allSixSections())

	ids := sectionIDs(plan)
	if contains(ids, SectionTags) {
		t.Error("Tags should be hidden")
	}
	if contains(ids, SectionBlocks) {
		t.Error("Blocks should be hidden")
	}
	if !contains(ids, SectionDependsOn) {
		t.Error("DependsOn should still be visible")
	}
	if len(plan.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(plan.Sections))
	}
	verifyTotalWidth(t, plan, 130)
}

func TestAllRightSideHidden(t *testing.T) {
	// without all right-side: 30+30+30 = 90 + 2 gaps = 92
	// give 95
	plan := CalculateMetadataLayout(95, allSixSections())

	if len(plan.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(plan.Sections))
	}
	for _, s := range plan.Sections {
		if isRightSide(s.ID) {
			t.Errorf("right-side section %d should be hidden", s.ID)
		}
	}

	// free = 95-92 = 3, 2 left gaps, perGap=min(3/2,7)=1, gaps=[2, 2]
	// leftover 1 goes to last gap → [2, 3]
	expected := []int{2, 3}
	for i, g := range plan.Gaps {
		if g != expected[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expected[i])
		}
	}
	verifyTotalWidth(t, plan, 95)
}

func TestDueGroupEmpty_BridgeShifts(t *testing.T) {
	// Due/Recurrence empty but always shown — shed Tags+Blocks to fit width
	sections := []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: true},
		{ID: SectionPeopleGroup, Width: 30, HasContent: true},
		{ID: SectionDueGroup, Width: 30, HasContent: true},
		{ID: SectionTags, Width: 10, HasContent: true},
		{ID: SectionDependsOn, Width: 30, HasContent: true},
		{ID: SectionBlocks, Width: 30, HasContent: true},
	}

	// 6 sections need 165 min > 149 → shed Tags(10), then Blocks(30) → 4 active: 120+3=123
	plan := CalculateMetadataLayout(149, sections)

	if len(plan.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(plan.Sections))
	}
	if contains(sectionIDs(plan), SectionTags) {
		t.Error("Tags should be shed (not enough width)")
	}

	// bridge at index 2 (DueGroup→DependsOn), 2 left gaps
	// free=149-123=26, perGap=min(26/2,7)=7, gaps[0]=8,gaps[1]=8
	// right expand: remaining=149-90-8-8-1=42, DependsOn=30+12=42
	expectedGaps := []int{8, 8, 1}
	for i, g := range plan.Gaps {
		if g != expectedGaps[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expectedGaps[i])
		}
	}
	verifyTotalWidth(t, plan, 149)
}

func TestOnlyRequiredSections(t *testing.T) {
	sections := []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: true},
		{ID: SectionPeopleGroup, Width: 30, HasContent: true},
		{ID: SectionDueGroup, Width: 30, HasContent: true},
		{ID: SectionTags, Width: 10, HasContent: false},
		{ID: SectionDependsOn, Width: 30, HasContent: false},
		{ID: SectionBlocks, Width: 30, HasContent: false},
	}

	plan := CalculateMetadataLayout(100, sections)

	if len(plan.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(plan.Sections))
	}
	if len(plan.Gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(plan.Gaps))
	}
	// 3 left-side sections, no right-side → no bridge, leftGapCount=numGaps=2
	// free=100-92=8, perGap=min(8/2,6)=4, gaps[0]=5, gaps[1]=5
	expectedGaps := []int{5, 5}
	for i, g := range plan.Gaps {
		if g != expectedGaps[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expectedGaps[i])
		}
	}
}

func TestExactFit(t *testing.T) {
	// 160 + 5 = 165 exactly — no free space, all gaps stay at 1
	plan := CalculateMetadataLayout(165, allSixSections())

	if len(plan.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(plan.Sections))
	}
	for i, g := range plan.Gaps {
		if g != 1 {
			t.Errorf("gap[%d] = %d, want 1", i, g)
		}
	}
}

func TestExtremelyNarrow(t *testing.T) {
	// even min required (30+30+1=61) doesn't fit — but algorithm guarantees at least 1 gap
	plan := CalculateMetadataLayout(50, allSixSections())

	// all right-side sections should be shed; only left-side remain
	for _, s := range plan.Sections {
		if isRightSide(s.ID) {
			t.Errorf("right-side section %d should be hidden", s.ID)
		}
	}
	for i, g := range plan.Gaps {
		if g < 1 {
			t.Errorf("gap[%d] = %d, should be at least 1", i, g)
		}
	}
}

func TestSingleSection(t *testing.T) {
	sections := []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: true},
		{ID: SectionPeopleGroup, Width: 30, HasContent: false},
	}
	plan := CalculateMetadataLayout(100, sections)

	if len(plan.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(plan.Sections))
	}
	if len(plan.Gaps) != 0 {
		t.Fatalf("expected 0 gaps, got %d", len(plan.Gaps))
	}
}

func TestNoActiveSections(t *testing.T) {
	sections := []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: false},
	}
	plan := CalculateMetadataLayout(100, sections)

	if len(plan.Sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(plan.Sections))
	}
}

func TestLeftSideGapsCapped(t *testing.T) {
	// wide terminal: all 6 sections, width 260
	// left gaps cap at 8. Right-side sections expand with remaining space.
	plan := CalculateMetadataLayout(260, allSixSections())

	if len(plan.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(plan.Sections))
	}

	// left-side gaps capped at 8, bridge+right gaps at 1
	expectedGaps := []int{8, 8, 1, 1, 1}
	for i, g := range plan.Gaps {
		if g != expectedGaps[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expectedGaps[i])
		}
	}

	// verify left-side gaps are all <= maxLeftSideGap
	for i := 0; i < len(plan.Gaps); i++ {
		leftSide := !isRightSide(plan.Sections[i].ID) && !isRightSide(plan.Sections[i+1].ID)
		if leftSide && plan.Gaps[i] > maxLeftSideGap {
			t.Errorf("left-side gap[%d] = %d, exceeds max %d", i, plan.Gaps[i], maxLeftSideGap)
		}
	}

	// right-side sections should have expanded significantly
	for _, s := range plan.Sections {
		if isRightSide(s.ID) && s.Width <= 30 {
			t.Errorf("right-side section %d width=%d, should be expanded beyond min 30", s.ID, s.Width)
		}
	}

	verifyTotalWidth(t, plan, 260)
}

func TestLeftSideGapsCapped_AllLeftSide(t *testing.T) {
	// 3 left-side sections only, wide terminal
	// widths = 90, free = 110, 2 gaps
	// left gaps: perGap=min(110/2,7)=7, gaps=[8,8], used=106, leftover=94 → last gap
	plan := CalculateMetadataLayout(200, allSixSections()[:3])

	if len(plan.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(plan.Sections))
	}

	expected := []int{8, 102}
	for i, g := range plan.Gaps {
		if g != expected[i] {
			t.Errorf("gap[%d] = %d, want %d", i, g, expected[i])
		}
	}
}

func TestTagsMinWidth(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want int
	}{
		{"longest tag wins", []string{"backend", "bug", "frontend", "search"}, 8},
		{"label wins over short tags", []string{"a", "b"}, 5},
		{"single long tag", []string{"infrastructure"}, 14},
		{"empty tags", nil, 5},
		{"tag exactly label length", []string{"abcd"}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagsMinWidth(tt.tags)
			if got != tt.want {
				t.Errorf("tagsMinWidth(%v) = %d, want %d", tt.tags, got, tt.want)
			}
		})
	}
}

func TestDynamicTagsWidth_KeepsTagsVisible(t *testing.T) {
	// with dynamic tags(8) and dep/blk at 30: total = 158+5 = 163
	// give 170 — comfortably fits all 6 sections
	sections := []SectionInput{
		{ID: SectionStatusGroup, Width: 30, HasContent: true},
		{ID: SectionPeopleGroup, Width: 30, HasContent: true},
		{ID: SectionDueGroup, Width: 30, HasContent: true},
		{ID: SectionTags, Width: 8, HasContent: true},
		{ID: SectionDependsOn, Width: 30, HasContent: true},
		{ID: SectionBlocks, Width: 30, HasContent: true},
	}

	plan := CalculateMetadataLayout(170, sections)

	if !contains(sectionIDs(plan), SectionTags) {
		t.Error("Tags should be visible with dynamic width 8, but was hidden")
	}
	if len(plan.Sections) != 6 {
		t.Errorf("expected 6 sections, got %d", len(plan.Sections))
	}
}

func TestRightSideSectionsExpand(t *testing.T) {
	// verify right-side sections get expanded widths, not just their minimum
	// 160 min + 5 gaps = 165, give 200 → 35 extra
	// left gaps: min(35/2, 7)=7 each → 14 used, 21 remaining for right sections
	plan := CalculateMetadataLayout(200, allSixSections())

	if len(plan.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(plan.Sections))
	}

	// right-side sections should be wider than their minimum
	rightTotal := 0
	for _, s := range plan.Sections {
		if isRightSide(s.ID) {
			rightTotal += s.Width
		}
	}
	// min right total = 10+30+30 = 70; should be 70+21 = 91
	if rightTotal <= 70 {
		t.Errorf("right-side total width=%d, should be expanded beyond minimum 70", rightTotal)
	}

	verifyTotalWidth(t, plan, 200)
}

// helpers

func verifyTotalWidth(t *testing.T, plan LayoutPlan, expected int) {
	t.Helper()
	total := 0
	for _, s := range plan.Sections {
		total += s.Width
	}
	for _, g := range plan.Gaps {
		total += g
	}
	if total != expected {
		t.Errorf("total width = %d, want %d", total, expected)
	}
}

func sectionIDs(plan LayoutPlan) []SectionID {
	ids := make([]SectionID, len(plan.Sections))
	for i, s := range plan.Sections {
		ids[i] = s.ID
	}
	return ids
}

func contains(ids []SectionID, target SectionID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
