package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vorzela/vorm/pkg/errors"
)

// Validator handles configuration validation
type Validator struct {
	config *Config
}

// NewValidator creates a new configuration validator
func NewValidator(config *Config) *Validator {
	return &Validator{config: config}
}

// Validate validates the entire configuration
func (v *Validator) Validate() error {
	if err := v.validateDatabase(); err != nil {
		return err
	}

	if err := v.validateMigration(); err != nil {
		return err
	}

	if err := v.validateLogging(); err != nil {
		return err
	}

	return nil
}

// validateDatabase validates database configuration
func (v *Validator) validateDatabase() error {
	db := v.config.Database

	if db.Connection == "" {
		return errors.NewValidationError("Database connection type is required", "connection field cannot be empty")
	}

	if db.Connection != "postgres" {
		return errors.NewValidationError("Unsupported database connection", fmt.Sprintf("only 'postgres' is supported, got '%s'", db.Connection))
	}

	if db.Host == "" {
		return errors.NewValidationError("Database host is required", "host field cannot be empty")
	}

	if db.Port <= 0 || db.Port > 65535 {
		return errors.NewValidationError("Invalid database port", fmt.Sprintf("port must be between 1 and 65535, got %d", db.Port))
	}

	if db.Database == "" {
		return errors.NewValidationError("Database name is required", "database field cannot be empty")
	}

	if db.Username == "" {
		return errors.NewValidationError("Database username is required", "username field cannot be empty")
	}

	// Password can be empty for some authentication methods
	// if db.Password == "" {
	//     return errors.NewValidationError("Database password is required", "password field cannot be empty")
	// }

	return nil
}

// validateMigration validates migration configuration
func (v *Validator) validateMigration() error {
	migration := v.config.Migration

	if migration.Table == "" {
		return errors.NewValidationError("Migration table name is required", "table field cannot be empty")
	}

	if migration.Directory == "" {
		return errors.NewValidationError("Migration directory is required", "directory field cannot be empty")
	}

	// Check if migrations directory exists or can be created
	migrationsPath := v.config.GetMigrationsPath()
	if err := v.ensureDirectoryExists(migrationsPath); err != nil {
		return errors.NewValidationError("Migration directory validation failed", err.Error())
	}

	if migration.Timezone == "" {
		return errors.NewValidationError("Migration timezone is required", "timezone field cannot be empty")
	}

	return nil
}

// validateLogging validates logging configuration
func (v *Validator) validateLogging() error {
	logging := v.config.Logging

	if logging.Enabled {
		if logging.Directory == "" {
			return errors.NewValidationError("Log directory is required when logging is enabled", "directory field cannot be empty")
		}

		if logging.Filename == "" {
			return errors.NewValidationError("Log filename is required when logging is enabled", "filename field cannot be empty")
		}

		// Check if logs directory exists or can be created
		logsPath := v.config.GetLogsPath()
		if err := v.ensureDirectoryExists(logsPath); err != nil {
			return errors.NewValidationError("Log directory validation failed", err.Error())
		}

		validLevels := map[string]bool{
			"debug":   true,
			"info":    true,
			"warning": true,
			"error":   true,
			"fatal":   true,
		}

		if !validLevels[logging.Level] {
			return errors.NewValidationError("Invalid log level", fmt.Sprintf("level must be one of: debug, info, warning, error, fatal. Got: %s", logging.Level))
		}

		if logging.MaxSize <= 0 {
			return errors.NewValidationError("Invalid log max size", "max_size must be greater than 0")
		}

		if logging.MaxBackups < 0 {
			return errors.NewValidationError("Invalid log max backups", "max_backups must be 0 or greater")
		}

		if logging.MaxAge < 0 {
			return errors.NewValidationError("Invalid log max age", "max_age must be 0 or greater")
		}
	}

	return nil
}

// ensureDirectoryExists checks if directory exists and creates it if it doesn't
func (v *Validator) ensureDirectoryExists(path string) error {
	// Check if directory exists
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path '%s' exists but is not a directory", path)
		}
		return nil
	}

	// Try to create directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %v", path, err)
	}

	return nil
}

// ValidateEnvironment validates that the environment is properly set
func (v *Validator) ValidateEnvironment() error {
	validEnvs := map[string]bool{
		"development": true,
		"dev":         true,
		"staging":     true,
		"stage":       true,
		"production":  true,
		"prod":        true,
		"testing":     true,
		"test":        true,
	}

	env := v.config.Environment
	if env == "" {
		env = "development" // default
	}

	if !validEnvs[env] {
		return errors.NewValidationError("Invalid environment", fmt.Sprintf("environment must be one of: development, staging, production, testing. Got: %s", env))
	}

	return nil
}

// ValidateFilePermissions validates that we have necessary file permissions
func (v *Validator) ValidateFilePermissions() error {
	// Check migrations directory permissions
	migrationsPath := v.config.GetMigrationsPath()
	if err := v.checkDirectoryPermissions(migrationsPath); err != nil {
		return errors.NewValidationError("Migration directory permission error", err.Error())
	}

	// Check logs directory permissions if logging is enabled
	if v.config.Logging.Enabled {
		logsPath := v.config.GetLogsPath()
		if err := v.checkDirectoryPermissions(logsPath); err != nil {
			return errors.NewValidationError("Log directory permission error", err.Error())
		}
	}

	return nil
}

// checkDirectoryPermissions checks if we can read/write to a directory
func (v *Validator) checkDirectoryPermissions(path string) error {
	// Check if we can read the directory
	if _, err := os.ReadDir(path); err != nil {
		return fmt.Errorf("cannot read directory '%s': %v", path, err)
	}

	// Check if we can write to the directory by creating a temp file
	tempFile := filepath.Join(path, ".vorm_permission_test")
	if file, err := os.Create(tempFile); err != nil {
		return fmt.Errorf("cannot write to directory '%s': %v", path, err)
	} else {
		file.Close()
		os.Remove(tempFile) // Clean up
	}

	return nil
}
