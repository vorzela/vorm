package migration

import (
	"context"
	"fmt"

	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/database"
	"github.com/vorzela/vorm/internal/logger"
)

// Manager coordinates all migration operations
type Manager struct {
	config    *config.Config
	conn      *database.Connection
	creator   *database.Creator
	generator *Generator
	executor  *Executor
	logger    *logger.Logger
}

// NewManager creates a new migration manager
func NewManager(cfg *config.Config, logger *logger.Logger) (*Manager, error) {
	conn := database.NewConnection(cfg)
	creator := database.NewCreator(cfg)
	generator := NewGenerator(cfg)

	return &Manager{
		config:    cfg,
		conn:      conn,
		creator:   creator,
		generator: generator,
		logger:    logger,
	}, nil
}

// Initialize sets up the migration system
func (m *Manager) Initialize(ctx context.Context) error {
	// Ensure database exists
	if err := m.creator.CreateDatabase(ctx); err != nil {
		return err
	}

	// Connect to database
	if err := m.conn.Connect(ctx); err != nil {
		return err
	}

	// Create executor after connection is established
	m.executor = NewExecutor(m.config, m.conn, m.logger)

	// Create migrations table
	if err := m.creator.CreateMigrationsTable(ctx, m.conn); err != nil {
		return err
	}

	m.logger.LogDatabaseConnection(m.config.Database.Database)
	return nil
}

// CreateMigration creates a new migration file
func (m *Manager) CreateMigration(name string) (*Migration, error) {
	m.logger.Info("Migration", fmt.Sprintf("Creating migration: %s", name))

	migration, err := m.generator.GenerateMigration(name)
	if err != nil {
		return nil, err
	}

	m.logger.Success("Migration", fmt.Sprintf("Created migration: %s", migration.Filename))
	return migration, nil
}

// RunMigrations executes pending migrations
func (m *Manager) RunMigrations(ctx context.Context, limit int) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	defer m.conn.Close(ctx)

	// Load all migrations
	allMigrations, err := m.generator.LoadMigrations()
	if err != nil {
		return err
	}

	// Get pending migrations
	pendingMigrations, err := m.executor.GetTracker().GetPendingMigrations(ctx, allMigrations)
	if err != nil {
		return err
	}

	// Verify checksums of already executed migrations
	for _, migration := range allMigrations {
		if err := m.executor.GetTracker().VerifyChecksum(ctx, migration); err != nil {
			return err
		}
	}

	return m.executor.RunMigrations(ctx, pendingMigrations, limit)
}

// RollbackMigrations rolls back migrations
func (m *Manager) RollbackMigrations(ctx context.Context, limit int) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	defer m.conn.Close(ctx)

	// Load all migrations
	allMigrations, err := m.generator.LoadMigrations()
	if err != nil {
		return err
	}

	// Get last batch for rollback
	lastBatch, err := m.executor.GetTracker().GetLastBatch(ctx)
	if err != nil {
		return err
	}

	if lastBatch == 0 {
		m.logger.Info("Migration", "No migrations to rollback")
		return nil
	}

	return m.executor.RollbackBatch(ctx, lastBatch, allMigrations)
}

// RollbackSteps rolls back a specific number of migration steps
func (m *Manager) RollbackSteps(ctx context.Context, steps int) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	defer m.conn.Close(ctx)

	// Load all migrations
	allMigrations, err := m.generator.LoadMigrations()
	if err != nil {
		return err
	}

	// Get executed migrations in reverse order
	executedMigrations, err := m.executor.GetTracker().GetExecutedMigrations(ctx)
	if err != nil {
		return err
	}

	if len(executedMigrations) == 0 {
		m.logger.Info("Migration", "No migrations to rollback")
		return nil
	}

	// Reverse order for rollback and limit steps
	for i := len(executedMigrations)/2 - 1; i >= 0; i-- {
		opp := len(executedMigrations) - 1 - i
		executedMigrations[i], executedMigrations[opp] = executedMigrations[opp], executedMigrations[i]
	}

	if steps > 0 && steps < len(executedMigrations) {
		executedMigrations = executedMigrations[:steps]
	}

	// Get migration files for Down SQL
	migrationsMap := make(map[string]*Migration)
	for _, migration := range allMigrations {
		migrationsMap[migration.Name] = migration
	}

	var migrationsToRollback []*Migration
	for _, executedMigration := range executedMigrations {
		if fullMigration, exists := migrationsMap[executedMigration.Name]; exists {
			migrationsToRollback = append(migrationsToRollback, fullMigration)
		}
	}

	return m.executor.RollbackMigrations(ctx, migrationsToRollback, 0)
}

// ResetAllMigrations rolls back all migrations
func (m *Manager) ResetAllMigrations(ctx context.Context) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	defer m.conn.Close(ctx)

	allMigrations, err := m.generator.LoadMigrations()
	if err != nil {
		return err
	}

	return m.executor.ResetAllMigrations(ctx, allMigrations)
}

// FreshMigrations drops all tables and re-runs migrations
func (m *Manager) FreshMigrations(ctx context.Context) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	defer m.conn.Close(ctx)

	// Drop all tables
	if err := m.creator.DropAllTables(ctx, m.conn); err != nil {
		return err
	}

	// Recreate migrations table
	if err := m.creator.CreateMigrationsTable(ctx, m.conn); err != nil {
		return err
	}

	// Run all migrations
	return m.RunMigrations(ctx, 0)
}

// GetMigrationStatus returns the status of all migrations
func (m *Manager) GetMigrationStatus(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.Initialize(ctx); err != nil {
		return nil, err
	}
	defer m.conn.Close(ctx)

	allMigrations, err := m.generator.LoadMigrations()
	if err != nil {
		return nil, err
	}

	return m.executor.GetTracker().GetMigrationStatus(ctx, allMigrations)
}

// ListMigrations returns all available migrations
func (m *Manager) ListMigrations() ([]*Migration, error) {
	return m.generator.LoadMigrations()
}

// GetMigrationHistory returns migration execution history
func (m *Manager) GetMigrationHistory(ctx context.Context) ([]*Migration, error) {
	if err := m.Initialize(ctx); err != nil {
		return nil, err
	}
	defer m.conn.Close(ctx)

	return m.executor.GetTracker().GetMigrationHistory(ctx)
}

// Close closes the database connection
func (m *Manager) Close(ctx context.Context) {
	if m.conn != nil {
		m.conn.Close(ctx)
	}
}
