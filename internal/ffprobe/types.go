// Package ffprobe runs ffprobe and decodes its JSON output into typed Go
// structs with convenience accessors (duration, frame rate, HDR detection,
// stream selection) used throughout ffgo.
package ffprobe

import (
	"strconv"
	"strings"
	"time"
)

// Result is the decoded output of `ffprobe -show_format -show_streams
// -show_chapters -of json`.
type Result struct {
	Streams  []Stream  `json:"streams"`
	Format   Format    `json:"format"`
	Chapters []Chapter `json:"chapters"`
}

// Format holds container-level metadata.
type Format struct {
	Filename       string            `json:"filename"`
	NBStreams      int               `json:"nb_streams"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

// Stream is a single media stream (video, audio, subtitle, data, attachment).
type Stream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecLongName string `json:"codec_long_name"`
	Profile       string `json:"profile"`
	CodecType     string `json:"codec_type"`
	CodecTag      string `json:"codec_tag_string"`

	// Video
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	CodedWidth         int    `json:"coded_width"`
	CodedHeight        int    `json:"coded_height"`
	SampleAspectRatio  string `json:"sample_aspect_ratio"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	PixFmt             string `json:"pix_fmt"`
	Level              int    `json:"level"`
	ColorRange         string `json:"color_range"`
	ColorSpace         string `json:"color_space"`
	ColorTransfer      string `json:"color_transfer"`
	ColorPrimaries     string `json:"color_primaries"`
	FieldOrder         string `json:"field_order"`
	RFrameRate         string `json:"r_frame_rate"`
	AvgFrameRate       string `json:"avg_frame_rate"`

	// Audio
	SampleFmt     string `json:"sample_fmt"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	BitsPerSample int    `json:"bits_per_sample"`

	// Common
	TimeBase    string            `json:"time_base"`
	StartTime   string            `json:"start_time"`
	Duration    string            `json:"duration"`
	BitRate     string            `json:"bit_rate"`
	NBFrames    string            `json:"nb_frames"`
	Disposition Disposition       `json:"disposition"`
	Tags        map[string]string `json:"tags"`
}

// Disposition flags describe a stream's role (default track, forced subs, etc).
type Disposition struct {
	Default         int `json:"default"`
	Forced          int `json:"forced"`
	AttachedPic     int `json:"attached_pic"`
	Comment         int `json:"comment"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
}

// Chapter is a container chapter marker.
type Chapter struct {
	ID        int64             `json:"id"`
	TimeBase  string            `json:"time_base"`
	Start     int64             `json:"start"`
	StartTime string            `json:"start_time"`
	End       int64             `json:"end"`
	EndTime   string            `json:"end_time"`
	Tags      map[string]string `json:"tags"`
}

// ---- Result accessors -------------------------------------------------------

// Duration returns the media duration, preferring the container value and
// falling back to the longest stream duration.
func (r *Result) Duration() time.Duration {
	if d := parseSeconds(r.Format.Duration); d > 0 {
		return d
	}
	var max time.Duration
	for i := range r.Streams {
		if d := parseSeconds(r.Streams[i].Duration); d > max {
			max = d
		}
	}
	return max
}

// Size returns the file size in bytes (0 if unknown).
func (r *Result) Size() int64 { return parseInt(r.Format.Size) }

// Bitrate returns the overall bitrate in bits/second, computing it from size
// and duration when the container omits it.
func (r *Result) Bitrate() int64 {
	if b := parseInt(r.Format.BitRate); b > 0 {
		return b
	}
	if d := r.Duration(); d > 0 {
		if sz := r.Size(); sz > 0 {
			return int64(float64(sz*8) / d.Seconds())
		}
	}
	return 0
}

// VideoStream returns the primary video stream, skipping attached cover art.
// Returns nil when there is no real video track.
func (r *Result) VideoStream() *Stream {
	for i := range r.Streams {
		s := &r.Streams[i]
		if s.CodecType == "video" && s.Disposition.AttachedPic == 0 {
			return s
		}
	}
	return nil
}

// Streams of a given codec type.
func (r *Result) streamsOfType(t string) []Stream {
	var out []Stream
	for i := range r.Streams {
		if r.Streams[i].CodecType == t {
			out = append(out, r.Streams[i])
		}
	}
	return out
}

func (r *Result) VideoStreams() []Stream    { return r.streamsOfType("video") }
func (r *Result) AudioStreams() []Stream    { return r.streamsOfType("audio") }
func (r *Result) SubtitleStreams() []Stream { return r.streamsOfType("subtitle") }

// HasAudio reports whether any audio stream is present.
func (r *Result) HasAudio() bool { return len(r.AudioStreams()) > 0 }

// IsHDR reports whether the primary video stream carries HDR signaling.
func (r *Result) IsHDR() bool {
	if v := r.VideoStream(); v != nil {
		return v.IsHDR()
	}
	return false
}

// Title returns the container title tag if present.
func (r *Result) Title() string { return tag(r.Format.Tags, "title") }

// ---- Stream accessors -------------------------------------------------------

// FPS returns the stream's frame rate, preferring avg_frame_rate.
func (s *Stream) FPS() float64 {
	if f := parseRatio(s.AvgFrameRate); f > 0 {
		return f
	}
	return parseRatio(s.RFrameRate)
}

// Resolution returns "WxH" or "" when unknown.
func (s *Stream) Resolution() string {
	if s.Width == 0 || s.Height == 0 {
		return ""
	}
	return strconv.Itoa(s.Width) + "x" + strconv.Itoa(s.Height)
}

// BitrateBPS returns the stream bitrate in bits/second (0 if unknown).
func (s *Stream) BitrateBPS() int64 { return parseInt(s.BitRate) }

// Language returns the ISO language tag, or "und" when unset.
func (s *Stream) Language() string {
	if l := tag(s.Tags, "language"); l != "" {
		return l
	}
	return "und"
}

// Title returns the stream title tag.
func (s *Stream) Title() string { return tag(s.Tags, "title") }

// IsDefault reports whether the stream is flagged as a default track.
func (s *Stream) IsDefault() bool { return s.Disposition.Default == 1 }

// IsHDR reports whether the video stream signals HDR via its transfer
// characteristics (PQ / HLG) or BT.2020 primaries.
func (s *Stream) IsHDR() bool {
	switch strings.ToLower(s.ColorTransfer) {
	case "smpte2084", "arib-std-b67", "smpte428", "bt2020-10", "bt2020-12":
		return true
	}
	return strings.Contains(strings.ToLower(s.ColorPrimaries), "bt2020")
}

// SampleRateHz returns the audio sample rate in Hz.
func (s *Stream) SampleRateHz() int { return int(parseInt(s.SampleRate)) }

// ---- parsing helpers --------------------------------------------------------

func parseSeconds(s string) time.Duration {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

func parseInt(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseRatio parses ffprobe rationals like "30000/1001" into a float.
func parseRatio(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, err1 := strconv.ParseFloat(num, 64)
		d, err2 := strconv.ParseFloat(den, 64)
		if err1 == nil && err2 == nil && d != 0 {
			return n / d
		}
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func tag(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	// ffprobe tag keys can vary in case across containers.
	if v, ok := m[key]; ok {
		return v
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
