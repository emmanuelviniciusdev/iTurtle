#!/usr/bin/env bash
# Install the pre-built iTurtle binary from GitHub Releases, plus ffmpeg and yt-dlp.
# Runtime tools are installed as standalone binaries so macOS does not compile
# MacPorts' full ffmpeg tree (cairo, Python, openssl3, ...).
set -euo pipefail

REPO="${ITURTLE_REPO:-emmanuelviniciusdev/iTurtle}"
PREFIX="${PREFIX:-/usr/local/bin}"

log() { printf '==> %s\n' "$*"; }
err() { printf 'error: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

github_api() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "Accept: application/vnd.github+json" "$@"
  else
    curl -fsSL -H "Accept: application/vnd.github+json" "$@"
  fi
}

download() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" "$@"
  else
    curl -fsSL "$@"
  fi
}

# Visible progress for large runtime binaries (ffmpeg/yt-dlp).
download_bin() {
  local dest="$1" url="$2"
  if [[ -n "${GITHUB_TOKEN:-}" && "${url}" == *github.com* ]]; then
    curl -fL --retry 3 --retry-delay 1 --progress-bar -H "Authorization: Bearer ${GITHUB_TOKEN}" -o "${dest}" "${url}"
  else
    curl -fL --retry 3 --retry-delay 1 --progress-bar -o "${dest}" "${url}"
  fi
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) err "unsupported OS: $(uname -s). Use scripts/install.ps1 on Windows." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    i386|i686|x86) printf '386' ;;
    *) err "unsupported architecture: $(uname -m)" ;;
  esac
}

latest_tag() {
  github_api "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' \
    | sed 's/.*"tag_name": *"//;s/".*//'
}

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

install_file() {
  local src="$1" dest="$2"
  if [[ -w "$(dirname "${dest}")" ]]; then
    install -m 0755 "${src}" "${dest}"
  else
    need_cmd sudo
    sudo install -m 0755 "${src}" "${dest}"
  fi
}

run_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  else
    need_cmd sudo
    sudo "$@"
  fi
}

installed_version() {
  local bin="$1"
  [[ -x "${bin}" ]] || return 0
  local out
  out="$("${bin}" version 2>/dev/null || true)"
  # "iTurtle 1.2.3 (commit ..., built ...)"
  printf '%s\n' "${out}" | awk '/^iTurtle / { print $2; exit }'
}

normalize_tag() {
  local tag="$1"
  [[ -n "${tag}" ]] || return 1
  if [[ "${tag}" != v* ]]; then
    tag="v${tag}"
  fi
  printf '%s\n' "${tag}"
}

extract_zip() {
  local archive="$1" dest="$2"
  mkdir -p "${dest}"
  if command -v unzip >/dev/null 2>&1; then
    unzip -q -o "${archive}" -d "${dest}"
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c 'import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "${archive}" "${dest}"
  else
    tar -xf "${archive}" -C "${dest}"
  fi
}

clear_quarantine() {
  local path="$1"
  [[ "$(detect_os)" == "darwin" ]] || return 0
  xattr -dr com.apple.quarantine "${path}" 2>/dev/null || \
    run_root xattr -dr com.apple.quarantine "${path}" 2>/dev/null || true
}

install_named_bin() {
  local src="$1" name="$2"
  mkdir -p "${PREFIX}" 2>/dev/null || run_root mkdir -p "${PREFIX}"
  install_file "${src}" "${PREFIX}/${name}"
  clear_quarantine "${PREFIX}/${name}"
}

find_extracted_bin() {
  local root="$1" name="$2" f
  if [[ -f "${root}/${name}" ]]; then
    printf '%s\n' "${root}/${name}"
    return 0
  fi
  while IFS= read -r f; do
    if [[ -n "${f}" ]]; then
      printf '%s\n' "${f}"
      return 0
    fi
  done <<EOF
$(find "${root}" -type f ! -path '*/__MACOSX/*' -name "${name}" 2>/dev/null)
EOF
  return 1
}

ytdlp_download_url() {
  case "$(detect_os)_$(detect_arch)" in
    darwin_*) printf '%s\n' "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos" ;;
    linux_amd64) printf '%s\n' "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux" ;;
    linux_arm64) printf '%s\n' "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64" ;;
    *) return 1 ;;
  esac
}

