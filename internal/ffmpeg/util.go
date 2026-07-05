package ffmpeg

import (
	"strconv"
	"strings"
)

// tailBuffer captures the last `limit` bytes written to it, used to keep the
// tail of ffmpeg's stderr for error reporting without buffering gigabytes.
type tailBuffer struct {
	limit int
	buf   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = t.buf[len(t.buf)-t.limit:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.buf) }

// parseMicros parses an integer microsecond value, returning -1 for the
// "N/A" placeholder ffmpeg emits before the first frame.
func parseMicros(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return -1
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
