package vorm

import (
	"context"

	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/logger"
	"github.com/vorzela/vorm/internal/migration"
	"github.com/vorzela/vorm/pkg/errors"
)

// Client is the main VORM client for programmatic access
type Client struct {
	config  *config.Config
	logger  *logger.Logger
	manager *migration.Manager
}

// NewClient creates a new VORM client
func NewClient(configPath string) (*Client, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, errors.NewValidationError("Failed to load configuration", err.Error())
	}

	// Validate configuration
	validator := config.NewValidator(cfg)
	if err := validator.Validate(); err != nil {
		return nil, err
	}

	// Create logger
	logger, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, errors.NewValidationError("Failed to create logger", err.Error())
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, logger)
	if err != nil {
		return nil, err
	}

	return &Client{
		config:  cfg,
		logger:  logger,
		manager: manager,
	}, nil
}

// CreateMigration creates a new migration file
func (c *Client) CreateMigration(name string) (*migration.Migration, error) {
	return c.manager.CreateMigration(name)
}

// Migrate runs pending migrations
func (c *Client) Migrate(ctx context.Context) error {
	return c.manager.RunMigrations(ctx, 0)
}

// MigrateSteps runs a specific number of pending migrations
func (c *Client) MigrateSteps(ctx context.Context, steps int) error {
	return c.manager.RunMigrations(ctx, steps)
}

// Rollback rolls back the last batch of migrations
func (c *Client) Rollback(ctx context.Context) error {
	return c.manager.RollbackMigrations(ctx, 0)
}

// RollbackSteps rolls back a specific number of migrations
func (c *Client) RollbackSteps(ctx context.Context, steps int) error {
	return c.manager.RollbackSteps(ctx, steps)
}

// Reset rolls back all migrations
func (c *Client) Reset(ctx context.Context) error {
	return c.manager.ResetAllMigrations(ctx)
}

// Fresh drops all tables and re-runs all migrations
func (c *Client) Fresh(ctx context.Context) error {
	return c.manager.FreshMigrations(ctx)
}

// Status returns the status of all migrations
func (c *Client) Status(ctx context.Context) ([]migration.MigrationStatus, error) {
	return c.manager.GetMigrationStatus(ctx)
}

// List returns all available migrations
func (c *Client) List() ([]*migration.Migration, error) {
	return c.manager.ListMigrations()
}

// History returns migration execution history
func (c *Client) History(ctx context.Context) ([]*migration.Migration, error) {
	return c.manager.GetMigrationHistory(ctx)
}

// Close closes the client and cleans up resources
func (c *Client) Close(ctx context.Context) error {
	if c.manager != nil {
		c.manager.Close(ctx)
	}
	if c.logger != nil {
		return c.logger.Close()
	}
	return nil
}

// GetConfig returns the current configuration
func (c *Client) GetConfig() *config.Config {
	return c.config
}
