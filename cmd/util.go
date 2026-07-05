package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arbazkhan971/ffgo/internal/ui"
)

// replaceExt returns path with its extension swapped to newExt (leading dot
// optional). "clip.mov", "mp4" -> "clip.mp4".
func replaceExt(path, newExt string) string {
	newExt = strings.TrimPrefix(newExt, ".")
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "." + newExt
}

// suffixName inserts a suffix before the extension: ("clip.mp4","_small") ->
// "clip_small.mp4". If newExt is non-empty it also swaps the extension.
func suffixName(path, suffix, newExt string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	if newExt != "" {
		ext = "." + strings.TrimPrefix(newExt, ".")
	}
	return base + suffix + ext
}

// h264SafeExt returns an output extension able to hold H.264/AAC (what the
// compress command always produces): the source extension when it qualifies,
// otherwise "mp4". This prevents muxing H.264 into, say, a .webm container,
// which ffmpeg would reject.
func h264SafeExt(path string) string {
	switch ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")); ext {
	case "mp4", "mov", "m4v", "mkv", "ts", "flv":
		return ext
	default:
		return "mp4"
	}
}

// checkOverwrite errors when out already exists and the user has not opted in
// via -y. Dry runs never touch the filesystem, so the check is skipped.
func checkOverwrite(out string) error {
	if globals.DryRun || globals.Yes {
		return nil
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%q already exists; pass -y to overwrite", out)
	}
	return nil
}

// requireFile validates that an input path exists and is readable, returning a
// friendly error otherwise.
func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such file: %q", path)
		}
		return fmt.Errorf("cannot read %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory, not a file", path)
	}
	return nil
}

// confirm prompts the user for a yes/no answer on stderr, defaulting to no.
// It returns true automatically when -y is set.
func confirm(prompt string) bool {
	if globals.Yes {
		return true
	}
	fmt.Fprintf(ui.Err, "%s %s [y/N] ", ui.Yellow("?"), prompt)
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes"
}
