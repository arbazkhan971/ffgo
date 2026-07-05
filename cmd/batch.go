package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/formats"
	"github.com/arbazkhan971/ffgo/internal/presets"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

// batch flag values, populated by cobra before RunE.
var (
	batchTo       string
	batchCompress bool
	batchTarget   string
	batchPreset   string
	batchQuality  string
	batchOutput   string
)

var batchCmd = &cobra.Command{
	Use:     "batch <glob>...",
	Aliases: []string{"bulk"},
	Short:   "Convert or compress many files in one pass",
	Long: "Batch applies the same operation to every file matched by one or more\n" +
		"glob patterns. Choose exactly one mode: --to converts each file into the\n" +
		"given container, while --compress shrinks each file (tune it with --target,\n" +
		"--preset or --quality).\n\n" +
		"Outputs land next to their source unless -o names a directory. A failed file\n" +
		"is reported but never stops the run; the exit code is non-zero if any failed.",
	Example: "  ffgo batch \"./videos/*.mov\" --to mp4\n" +
		"  ffgo batch \"./clips/*\" --compress --target 50mb\n" +
		"  ffgo batch \"*.mp4\" --compress --preset whatsapp -o out/",
	Args: cobra.MinimumNArgs(1),
	RunE: runBatch,
}

func init() {
	f := batchCmd.Flags()
	f.StringVar(&batchTo, "to", "", "convert each file to this container: "+strings.Join(formats.Names(), ", "))
	f.BoolVar(&batchCompress, "compress", false, "compress each file instead of converting")
	f.StringVar(&batchTarget, "target", "", "target size for --compress (e.g. 50mb, 1.5gb)")
	f.StringVar(&batchPreset, "preset", "", "compression preset for --compress: "+strings.Join(presets.Names(), ", "))
	f.StringVar(&batchQuality, "quality", "", "compression quality for --compress: low, medium, high")
	f.StringVarP(&batchOutput, "output", "o", "", "output directory (created if missing; default: alongside each source)")
	rootCmd.AddCommand(batchCmd)
}

// batchJob captures the resolved compression settings shared by every file so
// they are validated once, before the run begins.
type batchJob struct {
	baseEnc     presets.Encoding
	targetBytes int64 // > 0 when size-targeting per file
}

func runBatch(cmd *cobra.Command, args []string) error {
	// Exactly one mode.
	switch {
	case batchTo == "" && !batchCompress:
		return fmt.Errorf("choose a mode: --to <container> or --compress")
	case batchTo != "" && batchCompress:
		return fmt.Errorf("choose either --to or --compress, not both")
	}

	// Compression-only flags must not appear in convert mode.
	if batchTo != "" && (batchTarget != "" || batchPreset != "" || batchQuality != "") {
		return fmt.Errorf("--target/--preset/--quality only apply with --compress")
	}

	// Resolve the operation up front so a misconfiguration fails fast.
	var container formats.Container
	var job batchJob
	if batchCompress {
		j, err := resolveBatchJob()
		if err != nil {
			return err
		}
		job = j
	} else {
		c, ok := formats.Lookup(batchTo)
		if !ok {
			return fmt.Errorf("unknown container %q; supported: %s", batchTo, strings.Join(formats.Names(), ", "))
		}
		container = c
	}

	// Expand the glob patterns into a deduplicated list of regular files.
	files, err := batchCollectFiles(args)
	if err != nil {
		return err
	}

	// Prepare the output directory once if the user asked for one.
	outdir := batchOutput
	if outdir != "" && !globals.DryRun {
		if err := os.MkdirAll(outdir, 0o755); err != nil {
			return fmt.Errorf("cannot create output directory %q: %w", outdir, err)
		}
	}

	eng, err := newEngine()
	if err != nil {
		return err
	}

	n := len(files)
	var (
		succeeded int
		failures  []string
		saved     int64
	)

	for i, f := range files {
		base := filepath.Base(f)
		ui.Stepf("[%d/%d] %s", i+1, n, base)

		info, err := eng.Probe(cmd.Context(), f)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", base, err))
			continue
		}
		dur := info.Duration()

		dir := outdir
		if dir == "" {
			dir = filepath.Dir(f)
		}

		var (
			out     string
			runArgs []string
		)
		if batchCompress {
			out, runArgs, err = batchBuildCompress(f, dir, job, dur)
		} else {
			out, runArgs = batchBuildConvert(f, dir, container, info)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", base, err))
			continue
		}

		if filepath.Clean(out) == filepath.Clean(f) {
			failures = append(failures, fmt.Sprintf("%s: output %q would overwrite the source; use -o", base, out))
			continue
		}
		if err := checkOverwrite(out); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", base, err))
			continue
		}

		opts := baseRunOptions()
		opts.Args = runArgs
		opts.Total = dur
		opts.Label = base
		if err := eng.Run(cmd.Context(), opts); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", base, err))
			continue
		}
		succeeded++

		if batchCompress && !globals.DryRun {
			if st, err := os.Stat(out); err == nil {
				saved += info.Size() - st.Size()
			}
		}
	}

	return batchReport(succeeded, failures, saved)
}

