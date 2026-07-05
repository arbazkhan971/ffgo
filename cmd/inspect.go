package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var inspectJSON bool

var inspectCmd = &cobra.Command{
	Use:     "inspect <file>",
	Aliases: []string{"info", "i"},
	Short:   "Show codecs, resolution, bitrate, tracks and smart recommendations",
	Long: "Inspect a media file and print a readable summary of its streams,\n" +
		"container metadata and a few practical recommendations.\n\n" +
		"Use --json for the full, machine-readable ffprobe output.",
	Example: "  ffgo inspect movie.mkv\n  ffgo inspect clip.mp4 --json | jq .format",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		eng, err := newEngine()
		if err != nil {
			return err
		}
		prober, err := eng.Prober()
		if err != nil {
			return err
		}

		if inspectJSON {
			raw, err := prober.RawJSON(cmd.Context(), input)
			if err != nil {
				return err
			}
			ui.Printf("%s\n", raw)
			return nil
		}

		info, err := prober.Probe(cmd.Context(), input)
		if err != nil {
			return err
		}
		renderInspect(input, info)
		return nil
	},
}

func init() {
	inspectCmd.Flags().BoolVar(&inspectJSON, "json", false, "output raw ffprobe JSON")
	rootCmd.AddCommand(inspectCmd)
}

// renderInspect writes the human-readable report to stdout.
func renderInspect(input string, r *ffprobe.Result) {
	out := ui.Out
	fmt.Fprintf(out, "\n%s  %s\n", ui.Glyph("🎬", "#"), ui.Bold(input))

	// Container summary line.
	container := r.Format.FormatLongName
	if container == "" {
		container = r.Format.FormatName
	}
	kv := func(k, v string) { fmt.Fprintf(out, "  %-9s %s\n", ui.Dim(k), v) }
	kv("format", container)
	kv("duration", ui.Cyan(ui.Duration(r.Duration())))
	if sz := r.Size(); sz > 0 {
		kv("size", ui.Bytes(sz))
	}
	if br := r.Bitrate(); br > 0 {
		kv("bitrate", ui.Bitrate(br))
	}

	// Video streams (skipping attached cover-art, which is not real video).
	for _, v := range r.VideoStreams() {
		if v.Disposition.AttachedPic == 1 {
			continue
		}
		fmt.Fprintf(out, "\n%s %s\n", ui.Glyph("📹", ">"), ui.Bold("Video"))
		codec := v.CodecName
		if v.Profile != "" {
			codec += " (" + v.Profile + ")"
		}
		kv("codec", codec)
		if res := v.Resolution(); res != "" {
			line := ui.Cyan(res)
			if v.DisplayAspectRatio != "" && v.DisplayAspectRatio != "0:1" {
				line += ui.Dim("  " + v.DisplayAspectRatio)
			}
			kv("resolution", line)
		}
		if fps := v.FPS(); fps > 0 {
			kv("fps", fmt.Sprintf("%.3g", fps))
		}
		if v.PixFmt != "" {
			kv("pixels", v.PixFmt)
		}
		if v.IsHDR() {
			kv("dynamic", ui.Yellow("HDR")+ui.Dim("  "+v.ColorTransfer))
		}
		if br := v.BitrateBPS(); br > 0 {
			kv("bitrate", ui.Bitrate(br))
		}
	}

	// Audio streams.
	for i, a := range r.AudioStreams() {
		title := "Audio"
		if len(r.AudioStreams()) > 1 {
			title = fmt.Sprintf("Audio #%d", i+1)
		}
		fmt.Fprintf(out, "\n%s %s\n", ui.Glyph("🔊", ")"), ui.Bold(title))
		kv("codec", a.CodecName)
		layout := a.ChannelLayout
		if layout == "" {
			layout = fmt.Sprintf("%d ch", a.Channels)
		}
		kv("channels", layout)
		if a.SampleRateHz() > 0 {
			kv("sample", fmt.Sprintf("%d Hz", a.SampleRateHz()))
		}
		if br := a.BitrateBPS(); br > 0 {
			kv("bitrate", ui.Bitrate(br))
		}
		if a.Language() != "und" {
			kv("language", a.Language())
		}
		if t := a.Title(); t != "" {
			kv("title", t)
		}
	}

	// Subtitles.
	if subs := r.SubtitleStreams(); len(subs) > 0 {
		fmt.Fprintf(out, "\n%s %s\n", ui.Glyph("💬", "="), ui.Bold("Subtitles"))
		for _, s := range subs {
			desc := s.CodecName + "  " + ui.Dim(s.Language())
			if t := s.Title(); t != "" {
				desc += "  " + t
			}
			if s.Disposition.Forced == 1 {
				desc += ui.Yellow("  forced")
			}
			fmt.Fprintf(out, "  %s %s\n", ui.Cyan(ui.Glyph("•", "-")), desc)
		}
	}

	// Chapters + notable metadata tags.
	if len(r.Chapters) > 0 {
		fmt.Fprintf(out, "\n  %s %d chapters\n", ui.Dim("chapters"), len(r.Chapters))
	}
	if meta := notableTags(r.Format.Tags); len(meta) > 0 {
		fmt.Fprintf(out, "\n%s %s\n", ui.Glyph("🏷", "@"), ui.Bold("Metadata"))
		for _, kvp := range meta {
			fmt.Fprintf(out, "  %-11s %s\n", ui.Dim(kvp[0]), kvp[1])
		}
	}

	// Recommendations.
	if recs := recommendations(r); len(recs) > 0 {
		fmt.Fprintf(out, "\n%s %s\n", ui.Glyph("💡", "*"), ui.Bold("Recommendations"))
		for _, rec := range recs {
			fmt.Fprintf(out, "  %s %s\n", ui.Yellow(ui.Glyph("→", "->")), rec)
		}
	}
	fmt.Fprintln(out)
}

