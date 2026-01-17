package bootstrap

import (
	"log"

	"github.com/boolean-maybe/tiki/config"
)

// LoadConfigOrExit loads the application configuration.
// If configuration loading fails, it logs a fatal error and exits.
func LoadConfigOrExit() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	return cfg
}
