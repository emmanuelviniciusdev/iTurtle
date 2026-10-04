# Install the pre-built iTurtle.exe from GitHub Releases, plus ffmpeg, yt-dlp, and Deno.
# Deno is required by current yt-dlp to solve YouTube JavaScript challenges.
# Usage: irm https://raw.githubusercontent.com/emmanuelviniciusdev/iTurtle/main/scripts/install.ps1 | iex
[CmdletBinding()]
param(
    [string]$Version = $env:ITURTLE_VERSION,
    [string]$Repo = $(if ($env:ITURTLE_REPO) { $env:ITURTLE_REPO } else { "emmanuelviniciusdev/iTurtle" }),
    [string]$Prefix = $(if ($env:ITURTLE_PREFIX) { $env:ITURTLE_PREFIX } else { Join-Path $env:LOCALAPPDATA "iTurtle" })
)

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message)
    Write-Host "==> $Message"
}

function Get-OsArch {
    if ([Environment]::Is64BitOperatingSystem) {
        return "amd64"
    }
    return "386"
}

function Invoke-GitHubGet {
    param([string]$Uri, [string]$OutFile)
    $headers = @{ "User-Agent" = "iTurtle-install" }
    if ($env:GITHUB_TOKEN) {
        $headers["Authorization"] = "Bearer $($env:GITHUB_TOKEN)"
    }
    if ($OutFile) {
        Invoke-WebRequest -Uri $Uri -Headers $headers -OutFile $OutFile -UseBasicParsing
    } else {
        $headers["Accept"] = "application/vnd.github+json"
        return Invoke-RestMethod -Uri $Uri -Headers $headers
    }
}

