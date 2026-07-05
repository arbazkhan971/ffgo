package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Table is a minimal, dependency-free column-aligned table renderer with
// optional colored headers. It measures display width using rune counts,
// which is correct for the ASCII/common cases ffgo emits.
type Table struct {
	headers []string
	rows    [][]string
	// align holds 'l' or 'r' per column; defaults to left.
	align []byte
}

// NewTable creates a table with the given header labels.
func NewTable(headers ...string) *Table {
	return &Table{headers: headers, align: make([]byte, len(headers))}
}

// RightAlign marks a column (0-indexed) as right-aligned, useful for numbers.
func (t *Table) RightAlign(cols ...int) *Table {
	for _, c := range cols {
		if c >= 0 && c < len(t.align) {
			t.align[c] = 'r'
		}
	}
	return t
}

// Row appends a row. Extra cells are ignored; missing cells render empty.
func (t *Table) Row(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	return t
}

// Len reports the number of data rows.
func (t *Table) Len() int { return len(t.rows) }

func cellAt(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

// Render writes the table to w with two-space gutters between columns.
func (t *Table) Render(w io.Writer) {
	cols := len(t.headers)
	widths := make([]int, cols)
	for i, h := range t.headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range t.rows {
		for i := 0; i < cols; i++ {
			if l := utf8.RuneCountInString(cellAt(r, i)); l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Header row (bold), then rule, then data.
	var b strings.Builder
	for i, h := range t.headers {
		writeCell(&b, Bold(h), h, widths[i], t.align[i])
		if i < cols-1 {
			b.WriteString("  ")
		}
	}
	b.WriteByte('\n')
	for i := range t.headers {
		b.WriteString(Dim(strings.Repeat(Glyph("─", "-"), widths[i])))
		if i < cols-1 {
			b.WriteString("  ")
		}
	}
	b.WriteByte('\n')
	for _, r := range t.rows {
		for i := 0; i < cols; i++ {
			c := cellAt(r, i)
			writeCell(&b, c, c, widths[i], t.align[i])
			if i < cols-1 {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	fmt.Fprint(w, b.String())
}

// writeCell writes styled with padding computed from the plain text width, so
// ANSI escapes in styled do not distort alignment.
func writeCell(b *strings.Builder, styled, plain string, width int, align byte) {
	pad := width - utf8.RuneCountInString(plain)
	if pad < 0 {
		pad = 0
	}
	if align == 'r' {
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(styled)
	} else {
		b.WriteString(styled)
		b.WriteString(strings.Repeat(" ", pad))
	}
}
