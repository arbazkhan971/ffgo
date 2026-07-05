package ffprobe

import (
	"math"
	"testing"
	"time"
)

func TestParseRatio(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"30000/1001", 29.97002997},
		{"30/1", 30},
		{"0/0", 0},
		{"", 0},
		{"25", 25},
	}
	for _, c := range cases {
		if got := parseRatio(c.in); math.Abs(got-c.want) > 0.001 {
			t.Errorf("parseRatio(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestResultAccessors(t *testing.T) {
	r := &Result{
		Format: Format{Duration: "6.5", Size: "1048576", BitRate: "1290000"},
		Streams: []Stream{
			{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, AvgFrameRate: "30000/1001"},
			{CodecType: "audio", CodecName: "aac", Channels: 2, SampleRate: "48000"},
			{CodecType: "subtitle", CodecName: "subrip"},
		},
	}
	if d := r.Duration(); d != 6500*time.Millisecond {
		t.Errorf("Duration = %v", d)
	}
	if r.Size() != 1<<20 {
		t.Errorf("Size = %d", r.Size())
	}
	if r.Bitrate() != 1290000 {
		t.Errorf("Bitrate = %d", r.Bitrate())
	}
	v := r.VideoStream()
	if v == nil || v.Resolution() != "1920x1080" {
		t.Fatalf("VideoStream/Resolution wrong: %+v", v)
	}
	if math.Abs(v.FPS()-29.97) > 0.01 {
		t.Errorf("FPS = %v", v.FPS())
	}
	if !r.HasAudio() || len(r.AudioStreams()) != 1 {
		t.Error("audio stream not detected")
	}
	if len(r.SubtitleStreams()) != 1 {
		t.Error("subtitle stream not detected")
	}
	if r.AudioStreams()[0].SampleRateHz() != 48000 {
		t.Error("sample rate parse")
	}
}

func TestBitrateFallback(t *testing.T) {
	// No container bitrate → compute from size and duration.
	r := &Result{Format: Format{Duration: "10", Size: "1250000"}}
	// 1_250_000 bytes * 8 / 10s = 1_000_000 bps
	if got := r.Bitrate(); got != 1_000_000 {
		t.Errorf("computed bitrate = %d, want 1000000", got)
	}
}

func TestIsHDR(t *testing.T) {
	hdr := Stream{CodecType: "video", ColorTransfer: "smpte2084"}
	if !hdr.IsHDR() {
		t.Error("smpte2084 should be HDR")
	}
	hlg := Stream{CodecType: "video", ColorPrimaries: "bt2020"}
	if !hlg.IsHDR() {
		t.Error("bt2020 primaries should be HDR")
	}
	sdr := Stream{CodecType: "video", ColorTransfer: "bt709"}
	if sdr.IsHDR() {
		t.Error("bt709 should not be HDR")
	}
}

func TestVideoStreamSkipsAttachedPic(t *testing.T) {
	r := &Result{Streams: []Stream{
		{CodecType: "video", CodecName: "mjpeg", Disposition: Disposition{AttachedPic: 1}},
		{CodecType: "video", CodecName: "h264"},
	}}
	if v := r.VideoStream(); v == nil || v.CodecName != "h264" {
		t.Errorf("should skip attached pic, got %+v", v)
	}
}
