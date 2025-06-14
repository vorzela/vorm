package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vorzela/vorm/internal/config"
	"github.com/vorzela/vorm/internal/console"
	"github.com/vorzela/vorm/internal/utils"
)

// LogLevel represents different log levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	SUCCESS
	WARNING
	ERROR
	FATAL
)

// String returns the string representation of log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case SUCCESS:
		return "SUCCESS"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger handles logging operations as specified in AINOTES.md
type Logger struct {
	config   *config.Config
	logFile  *os.File
	enabled  bool
	minLevel LogLevel
}

// NewLogger creates a new logger instance
func NewLogger(cfg *config.Config) (*Logger, error) {
	logger := &Logger{
		config:   cfg,
		enabled:  cfg.Logging.Enabled,
		minLevel: parseLogLevel(cfg.Logging.Level),
	}

	if cfg.Logging.Enabled {
		if err := logger.initLogFile(); err != nil {
			return nil, err
		}
	}

	return logger, nil
}

// initLogFile initializes the log file
func (l *Logger) initLogFile() error {
	// Ensure log directory exists
	logDir := l.config.GetLogsPath()
	if err := utils.EnsureDirectoryExists(logDir); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open log file
	logPath := filepath.Join(logDir, l.config.Logging.Filename)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	l.logFile = file
	return nil
}

// Close closes the logger
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Log writes a log entry with the specified level
func (l *Logger) Log(level LogLevel, category, message string) {
	if level < l.minLevel {
		return
	}

	// Console output with colors
	l.logToConsole(level, category, message)

	// File logging if enabled
	if l.enabled && l.logFile != nil {
		l.logToFile(level, category, message)
	}
}

// logToConsole outputs colored log messages to console
func (l *Logger) logToConsole(level LogLevel, category, message string) {
	timestamp := time.Now().Format("15:04:05")
	logMsg := fmt.Sprintf("[%s] %s", timestamp, message)

	switch level {
	case SUCCESS:
		console.ColorSuccess.Printf("✓ %s\n", logMsg)
	case ERROR, FATAL:
		console.ColorError.Printf("✗ %s\n", logMsg)
	case WARNING:
		console.ColorWarning.Printf("⚠ %s\n", logMsg)
	case INFO:
		console.ColorInfo.Printf("ℹ %s\n", logMsg)
	case DEBUG:
		console.ColorDebug.Printf("🐛 %s\n", logMsg)
	}
}

// logToFile writes log entry to file in the format specified in AINOTES.md
func (l *Logger) logToFile(level LogLevel, category, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level.String(), category, message)

	if _, err := l.logFile.WriteString(logEntry); err != nil {
		// If we can't write to file, output to console instead
		console.ColorError.Printf("✗ Failed to write to log file: %v\n", err)
	}
}

// parseLogLevel converts string to LogLevel
func parseLogLevel(level string) LogLevel {
	switch level {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warning":
		return WARNING
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}

// Convenience methods for different log levels
func (l *Logger) Debug(category, message string) {
	l.Log(DEBUG, category, message)
}

func (l *Logger) Info(category, message string) {
	l.Log(INFO, category, message)
}

func (l *Logger) Success(category, message string) {
	l.Log(SUCCESS, category, message)
}

func (l *Logger) Warning(category, message string) {
	l.Log(WARNING, category, message)
}

func (l *Logger) Error(category, message string) {
	l.Log(ERROR, category, message)
}

func (l *Logger) Fatal(category, message string) {
	l.Log(FATAL, category, message)
}

// Migration-specific logging methods
func (l *Logger) LogMigrationStart(migration string) {
	l.Info("Migration", fmt.Sprintf("Starting migration: %s", migration))
}

func (l *Logger) LogMigrationSuccess(migration string, duration time.Duration) {
	l.Success("Migration", fmt.Sprintf("Completed: %s (%s)", migration, utils.FormatDuration(duration)))
}

func (l *Logger) LogMigrationError(migration string, err error) {
	l.Error("Migration", fmt.Sprintf("Failed: %s - %v", migration, err))
}

func (l *Logger) LogDatabaseConnection(database string) {
	l.Info("Database", fmt.Sprintf("Connected to database: %s", database))
}

func (l *Logger) LogRollbackStart(migration string) {
	l.Warning("Migration", fmt.Sprintf("Rolling back: %s", migration))
}

func (l *Logger) LogRollbackSuccess(migration string) {
	l.Success("Migration", fmt.Sprintf("Rolled back: %s", migration))
}
