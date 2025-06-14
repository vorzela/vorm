package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/database"
	"github.com/vorzela/vorm/internal/logger"
	"github.com/vorzela/vorm/pkg/errors"
)

// Executor handles migration execution
type Executor struct {
	config  *config.Config
	conn    *database.Connection
	tracker *Tracker
	logger  *logger.Logger
}

// NewExecutor creates a new migration executor
func NewExecutor(cfg *config.Config, conn *database.Connection, logger *logger.Logger) *Executor {
	tracker := NewTracker(cfg, conn)
	return &Executor{
		config:  cfg,
		conn:    conn,
		tracker: tracker,
		logger:  logger,
	}
}

// RunMigrations executes pending migrations
func (e *Executor) RunMigrations(ctx context.Context, migrations []*Migration, limit int) error {
	if len(migrations) == 0 {
		e.logger.Info("Migration", "No pending migrations to run")
		return nil
	}

	// Get next batch number
	lastBatch, err := e.tracker.GetLastBatch(ctx)
	if err != nil {
		return err
	}
	nextBatch := lastBatch + 1

	// Limit migrations if specified
	if limit > 0 && limit < len(migrations) {
		migrations = migrations[:limit]
	}

	e.logger.Info("Migration", fmt.Sprintf("Running %d migrations in batch %d", len(migrations), nextBatch))

	for _, migration := range migrations {
		if err := e.runSingleMigration(ctx, migration, nextBatch); err != nil {
			e.logger.LogMigrationError(migration.Name, err)
			return err
		}
	}

	e.logger.Success("Migration", fmt.Sprintf("Successfully ran %d migrations", len(migrations)))
	return nil
}

// runSingleMigration executes a single migration
func (e *Executor) runSingleMigration(ctx context.Context, migration *Migration, batch int) error {
	e.logger.LogMigrationStart(migration.Name)

	// Start timing
	start := time.Now()

	// Begin transaction for atomic migration
	tx, err := e.conn.Begin(ctx)
	if err != nil {
		return errors.NewMigrationError("Failed to begin transaction", err.Error(), migration.Name)
	}
	defer tx.Rollback(ctx)

	// Execute migration SQL
	if err := e.executeMigrationSQL(ctx, tx, migration.UpSQL, migration.Name); err != nil {
		return err
	}

	// Record migration in tracking table
	executionTime := time.Since(start)
	if err := e.tracker.RecordMigration(ctx, migration, batch, executionTime); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return errors.NewMigrationError("Failed to commit migration", err.Error(), migration.Name)
	}

	e.logger.LogMigrationSuccess(migration.Name, executionTime)
	return nil
}

// RollbackMigrations rolls back migrations
func (e *Executor) RollbackMigrations(ctx context.Context, migrations []*Migration, limit int) error {
	if len(migrations) == 0 {
		e.logger.Info("Migration", "No migrations to rollback")
		return nil
	}

	// Limit rollbacks if specified
	if limit > 0 && limit < len(migrations) {
		migrations = migrations[:limit]
	}

	e.logger.Warning("Migration", fmt.Sprintf("Rolling back %d migrations", len(migrations)))

	for _, migration := range migrations {
		if err := e.rollbackSingleMigration(ctx, migration); err != nil {
			e.logger.LogMigrationError(migration.Name, err)
			return err
		}
	}

	e.logger.Success("Migration", fmt.Sprintf("Successfully rolled back %d migrations", len(migrations)))
	return nil
}

// rollbackSingleMigration rolls back a single migration
func (e *Executor) rollbackSingleMigration(ctx context.Context, migration *Migration) error {
	e.logger.LogRollbackStart(migration.Name)

	// Begin transaction for atomic rollback
	tx, err := e.conn.Begin(ctx)
	if err != nil {
		return errors.NewMigrationError("Failed to begin transaction", err.Error(), migration.Name)
	}
	defer tx.Rollback(ctx)

	// Execute rollback SQL
	if err := e.executeMigrationSQL(ctx, tx, migration.DownSQL, migration.Name); err != nil {
		return err
	}

	// Remove migration record
	if err := e.tracker.RemoveMigration(ctx, migration); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return errors.NewMigrationError("Failed to commit rollback", err.Error(), migration.Name)
	}

	e.logger.LogRollbackSuccess(migration.Name)
	return nil
}

// executeMigrationSQL executes SQL statements within a transaction
func (e *Executor) executeMigrationSQL(ctx context.Context, tx pgx.Tx, sql, migrationName string) error {
	// Skip empty SQL
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil
	}

	// Split SQL by statements (basic implementation)
	statements := e.splitSQLStatements(sql)

	for i, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(statement, "--") {
			continue
		}

		if _, err := tx.Exec(ctx, statement); err != nil {
			return errors.NewMigrationError(
				fmt.Sprintf("Failed to execute SQL statement %d", i+1),
				err.Error(),
				migrationName,
			)
		}
	}

	return nil
}

// splitSQLStatements splits SQL into individual statements
// This is a basic implementation - more sophisticated parsing might be needed
func (e *Executor) splitSQLStatements(sql string) []string {
	// Split on semicolons, but be careful about semicolons in strings
	statements := strings.Split(sql, ";")

	var result []string
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}

// RollbackBatch rolls back all migrations from a specific batch
func (e *Executor) RollbackBatch(ctx context.Context, batch int, allMigrations []*Migration) error {
	// Get migrations from the batch
	batchMigrations, err := e.tracker.GetMigrationsByBatch(ctx, batch)
	if err != nil {
		return err
	}

	if len(batchMigrations) == 0 {
		e.logger.Info("Migration", fmt.Sprintf("No migrations found in batch %d", batch))
		return nil
	}

	// Load migration files to get Down SQL
	migrationsMap := make(map[string]*Migration)
	for _, migration := range allMigrations {
		migrationsMap[migration.Name] = migration
	}

	// Prepare migrations for rollback with Down SQL
	var migrationsToRollback []*Migration
	for _, batchMigration := range batchMigrations {
		if fullMigration, exists := migrationsMap[batchMigration.Name]; exists {
			migrationsToRollback = append(migrationsToRollback, fullMigration)
		}
	}

	return e.RollbackMigrations(ctx, migrationsToRollback, 0)
}

// ResetAllMigrations rolls back all migrations
func (e *Executor) ResetAllMigrations(ctx context.Context, allMigrations []*Migration) error {
	// Get all executed migrations in reverse order
	executedMigrations, err := e.tracker.GetExecutedMigrations(ctx)
	if err != nil {
		return err
	}

	if len(executedMigrations) == 0 {
		e.logger.Info("Migration", "No migrations to reset")
		return nil
	}

	// Reverse the order for rollback
	for i := len(executedMigrations)/2 - 1; i >= 0; i-- {
		opp := len(executedMigrations) - 1 - i
		executedMigrations[i], executedMigrations[opp] = executedMigrations[opp], executedMigrations[i]
	}

	// Load migration files to get Down SQL
	migrationsMap := make(map[string]*Migration)
	for _, migration := range allMigrations {
		migrationsMap[migration.Name] = migration
	}

	// Prepare migrations for rollback with Down SQL
	var migrationsToRollback []*Migration
	for _, executedMigration := range executedMigrations {
		if fullMigration, exists := migrationsMap[executedMigration.Name]; exists {
			migrationsToRollback = append(migrationsToRollback, fullMigration)
		}
	}

	return e.RollbackMigrations(ctx, migrationsToRollback, 0)
}

// GetTracker returns the migration tracker
func (e *Executor) GetTracker() *Tracker {
	return e.tracker
}
