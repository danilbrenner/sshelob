#!/usr/bin/env sh

set -eu

REPO="danilbrenner/sshelob"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'error: required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

os_name() {
  uname_s=$(uname -s)
  case "$uname_s" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT)
      printf 'error: Windows is not supported by get.sh; use get.ps1 instead\n' >&2
      exit 1
      ;;
    *)
      printf 'error: unsupported OS: %s\n' "$uname_s" >&2
      exit 1
      ;;
  esac
}

arch_name() {
  uname_m=$(uname -m)
  case "$uname_m" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *)
      printf 'error: unsupported architecture: %s\n' "$uname_m" >&2
      exit 1
      ;;
  esac
}

need_cmd curl
need_cmd mktemp
need_cmd rm

goos=$(os_name)
goarch=$(arch_name)

latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")
tag=${latest_url##*/}

if [ -z "$tag" ]; then
  printf 'error: could not parse latest release tag\n' >&2
  exit 1
fi

ext='tar.gz'
binary='sshelob'
if [ "$goos" = 'windows' ]; then
  ext='zip'
  binary='sshelob.exe'
fi

asset="sshelob_${tag}_${goos}_${goarch}.${ext}"
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

archive_path="${tmp_dir}/${asset}"

printf 'downloading %s\n' "$url"
curl -fL --progress-bar "$url" -o "$archive_path"

if [ "$ext" = 'tar.gz' ]; then
  need_cmd tar
  tar -xzf "$archive_path" -C "$tmp_dir"
else
  need_cmd unzip
  unzip -qq "$archive_path" -d "$tmp_dir"
fi

if [ ! -f "${tmp_dir}/${binary}" ]; then
  printf 'error: expected binary %s not found in archive\n' "$binary" >&2
  exit 1
fi

install_path="./${binary}"
cp "${tmp_dir}/${binary}" "$install_path"

if [ "$goos" != 'windows' ]; then
  chmod +x "$install_path"
fi

printf 'installed %s from %s\n' "$install_path" "$tag"
