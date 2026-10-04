package downloader

import "fmt"

// Metadata holds tags to embed into downloaded audio files.
type Metadata struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Composer    string
	Year        string
	Genre       string
	Track       string
	Comment     string
}

// TrackMetadata holds per-track metadata for playlist downloads.
type TrackMetadata struct {
	Position    int    // Track position in the playlist/album
	Title       string // Track title
	Duration    string // Track duration (e.g., "3:45")
	Artist      string // Track artist (if different from album artist)
	Composer    string // Track composer
	ISRC        string // International Standard Recording Code
	DiscNumber  int    // Disc number for multi-disc albums
	TotalDiscs  int    // Total number of discs
	Comment     string // Per-track comment
}

// AlbumMetadata holds album-level metadata.
type AlbumMetadata struct {
	Title       string // Album title
	Artist      string // Album artist
	AlbumArtist string // Album artist (for various artists compilations)
	Year        string // Release year
	Genre       string // Genre
	Label       string // Record label
	CatalogNum  string // Catalog number
	Country     string // Release country
	TotalTracks int    // Total number of tracks
	CoverURL    string // URL to album cover art
	CoverPath   string // Local path to cover art
	Comment     string // Album comment
}

// PlaylistMetadata combines album-level and per-track metadata.
type PlaylistMetadata struct {
	AlbumInfo AlbumMetadata
	Tracks    []TrackMetadata
}

// Config defines the parameters for a download run.
type Config struct {
	URL              string
	OutputDir        string
	Cover            string // Local file path or URL
	AudioFormat      string
	YtDLPPath        string
	FFmpegPath       string
	JSRuntime        string // yt-dlp --js-runtimes value, e.g. "deno:/usr/local/bin/deno"
	Metadata         Metadata
	PlaylistMetadata *PlaylistMetadata // Optional per-track metadata for playlists
}

// MergeTrackMetadata creates a Metadata struct by merging album-level and track-level data.
// Track-level values take precedence over album-level values when both are set.
func MergeTrackMetadata(album AlbumMetadata, track TrackMetadata, position int) Metadata {
	meta := Metadata{
		Album:       album.Title,
		AlbumArtist: album.AlbumArtist,
		Year:        album.Year,
		Genre:       album.Genre,
		Comment:     album.Comment,
	}

	// Use album artist if album artist field is empty
	if meta.AlbumArtist == "" {
		meta.AlbumArtist = album.Artist
	}

	// Track-level overrides
	if track.Title != "" {
		meta.Title = track.Title
	}

	// Use track artist if specified, otherwise use album artist
	if track.Artist != "" {
		meta.Artist = track.Artist
	} else {
		meta.Artist = album.Artist
	}

	if track.Composer != "" {
		meta.Composer = track.Composer
	}

	if track.Comment != "" {
		meta.Comment = track.Comment
	}

	// Track number: use position from track if set, otherwise use provided position
	trackNum := position
	if track.Position > 0 {
		trackNum = track.Position
	}
	if album.TotalTracks > 0 {
		meta.Track = formatTrackNumber(trackNum, album.TotalTracks)
	} else if trackNum > 0 {
		meta.Track = formatTrackNumber(trackNum, 0)
	}

	return meta
}

// formatTrackNumber formats track number, optionally with total.
func formatTrackNumber(track, total int) string {
	if total > 0 {
		return fmt.Sprintf("%d/%d", track, total)
	}
	return fmt.Sprintf("%d", track)
}
