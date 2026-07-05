// Package ffbin locates the ffmpeg and ffprobe executables, honoring
// environment overrides and producing an actionable error (with per-platform
// install instructions) when they are missing.
package ffbin

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// NotFoundError is returned when a required binary cannot be located.
type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found on your system.\n\nffgo drives FFmpeg, so it needs %s installed and on your PATH.\nInstall it with:\n%s\n\nAlready installed elsewhere? Point ffgo at it with FFGO_%s=/path/to/%s",
		e.Name, e.Name, installHint(), upper(e.Name), e.Name)
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "  brew install ffmpeg"
	case "windows":
		return "  winget install Gyan.FFmpeg\n  (or)  scoop install ffmpeg"
	default:
		return "  Debian/Ubuntu:  sudo apt install ffmpeg\n  Fedora:         sudo dnf install ffmpeg\n  Arch:           sudo pacman -S ffmpeg"
	}
}

// Locate resolves an executable by name. It first consults the given
// environment variables (in order), then falls back to a PATH lookup. On
// failure it returns a *NotFoundError.
func Locate(name string, envOverrides ...string) (string, error) {
	for _, ev := range envOverrides {
		if p := os.Getenv(ev); p != "" {
			if abs, err := exec.LookPath(p); err == nil {
				return abs, nil
			}
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", &NotFoundError{Name: name}
}
