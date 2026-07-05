package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var trimOpts struct {
	from     string
	to       string
	duration string
	output   string
	reencode bool
}

var trimCmd = &cobra.Command{
	Use:     "trim <input>",
	Aliases: []string{"cut"},
	Short:   "Cut a section out of a video or audio file",
	Long: "Trim extracts the span between --from and --to (or --from for --duration)\n" +
		"and writes it to a new file.\n\n" +
		"By default the cut is lossless (stream copy, instant) but snaps to the\n" +
		"nearest keyframe, so the start may shift slightly. Pass --reencode for a\n" +
		"frame-accurate cut that re-encodes the span.",
	Example: "  ffgo trim in.mp4 --from 00:01:20 --to 00:03:00\n" +
		"  ffgo trim in.mp4 --from 10s --duration 30s\n" +
		"  ffgo trim in.mp4 --from 5s --to 12s --reencode -o clip.mp4",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}

		// Start offset (default 0).
		from, err := ui.ParseTimecode(trimOpts.from)
		if err != nil {
			return fmt.Errorf("invalid --from %q: %w", trimOpts.from, err)
		}
		if from < 0 {
			return fmt.Errorf("--from must be >= 0")
		}

		// Clip duration: --duration wins, else derive from --to.
		var dur time.Duration
		switch {
		case trimOpts.duration != "":
			dur, err = ui.ParseTimecode(trimOpts.duration)
			if err != nil {
				return fmt.Errorf("invalid --duration %q: %w", trimOpts.duration, err)
			}
			if dur <= 0 {
				return fmt.Errorf("--duration must be greater than zero")
			}
		case trimOpts.to != "":
			to, err := ui.ParseTimecode(trimOpts.to)
			if err != nil {
				return fmt.Errorf("invalid --to %q: %w", trimOpts.to, err)
			}
			if to <= from {
				return fmt.Errorf("--to (%s) must be after --from (%s)", ui.Clock(to), ui.Clock(from))
			}
			dur = to - from
		default:
			return fmt.Errorf("specify --to or --duration")
		}

		out := trimOpts.output
		if out == "" {
			out = suffixName(input, "_trim", "")
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		eng, err := newEngine()
		if err != nil {
			return err
		}

		var codecArgs []string
		if trimOpts.reencode {
			// Frame-accurate: re-encode with codecs the output container accepts.
			info, err := eng.Probe(cmd.Context(), input)
			if err != nil {
				return err
			}
			codecArgs = reencodeTrimArgs(info, out)
		} else {
			codecArgs = []string{"-c", "copy", "-avoid_negative_ts", "make_zero"}
		}

		opts := baseRunOptions()
		opts.Args = append([]string{
			"-ss", ui.Clock(from), "-i", input, "-t", ui.Clock(dur),
		}, codecArgs...)
		opts.Args = append(opts.Args, out)
		opts.Total = dur
		opts.Label = "Trimming"

		if err := eng.Run(cmd.Context(), opts); err != nil {
			return err
		}
		if globals.DryRun {
			return nil
		}

		size := ""
		if fi, err := os.Stat(out); err == nil {
			size = ui.Bytes(fi.Size())
		}
		ui.Successf("Trimmed %s -> %s (%s)", ui.Duration(dur), out, size)
		return nil
	},
}

// reencodeTrimArgs returns frame-accurate re-encode arguments matched to the
// output container and whether the source actually has a video stream.
func reencodeTrimArgs(info *ffprobe.Result, out string) []string {
	if info.VideoStream() == nil {
		// Audio-only input: re-encode just the audio for the target container.
		return append([]string{"-vn"}, audioCodecForExt(out)...)
	}
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(out), ".")) {
	case "webm":
		return []string{"-c:v", "libvpx-vp9", "-b:v", "0", "-crf", "32",
			"-row-mt", "1", "-c:a", "libopus", "-b:a", "128k"}
	default:
		return []string{"-c:v", "libx264", "-crf", "18", "-preset", "medium",
			"-c:a", "aac", "-b:a", "192k"}
	}
}

func init() {
	f := trimCmd.Flags()
	f.StringVar(&trimOpts.from, "from", "0", "start timecode (e.g. 00:01:20, 90, 10s, 1m30s)")
	f.StringVar(&trimOpts.to, "to", "", "end timecode")
	f.StringVar(&trimOpts.duration, "duration", "", "clip length (alternative to --to)")
	f.StringVarP(&trimOpts.output, "output", "o", "", "output path (default: <input>_trim.<ext>)")
	f.BoolVar(&trimOpts.reencode, "reencode", false, "frame-accurate re-encode instead of lossless stream copy")
	rootCmd.AddCommand(trimCmd)
}
