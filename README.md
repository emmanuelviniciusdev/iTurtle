![tortoise-dance](tortoise-dance.gif)

# iTurtle

Welcome! iTurtle is a CLI utility that downloads audio streams from YouTube, converts them into high-quality MP3s, embeds complete ID3 tags and cover art, and organizes tracks seamlessly into the specified destination folder.

## Features

- **Single Songs & Full Playlists**: Grab an individual track or an entire album playlist in one go.
- **Great Sound Quality**: Saves your songs in crisp, high-quality audio files (like MP3) ready for your iPod.
- **Complete Song Details**: Automatically adds song titles, artist names, album titles, track numbers, release years, and genres so your library looks great.
- **Automatic Album Art**: Embeds cover art directly into your music files from online databases, web links, or image files on your computer.
- **Smart Music Search**: Search by artist and album name to pull accurate tracklists and release details automatically.
- **Batch Downloads**: Queue up multiple albums at once using a simple playlist file and let iTurtle handle the rest.
- **Safe & Tidy**: Only tags newly downloaded files—never alters or touches songs already on your computer.
- **Cheerful Progress** 🐢: A fun turtle-themed progress bar cheers along as your music downloads and gets tagged 🐢.
- **Works Everywhere**: Enjoy iTurtle on Macintosh, Windows, and Linux.



## Installation

Getting iTurtle up and running takes just a few seconds!

### Macintosh & Linux

Open your terminal and paste this one-liner. It downloads iTurtle and sets up everything you need automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/emmanuelviniciusdev/iTurtle/main/scripts/install.sh | bash
```

*Tip: If you ever want to update to the latest release, just run the command again!*

### Windows

Open PowerShell and run this quick command. It installs iTurtle, sets up any helpers needed, and adds it straight to your command line:

```powershell
irm https://raw.githubusercontent.com/emmanuelviniciusdev/iTurtle/main/scripts/install.ps1 | iex
```



### Pre-built Downloads

Prefer downloading files yourself?

1. Head over to the [Releases](https://github.com/emmanuelviniciusdev/iTurtle/releases) page.
2. Download the package for your operating system.
3. Unzip the file and place `iTurtle` anywhere in your system PATH.



### Building from Source

If you like building tools yourself:

```bash
git clone https://github.com/emmanuelviniciusdev/iTurtle.git
cd iTurtle
make install
```



## Usage Examples

Here are some quick and easy ways to use iTurtle with the indie-pop band **Black Kids**!

### 1. Download a Single Song with Details

Grab a single track, save it to your music folder, and give it full details:

```bash
iTurtle download \
  -url "https://www.youtube.com/watch?v=rOV6I4fYnvQ" \
  -out ./music/Black_Kids \
  -artist "Black Kids" \
  -album "Partie Traumatic" \
  -title "I'm Not Gonna Teach Your Boyfriend How to Dance with You" \
  -year 2008 \
  -genre "Indie Pop" \
  -track 5 \
  -comment "Lead single from Partie Traumatic"
```



### 2. Download a Full Album Playlist with Cover Art

Download a whole album playlist and attach a cover photo from your computer to every song:

```bash
iTurtle download \
  -url "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_PARTIE_TRAUMATIC" \
  -out ./music/Black_Kids/Partie_Traumatic \
  -cover ./art/partie-traumatic.jpg \
  -artist "Black Kids" \
  -album "Partie Traumatic" \
  -year 2008 \
  -genre "Indie Pop"
```



### 3. Add Cover Art Directly from a Web Link

Have an image link on the web? iTurtle will fetch the picture and embed it for you:

```bash
iTurtle download \
  -url "https://www.youtube.com/watch?v=VIDEO_ID" \
  -out ./music/Black_Kids/Rookie \
  -cover "https://example.com/black-kids-rookie.jpg" \
  -artist "Black Kids" \
  -album "Rookie" \
  -title "Obligatory Drugs" \
  -year 2017 \
  -genre "Indie Rock"
```



### 4. Let iTurtle Find Song Details Automatically

No need to type track names by hand! Just provide the band and album title, and iTurtle looks up the tracklist, release year, and album cover automatically:

```bash
iTurtle download \
  -url "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_PARTIE_TRAUMATIC" \
  -auto-fetch-metadata "Black Kids - Partie Traumatic" \
  -out ./music/Black_Kids/Partie_Traumatic
```



### 5. Look Up by Release ID

If you want exact database precision, you can also pass a MusicBrainz release ID:

```bash
iTurtle download \
  -url "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_PARTIE_TRAUMATIC" \
  -musicbrainz-id "e4396b29-e855-46eb-a9be-a82dcf3519c2" \
  -out ./music/Black_Kids/Partie_Traumatic
```



### 6. Batch Download Multiple Albums

Want to download multiple albums while you step away? Create a list and let iTurtle do the heavy lifting!

First, create a starter file:

```bash
iTurtle generate albums.yml
```

Next, list your albums in `albums.yml`:

```yaml
# albums.yml
albums:
  # Album 1: Partie Traumatic with custom track titles
  - url: "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_PARTIE_TRAUMATIC"
    artist: "Black Kids"
    album: "Partie Traumatic"
    year: "2008"
    genre: "Indie Pop"
    cover: "https://example.com/black-kids-partie-traumatic.jpg"
    output_dir: "./music/Black Kids/Partie Traumatic"
    tracks:
      - {num: 1, title: "Hit The Heartbrakes"}
      - {num: 2, title: "Partie Traumatic"}
      - {num: 3, title: "Listen to Your Heart Beat"}
      - {num: 4, title: "Hurricane Jane"}
      - {num: 5, title: "I'm Not Gonna Teach Your Boyfriend How to Dance with You"}
      - {num: 6, title: "Love's Not a Competition (But I'm Winning)"}
      - {num: 7, title: "Look at Me (When I Rock Fish)"}
      - {num: 8, title: "I've Underestimated My Charm (Again)"}
      - {num: 9, title: "I'm Making Eyes at You"}

  # Album 2: Rookie with automatic details lookup
  - url: "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_ROOKIE"
    auto_fetch: "Black Kids - Rookie"
    output_dir: "./music/Black Kids/Rookie"

  # Album 3: Wizard of Ahhhs EP with release ID lookup
  - url: "https://www.youtube.com/playlist?list=PL_BLACK_KIDS_WIZARD_OF_AHHHS"
    musicbrainz_id: "a0937c02-7744-42b4-82a1-e37456d20fa4"
    output_dir: "./music/Black Kids/Wizard of Ahhhs"
```

Finally, start the batch run:

```bash
iTurtle download -config albums.yml
```



### 7. Handy Quick Commands

```bash
# Generate a starter albums.yml
iTurtle generate albums.yml

# Check the installed version
iTurtle version

# See all available options and help
iTurtle help
```

