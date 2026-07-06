#!/bin/sh
# install.sh — fetch a prebuilt `constitution` binary from GitHub Releases and
# drop it on PATH. Neutral, OS/arch-aware, no toolchain required.
#
#   curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh
#   curl -sSfL .../install.sh | sh -s -- v0.1.0          # pin a version
#   curl -sSfL .../install.sh | BINDIR=/usr/local/bin sh # choose install dir
#
# Env / args:
#   $1 or CONSTITUTION_VERSION   version tag (e.g. v0.1.0); default: latest
#   BINDIR                        install dir; default: ~/.local/bin (user space)
#
# Installs the prebuilt release binary only. It never builds from source or
# installs a Go toolchain — that is the whole point (works in a bare container).
set -eu

REPO="kentra-io/adr-sourced-constitution"
BINDIR="${BINDIR:-$HOME/.local/bin}"
VERSION="${1:-${CONSTITUTION_VERSION:-latest}}"

err() { echo "install.sh: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

have curl || err "curl is required"
have tar  || err "tar is required"

# --- OS ---
case "$(uname -s)" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS $(uname -s) (Windows: use the .zip asset manually)" ;;
esac

# --- arch (normalize to Go's amd64/arm64) ---
case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported arch $(uname -m)" ;;
esac

# --- resolve latest via the releases/latest redirect (no jq/API token needed) ---
if [ "$VERSION" = latest ]; then
  VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest" | sed 's#.*/tag/##')"
  [ -n "$VERSION" ] || err "could not resolve the latest release tag"
fi

ver_no_v="${VERSION#v}"                       # tag v0.1.0 -> asset 0.1.0
asset="constitution_${ver_no_v}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${VERSION}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "install.sh: downloading ${asset} (${VERSION})"
curl -fsSL "${base}/${asset}"      -o "${tmp}/${asset}" || err "download failed: ${base}/${asset}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" || err "checksums download failed"

# --- verify sha256 (sha256sum on Linux, shasum -a 256 on macOS) ---
( cd "$tmp"
  if have sha256sum; then
    sha256sum -c --ignore-missing checksums.txt >/dev/null
  elif have shasum; then
    want="$(grep " ${asset}\$" checksums.txt | awk '{print $1}')"
    [ -n "$want" ] || err "no checksum for ${asset}"
    echo "${want}  ${asset}" | shasum -a 256 -c - >/dev/null
  else
    err "no sha256 tool (sha256sum or shasum) to verify the download"
  fi
) || err "checksum verification failed for ${asset}"

mkdir -p "$BINDIR"
tar -xzf "${tmp}/${asset}" -C "$BINDIR" constitution
chmod +x "${BINDIR}/constitution"

echo "install.sh: installed constitution ${VERSION} -> ${BINDIR}/constitution"

# --- PATH hint (user-space BINDIR is often not on PATH yet) ---
case ":${PATH}:" in
  *":${BINDIR}:"*) : ;;
  *) echo "install.sh: note — ${BINDIR} is not on PATH; add it, e.g.:"
     echo "  export PATH=\"${BINDIR}:\$PATH\"" ;;
esac

"${BINDIR}/constitution" --version
