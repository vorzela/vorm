package utils

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/vorzela/vorm/pkg/errors"
)

// PlatformPath converts a path to the platform-specific format
func PlatformPath(path string) string {
	return filepath.FromSlash(path)
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	// Check if directory exists
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return errors.NewFileError("Path exists but is not a directory", path)
		}
		return nil
	}

	// Create directory with proper permissions
	if err := os.MkdirAll(path, 0755); err != nil {
		return errors.NewFileError("Failed to create directory", err.Error())
	}

	return nil
}

// FileExists checks if a file exists
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// IsWritableDirectory checks if a directory is writable
func IsWritableDirectory(path string) error {
	// Check if directory exists
	if !FileExists(path) {
		return errors.NewFileError("Directory does not exist", path)
	}

	// Try to create a temporary file
	tempFile := filepath.Join(path, ".vorm_write_test")
	file, err := os.Create(tempFile)
	if err != nil {
		return errors.NewFileError("Directory is not writable", err.Error())
	}

	// Clean up
	file.Close()
	os.Remove(tempFile)

	return nil
}

// GetWorkingDirectory returns the current working directory
func GetWorkingDirectory() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", errors.NewFileError("Failed to get working directory", err.Error())
	}
	return wd, nil
}

// JoinPath joins path elements using the platform-specific separator
func JoinPath(elements ...string) string {
	return filepath.Join(elements...)
}

// GetAbsolutePath returns the absolute path
func GetAbsolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.NewFileError("Failed to get absolute path", err.Error())
	}
	return abs, nil
}

// IsAbsolutePath checks if a path is absolute
func IsAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

// GetHomeDirectory returns the user's home directory
func GetHomeDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.NewFileError("Failed to get home directory", err.Error())
	}
	return home, nil
}

// CreateFile creates a file with the given content
func CreateFile(filename, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := EnsureDirectoryExists(dir); err != nil {
		return err
	}

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return errors.NewFileError("Failed to create file", err.Error())
	}
	defer file.Close()

	// Write content
	if _, err := file.WriteString(content); err != nil {
		return errors.NewFileError("Failed to write file content", err.Error())
	}

	return nil
}

// ReadFile reads the entire content of a file
func ReadFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", errors.NewFileError("Failed to read file", err.Error())
	}
	return string(content), nil
}

// GetPlatformInfo returns information about the current platform
func GetPlatformInfo() map[string]string {
	return map[string]string{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
		"num_cpu":    string(rune(runtime.NumCPU())),
		"path_sep":   string(filepath.Separator),
		"list_sep":   string(filepath.ListSeparator),
	}
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsUnix returns true if running on Unix-like system
func IsUnix() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd"
}
