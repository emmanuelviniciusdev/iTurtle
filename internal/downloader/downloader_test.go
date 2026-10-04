package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsURL(t *testing.T) {
	if !isURL("https://example.com/video") {
		t.Fatalf("expected URL to be valid")
	}
	if isURL("not-a-url") {
		t.Fatalf("expected non URL to be invalid")
	}
}

func TestPrepareCoverWithURL(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			body := io.NopCloser(strings.NewReader("image-bytes"))
			return &http.Response{
				StatusCode: 200,
				Body:       body,
				Header:     http.Header{},
			}, nil
		}),
	}

	dl := New(nil, client)
	path, cleanup, err := dl.prepareCover(context.Background(), "https://example.com/cover.jpg")
	defer cleanup()
	if err != nil {
		t.Fatalf("prepareCover returned error: %v", err)
	}
	if path == "" {
		t.Fatalf("expected a downloaded cover path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected cover file to exist: %v", err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("unexpected cover content: %s", string(data))
	}
}

func TestBuildFFmpegArgsWithCoverAndMetadata(t *testing.T) {
	meta := Metadata{
		Title:       "Song",
		Artist:      "Artist",
		Album:       "Album",
		AlbumArtist: "Album Artist",
		Composer:    "Composer",
		Year:        "2024",
		Genre:       "Genre",
		Track:       "1",
		Comment:     "Note",
	}

	args := buildFFmpegArgs("in.mp3", "out.mp3", meta, "cover.jpg")
	argsJoined := strings.Join(args, " ")

	expected := []string{
		"cover.jpg",
		"artist=Artist",
		"album=Album",
		"title=Song",
		"attached_pic",
		"out.mp3",
	}

	for _, val := range expected {
		if !strings.Contains(argsJoined, val) {
			t.Fatalf("expected ffmpeg args to contain %q; args: %s", val, argsJoined)
		}
	}
}

func TestDownloadFlowCreatesAndTagsFiles(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeRunner{audioFormat: "mp3"}
	dl := New(runner, nil)

	cfg := Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
		Metadata: Metadata{
			Artist: "Tester",
			Album:  "Album",
			Year:   "2024",
		},
	}

	files, err := dl.Download(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".mp3") {
			t.Fatalf("expected mp3 extension for %s", file)
		}
		if _, err := os.Stat(filepath.Join(tempDir, file)); err != nil {
			t.Fatalf("expected file to exist: %v", err)
		}
	}

	ffmpegCalls := 0
	for _, call := range runner.calls {
		if call.name == "ffmpeg" {
			ffmpegCalls++
		}
	}
	if ffmpegCalls != len(files) {
		t.Fatalf("expected ffmpeg to be called %d times, got %d", len(files), ffmpegCalls)
	}
}

func TestDownloadRequiresURL(t *testing.T) {
	dl := New(&fakeRunner{}, nil)
	_, err := dl.Download(context.Background(), Config{OutputDir: t.TempDir()})
	if err == nil {
		t.Fatalf("expected error for missing URL")
	}
}

type fakeRunner struct {
	audioFormat string
	calls       []cmdCall
	ytErr       error
	skipWrite   bool
	existing    []string
}

type cmdCall struct {
	name string
	args []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, cmdCall{name: name, args: append([]string{}, args...)})

	switch name {
	case "yt-dlp":
		outDir := extractOutputDir(args)
		if outDir == "" {
			return "", errors.New("missing -o argument for yt-dlp")
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", err
		}
		var written []string
		if !f.skipWrite {
			for i := 1; i <= 2; i++ {
				path := filepath.Join(outDir, fmt.Sprintf("track%d.%s", i, f.audioFormat))
				if err := os.WriteFile(path, []byte("audio"), 0o644); err != nil {
					return "", err
				}
				written = append(written, path)
			}
		}
		written = append(written, f.existing...)
		writePrintLists(args, written)
		if f.ytErr != nil {
			return "ERROR: [youtube] abc123: Video unavailable", f.ytErr
		}
		return "ok", nil
	case "ffmpeg":
		if len(args) == 0 {
			return "", errors.New("ffmpeg missing args")
		}
		output := args[len(args)-1]
		if err := os.WriteFile(output, []byte("tagged"), 0o644); err != nil {
			return "", err
		}
		return "ok", nil
	default:
		return "", fmt.Errorf("unexpected command: %s", name)
	}
}

