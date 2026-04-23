package bootstrap

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitHeaderAndLayoutModels creates the header config and layout model with
// persisted visibility preferences applied.
func InitHeaderAndLayoutModels() (*model.HeaderConfig, *model.LayoutModel) {
	headerConfig := model.NewHeaderConfig()
	layoutModel := model.NewLayoutModel()

	// Load user preference from saved config
	headerVisible := config.GetHeaderVisible()
	headerConfig.SetUserPreference(headerVisible)
	headerConfig.SetVisible(headerVisible)

	return headerConfig, layoutModel
}

// InitStatuslineModel creates and populates the statusline config with base stats (version, workflow scope, branch, user).
func InitStatuslineModel(tikiStore *tikistore.TikiStore, workflowScope config.Scope) *model.StatuslineConfig {
	cfg := model.NewStatuslineConfig()
	cfg.SetLeftStat("Version", config.Version, 0)
	cfg.SetLeftStat("Workflow", config.WorkflowScopeLabel(workflowScope), 1)
	for _, stat := range tikiStore.GetStats() {
		value := stat.Value
		if stat.Name == "Branch" {
			value = "\ue0a0 " + value
		}
		cfg.SetLeftStat(stat.Name, value, stat.Order)
	}
	return cfg
}
