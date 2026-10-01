#!/usr/bin/env bash
# Install the pre-built iTurtle binary from GitHub Releases, plus ffmpeg and yt-dlp.
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

install_runtime_deps() {
  local missing=()
  command -v ffmpeg >/dev/null 2>&1 || missing+=(ffmpeg)
  command -v yt-dlp >/dev/null 2>&1 || missing+=(yt-dlp)
  if [[ ${#missing[@]} -eq 0 ]]; then
    log "ffmpeg and yt-dlp already on PATH"
    return 0
  fi

  log "installing missing runtime tools: ${missing[*]}"

  case "$(detect_os)" in
    darwin)
      if command -v brew >/dev/null 2>&1; then
        brew install "${missing[@]}"
      elif command -v port >/dev/null 2>&1; then
        run_root port install "${missing[@]}"
      else
        err "ffmpeg/yt-dlp are missing and neither Homebrew nor MacPorts was found. Install one of them, or install ffmpeg and yt-dlp manually."
      fi
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
        err "ffmpeg/yt-dlp are missing and no supported package manager was found (apt, dnf, yum, pacman)."
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