function Get-FileSha256 {
    param([string]$Path)
    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-InstalledVersion {
    param([string]$Bin)
    if (-not (Test-Path $Bin)) {
        return $null
    }
    try {
        $out = & $Bin version 2>$null
        if ($out -match '^iTurtle\s+(\S+)') {
            return $Matches[1]
        }
    } catch {
        return $null
    }
    return $null
}

function Normalize-Tag {
    param([string]$Tag)
    if (-not $Tag) { return $null }
    if ($Tag -notmatch '^v') {
        return "v$Tag"
    }
    return $Tag
}

function Install-DenoStandalone {
    param([string]$DestDir)
    $arch = if ([Environment]::Is64BitOperatingSystem) { "x86_64" } else { $null }
    if (-not $arch) {
        throw "32-bit Windows is not supported for Deno. Install Deno from https://deno.com and re-run this script."
    }
    $url = "https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip"
    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("iturtle-deno-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        $zipPath = Join-Path $tmp "deno.zip"
        Write-Log "downloading standalone Deno"
        Invoke-GitHubGet -Uri $url -OutFile $zipPath
        Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
        $exe = Join-Path $tmp "deno.exe"
        if (-not (Test-Path $exe)) {
            throw "Deno archive did not contain deno.exe"
        }
        New-Item -ItemType Directory -Path $DestDir -Force | Out-Null
        Copy-Item -Path $exe -Destination (Join-Path $DestDir "deno.exe") -Force
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

function Install-RuntimeDeps {
    $missing = @()
    if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) { $missing += "ffmpeg" }
    if (-not (Get-Command yt-dlp -ErrorAction SilentlyContinue)) { $missing += "yt-dlp" }
    if (-not (Get-Command deno -ErrorAction SilentlyContinue)) { $missing += "deno" }
    if ($missing.Count -eq 0) {
        Write-Log "ffmpeg, yt-dlp, and deno already on PATH"
        return
    }

    Write-Log "installing missing runtime tools: $($missing -join ', ')"
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    $choco = Get-Command choco -ErrorAction SilentlyContinue

    $wingetIds = @{
        "ffmpeg" = "Gyan.FFmpeg"
        "yt-dlp" = "yt-dlp.yt-dlp"
        "deno"   = "Deno.Deno"
    }

    $pkgMissing = @($missing | Where-Object { $_ -ne "deno" })
    if ($pkgMissing.Count -gt 0) {
        if (-not $winget -and -not $choco) {
            throw "ffmpeg/yt-dlp are missing and neither winget nor Chocolatey was found. Install one of them, or install ffmpeg and yt-dlp manually."
        }
        foreach ($name in $pkgMissing) {
            if ($winget) {
                & winget install --id $wingetIds[$name] -e --accept-package-agreements --accept-source-agreements
            } else {
                & choco install $name -y
            }
        }
    }

    if (-not (Get-Command deno -ErrorAction SilentlyContinue)) {
        try {
            if ($winget) {
                & winget install --id $wingetIds["deno"] -e --accept-package-agreements --accept-source-agreements
            } elseif ($choco) {
                & choco install deno -y
            }
        } catch {
            Write-Log "package manager could not install Deno; trying standalone binary"
        }
        if (-not (Get-Command deno -ErrorAction SilentlyContinue)) {
            Install-DenoStandalone -DestDir $Prefix
            $env:Path = "$Prefix;$env:Path"
        }
    }

    if (-not (Get-Command deno -ErrorAction SilentlyContinue)) {
        throw "deno is still not on PATH after installation. Install Deno from https://deno.com and re-run this script."
    }
}

$arch = Get-OsArch
if (-not $Version) {
    $release = Invoke-GitHubGet -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $tag = Normalize-Tag ([string]$release.tag_name)
} else {
    $tag = Normalize-Tag $Version
}
if (-not $tag) {
    throw "could not determine the latest iTurtle release tag"
}
$numeric = $tag.TrimStart("v")
$zipName = "iTurtle_${numeric}_windows_${arch}.zip"
$dest = Join-Path $Prefix "iTurtle.exe"
$current = Get-InstalledVersion -Bin $dest
if (-not $current) {
    $onPath = Get-Command iTurtle -ErrorAction SilentlyContinue
    if ($onPath) {
        $current = Get-InstalledVersion -Bin $onPath.Source
    }
}

if ($current -and ($current -eq $numeric) -and (Test-Path $dest)) {
    Write-Log "iTurtle $numeric already installed at $dest"
} else {
    if ($current) {
        Write-Log "updating iTurtle $current -> $numeric"
    } else {
        Write-Log "installing iTurtle $numeric"
    }

    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("iturtle-install-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        $zipPath = Join-Path $tmp $zipName
        $sumPath = Join-Path $tmp "checksums.txt"
        Write-Log "downloading $zipName ($tag)"
        Invoke-GitHubGet -Uri "https://github.com/$Repo/releases/download/$tag/$zipName" -OutFile $zipPath
        Invoke-GitHubGet -Uri "https://github.com/$Repo/releases/download/$tag/checksums.txt" -OutFile $sumPath

        $expectedLine = Get-Content $sumPath | Where-Object { $_ -match [regex]::Escape($zipName) } | Select-Object -First 1
        if (-not $expectedLine) {
            throw "no sha256 entry for $zipName in checksums.txt"
        }
        $expected = ($expectedLine -split '\s+')[0].ToLowerInvariant()
        $actual = Get-FileSha256 -Path $zipPath
        if ($expected -ne $actual) {
            throw "checksum mismatch for $zipName"
        }

        Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
        $exe = Join-Path $tmp "iTurtle.exe"
        if (-not (Test-Path $exe)) {
            throw "archive did not contain iTurtle.exe"
        }

        New-Item -ItemType Directory -Path $Prefix -Force | Out-Null
        Copy-Item -Path $exe -Destination $dest -Force
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $userPath) { $userPath = "" }
$parts = $userPath -split ';' | Where-Object { $_ -and $_.Trim() -ne "" }
if ($parts -notcontains $Prefix) {
    $newPath = ($parts + $Prefix) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$Prefix;$env:Path"
    Write-Log "added $Prefix to the user PATH"
}

Install-RuntimeDeps

Write-Log "iTurtle at $dest"
& $dest version
Write-Log "done. Open a new terminal and try: iTurtle help"
