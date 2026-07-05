package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/formats"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var (
	convertTo       string
	convertOutput   string
	convertCopy     bool
	convertReencode bool
)

var convertCmd = &cobra.Command{
	Use:     "convert <input>",
	Aliases: []string{"conv"},
	Short:   "Convert a media file into another container, copying streams when possible",
	Long: "Convert repackages a file into another container (mp4, mkv, webm, mov, avi).\n" +
		"By default it stream-copies compatible tracks — a fast, lossless remux — and\n" +
		"only re-encodes the streams the target container cannot accept.\n\n" +
		"Force the decision with --copy (never transcode) or --reencode (always transcode).",
	Example: "  ffgo convert input.mov --to mp4\n" +
		"  ffgo convert in.mkv -o out.webm\n" +
		"  ffgo convert clip.avi --to mkv --reencode",
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	f := convertCmd.Flags()
	f.StringVar(&convertTo, "to", "", "target container: "+strings.Join(formats.Names(), ", "))
	f.StringVarP(&convertOutput, "output", "o", "", "output path (extension selects the container)")
	f.BoolVar(&convertCopy, "copy", false, "force stream copy (never re-encode)")
	f.BoolVar(&convertReencode, "reencode", false, "force a full re-encode")
	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	input := args[0]
	if err := requireFile(input); err != nil {
		return err
	}

	if convertCopy && convertReencode {
		return fmt.Errorf("choose either --copy or --reencode, not both")
	}

	// Resolve the target container from --to, or infer it from the output
	// extension when only --output was given.
	target := convertTo
	if target == "" && convertOutput != "" {
		target = filepath.Ext(convertOutput)
	}
	if target == "" {
		return fmt.Errorf("specify a target container with --to (e.g. --to mp4) or an --output path")
	}
	container, ok := formats.Lookup(target)
	if !ok {
		return fmt.Errorf("unknown container %q; supported: %s", target, strings.Join(formats.Names(), ", "))
	}

	// Decide the output path.
	out := convertOutput
	if out == "" {
		out = replaceExt(input, container.Ext)
	}
	if filepath.Clean(out) == filepath.Clean(input) {
		return fmt.Errorf("output %q is the same as the input; choose a different name or container", out)
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

	srcVideo := ""
	if v := info.VideoStream(); v != nil {
		srcVideo = v.CodecName
	}
	var srcAudio []string
	for _, a := range info.AudioStreams() {
		srcAudio = append(srcAudio, a.CodecName)
	}

	mode := formats.ModeAuto
	switch {
	case convertCopy:
		mode = formats.ModeCopy
	case convertReencode:
		mode = formats.ModeReencode
	}
	plan := formats.PlanConvert(container, srcVideo, srcAudio, mode)

	opts := baseRunOptions()
	opts.Args = append([]string{"-i", input}, append(plan.Args, out)...)
	opts.Total = info.Duration()
	opts.Label = "Converting " + filepath.Base(input) + " -> ." + container.Ext
	if err := eng.Run(cmd.Context(), opts); err != nil {
		return err
	}
	if globals.DryRun {
		return nil
	}

	size := ""
	if st, err := os.Stat(out); err == nil {
		size = ui.Bytes(st.Size())
	}
	if plan.StreamCopied {
		ui.Successf("Remuxed losslessly -> %s (%s)", out, size)
	} else {
		ui.Successf("Converted -> %s (%s)", out, size)
	}
	return nil
}
