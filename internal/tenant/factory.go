package tenant

import (
	"os"

	"personal-assistant/internal/config"
	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// NewTenantManager creates either a YAML-based or database-based tenant manager
// based on environment configuration or availability
func NewTenantManager(cfg *config.Config, logger *log.Logger) (domain.TenantManager, error) {
	// Check if we should use database-first approach
	useDatabaseManager := shouldUseDatabaseManager(cfg, logger)

	if useDatabaseManager {
		logger.Info().Msg("using database-first tenant manager")
		return NewDatabaseManager(cfg, logger)
	} else {
		logger.Info().Msg("using YAML-based tenant manager")
		return NewManager(cfg, logger)
	}
}

// shouldUseDatabaseManager determines whether to use the database manager
func shouldUseDatabaseManager(cfg *config.Config, logger *log.Logger) bool {
	// Environment variable override
	if dbFirst := os.Getenv("TENANT_CONFIG_SOURCE"); dbFirst != "" {
		return dbFirst == "database"
	}

	// Check if central database URL is configured
	if cfg.DatabaseURL == "" {
		logger.Debug().Msg("no central database URL configured, using YAML manager")
		return false
	}

	// Check if tenants.yaml exists - if not, assume database-first
	if _, err := os.Stat(cfg.TenantsConfigPath); os.IsNotExist(err) {
		logger.Info().
			Str("yaml_path", cfg.TenantsConfigPath).
			Msg("tenants.yaml not found, using database manager")
		return true
	}

	// Default to YAML for backward compatibility
	logger.Debug().Msg("tenants.yaml exists, using YAML manager (set TENANT_CONFIG_SOURCE=database to override)")
	return false
}

// NewManager creates the legacy YAML-based tenant manager
// This is kept for backward compatibility
func NewManager(cfg *config.Config, logger *log.Logger) (domain.TenantManager, error) {
	return newYAMLManager(cfg, logger)
}

// NewDatabaseManager creates the new database-first tenant manager
func NewDatabaseManager(cfg *config.Config, logger *log.Logger) (domain.TenantManager, error) {
	return newDatabaseManager(cfg, logger)
}