ffmpeg_zip_urls() {
  local os arch plat
  os="$(detect_os)"
  arch="$(detect_arch)"
  case "${os}" in
    darwin) plat="macos" ;;
    linux) plat="linux" ;;
    *) return 1 ;;
  esac
  case "${arch}" in
    amd64|arm64) ;;
    *) return 1 ;;
  esac
  printf '%s\n' "https://ffmpeg.martin-riedl.de/redirect/latest/${plat}/${arch}/release/ffmpeg.zip"
  printf '%s\n' "https://ffmpeg.martin-riedl.de/redirect/latest/${plat}/${arch}/release/ffprobe.zip"
}

install_zipped_bin() {
  local url="$1" name="$2" tmpdir="$3"
  local zipfile extract found
  zipfile="${tmpdir}/${name}.zip"
  extract="${tmpdir}/${name}.extract"
  rm -rf "${extract}"
  mkdir -p "${extract}"
  download_bin "${zipfile}" "${url}" || return 1
  extract_zip "${zipfile}" "${extract}" || return 1
  found="$(find_extracted_bin "${extract}" "${name}")" || return 1
  chmod +x "${found}"
  install_named_bin "${found}" "${name}"
}

install_ytdlp_standalone() {
  local tmpdir="$1" url src
  url="$(ytdlp_download_url)" || return 1
  src="${tmpdir}/yt-dlp"
  log "downloading standalone yt-dlp"
  download_bin "${src}" "${url}" || return 1
  chmod +x "${src}"
  install_named_bin "${src}" "yt-dlp"
}

install_ffmpeg_standalone() {
  local tmpdir="$1"
  local ffmpeg_url ffprobe_url urls
  urls="$(ffmpeg_zip_urls)" || return 1
  ffmpeg_url="$(printf '%s\n' "${urls}" | awk 'NR==1 {print; exit}')"
  ffprobe_url="$(printf '%s\n' "${urls}" | awk 'NR==2 {print; exit}')"

  log "downloading standalone ffmpeg"
  if ! install_zipped_bin "${ffmpeg_url}" "ffmpeg" "${tmpdir}"; then
    if [[ "$(detect_os)" == "darwin" && "$(detect_arch)" == "amd64" ]]; then
      log "retrying ffmpeg from evermeet.cx"
      install_zipped_bin "https://evermeet.cx/ffmpeg/getrelease/zip" "ffmpeg" "${tmpdir}" || return 1
      install_zipped_bin "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip" "ffprobe" "${tmpdir}" || true
      return 0
    fi
    return 1
  fi
  log "downloading standalone ffprobe"
  install_zipped_bin "${ffprobe_url}" "ffprobe" "${tmpdir}" || true
}

install_from_package_manager() {
  local missing=("$@")
  case "$(detect_os)" in
    darwin)
      if command -v brew >/dev/null 2>&1; then
        brew install "${missing[@]}"
        return 0
      fi
      # MacPorts' ffmpeg port always pulls librsvg → Python → openssl3 and
      # often compiles them from source. That is far more than iTurtle needs.
      return 1
      ;;
    linux)
      if command -v apt-get >/dev/null 2>&1; then
        run_root apt-get update -y
        run_root apt-get install -y "${missing[@]}"
      elif command -v dnf >/dev/null 2>&1; then
        run_root dnf install -y "${missing[@]}"
      elif command -v yum >/dev/null 2>&1; then
        run_root yum install -y "${missing[@]}"
      elif command -v pacman >/dev/null 2>&1; then
        run_root pacman -Sy --noconfirm "${missing[@]}"
      else
        return 1
      fi
      ;;
    *)
      return 1
      ;;
  esac
}

install_standalone_tools() {
  local need dep_tmp
  dep_tmp="$(mktemp -d)"
  for need in "$@"; do
    case "${need}" in
      yt-dlp) install_ytdlp_standalone "${dep_tmp}" || true ;;
      ffmpeg) install_ffmpeg_standalone "${dep_tmp}" || true ;;
    esac
  done
  rm -rf "${dep_tmp}"
}

refresh_missing() {
  missing=()
  command -v ffmpeg >/dev/null 2>&1 || missing+=(ffmpeg)
  command -v yt-dlp >/dev/null 2>&1 || missing+=(yt-dlp)
}

