// Package cmd wires up ffgo's cobra command tree. Each subcommand lives in its
// own file and registers itself with rootCmd via an init function.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/buildinfo"
	"github.com/arbazkhan971/ffgo/internal/ffmpeg"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

// globals holds flags shared by every command, populated by cobra before Run.
var globals struct {
	DryRun      bool
	ShowCommand bool
	Yes         bool
	Quiet       bool
	Color       string // auto | always | never
}

var rootCmd = &cobra.Command{
	Use:   "ffgo",
	Short: "FFmpeg for humans — inspect, convert, compress and edit video with sane commands",
	Long: ui.Bold("ffgo") + " — FFmpeg for humans.\n\n" +
		"Beautiful, memorable commands for everyday video and audio tasks.\n" +
		"Every command shows the exact FFmpeg it runs; add --dry-run to preview it.",
	Example: "  ffgo inspect clip.mp4\n" +
		"  ffgo convert clip.mov --to mp4\n" +
		"  ffgo compress clip.mp4 --target 25mb\n" +
		"  ffgo trim clip.mp4 --from 00:01:20 --to 00:03:00\n" +
		"  ffgo gif clip.mp4 --from 10s --to 20s --width 480",
	SilenceUsage:  true,
	SilenceErrors: true,
	// Show help when invoked with no subcommand.
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command and translates errors into a clean exit.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		// Context cancellation (Ctrl-C) is a quiet exit, not a crash.
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(ui.Err, ui.Yellow("aborted"))
			os.Exit(130)
		}
		ui.Errorf("%s", err)
		os.Exit(1)
	}
}

func init() {
	version, commit, date := buildinfo.Resolve()
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
	rootCmd.SetVersionTemplate("ffgo {{.Version}}\n")

	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&globals.DryRun, "dry-run", false, "print the FFmpeg command without running it")
	pf.BoolVar(&globals.ShowCommand, "show-command", false, "print the FFmpeg command before running it")
	pf.BoolVarP(&globals.Yes, "yes", "y", false, "overwrite output files without asking")
	pf.BoolVarP(&globals.Quiet, "quiet", "q", false, "suppress progress output")
	pf.StringVar(&globals.Color, "color", "auto", "colorize output: auto, always or never")

	// Apply color mode as early as possible for every command.
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		switch globals.Color {
		case "always":
			ui.SetColor(true)
		case "never":
			ui.SetColor(false)
		}
	}
}

// newEngine constructs the ffmpeg engine, surfacing a friendly install hint on
// failure.
func newEngine() (*ffmpeg.Engine, error) {
	return ffmpeg.New()
}

// baseRunOptions returns RunOptions prefilled from the global flags. Commands
// fill in Args, Total and Label.
func baseRunOptions() ffmpeg.RunOptions {
	return ffmpeg.RunOptions{
		DryRun:      globals.DryRun,
		ShowCommand: globals.ShowCommand,
		Overwrite:   globals.Yes,
		Quiet:       globals.Quiet,
	}
}
