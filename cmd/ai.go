package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/ai"
	"github.com/arbazkhan971/ffgo/internal/ffprobe"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var (
	aiProvider string
	aiModel    string
	aiFile     string
)

var aiCmd = &cobra.Command{
	Use:     "ai [request]",
	Aliases: []string{"ask"},
	Short:   "Describe what you want in plain English; get and run the ffmpeg command",
	Long: "Turn a natural-language request into a single ffmpeg command using an LLM.\n" +
		"Point it at a file with --file so the model knows the codecs, resolution and\n" +
		"duration it is working with. Every command is shown for review before it runs.\n\n" +
		"With no request, ffgo ai starts an interactive prompt. Configure a provider\n" +
		"and key via FFGO_AI_PROVIDER and the matching *_API_KEY environment variable.",
	Example: "  ffgo ai \"make a 480p web-friendly mp4\" --file clip.mov\n" +
		"  ffgo ai \"extract the audio as mp3\" -f song.mkv\n" +
		"  ffgo ai --provider anthropic\n" +
		"  ffgo ai \"trim the first 10 seconds\" input.mp4",
	Args: cobra.ArbitraryArgs,
	RunE: runAI,
}

func init() {
	f := aiCmd.Flags()
	f.StringVar(&aiProvider, "provider", "", "AI provider: openai, anthropic, gemini, ollama, openrouter")
	f.StringVar(&aiModel, "model", "", "model name (defaults to the provider's recommended model)")
	f.StringVarP(&aiFile, "file", "f", "", "input media file for context (also usable as a positional arg)")
	rootCmd.AddCommand(aiCmd)
}

func runAI(cmd *cobra.Command, args []string) error {
	// A trailing argument that names an existing file is treated as --file so
	// that `ffgo ai "..." input.mp4` works like the other commands.
	request := strings.TrimSpace(strings.Join(args, " "))
	if aiFile == "" && len(args) > 0 {
		last := args[len(args)-1]
		if info, err := os.Stat(last); err == nil && !info.IsDir() {
			aiFile = last
			request = strings.TrimSpace(strings.Join(args[:len(args)-1], " "))
		}
	}

	if aiFile != "" {
		if err := requireFile(aiFile); err != nil {
			return err
		}
	}

	cfg, err := ai.LoadConfig()
	if err != nil {
		return err
	}
	if aiProvider != "" {
		cfg.Provider = aiProvider
		// Re-resolve the API key for the newly selected provider, otherwise we
		// would still hold the default provider's (likely empty) key.
		cfg.APIKey = ai.ResolveAPIKey(aiProvider)
	}
	if aiModel != "" {
		cfg.Model = aiModel
	}

	provider, err := ai.New(cfg)
	if err != nil {
		return fmt.Errorf("%w\n\nSet up a provider, for example:\n"+
			"  export FFGO_AI_PROVIDER=openai\n"+
			"  export OPENAI_API_KEY=sk-...\n"+
			"Other providers: anthropic (ANTHROPIC_API_KEY), gemini (GEMINI_API_KEY),\n"+
			"openrouter (OPENROUTER_API_KEY), or ollama (local, no key)", err)
	}

	// Build the media context once; it is reused across REPL turns.
	mediaContext := buildMediaContext(cmd.Context(), aiFile)

	if request != "" {
		return handleAIRequest(cmd, provider, mediaContext, request)
	}
	return runAIRepl(cmd, provider, mediaContext)
}

// runAIRepl reads requests line by line until the user exits.
func runAIRepl(cmd *cobra.Command, provider ai.Provider, mediaContext string) error {
	ui.Infof("ffgo AI via %s — type a request, or 'exit' to quit.", ui.Bold(provider.Name()))
	if aiFile != "" {
		ui.Stepf("context: %s", ui.Bold(aiFile))
	}

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(ui.Err, "%s ", ui.BoldCyan("ffgo>"))
		if !sc.Scan() {
			fmt.Fprintln(ui.Err)
			return nil
		}
		line := strings.TrimSpace(sc.Text())
		switch strings.ToLower(line) {
		case "", "quit", "exit":
			return nil
		}
		if err := handleAIRequest(cmd, provider, mediaContext, line); err != nil {
			ui.Errorf("%s", err)
		}
	}
}

