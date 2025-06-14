package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/vorzela/vorm/pkg/errors"
)

// Config represents the complete VORM configuration
type Config struct {
	Database    DatabaseConfig  `yaml:"database" mapstructure:"database"`
	Migration   MigrationConfig `yaml:"migration" mapstructure:"migration"`
	Logging     LoggingConfig   `yaml:"logging" mapstructure:"logging"`
	Environment string          `yaml:"environment" mapstructure:"environment"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Connection string `yaml:"connection" mapstructure:"connection"`
	Host       string `yaml:"host" mapstructure:"host"`
	Port       int    `yaml:"port" mapstructure:"port"`
	Database   string `yaml:"database" mapstructure:"database"`
	Username   string `yaml:"username" mapstructure:"username"`
	Password   string `yaml:"password" mapstructure:"password"`
	SSLMode    string `yaml:"sslmode" mapstructure:"sslmode"`
}

// MigrationConfig holds migration-specific settings
type MigrationConfig struct {
	Table     string `yaml:"table" mapstructure:"table"`
	Directory string `yaml:"directory" mapstructure:"directory"`
	Timezone  string `yaml:"timezone" mapstructure:"timezone"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	Directory  string `yaml:"directory" mapstructure:"directory"`
	Filename   string `yaml:"filename" mapstructure:"filename"`
	Level      string `yaml:"level" mapstructure:"level"`
	MaxSize    int    `yaml:"max_size" mapstructure:"max_size"`
	MaxBackups int    `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAge     int    `yaml:"max_age" mapstructure:"max_age"`
}

// Load loads configuration from config files and environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return nil, errors.NewValidationError("Failed to load .env file", err.Error())
		}
	}

	// Setup viper
	viper.SetConfigName("database")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("$HOME/.vorm")

	// Set defaults
	setDefaults()

	// Bind environment variables
	bindEnvVars()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, errors.NewValidationError("Failed to read config file", err.Error())
		}
	}

	// Unmarshal config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, errors.NewValidationError("Failed to unmarshal config", err.Error())
	}

	// Override with environment variables
	overrideWithEnv(&config)

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Database defaults
	viper.SetDefault("database.connection", "postgres")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.sslmode", "disable")

	// Migration defaults
	viper.SetDefault("migration.table", "schema_migrations")
	viper.SetDefault("migration.directory", "migrations")
	viper.SetDefault("migration.timezone", "UTC")

	// Logging defaults
	viper.SetDefault("logging.enabled", true)
	viper.SetDefault("logging.directory", "storage/logs")
	viper.SetDefault("logging.filename", "vorm.log")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 30)

	// Environment defaults
	viper.SetDefault("environment", "development")
}

// bindEnvVars binds environment variables to viper keys
func bindEnvVars() {
	viper.BindEnv("database.host", "VORM_DB_HOST")
	viper.BindEnv("database.port", "VORM_DB_PORT")
	viper.BindEnv("database.database", "VORM_DB_NAME")
	viper.BindEnv("database.username", "VORM_DB_USERNAME")
	viper.BindEnv("database.password", "VORM_DB_PASSWORD")
	viper.BindEnv("database.sslmode", "VORM_DB_SSLMODE")
	viper.BindEnv("environment", "VORM_ENVIRONMENT")
	viper.BindEnv("logging.level", "VORM_LOG_LEVEL")

	// Support for DATABASE_URL override
	viper.BindEnv("database_url", "DATABASE_URL")
}

// overrideWithEnv overrides config with environment variables
func overrideWithEnv(config *Config) {
	// Check for DATABASE_URL first (takes precedence)
	if databaseURL := os.Getenv("VORM_DATABASE_URL"); databaseURL != "" {
		if err := parseDatabaseURL(databaseURL, config); err == nil {
			return // Successfully parsed DATABASE_URL, skip individual env vars
		}
		// If DATABASE_URL parsing fails, fall back to individual env vars
	}

	// Individual environment variables with VORM_ prefix
	if host := os.Getenv("VORM_DB_HOST"); host != "" {
		config.Database.Host = host
	}
	if port := os.Getenv("VORM_DB_PORT"); port != "" {
		config.Database.Port = parseInt(port, 5432)
	}
	if database := os.Getenv("VORM_DB_NAME"); database != "" {
		config.Database.Database = database
	}
	if username := os.Getenv("VORM_DB_USERNAME"); username != "" {
		config.Database.Username = username
	}
	if password := os.Getenv("VORM_DB_PASSWORD"); password != "" {
		config.Database.Password = password
	}
	if sslmode := os.Getenv("VORM_DB_SSLMODE"); sslmode != "" {
		config.Database.SSLMode = sslmode
	}
	if env := os.Getenv("VORM_ENVIRONMENT"); env != "" {
		config.Environment = env
	}
	if level := os.Getenv("VORM_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
}

// parseInt safely parses a string to int with default fallback
func parseInt(s string, defaultVal int) int {
	if val := viper.Get("database.port"); val != nil {
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return defaultVal
}

// parseDatabaseURL parses a PostgreSQL DATABASE_URL and updates the config
func parseDatabaseURL(databaseURL string, config *Config) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return err
	}

	// Check if it's a PostgreSQL URL
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("unsupported database scheme: %s", u.Scheme)
	}

	// Extract components
	config.Database.Host = u.Hostname()

	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			config.Database.Port = port
		}
	}

	// Extract database name (remove leading slash)
	if u.Path != "" {
		config.Database.Database = strings.TrimPrefix(u.Path, "/")
	}

	// Extract username and password
	if u.User != nil {
		config.Database.Username = u.User.Username()
		if password, ok := u.User.Password(); ok {
			config.Database.Password = password
		}
	}

	// Extract SSL mode from query parameters
	if sslmode := u.Query().Get("sslmode"); sslmode != "" {
		config.Database.SSLMode = sslmode
	}

	return nil
}

// GetDSN returns the PostgreSQL connection string
func (c *Config) GetDSN() string {
	sslmode := c.Database.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.Username,
		c.Database.Password,
		c.Database.Database,
		sslmode,
	)
}

// GetAdminDSN returns DSN for administrative operations (without specific database)
func (c *Config) GetAdminDSN() string {
	sslmode := c.Database.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.Username,
		c.Database.Password,
		sslmode,
	)
}

// GetMigrationsPath returns the absolute path to migrations directory
func (c *Config) GetMigrationsPath() string {
	if filepath.IsAbs(c.Migration.Directory) {
		return c.Migration.Directory
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, c.Migration.Directory)
}

// GetLogsPath returns the absolute path to logs directory
func (c *Config) GetLogsPath() string {
	if filepath.IsAbs(c.Logging.Directory) {
		return c.Logging.Directory
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, c.Logging.Directory)
}

// IsProduction returns true if we're in production environment
func (c *Config) IsProduction() bool {
	env := strings.ToLower(c.Environment)
	return env == "production" || env == "prod"
}

// IsDevelopment returns true if we're in development environment
func (c *Config) IsDevelopment() bool {
	env := strings.ToLower(c.Environment)
	return env == "development" || env == "dev" || env == ""
}
