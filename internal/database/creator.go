package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/pkg/errors"
)

// Creator handles database creation and deletion operations
type Creator struct {
	config *config.Config
}

// NewCreator creates a new database creator
func NewCreator(cfg *config.Config) *Creator {
	return &Creator{
		config: cfg,
	}
}

// CreateDatabase creates the database if it doesn't exist
func (c *Creator) CreateDatabase(ctx context.Context) error {
	// Connect to PostgreSQL server (not specific database)
	conn, err := pgx.Connect(ctx, c.config.GetAdminDSN())
	if err != nil {
		return errors.NewConnectionError("Failed to connect to PostgreSQL server", err.Error())
	}
	defer conn.Close(ctx)

	// Check if database exists
	exists, err := c.databaseExists(ctx, conn, c.config.Database.Database)
	if err != nil {
		return err
	}

	if exists {
		return nil // Database already exists
	}

	// Create database
	sql := fmt.Sprintf("CREATE DATABASE %s OWNER %s",
		pgx.Identifier{c.config.Database.Database}.Sanitize(),
		pgx.Identifier{c.config.Database.Username}.Sanitize())
	if _, err := conn.Exec(ctx, sql); err != nil {
		return errors.NewMigrationError("Failed to create database", err.Error(), "")
	}

	// Grant all privileges on the database to the user
	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		pgx.Identifier{c.config.Database.Database}.Sanitize(),
		pgx.Identifier{c.config.Database.Username}.Sanitize())
	if _, err := conn.Exec(ctx, grantSQL); err != nil {
		// Log warning but don't fail - user might already have privileges
		// This is common when using superuser accounts
	}

	// Connect to the new database to set schema privileges
	dbConn, err := pgx.Connect(ctx, c.config.GetDSN())
	if err == nil {
		defer dbConn.Close(ctx)

		// Grant privileges on public schema
		schemaSQL := fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s",
			pgx.Identifier{c.config.Database.Username}.Sanitize())
		dbConn.Exec(ctx, schemaSQL) // Ignore errors as this might already be set

		// Grant default privileges for future tables
		defaultSQL := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s",
			pgx.Identifier{c.config.Database.Username}.Sanitize())
		dbConn.Exec(ctx, defaultSQL) // Ignore errors as this might already be set
	}

	return nil
}

// DropDatabase drops the database (used with confirmation)
// DISABLED in production environment for safety
func (c *Creator) DropDatabase(ctx context.Context) error {
	// SAFETY: Disable database dropping in production
	if c.config.IsProduction() {
		return errors.NewPermissionError(
			"Database drop operation is disabled in production",
			"This is a safety measure to prevent accidental data loss in production environments",
		)
	}

	// Connect to PostgreSQL server (not specific database)
	conn, err := pgx.Connect(ctx, c.config.GetAdminDSN())
	if err != nil {
		return errors.NewConnectionError("Failed to connect to PostgreSQL server", err.Error())
	}
	defer conn.Close(ctx)

	// Check if database exists
	exists, err := c.databaseExists(ctx, conn, c.config.Database.Database)
	if err != nil {
		return err
	}

	if !exists {
		return nil // Database doesn't exist
	}

	// Terminate active connections to the database
	terminateSQL := `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`
	if _, err := conn.Exec(ctx, terminateSQL, c.config.Database.Database); err != nil {
		return errors.NewMigrationError("Failed to terminate database connections", err.Error(), "")
	}

	// Drop database
	sql := fmt.Sprintf("DROP DATABASE %s", pgx.Identifier{c.config.Database.Database}.Sanitize())
	if _, err := conn.Exec(ctx, sql); err != nil {
		return errors.NewMigrationError("Failed to drop database", err.Error(), "")
	}

	return nil
}

// DatabaseExists checks if the database exists
func (c *Creator) DatabaseExists(ctx context.Context) (bool, error) {
	// Connect to PostgreSQL server
	conn, err := pgx.Connect(ctx, c.config.GetAdminDSN())
	if err != nil {
		return false, errors.NewConnectionError("Failed to connect to PostgreSQL server", err.Error())
	}
	defer conn.Close(ctx)

	return c.databaseExists(ctx, conn, c.config.Database.Database)
}

// databaseExists checks if a database exists
func (c *Creator) databaseExists(ctx context.Context, conn *pgx.Conn, dbName string) (bool, error) {
	var exists bool
	sql := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err := conn.QueryRow(ctx, sql, dbName).Scan(&exists)
	if err != nil {
		return false, errors.NewMigrationError("Failed to check database existence", err.Error(), "")
	}
	return exists, nil
}

// CreateMigrationsTable creates the schema_migrations table
func (c *Creator) CreateMigrationsTable(ctx context.Context, conn *Connection) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			migration VARCHAR(255) NOT NULL UNIQUE,
			batch INTEGER NOT NULL,
			executed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			execution_time INTEGER NOT NULL, -- milliseconds
			checksum VARCHAR(64) NOT NULL    -- SHA256 of migration file
		)`, pgx.Identifier{c.config.Migration.Table}.Sanitize())

	if err := conn.Exec(ctx, sql); err != nil {
		return errors.NewMigrationError("Failed to create migrations table", err.Error(), "")
	}

	// Create performance indexes as specified in AINOTES.md
	indexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_batch ON %s(batch);
		CREATE INDEX IF NOT EXISTS idx_%s_executed_at ON %s(executed_at);
	`,
		c.config.Migration.Table, pgx.Identifier{c.config.Migration.Table}.Sanitize(),
		c.config.Migration.Table, pgx.Identifier{c.config.Migration.Table}.Sanitize())

	if err := conn.Exec(ctx, indexSQL); err != nil {
		return errors.NewMigrationError("Failed to create migration table indexes", err.Error(), "")
	}

	return nil
}

// ResetDatabase drops and recreates the database
func (c *Creator) ResetDatabase(ctx context.Context) error {
	// Drop database
	if err := c.DropDatabase(ctx); err != nil {
		return err
	}

	// Create database
	return c.CreateDatabase(ctx)
}

// GetDatabaseSize returns the size of the database in bytes
func (c *Creator) GetDatabaseSize(ctx context.Context, conn *Connection) (int64, error) {
	var size int64
	err := conn.QueryRow(ctx, "SELECT pg_database_size($1)", c.config.Database.Database).Scan(&size)
	if err != nil {
		return 0, errors.NewMigrationError("Failed to get database size", err.Error(), "")
	}
	return size, nil
}

// ListTables returns all tables in the current database
func (c *Creator) ListTables(ctx context.Context, conn *Connection) ([]string, error) {
	sql := `
		SELECT tablename 
		FROM pg_tables 
		WHERE schemaname = 'public' 
		ORDER BY tablename`

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, errors.NewMigrationError("Failed to scan table name", err.Error(), "")
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewMigrationError("Error reading tables", err.Error(), "")
	}

	return tables, nil
}

// DropAllTables drops all tables in the database (for fresh command)
func (c *Creator) DropAllTables(ctx context.Context, conn *Connection) error {
	// Get all tables
	tables, err := c.ListTables(ctx, conn)
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		return nil // No tables to drop
	}

	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Drop all tables (CASCADE to handle dependencies)
	for _, table := range tables {
		sql := fmt.Sprintf(`DROP TABLE IF EXISTS "%s" CASCADE`, table)
		if _, err := tx.Exec(ctx, sql); err != nil {
			return errors.NewMigrationError("Failed to drop table", err.Error(), table)
		}
	}

	// Commit transaction
	return tx.Commit(ctx)
}
