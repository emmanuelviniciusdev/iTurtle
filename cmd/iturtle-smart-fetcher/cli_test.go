package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iturtle-smart-fetcher/internal/downloader"
)

func TestRunVersion(t *testing.T) {
	version = "1.2.3"
	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "iTurtle 1.2.3\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunAbout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"about"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := stdout.String()
	for _, want := range []string{"YouTube", "ffmpeg", "MusicBrainz", "Emmanuel Vinícius"} {
		if !strings.Contains(got, want) {
			t.Errorf("about text missing %q\n%s", want, got)
		}
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := stdout.String()
	for _, want := range []string{
		"iTurtle <command> [parameters]",
		"download    Download and tag music from YouTube",
		"generate    Generate sample files (currently only albums.yml)",
		"about       About iTurtle",
		"version     Print the current version",
		"iTurtle download -url",
		"iTurtle generate albums.yml",
		"iTurtle version",
		"iTurtle about",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help text missing %q\n%s", want, got)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	for _, old := range []string{
		"What iTurtle does and who created it",
		"-example-config",
		"albums.yaml",
	} {
		if strings.Contains(got, old) {
			t.Errorf("help still contains old text %q\n%s", old, got)
		}
	}
}

func TestRunNoCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "iTurtle <command> [parameters]") {
		t.Fatalf("stderr missing usage: %s", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"not-a-command"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown command: not-a-command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunDownloadRequiresURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"download"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "iTurtle download") {
		t.Fatalf("stderr missing download usage: %s", got)
	}
	if !strings.Contains(got, "-url") {
		t.Fatalf("stderr missing -url parameter: %s", got)
	}
}

func TestRunGenerate(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate", "albums.yml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "albums.yml") {
		t.Fatalf("stdout missing generated name: %q", stdout.String())
	}

	got, err := os.ReadFile("albums.yml")
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	body := string(got)
	if !strings.Contains(body, "albums:") {
		t.Fatalf("example config missing albums:\n%s", body)
	}
	if !strings.Contains(body, "musicbrainz_id") {
		t.Fatalf("example config missing musicbrainz_id:\n%s", body)
	}
}

func TestRunGenerateRequiresName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "iTurtle generate <name>") {
		t.Fatalf("stderr missing generate usage: %s", stderr.String())
	}
}

func TestRunGenerateUnknownTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate", "not-a-thing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown generate target "not-a-thing"`) {
		t.Fatalf("stderr missing unknown target: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "albums.yml") {
		t.Fatalf("stderr missing available targets: %s", stderr.String())
	}
}

func TestRunGenerateExistingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("albums.yml", []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate", "albums.yml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr missing exists error: %s", stderr.String())
	}
	got, err := os.ReadFile("albums.yml")
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(got) != "keep me" {
		t.Fatalf("existing file was overwritten: %q", got)
	}
}

func TestRunGenerateHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"generate", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "iTurtle generate <name>") {
		t.Fatalf("generate help missing usage: %s", got)
	}
	if !strings.Contains(got, "albums.yml") {
		t.Fatalf("generate help missing albums.yml: %s", got)
	}
}

func TestRunDownloadHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"download", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "iTurtle download [parameters]") {
		t.Fatalf("download help missing usage: %s", got)
	}
	if !strings.Contains(got, "-url") {
		t.Fatalf("download help missing -url: %s", got)
	}
	if !strings.Contains(got, "-cookies-from-browser") {
		t.Fatalf("download help missing -cookies-from-browser: %s", got)
	}
	if !strings.Contains(got, "-cookies") {
		t.Fatalf("download help missing -cookies: %s", got)
	}
}

func TestRunDownloadRejectsBothCookieFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"download",
		"-url", "https://example.com",
		"-cookies", "cookies.txt",
		"-cookies-from-browser", "safari",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not both") {
		t.Fatalf("stderr missing mutual-exclusion message: %s", stderr.String())
	}
}

func TestRunDownloadMissingCookiesFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"download",
		"-url", "https://example.com",
		"-cookies", filepath.Join(t.TempDir(), "missing-cookies.txt"),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cookies file") {
		t.Fatalf("stderr missing cookies file error: %s", stderr.String())
	}
}

func TestPrintMusicBrainzFound(t *testing.T) {
	var buf bytes.Buffer
	printMusicBrainzFound(&buf, &downloader.PlaylistMetadata{
		AlbumInfo: downloader.AlbumMetadata{
			Artist: "Paramore",
			Title:  "Brand New Eyes",
			Year:   "2009",
		},
		Tracks: make([]downloader.TrackMetadata, 14),
	})
	got := buf.String()
	want := "🎵 Found on MusicBrainz: Paramore - Brand New Eyes (2009)\n   14 tracks\n\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