// handleAIRequest asks the model for a plan, shows it, and (unless dry-run)
// confirms and executes the resulting ffmpeg command.
func handleAIRequest(cmd *cobra.Command, provider ai.Provider, mediaContext, request string) error {
	ctx := cmd.Context()

	ui.Stepf("Asking %s…", provider.Name())
	text, err := provider.Complete(ctx, ai.SystemPrompt(mediaContext), request)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", provider.Name(), err)
	}

	plan, err := ai.ParsePlan(text)
	if err != nil {
		return err
	}

	ui.Heading("Proposed command")
	ui.Printf("%s\n", ui.CommandLine("ffmpeg", plan.FFmpegArgs))
	if plan.Explanation != "" {
		fmt.Fprintf(ui.Err, "\n%s\n", plan.Explanation)
	}
	for _, w := range plan.Warnings {
		if strings.TrimSpace(w) != "" {
			ui.Warnf("%s", w)
		}
	}

	// A dry run is a preview only: show the command and never prompt.
	if globals.DryRun {
		return nil
	}

	if plan.Dangerous {
		ui.Warnf("This command is flagged as potentially destructive.")
		if !confirm("Proceed anyway?") {
			ui.Infof("Skipped.")
			return nil
		}
	}

	if !confirm("Run this command?") {
		ui.Infof("Skipped.")
		return nil
	}

	eng, err := newEngine()
	if err != nil {
		return err
	}

	opts := baseRunOptions()
	opts.Args = plan.FFmpegArgs
	opts.Label = "Running"
	// Best-effort progress bar: probe the input the model referenced.
	if in := inputFromArgs(plan.FFmpegArgs); in != "" {
		if info, err := eng.Probe(ctx, in); err == nil {
			opts.Total = info.Duration()
		}
	}

	if err := eng.Run(ctx, opts); err != nil {
		return err
	}
	ui.Successf("Done.")
	return nil
}

// inputFromArgs returns the value following the first -i flag, if any.
func inputFromArgs(args []string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-i" {
			return args[i+1]
		}
	}
	return ""
}

// buildMediaContext probes file (if given) and renders a compact summary for
// the model. It returns an empty string when no usable file is available.
func buildMediaContext(ctx context.Context, file string) string {
	if file == "" {
		return ""
	}
	eng, err := newEngine()
	if err != nil {
		return ""
	}
	info, err := eng.Probe(ctx, file)
	if err != nil {
		return ""
	}
	return summarizeMedia(file, info)
}

// summarizeMedia formats a few key facts about a probed file into plain lines.
func summarizeMedia(file string, r *ffprobe.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- filename: %s\n", file)
	if d := r.Duration(); d > 0 {
		fmt.Fprintf(&b, "- duration: %s\n", ui.Duration(d))
	}
	if sz := r.Size(); sz > 0 {
		fmt.Fprintf(&b, "- size: %s\n", ui.Bytes(sz))
	}
	if v := r.VideoStream(); v != nil {
		desc := v.CodecName
		if res := v.Resolution(); res != "" {
			desc += ", " + res
		}
		if fps := v.FPS(); fps > 0 {
			desc += fmt.Sprintf(", %.3g fps", fps)
		}
		if v.PixFmt != "" {
			desc += ", " + v.PixFmt
		}
		fmt.Fprintf(&b, "- video: %s\n", desc)
	}
	for i, a := range r.AudioStreams() {
		desc := a.CodecName
		if a.Channels > 0 {
			desc += fmt.Sprintf(", %d ch", a.Channels)
		}
		if a.Language() != "und" {
			desc += ", " + a.Language()
		}
		fmt.Fprintf(&b, "- audio %d: %s\n", i+1, desc)
	}
	if len(r.SubtitleStreams()) > 0 {
		fmt.Fprintf(&b, "- subtitle tracks: %d\n", len(r.SubtitleStreams()))
	}
	return strings.TrimRight(b.String(), "\n")
}
