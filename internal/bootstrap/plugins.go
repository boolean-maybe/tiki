package bootstrap

import (
	"log/slog"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
)

// LoadPlugins loads plugins and the workflow's top-level global actions from
// disk. Returns an error if workflow files exist but contain no valid view
// definitions. Global actions are returned separately so the controller
// layer can thread them into non-board views (where globals are not merged
// into per-view Actions slices).
func LoadPlugins(schema ruki.Schema) ([]plugin.Plugin, []plugin.PluginAction, error) {
	plugins, globals, err := plugin.LoadPluginsAndGlobals(schema)
	if err != nil {
		return nil, nil, err
	}
	if len(plugins) > 0 {
		slog.Info("loaded plugins", "count", len(plugins), "global_actions", len(globals))
	}
	return plugins, globals, nil
}

// InitPluginActionRegistry initializes the controller plugin action registry
// from loaded plugin activation keys.
func InitPluginActionRegistry(plugins []plugin.Plugin) {
	pluginInfos := make([]controller.PluginInfo, 0, len(plugins))
	detailViewIDs := make(map[model.ViewID]struct{})
	singleLaneViewIDs := make(map[model.ViewID]struct{})
	moveableViewIDs := make(map[model.ViewID]struct{})
	for _, p := range plugins {
		pk, pr, pm := p.GetActivationKey()
		pluginInfos = append(pluginInfos, controller.PluginInfo{
			Name:     p.GetName(),
			Key:      pk,
			Rune:     pr,
			Modifier: pm,
			// Surface the view's own require: so the activation-key gate
			// honors it (6B.15).
			Require: p.GetRequire(),
		})
		if p.GetKind() == plugin.KindDetail {
			detailViewIDs[model.MakePluginViewID(p.GetName())] = struct{}{}
		}
		wp, ok := p.(*plugin.WorkflowPlugin)
		if !ok {
			continue
		}
		if len(wp.Lanes) <= 1 {
			singleLaneViewIDs[model.MakePluginViewID(p.GetName())] = struct{}{}
		}
		if anyLaneHasMoveAction(wp.Lanes) {
			moveableViewIDs[model.MakePluginViewID(p.GetName())] = struct{}{}
		}
	}
	controller.InitPluginActions(pluginInfos)
	// the controller package can't depend on plugin, so bootstrap installs a
	// predicate that recognizes any `kind: detail` plugin's view id.
	controller.SetDetailViewIDPredicate(func(id model.ViewID) bool {
		_, ok := detailViewIDs[id]
		return ok
	})
	// gates workflow actions that require a Detail view: the action is
	// silently absent when the active workflow declares no Detail plugin.
	hasDetailPlugin := len(detailViewIDs) > 0
	controller.SetDetailPluginPredicate(func() bool { return hasDetailPlugin })

	// gates Move ←/→ on single-lane board/list views: lane navigation is
	// meaningless when there is one lane or fewer.
	controller.SetSingleLanePredicate(func(id model.ViewID) bool {
		_, ok := singleLaneViewIDs[id]
		return ok
	})

	// gates Move ←/→ on boards whose lanes carry no move action: filter-only
	// lanes (e.g. SLA Watch's dueBy ranges) have no field to set, so the move
	// would silently no-op — hide it there instead of advertising a dead key.
	controller.SetLaneMoveablePredicate(func(id model.ViewID) bool {
		_, ok := moveableViewIDs[id]
		return ok
	})
}

// anyLaneHasMoveAction reports whether at least one lane declares a move
// action. A board with no lane actions cannot relocate a tiki via Move ←/→.
func anyLaneHasMoveAction(lanes []plugin.TikiLane) bool {
	for i := range lanes {
		if lanes[i].Action != nil {
			return true
		}
	}
	return false
}

// BuildPluginConfigsAndDefs builds per-plugin configs and a name->definition map
// for view/controller wiring.
func BuildPluginConfigsAndDefs(plugins []plugin.Plugin) (map[string]*model.PluginConfig, map[string]plugin.Plugin) {
	pluginConfigs := make(map[string]*model.PluginConfig)
	pluginDefs := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pc := model.NewPluginConfig(p.GetName())

		if tp, ok := p.(*plugin.WorkflowPlugin); ok {
			columns := make([]int, len(tp.Lanes))
			widths := make([]int, len(tp.Lanes))
			for i, lane := range tp.Lanes {
				columns[i] = lane.Columns
				widths[i] = lane.Width
			}
			pc.SetLaneLayout(columns, widths)
		}

		pluginConfigs[p.GetName()] = pc
		pluginDefs[p.GetName()] = p
	}
	return pluginConfigs, pluginDefs
}
