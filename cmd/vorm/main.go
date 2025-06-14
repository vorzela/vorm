package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/console"
	"github.com/vorzela/vorm/internal/database"
	"github.com/vorzela/vorm/internal/logger"
	"github.com/vorzela/vorm/internal/migration"
)

var (
	version = "1.0.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vorm",
		Short: "PostgreSQL Migration Management Tool",
		Long: `VORM is a powerful database migration tool specifically optimized for 
PostgreSQL databases, providing version control for your schema changes.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s, %s)", version, commit, date, runtime.Version()),
	}

	// Add all commands
	addCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		console.ColorError.Printf("✗ %v\n", err)
		os.Exit(1)
	}
}

func addCommands(rootCmd *cobra.Command) {
	// Initialize command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize migration system",
		Run:   initCommand,
	})

	// Setup command (database and migrations table)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Setup database and migrations table",
		Run:   setupCommand,
	})

	// Make migration command
	makeCmd := &cobra.Command{
		Use:   "make:migration",
		Short: "Create new migration",
		Args:  cobra.ExactArgs(1),
		Run:   makeMigrationCommand,
	}
	rootCmd.AddCommand(makeCmd)

	// Migration operations
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run pending migrations",
		Run:   migrateCommand,
	}
	migrateCmd.Flags().IntP("step", "s", 0, "Run specific number of migrations")
	rootCmd.AddCommand(migrateCmd)

	rollbackCmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback migrations",
		Run:   rollbackCommand,
	}
	rollbackCmd.Flags().IntP("step", "s", 0, "Rollback specific number of migrations")
	rollbackCmd.Flags().String("to", "", "Rollback to specific migration")
	rootCmd.AddCommand(rollbackCmd)

	// Status and information commands
	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		Run:   statusCommand,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all migrations",
		Run:   listCommand,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "history",
		Short: "Show migration history",
		Run:   historyCommand,
	})

	// Database operations
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database operations",
	}

	dbCmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create database",
		Run:   dbCreateCommand,
	})

	dbCmd.AddCommand(&cobra.Command{
		Use:   "drop",
		Short: "Drop database (with confirmation)",
		Run:   dbDropCommand,
	})

	dbCmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Drop and recreate database",
		Run:   dbResetCommand,
	})

	rootCmd.AddCommand(dbCmd)

	// Config commands
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration operations",
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run:   configShowCommand,
	})

	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Run:   configValidateCommand,
	})

	rootCmd.AddCommand(configCmd)

	// Destructive operations
	rootCmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Rollback all migrations",
		Run:   resetCommand,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "fresh",
		Short: "Drop all tables and re-run migrations",
		Run:   freshCommand,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "refresh",
		Short: "Rollback and re-run migrations",
		Run:   refreshCommand,
	})
}

// Command handlers (placeholder implementations)
func initCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Initializing VORM migration system...")

	// Check if config files already exist
	configPath := "config/database.yaml"
	envPath := ".env"
	migrationsDir := "migrations"

	// Create directories if they don't exist
	if err := os.MkdirAll("config", 0755); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create config directory: %v", err))
		os.Exit(1)
	}

	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migrations directory: %v", err))
		os.Exit(1)
	}

	// Create database.yaml if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		console.PrintInfo("Creating database configuration file...")
		configContent := `# VORM Database Configuration
database:
  driver: postgres
  host: localhost
  port: 5432
  name: your_database_name
  username: your_username
  password: your_password
  sslmode: disable
  timezone: UTC
  max_connections: 10
  max_idle_connections: 5
  connection_max_lifetime: 1h

migration:
  table: migrations
  directory: migrations
  lock_timeout: 15m
  transaction_timeout: 30m

logging:
  level: info
  file: logs/vorm.log
  max_size: 100 # MB
  max_backups: 5
  max_age: 30 # days
  compress: true

environment: development

production:
  require_confirmation: true
  disable_destructive_operations: true
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			console.PrintError(fmt.Sprintf("Failed to create config file: %v", err))
			os.Exit(1)
		}
		console.PrintSuccess("Created config/database.yaml")
	} else {
		console.PrintInfo("Configuration file already exists")
	}

	// Create .env if it doesn't exist
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		console.PrintInfo("Creating environment file...")
		envContent := `# VORM Environment Variables
# Database connection (alternative to config file)

# Override specific config values
VORM_DB_HOST=localhost
VORM_DB_PORT=5432
VORM_DB_NAME=your_database
VORM_DB_USERNAME=your_username
VORM_DB_PASSWORD=your_password
VORM_ENVIRONMENT=development
VORM_LOG_LEVEL=info

# Optional: Override other settings
# VORM_DATABASE_URL=postgres://username:password@localhost:5432/database_name?sslmode=disable
`
		if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
			console.PrintError(fmt.Sprintf("Failed to create .env file: %v", err))
			os.Exit(1)
		}
		console.PrintSuccess("Created .env file")
	} else {
		console.PrintInfo("Environment file already exists")
	}

	// Create logs directory
	if err := os.MkdirAll("logs", 0755); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logs directory: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("VORM migration system initialized successfully!")
	console.PrintInfo("")
	console.PrintInfo("Next steps:")
	console.PrintInfo("1. Edit config/database.yaml with your database settings")
	console.PrintInfo("2. Set environment variables in .env if needed")
	console.PrintInfo("3. Run 'vorm setup' to create database and migrations table")
	console.PrintInfo("4. Create your first migration with 'vorm make:migration create_users_table'")
}

func setupCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Setting up database and migrations table...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		console.PrintInfo("Run 'vorm init' first to create configuration files")
		os.Exit(1)
	}

	// Validate configuration
	validator := config.NewValidator(cfg)
	if err := validator.Validate(); err != nil {
		console.PrintError(fmt.Sprintf("Configuration validation failed: %v", err))
		os.Exit(1)
	}

	// Create database creator
	creator := database.NewCreator(cfg)

	// Create database if it doesn't exist
	ctx := context.Background()
	console.PrintInfo("Checking if database exists...")
	if err := creator.CreateDatabase(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create database: %v", err))
		os.Exit(1)
	}

	// Connect to database
	conn := database.NewConnection(cfg)
	if err := conn.Connect(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Failed to connect to database: %v", err))
		os.Exit(1)
	}
	defer conn.Close(ctx)

	// Create migrations table
	console.PrintInfo("Creating migrations table...")
	if err := creator.CreateMigrationsTable(ctx, conn); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migrations table: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Database and migrations table setup completed successfully!")
	console.PrintInfo(fmt.Sprintf("Migrations table: %s", cfg.Migration.Table))
	console.PrintInfo(fmt.Sprintf("Migrations directory: %s", cfg.GetMigrationsPath()))
}

func makeMigrationCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo(fmt.Sprintf("Creating migration: %s", args[0]))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create migration generator
	generator := migration.NewGenerator(cfg)

	// Generate migration
	migrationFile, err := generator.GenerateMigration(args[0])
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess(fmt.Sprintf("Created migration: %s", migrationFile.Filename))
	console.PrintInfo(fmt.Sprintf("Migration file: %s", migrationFile.Filepath))
}

func migrateCommand(cmd *cobra.Command, args []string) {
	step, _ := cmd.Flags().GetInt("step")
	console.PrintInfo("Loading configuration...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Run migrations
	ctx := context.Background()
	if step > 0 {
		console.PrintInfo(fmt.Sprintf("Running %d migrations...", step))
		if err := manager.RunMigrations(ctx, step); err != nil {
			console.PrintError(fmt.Sprintf("Migration failed: %v", err))
			os.Exit(1)
		}
	} else {
		console.PrintInfo("Running pending migrations...")
		if err := manager.RunMigrations(ctx, 0); err != nil {
			console.PrintError(fmt.Sprintf("Migration failed: %v", err))
			os.Exit(1)
		}
	}

	console.PrintSuccess("Migrations completed successfully")
}

func rollbackCommand(cmd *cobra.Command, args []string) {
	step, _ := cmd.Flags().GetInt("step")

	console.PrintWarning("WARNING: Rollback operation cannot be undone!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Production safety check
	if cfg.IsProduction() {
		if !console.RequireTypedConfirmation("Rollback migrations in PRODUCTION", "ROLLBACK") {
			console.PrintInfo("Rollback cancelled")
			return
		}
	} else {
		if !console.ConfirmDestructiveOperation("Rollback migrations") {
			console.PrintInfo("Rollback cancelled")
			return
		}
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}
	ctx := context.Background()

	if step > 0 {
		// Rollback specific number of steps
		console.PrintInfo(fmt.Sprintf("Rolling back %d migrations...", step))
		if err := manager.RollbackSteps(ctx, step); err != nil {
			console.PrintError(fmt.Sprintf("Rollback failed: %v", err))
			os.Exit(1)
		}
	} else {
		// Rollback last batch
		console.PrintInfo("Rolling back last batch...")
		if err := manager.RollbackMigrations(ctx, 0); err != nil {
			console.PrintError(fmt.Sprintf("Rollback failed: %v", err))
			os.Exit(1)
		}
	}

	console.PrintSuccess("Rollback completed successfully")
}

func statusCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Checking migration status...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Get migration status
	ctx := context.Background()
	statuses, err := manager.GetMigrationStatus(ctx)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to get migration status: %v", err))
		os.Exit(1)
	}

	// Display status
	console.PrintHighlight("=== Migration Status ===")
	if len(statuses) == 0 {
		console.PrintInfo("No migrations found")
		return
	}

	fmt.Printf("%-50s %-10s %-20s %-10s\n", "Migration", "Status", "Executed At", "Batch")
	fmt.Println(strings.Repeat("-", 90))

	for _, status := range statuses {
		statusStr := "Pending"
		executedAt := "-"
		batch := "-"

		if status.Executed {
			statusStr = "Executed"
			executedAt = status.ExecutedAt.Format("2006-01-02 15:04:05")
			batch = fmt.Sprintf("%d", status.Batch)
		}

		fmt.Printf("%-50s %-10s %-20s %-10s\n",
			status.Migration.Name, statusStr, executedAt, batch)
	}
}

func listCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Listing all migrations...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Get all migrations
	migrations, err := manager.ListMigrations()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to list migrations: %v", err))
		os.Exit(1)
	}

	// Display migrations
	console.PrintHighlight("=== All Migrations ===")
	if len(migrations) == 0 {
		console.PrintInfo("No migrations found")
		return
	}

	fmt.Printf("%-50s %-15s %-15s\n", "Migration", "Filename", "Checksum")
	fmt.Println(strings.Repeat("-", 80))

	for _, migration := range migrations {
		checksum := migration.Checksum
		if len(checksum) > 12 {
			checksum = checksum[:12] + "..."
		}
		fmt.Printf("%-50s %-15s %-15s\n", migration.Name, migration.Filename, checksum)
	}
}

func historyCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Showing migration history...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Get migration history
	ctx := context.Background()
	history, err := manager.GetMigrationHistory(ctx)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to get migration history: %v", err))
		os.Exit(1)
	}

	// Display history
	console.PrintHighlight("=== Migration History ===")
	if len(history) == 0 {
		console.PrintInfo("No executed migrations found")
		return
	}

	fmt.Printf("%-50s %-20s %-10s %-15s\n", "Migration", "Executed At", "Batch", "Execution Time")
	fmt.Println(strings.Repeat("-", 95))

	for _, migration := range history {
		executionTimeStr := fmt.Sprintf("%dms", migration.ExecutionTime)
		fmt.Printf("%-50s %-20s %-10d %-15s\n",
			migration.Name,
			migration.ExecutedAt.Format("2006-01-02 15:04:05"),
			migration.Batch,
			executionTimeStr)
	}
}

func dbCreateCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Creating database...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Create database creator
	creator := database.NewCreator(cfg)

	// Create database
	ctx := context.Background()
	if err := creator.CreateDatabase(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Failed to create database: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess(fmt.Sprintf("Database '%s' created successfully", cfg.Database.Database))
}

