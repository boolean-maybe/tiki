package controller

import (
	"context"
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// pluginBase holds the shared fields and methods common to PluginController and DepsController.
// Methods that depend on per-controller filtering accept a filteredTikis callback.
type pluginBase struct {
	tikiStore     store.Store
	mutationGate  *service.TikiMutationGate
	pluginConfig  *model.PluginConfig
	pluginDef     *plugin.WorkflowPlugin
	navController *NavigationController
	statusline    *model.StatuslineConfig
	registry      *ActionRegistry
	schema        ruki.Schema
}

// newExecutor creates a ruki executor configured for plugin runtime.
func (pb *pluginBase) newExecutor() *ruki.Executor {
	var userFunc func() string
	if userName := getCurrentUserName(pb.tikiStore); userName != "" {
		userFunc = func() string { return userName }
	}
	return ruki.NewExecutor(pb.schema, userFunc,
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
}

func (pb *pluginBase) GetActionRegistry() *ActionRegistry { return pb.registry }
func (pb *pluginBase) GetPluginName() string              { return pb.pluginDef.Name }

// default no-op implementations for input-backed action methods
func (pb *pluginBase) GetActionInputSpec(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (pb *pluginBase) CanStartActionInput(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (pb *pluginBase) HandleActionInput(ActionID, string) InputSubmitResult { return InputKeepEditing }

// default no-op implementations for choose-backed action methods
func (pb *pluginBase) GetActionChooseSpec(ActionID) (string, bool) { return "", false }
func (pb *pluginBase) CanStartActionChoose(ActionID) (string, []*tikipkg.Tiki, bool) {
	return "", nil, false
}
func (pb *pluginBase) HandleActionChoose(ActionID, string) bool { return false }

func (pb *pluginBase) handleNav(direction string, filteredTikis func(int) []*tikipkg.Tiki) bool {
	lane := pb.pluginConfig.GetSelectedLane()
	tikis := filteredTikis(lane)
	switch direction {
	case "up", "down":
		return pb.handleVerticalNav(direction, lane, tikis)
	case "left", "right":
		return pb.handleHorizontalNav(direction, lane, tikis, filteredTikis)
	default:
		return false
	}
}

func (pb *pluginBase) handleVerticalNav(direction string, lane int, tikis []*tikipkg.Tiki) bool {
	if len(tikis) == 0 {
		return false
	}

	storedIndex := pb.pluginConfig.GetSelectedIndexForLane(lane)
	clampedIndex := clampTikiIndex(storedIndex, len(tikis))
	if storedIndex != clampedIndex {
		columns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(lane))
		finalIndex := moveVerticalIndex(direction, clampedIndex, columns, len(tikis))
		if storedIndex != finalIndex {
			pb.pluginConfig.SetSelectedIndexForLane(lane, finalIndex)
			return true
		}
		return false
	}

	return pb.pluginConfig.MoveSelection(direction, len(tikis))
}

func (pb *pluginBase) handleHorizontalNav(direction string, lane int, tikis []*tikipkg.Tiki, filteredTikis func(int) []*tikipkg.Tiki) bool {
	if len(tikis) > 0 {
		storedIndex := pb.pluginConfig.GetSelectedIndexForLane(lane)
		clampedIndex := clampTikiIndex(storedIndex, len(tikis))
		columns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(lane))
		if moved, targetIndex := moveHorizontalIndex(direction, clampedIndex, columns, len(tikis)); moved {
			pb.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
			return true
		}
	}
	return pb.handleLaneSwitch(direction, filteredTikis)
}

func (pb *pluginBase) handleLaneSwitch(direction string, filteredTikis func(int) []*tikipkg.Tiki) bool {
	if pb.pluginDef == nil {
		return false
	}
	currentLane := pb.pluginConfig.GetSelectedLane()
	step, ok := laneDirectionStep(direction)
	if !ok {
		return false
	}
	nextLane := currentLane + step
	if nextLane < 0 || nextLane >= len(pb.pluginDef.Lanes) {
		return false
	}

	sourceTikis := filteredTikis(currentLane)
	rowOffsetInViewport := 0
	if len(sourceTikis) > 0 {
		sourceColumns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(currentLane))
		sourceIndex := clampTikiIndex(pb.pluginConfig.GetSelectedIndexForLane(currentLane), len(sourceTikis))
		sourceRow := sourceIndex / sourceColumns
		maxSourceRow := maxRowIndex(len(sourceTikis), sourceColumns)
		sourceScroll := clampInt(pb.pluginConfig.GetScrollOffsetForLane(currentLane), maxSourceRow)
		rowOffsetInViewport = sourceRow - sourceScroll
	}

	adjacentTikis := filteredTikis(nextLane)
	if len(adjacentTikis) > 0 {
		return pb.applyLaneSwitch(nextLane, adjacentTikis, direction, rowOffsetInViewport, true)
	}

	// preserve existing skip-empty traversal order when adjacent lane is empty
	scanLane := nextLane + step
	for scanLane >= 0 && scanLane < len(pb.pluginDef.Lanes) {
		tikis := filteredTikis(scanLane)
		if len(tikis) > 0 {
			// skip-empty landing uses target viewport row semantics (no source row carry-over)
			return pb.applyLaneSwitch(scanLane, tikis, direction, 0, false)
		}
		scanLane += step
	}

	return false
}

func (pb *pluginBase) applyLaneSwitch(targetLane int, targetTikis []*tikipkg.Tiki, direction string, rowOffsetInViewport int, preserveRow bool) bool {
	if len(targetTikis) == 0 {
		return false
	}

	targetColumns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(targetLane))
	maxTargetRow := maxRowIndex(len(targetTikis), targetColumns)
	targetScroll := clampInt(pb.pluginConfig.GetScrollOffsetForLane(targetLane), maxTargetRow)
	targetRow := targetScroll
	if preserveRow {
		targetRow = clampInt(targetScroll+rowOffsetInViewport, maxTargetRow)
	}

	targetIndex := rowDirectionalIndex(direction, targetRow, targetColumns, len(targetTikis))
	pb.pluginConfig.SetScrollOffsetForLane(targetLane, targetScroll)
	pb.pluginConfig.SetSelectedLaneAndIndex(targetLane, targetIndex)
	return true
}

