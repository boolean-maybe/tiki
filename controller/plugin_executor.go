package controller

import (
	"context"
	"log/slog"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
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
	tikiStore    store.Store
	mutationGate *service.TikiMutationGate
	statusline   *model.StatuslineConfig
	schema       ruki.Schema

	// onTikiUpdated is invoked for each updated tiki so the host controller
	// can refresh its own derived state (e.g. search result caches). May be
	// nil when the caller has no such state.
	onTikiUpdated func(*tikipkg.Tiki)

	// pluginName is carried on log lines so entries from different view
	// kinds are disambiguated. It's set per-action via Execute's pa.Label.
	pluginName string
}

// NewPluginExecutor constructs an executor. onTikiUpdated may be nil.
func NewPluginExecutor(
	tikiStore store.Store,
	mutationGate *service.TikiMutationGate,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
	pluginName string,
	onTikiUpdated func(*tikipkg.Tiki),
) *PluginExecutor {
	return &PluginExecutor{
		tikiStore:     tikiStore,
		mutationGate:  mutationGate,
		statusline:    statusline,
		schema:        schema,
		pluginName:    pluginName,
		onTikiUpdated: onTikiUpdated,
	}
}

// BuildExecutionInput performs the selection/create-template preflight for a
// ruki-kind action. Returns ok=false when the caller should not proceed
// (selection requirements unmet or template construction failed).
//
// selectedIDs are the currently selected tiki ids as seen by the calling
// view. Non-board views pass empty or a single id threaded through from a
// `kind: view` navigation; board views pass their lane selection.
func (pe *PluginExecutor) BuildExecutionInput(pa *plugin.PluginAction, selectedIDs []string) (ruki.ExecutionInput, bool) {
	input := ruki.ExecutionInput{}

	if !selectionSatisfies(pa.Require, len(selectedIDs)) {
		return input, false
	}
	if len(selectedIDs) > 0 {
		input.SelectedTikiIDs = selectedIDs
	}

	if pa.Action != nil && pa.Action.IsCreate() {
		template, err := pe.tikiStore.NewTikiTemplate()
		if err != nil {
			slog.Error("failed to create tiki template for plugin action", "error", err)
			return input, false
		}
		input.CreateTemplate = tikipkg.WrapDoc(template)
	}
	return input, true
}

// EvalChooseFilter evaluates an action's choose subquery against the current
// store state, returning the candidate tikis sorted by priority/title. Errors
// surface to the statusline so the caller can simply abort dispatch on
// ok=false.
//
// Empty results are handled by the caller, not here, because the right
// behavior depends on the action kind: a ruki-kind choose() action with no
// candidates is a dead end (statusline message, abort), but a view-kind
// `choose:` action with no candidates should still open an empty QuickSelect
// so the user sees "this list is empty" directly rather than missing a
// statusline notification.
func (pe *PluginExecutor) EvalChooseFilter(pa *plugin.PluginAction, input ruki.ExecutionInput) ([]*tikipkg.Tiki, bool) {
	if pa.ChooseFilter == nil {
		return nil, false
	}
	allTikis := pe.tikiStore.GetAllTikis()
	executor := ruki.NewExecutor(pe.schema, pe.factory(), pe.userFunc(),
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
	candidateDocs, err := executor.EvalSubQueryFilter(pa.ChooseFilter, tikipkg.WrapDocs(allTikis), input)
	if err != nil {
		slog.Error("failed to evaluate choose filter", "key", pa.KeyStr, "error", err)
		if pe.statusline != nil {
			pe.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return nil, false
	}
	candidates := tikipkg.UnwrapDocs(candidateDocs)
	sortTikisByPriorityTitle(candidates)
	return candidates, true
}

// Execute runs a ruki-kind action and applies its result. Returns true on
// success (including a benign select-only pipeline).
func (pe *PluginExecutor) Execute(pa *plugin.PluginAction, input ruki.ExecutionInput) bool {
	if pa.Action == nil {
		slog.Error("plugin executor called with nil ruki statement", "key", pa.KeyStr)
		return false
	}

	executor := ruki.NewExecutor(pe.schema, pe.factory(), pe.userFunc(),
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
	allTikis := pe.tikiStore.GetAllTikis()

	result, err := executor.Execute(pa.Action, tikipkg.WrapDocs(allTikis), input)
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
		for _, doc := range result.Update.Updated {
			tk := tikipkg.UnwrapDoc(doc)
			if err := pe.mutationGate.UpdateTiki(ctx, tk); err != nil {
				slog.Error("failed to update tiki after plugin action", "tiki_id", tk.ID(), "key", pa.KeyStr, "error", err)
				pe.setError(err)
				return false
			}
			if pe.onTikiUpdated != nil {
				pe.onTikiUpdated(tk)
			}
		}
	case result.Create != nil:
		if err := pe.mutationGate.CreateTiki(ctx, tikipkg.UnwrapDoc(result.Create.Tiki)); err != nil {
			slog.Error("failed to create tiki from plugin action", "key", pa.KeyStr, "error", err)
			pe.setError(err)
			return false
		}
	case result.Delete != nil:
		for _, doc := range result.Delete.Deleted {
			tk := tikipkg.UnwrapDoc(doc)
			if err := pe.mutationGate.DeleteTiki(ctx, tk); err != nil {
				slog.Error("failed to delete tiki from plugin action", "tiki_id", tk.ID(), "key", pa.KeyStr, "error", err)
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

// factory returns the DocumentFactory the executor's create path uses to mint
// a blank document. It wraps a fresh *Tiki so the create path stays
// host-agnostic at the ruki boundary.
func (pe *PluginExecutor) factory() ruki.DocumentFactory {
	return tikipkg.NewDoc
}

// userFunc returns a closure that resolves the current user's name for the
// ruki executor's `user()` builtin, or nil when no identity is available.
func (pe *PluginExecutor) userFunc() func() string {
	if name := getCurrentUserName(pe.tikiStore); name != "" {
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