// resolveBatchJob builds the compression template from --preset/--quality and
// the optional --target size, validating the flag combination.
func resolveBatchJob() (batchJob, error) {
	if batchPreset != "" && batchQuality != "" {
		return batchJob{}, fmt.Errorf("choose either --preset or --quality, not both")
	}

	var job batchJob
	switch {
	case batchPreset != "":
		p, ok := presets.Lookup(batchPreset)
		if !ok {
			return batchJob{}, fmt.Errorf("unknown preset %q; supported: %s", batchPreset, strings.Join(presets.Names(), ", "))
		}
		job.baseEnc = p.Enc
		if p.TargetMB > 0 {
			job.targetBytes = int64(p.TargetMB * (1 << 20))
		}
	case batchQuality != "":
		enc, err := presets.QualityEncoding(presets.Quality(batchQuality))
		if err != nil {
			return batchJob{}, err
		}
		job.baseEnc = enc
	default:
		// A bare --compress uses a balanced quality encode.
		enc, err := presets.QualityEncoding(presets.QualityMedium)
		if err != nil {
			return batchJob{}, err
		}
		job.baseEnc = enc
	}

	// An explicit --target overrides any preset budget and forces size mode.
	if batchTarget != "" {
		tb, err := ui.ParseBytes(batchTarget)
		if err != nil {
			return batchJob{}, err
		}
		if tb <= 0 {
			return batchJob{}, fmt.Errorf("--target must be greater than zero")
		}
		job.targetBytes = tb
	}
	return job, nil
}

// batchBuildConvert produces the output path and ffmpeg args for repackaging f into
// the target container, stream-copying compatible tracks and re-encoding the
// rest (ModeAuto).
func batchBuildConvert(f, dir string, container formats.Container, info *ffprobe.Result) (string, []string) {
	srcVideo := ""
	if v := info.VideoStream(); v != nil {
		srcVideo = v.CodecName
	}
	var srcAudio []string
	for _, a := range info.AudioStreams() {
		srcAudio = append(srcAudio, a.CodecName)
	}
	plan := formats.PlanConvert(container, srcVideo, srcAudio, formats.ModeAuto)

	out := filepath.Join(dir, batchStem(f)+"."+container.Ext)
	args := append([]string{"-i", f}, append(plan.Args, out)...)
	return out, args
}

// batchBuildCompress produces the output path and ffmpeg args for compressing f. For
// a size target it solves the video bitrate against this file's duration.
func batchBuildCompress(f, dir string, job batchJob, dur time.Duration) (string, []string, error) {
	enc := job.baseEnc

	if job.targetBytes > 0 {
		audioKbps := 0
		if enc.AudioCodec != "" && enc.AudioCodec != "none" {
			audioKbps = enc.AudioBitrateK
			if audioKbps == 0 {
				audioKbps = 128
			}
		}
		kbps, err := presets.SolveBitrate(job.targetBytes, dur, audioKbps)
		if err != nil {
			return "", nil, err
		}
		enc.CRF = 0
		enc.VideoBitrateK = kbps
	}

	// Use an H.264/AAC-safe container so incompatible sources (e.g. .webm)
	// don't fail the encode.
	ext := "." + h264SafeExt(f)
	out := filepath.Join(dir, batchStem(f)+"_compressed"+ext)

	args := append([]string{"-i", f}, enc.OutputArgs()...)
	// Progressive-download flag for containers that support it (mp4/mov).
	if c, ok := formats.Lookup(strings.TrimPrefix(ext, ".")); ok && c.Faststart {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, out)
	return out, args, nil
}

// stem returns the base name of path without its extension.
func batchStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// batchCollectFiles expands every glob pattern, keeping only existing regular files,
// deduplicated while preserving first-seen order.
func batchCollectFiles(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	for _, pat := range patterns {
		matches, err := filepath.Glob(pat)
		if err != nil {
			return nil, fmt.Errorf("bad pattern %q: %w", pat, err)
		}
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || st.IsDir() || !st.Mode().IsRegular() {
				continue
			}
			clean := filepath.Clean(m)
			if seen[clean] {
				continue
			}
			seen[clean] = true
			files = append(files, m)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files matched: %s", strings.Join(patterns, " "))
	}
	return files, nil
}

// batchReport prints the run summary and returns a non-zero error when any file
// failed, after every file has been attempted.
func batchReport(succeeded int, failures []string, saved int64) error {
	for _, msg := range failures {
		ui.Errorf("%s", msg)
	}
	failed := len(failures)
	showSaved := batchCompress && saved > 0 && !globals.DryRun
	switch {
	case failed == 0 && showSaved:
		ui.Successf("%d succeeded, %d failed — saved %s total", succeeded, failed, ui.Bytes(saved))
	case failed == 0:
		ui.Successf("%d succeeded, %d failed", succeeded, failed)
	case showSaved:
		ui.Warnf("%d succeeded, %d failed — saved %s total", succeeded, failed, ui.Bytes(saved))
	default:
		ui.Warnf("%d succeeded, %d failed", succeeded, failed)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d file(s) failed", failed, succeeded+failed)
	}
	return nil
}
