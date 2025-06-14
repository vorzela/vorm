package errors

import (
	"fmt"
	"time"
)

// MigrationError represents all types of migration-related errors
type MigrationError struct {
	Type      string    // "connection", "migration", "validation", "file", "permission"
	Message   string    // Human-readable message
	Details   string    // Technical details
	Migration string    // Migration name (if applicable)
	Timestamp time.Time // When error occurred
}

func (e *MigrationError) Error() string {
	if e.Migration != "" {
		return fmt.Sprintf("[%s] %s (migration: %s): %s", e.Type, e.Message, e.Migration, e.Details)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Message, e.Details)
}

// NewConnectionError creates a database connection error
func NewConnectionError(message, details string) *MigrationError {
	return &MigrationError{
		Type:      "connection",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewMigrationError creates a migration execution error
func NewMigrationError(message, details, migration string) *MigrationError {
	return &MigrationError{
		Type:      "migration",
		Message:   message,
		Details:   details,
		Migration: migration,
		Timestamp: time.Now(),
	}
}

// NewValidationError creates a configuration/file validation error
func NewValidationError(message, details string) *MigrationError {
	return &MigrationError{
		Type:      "validation",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewFileError creates a file system operation error
func NewFileError(message, details string) *MigrationError {
	return &MigrationError{
		Type:      "file",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewPermissionError creates a database permission error
func NewPermissionError(message, details string) *MigrationError {
	return &MigrationError{
		Type:      "permission",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}
