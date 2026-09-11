#!/bin/sh
# Installs the latest iagram release into /usr/local/bin (or $IAGRAM_INSTALL_DIR),
# verifying the archive against the release SHA256SUMS.
#
#   curl -fsSL https://raw.githubusercontent.com/iagram/iagram/main/install.sh | sh
set -eu

REPO="iagram/iagram"
DIR="${IAGRAM_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin|linux) ;;
  *) echo "unsupported OS: $os (download a release manually from https://github.com/$REPO/releases)" >&2; exit 1 ;;
esac

# IAGRAM_VERSION=v0.1.0-beta pins a version. Otherwise the latest stable
# release, falling back to the newest pre-release while none is stable yet.
tag="${IAGRAM_VERSION:-}"
if [ -z "$tag" ]; then
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
fi
if [ -z "$tag" ]; then
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases?per_page=1" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
fi
[ -n "$tag" ] || { echo "could not determine a release to install" >&2; exit 1; }
version=${tag#v}
archive="iagram_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading iagram $tag ($os/$arch)..."
curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/iagram_${version}_SHA256SUMS"

want=$(grep " $archive\$" "$tmp/SHA256SUMS" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/$archive" | cut -d' ' -f1); else got=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1); fi
[ "$want" = "$got" ] || { echo "checksum mismatch for $archive" >&2; exit 1; }

tar -xzf "$tmp/$archive" -C "$tmp" iagram
if [ -w "$DIR" ]; then install -m 0755 "$tmp/iagram" "$DIR/iagram"; else sudo install -m 0755 "$tmp/iagram" "$DIR/iagram"; fi
echo "Installed $("$DIR/iagram" version) to $DIR/iagram"
echo "Next: cd your-project && iagram init && iagram up"
