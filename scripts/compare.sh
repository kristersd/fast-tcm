#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

TCM="$REPO_ROOT/node_modules/.bin/tcm"
if [ ! -x "$TCM" ]; then
  echo "typed-css-modules not found. Run: npm install" >&2
  exit 1
fi

echo "Building fast-tcm..."
mkdir -p "$REPO_ROOT/bin"
go build -o "$REPO_ROOT/bin/ftcm" ./cmd/ftcm

UPSTREAM_DIR="$REPO_ROOT/testdata/upstream"

rm -rf "$TMPDIR/upstream-tcm" "$TMPDIR/upstream-ftcm"
cp -r "$UPSTREAM_DIR" "$TMPDIR/upstream-tcm"
cp -r "$UPSTREAM_DIR" "$TMPDIR/upstream-ftcm"

"$TCM" -p "**/*.{css,pcss}" "$TMPDIR/upstream-tcm" >/dev/null 2>&1 || true
"$REPO_ROOT/bin/ftcm" "$TMPDIR/upstream-ftcm" >/dev/null 2>&1

echo ""
echo "Comparing outputs..."
echo ""

MATCH=0
MISMATCH=0

for file in $(find "$UPSTREAM_DIR" \( -name '*.css' -o -name '*.pcss' \) | sort); do
    rel="${file#$UPSTREAM_DIR/}"
    tcm_dts="$TMPDIR/upstream-tcm/${rel}.d.ts"
    ftcm_dts="$TMPDIR/upstream-ftcm/${rel}.d.ts"

    if [ ! -f "$tcm_dts" ] && [ ! -f "$ftcm_dts" ]; then
        continue
    fi

    if [ ! -f "$tcm_dts" ]; then
        echo "MISS  $rel (upstream missing)"
        MISMATCH=$((MISMATCH+1))
        continue
    fi

    if [ ! -f "$ftcm_dts" ]; then
        echo "MISS  $rel (fast-tcm missing)"
        MISMATCH=$((MISMATCH+1))
        continue
    fi

    tcm_content=$(tr -d '\r' < "$tcm_dts")
    ftcm_content=$(tr -d '\r' < "$ftcm_dts")

    if [ "$tcm_content" = "$ftcm_content" ]; then
        echo "OK    $rel"
        MATCH=$((MATCH+1))
    else
        echo "DIFF  $rel"
        echo "--- upstream ---"
        echo "$tcm_content"
        echo "--- fast-tcm ---"
        echo "$ftcm_content"
        MISMATCH=$((MISMATCH+1))
    fi
done

echo ""
echo "Results: $MATCH matched, $MISMATCH differed"
if [ "$MISMATCH" -gt 0 ]; then
    exit 1
fi
