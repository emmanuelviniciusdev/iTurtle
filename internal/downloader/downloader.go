package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Downloader orchestrates fetching audio with yt-dlp and tagging it with ffmpeg.
type Downloader struct {
	runner     Runner
	httpClient *http.Client
	progress   *ProgressPrinter
}

// New creates a Downloader with sensible defaults for runner and HTTP client.
func New(r Runner, client *http.Client) *Downloader {
	if r == nil {
		r = ExecRunner{}
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Downloader{
		runner:     r,
		httpClient: client,
		progress:   NewProgressPrinter(os.Stdout),
	}
}

// Download fetches audio from the provided URL, embeds metadata and cover art,
// and returns the relative paths of the new files.
func (d *Downloader) Download(ctx context.Context, cfg Config) ([]string, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("url is required")
	}

	if cfg.OutputDir == "" {
		cfg.OutputDir = "."
	}

	format := strings.ToLower(strings.TrimSpace(cfg.AudioFormat))
	if format == "" {
		format = "mp3"
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	before, err := snapshotFiles(cfg.OutputDir, format)
	if err != nil {
		return nil, err
	}

	ytCmd := strings.TrimSpace(cfg.YtDLPPath)
	if ytCmd == "" {
		ytCmd = "yt-dlp"
	}

	d.progress.PrintSection("Downloading from YouTube")
	d.progress.PrintStart("Fetching audio")

	movedList, err := os.CreateTemp("", "iturtle-ytdlp-moved-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create yt-dlp file list: %w", err)
	}
	movedPath := movedList.Name()
	_ = movedList.Close()
	defer os.Remove(movedPath)

	plannedList, err := os.CreateTemp("", "iturtle-ytdlp-planned-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create yt-dlp file list: %w", err)
	}
	plannedPath := plannedList.Name()
	_ = plannedList.Close()
	defer os.Remove(plannedPath)

	ytArgs := withPrintToFile(buildYtDlpArgs(cfg.URL, cfg.OutputDir, format, cfg.JSRuntime), movedPath, plannedPath)
	ytOutput, ytErr := d.runYtDlp(ctx, ytCmd, ytArgs)

	after, err := snapshotFiles(cfg.OutputDir, format)
	if err != nil {
		return nil, err
	}

	files := collectDownloadedAudio(cfg.OutputDir, format, ytOutput, movedPath, plannedPath, diffFiles(before, after))
	if len(files) == 0 {
		d.progress.PrintError("Download failed")
		if ytErr != nil {
			return nil, ytErr
		}
		d.progress.PrintError("No new audio files found")
		return nil, errors.New("no new audio files found after download")
	}
	if ytErr != nil {
		d.progress.PrintWarning("Some playlist items could not be downloaded; tagging the files that succeeded")
	}

	d.progress.PrintComplete("Downloaded", len(files))
	for _, file := range files {
		d.progress.PrintFile(file)
	}

	// Determine cover path - check playlist metadata first, then config
	coverSource := cfg.Cover
	if cfg.PlaylistMetadata != nil && cfg.PlaylistMetadata.AlbumInfo.CoverURL != "" {
		coverSource = cfg.PlaylistMetadata.AlbumInfo.CoverURL
	}
	if cfg.PlaylistMetadata != nil && cfg.PlaylistMetadata.AlbumInfo.CoverPath != "" {
		coverSource = cfg.PlaylistMetadata.AlbumInfo.CoverPath
	}

	coverPath, cleanup, err := d.prepareCover(ctx, coverSource)
	if err != nil {
		d.progress.PrintWarning(fmt.Sprintf("Cover preparation failed: %v", err))
	}
	defer cleanup()

	// Check if any metadata or cover is being applied
	hasMetadata := cfg.Metadata.Title != "" || cfg.Metadata.Artist != "" ||
		cfg.Metadata.Album != "" || cfg.Metadata.AlbumArtist != "" ||
		cfg.Metadata.Composer != "" || cfg.Metadata.Year != "" ||
		cfg.Metadata.Genre != "" || cfg.Metadata.Track != "" ||
		cfg.Metadata.Comment != "" || coverPath != "" ||
		cfg.PlaylistMetadata != nil

	if hasMetadata {
		d.progress.PrintSection("Applying Metadata")
		d.progress.PrintStart("Embedding ID3 tags and cover art")

		ffmpegCmd := strings.TrimSpace(cfg.FFmpegPath)
		if ffmpegCmd == "" {
			ffmpegCmd = "ffmpeg"
		}

		for i, file := range files {
			d.progress.PrintProgress(fmt.Sprintf("Tagging %d/%d: %s", i+1, len(files), filepath.Base(file)))

			// Determine metadata for this file
			meta := cfg.Metadata
			if cfg.PlaylistMetadata != nil {
				meta = d.getTrackMetadata(cfg.PlaylistMetadata, file, i)
			}

			if err := d.applyMetadata(ctx, ffmpegCmd, filepath.Join(cfg.OutputDir, file), coverPath, meta); err != nil {
				d.progress.ClearLine()
				d.progress.PrintError(fmt.Sprintf("Failed to tag %s: %v", file, err))
				return files, err
			}
		}
		d.progress.ClearLine()
		d.progress.PrintComplete("Metadata applied to all files", len(files))
	}

	d.progress.PrintSection("Complete")
	fmt.Fprintf(os.Stdout, "🎵 Successfully processed %d file(s) 🎵\n\n", len(files))

	return files, nil
}

