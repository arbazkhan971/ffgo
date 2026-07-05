package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Status and structured output helpers. Data intended for machine consumption
// (JSON, the inspect report body) goes to stdout; human status/progress goes
// to stderr so that `ffgo ... | jq` and shell redirection stay clean.

var (
	// Out is the destination for primary results. Overridable in tests.
	Out io.Writer = os.Stdout
	// Err is the destination for status, warnings and progress.
	Err io.Writer = os.Stderr
)

// Symbols, resolved lazily so a late SetColor still affects them.
func iconSuccess() string { return Glyph("✓", "+") }
func iconWarn() string    { return Glyph("⚠", "!") }
func iconError() string   { return Glyph("✗", "x") }
func iconInfo() string    { return Glyph("ℹ", "i") }
func iconArrow() string   { return Glyph("→", "->") }
func iconBullet() string  { return Glyph("•", "*") }

// Successf prints a green success line to stderr.
func Successf(format string, a ...any) {
	fmt.Fprintf(Err, "%s %s\n", Green(iconSuccess()), fmt.Sprintf(format, a...))
}

// Warnf prints a yellow warning line to stderr.
func Warnf(format string, a ...any) {
	fmt.Fprintf(Err, "%s %s\n", Yellow(iconWarn()), fmt.Sprintf(format, a...))
}

// Errorf prints a red error line to stderr.
func Errorf(format string, a ...any) {
	fmt.Fprintf(Err, "%s %s\n", Red(iconError()), fmt.Sprintf(format, a...))
}

// Infof prints a cyan informational line to stderr.
func Infof(format string, a ...any) {
	fmt.Fprintf(Err, "%s %s\n", Cyan(iconInfo()), fmt.Sprintf(format, a...))
}

// Stepf prints an arrow-prefixed progress step to stderr.
func Stepf(format string, a ...any) {
	fmt.Fprintf(Err, "%s %s\n", Cyan(iconArrow()), fmt.Sprintf(format, a...))
}

// Heading prints a bold section header to stderr with a subtle underline rule.
func Heading(title string) {
	fmt.Fprintf(Err, "\n%s\n", Bold(title))
}

// Field prints an aligned "key   value" pair to stderr. keyWidth pads the key
// column; pass the max key length in the group for clean alignment.
func Field(key, value string, keyWidth int) {
	pad := keyWidth - len([]rune(key))
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(Err, "  %s%s  %s\n", Dim(key), strings.Repeat(" ", pad), value)
}

// Bullet prints an indented bullet line to stderr.
func Bullet(format string, a ...any) {
	fmt.Fprintf(Err, "  %s %s\n", Cyan(iconBullet()), fmt.Sprintf(format, a...))
}

// Println writes a line to the primary output (stdout).
func Println(a ...any) { fmt.Fprintln(Out, a...) }

// Printf writes to the primary output (stdout).
func Printf(format string, a ...any) { fmt.Fprintf(Out, format, a...) }

// Rule prints a horizontal divider of the given width to stderr.
func Rule(width int) {
	if width <= 0 {
		width = 48
	}
	fmt.Fprintln(Err, Dim(strings.Repeat(Glyph("─", "-"), width)))
}

// CommandLine renders a shell command for display: the binary in bold cyan and
// arguments quoted where necessary. Used by --dry-run and --show-command.
func CommandLine(bin string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, BoldCyan(shellQuote(bin)))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// shellQuote wraps an argument in single quotes when it contains characters
// that a POSIX shell would interpret.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'\\$`*?()[]{}<>|&;#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