func laneDirectionStep(direction string) (int, bool) {
	switch direction {
	case "left":
		return -1, true
	case "right":
		return 1, true
	default:
		return 0, false
	}
}

func normalizeColumns(columns int) int {
	if columns <= 0 {
		return 1
	}
	return columns
}

func clampTikiIndex(index int, tikiCount int) int {
	if tikiCount <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= tikiCount {
		return tikiCount - 1
	}
	return index
}

func maxRowIndex(tikiCount int, columns int) int {
	if tikiCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	return (tikiCount - 1) / columns
}

func clampInt(value int, maxValue int) int {
	if value < 0 {
		return 0
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func moveVerticalIndex(direction string, index int, columns int, tikiCount int) int {
	if tikiCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	index = clampTikiIndex(index, tikiCount)

	switch direction {
	case "up":
		next := index - columns
		if next >= 0 {
			return next
		}
	case "down":
		next := index + columns
		if next < tikiCount {
			return next
		}
	}
	return index
}

func moveHorizontalIndex(direction string, index int, columns int, tikiCount int) (bool, int) {
	if tikiCount <= 0 {
		return false, 0
	}
	columns = normalizeColumns(columns)
	index = clampTikiIndex(index, tikiCount)
	col := index % columns

	switch direction {
	case "left":
		if col > 0 {
			return true, index - 1
		}
	case "right":
		if col < columns-1 && index+1 < tikiCount {
			return true, index + 1
		}
	}
	return false, index
}

func rowDirectionalIndex(direction string, row int, columns int, tikiCount int) int {
	if tikiCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	maxRow := maxRowIndex(tikiCount, columns)
	row = clampInt(row, maxRow)
	rowStart := row * columns
	if rowStart >= tikiCount {
		return tikiCount - 1
	}

	switch direction {
	case "left":
		rowEnd := rowStart + columns - 1
		if rowEnd >= tikiCount {
			rowEnd = tikiCount - 1
		}
		return rowEnd
	case "right":
		return rowStart
	default:
		return rowStart
	}
}

func (pb *pluginBase) getSelectedTikiID(filteredTikis func(int) []*tikipkg.Tiki) string {
	lane := pb.pluginConfig.GetSelectedLane()
	tikis := filteredTikis(lane)
	idx := pb.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tikis) {
		return ""
	}
	return tikis[idx].ID
}

// getSelectedTikiIDs returns all currently selected tiki IDs. Today the UI
// only supports single-selection, so the result is a one-item slice (or nil)
// — but callers should treat this as the canonical multi-selection accessor
// so plumbing is ready when true multi-select lands.
func (pb *pluginBase) getSelectedTikiIDs(filteredTikis func(int) []*tikipkg.Tiki) []string {
	id := pb.getSelectedTikiID(filteredTikis)
	if id == "" {
		return nil
	}
	return []string{id}
}

func (pb *pluginBase) selectTikiInLane(lane int, tikiID string, filteredTikis func(int) []*tikipkg.Tiki) {
	if lane < 0 || lane >= len(pb.pluginDef.Lanes) {
		return
	}
	tikis := filteredTikis(lane)
	targetIndex := 0
	for i, t := range tikis {
		if t.ID == tikiID {
			targetIndex = i
			break
		}
	}
	pb.pluginConfig.SetSelectedLane(lane)
	pb.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
}

func (pb *pluginBase) selectFirstNonEmptyLane(filteredTikis func(int) []*tikipkg.Tiki) bool {
	for lane := range pb.pluginDef.Lanes {
		if len(filteredTikis(lane)) > 0 {
			pb.pluginConfig.SetSelectedLaneAndIndex(lane, 0)
			return true
		}
	}
	return false
}

// EnsureFirstNonEmptyLaneSelection selects the first non-empty lane if the current lane is empty.
func (pb *pluginBase) EnsureFirstNonEmptyLaneSelection(filteredTikis func(int) []*tikipkg.Tiki) bool {
	if pb.pluginDef == nil {
		return false
	}
	currentLane := pb.pluginConfig.GetSelectedLane()
	if currentLane >= 0 && currentLane < len(pb.pluginDef.Lanes) {
		if len(filteredTikis(currentLane)) > 0 {
			return false
		}
	}
	return pb.selectFirstNonEmptyLane(filteredTikis)
}

func (pb *pluginBase) handleSearch(query string, selectFirst func() bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	pb.pluginConfig.SavePreSearchState()
	results := pb.tikiStore.SearchTikis(query, nil)
	if len(results) == 0 {
		pb.pluginConfig.SetSearchResults([]*tikipkg.Tiki{}, query)
		return
	}
	pb.pluginConfig.SetSearchResults(results, query)
	selectFirst()
}

// createDraftTiki returns a fresh in-memory draft tiki built from the
// store's creation template. The draft is not persisted; the caller is
// responsible for starting a TikiEditSession in draft mode and routing
// to the destination view. Persistence happens on commit.
func createDraftTiki(s store.Store) (*tikipkg.Tiki, error) {
	return s.NewTikiTemplate()
}

func (pb *pluginBase) handleDeleteTiki(filteredTikis func(int) []*tikipkg.Tiki) bool {
	tikiID := pb.getSelectedTikiID(filteredTikis)
	if tikiID == "" {
		return false
	}
	tk := pb.tikiStore.GetTiki(tikiID)
	if tk == nil {
		return false
	}
	if err := pb.mutationGate.DeleteTiki(context.Background(), tk); err != nil {
		slog.Error("failed to delete tiki", "tiki_id", tikiID, "error", err)
		if pb.statusline != nil {
			pb.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}
	return true
}

func filterTikisBySearch(tikis []*tikipkg.Tiki, searchMap map[string]bool) []*tikipkg.Tiki {
	if searchMap == nil {
		return tikis
	}
	filtered := make([]*tikipkg.Tiki, 0, len(tikis))
	for _, tk := range tikis {
		if searchMap[tk.ID] {
			filtered = append(filtered, tk)
		}
	}
	return filtered
}

// sortTikisByPriorityTitle sorts tikis by priority rank (lower rank =
// declared first = higher urgency), then title (ascending). Absent priority
// sorts after all declared values so plain documents drop to the bottom of
// urgency-ordered lists.
func sortTikisByPriorityTitle(tikis []*tikipkg.Tiki) {
	n := len(tikis)
	for i := 1; i < n; i++ {
		for j := i; j > 0; j-- {
			pi := tikiPriorityForSort(tikis[j])
			pj := tikiPriorityForSort(tikis[j-1])
			ti, tj := strings.ToLower(tikis[j].Title), strings.ToLower(tikis[j-1].Title)
			if pi < pj || (pi == pj && ti < tj) || (pi == pj && ti == tj && tikis[j].ID < tikis[j-1].ID) {
				tikis[j], tikis[j-1] = tikis[j-1], tikis[j]
			} else {
				break
			}
		}
	}
}

// tikiPriorityForSort returns a sort rank for the tiki's priority value,
// reading it as a workflow-enum key. Lower rank = earlier in declaration =
// higher urgency. Absent or unrecognized values return MaxInt so they sort
// after all declared keys (consistent with the previous int-based "0 sorts
// last" semantics now that priority is a string enum).
func tikiPriorityForSort(tk *tikipkg.Tiki) int {
	if tk == nil {
		return absentPriorityRank
	}
	key, _, _ := tk.StringField(tikipkg.FieldPriority)
	if key == "" {
		return absentPriorityRank
	}
	fd, ok := workflow.Field(tikipkg.FieldPriority)
	if !ok {
		return absentPriorityRank
	}
	for i, k := range fd.AllowedValues() {
		if k == key {
			return i
		}
	}
	return absentPriorityRank
}

// absentPriorityRank sorts absent / unknown priorities after all declared
// values. Using a large sentinel rather than math.MaxInt keeps the imports
// lean; the value just needs to exceed any realistic enum cardinality.
const absentPriorityRank = 1 << 30
