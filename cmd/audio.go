package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ffmpeg"
	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

// audioCmd groups the audio-focused operations under a single parent command.
var audioCmd = &cobra.Command{
	Use:     "audio",
	Aliases: []string{"a"},
	Short:   "Extract, normalize, clean and convert audio tracks",
	Long: "Audio toolkit: pull audio out of videos, normalize loudness, trim\n" +
		"leading/trailing silence and convert between common audio formats.\n\n" +
		"Every subcommand prints the exact FFmpeg it runs; add --dry-run to preview.",
	Example: "  ffgo audio extract clip.mp4 --format mp3\n" +
		"  ffgo audio normalize podcast.wav\n" +
		"  ffgo audio silence-remove voice.m4a\n" +
		"  ffgo audio convert song.wav --to flac",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// extract flags.
var (
	extractOut    string
	extractFormat string
	extractCopy   bool
)

var audioExtractCmd = &cobra.Command{
	Use:   "extract <input>",
	Short: "Strip the video and keep only the audio track",
	Long: "Extract the audio from a media file, dropping any video.\n\n" +
		"By default the audio is re-encoded to the chosen --format (mp3). Pass\n" +
		"--copy to remux the existing audio stream without re-encoding.",
	Example: "  ffgo audio extract movie.mkv --format mp3\n" +
		"  ffgo audio extract clip.mp4 --copy\n" +
		"  ffgo audio extract talk.mov -o talk.flac --format flac",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
		if err := requireAudio(info, input); err != nil {
			return err
		}

		var encArgs []string
		var ext string
		if extractCopy {
			srcCodec := ""
			if streams := info.AudioStreams(); len(streams) > 0 {
				srcCodec = streams[0].CodecName
			}
			// Copy mode must use a container that accepts the source codec,
			// otherwise ffmpeg rejects it at mux time.
			ext = audioCopyExt(srcCodec)
			encArgs = []string{"-vn", "-c:a", "copy"}
		} else {
			var codecArgs []string
			if codecArgs, ext, err = audioEncoding(extractFormat); err != nil {
				return err
			}
			encArgs = append([]string{"-vn"}, codecArgs...)
		}

		out := extractOut
		if out == "" {
			out = replaceExt(input, ext)
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		fargs := []string{"-i", input}
		fargs = append(fargs, encArgs...)
		fargs = append(fargs, out)

		return runAudio(cmd, eng, fargs, info.Duration(), "Extracting audio", out)
	},
}

// normalize flags.
var normalizeOut string

var audioNormalizeCmd = &cobra.Command{
	Use:     "normalize <input>",
	Aliases: []string{"loudnorm"},
	Short:   "Loudness-normalize audio to the EBU R128 standard",
	Long: "Apply EBU R128 loudness normalization (I=-16, TP=-1.5, LRA=11), a\n" +
		"common target for podcasts and streaming.\n\n" +
		"Any video track is stream-copied so only the audio is touched.",
	Example: "  ffgo audio normalize podcast.wav\n" +
		"  ffgo audio normalize lecture.mp4 -o lecture_loud.mp4",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
		if err := requireAudio(info, input); err != nil {
			return err
		}

		out := normalizeOut
		if out == "" {
			out = suffixName(input, "_normalized", "")
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		fargs := []string{"-i", input, "-af", "loudnorm=I=-16:TP=-1.5:LRA=11"}
		if info.VideoStream() != nil {
			fargs = append(fargs, "-c:v", "copy")
		}
		fargs = append(fargs, audioCodecForExt(out)...)
		fargs = append(fargs, out)

		return runAudio(cmd, eng, fargs, info.Duration(), "Normalizing", out)
	},
}

// silence-remove flags.
var silenceOut string

var audioSilenceCmd = &cobra.Command{
	Use:     "silence-remove <input>",
	Aliases: []string{"trim-silence"},
	Short:   "Remove leading, trailing and mid-track silence",
	Long: "Strip silence from the start and throughout a track using FFmpeg's\n" +
		"silenceremove filter with a -50 dB threshold.\n\n" +
		"The audio is re-encoded (AAC, or PCM for a WAV output).",
	Example: "  ffgo audio silence-remove voice.m4a\n" +
		"  ffgo audio silence-remove recording.wav -o tight.wav",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
		if err := requireAudio(info, input); err != nil {
			return err
		}

		out := silenceOut
		if out == "" {
			out = suffixName(input, "_nosilence", "")
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		const filter = "silenceremove=start_periods=1:start_silence=0.1:" +
			"start_threshold=-50dB:stop_periods=-1:stop_silence=0.5:stop_threshold=-50dB"
		fargs := []string{"-i", input, "-af", filter}
		fargs = append(fargs, audioCodecForExt(out)...)
		fargs = append(fargs, out)

		// Silence removal shortens the track, so the input duration is an
		// upper bound for the progress bar.
		return runAudio(cmd, eng, fargs, info.Duration(), "Removing silence", out)
	},
}

// convert flags.
var (
	convertOut     string
	audioConvertTo string
)

var audioConvertCmd = &cobra.Command{
	Use:   "convert <input> --to <format>",
	Short: "Convert audio to another format",
	Long: "Re-encode an audio file into a different format (mp3, aac, m4a, wav,\n" +
		"flac or opus), picking a sensible codec and quality for each.",
	Example: "  ffgo audio convert song.wav --to flac\n" +
		"  ffgo audio convert voice.m4a --to mp3 -o voice.mp3",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		if err := requireFile(input); err != nil {
			return err
		}
		codecArgs, ext, err := audioEncoding(audioConvertTo)
		if err != nil {
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
		if err := requireAudio(info, input); err != nil {
			return err
		}

		out := convertOut
		if out == "" {
			out = replaceExt(input, ext)
		}
		if err := checkOverwrite(out); err != nil {
			return err
		}

		fargs := []string{"-i", input, "-vn"}
		fargs = append(fargs, codecArgs...)
		fargs = append(fargs, out)

		return runAudio(cmd, eng, fargs, info.Duration(), "Converting audio", out)
	},
}

func init() {
	audioExtractCmd.Flags().StringVarP(&extractOut, "output", "o", "", "output file (default derives from input)")
	audioExtractCmd.Flags().StringVar(&extractFormat, "format", "mp3", "audio format: mp3, aac, m4a, wav, flac or opus")
	audioExtractCmd.Flags().BoolVar(&extractCopy, "copy", false, "copy the audio stream without re-encoding")

	audioNormalizeCmd.Flags().StringVarP(&normalizeOut, "output", "o", "", "output file (default adds _normalized)")

	audioSilenceCmd.Flags().StringVarP(&silenceOut, "output", "o", "", "output file (default adds _nosilence)")

	audioConvertCmd.Flags().StringVarP(&convertOut, "output", "o", "", "output file (default derives from input)")
	audioConvertCmd.Flags().StringVar(&audioConvertTo, "to", "", "target format: mp3, aac, m4a, wav, flac or opus")
	_ = audioConvertCmd.MarkFlagRequired("to")

	audioCmd.AddCommand(audioExtractCmd, audioNormalizeCmd, audioSilenceCmd, audioConvertCmd)
	rootCmd.AddCommand(audioCmd)
}

// audioEncoding maps a target audio format name to the ffmpeg audio codec
// arguments and the file extension that carries it.
func audioEncoding(format string) (args []string, ext string, err error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-q:a", "2"}, "mp3", nil
	case "aac":
		return []string{"-c:a", "aac", "-b:a", "192k"}, "aac", nil
	case "m4a":
		return []string{"-c:a", "aac", "-b:a", "192k"}, "m4a", nil
	case "wav":
		return []string{"-c:a", "pcm_s16le"}, "wav", nil
	case "flac":
		return []string{"-c:a", "flac"}, "flac", nil
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", "128k"}, "opus", nil
	default:
		return nil, "", fmt.Errorf("unsupported audio format %q (choose mp3, aac, m4a, wav, flac or opus)", format)
	}
}

