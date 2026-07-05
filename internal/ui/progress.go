package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Bar is a single-line progress indicator rendered to stderr with carriage
// returns. With a known total it draws a filled bar plus percentage and ETA;
// with total <= 0 it shows an animated spinner (indeterminate mode).
//
// When stderr is not a terminal, per-frame rendering is suppressed to keep
// piped logs clean; Finish still prints one concise completion line.
type Bar struct {
	total       int64
	cur         int64
	label       string
	start       time.Time
	lastRender  time.Time
	spinner     int
	finished    bool
	interactive bool
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewBar creates and immediately draws a progress bar. total <= 0 selects
// indeterminate (spinner) mode.
func NewBar(total int64, label string) *Bar {
	b := &Bar{
		total:       total,
		label:       label,
		start:       time.Now(),
		interactive: IsTerminal(os.Stderr) && colorEnabled,
	}
	b.render(true)
	return b
}

// Set updates the current progress value and redraws (throttled).
func (b *Bar) Set(cur int64) {
	if b.finished {
		return
	}
	b.cur = cur
	b.render(false)
}

// Add increments progress by n.
func (b *Bar) Add(n int64) { b.Set(b.cur + n) }

// SetLabel changes the trailing label shown next to the bar.
func (b *Bar) SetLabel(label string) {
	b.label = label
	b.render(false)
}

// Finish completes the bar, drawing a final 100% state and a newline.
func (b *Bar) Finish() {
	if b.finished {
		return
	}
	b.finished = true
	if b.total > 0 {
		b.cur = b.total
	}
	elapsed := time.Since(b.start).Round(time.Millisecond)
	if b.interactive {
		b.clearLine()
		fmt.Fprintf(Err, "%s %s %s\n",
			Green(iconSuccess()), b.label, Dim("("+Duration(elapsed)+")"))
	} else {
		fmt.Fprintf(Err, "%s %s (%s)\n", iconSuccess(), b.label, Duration(elapsed))
	}
}

// Fail aborts the bar, clearing the line (the caller prints the error).
func (b *Bar) Fail() {
	if b.finished {
		return
	}
	b.finished = true
	if b.interactive {
		b.clearLine()
	}
}

func (b *Bar) clearLine() {
	fmt.Fprint(Err, "\r\033[K")
}

// render draws the current frame. force bypasses the throttle.
func (b *Bar) render(force bool) {
	if !b.interactive || b.finished {
		return
	}
	now := time.Now()
	if !force && now.Sub(b.lastRender) < 70*time.Millisecond {
		return
	}
	b.lastRender = now
	b.spinner = (b.spinner + 1) % len(spinnerFrames)

	if b.total <= 0 {
		fmt.Fprintf(Err, "\r\033[K%s %s %s",
			Cyan(spinnerFrames[b.spinner]), b.label,
			Dim(Duration(time.Since(b.start).Round(time.Second))))
		return
	}

	ratio := float64(b.cur) / float64(b.total)
	if ratio > 1 {
		ratio = 1
	}
	const barWidth = 28
	filled := int(ratio * barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	eta := ""
	if b.cur > 0 {
		elapsed := time.Since(b.start).Seconds()
		remain := time.Duration((elapsed/float64(b.cur))*float64(b.total-b.cur)) * time.Second
		eta = "ETA " + Duration(remain.Round(time.Second))
	}
	fmt.Fprintf(Err, "\r\033[K%s %s %s %s",
		Cyan(bar),
		Bold(strconv.Itoa(int(ratio*100))+"%"),
		b.label,
		Dim(eta))
}
