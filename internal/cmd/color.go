package cmd

import (
	"os"
)

// ANSI color escape codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

var useColor bool

func init() {
	// Enable color if stdout is a terminal and NO_COLOR is not set
	useColor = isTerminal() && os.Getenv("NO_COLOR") == ""
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func colorize(color, text string) string {
	if !useColor {
		return text
	}
	return color + text + colorReset
}

func green(text string) string  { return colorize(colorGreen, text) }
func red(text string) string    { return colorize(colorRed, text) }
func yellow(text string) string { return colorize(colorYellow, text) }
func cyan(text string) string   { return colorize(colorCyan, text) }
func bold(text string) string   { return colorize(colorBold, text) }
