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

// InitHeaderBaseStats initializes base header stats that are always visible regardless of view.
func InitHeaderBaseStats(headerConfig *model.HeaderConfig, tikiStore *tikistore.TikiStore) {
	headerConfig.SetBaseStat("Version", config.Version, 0)
	headerConfig.SetBaseStat("Mode", "kanban", 1)
	headerConfig.SetBaseStat("Store", "local", 2)
	for _, stat := range tikiStore.GetStats() {
		headerConfig.SetBaseStat(stat.Name, stat.Value, stat.Order)
	}
}

// InitStatuslineModel creates and populates the statusline config with base stats (version, branch, user).
func InitStatuslineModel(tikiStore *tikistore.TikiStore) *model.StatuslineConfig {
	cfg := model.NewStatuslineConfig()
	cfg.SetLeftStat("Version", config.Version, 0)
	for _, stat := range tikiStore.GetStats() {
		value := stat.Value
		if stat.Name == "Branch" {
			value = "\ue0a0 " + value
		}
		cfg.SetLeftStat(stat.Name, value, stat.Order)
	}
	return cfg
}