func (d *Downloader) runYtDlp(ctx context.Context, ytCmd string, ytArgs []string) (string, error) {
	live := d.progress.startLive("Fetching audio")
	defer live.Stop()

	var state ytProgressState
	onLine := func(line string) {
		if msg := parseYtDlpProgress(line, &state); msg != "" {
			live.SetMessage(msg)
		}
	}

	if streamer, ok := d.runner.(outputStreamer); ok {
		return streamer.RunStreaming(ctx, onLine, ytCmd, ytArgs...)
	}
	return d.runner.Run(ctx, ytCmd, ytArgs...)
}

func buildYtDlpArgs(url, outputDir, format, jsRuntime string) []string {
	// Use playlist index in filename to ensure proper ordering for per-track metadata
	template := filepath.Join(outputDir, "%(playlist_index|0)s - %(title)s.%(ext)s")
	args := []string{
		"--extract-audio",
		"--audio-format", format,
		"--audio-quality", "0", // Highest quality (0 = best, 10 = worst for VBR)
		"--yes-playlist",
		"--ignore-errors",
		"--ignore-no-formats-error",
		"--no-continue",
		"--newline",
	}
	if spec := strings.TrimSpace(jsRuntime); spec != "" {
		args = append(args, "--js-runtimes", spec)
	}
	return append(args, "-o", template, url)
}

func withPrintToFile(args []string, movedPath, plannedPath string) []string {
	if len(args) == 0 {
		return args
	}
	url := args[len(args)-1]
	head := args[:len(args)-1]
	if strings.TrimSpace(movedPath) != "" {
		head = append(head, "--print-to-file", "after_move:%(filepath)s", movedPath)
	}
	if strings.TrimSpace(plannedPath) != "" {
		head = append(head, "--print-to-file", "video:%(filepath)s", plannedPath)
	}
	return append(head, url)
}

func collectDownloadedAudio(outputDir, format, ytOutput, movedPath, plannedPath string, newFiles []string) []string {
	seen := map[string]struct{}{}
	var files []string
	add := func(rel string) {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			return
		}
		rel = filepath.Clean(rel)
		if _, ok := seen[rel]; ok {
			return
		}
		if _, err := os.Stat(filepath.Join(outputDir, rel)); err != nil {
			return
		}
		seen[rel] = struct{}{}
		files = append(files, rel)
	}

	for _, f := range newFiles {
		add(f)
	}
	for _, p := range readListFile(movedPath) {
		add(resolveAudioRel(p, outputDir, format))
	}
	for _, p := range readListFile(plannedPath) {
		add(resolveAudioRel(p, outputDir, format))
	}
	for _, p := range parseYtDlpDestinations(ytOutput) {
		add(resolveAudioRel(p, outputDir, format))
	}
	sort.Strings(files)
	return files
}