func extractOutputDir(args []string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-o" {
			return filepath.Dir(args[i+1])
		}
	}
	return ""
}

func writePrintLists(args []string, files []string) {
	var body strings.Builder
	for _, f := range files {
		body.WriteString(f)
		body.WriteByte('\n')
	}
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--print-to-file" && i+2 < len(args) {
			_ = os.WriteFile(args[i+2], []byte(body.String()), 0o644)
			i += 2
		}
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestExtractPlaylistIndex(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"1 - Track Title.mp3", 1},
		{"01 - Track Title.mp3", 1},
		{"12 - Some Song.mp3", 12},
		{"Track Title.mp3", 0},
		{"NoIndex.mp3", 0},
		{"/path/to/3 - Song.mp3", 3},
	}

	for _, tc := range tests {
		result := extractPlaylistIndex(tc.filename)
		if result != tc.expected {
			t.Errorf("extractPlaylistIndex(%q) = %d, expected %d", tc.filename, result, tc.expected)
		}
	}
}

func TestMergeTrackMetadata(t *testing.T) {
	album := AlbumMetadata{
		Title:       "Test Album",
		Artist:      "Album Artist",
		AlbumArtist: "Album Artist",
		Year:        "2024",
		Genre:       "Rock",
		TotalTracks: 10,
	}

	track := TrackMetadata{
		Position: 3,
		Title:    "Track Title",
		Composer: "Composer Name",
	}

	meta := MergeTrackMetadata(album, track, 3)

	if meta.Title != "Track Title" {
		t.Errorf("expected title %q, got %q", "Track Title", meta.Title)
	}
	if meta.Artist != "Album Artist" {
		t.Errorf("expected artist %q, got %q", "Album Artist", meta.Artist)
	}
	if meta.Album != "Test Album" {
		t.Errorf("expected album %q, got %q", "Test Album", meta.Album)
	}
	if meta.Year != "2024" {
		t.Errorf("expected year %q, got %q", "2024", meta.Year)
	}
	if meta.Composer != "Composer Name" {
		t.Errorf("expected composer %q, got %q", "Composer Name", meta.Composer)
	}
	if meta.Track != "3/10" {
		t.Errorf("expected track %q, got %q", "3/10", meta.Track)
	}
}

func TestMergeTrackMetadataWithTrackArtist(t *testing.T) {
	album := AlbumMetadata{
		Title:  "Compilation",
		Artist: "Various Artists",
	}

	track := TrackMetadata{
		Title:  "Guest Track",
		Artist: "Guest Artist",
	}

	meta := MergeTrackMetadata(album, track, 1)

	if meta.Artist != "Guest Artist" {
		t.Errorf("expected track artist to override, got %q", meta.Artist)
	}
}

func TestFormatTrackNumber(t *testing.T) {
	tests := []struct {
		track    int
		total    int
		expected string
	}{
		{1, 10, "1/10"},
		{5, 0, "5"},
		{12, 12, "12/12"},
	}

	for _, tc := range tests {
		result := formatTrackNumber(tc.track, tc.total)
		if result != tc.expected {
			t.Errorf("formatTrackNumber(%d, %d) = %q, expected %q", tc.track, tc.total, result, tc.expected)
		}
	}
}

