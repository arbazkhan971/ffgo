package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ui"
)

// Subtitle command flags, grouped per subcommand. Names are prefixed to avoid
// clashing with other commands' package-level flag variables.
var (
	subsBurnFile string
	subsBurnOut  string

	subsExtractTrack  int
	subsExtractOut    string
	subsExtractFormat string

	subsConvertTo  string
	subsConvertOut string
)

var subtitlesCmd = &cobra.Command{
	Use:     "subtitles",
	Aliases: []string{"subs"},
	Short:   "Burn, extract and convert subtitle tracks",
	Long: "Work with subtitles the easy way.\n\n" +
		"Hardcode captions into a video, pull an embedded subtitle stream out to a\n" +
		"file, or convert a subtitle file between srt, vtt and ass.",
	Example: "  ffgo subtitles burn movie.mp4 --sub movie.srt\n" +
		"  ffgo subs extract movie.mkv --track 0 -o movie.srt\n" +
		"  ffgo subs convert movie.srt --to vtt",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var subtitlesBurnCmd = &cobra.Command{
	Use:   "burn <input>",
	Short: "Hardcode a subtitle file into the video picture",
	Long: "Burn (hardcode) subtitles from a .srt/.vtt/.ass file directly into the\n" +
		"video frames. The video is re-encoded with libx264; audio is copied.\n" +
		"The result plays captions everywhere, but they can no longer be toggled.",
	Example: "  ffgo subtitles burn movie.mp4 --sub movie.srt\n" +
		"  ffgo subtitles burn talk.mkv --sub talk.ass -o talk_final.mp4",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		if subsBurnFile == "" {
			return fmt.Errorf("a subtitle file is required; pass --sub <file>")
		}
		if err := requireFile(subsBurnFile); err != nil {
			return err
		}

		out := subsBurnOut
		if out == "" {
			out = suffixName(input, "_subbed", "")
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		eng, err := newEngine()
		if err != nil {
			return err
		}
		info, err := eng.Probe(cmd.Context(), input)
		if err != nil {
			return err
		}

		opts := baseRunOptions()
		opts.Args = []string{
			"-i", input,
			"-vf", subtitleFilter(subsBurnFile),
			"-c:v", "libx264", "-crf", "20", "-preset", "medium",
			"-c:a", "copy",
			out,
		}
		opts.Total = info.Duration()
		opts.Label = "Burning subtitles"
		if err := eng.Run(cmd.Context(), opts); err != nil {
			return err
		}
		announceSubtitleOutput(out)
		return nil
	},
}

var subtitlesExtractCmd = &cobra.Command{
	Use:   "extract <input>",
	Short: "Pull an embedded subtitle stream out to a file",
	Long: "Extract a subtitle stream from a container (mkv, mp4, ...) into a\n" +
		"standalone subtitle file. Pick the stream with --track and the output\n" +
		"format with --format (srt, vtt or ass).",
	Example: "  ffgo subtitles extract movie.mkv -o movie.srt\n" +
		"  ffgo subtitles extract movie.mkv --track 1 --format vtt",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		if err := validSubtitleFormat(subsExtractFormat); err != nil {
			return err
		}

		eng, err := newEngine()
		if err != nil {
			return err
		}
		info, err := eng.Probe(cmd.Context(), input)
		if err != nil {
			return err
		}
		subs := info.SubtitleStreams()
		if len(subs) == 0 {
			return fmt.Errorf("no subtitle streams found in %q", input)
		}
		if subsExtractTrack < 0 || subsExtractTrack >= len(subs) {
			return fmt.Errorf("subtitle track %d out of range: %q has %d subtitle stream(s)",
				subsExtractTrack, input, len(subs))
		}

		out := subsExtractOut
		if out == "" {
			out = replaceExt(input, subsExtractFormat)
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		opts := baseRunOptions()
		opts.Args = []string{
			"-i", input,
			"-map", fmt.Sprintf("0:s:%d", subsExtractTrack),
			out,
		}
		opts.Total = info.Duration()
		opts.Label = "Extracting subtitles"
		if err := eng.Run(cmd.Context(), opts); err != nil {
			return err
		}
		announceSubtitleOutput(out)
		return nil
	},
}

var subtitlesConvertCmd = &cobra.Command{
	Use:   "convert <input>",
	Short: "Convert a subtitle file between srt, vtt and ass",
	Long: "Convert a standalone subtitle file to another format with --to.\n" +
		"Supported formats are srt, vtt and ass. The output defaults to the input\n" +
		"name with the new extension.",
	Example: "  ffgo subtitles convert movie.srt --to vtt\n" +
		"  ffgo subtitles convert movie.vtt --to srt -o clean.srt",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		if subsConvertTo == "" {
			return fmt.Errorf("a target format is required; pass --to <srt|vtt|ass>")
		}
		if err := validSubtitleFormat(subsConvertTo); err != nil {
			return err
		}

		out := subsConvertOut
		if out == "" {
			out = replaceExt(input, subsConvertTo)
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		eng, err := newEngine()
		if err != nil {
			return err
		}

		opts := baseRunOptions()
		opts.Args = []string{"-i", input, out}
		opts.Total = 0 // a plain subtitle file has no media duration
		opts.Label = "Converting subtitles"
		if err := eng.Run(cmd.Context(), opts); err != nil {
			return err
		}
		announceSubtitleOutput(out)
		return nil
	},
}

func init() {
	subtitlesBurnCmd.Flags().StringVar(&subsBurnFile, "sub", "", "subtitle file to burn in (.srt, .vtt or .ass)")
	subtitlesBurnCmd.Flags().StringVarP(&subsBurnOut, "output", "o", "", "output path (default <input>_subbed.<ext>)")

	subtitlesExtractCmd.Flags().IntVar(&subsExtractTrack, "track", 0, "subtitle stream index to extract")
	subtitlesExtractCmd.Flags().StringVarP(&subsExtractOut, "output", "o", "", "output path (default <input>.<format>)")
	subtitlesExtractCmd.Flags().StringVar(&subsExtractFormat, "format", "srt", "output format: srt, vtt or ass")

	subtitlesConvertCmd.Flags().StringVar(&subsConvertTo, "to", "", "target format: srt, vtt or ass")
	subtitlesConvertCmd.Flags().StringVarP(&subsConvertOut, "output", "o", "", "output path (default <input>.<to>)")

	subtitlesCmd.AddCommand(subtitlesBurnCmd)
	subtitlesCmd.AddCommand(subtitlesExtractCmd)
	subtitlesCmd.AddCommand(subtitlesConvertCmd)
	rootCmd.AddCommand(subtitlesCmd)
}

// validSubtitleFormat rejects any format ffgo doesn't offer.
func validSubtitleFormat(f string) error {
	switch strings.ToLower(f) {
	case "srt", "vtt", "ass":
		return nil
	default:
		return fmt.Errorf("unsupported subtitle format %q (use srt, vtt or ass)", f)
	}
}

// subtitleFilter builds the -vf value that renders subPath onto the video. It
// uses the ass filter for .ass sources and the subtitles filter otherwise,
// escaping the path so filtergraph metacharacters survive.
func subtitleFilter(subPath string) string {
	esc := escapeSubtitleFilterPath(subPath)
	if strings.EqualFold(filepath.Ext(subPath), ".ass") {
		return "ass=filename=" + esc
	}
	return "subtitles=filename=" + esc
}

// escapeSubtitleFilterPath makes a filesystem path safe to embed in an ffmpeg
// filtergraph option value. FFmpeg parses such values in two layers: the
// filtergraph splits on , ; [ ] and quotes, then the subtitles/ass filter
// splits its own options on ':'. We therefore single-quote the whole value
// (so , ; [ ] and spaces are taken literally by the graph) and additionally
// backslash-escape ':' for the filter's option parser. Windows separators are
// normalised to forward slashes, which ffmpeg accepts.
func escapeSubtitleFilterPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = strings.ReplaceAll(p, `'`, `'\''`) // break out of quoting for a literal quote
	p = strings.ReplaceAll(p, `:`, `\:`)   // escape colon for the filter option layer
	return "'" + p + "'"
}

// announceSubtitleOutput prints a success line with the output size, unless the
// run was a dry run (nothing was written).
func announceSubtitleOutput(out string) {
	if globals.DryRun {
		return
	}
	if info, err := os.Stat(out); err == nil {
		ui.Successf("wrote %s  %s", ui.Bold(out), ui.Dim(ui.Bytes(info.Size())))
		return
	}
	ui.Successf("wrote %s", ui.Bold(out))
}
