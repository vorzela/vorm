package migration

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/utils"
	"github.com/vorzela/vorm/pkg/errors"
)

// Migration represents a single database migration
type Migration struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Filename      string    `json:"filename"`
	Filepath      string    `json:"filepath"`
	Batch         int       `json:"batch"`
	ExecutedAt    time.Time `json:"executed_at"`
	ExecutionTime int       `json:"execution_time"` // milliseconds
	Checksum      string    `json:"checksum"`
	UpSQL         string    `json:"up_sql"`
	DownSQL       string    `json:"down_sql"`
}

// Generator handles migration file generation
type Generator struct {
	config *config.Config
}

// NewGenerator creates a new migration generator
func NewGenerator(cfg *config.Config) *Generator {
	return &Generator{
		config: cfg,
	}
}

// GenerateMigration creates a new migration file
func (g *Generator) GenerateMigration(name string) (*Migration, error) {
	// Validate and sanitize migration name
	if !utils.ValidateMigrationName(name) {
		name = utils.SanitizeMigrationName(name)
	}

	// Generate migration filename
	filename := utils.GenerateMigrationFilename(name)
	filepath := filepath.Join(g.config.GetMigrationsPath(), filename)

	// Create migration content
	content := g.generateMigrationContent(name)

	// Write migration file
	if err := utils.CreateFile(filepath, content); err != nil {
		return nil, errors.NewFileError("Failed to create migration file", err.Error())
	}

	// Calculate checksum
	checksum := g.calculateChecksum(content)

	migration := &Migration{
		Name:     name,
		Filename: filename,
		Filepath: filepath,
		Checksum: checksum,
	}

	return migration, nil
}

// generateMigrationContent creates the migration file content as specified in AINOTES.md
func (g *Generator) generateMigrationContent(name string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Determine table name from migration name for common patterns
	var upSQL, downSQL string

	if strings.HasPrefix(name, "create_") {
		tableName := g.extractTableNameFromCreate(name)
		upSQL, downSQL = g.generateCreateTableSQL(tableName)
	} else if strings.HasPrefix(name, "add_") {
		upSQL, downSQL = g.generateAddColumnSQL(name)
	} else if strings.HasPrefix(name, "drop_") {
		upSQL, downSQL = g.generateDropColumnSQL(name)
	} else {
		upSQL, downSQL = g.generateGenericSQL()
	}

	return fmt.Sprintf(`-- Migration: %s
-- Created: %s
-- Batch: 1

-- +migrate Up
%s

-- +migrate Down
%s`, name, timestamp, upSQL, downSQL)
}

// extractTableNameFromCreate extracts table name from "create_tablename_table" pattern
func (g *Generator) extractTableNameFromCreate(name string) string {
	// Remove "create_" prefix and "_table" suffix
	tableName := strings.TrimPrefix(name, "create_")
	tableName = strings.TrimSuffix(tableName, "_table")

	// Ensure plural form as per AINOTES.md requirements
	if !strings.HasSuffix(tableName, "s") {
		tableName = utils.Pluralize(tableName)
	}

	return tableName
}

// generateCreateTableSQL generates CREATE TABLE SQL template
func (g *Generator) generateCreateTableSQL(tableName string) (upSQL, downSQL string) {
	upSQL = fmt.Sprintf(`CREATE TABLE %s (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for performance
CREATE INDEX idx_%s_created_at ON %s(created_at);`, tableName, tableName, tableName)

	downSQL = fmt.Sprintf(`DROP INDEX IF EXISTS idx_%s_created_at;
DROP TABLE IF EXISTS %s;`, tableName, tableName)

	return upSQL, downSQL
}

// generateAddColumnSQL generates ADD COLUMN SQL template
func (g *Generator) generateAddColumnSQL(name string) (upSQL, downSQL string) {
	upSQL = `-- Add your column here
-- ALTER TABLE table_name ADD COLUMN column_name VARCHAR(255);
-- CREATE INDEX idx_table_column ON table_name(column_name);`

	downSQL = `-- Remove the column here
-- DROP INDEX IF EXISTS idx_table_column;
-- ALTER TABLE table_name DROP COLUMN column_name;`

	return upSQL, downSQL
}

// generateDropColumnSQL generates DROP COLUMN SQL template
func (g *Generator) generateDropColumnSQL(name string) (upSQL, downSQL string) {
	upSQL = `-- Drop your column here
-- DROP INDEX IF EXISTS idx_table_column;
-- ALTER TABLE table_name DROP COLUMN column_name;`

	downSQL = `-- Add the column back (be careful with data loss)
-- ALTER TABLE table_name ADD COLUMN column_name VARCHAR(255);
-- CREATE INDEX idx_table_column ON table_name(column_name);`

	return upSQL, downSQL
}

// generateGenericSQL generates generic SQL template
func (g *Generator) generateGenericSQL() (upSQL, downSQL string) {
	upSQL = `-- Add your migration code here
-- Example:
-- CREATE TABLE new_table (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
-- );`

	downSQL = `-- Add your rollback code here
-- Example:
-- DROP TABLE IF EXISTS new_table;`

	return upSQL, downSQL
}

// calculateChecksum calculates SHA256 checksum of migration content
func (g *Generator) calculateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// LoadMigrations loads all migration files from the migrations directory
func (g *Generator) LoadMigrations() ([]*Migration, error) {
	migrationsPath := g.config.GetMigrationsPath()

	// Ensure migrations directory exists
	if err := utils.EnsureDirectoryExists(migrationsPath); err != nil {
		return nil, errors.NewFileError("Failed to access migrations directory", err.Error())
	}

	var migrations []*Migration

	err := filepath.WalkDir(migrationsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-SQL files
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		migration, err := g.loadMigrationFile(path)
		if err != nil {
			return err
		}

		migrations = append(migrations, migration)
		return nil
	})

	if err != nil {
		return nil, errors.NewFileError("Failed to load migrations", err.Error())
	}

	// Sort migrations by filename (timestamp)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Filename < migrations[j].Filename
	})

	return migrations, nil
}

// loadMigrationFile loads a single migration file
func (g *Generator) loadMigrationFile(filepath string) (*Migration, error) {
	// Read file content
	content, err := utils.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// Parse filename
	filename := filepath[strings.LastIndex(filepath, "/")+1:]
	_, name, valid := utils.ParseMigrationFilename(filename)
	if !valid {
		return nil, errors.NewValidationError("Invalid migration filename", filename)
	}

	// Parse migration content
	upSQL, downSQL := g.parseMigrationContent(content)

	// Calculate checksum
	checksum := g.calculateChecksum(content)

	migration := &Migration{
		Name:     name,
		Filename: filename,
		Filepath: filepath,
		Checksum: checksum,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
	}

	return migration, nil
}

// parseMigrationContent separates Up and Down SQL from migration file
func (g *Generator) parseMigrationContent(content string) (upSQL, downSQL string) {
	lines := strings.Split(content, "\n")

	var upLines, downLines []string
	var currentSection string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "-- +migrate Up") {
			currentSection = "up"
			continue
		} else if strings.Contains(trimmed, "-- +migrate Down") {
			currentSection = "down"
			continue
		}

		switch currentSection {
		case "up":
			upLines = append(upLines, line)
		case "down":
			downLines = append(downLines, line)
		}
	}

	return strings.Join(upLines, "\n"), strings.Join(downLines, "\n")
}
