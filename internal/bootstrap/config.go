package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// LoadConfig loads the application configuration.
// Returns an error if configuration loading fails.
func LoadConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return cfg, nil
}
