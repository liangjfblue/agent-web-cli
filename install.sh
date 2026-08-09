#!/bin/sh
# install.sh — one-line installer for awc (agent-web-cli).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
#
# What it does:
#   1. Detects OS + CPU (with Apple Rosetta correction)
#   2. Downloads the matching binary tarball from GitHub Releases
#   3. Extracts to ~/.awc (bin/ + extension/ + install-native-host.js)
#   4. Adds ~/.awc/bin to PATH (in ~/.zshrc or ~/.bashrc)
#   5. Runs `awc sys:setup` to register the native host + guide extension install
#
# No Node, no sudo required. macOS/Linux only.
# Windows users: download awc-windows-amd64-<ver>.zip from Releases, or use WSL.
set -eu

OWNER="liangjfblue"
REPO="agent-web-cli"
INSTALL_DIR="${HOME}/.awc"
BIN_DIR="${INSTALL_DIR}/bin"

# ── Colors (only if stdout is a tty) ──
if [ -t 1 ]; then
    RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'; BOLD='\033[1m'; RESET='\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; BOLD=''; RESET=''
fi
info()  { printf "${GREEN}✓${RESET} %s\n" "$*"; }
warn()  { printf "${YELLOW}⚠${RESET} %s\n" "$*"; }
bold()  { printf "${BOLD}%s${RESET}\n" "$*"; }
err()   { printf "${RED}✗ %s${RESET}\n" "$*" >&2; exit 1; }

# ── Step 1: detect platform ──
bold "awc installer"
echo "────────────────────────────────────────"

os=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin / linux
arch=$(uname -m)                                # arm64 / x86_64 / aarch64

# Normalize arch aliases
case "$arch" in
    x86_64|amd64|x64)  arch="amd64" ;;
    aarch64|arm64)     arch="arm64" ;;
esac

# Apple Rosetta correction: a process running under Rosetta 2 reports x86_64
# even on Apple Silicon. Re-check via sysctl so we grab the native arm64 build.
if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
    if sysctl -n hw.optional.arm64 2>/dev/null | grep -q 1; then
        arch="arm64"
    fi
fi

case "${os}-${arch}" in
    darwin-arm64|darwin-amd64|linux-amd64|linux-arm64) : ;;  # supported
    *) err "unsupported platform: ${os}-${arch}. See Releases page for available builds." ;;
esac
info "platform: ${os}-${arch}"

# ── Step 2: resolve the latest release tag ──
# GitHub API returns the latest non-prerelease; fall back to 'latest' redirect.
tag=$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" \
      2>/dev/null | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || true)
if [ -z "$tag" ]; then
    err "could not resolve latest release. Check https://github.com/${OWNER}/${REPO}/releases"
fi
info "version:  ${tag}"

# The tarball embeds the version number (cross-build.sh names it awc-<os>-<arch>-<ver>.tar.gz).
# Strip a leading 'v' from the tag for the filename.
ver=$(printf '%s' "$tag" | sed 's/^v//')
asset="awc-${os}-${arch}-${ver}.tar.gz"
url="https://github.com/${OWNER}/${REPO}/releases/download/${tag}/${asset}"

# ── Step 3: download + extract ──
info "downloading ${asset}..."
mkdir -p "$INSTALL_DIR"
# The tarball contains bin/ + extension/ + install-native-host.js at its root,
# so strip the top-level platform dir (awc-<os>-<arch>/) when extracting.
curl -fL --progress-bar -o "/tmp/${asset}" "$url" || err "download failed: $url"
tar xzf "/tmp/${asset}" -C "$INSTALL_DIR" --strip-components=1 || err "extraction failed"
rm -f "/tmp/${asset}"
[ -x "${BIN_DIR}/awc" ] || err "awc binary missing after extract (${BIN_DIR}/awc)"
info "installed to ${BIN_DIR}"

# ── Step 4: add ~/.awc/bin to PATH ──
# Append to the right rc file for the user's shell, but only if not already there.
add_path_line='export PATH="$HOME/.awc/bin:$PATH"'
rc_file=""
case "${SHELL:-}" in
    */zsh)  rc_file="${HOME}/.zshrc" ;;
    */bash) rc_file="${HOME}/.bashrc" ;;
    *)      rc_file="${HOME}/.profile" ;;
esac
if [ -n "$rc_file" ] && ! grep -q '.awc/bin' "$rc_file" 2>/dev/null; then
    printf '\n# added by awc installer\n%s\n' "$add_path_line" >> "$rc_file"
    info "PATH entry added to ${rc_file}"
    warn "run \`source ${rc_file}\` or open a new terminal for PATH to take effect"
else
    info "PATH already configured (${rc_file})"
fi

# ── Step 5: hand off to awc sys:setup ──
# sys:setup does the rest: registers the native-messaging host, configures
# PATH, installs skills, and prints the exact extension folder to load.
echo
bold "running awc sys:setup..."
export PATH="${BIN_DIR}:$PATH"
"${BIN_DIR}/awc" sys:setup || warn "sys:setup did not complete — you can re-run it later with: awc sys:setup"

echo
bold "done."
echo "next:"
echo "  awc --version     # verify install"
echo "  awc sys:status    # check host + extension"
echo "  # then load the extension Chrome reported during sys:setup, at chrome://extensions"
