package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/buildinfo"
	"github.com/arbazkhan971/ffgo/internal/ffbin"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print detailed version information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		v, c, d := buildinfo.Resolve()
		fmt.Fprintf(ui.Out, "%s %s\n", ui.Bold("ffgo"), ui.Cyan(v))
		fmt.Fprintf(ui.Out, "  commit   %s\n", c)
		fmt.Fprintf(ui.Out, "  built    %s\n", d)
		fmt.Fprintf(ui.Out, "  go       %s\n", runtime.Version())
		fmt.Fprintf(ui.Out, "  platform %s/%s\n", runtime.GOOS, runtime.GOARCH)

		if path, err := ffbin.Locate("ffmpeg", "FFGO_FFMPEG", "FFMPEG_PATH"); err == nil {
			fmt.Fprintf(ui.Out, "  ffmpeg   %s\n", ui.Green(path))
		} else {
			fmt.Fprintf(ui.Out, "  ffmpeg   %s\n", ui.Red("not found"))
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
