package formats

import (
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	if _, ok := Lookup("mp4"); !ok {
		t.Error("mp4 should resolve")
	}
	if c, ok := Lookup("m4v"); !ok || c.Ext != "mp4" {
		t.Error("m4v alias should resolve to mp4")
	}
	if _, ok := Lookup(".MKV"); !ok {
		t.Error("lookup should be case/dot tolerant")
	}
	if _, ok := Lookup("xyz"); ok {
		t.Error("unknown container should not resolve")
	}
}

func argsStr(p Plan) string { return strings.Join(p.Args, " ") }

func TestPlanConvertStreamCopy(t *testing.T) {
	mp4, _ := Lookup("mp4")
	p := PlanConvert(mp4, "h264", []string{"aac"}, ModeAuto)
	if !p.StreamCopied || p.ReencodedVideo || p.ReencodedAudio {
		t.Errorf("h264/aac into mp4 should stream-copy: %+v", p)
	}
	s := argsStr(p)
	if !strings.Contains(s, "-c:v copy") || !strings.Contains(s, "-c:a copy") {
		t.Errorf("expected copy codecs: %q", s)
	}
	if !strings.Contains(s, "+faststart") {
		t.Errorf("mp4 should get faststart: %q", s)
	}
}

func TestPlanConvertReencode(t *testing.T) {
	mp4, _ := Lookup("mp4")
	p := PlanConvert(mp4, "vp9", []string{"opus"}, ModeAuto)
	if !p.ReencodedVideo || !p.ReencodedAudio || p.StreamCopied {
		t.Errorf("vp9/opus into mp4 should re-encode: %+v", p)
	}
	s := argsStr(p)
	if !strings.Contains(s, "-c:v libx264") || !strings.Contains(s, "-c:a aac") {
		t.Errorf("expected default codecs: %q", s)
	}
}

func TestPlanConvertMatroskaAny(t *testing.T) {
	mkv, _ := Lookup("mkv")
	p := PlanConvert(mkv, "vp9", []string{"opus"}, ModeAuto)
	if !p.StreamCopied {
		t.Errorf("mkv accepts anything, should copy: %+v", p)
	}
	if !strings.Contains(argsStr(p), "-c:s copy") {
		t.Error("mkv should copy subtitles")
	}
}

func TestPlanConvertMixedAudio(t *testing.T) {
	mp4, _ := Lookup("mp4")
	// One compatible (aac) + one incompatible (pcm) audio track: because a
	// single -c:a applies to all mapped streams, audio must be re-encoded.
	p := PlanConvert(mp4, "h264", []string{"aac", "pcm_s16le"}, ModeAuto)
	if !p.ReencodedAudio {
		t.Errorf("mixed compatible/incompatible audio should re-encode: %+v", p)
	}
	// Two compatible tracks can still be copied.
	p = PlanConvert(mp4, "h264", []string{"aac", "ac3"}, ModeAuto)
	if p.ReencodedAudio {
		t.Errorf("all-compatible audio should copy: %+v", p)
	}
}

func TestPlanConvertModes(t *testing.T) {
	mp4, _ := Lookup("mp4")
	// Force re-encode even for compatible codecs.
	if p := PlanConvert(mp4, "h264", []string{"aac"}, ModeReencode); !p.ReencodedVideo {
		t.Error("ModeReencode should force re-encode")
	}
	// Force copy even for incompatible codecs.
	if p := PlanConvert(mp4, "vp9", []string{"opus"}, ModeCopy); p.ReencodedVideo {
		t.Error("ModeCopy should force copy")
	}
}