func dbDropCommand(cmd *cobra.Command, args []string) {
	console.PrintWarning("WARNING: This will permanently delete the database!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Check if in production environment
	if cfg.IsProduction() {
		console.PrintError("Database drop operation is disabled in production environment")
		console.PrintInfo("This is a safety measure to prevent accidental data loss")
		os.Exit(1)
	}

	// Require typed confirmation
	if !console.RequireTypedConfirmation("Drop database", "DROP") {
		console.PrintInfo("Database drop cancelled")
		return
	}

	// Create database creator
	creator := database.NewCreator(cfg)

	// Drop database
	ctx := context.Background()
	console.PrintInfo("Dropping database...")
	if err := creator.DropDatabase(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Failed to drop database: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Database dropped successfully")
}

func dbResetCommand(cmd *cobra.Command, args []string) {
	console.PrintWarning("WARNING: This will drop and recreate the database!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Check if in production environment
	if cfg.IsProduction() {
		console.PrintError("Database reset operation is disabled in production environment")
		console.PrintInfo("This is a safety measure to prevent accidental data loss")
		os.Exit(1)
	}

	// Require typed confirmation
	if !console.RequireTypedConfirmation("Reset database (drop and recreate)", "RESET") {
		console.PrintInfo("Database reset cancelled")
		return
	}

	// Create database creator
	creator := database.NewCreator(cfg)

	// Reset database
	ctx := context.Background()
	console.PrintInfo("Resetting database...")
	if err := creator.ResetDatabase(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Failed to reset database: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Database reset completed successfully")
}

func configShowCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Loading configuration...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Display configuration
	console.PrintHighlight("=== VORM Configuration ===")
	fmt.Printf("Environment: %s\n", cfg.Environment)
	fmt.Printf("\nDatabase:\n")
	fmt.Printf("  Connection: %s\n", cfg.Database.Connection)
	fmt.Printf("  Host: %s\n", cfg.Database.Host)
	fmt.Printf("  Port: %d\n", cfg.Database.Port)
	fmt.Printf("  Database: %s\n", cfg.Database.Database)
	fmt.Printf("  Username: %s\n", cfg.Database.Username)
	fmt.Printf("  SSL Mode: %s\n", cfg.Database.SSLMode)

	fmt.Printf("\nMigration:\n")
	fmt.Printf("  Table: %s\n", cfg.Migration.Table)
	fmt.Printf("  Directory: %s\n", cfg.Migration.Directory)
	fmt.Printf("  Timezone: %s\n", cfg.Migration.Timezone)

	fmt.Printf("\nLogging:\n")
	fmt.Printf("  Enabled: %t\n", cfg.Logging.Enabled)
	if cfg.Logging.Enabled {
		fmt.Printf("  Directory: %s\n", cfg.Logging.Directory)
		fmt.Printf("  Filename: %s\n", cfg.Logging.Filename)
		fmt.Printf("  Level: %s\n", cfg.Logging.Level)
	}

	console.PrintSuccess("Configuration loaded successfully")
}

func configValidateCommand(cmd *cobra.Command, args []string) {
	console.PrintInfo("Validating configuration...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Configuration validation failed: %v", err))
		os.Exit(1)
	}

	// Validate configuration
	validator := config.NewValidator(cfg)
	if err := validator.Validate(); err != nil {
		console.PrintError(fmt.Sprintf("Configuration validation failed: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Configuration is valid")
}

func resetCommand(cmd *cobra.Command, args []string) {
	console.PrintWarning("WARNING: This will rollback ALL migrations!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Production safety check
	if cfg.IsProduction() {
		console.PrintError("Reset operation is disabled in production environment")
		console.PrintInfo("This is a safety measure to prevent accidental data loss")
		os.Exit(1)
	}

	// Require typed confirmation
	if !console.RequireTypedConfirmation("Reset all migrations (rollback ALL)", "RESET") {
		console.PrintInfo("Reset cancelled")
		return
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Reset all migrations
	ctx := context.Background()
	console.PrintInfo("Resetting all migrations...")
	if err := manager.ResetAllMigrations(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Reset failed: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("All migrations have been reset successfully")
}

func freshCommand(cmd *cobra.Command, args []string) {
	console.PrintWarning("WARNING: This will drop ALL tables and re-run migrations!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Production safety check
	if cfg.IsProduction() {
		console.PrintError("Fresh operation is disabled in production environment")
		console.PrintInfo("This is a safety measure to prevent accidental data loss")
		os.Exit(1)
	}

	// Require typed confirmation
	if !console.RequireTypedConfirmation("Drop all tables and re-run migrations", "FRESH") {
		console.PrintInfo("Fresh operation cancelled")
		return
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Run fresh migrations
	ctx := context.Background()
	console.PrintInfo("Running fresh migrations...")
	if err := manager.FreshMigrations(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Fresh operation failed: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Fresh migrations completed successfully")
}

func refreshCommand(cmd *cobra.Command, args []string) {
	console.PrintWarning("WARNING: This will rollback and re-run all migrations!")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	// Production safety check
	if cfg.IsProduction() {
		console.PrintError("Refresh operation is disabled in production environment")
		console.PrintInfo("This is a safety measure to prevent accidental data loss")
		os.Exit(1)
	}

	// Require typed confirmation
	if !console.RequireTypedConfirmation("Rollback and re-run all migrations", "REFRESH") {
		console.PrintInfo("Refresh operation cancelled")
		return
	}

	// Create logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		os.Exit(1)
	}

	// Create migration manager
	manager, err := migration.NewManager(cfg, log)
	if err != nil {
		console.PrintError(fmt.Sprintf("Failed to create migration manager: %v", err))
		os.Exit(1)
	}

	// Run refresh (rollback all, then migrate up)
	ctx := context.Background()

	console.PrintInfo("Rolling back all migrations...")
	if err := manager.ResetAllMigrations(ctx); err != nil {
		console.PrintError(fmt.Sprintf("Rollback failed: %v", err))
		os.Exit(1)
	}

	console.PrintInfo("Re-running all migrations...")
	if err := manager.RunMigrations(ctx, 0); err != nil {
		console.PrintError(fmt.Sprintf("Migration failed: %v", err))
		os.Exit(1)
	}

	console.PrintSuccess("Refresh completed successfully")
}
