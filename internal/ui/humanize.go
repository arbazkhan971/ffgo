package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Bytes formats a byte count as a human-readable string using binary units
// with a decimal-ish presentation familiar from tools like ls -h and du -h.
//
//	1536      -> "1.5 KB"
//	5_242_880 -> "5.0 MB"
func Bytes(n int64) string {
	if n < 0 {
		return "-" + Bytes(-n)
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	val := float64(n) / float64(div)
	// Show one decimal below 100, none above, to keep columns tidy.
	if val >= 100 {
		return fmt.Sprintf("%.0f %s", val, units[exp])
	}
	return fmt.Sprintf("%.1f %s", val, units[exp])
}

// ParseBytes parses a human byte size such as "25mb", "1.5 GB", "900k" or a
// bare number of bytes. It is deliberately lenient about spacing and case.
func ParseBytes(s string) (int64, error) {
	orig := s
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "b") // "mb" -> "m", "kb" -> "k", bare "b" -> ""
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "k"):
		mult, s = 1<<10, strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult, s = 1<<20, strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "g"):
		mult, s = 1<<30, strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "t"):
		mult, s = 1<<40, strings.TrimSuffix(s, "t")
	}
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", orig)
	}
	if f < 0 {
		return 0, fmt.Errorf("size cannot be negative: %q", orig)
	}
	return int64(f * float64(mult)), nil
}

// Bitrate formats bits-per-second as "N.N Mbps" / "N kbps".
func Bitrate(bps int64) string {
	if bps <= 0 {
		return "—"
	}
	switch {
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.0f kbps", float64(bps)/1_000)
	default:
		return fmt.Sprintf("%d bps", bps)
	}
}

// Duration formats a duration as H:MM:SS(.mmm) or M:SS depending on length.
// Fractional seconds are shown only when non-zero.
func Duration(d time.Duration) string {
	if d < 0 {
		return "-" + Duration(-d)
	}
	total := d.Seconds()
	h := int(total) / 3600
	m := (int(total) % 3600) / 60
	s := int(total) % 60
	ms := int(math.Round((total-math.Floor(total))*1000)) % 1000
	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%d:%02d:%02d", h, m, s)
	} else {
		fmt.Fprintf(&b, "%d:%02d", m, s)
	}
	if ms > 0 {
		fmt.Fprintf(&b, ".%03d", ms)
	}
	return b.String()
}

// Clock formats a duration as HH:MM:SS.mmm, the canonical timestamp form
// accepted by ffmpeg's -ss/-to options.
func Clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := d.Seconds()
	h := int(total) / 3600
	m := (int(total) % 3600) / 60
	s := total - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, s)
}

// Percent formats a 0..1 ratio as an integer percentage, clamped.
func Percent(ratio float64) string {
	if math.IsNaN(ratio) || ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return fmt.Sprintf("%d%%", int(math.Round(ratio*100)))
}

// ParseTimecode parses a timestamp into a duration. It accepts:
//   - clock form: "HH:MM:SS", "MM:SS", with optional ".mmm" fraction
//   - Go duration form: "1m30s", "1.5s", "500ms"
//   - a bare number of seconds: "90", "12.5"
func ParseTimecode(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty timecode")
	}
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) > 3 {
			return 0, fmt.Errorf("invalid timecode %q", s)
		}
		var total float64
		for _, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid timecode %q", s)
			}
			total = total*60 + v
		}
		return time.Duration(total * float64(time.Second)), nil
	}
	// Unit form like "1m30s" / "500ms" / "10s".
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Bare seconds.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(f * float64(time.Second)), nil
	}
	return 0, fmt.Errorf("invalid timecode %q", s)
}

// Count formats an integer with thousands separators (e.g. 1234567 -> "1,234,567").
func Count(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	if neg {
		return "-" + out.String()
	}
	return out.String()
}
