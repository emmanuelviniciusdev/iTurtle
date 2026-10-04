package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureUsesOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binaries
	ytdlpPath := filepath.Join(tmpDir, "yt-dlp")
	ffmpegPath := filepath.Join(tmpDir, "ffmpeg")
	denoPath := filepath.Join(tmpDir, "deno")

	if err := os.WriteFile(ytdlpPath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ffmpegPath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(denoPath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	m := New()
	paths, err := m.Ensure(Options{
		YtDLPPath:     ytdlpPath,
		FFmpegPath:    ffmpegPath,
		JSRuntimePath: denoPath,
	})
	if err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	if paths.YtDLP != ytdlpPath {
		t.Errorf("expected yt-dlp path %s, got %s", ytdlpPath, paths.YtDLP)
	}
	if paths.FFmpeg != ffmpegPath {
		t.Errorf("expected ffmpeg path %s, got %s", ffmpegPath, paths.FFmpeg)
	}
	wantJS := "deno:" + denoPath
	if paths.JSRuntime != wantJS {
		t.Errorf("expected JS runtime %s, got %s", wantJS, paths.JSRuntime)
	}
}

func TestEnsureUsesNodeRuntimeOverride(t *testing.T) {
	tmpDir := t.TempDir()
	ytdlpPath := filepath.Join(tmpDir, "yt-dlp")
	ffmpegPath := filepath.Join(tmpDir, "ffmpeg")
	nodePath := filepath.Join(tmpDir, "node")
	for _, path := range []string{ytdlpPath, ffmpegPath, nodePath} {
		if err := os.WriteFile(path, []byte("fake"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	m := New()
	paths, err := m.Ensure(Options{
		YtDLPPath:     ytdlpPath,
		FFmpegPath:    ffmpegPath,
		JSRuntimePath: nodePath,
	})
	if err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}
	wantJS := "node:" + nodePath
	if paths.JSRuntime != wantJS {
		t.Errorf("expected JS runtime %s, got %s", wantJS, paths.JSRuntime)
	}
}

func TestEnsureFailsWithUnknownJSRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	ytdlpPath := filepath.Join(tmpDir, "yt-dlp")
	ffmpegPath := filepath.Join(tmpDir, "ffmpeg")
	pythonPath := filepath.Join(tmpDir, "python")
	for _, path := range []string{ytdlpPath, ffmpegPath, pythonPath} {
		if err := os.WriteFile(path, []byte("fake"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	m := New()
	_, err := m.Ensure(Options{
		YtDLPPath:     ytdlpPath,
		FFmpegPath:    ffmpegPath,
		JSRuntimePath: pythonPath,
	})
	if err == nil {
		t.Fatal("expected error for unrecognized JS runtime")
	}
}

func TestEnsureFailsWithInvalidPaths(t *testing.T) {
	m := New()
	_, err := m.Ensure(Options{
		YtDLPPath:  "/nonexistent/yt-dlp",
		FFmpegPath: "/nonexistent/ffmpeg",
	})
	if err == nil {
		t.Error("expected error for nonexistent paths")
	}
}

func TestEnsureSearchesPATH(t *testing.T) {
	// This test will only pass if yt-dlp, ffmpeg, and a JS runtime are installed
	// Skip if not available
	m := New()
	paths, err := m.Ensure(Options{})

	// If tools aren't on PATH, this should fail with a helpful message
	if err != nil {
		t.Logf("Tools not found on PATH (expected if not installed): %v", err)
		t.Skip("yt-dlp, ffmpeg, or a JS runtime not on PATH, skipping")
	}

	// If we got here, tools were found
	if paths.YtDLP == "" {
		t.Error("yt-dlp path should not be empty")
	}
	if paths.FFmpeg == "" {
		t.Error("ffmpeg path should not be empty")
	}
	if paths.JSRuntime == "" {
		t.Error("JS runtime should not be empty")
	}
}
