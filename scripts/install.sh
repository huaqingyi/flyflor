#!/usr/bin/env sh
set -eu

repo="${FLYFLOR_REPO:-huaqingyi/flyflor}"
version="${FLYFLOR_VERSION:-latest}"
install_dir="${FLYFLOR_INSTALL_DIR:-$HOME/.local/bin}"
from_source="${FLYFLOR_FROM_SOURCE:-0}"
tmp_dir="$(mktemp -d 2>/dev/null || mktemp -d -t flyflor-install)"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

say() {
  printf '%s\n' "$*"
}

fail() {
  printf 'flyflor install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

has() {
  command -v "$1" >/dev/null 2>&1
}

os_name() {
  uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$uname_s" in
    linux*) printf 'linux' ;;
    darwin*) printf 'darwin' ;;
    msys*|mingw*|cygwin*) printf 'windows' ;;
    *) fail "unsupported OS: $uname_s" ;;
  esac
}

arch_name() {
  uname_m="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$uname_m" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    armv7l|armv7*) printf 'armv7' ;;
    armv6l|armv6*) printf 'arm' ;;
    riscv64) printf 'riscv64' ;;
    loongarch64|loong64) printf 'loong64' ;;
    mipsel|mipsle) printf 'mipsle' ;;
    *) fail "unsupported architecture: $uname_m" ;;
  esac
}

release_api_url() {
  if [ "$version" = "latest" ]; then
    printf 'https://api.github.com/repos/%s/releases/latest' "$repo"
  else
    printf 'https://api.github.com/repos/%s/releases/tags/%s' "$repo" "$version"
  fi
}

asset_url_from_release() {
  os="$1"
  arch="$2"
  api_url="$(release_api_url)"
  json="$tmp_dir/release.json"
  need curl
  curl -fsSL "$api_url" -o "$json"

  urls="$(sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$json")"
  [ -n "$urls" ] || return 1

  case "$arch" in
    amd64) arch_re='amd64|x86_64' ;;
    arm64) arch_re='arm64|aarch64' ;;
    armv7) arch_re='armv7|arm-?v7|linux-arm' ;;
    *) arch_re="$arch" ;;
  esac

  printf '%s\n' "$urls" |
    grep -Ei "$os" |
    grep -Ei "$arch_re" |
    grep -Ei '\.(tar\.gz|tgz|zip|tar)$' |
    head -n 1
}

extract_archive() {
  archive="$1"
  case "$archive" in
    *.tar.gz|*.tgz)
      need tar
      tar -xzf "$archive" -C "$tmp_dir/extract"
      ;;
    *.tar)
      need tar
      tar -xf "$archive" -C "$tmp_dir/extract"
      ;;
    *.zip)
      need unzip
      unzip -q "$archive" -d "$tmp_dir/extract"
      ;;
    *)
      fail "unsupported archive: $archive"
      ;;
  esac
}

find_binary() {
  for name in flyflor picoclaw flyflor.exe picoclaw.exe; do
    found="$(find "$tmp_dir/extract" -type f -name "$name" 2>/dev/null | head -n 1 || true)"
    if [ -n "$found" ]; then
      printf '%s' "$found"
      return 0
    fi
  done
  return 1
}

install_binary() {
  src="$1"
  mkdir -p "$install_dir"
  dst="$install_dir/flyflor"
  cp "$src" "$dst.tmp"
  chmod +x "$dst.tmp"
  mv -f "$dst.tmp" "$dst"
  if ln -sf "$dst" "$install_dir/picoclaw" 2>/dev/null; then
    :
  fi
  say "Installed Flyflor to $dst"
  case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) say "Add $install_dir to PATH to run flyflor from any shell." ;;
  esac
}

install_from_release() {
  os="$(os_name)"
  arch="$(arch_name)"
  say "Installing Flyflor release for $os/$arch from $repo ($version)"
  url="$(asset_url_from_release "$os" "$arch" || true)"
  [ -n "$url" ] || return 1
  archive_name="${url##*/}"
  archive="$tmp_dir/$archive_name"
  mkdir -p "$tmp_dir/extract"
  curl -fL "$url" -o "$archive"
  extract_archive "$archive"
  bin="$(find_binary || true)"
  [ -n "$bin" ] || fail "archive did not contain flyflor or picoclaw binary"
  install_binary "$bin"
}

install_from_source() {
  need git
  need make
  need go
  say "Building Flyflor from source: https://github.com/$repo"
  git clone --depth 1 "https://github.com/$repo.git" "$tmp_dir/src"
  (cd "$tmp_dir/src" && make build)
  [ -f "$tmp_dir/src/build/picoclaw" ] || fail "source build did not produce build/picoclaw"
  install_binary "$tmp_dir/src/build/picoclaw"
}

if [ "$from_source" = "1" ]; then
  install_from_source
else
  if ! install_from_release; then
    say "No matching release asset found; trying source build."
    if has git && has make && has go; then
      install_from_source
    else
      fail "source build requires git, make, and go. Install them or re-run after a Flyflor release asset is available."
    fi
  fi
fi

say "Try: flyflor version"
