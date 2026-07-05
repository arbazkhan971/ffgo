// Package explain turns a raw FFmpeg argument list into a human-readable,
// ordered walkthrough. It is a static analyser: it never runs FFmpeg, it simply
// maps flags and their values to plain-English explanations so people can
// understand a command they were handed (or one ffgo itself is about to run).
package explain

import (
	"fmt"
	"strconv"
	"strings"
)

// Segment is a single explained piece of a command: the literal Token as it
// appeared (a flag together with its value, or a bare output path) and a
// one-sentence Detail describing what it does.
type Segment struct {
	Token  string
	Detail string
}

// flagSpec describes how to explain one flag.
type flagSpec struct {
	// takesValue reports whether the flag consumes the following token.
	takesValue bool
	// explain builds the Detail string. value is "" when takesValue is false.
	explain func(flag, value string) string
}

// Explain walks an FFmpeg argument slice (WITHOUT the leading "ffmpeg") and
// returns an ordered explanation of every flag, its value, and the output file.
// Unrecognised flags fall back to a generic note; a bare token in output
// position is described as the output file.
func Explain(args []string) []Segment {
	var out []Segment
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			// A bare token that is not a flag value is the output file.
			out = append(out, Segment{
				Token:  arg,
				Detail: fmt.Sprintf("output file: %s", arg),
			})
			continue
		}

		spec, ok := lookup(arg)
		if !ok {
			// Unknown flag: peek at the next token to guess whether it is a value.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				value := args[i+1]
				i++
				out = append(out, Segment{
					Token:  arg + " " + value,
					Detail: fmt.Sprintf("option %s with value %q", arg, value),
				})
			} else {
				out = append(out, Segment{
					Token:  arg,
					Detail: fmt.Sprintf("option %s", arg),
				})
			}
			continue
		}

		if spec.takesValue && i+1 < len(args) {
			value := args[i+1]
			i++
			out = append(out, Segment{
				Token:  arg + " " + value,
				Detail: spec.explain(arg, value),
			})
			continue
		}

		out = append(out, Segment{
			Token:  arg,
			Detail: spec.explain(arg, ""),
		})
	}
	return out
}

// lookup resolves a flag to its spec, normalising stream-specifier variants
// such as -c:v, -b:a and -q:a to their base handler.
func lookup(flag string) (flagSpec, bool) {
	if spec, ok := flagTable[flag]; ok {
		return spec, true
	}
	// Handle stream-specifier forms like -c:v, -codec:a, -b:v, -q:a.
	base := flag
	if idx := strings.IndexByte(flag, ':'); idx >= 0 {
		base = flag[:idx]
	}
	switch base {
	case "-c", "-codec":
		return flagSpec{takesValue: true, explain: explainCodec}, true
	case "-b":
		return flagSpec{takesValue: true, explain: explainBitrate}, true
	case "-q", "-qscale":
		return flagSpec{takesValue: true, explain: explainQscale}, true
	case "-filter":
		return flagSpec{takesValue: true, explain: explainFilter}, true
	}
	return flagSpec{}, false
}

// streamKind turns a stream specifier suffix into a friendly noun.
func streamKind(flag string) string {
	idx := strings.IndexByte(flag, ':')
	if idx < 0 {
		return "streams"
	}
	switch flag[idx+1:] {
	case "v", "V":
		return "video"
	case "a":
		return "audio"
	case "s":
		return "subtitle"
	case "d":
		return "data"
	default:
		return "stream " + flag[idx+1:]
	}
}

func explainCodec(flag, value string) string {
	kind := streamKind(flag)
	if value == "copy" {
		return fmt.Sprintf("copy the %s stream through unchanged (stream copy, lossless, no re-encode)", kind)
	}
	return fmt.Sprintf("encode the %s stream with the %q codec", kind, value)
}

func explainBitrate(flag, value string) string {
	kind := streamKind(flag)
	return fmt.Sprintf("target %s bitrate of %s (higher = better quality, larger file)", kind, value)
}

func explainQscale(flag, value string) string {
	kind := streamKind(flag)
	return fmt.Sprintf("fixed quality scale %s for %s (lower number = better quality)", value, kind)
}

func explainCRF(flag, value string) string {
	note := ""
	if n, err := strconv.Atoi(value); err == nil {
		switch {
		case n <= 18:
			note = " — near-lossless"
		case n <= 23:
			note = " — visually high quality"
		case n <= 28:
			note = " — balanced quality/size"
		default:
			note = " — smaller file, softer quality"
		}
	}
	return fmt.Sprintf("constant rate factor %s: quality target on a 0-51 scale where lower is better%s", value, note)
}

func explainFilter(flag, value string) string {
	label := "video"
	switch {
	case strings.HasPrefix(flag, "-af"):
		label = "audio"
	case strings.HasPrefix(flag, "-filter_complex"), flag == "-lc":
		label = "multi-stream"
	case strings.HasPrefix(flag, "-filter"):
		// -filter:a / -filter:v / -filter:s carry a stream specifier.
		label = ""
		if idx := strings.IndexByte(flag, ':'); idx >= 0 {
			switch flag[idx+1:] {
			case "a":
				label = "audio"
			case "v":
				label = "video"
			case "s":
				label = "subtitle"
			}
		}
	}
	graph := describeFilters(value)
	prefix := "filter graph"
	if label != "" {
		prefix = label + " filter graph"
	}
	if graph == "" {
		return fmt.Sprintf("%s: %s", prefix, value)
	}
	return fmt.Sprintf("%s applying %s", prefix, graph)
}

