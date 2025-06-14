package database

import (
	"context"

	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/pkg/errors"
)

// Validator handles database validation operations
type Validator struct {
	config *config.Config
}

// NewValidator creates a new database validator
func NewValidator(cfg *config.Config) *Validator {
	return &Validator{
		config: cfg,
	}
}

// ValidateConnection validates the database connection
func (v *Validator) ValidateConnection(ctx context.Context) error {
	conn := NewConnection(v.config)

	if err := conn.Connect(ctx); err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Test the connection
	return conn.TestConnection(ctx)
}

// ValidateDatabase validates that the database exists and is accessible
func (v *Validator) ValidateDatabase(ctx context.Context) error {
	creator := NewCreator(v.config)

	// Check if database exists
	exists, err := creator.DatabaseExists(ctx)
	if err != nil {
		return err
	}

	if !exists {
		return errors.NewValidationError("Database does not exist",
			"Run 'vorm db:create' to create the database")
	}

	return nil
}

// ValidatePermissions validates that we have necessary database permissions
func (v *Validator) ValidatePermissions(ctx context.Context) error {
	conn := NewConnection(v.config)

	if err := conn.Connect(ctx); err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Check if we can create tables
	if err := v.checkCreateTablePermission(ctx, conn); err != nil {
		return err
	}

	// Check if we can create indexes
	if err := v.checkCreateIndexPermission(ctx, conn); err != nil {
		return err
	}

	return nil
}

// checkCreateTablePermission checks if we can create tables
func (v *Validator) checkCreateTablePermission(ctx context.Context, conn *Connection) error {
	// Try to create a temporary table
	sql := `CREATE TEMPORARY TABLE vorm_permission_test (id INTEGER)`
	if err := conn.Exec(ctx, sql); err != nil {
		return errors.NewPermissionError("No CREATE TABLE permission", err.Error())
	}

	// Clean up
	sql = `DROP TABLE IF EXISTS vorm_permission_test`
	conn.Exec(ctx, sql) // Ignore error on cleanup

	return nil
}

// checkCreateIndexPermission checks if we can create indexes
func (v *Validator) checkCreateIndexPermission(ctx context.Context, conn *Connection) error {
	// Create a temporary table first
	sql := `CREATE TEMPORARY TABLE vorm_index_test (id INTEGER)`
	if err := conn.Exec(ctx, sql); err != nil {
		return errors.NewPermissionError("Cannot create temporary table for index test", err.Error())
	}

	// Try to create an index
	sql = `CREATE INDEX vorm_idx_test ON vorm_index_test (id)`
	if err := conn.Exec(ctx, sql); err != nil {
		return errors.NewPermissionError("No CREATE INDEX permission", err.Error())
	}

	// Clean up
	sql = `DROP TABLE IF EXISTS vorm_index_test`
	conn.Exec(ctx, sql) // Ignore error on cleanup

	return nil
}

// ValidateSchema validates the migration schema exists
func (v *Validator) ValidateSchema(ctx context.Context) error {
	conn := NewConnection(v.config)

	if err := conn.Connect(ctx); err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Check if migration table exists
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)`

	var exists bool
	err := conn.QueryRow(ctx, sql, v.config.Migration.Table).Scan(&exists)
	if err != nil {
		return errors.NewValidationError("Failed to check migration table", err.Error())
	}

	if !exists {
		return errors.NewValidationError("Migration table does not exist",
			"Run 'vorm init' to initialize the migration system")
	}

	return nil
}

// ValidateSchemaStructure validates the migration table structure
func (v *Validator) ValidateSchemaStructure(ctx context.Context) error {
	conn := NewConnection(v.config)

	if err := conn.Connect(ctx); err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Check migration table columns
	sql := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema = 'public' 
		AND table_name = $1 
		ORDER BY ordinal_position`

	rows, err := conn.Query(ctx, sql, v.config.Migration.Table)
	if err != nil {
		return errors.NewValidationError("Failed to check migration table structure", err.Error())
	}
	defer rows.Close()

	expectedColumns := map[string]string{
		"id":             "bigint",
		"migration":      "character varying",
		"batch":          "integer",
		"executed_at":    "timestamp with time zone",
		"execution_time": "integer",
		"checksum":       "character varying",
	}

	foundColumns := make(map[string]string)
	for rows.Next() {
		var columnName, dataType string
		if err := rows.Scan(&columnName, &dataType); err != nil {
			return errors.NewValidationError("Failed to scan column info", err.Error())
		}
		foundColumns[columnName] = dataType
	}

	// Check if all expected columns exist
	for column, expectedType := range expectedColumns {
		if foundType, exists := foundColumns[column]; !exists {
			return errors.NewValidationError("Missing migration table column",
				"Column '"+column+"' not found. Run 'vorm init' to recreate the migration table")
		} else if foundType != expectedType {
			return errors.NewValidationError("Invalid migration table column type",
				"Column '"+column+"' has type '"+foundType+"', expected '"+expectedType+"'")
		}
	}

	return nil
}