// notableTags picks a few human-interesting container tags in a stable order.
func notableTags(tags map[string]string) [][2]string {
	order := []string{"title", "artist", "album", "comment", "encoder", "creation_time"}
	var out [][2]string
	for _, k := range order {
		for tk, tv := range tags {
			if strings.EqualFold(tk, k) && strings.TrimSpace(tv) != "" {
				out = append(out, [2]string{k, tv})
				break
			}
		}
	}
	return out
}

// recommendations produces practical, actionable suggestions.
func recommendations(r *ffprobe.Result) []string {
	var recs []string
	v := r.VideoStream()
	sizeMB := float64(r.Size()) / (1 << 20)

	if v != nil && sizeMB > 50 {
		recs = append(recs, fmt.Sprintf(
			"Large file (%s) — shrink it with %s",
			ui.Bytes(r.Size()), ui.Cyan("ffgo compress "+quoteBase(r.Format.Filename)+" --target 25mb")))
	}
	if v != nil && v.CodecName != "" && v.CodecName != "h264" && v.CodecName != "hevc" &&
		!strings.EqualFold(filepath.Ext(r.Format.Filename), ".mp4") &&
		strings.Contains(r.Format.FormatName, "mov") {
		recs = append(recs, fmt.Sprintf(
			"%s isn't the most compatible codec — %s for wide playback",
			v.CodecName, ui.Cyan("ffgo convert "+quoteBase(r.Format.Filename)+" --to mp4")))
	}
	if r.IsHDR() {
		recs = append(recs, "HDR content — converting to SDR will need tone-mapping")
	}
	if v != nil && v.FPS() > 60 {
		recs = append(recs, fmt.Sprintf("High frame rate (%.0f fps) — halve it if you need smaller output", v.FPS()))
	}
	if v != nil && !r.HasAudio() {
		recs = append(recs, "No audio track — this is a silent video")
	}
	if len(r.AudioStreams()) > 1 {
		recs = append(recs, fmt.Sprintf("%d audio tracks — select one with a stream map when converting", len(r.AudioStreams())))
	}
	return recs
}

// quoteBase returns just the base filename for use in suggestion snippets.
func quoteBase(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	if strings.ContainsAny(path, " ") {
		return `"` + path + `"`
	}
	return path
}
