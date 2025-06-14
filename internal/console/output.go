package console

// PrintSuccess prints a success message with checkmark
func PrintSuccess(message string) {
	ColorSuccess.Printf("✓ %s\n", message)
}

// PrintError prints an error message with X mark
func PrintError(message string) {
	ColorError.Printf("✗ %s\n", message)
}

// PrintWarning prints a warning message with warning symbol
func PrintWarning(message string) {
	ColorWarning.Printf("⚠ %s\n", message)
}

// PrintInfo prints an info message with info symbol
func PrintInfo(message string) {
	ColorInfo.Printf("ℹ %s\n", message)
}

// PrintDebug prints a debug message
func PrintDebug(message string) {
	ColorDebug.Printf("🐛 %s\n", message)
}

// PrintHighlight prints highlighted text
func PrintHighlight(message string) {
	ColorHighlight.Printf("%s\n", message)
}
