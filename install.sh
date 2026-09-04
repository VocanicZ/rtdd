#!/bin/sh
# Install the latest (or a pinned) rtdd release binary.
#
#   curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
#
# Env vars:
#   RTDD_VERSION      tag to install, e.g. v1.2.3 (default: latest release)
#   RTDD_INSTALL_DIR  install destination (default: /usr/local/bin, falling back to
#                     ~/.local/bin when that is not writable)
#   RTDD_BASE_URL     override the release-assets base URL (for testing against a
#                     local fixture server; undocumented for end users)
#   RTDD_API_URL      override the GitHub "latest release" API URL (same purpose)
set -eu

REPO="VocanicZ/rtdd"
BASE_URL="${RTDD_BASE_URL:-https://github.com/${REPO}/releases/download}"
API_URL="${RTDD_API_URL:-https://api.github.com/repos/${REPO}/releases/latest}"

log() { printf '%s\n' "$*" >&2; }
die() {
	log "install.sh: $*"
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need_cmd uname
need_cmd tar
need_cmd mktemp

if command -v curl >/dev/null 2>&1; then
	DOWNLOADER=curl
elif command -v wget >/dev/null 2>&1; then
	DOWNLOADER=wget
else
	die "required command not found: curl or wget"
fi

if command -v sha256sum >/dev/null 2>&1; then
	SHASUM="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	SHASUM="shasum -a 256"
else
	die "required command not found: sha256sum or shasum"
fi

# fetch_to <url> <dest-file> -- download url to dest, aborting (set -e) on failure.
fetch_to() {
	if [ "$DOWNLOADER" = curl ]; then
		curl -fsSL "$1" -o "$2"
	else
		wget -q "$1" -O "$2"
	fi
}

# fetch_stdout <url> -- download url and print its body.
fetch_stdout() {
	if [ "$DOWNLOADER" = curl ]; then
		curl -fsSL "$1"
	else
		wget -q -O - "$1"
	fi
}

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) die "unsupported OS: $(uname -s); rtdd ships linux and darwin binaries only" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) die "unsupported architecture: $(uname -m); rtdd ships amd64 and arm64 binaries only" ;;
	esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

VERSION="${RTDD_VERSION:-}"
if [ -z "$VERSION" ]; then
	log "resolving the latest rtdd release..."
	VERSION="$(fetch_stdout "$API_URL" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
	[ -n "$VERSION" ] || die "could not resolve the latest release version from $API_URL"
fi

VERSION_NUM="${VERSION#v}"
ARCHIVE="rtdd_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"

INSTALL_DIR="${RTDD_INSTALL_DIR:-}"
if [ -z "$INSTALL_DIR" ]; then
	if [ -w /usr/local/bin ] 2>/dev/null; then
		INSTALL_DIR=/usr/local/bin
	else
		INSTALL_DIR="$HOME/.local/bin"
	fi
fi
mkdir -p "$INSTALL_DIR" || die "cannot create install dir: $INSTALL_DIR"
[ -w "$INSTALL_DIR" ] || die "install dir not writable: $INSTALL_DIR"

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/rtdd-install.XXXXXX")"
cleanup() { rm -rf "$WORKDIR"; }
trap cleanup EXIT INT TERM

log "downloading ${ARCHIVE} (${VERSION})..."
fetch_to "${BASE_URL}/${VERSION}/${ARCHIVE}" "${WORKDIR}/${ARCHIVE}"
fetch_to "${BASE_URL}/${VERSION}/checksums.txt" "${WORKDIR}/checksums.txt"

log "verifying checksum..."
EXPECTED="$(grep " ${ARCHIVE}\$" "${WORKDIR}/checksums.txt" | awk '{print $1}')"
[ -n "$EXPECTED" ] || die "no checksum entry for ${ARCHIVE} in checksums.txt"
ACTUAL="$(cd "$WORKDIR" && $SHASUM "${ARCHIVE}" | awk '{print $1}')"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum mismatch for ${ARCHIVE}: expected ${EXPECTED}, got ${ACTUAL}"

log "extracting..."
tar -xzf "${WORKDIR}/${ARCHIVE}" -C "${WORKDIR}" rtdd
[ -f "${WORKDIR}/rtdd" ] || die "${ARCHIVE} did not contain an rtdd binary"
chmod +x "${WORKDIR}/rtdd"

mv -f "${WORKDIR}/rtdd" "${INSTALL_DIR}/rtdd"
log "rtdd ${VERSION} installed to ${INSTALL_DIR}/rtdd"