install_runtime_deps() {
  local missing=()
  export PATH="${PREFIX}:${PATH}"
  refresh_missing
  if [[ ${#missing[@]} -eq 0 ]]; then
    log "ffmpeg and yt-dlp already on PATH"
    return 0
  fi

  log "installing missing runtime tools: ${missing[*]}"

  case "$(detect_os)" in
    darwin)
      # Prefer static binaries. MacPorts ffmpeg always depends on librsvg,
      # which pulls Python and openssl3 and often compiles them from source.
      install_standalone_tools "${missing[@]}"
      refresh_missing
      if [[ ${#missing[@]} -ne 0 ]]; then
        log "standalone download incomplete; trying Homebrew for: ${missing[*]}"
        install_from_package_manager "${missing[@]}" || \
          err "could not install ${missing[*]} as standalone binaries, and Homebrew was not found. Install ffmpeg and yt-dlp manually (brew install ffmpeg yt-dlp)."
      fi
      ;;
    linux)
      if ! install_from_package_manager "${missing[@]}"; then
        log "no supported package manager; downloading standalone binaries"
        install_standalone_tools "${missing[@]}"
      fi
      ;;
  esac

  command -v ffmpeg >/dev/null 2>&1 || err "ffmpeg is still not on PATH after installation"
  command -v yt-dlp >/dev/null 2>&1 || err "yt-dlp is still not on PATH after installation"
}

main() {
  need_cmd curl
  need_cmd tar
  need_cmd install
  need_cmd grep
  need_cmd sed
  need_cmd awk

  local os arch tag version archive checksums expected actual tmp dest current
  os="$(detect_os)"
  arch="$(detect_arch)"
  if [[ "${os}" == "darwin" && "${arch}" == "386" ]]; then
    err "32-bit macOS is not supported"
  fi

  if [[ -n "${ITURTLE_VERSION:-}" ]]; then
    tag="$(normalize_tag "${ITURTLE_VERSION}")"
  else
    tag="$(normalize_tag "$(latest_tag)")"
  fi
  [[ -n "${tag}" ]] || err "could not determine the latest iTurtle release tag"
  version="${tag#v}"
  archive="iTurtle_${version}_${os}_${arch}.tar.gz"
  checksums="checksums.txt"

  dest="${PREFIX}/iTurtle"
  current="$(installed_version "${dest}")"
  if [[ -z "${current}" ]] && command -v iTurtle >/dev/null 2>&1; then
    current="$(installed_version "$(command -v iTurtle)")"
  fi

  if [[ -n "${current}" && "${current}" == "${version}" && -x "${dest}" ]]; then
    log "iTurtle ${version} already installed at ${dest}"
  else
    if [[ -n "${current}" ]]; then
      log "updating iTurtle ${current} -> ${version}"
    else
      log "installing iTurtle ${version}"
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "${tmp}"' EXIT

    log "downloading ${archive} (${tag})"
    download -o "${tmp}/${archive}" "https://github.com/${REPO}/releases/download/${tag}/${archive}"
    download -o "${tmp}/${checksums}" "https://github.com/${REPO}/releases/download/${tag}/${checksums}"

    expected="$(grep -E "[[:space:]]${archive}\$" "${tmp}/${checksums}" | awk '{print $1}' | head -n1)"
    [[ -n "${expected}" ]] || err "no sha256 entry for ${archive} in checksums.txt"
    actual="$(file_sha256 "${tmp}/${archive}")"
    [[ "${expected}" == "${actual}" ]] || err "checksum mismatch for ${archive}"

    tar -xzf "${tmp}/${archive}" -C "${tmp}"
    [[ -f "${tmp}/iTurtle" ]] || err "archive did not contain iTurtle"

    mkdir -p "${PREFIX}" 2>/dev/null || run_root mkdir -p "${PREFIX}"
    install_file "${tmp}/iTurtle" "${dest}"
    if [[ "${os}" == "darwin" ]]; then
      xattr -dr com.apple.quarantine "${dest}" 2>/dev/null || true
    fi
  fi

  install_runtime_deps

  log "iTurtle at ${dest}"
  "${dest}" version || true
  log "done. Try: iTurtle help"
}

main "$@"
