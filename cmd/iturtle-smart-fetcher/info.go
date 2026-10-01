package main

import (
	"fmt"
	"io"
	"os"
)

const (
	creatorName   = "Emmanuel Vinícius"
	creatorEmail  = "emmanuel.bergmann@icloud.com"
	creatorGitHub = "https://github.com/emmanuelviniciusdev"
)

func printVersion() {
	printVersionTo(os.Stdout)
}

func printVersionTo(w io.Writer) {
	fmt.Fprintf(w, "iTurtle %s\n", version)
}

func printAbout() {
	printAboutTo(os.Stdout)
}

func printAboutTo(w io.Writer) {
	fmt.Fprintf(w, `iTurtle %s downloads music from YouTube (single videos or playlists), converts
it with ffmpeg, and embeds ID3 metadata such as artist, album, year, track
number, and cover art. It can also pull album and track information from
MusicBrainz and process several albums from a YAML batch file.

Created by %s
Email: %s
GitHub: %s
`, version, creatorName, creatorEmail, creatorGitHub)
}
