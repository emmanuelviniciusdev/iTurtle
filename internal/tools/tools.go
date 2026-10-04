package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options controls how external tool binaries are resolved.
type Options struct {
	YtDLPPath     string
	FFmpegPath    string
	JSRuntimePath string
}

// Paths contains resolved executable paths for required tools.
type Paths struct {
	YtDLP     string
	FFmpeg    string
	JSRuntime string // yt-dlp --js-runtimes value, e.g. "deno:/usr/local/bin/deno"
}

// Manager resolves yt-dlp, ffmpeg, and a JavaScript runtime.
type Manager struct{}

// New returns a new Manager.
func New() *Manager {
	return &Manager{}
}

// Ensure locates yt-dlp, ffmpeg, and a JS runtime using explicit paths or PATH.
func (m *Manager) Ensure(opts Options) (Paths, error) {
	ytdlp, err := resolveTool("yt-dlp", opts.YtDLPPath)
	if err != nil {
		return Paths{}, err
	}

	ffmpeg, err := resolveTool("ffmpeg", opts.FFmpegPath)
	if err != nil {
		return Paths{}, err
	}

	jsRuntime, err := resolveJSRuntime(opts.JSRuntimePath)
	if err != nil {
		return Paths{}, err
	}

	return Paths{YtDLP: ytdlp, FFmpeg: ffmpeg, JSRuntime: jsRuntime}, nil
}

// resolveTool finds a tool binary using the explicit path or system PATH.
func resolveTool(name, explicitPath string) (string, error) {
	// If explicit path provided, use it
	if strings.TrimSpace(explicitPath) != "" {
		if isExecutable(explicitPath) {
			return explicitPath, nil
		}
		return "", fmt.Errorf("%s not found or not executable: %s", name, explicitPath)
	}

	// Try to find in system PATH
	path, err := exec.LookPath(name)
	if err == nil && isExecutable(path) {
		return path, nil
	}

	return "", fmt.Errorf("%s not found on PATH. Install it or use -%s-path to specify location", name, name)
}

func resolveJSRuntime(explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		if !isExecutable(explicitPath) {
			return "", fmt.Errorf("javascript runtime not found or not executable: %s", explicitPath)
		}
		spec, err := jsRuntimeSpec(explicitPath)
		if err != nil {
			return "", err
		}
		return spec, nil
	}

	for _, name := range []string{"deno", "node"} {
		path, err := exec.LookPath(name)
		if err == nil && isExecutable(path) {
			spec, err := jsRuntimeSpec(path)
			if err != nil {
				return "", err
			}
			return spec, nil
		}
	}

	return "", fmt.Errorf("no JavaScript runtime found (deno or node). YouTube downloads require one; re-run the iTurtle installer, or install Deno from https://deno.com")
}

func jsRuntimeSpec(path string) (string, error) {
	name := runtimeNameFromPath(path)
	if name == "" {
		return "", fmt.Errorf("unrecognized JavaScript runtime %s (expected deno or node)", path)
	}
	return name + ":" + path, nil
}

func runtimeNameFromPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	base = strings.TrimSuffix(base, ".exe")
	switch base {
	case "deno", "node":
		return base
	default:
		return ""
	}
}

// isExecutable checks if a path points to an executable file.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