// audioCodecForExt returns re-encode arguments that produce audio the given
// output container will accept. The extension may include a leading dot.
// Video containers (mp4, mkv, mov) fall through to AAC; webm needs Opus.
func audioCodecForExt(path string) []string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-q:a", "2"}
	case "wav":
		return []string{"-c:a", "pcm_s16le"}
	case "flac":
		return []string{"-c:a", "flac"}
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", "128k"}
	case "ogg":
		return []string{"-c:a", "libvorbis", "-q:a", "5"}
	case "webm":
		return []string{"-c:a", "libopus", "-b:a", "128k"}
	default: // m4a, aac, mp4, mov, mkv, ... — AAC is broadly compatible
		return []string{"-c:a", "aac", "-b:a", "192k"}
	}
}

// audioCopyExt returns a container extension that can stream-copy the given
// source codec. Unknown codecs fall back to Matroska audio (.mka), which
// accepts virtually any codec without re-encoding.
func audioCopyExt(codec string) string {
	switch strings.ToLower(codec) {
	case "aac", "alac":
		return "m4a"
	case "mp3":
		return "mp3"
	case "ac3":
		return "ac3"
	case "eac3":
		return "eac3"
	case "flac":
		return "flac"
	case "opus":
		return "opus"
	case "vorbis":
		return "ogg"
	case "pcm_s16le", "pcm_s24le", "pcm_s16be", "pcm_s24be":
		return "wav"
	default:
		return "mka"
	}
}

// requireAudio errors when the probed file carries no audio to work on.
func requireAudio(info *ffprobe.Result, input string) error {
	if !info.HasAudio() {
		return fmt.Errorf("%q has no audio track", input)
	}
	return nil
}

// runAudio executes an audio operation through the engine and reports the
// result. It respects --dry-run via the engine and stays quiet on failure.
func runAudio(cmd *cobra.Command, eng *ffmpeg.Engine, fargs []string, total time.Duration, label, out string) error {
	opts := baseRunOptions()
	opts.Args = fargs
	opts.Total = total
	opts.Label = label
	if err := eng.Run(cmd.Context(), opts); err != nil {
		return err
	}
	reportAudioDone(out)
	return nil
}

// reportAudioDone prints a success line for a finished audio operation, adding
// the output size when it can be determined. Dry runs write nothing.
func reportAudioDone(out string) {
	if globals.DryRun {
		return
	}
	if info, err := os.Stat(out); err == nil {
		ui.Successf("%s  %s", ui.Bold(out), ui.Dim(ui.Bytes(info.Size())))
		return
	}
	ui.Successf("%s", ui.Bold(out))
}
