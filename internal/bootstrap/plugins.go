package bootstrap

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
)

// LoadPlugins loads plugins from disk. Returns an error if workflow files
// exist but contain no valid view definitions.
func LoadPlugins() ([]plugin.Plugin, error) {
	plugins, err := plugin.LoadPlugins()
	if err != nil {
		return nil, err
	}
	if len(plugins) > 0 {
		slog.Info("loaded plugins", "count", len(plugins))
	}
	return plugins, nil
}

// InitPluginActionRegistry initializes the controller plugin action registry
// from loaded plugin activation keys.
func InitPluginActionRegistry(plugins []plugin.Plugin) {
	pluginInfos := make([]controller.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		pk, pr, pm := p.GetActivationKey()
		pluginInfos = append(pluginInfos, controller.PluginInfo{
			Name:     p.GetName(),
			Key:      pk,
			Rune:     pr,
			Modifier: pm,
		})
	}
	controller.InitPluginActions(pluginInfos)
}

// BuildPluginConfigsAndDefs builds per-plugin configs and a name->definition map
// for view/controller wiring.
func BuildPluginConfigsAndDefs(plugins []plugin.Plugin) (map[string]*model.PluginConfig, map[string]plugin.Plugin) {
	pluginConfigs := make(map[string]*model.PluginConfig)
	pluginDefs := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pc := model.NewPluginConfig(p.GetName())
		pc.SetConfigIndex(p.GetConfigIndex()) // Pass ConfigIndex for saving view mode changes

		if tp, ok := p.(*plugin.TikiPlugin); ok {
			if tp.ViewMode == "expanded" {
				pc.SetViewMode("expanded")
			}
			columns := make([]int, len(tp.Lanes))
			for i, lane := range tp.Lanes {
				columns[i] = lane.Columns
			}
			pc.SetLaneLayout(columns)
		}

		pluginConfigs[p.GetName()] = pc
		pluginDefs[p.GetName()] = p
	}
	return pluginConfigs, pluginDefs
}