func readListFile(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

type ytProgressState struct {
	item    string
	percent string
	file    string
}

func parseYtDlpProgress(line string, state *ytProgressState) string {
	line = strings.TrimSpace(line)
	if line == "" || state == nil {
		return ""
	}

	if rest, ok := strings.CutPrefix(line, "[download] Downloading item "); ok {
		state.item = strings.ReplaceAll(strings.TrimSpace(rest), " of ", "/")
		state.percent = ""
		return state.message("Downloading")
	}
	if rest, ok := strings.CutPrefix(line, "[download] Destination: "); ok {
		state.file = filepath.Base(strings.TrimSpace(rest))
		return state.message("Downloading")
	}
	if rest, ok := strings.CutPrefix(line, "[ExtractAudio] Destination: "); ok {
		state.file = filepath.Base(strings.TrimSpace(rest))
		state.percent = ""
		return state.message("Converting")
	}
	if strings.HasPrefix(line, "[download]") && strings.Contains(line, "%") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "[download]"))
		if chunk := progressPercentChunk(rest); chunk != "" {
			state.percent = chunk
			return state.message("Downloading")
		}
	}
	return ""
}

func progressPercentChunk(rest string) string {
	fields := strings.Fields(rest)
	for i, f := range fields {
		if !strings.Contains(f, "%") {
			continue
		}
		parts := []string{f}
		if i+2 < len(fields) && fields[i+1] == "of" {
			parts = append(parts, "of", fields[i+2])
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func (s ytProgressState) message(verb string) string {
	var b strings.Builder
	b.WriteString(verb)
	if s.item != "" {
		b.WriteString(" ")
		b.WriteString(s.item)
	}
	if s.percent != "" && verb == "Downloading" {
		b.WriteString(" ")
		b.WriteString(s.percent)
	}
	if s.file != "" {
		b.WriteString(": ")
		b.WriteString(s.file)
	}
	return b.String()
}

func parseYtDlpDestinations(output string) []string {
	var paths []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "[ExtractAudio] Destination: "); ok {
			paths = append(paths, strings.TrimSpace(rest))
			continue
		}
		if strings.HasPrefix(line, "[download] ") && strings.HasSuffix(line, " has already been downloaded") {
			p := strings.TrimSuffix(strings.TrimPrefix(line, "[download] "), " has already been downloaded")
			paths = append(paths, strings.TrimSpace(p))
		}
	}
	return paths
}

func resolveAudioRel(path, outputDir, format string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	format = strings.ToLower(strings.TrimPrefix(format, "."))
	ext := filepath.Ext(path)
	baseNoExt := strings.TrimSuffix(filepath.Base(path), ext)

	candidates := []string{path}
	if format != "" {
		withFormat := strings.TrimSuffix(path, ext) + "." + format
		candidates = append(candidates, withFormat, filepath.Join(outputDir, baseNoExt+"."+format))
	}
	candidates = append(candidates, filepath.Join(outputDir, filepath.Base(path)))

	for _, c := range candidates {
		info, err := os.Stat(c)
		if err != nil || info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(outputDir, c)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		return rel
	}
	return ""
}

func snapshotFiles(dir, format string) (map[string]struct{}, error) {
	files := map[string]struct{}{}
	targetExt := strings.ToLower("." + strings.TrimPrefix(format, "."))

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if targetExt == "." && !strings.HasPrefix(format, ".") {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), targetExt) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[rel] = struct{}{}
		return nil
	})

	return files, err
}

