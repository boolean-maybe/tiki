package controller

import (
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// pluginBase holds the shared fields and methods common to PluginController and DepsController.
// Methods that depend on per-controller filtering accept a filteredTasks callback.
type pluginBase struct {
	taskStore     store.Store
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
	if direction == "left" || direction == "right" {
		if pb.pluginConfig.MoveSelection(direction, len(tasks)) {
			return true
		}
		return pb.handleLaneSwitch(direction, filteredTasks)
	}
	return pb.pluginConfig.MoveSelection(direction, len(tasks))
}

func (pb *pluginBase) handleLaneSwitch(direction string, filteredTasks func(int) []*task.Task) bool {
	currentLane := pb.pluginConfig.GetSelectedLane()
	nextLane := currentLane
	switch direction {
	case "left":
		nextLane--
	case "right":
		nextLane++
	default:
		return false
	}

	for nextLane >= 0 && nextLane < len(pb.pluginDef.Lanes) {
		tasks := filteredTasks(nextLane)
		if len(tasks) > 0 {
			pb.pluginConfig.SetSelectedLane(nextLane)
			// select the task at top of viewport (scroll offset) rather than keeping stale index
			scrollOffset := pb.pluginConfig.GetScrollOffsetForLane(nextLane)
			if scrollOffset >= len(tasks) {
				scrollOffset = len(tasks) - 1
			}
			if scrollOffset < 0 {
				scrollOffset = 0
			}
			pb.pluginConfig.SetSelectedIndexForLane(nextLane, scrollOffset)
			return true
		}
		switch direction {
		case "left":
			nextLane--
		case "right":
			nextLane++
		}
	}
	return false
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
	pb.taskStore.DeleteTask(taskID)
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
