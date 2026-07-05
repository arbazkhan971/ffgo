package ffprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/arbazkhan971/ffgo/internal/ffbin"
)

// Prober runs a specific ffprobe binary.
type Prober struct {
	Path string
}

// New locates ffprobe on the system (honoring FFGO_FFPROBE / FFPROBE_PATH).
func New() (*Prober, error) {
	path, err := ffbin.Locate("ffprobe", "FFGO_FFPROBE", "FFPROBE_PATH")
	if err != nil {
		return nil, err
	}
	return &Prober{Path: path}, nil
}

// At returns a Prober using the given ffprobe path.
func At(path string) *Prober { return &Prober{Path: path} }

// Probe inspects a media file (or URL) and returns the decoded result.
func (p *Prober) Probe(ctx context.Context, input string) (*Result, error) {
	args := []string{
		"-v", "error",
		"-hide_banner",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		"-of", "json",
		input,
	}
	cmd := exec.CommandContext(ctx, p.Path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := bytes.TrimSpace(stderr.Bytes())
		if len(msg) > 0 {
			return nil, fmt.Errorf("ffprobe failed for %q: %s", input, msg)
		}
		return nil, fmt.Errorf("ffprobe failed for %q: %w", input, err)
	}

	var res Result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return nil, fmt.Errorf("parsing ffprobe output for %q: %w", input, err)
	}
	if res.Format.Filename == "" && len(res.Streams) == 0 {
		return nil, fmt.Errorf("%q does not appear to be a media file", input)
	}
	return &res, nil
}

// RawJSON returns ffprobe's untouched JSON for a file, used by `inspect --json`
// when the caller wants the full fidelity ffprobe payload.
func (p *Prober) RawJSON(ctx context.Context, input string) ([]byte, error) {
	res, err := p.Probe(ctx, input)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(res, "", "  ")
}
