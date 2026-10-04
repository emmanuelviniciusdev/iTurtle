package main

import (
	"bytes"
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
		"about       What iTurtle does and who created it",
		"version     Print the current version",
		"iTurtle download -url",
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

func TestRunDownloadExampleConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"download", "-example-config"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "albums:") {
		t.Fatalf("example config missing albums:\n%s", got)
	}
	if !strings.Contains(got, "musicbrainz_id") {
		t.Fatalf("example config missing musicbrainz_id:\n%s", got)
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
