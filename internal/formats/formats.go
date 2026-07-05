// Package formats holds the container registry and the copy-vs-reencode logic
// used by convert and batch. It decides, per stream, whether ffmpeg can simply
// remux (fast, lossless) or must transcode to make the output valid.
package formats

import (
	"sort"
	"strings"
)

// Container describes an output container and the codecs to fall back to when
// a source stream must be re-encoded for it.
type Container struct {
	Ext            string
	Description    string
	Faststart      bool     // add +faststart (progressive MP4/MOV)
	DefaultVideo   string   // codec to use when re-encoding video
	DefaultAudio   string   // codec to use when re-encoding audio
	VideoCompat    []string // source codecs that can be stream-copied
	AudioCompat    []string // source codecs that can be stream-copied
	AnyVideoCompat bool     // container accepts any video codec (e.g. mkv)
	AnyAudioCompat bool
}

var registry = map[string]Container{
	"mp4": {
		Ext: "mp4", Description: "MPEG-4 / H.264 — the most compatible option",
		Faststart: true, DefaultVideo: "libx264", DefaultAudio: "aac",
		VideoCompat: []string{"h264", "hevc", "mpeg4", "av1"},
		AudioCompat: []string{"aac", "mp3", "ac3", "alac", "eac3"},
	},
	"mov": {
		Ext: "mov", Description: "QuickTime — Apple-friendly",
		Faststart: true, DefaultVideo: "libx264", DefaultAudio: "aac",
		VideoCompat: []string{"h264", "hevc", "prores", "mpeg4"},
		AudioCompat: []string{"aac", "alac", "pcm_s16le", "ac3"},
	},
	"mkv": {
		Ext: "mkv", Description: "Matroska — accepts virtually any codec",
		DefaultVideo: "libx264", DefaultAudio: "aac",
		AnyVideoCompat: true, AnyAudioCompat: true,
	},
	"webm": {
		Ext: "webm", Description: "WebM — VP9/Opus for the web",
		DefaultVideo: "libvpx-vp9", DefaultAudio: "libopus",
		VideoCompat: []string{"vp8", "vp9", "av1"},
		AudioCompat: []string{"opus", "vorbis"},
	},
	"avi": {
		Ext: "avi", Description: "AVI — legacy container",
		DefaultVideo: "mpeg4", DefaultAudio: "libmp3lame",
		VideoCompat: []string{"mpeg4", "mjpeg", "h264"},
		AudioCompat: []string{"mp3", "ac3", "pcm_s16le"},
	},
}

// aliases maps common spellings to registry keys.
var aliases = map[string]string{
	"m4v": "mp4", "qt": "mov", "matroska": "mkv",
}

// Lookup resolves a container by name/alias (case-insensitive, dot-tolerant).
func Lookup(name string) (Container, bool) {
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "."))
	if a, ok := aliases[key]; ok {
		key = a
	}
	c, ok := registry[key]
	return c, ok
}

// Names returns the known container names, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// videoQualityArgs returns encoder-appropriate constant-quality arguments.
// x264/x265 use -crf plus a -preset; libvpx-vp9 needs -crf with -b:v 0 and
// -deadline/-cpu-used (it has no -preset); mpeg4 uses -q:v.
func videoQualityArgs(codec string) []string {
	switch codec {
	case "libvpx-vp9", "libvpx":
		return []string{"-crf", "31", "-b:v", "0", "-deadline", "good", "-cpu-used", "2", "-row-mt", "1"}
	case "mpeg4":
		return []string{"-q:v", "4"}
	default: // libx264, libx265, ...
		return []string{"-crf", "20", "-preset", "medium"}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// videoCopyable reports whether srcCodec can be stream-copied into c.
func (c Container) videoCopyable(srcCodec string) bool {
	return c.AnyVideoCompat || contains(c.VideoCompat, srcCodec)
}

func (c Container) audioCopyable(srcCodec string) bool {
	return c.AnyAudioCompat || contains(c.AudioCompat, srcCodec)
}

// allAudioCopyable reports whether every audio codec can be stream-copied into
// c. It is false for an empty list (nothing to copy).
func (c Container) allAudioCopyable(codecs []string) bool {
	if len(codecs) == 0 {
		return false
	}
	for _, cc := range codecs {
		if !c.audioCopyable(cc) {
			return false
		}
	}
	return true
}

// Mode selects how aggressively convert transcodes.
type Mode int

const (
	// ModeAuto stream-copies compatible streams and re-encodes the rest.
	ModeAuto Mode = iota
	// ModeCopy forces stream copy (fast, may produce an invalid file if the
	// source codecs are incompatible — ffmpeg will error, which we surface).
	ModeCopy
	// ModeReencode always re-encodes with the container defaults.
	ModeReencode
)

// Plan is the outcome of deciding how to convert into a container.
type Plan struct {
	Container      Container
	Args           []string // output args (after input, before output path)
	ReencodedVideo bool
	ReencodedAudio bool
	StreamCopied   bool
}

// PlanConvert builds the ffmpeg output arguments to place the given source
// codecs into the target container, choosing copy or re-encode per stream.
//
// srcVideoCodec may be empty when the source lacks video. srcAudioCodecs lists
// every audio stream's codec; because the plan maps all streams through with a
// single -c:a directive, audio is stream-copied only when *all* audio streams
// are compatible with the container.
func PlanConvert(target Container, srcVideoCodec string, srcAudioCodecs []string, mode Mode) Plan {
	p := Plan{Container: target}
	// Map every input stream through so multi-track files are preserved.
	p.Args = append(p.Args, "-map", "0")

	// Video.
	switch {
	case srcVideoCodec == "":
		// no video stream; nothing to do
	case mode == ModeCopy || (mode == ModeAuto && target.videoCopyable(srcVideoCodec)):
		p.Args = append(p.Args, "-c:v", "copy")
	default:
		p.Args = append(p.Args, "-c:v", target.DefaultVideo, "-pix_fmt", "yuv420p")
		p.Args = append(p.Args, videoQualityArgs(target.DefaultVideo)...)
		p.ReencodedVideo = true
	}

	// Audio.
	switch {
	case len(srcAudioCodecs) == 0:
	case mode == ModeCopy || (mode == ModeAuto && target.allAudioCopyable(srcAudioCodecs)):
		p.Args = append(p.Args, "-c:a", "copy")
	default:
		p.Args = append(p.Args, "-c:a", target.DefaultAudio, "-b:a", "192k")
		p.ReencodedAudio = true
	}

	// Subtitles/data: copy through where the container allows it.
	if target.AnyVideoCompat { // mkv-like: keep everything
		p.Args = append(p.Args, "-c:s", "copy")
	} else {
		// Drop subtitle/data streams that many containers reject on copy.
		p.Args = append(p.Args, "-sn", "-dn")
	}

	if target.Faststart {
		p.Args = append(p.Args, "-movflags", "+faststart")
	}
	p.StreamCopied = !p.ReencodedVideo && !p.ReencodedAudio
	return p
}
