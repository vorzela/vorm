package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/pkg/errors"
)

// Connection represents a simple database connection for migration operations
// Since this is a migration tool that runs commands occasionally, we use
// simple connections rather than complex pool management
type Connection struct {
	config *config.Config
	conn   *pgx.Conn
}

// NewConnection creates a new database connection manager
func NewConnection(cfg *config.Config) *Connection {
	return &Connection{
		config: cfg,
	}
}

// Connect establishes a connection to the database
// Uses a simple connection since migration tools don't need connection pooling
func (c *Connection) Connect(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.config.GetDSN())
	if err != nil {
		return errors.NewConnectionError("Failed to connect to database", err.Error())
	}

	// Test connection with a simple ping
	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return errors.NewConnectionError("Failed to ping database", err.Error())
	}

	c.conn = conn
	return nil
}

// ConnectAdmin establishes an admin connection (without specific database)
// Used for database creation/deletion operations
func (c *Connection) ConnectAdmin(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, c.config.GetAdminDSN())
	if err != nil {
		return nil, errors.NewConnectionError("Failed to connect to PostgreSQL server", err.Error())
	}

	// Test connection
	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return nil, errors.NewConnectionError("Failed to ping PostgreSQL server", err.Error())
	}

	return conn, nil
}

// Close closes the database connection
func (c *Connection) Close(ctx context.Context) {
	if c.conn != nil {
		c.conn.Close(ctx)
	}
}

// Conn returns the database connection
func (c *Connection) Conn() *pgx.Conn {
	return c.conn
}

// Exec executes a SQL statement
func (c *Connection) Exec(ctx context.Context, sql string, args ...interface{}) error {
	if c.conn == nil {
		return errors.NewConnectionError("No database connection", "connection is nil")
	}

	_, err := c.conn.Exec(ctx, sql, args...)
	if err != nil {
		return errors.NewMigrationError("Failed to execute SQL", err.Error(), "")
	}

	return nil
}

// Query executes a SQL query and returns rows
func (c *Connection) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if c.conn == nil {
		return nil, errors.NewConnectionError("No database connection", "connection is nil")
	}

	rows, err := c.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, errors.NewMigrationError("Failed to execute query", err.Error(), "")
	}

	return rows, nil
}

// QueryRow executes a SQL query that returns at most one row
func (c *Connection) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if c.conn == nil {
		return nil
	}

	return c.conn.QueryRow(ctx, sql, args...)
}

// Begin starts a transaction
// Migrations should run in transactions for atomicity
func (c *Connection) Begin(ctx context.Context) (pgx.Tx, error) {
	if c.conn == nil {
		return nil, errors.NewConnectionError("No database connection", "connection is nil")
	}

	tx, err := c.conn.Begin(ctx)
	if err != nil {
		return nil, errors.NewMigrationError("Failed to begin transaction", err.Error(), "")
	}

	return tx, nil
}

// TestConnection tests the database connection
func (c *Connection) TestConnection(ctx context.Context) error {
	if c.conn == nil {
		return errors.NewConnectionError("No database connection", "connection is nil")
	}

	return c.conn.Ping(ctx)
}
