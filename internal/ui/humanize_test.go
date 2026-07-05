package ui

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	cases := map[int64]string{
		0:               "0 B",
		500:             "500 B",
		1536:            "1.5 KB",
		5 * 1024 * 1024: "5.0 MB",
		150 * 1024:      "150 KB",
	}
	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"25mb", 25 * 1 << 20},
		{"1.5 GB", 1.5 * (1 << 30)},
		{"900k", 900 * 1 << 10},
		{"100", 100},
		{"2G", 2 << 30},
		{"512KB", 512 << 10},
	}
	for _, c := range cases {
		got, err := ParseBytes(c.in)
		if err != nil {
			t.Errorf("ParseBytes(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	if _, err := ParseBytes("banana"); err == nil {
		t.Error("ParseBytes(banana) should error")
	}
}

func TestDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{6 * time.Second, "0:06"},
		{90 * time.Second, "1:30"},
		{3661 * time.Second, "1:01:01"},
		{1500 * time.Millisecond, "0:01.500"},
	}
	for _, c := range cases {
		if got := Duration(c.d); got != c.want {
			t.Errorf("Duration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestParseTimecode(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"00:01:20", 80 * time.Second},
		{"1:20", 80 * time.Second},
		{"90", 90 * time.Second},
		{"10s", 10 * time.Second},
		{"1m30s", 90 * time.Second},
		{"1:20.5", 80500 * time.Millisecond},
	}
	for _, c := range cases {
		got, err := ParseTimecode(c.in)
		if err != nil {
			t.Errorf("ParseTimecode(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseTimecode(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	if _, err := ParseTimecode(""); err == nil {
		t.Error("empty timecode should error")
	}
}

func TestClock(t *testing.T) {
	if got := Clock(90 * time.Second); got != "00:01:30.000" {
		t.Errorf("Clock = %q", got)
	}
	if got := Clock(3661500 * time.Millisecond); got != "01:01:01.500" {
		t.Errorf("Clock = %q", got)
	}
}

func TestBitrate(t *testing.T) {
	cases := map[int64]string{
		3_100_000: "3.1 Mbps",
		128_000:   "128 kbps",
		500:       "500 bps",
	}
	for in, want := range cases {
		if got := Bitrate(in); got != want {
			t.Errorf("Bitrate(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	if got := Count(1234567); got != "1,234,567" {
		t.Errorf("Count = %q", got)
	}
	if got := Count(-1000); got != "-1,000" {
		t.Errorf("Count = %q", got)
	}
}
