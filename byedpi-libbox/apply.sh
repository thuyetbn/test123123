#!/usr/bin/env bash
# Copy the embedded ByeDPI (ciadpi) integration into the sing-box source tree
# and hook it into libbox. Run from CI after checking out sing-box.
#
# Usage: apply.sh <path-to-sing-box-checkout>
set -euo pipefail

SING_BOX_DIR="${1:?usage: apply.sh <sing-box-checkout>}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEST="$SING_BOX_DIR/experimental/libbox/byedpi"

if [ ! -f "$SING_BOX_DIR/experimental/libbox/command_server.go" ]; then
    echo "[apply] ERROR: $SING_BOX_DIR does not look like a sing-box checkout"
    exit 1
fi

echo "[apply] vendoring ciadpi C sources + Go bridge into $DEST"
mkdir -p "$DEST"
cp "$HERE"/csrc/*.c "$HERE"/csrc/*.h "$DEST"/
cp "$HERE"/shim/byedpi_shim.h "$HERE"/shim/byedpi_shim.c "$DEST"/
cp "$HERE"/go/byedpi.go "$HERE"/go/inject.go "$DEST"/

echo "[apply] patching experimental/libbox/command_server.go"
python3 "$HERE/patch_hook.py" "$SING_BOX_DIR/experimental/libbox/command_server.go"

# Keep gofmt happy so gomobile bind never trips on formatting-sensitive tools.
cd "$SING_BOX_DIR"
gofmt -w experimental/libbox/command_server.go experimental/libbox/byedpi/*.go || true
go vet ./experimental/libbox/... >/dev/null 2>&1 || true

echo "[apply] done"
