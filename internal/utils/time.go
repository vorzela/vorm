package utils

import (
	"time"
)

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}

	if d < time.Second {
		return formatMilliseconds(d)
	}

	if d < time.Minute {
		return formatSeconds(d)
	}

	return d.String()
}

// formatMilliseconds formats duration in milliseconds
func formatMilliseconds(d time.Duration) string {
	ms := d.Milliseconds()
	if ms == 1 {
		return "1ms"
	}
	return d.String()
}

// formatSeconds formats duration in seconds
func formatSeconds(d time.Duration) string {
	seconds := d.Seconds()
	if seconds < 10 {
		return d.Truncate(time.Millisecond * 10).String()
	}
	return d.Truncate(time.Millisecond * 100).String()
}

// GetCurrentTime returns the current time in the configured timezone
func GetCurrentTime(timezone string) time.Time {
	if timezone == "" || timezone == "UTC" {
		return time.Now().UTC()
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fallback to UTC if timezone is invalid
		return time.Now().UTC()
	}

	return time.Now().In(loc)
}

// FormatTimestamp formats a timestamp for logging
func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// FormatMigrationTimestamp formats a timestamp for migration files
func FormatMigrationTimestamp(t time.Time) string {
	return t.Format("2006_01_02_150405")
}

// ParseMigrationTimestamp parses a migration timestamp
func ParseMigrationTimestamp(timestamp string) (time.Time, error) {
	return time.Parse("2006_01_02_150405", timestamp)
}

// StartTimer returns a function that measures elapsed time
func StartTimer() func() time.Duration {
	start := time.Now()
	return func() time.Duration {
		return time.Since(start)
	}
}

// GetExecutionTime returns the execution time in milliseconds
func GetExecutionTime(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}
