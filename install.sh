#!/usr/bin/env bash
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
response="$(fetch "$api_url")"
tag="$(printf '%s' "$response" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
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
download "$base_url/$asset" "$tmp_dir/$asset" || {
  say "failed to download $asset"
  exit 1
}
download "$base_url/checksums.txt" "$tmp_dir/checksums.txt" || {
  say "failed to download checksums.txt"
  exit 1
}

expected_checksum="$(grep -E "^[a-f0-9]+\s+\*?$asset\$" "$tmp_dir/checksums.txt" | awk '{print $1}')"
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

# Add to PATH if not already present
case ":$PATH:" in
  *":$install_dir:"*)
    say "tiki is already in PATH"
    ;;
  *)
    say "$install_dir is not in PATH, adding it to shell configuration"

    # Determine which shell config files to update based on user's actual shell
    shell_configs=()

    # Get the user's login shell
    user_shell="${SHELL:-}"

    # Detect shell-specific config files that exist
    case "$user_shell" in
      */bash)
        [ -f "$HOME/.bash_profile" ] && shell_configs+=("$HOME/.bash_profile")
        [ -f "$HOME/.bashrc" ] && shell_configs+=("$HOME/.bashrc")
        ;;
      */zsh)
        [ -f "$HOME/.zshrc" ] && shell_configs+=("$HOME/.zshrc")
        ;;
      */fish)
        [ -f "$HOME/.config/fish/config.fish" ] && shell_configs+=("$HOME/.config/fish/config.fish")
        ;;
      */ksh)
        [ -f "$HOME/.kshrc" ] && shell_configs+=("$HOME/.kshrc")
        ;;
      *)
        # Unknown or no SHELL set - check for any existing config files
        [ -f "$HOME/.bash_profile" ] && shell_configs+=("$HOME/.bash_profile")
        [ -f "$HOME/.bashrc" ] && shell_configs+=("$HOME/.bashrc")
        [ -f "$HOME/.zshrc" ] && shell_configs+=("$HOME/.zshrc")
        [ -f "$HOME/.profile" ] && shell_configs+=("$HOME/.profile")
        ;;
    esac

    # If no config files found, use POSIX-compliant .profile
    if [ ${#shell_configs[@]} -eq 0 ]; then
      shell_configs+=("$HOME/.profile")
      say "no shell config found, will create $HOME/.profile"
    fi

    first_config="${shell_configs[0]}"
    for config_file in "${shell_configs[@]}"; do
      # Check if any tiki PATH entry exists
      if [ -f "$config_file" ] && grep -qF "$install_dir" "$config_file"; then
        say "tiki PATH entry already exists in $config_file"
        continue
      fi

      # Create parent directories if needed
      config_dir="$(dirname "$config_file")"
      if [ ! -d "$config_dir" ]; then
        mkdir -p "$config_dir" || {
          say "failed to create directory $config_dir"
          exit 1
        }
      fi

      # Determine the correct syntax based on the shell
      case "$config_file" in
        */config.fish)
          # Fish shell syntax
          path_line="set -gx PATH \"$install_dir\" \$PATH"
          ;;
        *)
          # POSIX shell syntax (bash, zsh, ksh, sh, etc.)
          path_line="export PATH=\"$install_dir:\$PATH\""
          ;;
      esac

      # Add the PATH export
      say "adding PATH to $config_file"
      printf '%s\n# Added by tiki installer\n%s\n' '' "$path_line" >> "$config_file" || {
        say "failed to write to $config_file"
        exit 1
      }
    done

    say "PATH updated. Please run: source $first_config or start a new terminal session"
    ;;
esac

say "run: tiki --version"