func diffFiles(before, after map[string]struct{}) []string {
	var files []string
	for path := range after {
		if _, exists := before[path]; !exists {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func (d *Downloader) prepareCover(ctx context.Context, cover string) (string, func(), error) {
	if strings.TrimSpace(cover) == "" {
		return "", func() {}, nil
	}

	if !isURL(cover) {
		if _, err := os.Stat(cover); err != nil {
			return "", func() {}, fmt.Errorf("cover file: %w", err)
		}
		return cover, func() {}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cover, nil)
	if err != nil {
		return "", func() {}, fmt.Errorf("create cover request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", func() {}, fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", func() {}, fmt.Errorf("download cover: unexpected status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "iturtle-cover-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp cover: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", func() {}, fmt.Errorf("write cover: %w", err)
	}

	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func (d *Downloader) applyMetadata(ctx context.Context, ffmpegCmd, filePath, coverPath string, meta Metadata) error {
	tmpPath := filePath + ".tagged"
	_ = os.Remove(tmpPath)

	args := buildFFmpegArgs(filePath, tmpPath, meta, coverPath)
	if _, err := d.runner.Run(ctx, ffmpegCmd, args...); err != nil {
		return err
	}

	return os.Rename(tmpPath, filePath)
}

func buildFFmpegArgs(input, output string, meta Metadata, coverPath string) []string {
	args := []string{"-y", "-i", input}
	hasCover := strings.TrimSpace(coverPath) != ""

	if hasCover {
		args = append(args, "-i", coverPath)
	}

	args = append(args, "-map", "0:a")
	if hasCover {
		args = append(args,
			"-map", "1",
			"-c:a", "copy",
			"-c:v", "mjpeg",
			"-metadata:s:v", "title=Album cover",
			"-metadata:s:v", "comment=Cover (front)",
			"-disposition:v:0", "attached_pic",
		)
	} else {
		args = append(args, "-c", "copy")
	}

	args = appendMetadata(args, meta)
	args = append(args, "-id3v2_version", "3")

	// Explicitly specify output format for ffmpeg 8.x compatibility
	// (needed because .tagged extension doesn't auto-detect as mp3)
	args = append(args, "-f", "mp3", output)
	return args
}

func appendMetadata(args []string, meta Metadata) []string {
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, value))
		}
	}

	add("title", meta.Title)
	add("artist", meta.Artist)
	add("album", meta.Album)
	add("album_artist", meta.AlbumArtist)
	add("composer", meta.Composer)
	add("year", meta.Year)
	add("date", meta.Year)
	add("genre", meta.Genre)
	add("track", meta.Track)
	add("comment", meta.Comment)
	return args
}

func isURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// getTrackMetadata determines the metadata for a specific file based on playlist metadata.
// It tries to match by playlist index in the filename, falling back to position-based matching.
func (d *Downloader) getTrackMetadata(pm *PlaylistMetadata, filename string, fileIndex int) Metadata {
	// Try to extract playlist index from filename (format: "N - title.ext")
	extractedIndex := extractPlaylistIndex(filename)
	trackIndex := extractedIndex
	if trackIndex <= 0 {
		// Fall back to file index (1-based)
		trackIndex = fileIndex + 1
	}

	// Find matching track metadata
	var trackMeta TrackMetadata
	matchedByTitle := false
	if extractedIndex > 0 && extractedIndex <= len(pm.Tracks) {
		trackMeta = pm.Tracks[extractedIndex-1]
	} else if meta, ok := matchTrackByTitle(pm.Tracks, filename); ok {
		trackMeta = meta
		matchedByTitle = true
		if trackMeta.Position > 0 {
			trackIndex = trackMeta.Position
		}
	} else if extractedIndex <= 0 && len(pm.Tracks) > fileIndex {
		trackMeta = pm.Tracks[fileIndex]
	}

	meta := MergeTrackMetadata(pm.AlbumInfo, trackMeta, trackIndex)
	if !matchedByTitle && trackMeta.Title == "" {
		if title := titleFromFilename(filename); title != "" {
			meta.Title = title
		}
	}
	return meta
}

// matchTrackByTitle finds the MusicBrainz track whose title appears in the filename.
func matchTrackByTitle(tracks []TrackMetadata, filename string) (TrackMetadata, bool) {
	haystack := normalizeTitle(titleFromFilename(filename))
	if haystack == "" {
		return TrackMetadata{}, false
	}

	best := -1
	bestLen := 0
	for i, tr := range tracks {
		title := normalizeTitle(tr.Title)
		if title == "" {
			continue
		}
		if haystack == title {
			if len(title) >= bestLen {
				best = i
				bestLen = len(title) + 1 // prefer exact matches
			}
			continue
		}
		if len(title) < 8 {
			continue
		}
		if strings.Contains(haystack, title) || strings.Contains(title, haystack) {
			if len(title) > bestLen {
				best = i
				bestLen = len(title)
			}
		}
	}
	if best < 0 {
		return TrackMetadata{}, false
	}
	return tracks[best], true
}

func titleFromFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	for i, c := range base {
		if c == ' ' && i > 0 && i+2 < len(base) && base[i+1] == '-' && base[i+2] == ' ' {
			base = strings.TrimSpace(base[i+3:])
			break
		}
	}
	return strings.TrimSpace(base)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// extractPlaylistIndex extracts the playlist index from a filename.
// Expected format: "N - title.ext" where N is the playlist index.
func extractPlaylistIndex(filename string) int {
	base := filepath.Base(filename)
	// Look for pattern "N - " at the start
	for i, c := range base {
		if c == ' ' && i > 0 && i+2 < len(base) && base[i+1] == '-' && base[i+2] == ' ' {
			// Parse the number before the dash
			numStr := base[:i]
			var num int
			if _, err := fmt.Sscanf(numStr, "%d", &num); err == nil {
				return num
			}
			break
		}
	}
	return 0
}
