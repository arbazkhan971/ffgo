// Package ui provides the terminal presentation layer for ffgo: colors,
// progress bars, tables, humanized values and styled status output.
//
// It has zero third-party dependencies. Color and Unicode output degrade
// gracefully when stdout/stderr is not a terminal, when NO_COLOR is set,
// or when the user passes --no-color.
package ui

import (
	"os"
	"strings"
)

// ANSI SGR codes used across the UI. Kept internal; callers use the Style
// helpers below rather than raw escapes.
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiItalic    = "\033[3m"
	ansiUnderline = "\033[4m"

	fgRed     = "\033[31m"
	fgGreen   = "\033[32m"
	fgYellow  = "\033[33m"
	fgBlue    = "\033[34m"
	fgMagenta = "\033[35m"
	fgCyan    = "\033[36m"
	fgGray    = "\033[90m"

	fgBrightRed   = "\033[91m"
	fgBrightGreen = "\033[92m"
	fgBrightCyan  = "\033[96m"
)

// colorEnabled reflects whether ANSI styling should be emitted. It is
// initialized from the environment and TTY detection, and can be overridden
// via SetColor (e.g. from the --no-color / --color flags).
var colorEnabled = detectColor()

// unicodeEnabled controls whether we use fancy glyphs (✓, ▏) versus ASCII
// fallbacks. Tied to color/TTY by default so dumb terminals stay readable.
var unicodeEnabled = colorEnabled

// detectColor decides the default color mode.
//
// Precedence: NO_COLOR disables (https://no-color.org). FORCE_COLOR or a
// CLICOLOR_FORCE flag enables regardless of TTY. Otherwise color is enabled
// only when stdout is a terminal and TERM is not "dumb".
func detectColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if v, ok := os.LookupEnv("FORCE_COLOR"); ok && v != "0" && v != "false" {
		return true
	}
	if v, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && v != "0" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsTerminal(os.Stdout)
}

// SetColor overrides color output. Passing false also disables fancy glyphs.
func SetColor(on bool) {
	colorEnabled = on
	if !on {
		unicodeEnabled = false
	} else {
		unicodeEnabled = IsTerminal(os.Stdout)
	}
}

// ColorEnabled reports whether ANSI styling is currently active.
func ColorEnabled() bool { return colorEnabled }

// IsTerminal reports whether f refers to a terminal (character device).
// It avoids a syscall dependency by inspecting the file mode, which is
// portable across Linux, macOS and Windows consoles.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// wrap applies one or more SGR codes to s when color is enabled.
func wrap(s string, codes ...string) string {
	if !colorEnabled || s == "" {
		return s
	}
	return strings.Join(codes, "") + s + ansiReset
}

// Style helpers. Each returns s unchanged when color is disabled.

func Bold(s string) string      { return wrap(s, ansiBold) }
func Dim(s string) string       { return wrap(s, ansiDim) }
func Italic(s string) string    { return wrap(s, ansiItalic) }
func Underline(s string) string { return wrap(s, ansiUnderline) }

func Red(s string) string     { return wrap(s, fgRed) }
func Green(s string) string   { return wrap(s, fgGreen) }
func Yellow(s string) string  { return wrap(s, fgYellow) }
func Blue(s string) string    { return wrap(s, fgBlue) }
func Magenta(s string) string { return wrap(s, fgMagenta) }
func Cyan(s string) string    { return wrap(s, fgCyan) }
func Gray(s string) string    { return wrap(s, fgGray) }

func BoldCyan(s string) string  { return wrap(s, ansiBold, fgCyan) }
func BoldGreen(s string) string { return wrap(s, ansiBold, fgBrightGreen) }
func BoldRed(s string) string   { return wrap(s, ansiBold, fgBrightRed) }

// Glyph returns fancy when Unicode output is enabled, else the ASCII fallback.
func Glyph(fancy, ascii string) string {
	if unicodeEnabled {
		return fancy
	}
	return ascii
}
