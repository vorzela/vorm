package main

import (
	"os"
	"strings"
)

// ANSI codes — same palette as vm (vorzela-migrate). Disabled when stdout is
// not a TTY or when NO_COLOR / VORM_NO_COLOR is set.
const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
)

func colorEnabled() bool {
	if v := os.Getenv("NO_COLOR"); v != "" {
		return false
	}
	if v := os.Getenv("VORM_NO_COLOR"); v != "" && v != "0" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorEnabled() || s == "" {
		return s
	}
	return code + s + cReset
}

func green(s string) string  { return paint(cGreen, s) }
func yellow(s string) string { return paint(cYellow, s) }
func gray(s string) string   { return paint(cGray, s) }

func greenBold(s string) string {
	if !colorEnabled() {
		return s
	}
	return cGreen + cBold + s + cReset
}

func redBold(s string) string {
	if !colorEnabled() {
		return s
	}
	return cRed + cBold + s + cReset
}

func yellowBold(s string) string {
	if !colorEnabled() {
		return s
	}
	return cYellow + cBold + s + cReset
}

func cyanBold(s string) string {
	if !colorEnabled() {
		return s
	}
	return cCyan + cBold + s + cReset
}

// padDots fills between a migration name and its status like Laravel's migrate.
func padDots(name, status string, width int) string {
	if width < 8 {
		width = 8
	}
	n := width - len(name) - len(status) - 2
	if n < 3 {
		n = 3
	}
	return name + " " + strings.Repeat(".", n) + " " + status
}
