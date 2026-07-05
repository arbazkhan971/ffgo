package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/arbazkhan971/ffgo/internal/explain"
	"github.com/arbazkhan971/ffgo/internal/ui"
)

var explainCmd = &cobra.Command{
	Use:     "explain <command>",
	Aliases: []string{"why", "decode"},
	Short:   "Explain an FFmpeg command in plain English",
	Long: "Decode a raw FFmpeg command line into a readable, flag-by-flag walkthrough.\n\n" +
		"Pass the command as one quoted string, or list its tokens after --.\n" +
		"A leading \"ffmpeg\"/\"ffprobe\" is stripped automatically; nothing is executed.",
	Example: "  ffgo explain \"ffmpeg -i in.mp4 -vf scale=1280:-1 -crf 23 out.mp4\"\n" +
		"  ffgo explain -- ffmpeg -i in.mov -c:v libx264 -c:a copy out.mp4",
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		tokens := collectTokens(args)
		tokens = stripLeadingBinary(tokens)

		if len(tokens) == 0 {
			ui.Infof("nothing to explain — pass an FFmpeg command")
			ui.Bullet("ffgo explain %q", "ffmpeg -i in.mp4 -crf 23 out.mp4")
			ui.Bullet("ffgo explain -- ffmpeg -i in.mp4 -crf 23 out.mp4")
			return nil
		}

		segments := explain.Explain(tokens)

		ui.Heading("What this FFmpeg command does")
		ui.Println("")
		for _, seg := range segments {
			ui.Printf("  %s\n", ui.Bold(seg.Token))
			ui.Printf("      %s\n", ui.Dim(seg.Detail))
		}
		return nil
	},
}

// collectTokens turns the CLI arguments into a flat token slice. A single arg
// is treated as a whole command line and shell-tokenised; multiple args (as
// produced by `-- ffmpeg ...`) are used verbatim.
func collectTokens(args []string) []string {
	if len(args) == 1 {
		return shellSplit(args[0])
	}
	return args
}

// stripLeadingBinary removes a leading "ffmpeg" or "ffprobe" token so users can
// paste a full command line unchanged.
func stripLeadingBinary(tokens []string) []string {
	if len(tokens) == 0 {
		return tokens
	}
	switch strings.ToLower(tokens[0]) {
	case "ffmpeg", "ffprobe":
		return tokens[1:]
	}
	return tokens
}

// shellSplit tokenises a command line with simple shell-like quoting: spaces
// separate tokens, while single and double quotes group text (including
// spaces) and a backslash escapes the next character. Unterminated quotes are
// tolerated and treated as running to the end of the input.
func shellSplit(s string) []string {
	var tokens []string
	var buf strings.Builder
	inToken := false

	const (
		none = iota
		single
		double
	)
	quote := none

	flush := func() {
		if inToken {
			tokens = append(tokens, buf.String())
			buf.Reset()
			inToken = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch quote {
		case single:
			if c == '\'' {
				quote = none
			} else {
				buf.WriteRune(c)
			}
		case double:
			if c == '"' {
				quote = none
			} else if c == '\\' && i+1 < len(runes) {
				i++
				buf.WriteRune(runes[i])
			} else {
				buf.WriteRune(c)
			}
		default:
			switch {
			case c == '\'':
				inToken = true
				quote = single
			case c == '"':
				inToken = true
				quote = double
			case c == '\\' && i+1 < len(runes):
				inToken = true
				i++
				buf.WriteRune(runes[i])
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
				flush()
			default:
				inToken = true
				buf.WriteRune(c)
			}
		}
	}
	flush()
	return tokens
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
