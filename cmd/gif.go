package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ui"
)

// gif flag values, populated by cobra before RunE.
var (
	gifFrom     string
	gifTo       string
	gifDuration string
	gifWidth    int
	gifFPS      int
	gifOutput   string
)

var gifCmd = &cobra.Command{
	Use:     "gif <input>",
	Aliases: []string{"togif"},
	Short:   "Turn a video segment into a high-quality animated GIF",
	Long: "Create a crisp animated GIF from a video (or a slice of one) using a\n" +
		"generated colour palette for far better quality than a naive export.\n\n" +
		"Pick the segment with --from plus either --to or --duration; with neither,\n" +
		"the whole file from --from onward is converted. Tune size with --width/--fps.",
	Example: "  ffgo gif clip.mp4 --from 10s --to 20s --width 480\n" +
		"  ffgo gif clip.mp4 --from 00:00:05 --duration 3s --fps 20\n" +
		"  ffgo gif loop.mov -o out.gif",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		if gifWidth <= 0 {
			return fmt.Errorf("--width must be a positive number of pixels")
		}
		if gifFPS <= 0 {
			return fmt.Errorf("--fps must be a positive frame rate")
		}
		if gifTo != "" && gifDuration != "" {
			return fmt.Errorf("use either --to or --duration, not both")
		}

		from, err := ui.ParseTimecode(gifFrom)
		if err != nil {
			return fmt.Errorf("invalid --from: %w", err)
		}
		if from < 0 {
			return fmt.Errorf("--from cannot be negative")
		}

		eng, err := newEngine()
		if err != nil {
			return err
		}

		info, err := eng.Probe(cmd.Context(), input)
		if err != nil {
			return err
		}
		total := info.Duration()

		// Resolve the clip duration from --to, --duration, or the remainder of
		// the file after --from.
		var dur time.Duration
		switch {
		case gifTo != "":
			to, err := ui.ParseTimecode(gifTo)
			if err != nil {
				return fmt.Errorf("invalid --to: %w", err)
			}
			if to <= from {
				return fmt.Errorf("--to (%s) must be after --from (%s)",
					ui.Clock(to), ui.Clock(from))
			}
			dur = to - from
		case gifDuration != "":
			d, err := ui.ParseTimecode(gifDuration)
			if err != nil {
				return fmt.Errorf("invalid --duration: %w", err)
			}
			if d <= 0 {
				return fmt.Errorf("--duration must be positive")
			}
			dur = d
		default:
			if total > 0 {
				dur = total - from
				if dur <= 0 {
					return fmt.Errorf("--from (%s) is at or past the end of the file (%s)",
						ui.Clock(from), ui.Clock(total))
				}
			}
		}

		out := gifOutput
		if out == "" {
			out = replaceExt(input, "gif")
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		// Single-pass palette generation and application for a high-quality GIF.
		vf := fmt.Sprintf(
			"fps=%d,scale=%d:-1:flags=lanczos,split[s0][s1];"+
				"[s0]palettegen=stats_mode=diff[p];"+
				"[s1][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle",
			gifFPS, gifWidth)

		args2 := []string{"-ss", ui.Clock(from), "-i", input}
		if dur > 0 {
			args2 = append(args2, "-t", ui.Clock(dur))
		}
		args2 = append(args2, "-filter_complex", vf, "-loop", "0", out)

		opts := baseRunOptions()
		opts.Args = args2
		opts.Total = dur
		opts.Label = "Making GIF"

		if err := eng.Run(cmd.Context(), opts); err != nil {
			return err
		}
		if globals.DryRun {
			return nil
		}

		if st, err := os.Stat(out); err == nil {
			ui.Successf("GIF written to %s (%s)", ui.Bold(out), ui.Bytes(st.Size()))
		} else {
			ui.Successf("GIF written to %s", ui.Bold(out))
		}
		return nil
	},
}

func init() {
	f := gifCmd.Flags()
	f.StringVar(&gifFrom, "from", "0", "start time (e.g. 90, 10s, 1m30s, 00:01:30)")
	f.StringVar(&gifTo, "to", "", "end time; overrides --duration")
	f.StringVar(&gifDuration, "duration", "", "clip length instead of --to")
	f.IntVar(&gifWidth, "width", 480, "output width in pixels (height auto)")
	f.IntVar(&gifFPS, "fps", 15, "frames per second")
	f.StringVarP(&gifOutput, "output", "o", "", "output path (default: input with .gif)")
	rootCmd.AddCommand(gifCmd)
}
