package presets

import (
	"strings"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	if _, ok := Lookup("whatsapp"); !ok {
		t.Error("whatsapp preset should exist")
	}
	if _, ok := Lookup("WhatsApp"); !ok {
		t.Error("lookup should be case-insensitive")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown preset should not resolve")
	}
	if len(Names()) == 0 || len(Describe()) != len(Names()) {
		t.Error("Names/Describe mismatch")
	}
}

func TestQualityEncoding(t *testing.T) {
	high, err := QualityEncoding(QualityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if high.CRF != 20 {
		t.Errorf("high CRF = %d, want 20", high.CRF)
	}
	if _, err := QualityEncoding("ultra"); err == nil {
		t.Error("unknown quality should error")
	}
}

func TestSolveBitrate(t *testing.T) {
	// 25 MB over 60s with 128k audio should yield a sane positive bitrate.
	kbps, err := SolveBitrate(25<<20, 60*time.Second, 128)
	if err != nil {
		t.Fatal(err)
	}
	if kbps < 2000 || kbps > 3400 {
		t.Errorf("kbps = %d, expected ~2900", kbps)
	}
	// Impossible target: tiny size, long duration.
	if _, err := SolveBitrate(1<<10, 3600*time.Second, 128); err == nil {
		t.Error("infeasible target should error")
	}
	// Zero duration is an error (cannot target size).
	if _, err := SolveBitrate(25<<20, 0, 128); err == nil {
		t.Error("zero duration should error")
	}
}

func TestEncodingArgs(t *testing.T) {
	e := Encoding{VideoCodec: "libx264", CRF: 23, Preset: "medium",
		MaxWidth: 848, FPS: 15, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128}
	got := strings.Join(e.OutputArgs(), " ")
	for _, want := range []string{"-c:v libx264", "-crf 23", "-preset medium", "-pix_fmt yuv420p", "-c:a aac", "-b:a 128k"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
	if !strings.Contains(got, "fps=15") || !strings.Contains(got, "scale='min(848,iw)':-2") {
		t.Errorf("filter chain missing in %q", got)
	}

	// Bitrate (size) mode should emit -b:v and rate control, not -crf.
	sz := Encoding{VideoCodec: "libx264", VideoBitrateK: 2000, MaxrateK: 2000, BufsizeK: 4000, AudioCodec: "aac", AudioBitrateK: 128}
	got = strings.Join(sz.OutputArgs(), " ")
	if !strings.Contains(got, "-b:v 2000k") || strings.Contains(got, "-crf") {
		t.Errorf("size mode args wrong: %q", got)
	}

	// AudioCodec none drops audio.
	none := Encoding{VideoCodec: "libx264", CRF: 20, AudioCodec: "none"}
	if !strings.Contains(strings.Join(none.AudioArgs(), " "), "-an") {
		t.Error("expected -an for audio none")
	}
}

func BenchmarkSolveBitrate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SolveBitrate(25<<20, 90*time.Second, 128)
	}
}

func BenchmarkEncodingArgs(b *testing.B) {
	e := Encoding{VideoCodec: "libx264", CRF: 23, Preset: "medium",
		MaxWidth: 1280, FPS: 30, PixFmt: "yuv420p", AudioCodec: "aac", AudioBitrateK: 128}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.OutputArgs()
	}
}
