package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/presets"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var (
	compressTarget  string
	compressQuality string
	compressPreset  string
	compressOutput  string
)

var compressCmd = &cobra.Command{
	Use:     "compress <input>",
	Aliases: []string{"shrink", "c"},
	Short:   "Shrink a video to a target size, quality level or platform preset",
	Long: "Compress a video in one of three ways: to a target file size, to a\n" +
		"named quality level, or with a platform preset. Size mode solves for the\n" +
		"bitrate that best fills your budget; quality mode uses a constant-rate-\n" +
		"factor (CRF) encode. With no flags, --quality medium is used.\n\n" +
		"Qualities: low, medium, high\n\n" +
		"Presets:\n" + indentLines(presets.Describe()),
	Example: "  ffgo compress clip.mp4 --target 25mb\n" +
		"  ffgo compress clip.mp4 --quality high\n" +
		"  ffgo compress clip.mp4 --preset whatsapp -o small.mp4",
	Args: cobra.ExactArgs(1),
	RunE: runCompress,
}

func init() {
	f := compressCmd.Flags()
	f.StringVar(&compressTarget, "target", "", "target output size, e.g. 25mb or 500k")
	f.StringVar(&compressQuality, "quality", "", "quality level: low, medium or high")
	f.StringVar(&compressPreset, "preset", "", "platform preset ("+strings.Join(presets.Names(), ", ")+")")
	f.StringVarP(&compressOutput, "output", "o", "", "output file (default: <input>_compressed.<ext>)")
	rootCmd.AddCommand(compressCmd)
}

func runCompress(cmd *cobra.Command, args []string) error {
	input := args[0]
	if err := requireFile(input); err != nil {
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
	dur := info.Duration()

	// Decide the encoding and whether we are targeting a specific size.
	enc, sizeMode, targetBytes, err := planCompress()
	if err != nil {
		return err
	}

	// In size mode, solve for the bitrate that best fills the byte budget.
	if sizeMode {
		audioKbps := enc.AudioBitrateK
		if audioKbps == 0 {
			audioKbps = 128
		}
		kbps, err := presets.SolveBitrate(targetBytes, dur, audioKbps)
		if err != nil {
			return err
		}
		enc.VideoBitrateK = kbps
		enc.CRF = 0
		enc.MaxrateK = kbps
		enc.BufsizeK = kbps * 2
		if enc.Preset == "" {
			enc.Preset = "medium"
		}
	}

	// Keep the input container when it can hold H.264/AAC, else fall back to
	// MP4 so the encode never fails on an incompatible container (e.g. .webm).
	out := compressOutput
	if out == "" {
		out = suffixName(input, "_compressed", h264SafeExt(input))
	}
	if err := checkOverwrite(out); err != nil {
		return err
	}

	// Assemble output args: encoder settings, faststart for MP4-family, path.
	ffArgs := append([]string{"-i", input}, enc.OutputArgs()...)
	if isFaststartExt(out) {
		ffArgs = append(ffArgs, "-movflags", "+faststart")
	}
	ffArgs = append(ffArgs, out)

	opts := baseRunOptions()
	opts.Args = ffArgs
	opts.Total = dur
	opts.Label = "Compressing " + filepath.Base(input)

	if err := eng.Run(cmd.Context(), opts); err != nil {
		return err
	}
	if globals.DryRun {
		return nil
	}

	reportCompress(input, out, sizeMode, targetBytes)
	return nil
}

// planCompress selects the encoding template and size target from the flags,
// following the precedence: --preset, then --target, then --quality, then a
// medium-quality default.
func planCompress() (enc presets.Encoding, sizeMode bool, targetBytes int64, err error) {
	switch {
	case compressPreset != "":
		p, ok := presets.Lookup(compressPreset)
		if !ok {
			return enc, false, 0, fmt.Errorf("unknown preset %q. Available presets:\n%s",
				compressPreset, indentLines(presets.Describe()))
		}
		enc = p.Enc
		switch {
		case compressTarget != "":
			targetBytes, err = ui.ParseBytes(compressTarget)
			if err != nil {
				return enc, false, 0, err
			}
			sizeMode = true
		case p.TargetMB > 0:
			targetBytes = int64(p.TargetMB * 1024 * 1024)
			sizeMode = true
		}
		return enc, sizeMode, targetBytes, nil

	case compressTarget != "":
		targetBytes, err = ui.ParseBytes(compressTarget)
		if err != nil {
			return enc, false, 0, err
		}
		// Use a medium-quality template purely for its codec/audio choices.
		enc, err = presets.QualityEncoding(presets.QualityMedium)
		if err != nil {
			return enc, false, 0, err
		}
		return enc, true, targetBytes, nil

	case compressQuality != "":
		enc, err = presets.QualityEncoding(presets.Quality(compressQuality))
		if err != nil {
			return enc, false, 0, err
		}
		return enc, false, 0, nil

	default:
		enc, err = presets.QualityEncoding(presets.QualityMedium)
		if err != nil {
			return enc, false, 0, err
		}
		return enc, false, 0, nil
	}
}

// reportCompress prints a concise before/after summary and warns when a size
// target was missed by a wide margin.
func reportCompress(input, out string, sizeMode bool, targetBytes int64) {
	inStat, err1 := os.Stat(input)
	outStat, err2 := os.Stat(out)
	if err1 != nil || err2 != nil {
		ui.Successf("Compressed %s %s %s", filepath.Base(input), ui.Glyph("→", "->"), out)
		return
	}

	oldSize, newSize := inStat.Size(), outStat.Size()
	arrow := ui.Glyph("→", "->")
	change := "same size"
	if oldSize > 0 && newSize != oldSize {
		pct := (1 - float64(newSize)/float64(oldSize)) * 100
		if pct >= 0 {
			change = fmt.Sprintf("%.0f%% smaller", pct)
		} else {
			change = fmt.Sprintf("%.0f%% larger", -pct)
		}
	}
	ui.Successf("Compressed %s %s %s (%s) %s %s",
		ui.Bytes(oldSize), arrow, ui.Bytes(newSize), change, arrow, out)

	if sizeMode && float64(newSize) > float64(targetBytes)*1.10 {
		ui.Warnf("Output %s exceeds the %s target — try a lower --quality, downscaling or a shorter clip",
			ui.Bytes(newSize), ui.Bytes(targetBytes))
	}
}

// isFaststartExt reports whether out uses an MP4-family container that benefits
// from moving the moov atom to the front for streaming.
func isFaststartExt(out string) bool {
	switch strings.ToLower(filepath.Ext(out)) {
	case ".mp4", ".mov", ".m4v":
		return true
	default:
		return false
	}
}

// indentLines prefixes each line with two spaces for tidy help blocks.
func indentLines(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("  ")
		b.WriteString(l)
	}
	return b.String()
}
