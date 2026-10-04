package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"iturtle-smart-fetcher/internal/config"
	"iturtle-smart-fetcher/internal/downloader"
	"iturtle-smart-fetcher/internal/musicbrainz"
	"iturtle-smart-fetcher/internal/tools"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stderr)
		return 1
	}

	switch args[0] {
	case "download":
		return runDownload(args[1:], stdout, stderr)
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	case "about":
		printAboutTo(stdout)
		return 0
	case "version":
		printVersionTo(stdout)
		return 0
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printRootUsage(stderr)
		return 1
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintf(w, `iTurtle - download and tag music from YouTube

Usage:
  iTurtle <command> [parameters]

Commands:
  download    Download and tag music from YouTube
  generate    Generate sample files (currently only albums.yml)
  about       About iTurtle
  version     Print the current version
  help        Show this help

Examples:
  iTurtle download -url https://youtube.com/watch?v=VIDEO_ID -out ./music \
    -artist "Black Kids" -album "Partie Traumatic" -year 2008 -genre "Indie Pop"

  iTurtle download -url "..." -musicbrainz-id "abc-123-def"

  iTurtle download -url "..." -auto-fetch-metadata "Black Kids - Partie Traumatic"

  iTurtle download -config albums.yml

  iTurtle generate albums.yml

  iTurtle version
  iTurtle about
`)
}

func printGenerateUsage(w io.Writer) {
	fmt.Fprintf(w, `iTurtle generate - generate albums.yml

Usage:
  iTurtle generate <name>

Available:
  albums.yml    Example batch configuration file
`)
}

func runGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printGenerateUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printGenerateUsage(stderr)
		return 1
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "❌ generate accepts exactly one name\n\n")
		printGenerateUsage(stderr)
		return 2
	}

	name := strings.TrimSpace(rest[0])
	if name == "" {
		printGenerateUsage(stderr)
		return 1
	}
	content, ok := generateContents(name)
	if !ok {
		fmt.Fprintf(stderr, "❌ unknown generate target %q\n\n", rest[0])
		printGenerateUsage(stderr)
		return 2
	}

	if _, err := os.Stat(name); err == nil {
		fmt.Fprintf(stderr, "❌ %s already exists\n", name)
		return 1
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "❌ %v\n", err)
		return 1
	}

	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stderr, "❌ Failed to write %s: %v\n", name, err)
		return 1
	}

	fmt.Fprintf(stdout, "🐢 Generated %s\n", name)
	return 0
}

func generateContents(name string) (string, bool) {
	switch name {
	case "albums.yml":
		return config.Example(), true
	default:
		return "", false
	}
}

