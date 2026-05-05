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
)

// pluginBase holds the shared fields and methods common to PluginController and DepsController.
// Methods that depend on per-controller filtering accept a filteredTasks callback.
type pluginBase struct {
	taskStore     store.Store
	mutationGate  *service.TaskMutationGate
	pluginConfig  *model.PluginConfig
	pluginDef     *plugin.TikiPlugin
	navController *NavigationController
	statusline    *model.StatuslineConfig
	registry      *ActionRegistry
	schema        ruki.Schema
}

// newExecutor creates a ruki executor configured for plugin runtime.
func (pb *pluginBase) newExecutor() *ruki.Executor {
	var userFunc func() string
	if userName := getCurrentUserName(pb.taskStore); userName != "" {
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

func (pb *pluginBase) handleNav(direction string, filteredTasks func(int) []*tikipkg.Tiki) bool {
	lane := pb.pluginConfig.GetSelectedLane()
	tasks := filteredTasks(lane)
	switch direction {
	case "up", "down":
		return pb.handleVerticalNav(direction, lane, tasks)
	case "left", "right":
		return pb.handleHorizontalNav(direction, lane, tasks, filteredTasks)
	default:
		return false
	}
}

func (pb *pluginBase) handleVerticalNav(direction string, lane int, tasks []*tikipkg.Tiki) bool {
	if len(tasks) == 0 {
		return false
	}

	storedIndex := pb.pluginConfig.GetSelectedIndexForLane(lane)
	clampedIndex := clampTaskIndex(storedIndex, len(tasks))
	if storedIndex != clampedIndex {
		columns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(lane))
		finalIndex := moveVerticalIndex(direction, clampedIndex, columns, len(tasks))
		if storedIndex != finalIndex {
			pb.pluginConfig.SetSelectedIndexForLane(lane, finalIndex)
			return true
		}
		return false
	}

	return pb.pluginConfig.MoveSelection(direction, len(tasks))
}

func (pb *pluginBase) handleHorizontalNav(direction string, lane int, tasks []*tikipkg.Tiki, filteredTasks func(int) []*tikipkg.Tiki) bool {
	if len(tasks) > 0 {
		storedIndex := pb.pluginConfig.GetSelectedIndexForLane(lane)
		clampedIndex := clampTaskIndex(storedIndex, len(tasks))
		columns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(lane))
		if moved, targetIndex := moveHorizontalIndex(direction, clampedIndex, columns, len(tasks)); moved {
			pb.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
			return true
		}
	}
	return pb.handleLaneSwitch(direction, filteredTasks)
}

func (pb *pluginBase) handleLaneSwitch(direction string, filteredTasks func(int) []*tikipkg.Tiki) bool {
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

	sourceTasks := filteredTasks(currentLane)
	rowOffsetInViewport := 0
	if len(sourceTasks) > 0 {
		sourceColumns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(currentLane))
		sourceIndex := clampTaskIndex(pb.pluginConfig.GetSelectedIndexForLane(currentLane), len(sourceTasks))
		sourceRow := sourceIndex / sourceColumns
		maxSourceRow := maxRowIndex(len(sourceTasks), sourceColumns)
		sourceScroll := clampInt(pb.pluginConfig.GetScrollOffsetForLane(currentLane), maxSourceRow)
		rowOffsetInViewport = sourceRow - sourceScroll
	}

	adjacentTasks := filteredTasks(nextLane)
	if len(adjacentTasks) > 0 {
		return pb.applyLaneSwitch(nextLane, adjacentTasks, direction, rowOffsetInViewport, true)
	}

	// preserve existing skip-empty traversal order when adjacent lane is empty
	scanLane := nextLane + step
	for scanLane >= 0 && scanLane < len(pb.pluginDef.Lanes) {
		tasks := filteredTasks(scanLane)
		if len(tasks) > 0 {
			// skip-empty landing uses target viewport row semantics (no source row carry-over)
			return pb.applyLaneSwitch(scanLane, tasks, direction, 0, false)
		}
		scanLane += step
	}

	return false
}

