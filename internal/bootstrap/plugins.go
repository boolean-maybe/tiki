package bootstrap

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
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
	}
	controller.InitPluginActions(pluginInfos)
	// Successor to the legacy `viewID == TaskDetailViewID` comparison: the
	// controller package can't depend on plugin, so bootstrap installs a
	// predicate that recognizes any `kind: detail` plugin's view id.
	controller.SetDetailViewIDPredicate(func(id model.ViewID) bool {
		_, ok := detailViewIDs[id]
		return ok
	})
}

// BuildPluginConfigsAndDefs builds per-plugin configs and a name->definition map
// for view/controller wiring.
func BuildPluginConfigsAndDefs(plugins []plugin.Plugin) (map[string]*model.PluginConfig, map[string]plugin.Plugin) {
	pluginConfigs := make(map[string]*model.PluginConfig)
	pluginDefs := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pc := model.NewPluginConfig(p.GetName())

		if tp, ok := p.(*plugin.TikiPlugin); ok {
			if tp.Mode == "expanded" {
				pc.SetViewMode("expanded")
			}
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
