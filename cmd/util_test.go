package cmd

import (
	"path/filepath"
	"testing"
)

func TestReplaceExt(t *testing.T) {
	cases := []struct{ in, ext, want string }{
		{"clip.mov", "mp4", "clip.mp4"},
		{"clip.mov", ".mp4", "clip.mp4"},
		{"a.b.c.mkv", "webm", "a.b.c.webm"},
		{"noext", "gif", "noext.gif"},
		{filepath.Join("dir", "clip.mov"), "mp4", filepath.Join("dir", "clip.mp4")},
	}
	for _, c := range cases {
		if got := replaceExt(c.in, c.ext); got != c.want {
			t.Errorf("replaceExt(%q,%q) = %q, want %q", c.in, c.ext, got, c.want)
		}
	}
}

func TestSuffixName(t *testing.T) {
	cases := []struct{ in, suffix, ext, want string }{
		{"clip.mp4", "_small", "", "clip_small.mp4"},
		{"clip.mp4", "_x", "webm", "clip_x.webm"},
		{filepath.Join("d", "clip.mov"), "_c", "", filepath.Join("d", "clip_c.mov")},
	}
	for _, c := range cases {
		if got := suffixName(c.in, c.suffix, c.ext); got != c.want {
			t.Errorf("suffixName(%q,%q,%q) = %q, want %q", c.in, c.suffix, c.ext, got, c.want)
		}
	}
}
