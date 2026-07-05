// Package ffmpeg is ffgo's execution engine. It locates the ffmpeg binary,
// builds invocations, runs them while streaming a live progress bar, and
// surfaces clean errors. It also renders the exact command for --dry-run and
// the explain subcommand so every generated FFmpeg call is transparent.
package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/arbazkhan971/ffgo/internal/ffbin"
	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

// Engine runs a specific ffmpeg binary and knows where ffprobe lives.
type Engine struct {
	Path      string // ffmpeg
	ProbePath string // ffprobe
}

// New locates ffmpeg (and ffprobe) on the system. ffprobe is optional — some
// commands never need it — so a missing ffprobe is tolerated here and only
// reported when a command actually probes.
func New() (*Engine, error) {
	fm, err := ffbin.Locate("ffmpeg", "FFGO_FFMPEG", "FFMPEG_PATH")
	if err != nil {
		return nil, err
	}
	e := &Engine{Path: fm}
	if fp, err := ffbin.Locate("ffprobe", "FFGO_FFPROBE", "FFPROBE_PATH"); err == nil {
		e.ProbePath = fp
	}
	return e, nil
}

// Prober returns an ffprobe wrapper bound to this engine's ffprobe path.
func (e *Engine) Prober() (*ffprobe.Prober, error) {
	if e.ProbePath == "" {
		return ffprobe.New()
	}
	return ffprobe.At(e.ProbePath), nil
}

// Probe is a convenience wrapper returning decoded ffprobe output.
func (e *Engine) Probe(ctx context.Context, input string) (*ffprobe.Result, error) {
	p, err := e.Prober()
	if err != nil {
		return nil, err
	}
	return p.Probe(ctx, input)
}

// RunOptions configures a single ffmpeg invocation.
type RunOptions struct {
	// Args are the ffmpeg arguments after global flags: typically the input(s),
	// filters, codecs and the output path. The engine injects its own logging
	// and progress flags; do not include -y, -progress, -loglevel or -nostats.
	Args []string

	// Total is the expected output duration, used to render a percentage bar.
	// Zero selects an indeterminate spinner.
	Total time.Duration

	// Label is shown next to the progress bar, e.g. "Compressing video.mp4".
	Label string

	// Overwrite adds -y (otherwise -n; ffmpeg refuses to clobber existing output).
	Overwrite bool

	// DryRun prints the command to stdout and returns without executing.
	DryRun bool

	// ShowCommand prints the command to stderr before executing.
	ShowCommand bool

	// Quiet suppresses the progress bar (the command still runs).
	Quiet bool
}

// DisplayArgs returns the clean, copy-pasteable argument list (without ffgo's
// internal progress plumbing) used for --dry-run, --show-command and explain.
func (e *Engine) DisplayArgs(opts RunOptions) []string {
	a := []string{"-hide_banner"}
	if opts.Overwrite {
		a = append(a, "-y")
	}
	return append(a, opts.Args...)
}

// execArgs returns the full argument list actually passed to ffmpeg, including
// the logging and progress flags the engine manages.
func (e *Engine) execArgs(opts RunOptions) []string {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostats"}
	if opts.Overwrite {
		a = append(a, "-y")
	} else {
		a = append(a, "-n")
	}
	a = append(a, "-progress", "pipe:1", "-stats_period", "0.2")
	return append(a, opts.Args...)
}

// Run executes ffmpeg with a live progress bar, or prints the command for a
// dry run. It returns a rich error including ffmpeg's own message on failure.
func (e *Engine) Run(ctx context.Context, opts RunOptions) error {
	display := ui.CommandLine("ffmpeg", e.DisplayArgs(opts))

	if opts.DryRun {
		ui.Printf("%s\n", display)
		return nil
	}
	if opts.ShowCommand {
		fmt.Fprintf(ui.Err, "%s %s\n", ui.Dim("$"), display)
	}

	cmd := exec.CommandContext(ctx, e.Path, e.execArgs(opts)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	stderr := &tailBuffer{limit: 8 << 10}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg: %w", err)
	}

	var bar *ui.Bar
	if !opts.Quiet {
		bar = ui.NewBar(int64(opts.Total/time.Millisecond), opts.Label)
	}
	consumeProgress(stdout, opts.Total, bar)

	waitErr := cmd.Wait()
	if waitErr != nil {
		if bar != nil {
			bar.Fail()
		}
		return decodeError(waitErr, stderr.String())
	}
	if bar != nil {
		bar.Finish()
	}
	return nil
}

// consumeProgress drains ffmpeg's -progress stream, updating the bar. It must
// fully consume stdout so ffmpeg never blocks on a full pipe.
func consumeProgress(r io.Reader, total time.Duration, bar *ui.Bar) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "out_time_us", "out_time_ms":
			// Both are reported in microseconds by ffmpeg.
			if bar != nil && total > 0 {
				if us := parseMicros(val); us >= 0 {
					bar.Set(int64(time.Duration(us) * time.Microsecond / time.Millisecond))
				}
			}
		case "progress":
			if strings.TrimSpace(val) == "end" && bar != nil && total > 0 {
				bar.Set(int64(total / time.Millisecond))
			}
		}
	}
}

// decodeError turns ffmpeg's exit into a helpful message using its stderr tail.
func decodeError(runErr error, stderrTail string) error {
	tail := strings.TrimSpace(stderrTail)
	// Keep only the last few non-empty lines; ffmpeg errors are usually terse.
	if tail != "" {
		lines := splitNonEmpty(tail)
		if n := len(lines); n > 4 {
			lines = lines[n-4:]
		}
		tail = strings.Join(lines, "\n")
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		if tail != "" {
			return fmt.Errorf("ffmpeg failed:\n%s", indent(tail, "  "))
		}
		return fmt.Errorf("ffmpeg exited with status %d", exitErr.ExitCode())
	}
	if tail != "" {
		return fmt.Errorf("ffmpeg failed: %w\n%s", runErr, indent(tail, "  "))
	}
	return fmt.Errorf("ffmpeg failed: %w", runErr)
}