func runDownload(args []string, stdout, stderr io.Writer) int {
	var cfg downloader.Config
	var (
		ytDLPPath      string
		ffmpegPath     string
		jsRuntimePath  string
		configFile     string
		musicBrainzID  string
		autoFetchQuery string
	)

	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.URL, "url", "", "YouTube video or playlist URL (required unless -config is used)")
	fs.StringVar(&cfg.OutputDir, "out", ".", "Directory where songs will be stored")
	fs.StringVar(&cfg.Cover, "cover", "", "Path or URL to album / track cover image")
	fs.StringVar(&cfg.AudioFormat, "format", "mp3", "Audio format to save (mp3 recommended)")
	fs.StringVar(&ytDLPPath, "yt-dlp-path", "", "Path to yt-dlp binary (optional, searches PATH if not specified)")
	fs.StringVar(&ffmpegPath, "ffmpeg-path", "", "Path to ffmpeg binary (optional, searches PATH if not specified)")
	fs.StringVar(&jsRuntimePath, "js-runtime-path", "", "Path to Deno or Node (optional, searches PATH if not specified)")
	fs.StringVar(&cfg.CookiesFromBrowser, "cookies-from-browser", "", "Read YouTube cookies from a browser (safari, chrome, firefox, brave, edge)")
	fs.StringVar(&cfg.Cookies, "cookies", "", "Path to a Netscape-format cookies file for YouTube")

	fs.StringVar(&cfg.Metadata.Title, "title", "", "Song title metadata override")
	fs.StringVar(&cfg.Metadata.Artist, "artist", "", "Artist metadata")
	fs.StringVar(&cfg.Metadata.Album, "album", "", "Album metadata")
	fs.StringVar(&cfg.Metadata.AlbumArtist, "album-artist", "", "Album artist metadata")
	fs.StringVar(&cfg.Metadata.Composer, "composer", "", "Composer metadata")
	fs.StringVar(&cfg.Metadata.Year, "year", "", "Release year metadata")
	fs.StringVar(&cfg.Metadata.Genre, "genre", "", "Genre metadata")
	fs.StringVar(&cfg.Metadata.Track, "track", "", "Track number metadata")
	fs.StringVar(&cfg.Metadata.Comment, "comment", "", "Comment metadata")

	fs.StringVar(&configFile, "config", "", "Path to YAML batch configuration file")
	fs.StringVar(&musicBrainzID, "musicbrainz-id", "", "MusicBrainz release ID to fetch metadata")
	fs.StringVar(&autoFetchQuery, "auto-fetch-metadata", "", "Auto-search MusicBrainz (format: \"Artist - Album\")")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "iTurtle download - download and tag music from YouTube\n\n")
		fmt.Fprintf(stderr, "Usage:\n  iTurtle download [parameters]\n\nParameters:\n")
		fs.PrintDefaults()
		fmt.Fprintf(stderr, `
Examples:
  iTurtle download -url https://youtube.com/watch?v=VIDEO_ID -out ./music \
    -artist "Black Kids" -album "Partie Traumatic" -year 2008 -genre "Indie Pop"

  iTurtle download -url "..." -musicbrainz-id "abc-123-def"

  iTurtle download -url "..." -auto-fetch-metadata "Black Kids - Partie Traumatic"

  iTurtle download -url "..." -cookies-from-browser safari

  iTurtle download -config albums.yml
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if configFile == "" && strings.TrimSpace(cfg.URL) == "" {
		fs.Usage()
		return 1
	}

	if strings.TrimSpace(cfg.Cookies) != "" && strings.TrimSpace(cfg.CookiesFromBrowser) != "" {
		fmt.Fprintf(stderr, "❌ use either -cookies-from-browser or -cookies, not both\n")
		return 2
	}
	if cookies := strings.TrimSpace(cfg.Cookies); cookies != "" {
		if _, err := os.Stat(cookies); err != nil {
			fmt.Fprintf(stderr, "❌ cookies file: %v\n", err)
			return 1
		}
	}

	ctx := context.Background()

	manager := tools.New()
	paths, err := manager.Ensure(tools.Options{
		YtDLPPath:     ytDLPPath,
		FFmpegPath:    ffmpegPath,
		JSRuntimePath: jsRuntimePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "❌ Tool setup failed: %v\n", err)
		return 1
	}

	if configFile != "" {
		if err := runBatchMode(ctx, configFile, paths, cfg); err != nil {
			fmt.Fprintf(stderr, "\n❌ Batch download failed: %v\n", err)
			return 1
		}
		return 0
	}

	cfg.YtDLPPath = paths.YtDLP
	cfg.FFmpegPath = paths.FFmpeg
	cfg.JSRuntime = paths.JSRuntime

	if musicBrainzID != "" || autoFetchQuery != "" {
		pm, err := fetchMusicBrainzMetadata(ctx, musicBrainzID, autoFetchQuery)
		if err != nil {
			fmt.Fprintf(stderr, "⚠️  MusicBrainz lookup failed: %v\n", err)
			fmt.Fprintf(stderr, "    Continuing without MusicBrainz metadata...\n\n")
		} else {
			cfg.PlaylistMetadata = pm
			printMusicBrainzFound(stdout, pm)
		}
	}

	dl := downloader.New(nil, nil)

	_, err = dl.Download(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "\n❌ Download failed: %v\n", err)
		return 1
	}

	return 0
}

// runBatchMode processes albums from a configuration file.
func runBatchMode(ctx context.Context, configFile string, paths tools.Paths, defaults downloader.Config) error {
	batchCfg, err := config.LoadFromFile(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "🐢 Processing %d album(s) from configuration...\n\n", len(batchCfg.Albums))

	dl := downloader.New(nil, nil)
	var failed []string

	for i, albumCfg := range batchCfg.Albums {
		fmt.Fprintf(os.Stdout, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Fprintf(os.Stdout, "Album %d/%d", i+1, len(batchCfg.Albums))
		if albumCfg.Album != "" {
			fmt.Fprintf(os.Stdout, ": %s", albumCfg.Album)
		}
		if albumCfg.Artist != "" {
			fmt.Fprintf(os.Stdout, " by %s", albumCfg.Artist)
		}
		fmt.Fprintf(os.Stdout, "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		cfg := albumCfg.ToDownloaderConfig(".")
		cfg.YtDLPPath = paths.YtDLP
		cfg.FFmpegPath = paths.FFmpeg
		cfg.JSRuntime = paths.JSRuntime
		cfg.Cookies = defaults.Cookies
		cfg.CookiesFromBrowser = defaults.CookiesFromBrowser
		if cfg.AudioFormat == "" {
			cfg.AudioFormat = defaults.AudioFormat
		}

		if albumCfg.NeedsMusicBrainzLookup() {
			pm, err := fetchMusicBrainzMetadata(ctx, albumCfg.MusicBrainzID, albumCfg.AutoFetch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  MusicBrainz lookup failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "    Continuing with manual metadata...\n\n")
			} else {
				cfg.PlaylistMetadata = pm
				printMusicBrainzFound(os.Stdout, pm)
			}
		}

		_, err := dl.Download(ctx, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Failed to download album: %v\n\n", err)
			name := albumCfg.Album
			if name == "" {
				name = albumCfg.URL
			}
			failed = append(failed, name)
			continue
		}
		fmt.Fprintf(os.Stdout, "\n")
	}

	fmt.Fprintf(os.Stdout, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(os.Stdout, "Batch Complete: %d/%d albums successful\n", len(batchCfg.Albums)-len(failed), len(batchCfg.Albums))
	if len(failed) > 0 {
		fmt.Fprintf(os.Stdout, "Failed albums:\n")
		for _, name := range failed {
			fmt.Fprintf(os.Stdout, "  - %s\n", name)
		}
		return fmt.Errorf("%d album(s) failed", len(failed))
	}

	return nil
}

func printMusicBrainzFound(w io.Writer, pm *downloader.PlaylistMetadata) {
	fmt.Fprintf(w, "🎵 Found on MusicBrainz: %s - %s (%s)\n", pm.AlbumInfo.Artist, pm.AlbumInfo.Title, pm.AlbumInfo.Year)
	fmt.Fprintf(w, "   %d tracks\n\n", len(pm.Tracks))
}

// fetchMusicBrainzMetadata fetches album and track metadata from MusicBrainz.
func fetchMusicBrainzMetadata(ctx context.Context, mbID, autoQuery string) (*downloader.PlaylistMetadata, error) {
	client := musicbrainz.NewClient(nil)

	var release *musicbrainz.Release
	var err error

	if mbID != "" {
		release, err = client.GetReleaseByID(ctx, mbID)
		if err != nil {
			return nil, fmt.Errorf("fetch release by ID: %w", err)
		}
	} else if autoQuery != "" {
		results, err := client.AutoSearch(ctx, autoQuery)
		if err != nil {
			return nil, fmt.Errorf("search releases: %w", err)
		}
		if len(results.Releases) == 0 {
			return nil, fmt.Errorf("no releases found for query: %s", autoQuery)
		}
		release, err = client.GetReleaseByID(ctx, results.Releases[0].ID)
		if err != nil {
			return nil, fmt.Errorf("fetch release details: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either musicbrainz-id or auto-fetch-metadata is required")
	}

	var coverURL string
	coverURL, err = client.GetFrontCoverURL(ctx, release.ID)
	if err != nil {
		coverURL = ""
	}

	return musicbrainz.ToPlaylistMetadataWithCover(release, coverURL), nil
}