func (pb *pluginBase) applyLaneSwitch(targetLane int, targetTasks []*tikipkg.Tiki, direction string, rowOffsetInViewport int, preserveRow bool) bool {
	if len(targetTasks) == 0 {
		return false
	}

	targetColumns := normalizeColumns(pb.pluginConfig.GetColumnsForLane(targetLane))
	maxTargetRow := maxRowIndex(len(targetTasks), targetColumns)
	targetScroll := clampInt(pb.pluginConfig.GetScrollOffsetForLane(targetLane), maxTargetRow)
	targetRow := targetScroll
	if preserveRow {
		targetRow = clampInt(targetScroll+rowOffsetInViewport, maxTargetRow)
	}

	targetIndex := rowDirectionalIndex(direction, targetRow, targetColumns, len(targetTasks))
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

func clampTaskIndex(index int, taskCount int) int {
	if taskCount <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= taskCount {
		return taskCount - 1
	}
	return index
}

func maxRowIndex(taskCount int, columns int) int {
	if taskCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	return (taskCount - 1) / columns
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

func moveVerticalIndex(direction string, index int, columns int, taskCount int) int {
	if taskCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	index = clampTaskIndex(index, taskCount)

	switch direction {
	case "up":
		next := index - columns
		if next >= 0 {
			return next
		}
	case "down":
		next := index + columns
		if next < taskCount {
			return next
		}
	}
	return index
}

func moveHorizontalIndex(direction string, index int, columns int, taskCount int) (bool, int) {
	if taskCount <= 0 {
		return false, 0
	}
	columns = normalizeColumns(columns)
	index = clampTaskIndex(index, taskCount)
	col := index % columns

	switch direction {
	case "left":
		if col > 0 {
			return true, index - 1
		}
	case "right":
		if col < columns-1 && index+1 < taskCount {
			return true, index + 1
		}
	}
	return false, index
}

func rowDirectionalIndex(direction string, row int, columns int, taskCount int) int {
	if taskCount <= 0 {
		return 0
	}
	columns = normalizeColumns(columns)
	maxRow := maxRowIndex(taskCount, columns)
	row = clampInt(row, maxRow)
	rowStart := row * columns
	if rowStart >= taskCount {
		return taskCount - 1
	}

	switch direction {
	case "left":
		rowEnd := rowStart + columns - 1
		if rowEnd >= taskCount {
			rowEnd = taskCount - 1
		}
		return rowEnd
	case "right":
		return rowStart
	default:
		return rowStart
	}
}

func (pb *pluginBase) getSelectedTaskID(filteredTasks func(int) []*tikipkg.Tiki) string {
	lane := pb.pluginConfig.GetSelectedLane()
	tasks := filteredTasks(lane)
	idx := pb.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

// getSelectedTaskIDs returns all currently selected task IDs. Today the UI
// only supports single-selection, so the result is a one-item slice (or nil)
// — but callers should treat this as the canonical multi-selection accessor
// so plumbing is ready when true multi-select lands.
func (pb *pluginBase) getSelectedTaskIDs(filteredTasks func(int) []*tikipkg.Tiki) []string {
	id := pb.getSelectedTaskID(filteredTasks)
	if id == "" {
		return nil
	}
	return []string{id}
}

func (pb *pluginBase) selectTaskInLane(lane int, taskID string, filteredTasks func(int) []*tikipkg.Tiki) {
	if lane < 0 || lane >= len(pb.pluginDef.Lanes) {
		return
	}
	tasks := filteredTasks(lane)
	targetIndex := 0
	for i, t := range tasks {
		if t.ID == taskID {
			targetIndex = i
			break
		}
	}
	pb.pluginConfig.SetSelectedLane(lane)
	pb.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
}

func (pb *pluginBase) selectFirstNonEmptyLane(filteredTasks func(int) []*tikipkg.Tiki) bool {
	for lane := range pb.pluginDef.Lanes {
		if len(filteredTasks(lane)) > 0 {
			pb.pluginConfig.SetSelectedLaneAndIndex(lane, 0)
			return true
		}
	}
	return false
}

// EnsureFirstNonEmptyLaneSelection selects the first non-empty lane if the current lane is empty.
func (pb *pluginBase) EnsureFirstNonEmptyLaneSelection(filteredTasks func(int) []*tikipkg.Tiki) bool {
	if pb.pluginDef == nil {
		return false
	}
	currentLane := pb.pluginConfig.GetSelectedLane()
	if currentLane >= 0 && currentLane < len(pb.pluginDef.Lanes) {
		if len(filteredTasks(currentLane)) > 0 {
			return false
		}
	}
	return pb.selectFirstNonEmptyLane(filteredTasks)
}

func (pb *pluginBase) handleSearch(query string, selectFirst func() bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	pb.pluginConfig.SavePreSearchState()
	results := pb.taskStore.SearchTikis(query, nil)
	if len(results) == 0 {
		pb.pluginConfig.SetSearchResults([]*tikipkg.Tiki{}, query)
		return
	}
	pb.pluginConfig.SetSearchResults(results, query)
	selectFirst()
}

func (pb *pluginBase) handleNewTask() bool {
	t, err := pb.taskStore.NewTikiTemplate()
	if err != nil {
		slog.Error("failed to create task template", "error", err)
		return false
	}
	pb.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: t.ID,
		Draft:  t,
		Focus:  model.EditFieldTitle,
	}))
	slog.Info("new tiki draft started from plugin", "task_id", t.ID, "plugin", pb.pluginDef.Name)
	return true
}

func (pb *pluginBase) handleDeleteTask(filteredTasks func(int) []*tikipkg.Tiki) bool {
	taskID := pb.getSelectedTaskID(filteredTasks)
	if taskID == "" {
		return false
	}
	tk := pb.taskStore.GetTiki(taskID)
	if tk == nil {
		return false
	}
	if err := pb.mutationGate.DeleteTiki(context.Background(), tk); err != nil {
		slog.Error("failed to delete task", "task_id", taskID, "error", err)
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

// sortTikisByPriorityTitle sorts tikis by priority (ascending) then title (ascending).
// Zero priority (absent field) sorts last, matching task.Sort behavior.
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

func tikiPriorityForSort(tk *tikipkg.Tiki) int {
	if tk == nil {
		return 0
	}
	if n, ok := tk.Fields[tikipkg.FieldPriority].(int); ok {
		return n
	}
	return 0
}
