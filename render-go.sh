#!/usr/bin/env bash
# Render this repo's shared Go installers, makefile, workflows, and changelog config.
#
#   ./render-go.sh           rewrite each target, report updated/unchanged
#   ./render-go.sh --check   verify each matches; exit 1 on drift
#
# Copy this entry script into a consumer repo as render-go.sh. Bootstrap makefile
# with APP_NAME, VERSION_PKG, PLATFORMS; installers need their own config and marker.
# Adjust TARGETS for this repo; libraries need only the test workflow.
# Templates and rendering live in the pinned sh-templates submodule.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT" || exit 1
RENDERER="$ROOT/sh-templates/go/render-target.sh"

TARGETS=(
  "install.sh"
  "install.ps1"
  "makefile"
  "cliff.toml"
  # test.yml and release.yml are custom: libusb needs cgo, which the shared
  # single-runner CGO_ENABLED=0 workflows can't build.
)

case "${1:-}" in
  --check|"") ;;
  -h|--help) sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
  *) echo "unknown option: $1" >&2; exit 2 ;;
esac
[ "$#" -le 1 ] || { echo "error: expected at most one option" >&2; exit 2; }

if [ ! -x "$RENDERER" ]; then
  echo "$(basename "$0"): required submodule renderer not found: $RENDERER" >&2
  echo "Initialize it with: git submodule update --init sh-templates" >&2
  exit 1
fi

fail=0
for target in "${TARGETS[@]}"; do
  "$RENDERER" "$@" "$target" || fail=1
done
exit "$fail"
