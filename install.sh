#!/usr/bin/env sh
set -euo pipefail

repo_owner="boolean-maybe"
repo_name="tiki"

say() {
  printf '%s\n' "$*"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

fetch() {
  if need_cmd curl; then
    curl -fsSL "$1"
    return
  fi
  if need_cmd wget; then
    wget -qO- "$1"
    return
  fi
  say "curl or wget is required"
  exit 1
}

download() {
  if need_cmd curl; then
    curl -fL "$1" -o "$2"
    return
  fi
  if need_cmd wget; then
    wget -qO "$2" "$1"
    return
  fi
  say "curl or wget is required"
  exit 1
}

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|linux) ;;
  *)
    say "unsupported os: $os"
    say "use install.ps1 on windows"
    exit 1
    ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    say "unsupported architecture: $arch"
    exit 1
    ;;
esac

api_url="https://api.github.com/repos/$repo_owner/$repo_name/releases/latest"
tag="$(fetch "$api_url" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
if [ -z "$tag" ]; then
  say "failed to resolve latest release tag"
  exit 1
fi

version="${tag#v}"
asset="tiki_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repo_owner/$repo_name/releases/download/$tag"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

say "downloading $asset"
download "$base_url/$asset" "$tmp_dir/$asset"
download "$base_url/checksums.txt" "$tmp_dir/checksums.txt"

expected_checksum="$(grep "  $asset\$" "$tmp_dir/checksums.txt" | awk '{print $1}')"
if [ -z "$expected_checksum" ]; then
  say "checksum not found for $asset"
  exit 1
fi

if need_cmd shasum; then
  actual_checksum="$(shasum -a 256 "$tmp_dir/$asset" | awk '{print $1}')"
elif need_cmd sha256sum; then
  actual_checksum="$(sha256sum "$tmp_dir/$asset" | awk '{print $1}')"
else
  say "sha256 tool not found (need shasum or sha256sum)"
  exit 1
fi

if [ "$expected_checksum" != "$actual_checksum" ]; then
  say "checksum mismatch"
  exit 1
fi

tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
if [ ! -f "$tmp_dir/tiki" ]; then
  say "tiki binary not found in archive"
  exit 1
fi

install_dir="${TIKI_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -w "/usr/local/bin" ]; then
    install_dir="/usr/local/bin"
  else
    install_dir="$HOME/.local/bin"
  fi
fi

mkdir -p "$install_dir"
install -m 0755 "$tmp_dir/tiki" "$install_dir/tiki"

say "installed tiki to $install_dir/tiki"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    say "add to path: export PATH=\"$install_dir:\$PATH\""
    ;;
esac
say "run: tiki --version"
