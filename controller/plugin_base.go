package controller

import (
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
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
}

func (pb *pluginBase) GetActionRegistry() *ActionRegistry { return pb.registry }
func (pb *pluginBase) GetPluginName() string              { return pb.pluginDef.Name }

func (pb *pluginBase) handleNav(direction string, filteredTasks func(int) []*task.Task) bool {
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

func (pb *pluginBase) handleVerticalNav(direction string, lane int, tasks []*task.Task) bool {
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

func (pb *pluginBase) handleHorizontalNav(direction string, lane int, tasks []*task.Task, filteredTasks func(int) []*task.Task) bool {
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

func (pb *pluginBase) handleLaneSwitch(direction string, filteredTasks func(int) []*task.Task) bool {
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

func (pb *pluginBase) applyLaneSwitch(targetLane int, targetTasks []*task.Task, direction string, rowOffsetInViewport int, preserveRow bool) bool {
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

func (pb *pluginBase) getSelectedTaskID(filteredTasks func(int) []*task.Task) string {
	lane := pb.pluginConfig.GetSelectedLane()
	tasks := filteredTasks(lane)
	idx := pb.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

func (pb *pluginBase) selectTaskInLane(lane int, taskID string, filteredTasks func(int) []*task.Task) {
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

func (pb *pluginBase) selectFirstNonEmptyLane(filteredTasks func(int) []*task.Task) bool {
	for lane := range pb.pluginDef.Lanes {
		if len(filteredTasks(lane)) > 0 {
			pb.pluginConfig.SetSelectedLaneAndIndex(lane, 0)
			return true
		}
	}
	return false
}

// EnsureFirstNonEmptyLaneSelection selects the first non-empty lane if the current lane is empty.
func (pb *pluginBase) EnsureFirstNonEmptyLaneSelection(filteredTasks func(int) []*task.Task) bool {
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
	results := pb.taskStore.Search(query, nil)
	if len(results) == 0 {
		pb.pluginConfig.SetSearchResults([]task.SearchResult{}, query)
		return
	}
	pb.pluginConfig.SetSearchResults(results, query)
	selectFirst()
}

func (pb *pluginBase) handleOpenTask(filteredTasks func(int) []*task.Task) bool {
	taskID := pb.getSelectedTaskID(filteredTasks)
	if taskID == "" {
		return false
	}
	pb.navController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	return true
}

func (pb *pluginBase) handleNewTask() bool {
	t, err := pb.taskStore.NewTaskTemplate()
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

func (pb *pluginBase) handleDeleteTask(filteredTasks func(int) []*task.Task) bool {
	taskID := pb.getSelectedTaskID(filteredTasks)
	if taskID == "" {
		return false
	}
	taskItem := pb.taskStore.GetTask(taskID)
	if taskItem == nil {
		return false
	}
	if err := pb.mutationGate.DeleteTask(taskItem); err != nil {
		slog.Error("failed to delete task", "task_id", taskID, "error", err)
		if pb.statusline != nil {
			pb.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}
	return true
}

func filterTasksBySearch(tasks []*task.Task, searchMap map[string]bool) []*task.Task {
	if searchMap == nil {
		return tasks
	}
	filtered := make([]*task.Task, 0, len(tasks))
	for _, t := range tasks {
		if searchMap[t.ID] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
