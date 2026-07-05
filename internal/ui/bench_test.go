package ui

import (
	"io"
	"testing"
)

func BenchmarkBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Bytes(int64(i) * 991)
	}
}

func BenchmarkParseTimecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseTimecode("01:23:45.678")
	}
}

func BenchmarkTableRender(b *testing.B) {
	t := NewTable("Codec", "Res", "Bitrate").RightAlign(2)
	for i := 0; i < 20; i++ {
		t.Row("h264", "1920x1080", "12.8 Mbps")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t2 := NewTable("Codec", "Res", "Bitrate").RightAlign(2)
		for j := 0; j < 20; j++ {
			t2.Row("h264", "1920x1080", "12.8 Mbps")
		}
		t2.Render(io.Discard)
	}
}
