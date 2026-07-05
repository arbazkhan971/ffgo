package cmd

import (
	"strings"
	"testing"
)

func TestAudioCodecForExt(t *testing.T) {
	cases := map[string]string{
		"song.mp3":  "libmp3lame",
		"a.flac":    "flac",
		"a.wav":     "pcm_s16le",
		"a.opus":    "libopus",
		"a.webm":    "libopus",
		"a.ogg":     "libvorbis",
		"movie.mkv": "aac",
		"clip.mp4":  "aac",
	}
	for path, want := range cases {
		got := strings.Join(audioCodecForExt(path), " ")
		if !strings.Contains(got, want) {
			t.Errorf("audioCodecForExt(%q) = %q, want codec %q", path, got, want)
		}
	}
}

func TestAudioCopyExt(t *testing.T) {
	cases := map[string]string{
		"aac": "m4a", "mp3": "mp3", "opus": "opus", "vorbis": "ogg",
		"flac": "flac", "ac3": "ac3", "pcm_s16le": "wav",
		"weirdcodec": "mka", // unknown -> Matroska audio
	}
	for codec, want := range cases {
		if got := audioCopyExt(codec); got != want {
			t.Errorf("audioCopyExt(%q) = %q, want %q", codec, got, want)
		}
	}
}