func TestDownloadWithPlaylistMetadata(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeRunnerWithIndex{audioFormat: "mp3"}
	dl := New(runner, nil)

	cfg := Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
		PlaylistMetadata: &PlaylistMetadata{
			AlbumInfo: AlbumMetadata{
				Title:       "Test Album",
				Artist:      "Test Artist",
				Year:        "2024",
				TotalTracks: 2,
			},
			Tracks: []TrackMetadata{
				{Position: 1, Title: "First Track"},
				{Position: 2, Title: "Second Track"},
			},
		},
	}

	files, err := dl.Download(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Verify ffmpeg was called for each file with metadata
	ffmpegCalls := 0
	for _, call := range runner.calls {
		if call.name == "ffmpeg" {
			ffmpegCalls++
			// Check that metadata args are present
			argsStr := strings.Join(call.args, " ")
			if !strings.Contains(argsStr, "Test Album") {
				t.Errorf("expected ffmpeg args to contain album name")
			}
		}
	}

	if ffmpegCalls != 2 {
		t.Errorf("expected 2 ffmpeg calls, got %d", ffmpegCalls)
	}
}

// fakeRunnerWithIndex creates files with playlist index prefix
type fakeRunnerWithIndex struct {
	audioFormat string
	calls       []cmdCall
}

func (f *fakeRunnerWithIndex) Run(ctx context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, cmdCall{name: name, args: append([]string{}, args...)})

	switch name {
	case "yt-dlp":
		outDir := extractOutputDir(args)
		if outDir == "" {
			return "", errors.New("missing -o argument for yt-dlp")
		}
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", err
		}
		// Create files with playlist index prefix
		var written []string
		for i := 1; i <= 2; i++ {
			path := filepath.Join(outDir, fmt.Sprintf("%d - track%d.%s", i, i, f.audioFormat))
			if err := os.WriteFile(path, []byte("audio"), 0o644); err != nil {
				return "", err
			}
			written = append(written, path)
		}
		writePrintLists(args, written)
		return "ok", nil
	case "ffmpeg":
		if len(args) == 0 {
			return "", errors.New("ffmpeg missing args")
		}
		output := args[len(args)-1]
		if err := os.WriteFile(output, []byte("tagged"), 0o644); err != nil {
			return "", err
		}
		return "ok", nil
	default:
		return "", fmt.Errorf("unexpected command: %s", name)
	}
}

func TestBuildYtDlpArgsHighestQuality(t *testing.T) {
	args := buildYtDlpArgs("https://example.com/video", "/output", "mp3", "")
	argsStr := strings.Join(args, " ")

	// Check for highest quality flag
	if !strings.Contains(argsStr, "--audio-quality 0") {
		t.Errorf("expected --audio-quality 0 for highest quality, args: %s", argsStr)
	}

	// Check for playlist index in template
	if !strings.Contains(argsStr, "%(playlist_index") {
		t.Errorf("expected playlist_index in output template, args: %s", argsStr)
	}
}

func TestBuildYtDlpArgsIncludesJSRuntime(t *testing.T) {
	args := buildYtDlpArgs("https://example.com/video", "/output", "mp3", "deno:/usr/local/bin/deno")
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--js-runtimes deno:/usr/local/bin/deno") {
		t.Errorf("expected --js-runtimes in yt-dlp args, got: %s", argsStr)
	}
}

func TestBuildYtDlpArgsOmitsDeprecatedPreferFFmpeg(t *testing.T) {
	args := buildYtDlpArgs("https://example.com/video", "/output", "mp3", "")
	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, "--prefer-ffmpeg") {
		t.Errorf("did not expect deprecated --prefer-ffmpeg, args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--ignore-errors") {
		t.Errorf("expected --ignore-errors, args: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--ignore-no-formats-error") {
		t.Errorf("expected --ignore-no-formats-error, args: %s", argsStr)
	}
}

func TestDownloadTagsFilesWhenYtDlpReportsPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeRunner{
		audioFormat: "mp3",
		ytErr:       errors.New("exit status 1"),
	}
	dl := New(runner, nil)

	cfg := Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
		Metadata: Metadata{
			Artist: "Hannah Montana",
			Album:  "Hannah Montana",
			Year:   "2006",
		},
	}

	files, err := dl.Download(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected tagging to continue after partial yt-dlp failure, got: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}

	ffmpegCalls := 0
	for _, call := range runner.calls {
		if call.name == "ffmpeg" {
			ffmpegCalls++
		}
	}
	if ffmpegCalls != len(files) {
		t.Fatalf("expected ffmpeg to tag %d files, got %d calls", len(files), ffmpegCalls)
	}
}

