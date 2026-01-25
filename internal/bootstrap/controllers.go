package bootstrap

import (
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
)

// Controllers holds all application controllers.
type Controllers struct {
	Nav     *controller.NavigationController
	Task    *controller.TaskController
	Plugins map[string]controller.PluginControllerInterface
}

// BuildControllers constructs navigation/domain/plugin controllers for the application.
func BuildControllers(
	app *tview.Application,
	taskStore store.Store,
	plugins []plugin.Plugin,
	pluginConfigs map[string]*model.PluginConfig,
) *Controllers {
	navController := controller.NewNavigationController(app)
	taskController := controller.NewTaskController(taskStore, navController)

	pluginControllers := make(map[string]controller.PluginControllerInterface)
	for _, p := range plugins {
		if tp, ok := p.(*plugin.TikiPlugin); ok {
			pluginControllers[p.GetName()] = controller.NewPluginController(
				taskStore,
				pluginConfigs[p.GetName()],
				tp,
				navController,
			)
			continue
		}
		if dp, ok := p.(*plugin.DokiPlugin); ok {
			pluginControllers[p.GetName()] = controller.NewDokiController(dp, navController)
		}
	}

	return &Controllers{
		Nav:     navController,
		Task:    taskController,
		Plugins: pluginControllers,
	}
}
