package header

import (
	"strings"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
)

// modelActionToControllerAction converts a model.HeaderAction to controller.Action
func modelActionToControllerAction(a model.HeaderAction) controller.Action {
	return controller.Action{
		ID:           controller.ActionID(a.ID),
		Key:          a.Key,
		Rune:         a.Rune,
		Label:        a.Label,
		Modifier:     a.Modifier,
		ShowInHeader: a.ShowInHeader,
	}
}

// convertHeaderActions converts a slice of model.HeaderAction to controller.Action,
// filtering out actions that should not be shown in the header.
func convertHeaderActions(actions []model.HeaderAction) []controller.Action {
	var result []controller.Action
	for _, a := range actions {
		if a.ShowInHeader {
			result = append(result, modelActionToControllerAction(a))
		}
	}
	return result
}

// extractViewActions extracts view-specific actions from a registry,
// filtering out global actions and plugin actions (identified by "plugin:" prefix).
func extractViewActions(registry *controller.ActionRegistry, globalIDs map[controller.ActionID]bool) []controller.Action {
	var viewActions []controller.Action
	seen := make(map[controller.ActionID]bool)

	for _, action := range registry.GetHeaderActions() {
		// skip if this is a global action or duplicate
		if globalIDs[action.ID] || seen[action.ID] {
			continue
		}
		// skip plugin actions (they're handled separately)
		if strings.HasPrefix(string(action.ID), "plugin:") {
			continue
		}
		seen[action.ID] = true
		viewActions = append(viewActions, action)
	}

	return viewActions
}

// extractViewActionsFromModel extracts view-specific actions from model.HeaderAction slice,
// filtering out global actions and plugin actions.
func extractViewActionsFromModel(
	viewActions []model.HeaderAction,
	globalIDs map[controller.ActionID]bool,
) []controller.Action {
	var result []controller.Action
	seen := make(map[controller.ActionID]bool)

	for _, a := range viewActions {
		if !a.ShowInHeader {
			continue
		}

		actionID := controller.ActionID(a.ID)
		// skip if this is a global action or duplicate
		if globalIDs[actionID] || seen[actionID] {
			continue
		}
		// skip plugin actions (they're handled separately)
		if strings.HasPrefix(a.ID, "plugin:") {
			continue
		}

		seen[actionID] = true
		result = append(result, modelActionToControllerAction(a))
	}

	return result
}