func TestDownloadTagsExistingFilesListedByYtDlp(t *testing.T) {
	tempDir := t.TempDir()
	existing := filepath.Join(tempDir, "01 - The Best Of Both Worlds.mp3")
	if err := os.WriteFile(existing, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{
		audioFormat: "mp3",
		skipWrite:   true,
		existing:    []string{existing},
		ytErr:       errors.New("exit status 1"),
	}
	dl := New(runner, nil)

	cfg := Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
		Metadata: Metadata{
			Artist: "Hannah Montana",
			Album:  "Hannah Montana",
		},
	}

	files, err := dl.Download(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected existing downloaded files to be tagged, got: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d (%v)", len(files), files)
	}

	ffmpegCalls := 0
	for _, call := range runner.calls {
		if call.name == "ffmpeg" {
			ffmpegCalls++
		}
	}
	if ffmpegCalls != 1 {
		t.Fatalf("expected 1 ffmpeg call, got %d", ffmpegCalls)
	}
}

func TestDownloadFailsWhenYtDlpFailsAndNoFilesExist(t *testing.T) {
	tempDir := t.TempDir()
	runner := &fakeRunner{
		audioFormat: "mp3",
		skipWrite:   true,
		ytErr:       errors.New("exit status 1"),
	}
	dl := New(runner, nil)

	_, err := dl.Download(context.Background(), Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
	})
	if err == nil {
		t.Fatal("expected error when yt-dlp fails and no files were downloaded")
	}
}

func TestParseYtDlpDestinations(t *testing.T) {
	output := `
[ExtractAudio] Destination: /tmp/music/01 - Song.mp3
[download] Downloading item 2 of 2
[download] /tmp/music/02 - Other.webm has already been downloaded
`
	got := parseYtDlpDestinations(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 paths, got %v", got)
	}
	if got[0] != "/tmp/music/01 - Song.mp3" {
		t.Errorf("unexpected first path: %q", got[0])
	}
	if got[1] != "/tmp/music/02 - Other.webm" {
		t.Errorf("unexpected second path: %q", got[1])
	}
}

func TestParseYtDlpProgress(t *testing.T) {
	var state ytProgressState

	got := parseYtDlpProgress("[download] Downloading item 2 of 10", &state)
	if got != "Downloading 2/10" {
		t.Fatalf("item line: got %q", got)
	}

	got = parseYtDlpProgress("[download] Destination: /tmp/music/02 - Song.webm", &state)
	if got != "Downloading 2/10: 02 - Song.webm" {
		t.Fatalf("destination line: got %q", got)
	}

	got = parseYtDlpProgress("[download]  45.2% of  4.50MiB at  1.23MiB/s ETA 00:03", &state)
	if got != "Downloading 2/10 45.2% of 4.50MiB: 02 - Song.webm" {
		t.Fatalf("percent line: got %q", got)
	}

	got = parseYtDlpProgress("[ExtractAudio] Destination: /tmp/music/02 - Song.mp3", &state)
	if got != "Converting 2/10: 02 - Song.mp3" {
		t.Fatalf("extract line: got %q", got)
	}

	if msg := parseYtDlpProgress("[youtube] abc: Downloading webpage", &state); msg != "" {
		t.Fatalf("expected ignored youtube line, got %q", msg)
	}
}

func TestPrintStartStaysOnSameLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)
	p.PrintStart("Fetching audio")
	got := buf.String()
	if !strings.HasPrefix(got, "\n🐢 Fetching audio...") {
		t.Fatalf("unexpected start line: %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatal("PrintStart should not end with a newline; the turtle overwrites this line")
	}
}

func TestPrintFrameClearsRestOfLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)
	p.printFrame("Fetching audio")
	got := buf.String()
	if !strings.Contains(got, "\033[K") {
		t.Fatalf("expected ANSI clear to end of line, got %q", got)
	}
	if strings.Contains(got, strings.Repeat(" ", 40)) {
		t.Fatal("progress frames should not pad with spaces that wrap the terminal")
	}
}

func TestStartLiveWritesTurtle(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)
	live := p.startLive("Fetching audio")
	live.SetMessage("Downloading 1/2 45.2% of 4.50MiB")
	time.Sleep(400 * time.Millisecond)
	live.Stop()

	got := buf.String()
	if !strings.Contains(got, "🐢") {
		t.Fatalf("expected turtle animation, got %q", got)
	}
	if !strings.Contains(got, "Downloading 1/2") {
		t.Fatalf("expected live progress message, got %q", got)
	}
}

func TestDownloadAnimatesTurtleDuringYtDlp(t *testing.T) {
	tempDir := t.TempDir()
	runner := &streamingFakeRunner{
		fakeRunner: fakeRunner{audioFormat: "mp3"},
		lines: []string{
			"[download] Downloading item 1 of 2",
			"[download]  12.0% of  3.00MiB at  500.00KiB/s ETA 00:05",
		},
	}
	var buf bytes.Buffer
	dl := New(runner, nil)
	dl.progress = NewProgressPrinter(&buf)

	_, err := dl.Download(context.Background(), Config{
		URL:         "https://example.com/playlist",
		OutputDir:   tempDir,
		AudioFormat: "mp3",
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "🐢") {
		t.Fatalf("expected turtle during download, got %q", got)
	}
	if !strings.Contains(got, "Downloading 1/2") {
		t.Fatalf("expected download animation text, got %q", got)
	}
}

type streamingFakeRunner struct {
	fakeRunner
	lines []string
}

func (f *streamingFakeRunner) RunStreaming(ctx context.Context, onLine func(string), name string, args ...string) (string, error) {
	if name == "yt-dlp" {
		for _, line := range f.lines {
			if onLine != nil {
				onLine(line)
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return f.Run(ctx, name, args...)
}

func TestGetTrackMetadataMatchesByTitleWhenIndexIsMissing(t *testing.T) {
	dl := New(nil, nil)
	pm := &PlaylistMetadata{
		AlbumInfo: AlbumMetadata{
			Title:       "Hannah Montana",
			Artist:      "Hannah Montana",
			Year:        "2006",
			TotalTracks: 7,
		},
		Tracks: []TrackMetadata{
			{Position: 1, Title: "The Best of Both Worlds"},
			{Position: 5, Title: "If We Were a Movie"},
		},
	}

	meta := dl.getTrackMetadata(pm, "10 - Hannah Montana - If We Were A Movie.mp3", 0)
	if meta.Title != "If We Were a Movie" {
		t.Errorf("expected title match, got %q", meta.Title)
	}
	if meta.Album != "Hannah Montana" {
		t.Errorf("expected album tags, got %q", meta.Album)
	}
	if meta.Track != "5/7" {
		t.Errorf("expected track 5/7, got %q", meta.Track)
	}
}

func TestGetTrackMetadataUsesFilenameTitleForExtraTracks(t *testing.T) {
	dl := New(nil, nil)
	pm := &PlaylistMetadata{
		AlbumInfo: AlbumMetadata{
			Title:       "Hannah Montana",
			Artist:      "Hannah Montana",
			TotalTracks: 7,
		},
		Tracks: []TrackMetadata{
			{Position: 1, Title: "The Best of Both Worlds"},
		},
	}

	meta := dl.getTrackMetadata(pm, "10 - Jesse McCartney - She's No You.mp3", 0)
	if meta.Title != "Jesse McCartney - She's No You" {
		t.Errorf("expected filename title, got %q", meta.Title)
	}
	if meta.Artist != "Hannah Montana" {
		t.Errorf("expected album artist, got %q", meta.Artist)
	}
}
