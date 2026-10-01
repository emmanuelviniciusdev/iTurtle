package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	version = "1.2.3"
	var buf bytes.Buffer
	printVersionTo(&buf)
	got := buf.String()
	if got != "iTurtle 1.2.3\n" {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestPrintAbout(t *testing.T) {
	var buf bytes.Buffer
	printAboutTo(&buf)
	got := buf.String()
	for _, want := range []string{
		"YouTube",
		"ffmpeg",
		"MusicBrainz",
		"Emmanuel Vinícius",
		"emmanuel.bergmann@icloud.com",
		"https://github.com/emmanuelviniciusdev",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("about text missing %q\n%s", want, got)
		}
	}
}
