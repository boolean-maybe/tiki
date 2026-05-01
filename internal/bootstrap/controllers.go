package bootstrap

import (
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

// Controllers holds all application controllers.
type Controllers struct {
	Nav     *controller.NavigationController
	Task    *controller.TaskController
	Plugins map[string]controller.PluginControllerInterface
}

// BuildControllers constructs navigation/domain/plugin controllers for the application.
// globalActions carries the workflow's top-level `actions:` list so non-board
// views can reach them too (6A wires `kind: view` navigation; `kind: ruki`
// dispatch from non-board views lands in 6B).
func BuildControllers(
	app *tview.Application,
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	plugins []plugin.Plugin,
	globalActions []plugin.PluginAction,
	pluginConfigs map[string]*model.PluginConfig,
	statuslineConfig *model.StatuslineConfig,
	schema ruki.Schema,
) *Controllers {
	navController := controller.NewNavigationController(app)
	taskController := controller.NewTaskController(taskStore, mutationGate, navController, statuslineConfig)

	pluginControllers := make(map[string]controller.PluginControllerInterface)
	for _, p := range plugins {
		switch p.GetKind() {
		case plugin.KindBoard, plugin.KindList:
			tp, ok := p.(*plugin.TikiPlugin)
			if !ok {
				continue
			}
			pluginControllers[p.GetName()] = controller.NewPluginController(
				taskStore,
				mutationGate,
				pluginConfigs[p.GetName()],
				tp,
				navController,
				statuslineConfig,
				schema,
			)
		case plugin.KindWiki, plugin.KindDetail:
			pluginControllers[p.GetName()] = controller.NewDokiController(
				p, navController, statuslineConfig, globalActions,
				taskStore, mutationGate, schema,
			)
		}
	}

	return &Controllers{
		Nav:     navController,
		Task:    taskController,
		Plugins: pluginControllers,
	}
}
