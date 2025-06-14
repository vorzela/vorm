package console

import (
	"fmt"
	"os"
	"strings"
)

// ConfirmDestructiveOperation implements the exact warning system as specified
func ConfirmDestructiveOperation(operation string) bool {
	if isProduction() {
		ColorWarning.Printf("⚠ WARNING: You are in PRODUCTION environment!\n")
		ColorWarning.Printf("⚠ Operation: %s\n", operation)
		ColorWarning.Printf("⚠ This operation CANNOT be undone!\n")
		ColorPrompt.Printf("⚠ Type 'YES' to confirm (case-sensitive): ")

		var input string
		fmt.Scanln(&input)
		return input == "YES"
	}

	ColorWarning.Printf("⚠ Warning: %s cannot be undone.\n", operation)
	ColorPrompt.Printf("⚠ Continue? (y/N): ")

	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

// RequireTypedConfirmation requires exact text input for dangerous operations
func RequireTypedConfirmation(operation, requiredText string) bool {
	if isProduction() {
		ColorWarning.Printf("⚠ WARNING: You are in PRODUCTION environment!\n")
		ColorWarning.Printf("⚠ Operation: %s\n", operation)
		ColorWarning.Printf("⚠ This operation CANNOT be undone!\n")
		ColorPrompt.Printf("⚠ Type 'YES' to confirm (case-sensitive): ")

		var input string
		fmt.Scanln(&input)
		return input == "YES"
	}

	ColorWarning.Printf("⚠ WARNING: %s cannot be undone!\n", operation)
	ColorPrompt.Printf("⚠ Type '%s' to confirm: ", requiredText)

	var input string
	fmt.Scanln(&input)
	return input == requiredText
}

// PromptForInput prompts user for input with colored prompt
func PromptForInput(prompt string) string {
	ColorPrompt.Printf("%s: ", prompt)
	var input string
	fmt.Scanln(&input)
	return input
}

// isProduction checks if we're in production environment
func isProduction() bool {
	env := strings.ToLower(os.Getenv("VORM_ENV"))
	if env == "" {
		env = strings.ToLower(os.Getenv("APP_ENV"))
	}
	return env == "production" || env == "prod"
}
