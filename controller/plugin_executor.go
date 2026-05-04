package controller

import (
	"context"
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// PluginExecutor owns the deps needed to run a ruki-kind plugin action. It
// deliberately carries no selection or lane state so the same instance can
// be composed into any controller that needs to fire plugin actions —
// board/list controllers plus wiki/detail controllers for global actions.
//
// Phase 6B.4: extracted from PluginController so non-board views can
// dispatch `kind: ruki` globals without duplicating the executor pipeline.
type PluginExecutor struct {
	taskStore    store.Store
	mutationGate *service.TaskMutationGate
	statusline   *model.StatuslineConfig
	schema       ruki.Schema

	// onTaskUpdated is invoked for each updated tiki so the host controller
	// can refresh its own derived state (e.g. search result caches). May be
	// nil when the caller has no such state.
	onTaskUpdated func(*tikipkg.Tiki)

	// pluginName is carried on log lines so entries from different view
	// kinds are disambiguated. It's set per-action via Execute's pa.Label.
	pluginName string
}

// NewPluginExecutor constructs an executor. onTaskUpdated may be nil.
func NewPluginExecutor(
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
	pluginName string,
	onTaskUpdated func(*tikipkg.Tiki),
) *PluginExecutor {
	return &PluginExecutor{
		taskStore:     taskStore,
		mutationGate:  mutationGate,
		statusline:    statusline,
		schema:        schema,
		pluginName:    pluginName,
		onTaskUpdated: onTaskUpdated,
	}
}

// BuildExecutionInput performs the selection/create-template preflight for a
// ruki-kind action. Returns ok=false when the caller should not proceed
// (selection requirements unmet or template construction failed).
//
// selectedIDs are the currently selected task ids as seen by the calling
// view. Non-board views pass empty or a single id threaded through from a
// `kind: view` navigation; board views pass their lane selection.
func (pe *PluginExecutor) BuildExecutionInput(pa *plugin.PluginAction, selectedIDs []string) (ruki.ExecutionInput, bool) {
	input := ruki.ExecutionInput{}

	if !selectionSatisfies(pa.Require, len(selectedIDs)) {
		return input, false
	}
	if len(selectedIDs) > 0 {
		input.SelectedTaskIDs = selectedIDs
	}

	if pa.Action != nil && pa.Action.IsCreate() {
		template, err := pe.taskStore.NewTikiTemplate()
		if err != nil {
			slog.Error("failed to create task template for plugin action", "error", err)
			return input, false
		}
		input.CreateTemplate = template
	}
	return input, true
}

// Execute runs a ruki-kind action and applies its result. Returns true on
// success (including a benign select-only pipeline).
func (pe *PluginExecutor) Execute(pa *plugin.PluginAction, input ruki.ExecutionInput) bool {
	if pa.Action == nil {
		slog.Error("plugin executor called with nil ruki statement", "key", pa.KeyStr)
		return false
	}

	executor := ruki.NewExecutor(pe.schema, pe.userFunc(),
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
	allTikis := pe.taskStore.GetAllTikis()

	result, err := executor.Execute(pa.Action, allTikis, input)
	if err != nil {
		args := append(logSelectionFields(input), "key", pa.KeyStr, "error", err)
		slog.Error("failed to execute plugin action", args...)
		pe.setError(err)
		return false
	}

	return pe.applyResult(pa, input, result)
}

// applyResult routes the result to the appropriate sink (mutation gate,
// pipe, clipboard). Mutation errors surface to the statusline; successful
// pipe executions stay silent except in the clipboard case where the user
// expects a "copied" confirmation.
func (pe *PluginExecutor) applyResult(pa *plugin.PluginAction, input ruki.ExecutionInput, result *ruki.Result) bool {
	ctx := context.Background()
	switch {
	case result.Select != nil:
		args := append(logSelectionFields(input), "key", pa.KeyStr, "label", pa.Label, "matched", len(result.Select.Tikis))
		slog.Info("select plugin action executed", args...)
		return true
	case result.Update != nil:
		for _, tk := range result.Update.Updated {
			if err := pe.mutationGate.UpdateTiki(ctx, tk); err != nil {
				slog.Error("failed to update task after plugin action", "task_id", tk.ID, "key", pa.KeyStr, "error", err)
				pe.setError(err)
				return false
			}
			if pe.onTaskUpdated != nil {
				pe.onTaskUpdated(tk)
			}
		}
	case result.Create != nil:
		if err := pe.mutationGate.CreateTiki(ctx, result.Create.Tiki); err != nil {
			slog.Error("failed to create task from plugin action", "key", pa.KeyStr, "error", err)
			pe.setError(err)
			return false
		}
	case result.Delete != nil:
		for _, tk := range result.Delete.Deleted {
			if err := pe.mutationGate.DeleteTiki(ctx, tk); err != nil {
				slog.Error("failed to delete task from plugin action", "task_id", tk.ID, "key", pa.KeyStr, "error", err)
				pe.setError(err)
				return false
			}
		}
	case result.Pipe != nil:
		for _, row := range result.Pipe.Rows {
			if err := service.ExecutePipeCommand(ctx, result.Pipe.Command, row); err != nil {
				slog.Error("pipe command failed", "command", result.Pipe.Command, "args", row, "key", pa.KeyStr, "error", err)
				pe.setError(err)
			}
		}
	case result.Clipboard != nil:
		if err := service.ExecuteClipboardPipe(result.Clipboard.Rows); err != nil {
			slog.Error("clipboard pipe failed", "key", pa.KeyStr, "error", err)
			pe.setError(err)
			return false
		}
		if pe.statusline != nil {
			pe.statusline.SetMessage("copied to clipboard", model.MessageLevelInfo, true)
		}
	}

	args := append(logSelectionFields(input), "key", pa.KeyStr, "label", pa.Label, "plugin", pe.pluginName)
	slog.Info("plugin action applied", args...)
	return true
}

// userFunc returns a closure that resolves the current user's name for the
// ruki executor's `user()` builtin, or nil when no identity is available.
func (pe *PluginExecutor) userFunc() func() string {
	if name := getCurrentUserName(pe.taskStore); name != "" {
		return func() string { return name }
	}
	return nil
}

// setError posts err to the statusline when one is attached.
func (pe *PluginExecutor) setError(err error) {
	if pe.statusline != nil {
		pe.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
	}
}
