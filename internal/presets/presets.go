// Package presets defines compression profiles (platform targets and quality
// levels) and the math that turns a desired output size into an encoding plan.
// It is shared by the compress, batch and ai commands so they all speak the
// same language.
package presets

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Encoding fully describes how to (re-)encode a file. Zero-valued fields are
// omitted from the generated arguments, so an Encoding can express both
// CRF-based (quality) and bitrate-based (size) encodes.
type Encoding struct {
	VideoCodec    string // libx264, libx265, libvpx-vp9; "copy" to stream-copy
	CRF           int    // constant-rate-factor; used when VideoBitrateK == 0
	Preset        string // x264/x265 speed preset
	Tune          string // optional -tune value
	VideoBitrateK int    // target video bitrate in kbps (size mode)
	MaxrateK      int    // optional cap (size mode)
	BufsizeK      int    // rate-control buffer (size mode)
	MaxWidth      int    // downscale if wider (keeps aspect, even dims)
	FPS           int    // cap frame rate
	PixFmt        string // e.g. yuv420p for broad compatibility
	AudioCodec    string // aac, libopus, libmp3lame; "copy"; "none" to drop
	AudioBitrateK int    // audio bitrate in kbps
}

// Preset is a named, opinionated target. A non-zero TargetMB switches compress
// into size-targeting mode using Enc's codecs/scaling as the template.
type Preset struct {
	Name        string
	Description string
	TargetMB    float64
	Enc         Encoding
}

// registry is the built-in preset table.
var registry = map[string]Preset{
	"whatsapp": {
		Name:        "whatsapp",
		Description: "Small, compatible clip for WhatsApp (≤848px, H.264/AAC)",
		Enc: Encoding{VideoCodec: "libx264", CRF: 28, Preset: "veryfast",
			MaxWidth: 848, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128},
	},
	"youtube": {
		Name:        "youtube",
		Description: "High-quality H.264 master for YouTube uploads",
		Enc: Encoding{VideoCodec: "libx264", CRF: 18, Preset: "slow",
			PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 384},
	},
	"discord": {
		Name:        "discord",
		Description: "Fits Discord's 25 MB upload limit (H.264/AAC)",
		TargetMB:    24.5,
		Enc: Encoding{VideoCodec: "libx264", Preset: "medium",
			PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128},
	},
	"telegram": {
		Name:        "telegram",
		Description: "Compatible H.264 clip for Telegram",
		Enc: Encoding{VideoCodec: "libx264", CRF: 26, Preset: "veryfast",
			MaxWidth: 1280, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128},
	},
	"twitter": {
		Name:        "twitter",
		Description: "Twitter/X-friendly 1280px H.264 clip",
		Enc: Encoding{VideoCodec: "libx264", CRF: 23, Preset: "medium",
			MaxWidth: 1280, FPS: 30, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128},
	},
	"web": {
		Name:        "web",
		Description: "Balanced 1080p H.264 for websites and embeds",
		Enc: Encoding{VideoCodec: "libx264", CRF: 23, Preset: "medium",
			MaxWidth: 1920, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 160},
	},
	"email": {
		Name:        "email",
		Description: "Tiny clip under 10 MB for email attachments",
		TargetMB:    9.5,
		Enc: Encoding{VideoCodec: "libx264", Preset: "medium", MaxWidth: 1280,
			PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 96},
	},
}

