package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/database"
	"github.com/vorzela/vorm/pkg/errors"
)

// Tracker handles migration tracking in the database
type Tracker struct {
	config *config.Config
	conn   *database.Connection
}

// NewTracker creates a new migration tracker
func NewTracker(cfg *config.Config, conn *database.Connection) *Tracker {
	return &Tracker{
		config: cfg,
		conn:   conn,
	}
}

// GetExecutedMigrations returns all executed migrations from database
func (t *Tracker) GetExecutedMigrations(ctx context.Context) ([]*Migration, error) {
	sql := fmt.Sprintf(`
		SELECT id, migration, batch, executed_at, execution_time, checksum
		FROM %s
		ORDER BY id ASC
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	rows, err := t.conn.Query(ctx, sql)
	if err != nil {
		return nil, errors.NewMigrationError("Failed to get executed migrations", err.Error(), "")
	}
	defer rows.Close()

	var migrations []*Migration
	for rows.Next() {
		migration := &Migration{}
		err := rows.Scan(
			&migration.ID,
			&migration.Name,
			&migration.Batch,
			&migration.ExecutedAt,
			&migration.ExecutionTime,
			&migration.Checksum,
		)
		if err != nil {
			return nil, errors.NewMigrationError("Failed to scan migration row", err.Error(), "")
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// GetPendingMigrations returns migrations that haven't been executed
func (t *Tracker) GetPendingMigrations(ctx context.Context, allMigrations []*Migration) ([]*Migration, error) {
	executedMigrations, err := t.GetExecutedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	// Create map of executed migrations for quick lookup
	executed := make(map[string]bool)
	for _, migration := range executedMigrations {
		executed[migration.Name] = true
	}

	// Filter out executed migrations
	var pending []*Migration
	for _, migration := range allMigrations {
		if !executed[migration.Name] {
			pending = append(pending, migration)
		}
	}

	return pending, nil
}

// RecordMigration records a successful migration execution
func (t *Tracker) RecordMigration(ctx context.Context, migration *Migration, batch int, executionTime time.Duration) error {
	sql := fmt.Sprintf(`
		INSERT INTO %s (migration, batch, executed_at, execution_time, checksum)
		VALUES ($1, $2, $3, $4, $5)
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	_, err := t.conn.Conn().Exec(ctx, sql,
		migration.Name,
		batch,
		time.Now(),
		int(executionTime.Milliseconds()),
		migration.Checksum,
	)

	if err != nil {
		return errors.NewMigrationError("Failed to record migration", err.Error(), migration.Name)
	}

	return nil
}

// RemoveMigration removes a migration record (used for rollbacks)
func (t *Tracker) RemoveMigration(ctx context.Context, migration *Migration) error {
	sql := fmt.Sprintf(`
		DELETE FROM %s WHERE migration = $1
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	_, err := t.conn.Conn().Exec(ctx, sql, migration.Name)
	if err != nil {
		return errors.NewMigrationError("Failed to remove migration record", err.Error(), migration.Name)
	}

	return nil
}

// GetLastBatch returns the highest batch number
func (t *Tracker) GetLastBatch(ctx context.Context) (int, error) {
	sql := fmt.Sprintf(`
		SELECT COALESCE(MAX(batch), 0) FROM %s
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	var lastBatch int
	err := t.conn.QueryRow(ctx, sql).Scan(&lastBatch)
	if err != nil {
		return 0, errors.NewMigrationError("Failed to get last batch", err.Error(), "")
	}

	return lastBatch, nil
}

// GetMigrationsByBatch returns migrations from a specific batch
func (t *Tracker) GetMigrationsByBatch(ctx context.Context, batch int) ([]*Migration, error) {
	sql := fmt.Sprintf(`
		SELECT id, migration, batch, executed_at, execution_time, checksum
		FROM %s
		WHERE batch = $1
		ORDER BY id DESC
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	rows, err := t.conn.Query(ctx, sql, batch)
	if err != nil {
		return nil, errors.NewMigrationError("Failed to get migrations by batch", err.Error(), "")
	}
	defer rows.Close()

	var migrations []*Migration
	for rows.Next() {
		migration := &Migration{}
		err := rows.Scan(
			&migration.ID,
			&migration.Name,
			&migration.Batch,
			&migration.ExecutedAt,
			&migration.ExecutionTime,
			&migration.Checksum,
		)
		if err != nil {
			return nil, errors.NewMigrationError("Failed to scan migration row", err.Error(), "")
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// GetMigrationStatus returns the status of all migrations
func (t *Tracker) GetMigrationStatus(ctx context.Context, allMigrations []*Migration) ([]MigrationStatus, error) {
	executedMigrations, err := t.GetExecutedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	// Create map for quick lookup
	executedMap := make(map[string]*Migration)
	for _, migration := range executedMigrations {
		executedMap[migration.Name] = migration
	}

	var statuses []MigrationStatus
	for _, migration := range allMigrations {
		status := MigrationStatus{
			Migration: migration,
			Executed:  false,
		}

		if executed, exists := executedMap[migration.Name]; exists {
			status.Executed = true
			status.ExecutedAt = executed.ExecutedAt
			status.Batch = executed.Batch
			status.ExecutionTime = executed.ExecutionTime
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Migration     *Migration `json:"migration"`
	Executed      bool       `json:"executed"`
	ExecutedAt    time.Time  `json:"executed_at"`
	Batch         int        `json:"batch"`
	ExecutionTime int        `json:"execution_time"`
}

// VerifyChecksum verifies that a migration file hasn't been modified
func (t *Tracker) VerifyChecksum(ctx context.Context, migration *Migration) error {
	sql := fmt.Sprintf(`
		SELECT checksum FROM %s WHERE migration = $1
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	var storedChecksum string
	err := t.conn.QueryRow(ctx, sql, migration.Name).Scan(&storedChecksum)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Migration not executed yet, no need to verify
			return nil
		}
		return errors.NewMigrationError("Failed to get stored checksum", err.Error(), migration.Name)
	}

	if storedChecksum != migration.Checksum {
		return errors.NewValidationError(
			fmt.Sprintf("Migration %s has been modified", migration.Name),
			fmt.Sprintf("stored checksum: %s, current checksum: %s", storedChecksum, migration.Checksum),
		)
	}

	return nil
}

// GetMigrationHistory returns the complete migration history
func (t *Tracker) GetMigrationHistory(ctx context.Context) ([]*Migration, error) {
	sql := fmt.Sprintf(`
		SELECT id, migration, batch, executed_at, execution_time, checksum
		FROM %s
		ORDER BY executed_at DESC
	`, pgx.Identifier{t.config.Migration.Table}.Sanitize())

	rows, err := t.conn.Query(ctx, sql)
	if err != nil {
		return nil, errors.NewMigrationError("Failed to get migration history", err.Error(), "")
	}
	defer rows.Close()

	var migrations []*Migration
	for rows.Next() {
		migration := &Migration{}
		err := rows.Scan(
			&migration.ID,
			&migration.Name,
			&migration.Batch,
			&migration.ExecutedAt,
			&migration.ExecutionTime,
			&migration.Checksum,
		)
		if err != nil {
			return nil, errors.NewMigrationError("Failed to scan migration row", err.Error(), "")
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}
