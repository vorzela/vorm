package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// ToSnakeCase converts a string to snake_case
func ToSnakeCase(s string) string {
	// Handle empty string
	if s == "" {
		return ""
	}

	// Regular expression to find camelCase boundaries
	re := regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := re.ReplaceAllString(s, "${1}_${2}")

	// Convert to lowercase and replace spaces/hyphens with underscores
	snake = strings.ToLower(snake)
	snake = strings.ReplaceAll(snake, " ", "_")
	snake = strings.ReplaceAll(snake, "-", "_")

	// Remove multiple consecutive underscores
	re = regexp.MustCompile("_+")
	snake = re.ReplaceAllString(snake, "_")

	// Trim leading/trailing underscores
	return strings.Trim(snake, "_")
}

// ToPascalCase converts a string to PascalCase
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	// Split on non-alphanumeric characters
	words := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(s, -1)

	var result strings.Builder
	for _, word := range words {
		if word == "" {
			continue
		}
		// Capitalize first letter of each word
		result.WriteString(strings.Title(strings.ToLower(word)))
	}

	return result.String()
}

// ToCamelCase converts a string to camelCase
func ToCamelCase(s string) string {
	pascal := ToPascalCase(s)
	if pascal == "" {
		return ""
	}

	// Make first character lowercase
	runes := []rune(pascal)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// GenerateTimestamp generates a timestamp string for migration files
func GenerateTimestamp() string {
	return time.Now().Format("2006_01_02_150405")
}

// ValidateMigrationName validates that a migration name is valid
func ValidateMigrationName(name string) bool {
	if name == "" {
		return false
	}

	// Migration name should only contain letters, numbers, and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, name)
	return matched
}

// SanitizeMigrationName sanitizes a migration name
func SanitizeMigrationName(name string) string {
	// Convert to snake_case
	name = ToSnakeCase(name)

	// Remove any characters that aren't letters, numbers, or underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "")

	// Ensure it doesn't start with a number
	if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
		name = "migration_" + name
	}

	return name
}

// ParseMigrationFilename extracts components from a migration filename
func ParseMigrationFilename(filename string) (timestamp, name string, valid bool) {
	// Remove .sql extension if present
	filename = strings.TrimSuffix(filename, ".sql")

	// Migration filename format: YYYY_MM_DD_HHMMSS_migration_name
	re := regexp.MustCompile(`^(\d{4}_\d{2}_\d{2}_\d{6})_(.+)$`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) != 3 {
		return "", "", false
	}

	return matches[1], matches[2], true
}

// GenerateMigrationFilename generates a migration filename
func GenerateMigrationFilename(name string) string {
	timestamp := GenerateTimestamp()
	sanitizedName := SanitizeMigrationName(name)
	return fmt.Sprintf("%s_%s.sql", timestamp, sanitizedName)
}

// Pluralize attempts to pluralize a word (basic English rules)
func Pluralize(word string) string {
	if word == "" {
		return ""
	}

	word = strings.ToLower(word)

	// Special cases
	specialCases := map[string]string{
		"person": "people",
		"child":  "children",
		"foot":   "feet",
		"tooth":  "teeth",
		"mouse":  "mice",
		"goose":  "geese",
	}

	if plural, exists := specialCases[word]; exists {
		return plural
	}

	// Basic pluralization rules
	if strings.HasSuffix(word, "y") && len(word) > 1 {
		// Check if the character before 'y' is a consonant
		beforeY := word[len(word)-2]
		if !isVowel(beforeY) {
			return word[:len(word)-1] + "ies"
		}
	}

	if strings.HasSuffix(word, "s") ||
		strings.HasSuffix(word, "ss") ||
		strings.HasSuffix(word, "sh") ||
		strings.HasSuffix(word, "ch") ||
		strings.HasSuffix(word, "x") ||
		strings.HasSuffix(word, "z") {
		return word + "es"
	}

	if strings.HasSuffix(word, "f") {
		return word[:len(word)-1] + "ves"
	}

	if strings.HasSuffix(word, "fe") {
		return word[:len(word)-2] + "ves"
	}

	// Default: just add 's'
	return word + "s"
}

// isVowel checks if a character is a vowel
func isVowel(c byte) bool {
	vowels := "aeiou"
	return strings.ContainsRune(vowels, rune(c))
}

// Truncate truncates a string to a maximum length
func Truncate(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	if maxLength <= 3 {
		return s[:maxLength]
	}

	return s[:maxLength-3] + "..."
}

// PadLeft pads a string to the left with the specified character
func PadLeft(s string, length int, padChar rune) string {
	if len(s) >= length {
		return s
	}

	padding := strings.Repeat(string(padChar), length-len(s))
	return padding + s
}

// PadRight pads a string to the right with the specified character
func PadRight(s string, length int, padChar rune) string {
	if len(s) >= length {
		return s
	}

	padding := strings.Repeat(string(padChar), length-len(s))
	return s + padding
}

// IsEmpty checks if a string is empty or contains only whitespace
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Contains checks if a slice contains a string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
