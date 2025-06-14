package console

import (
	"os"

	"github.com/fatih/color"
)

// Color definitions - DO NOT CHANGE (as per requirements)
var (
	ColorSuccess   = color.New(color.FgGreen, color.Bold)
	ColorError     = color.New(color.FgRed, color.Bold)
	ColorWarning   = color.New(color.FgYellow, color.Bold)
	ColorInfo      = color.New(color.FgBlue, color.Bold)
	ColorDebug     = color.New(color.FgMagenta)
	ColorPrompt    = color.New(color.FgCyan, color.Bold)
	ColorHighlight = color.New(color.FgWhite, color.Bold)
)

func init() {
	// Force color output if NO_COLOR is not set
	if os.Getenv("NO_COLOR") == "" {
		color.NoColor = false
	}
}