// describeFilters scans a filter-graph value and returns a comma-joined,
// human-readable list of the common filters it recognises.
func describeFilters(value string) string {
	// Split on graph separators: chains ';' and filters ','.
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var parts []string
	seen := make(map[string]bool)
	for _, f := range fields {
		name := filterName(f)
		desc, ok := filterDescs[name]
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		parts = append(parts, desc)
	}
	return strings.Join(parts, "; ")
}

// filterName extracts the filter name from a single filter expression,
// stripping input/output labels like "[0:v]scale=..." and arguments.
func filterName(f string) string {
	f = strings.TrimSpace(f)
	// Drop leading [label] pads.
	for strings.HasPrefix(f, "[") {
		if idx := strings.IndexByte(f, ']'); idx >= 0 {
			f = strings.TrimSpace(f[idx+1:])
		} else {
			break
		}
	}
	// Cut trailing [label] pads.
	if idx := strings.IndexByte(f, '['); idx >= 0 {
		f = f[:idx]
	}
	// Filter name ends at '=' (arguments) or '@' (instance name).
	if idx := strings.IndexByte(f, '='); idx >= 0 {
		f = f[:idx]
	}
	if idx := strings.IndexByte(f, '@'); idx >= 0 {
		f = f[:idx]
	}
	return strings.TrimSpace(f)
}

// filterDescs maps common filter names to a short description.
var filterDescs = map[string]string{
	"scale":         "scale (resize the picture)",
	"fps":           "fps (change the frame rate)",
	"crop":          "crop (cut out a rectangular region)",
	"pad":           "pad (add borders around the picture)",
	"palettegen":    "palettegen (build an optimal colour palette)",
	"paletteuse":    "paletteuse (map frames onto the generated palette)",
	"subtitles":     "subtitles (burn subtitles into the video)",
	"ass":           "ass (burn styled ASS/SSA subtitles into the video)",
	"loudnorm":      "loudnorm (normalise perceived audio loudness)",
	"silenceremove": "silenceremove (trim silent stretches of audio)",
	"overlay":       "overlay (composite one video on top of another)",
	"split":         "split (fork the stream into multiple outputs)",
	"format":        "format (convert the pixel format)",
	"transpose":     "transpose (rotate the video 90 degrees)",
	"hflip":         "hflip (flip the video horizontally)",
	"vflip":         "vflip (flip the video vertically)",
}

// flagTable holds the fixed-name flags. Stream-specifier families (-c, -b, -q,
// -filter) are handled in lookup instead.
var flagTable = map[string]flagSpec{
	"-i": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("input file: %s", v)
	}},
	"-crf": {takesValue: true, explain: explainCRF},
	"-preset": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("encoder preset %q: trades encoding speed against compression efficiency", v)
	}},
	"-tune": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("tune the encoder for %q content", v)
	}},
	"-vf":             {takesValue: true, explain: explainFilter},
	"-af":             {takesValue: true, explain: explainFilter},
	"-filter_complex": {takesValue: true, explain: explainFilter},
	"-lc":             {takesValue: true, explain: explainFilter},
	"-ss": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("seek to start position %s before processing", v)
	}},
	"-t": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("limit the output duration to %s", v)
	}},
	"-to": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("stop at end position %s", v)
	}},
	"-r": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("set the frame rate to %s frames per second", v)
	}},
	"-s": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("set the frame size to %s (width x height)", v)
	}},
	"-an": {takesValue: false, explain: func(_, _ string) string {
		return "drop the audio (no audio in the output)"
	}},
	"-vn": {takesValue: false, explain: func(_, _ string) string {
		return "drop the video (no video in the output)"
	}},
	"-sn": {takesValue: false, explain: func(_, _ string) string {
		return "drop the subtitles (no subtitles in the output)"
	}},
	"-dn": {takesValue: false, explain: func(_, _ string) string {
		return "drop data streams (no data in the output)"
	}},
	"-map": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("select stream %s to include in the output", v)
	}},
	"-pix_fmt": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("use the %q pixel format (colour/chroma layout)", v)
	}},
	"-movflags": {takesValue: true, explain: func(_, v string) string {
		if strings.Contains(v, "faststart") {
			return "move the MP4 index to the front (+faststart) so the file streams before it fully downloads"
		}
		return fmt.Sprintf("set MP4/MOV muxer flags: %s", v)
	}},
	"-loop": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("loop setting %s (0 loops forever for GIF/image output)", v)
	}},
	"-ar": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("resample audio to %s Hz", v)
	}},
	"-ac": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("set the number of audio channels to %s", v)
	}},
	"-y": {takesValue: false, explain: func(_, _ string) string {
		return "overwrite the output file if it already exists"
	}},
	"-n": {takesValue: false, explain: func(_, _ string) string {
		return "never overwrite an existing output file"
	}},
	"-shortest": {takesValue: false, explain: func(_, _ string) string {
		return "stop encoding when the shortest input stream ends"
	}},
	"-nostdin": {takesValue: false, explain: func(_, _ string) string {
		return "disable interaction on standard input"
	}},
	"-re": {takesValue: false, explain: func(_, _ string) string {
		return "read the input at its native frame rate (real time)"
	}},
	"-autorotate": {takesValue: false, explain: func(_, _ string) string {
		return "auto-rotate the video according to its rotation metadata"
	}},
	"-noautorotate": {takesValue: false, explain: func(_, _ string) string {
		return "do not auto-rotate the video by its metadata"
	}},
	"-copyts": {takesValue: false, explain: func(_, _ string) string {
		return "copy input timestamps without rebasing them"
	}},
	"-ignore_unknown": {takesValue: false, explain: func(_, _ string) string {
		return "ignore streams of unknown type instead of failing"
	}},
	"-metadata": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("set metadata %s on the output", v)
	}},
	"-threads": {takesValue: true, explain: func(_, v string) string {
		return fmt.Sprintf("use %s threads for processing (0 = auto)", v)
	}},
}
