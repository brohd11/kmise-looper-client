#!/usr/bin/env bash
# Build a static libusb for linking into kmise via cgo (gousb).
#
#   scripts/build-libusb.sh PREFIX
#
# Installs only libusb-1.0.a + headers + .pc into PREFIX, so the binary carries libusb and
# users install nothing. Honours GOARCH on macOS to cross-build amd64 on arm64 (and back).
# Then point cgo at it -- this script prints the exports, and appends them to $GITHUB_ENV
# when run in GitHub Actions:
#
#   PKG_CONFIG_PATH=PREFIX/lib/pkgconfig
#   CGO_LDFLAGS=$(pkg-config --static --libs libusb-1.0)   (+ -static on Windows)
#
# Windows: run from an MSYS2 UCRT64 shell (gcc, make, pkgconf, bzip2, diffutils).
set -euo pipefail

VERSION=1.0.29
SHA256=5977fc950f8d1395ccea9bd48c06b3f808fd3c2c961b44b0c2e6e29fc3a70a85
URL="https://github.com/libusb/libusb/releases/download/v$VERSION/libusb-$VERSION.tar.bz2"

[ "$#" -eq 1 ] || { sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
mkdir -p "$1"
prefix=$(cd "$1" && pwd)

os=$(uname -s)
case "$os" in
  MINGW*|MSYS*) prefix=$(cygpath -m "$prefix") ;; # native gcc/pkgconf need C:/ paths
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fsSL -o "$work/libusb.tar.bz2" "$URL"
if command -v sha256sum >/dev/null 2>&1; then
  echo "$SHA256  $work/libusb.tar.bz2" | sha256sum -c - >/dev/null
else
  echo "$SHA256  $work/libusb.tar.bz2" | shasum -a 256 -c - >/dev/null
fi
tar -xjf "$work/libusb.tar.bz2" -C "$work"

flags=(--prefix="$prefix" --enable-static --disable-shared --disable-examples-build --disable-tests-build)
cflags="-O2"
case "$os" in
  Linux)
    # No libudev: it has no static build. Enumeration falls back to sysfs, which is all
    # a CLI needs (no hotplug).
    flags+=(--disable-udev)
    ;;
  Darwin)
    export MACOSX_DEPLOYMENT_TARGET=11.0
    case "${GOARCH:-$(uname -m)}" in
      amd64|x86_64) cflags+=" -arch x86_64"; flags+=(--host=x86_64-apple-darwin) ;;
      arm64)        cflags+=" -arch arm64";  flags+=(--host=aarch64-apple-darwin) ;;
    esac
    ;;
esac

log="$work/build.log"
if ! (
  cd "$work/libusb-$VERSION"
  ./configure "${flags[@]}" CFLAGS="$cflags"
  make -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)"
  make install
) >"$log" 2>&1; then
  tail -n 40 "$log" >&2
  echo "libusb build failed" >&2
  exit 1
fi

pc_path="$prefix/lib/pkgconfig"
ldflags=$(PKG_CONFIG_PATH="$pc_path" pkg-config --static --libs libusb-1.0)
case "$os" in
  MINGW*|MSYS*) ldflags+=" -static" ;; # also fold in libgcc/winpthread
esac

echo "libusb $VERSION -> $prefix" >&2
echo "export PKG_CONFIG_PATH=\"$pc_path\""
echo "export CGO_LDFLAGS=\"$ldflags\""
if [ -n "${GITHUB_ENV:-}" ]; then
  {
    echo "PKG_CONFIG_PATH=$pc_path"
    # Each `msys2 {0}` step is a login shell, and MSYS2's /etc/profile overwrites
    # PKG_CONFIG_PATH. PKG_CONFIG_LIBDIR is left alone, so carry the path there too.
    case "$os" in
      MINGW*|MSYS*) echo "PKG_CONFIG_LIBDIR=$pc_path" ;;
    esac
    echo "CGO_LDFLAGS=$ldflags"
    echo "CGO_ENABLED=1"
  } >> "$GITHUB_ENV"
fi