// Lookup returns a preset by name (case-insensitive).
func Lookup(name string) (Preset, bool) {
	p, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// Names returns the preset names sorted alphabetically.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Describe returns "name — description" lines for help text.
func Describe() []string {
	var out []string
	for _, n := range Names() {
		out = append(out, fmt.Sprintf("%-9s %s", n, registry[n].Description))
	}
	return out
}

// Quality maps a subjective level to a CRF for H.264/H.265.
type Quality string

const (
	QualityLow    Quality = "low"
	QualityMedium Quality = "medium"
	QualityHigh   Quality = "high"
)

// CRF returns the constant-rate-factor for a quality level (H.264 scale).
func (q Quality) CRF() (int, bool) {
	switch Quality(strings.ToLower(string(q))) {
	case QualityLow:
		return 30, true
	case QualityMedium:
		return 24, true
	case QualityHigh:
		return 20, true
	default:
		return 0, false
	}
}

// QualityEncoding returns a sensible H.264 encoding for a quality level.
func QualityEncoding(q Quality) (Encoding, error) {
	crf, ok := q.CRF()
	if !ok {
		return Encoding{}, fmt.Errorf("unknown quality %q (use low, medium or high)", q)
	}
	return Encoding{
		VideoCodec: "libx264", CRF: crf, Preset: "medium",
		PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 160,
	}, nil
}

// muxOverhead reserves a fraction of the size budget for container overhead.
const muxOverhead = 0.03

// SolveBitrate computes the video bitrate (kbps) needed to land near
// targetBytes over dur, after reserving space for audio and container overhead.
func SolveBitrate(targetBytes int64, dur time.Duration, audioKbps int) (int, error) {
	if dur <= 0 {
		return 0, fmt.Errorf("cannot target a size without a known duration")
	}
	if targetBytes <= 0 {
		return 0, fmt.Errorf("target size must be positive")
	}
	seconds := dur.Seconds()
	budgetBits := float64(targetBytes) * 8 * (1 - muxOverhead)
	audioBits := float64(audioKbps) * 1000 * seconds
	videoBits := budgetBits - audioBits
	if videoBits <= 0 {
		return 0, fmt.Errorf("target size %s is too small for %ds of %dk audio",
			humanMB(targetBytes), int(seconds), audioKbps)
	}
	kbps := int(videoBits / 1000 / seconds)
	if kbps < 1 {
		kbps = 1
	}
	return kbps, nil
}

func humanMB(b int64) string {
	return strconv.FormatFloat(float64(b)/(1<<20), 'f', 1, 64) + " MB"
}

// videoFilters returns the -vf filter chain parts implied by scaling/fps.
func (e Encoding) videoFilters() []string {
	var f []string
	if e.FPS > 0 {
		f = append(f, "fps="+strconv.Itoa(e.FPS))
	}
	if e.MaxWidth > 0 {
		// Downscale only if wider than MaxWidth; keep aspect, force even height.
		f = append(f, fmt.Sprintf("scale='min(%d,iw)':-2", e.MaxWidth))
	}
	return f
}

// VideoArgs returns the video-related output arguments (codec, rate control,
// filters, pixel format) but not audio and not the output path.
func (e Encoding) VideoArgs() []string {
	var a []string
	a = append(a, "-c:v", e.VideoCodec)
	if e.VideoCodec != "copy" {
		if e.VideoBitrateK > 0 {
			a = append(a, "-b:v", fmt.Sprintf("%dk", e.VideoBitrateK))
			if e.MaxrateK > 0 {
				a = append(a, "-maxrate", fmt.Sprintf("%dk", e.MaxrateK))
			}
			if e.BufsizeK > 0 {
				a = append(a, "-bufsize", fmt.Sprintf("%dk", e.BufsizeK))
			}
		} else if e.CRF > 0 {
			a = append(a, "-crf", strconv.Itoa(e.CRF))
		}
		if e.Preset != "" {
			a = append(a, "-preset", e.Preset)
		}
		if e.Tune != "" {
			a = append(a, "-tune", e.Tune)
		}
		if f := e.videoFilters(); len(f) > 0 {
			a = append(a, "-vf", strings.Join(f, ","))
		}
		if e.PixFmt != "" {
			a = append(a, "-pix_fmt", e.PixFmt)
		}
	}
	return a
}

// AudioArgs returns the audio-related output arguments.
func (e Encoding) AudioArgs() []string {
	switch e.AudioCodec {
	case "", "none":
		return []string{"-an"}
	case "copy":
		return []string{"-c:a", "copy"}
	default:
		a := []string{"-c:a", e.AudioCodec}
		if e.AudioBitrateK > 0 {
			a = append(a, "-b:a", fmt.Sprintf("%dk", e.AudioBitrateK))
		}
		return a
	}
}

// OutputArgs returns the full set of output encoding arguments (video + audio).
func (e Encoding) OutputArgs() []string {
	return append(e.VideoArgs(), e.AudioArgs()...)
}
