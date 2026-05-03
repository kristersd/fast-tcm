#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
rm -rf npm/bin
mkdir -p npm/bin

targets=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/arm64"
  "linux/amd64"
  "windows/arm64"
  "windows/amd64"
)

for t in "${targets[@]}"; do
  goos=${t%/*}
  goarch=${t#*/}
  outdir="npm/bin/${goos}-${goarch}"
  mkdir -p "$outdir"
  if [ "$goos" = "windows" ]; then
    outfile="$outdir/ftcm.exe"
  else
    outfile="$outdir/ftcm"
  fi
  echo "Building $goos/$goarch → $outfile"
  GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath \
    -ldflags="-s -w" \
    -o "$outfile" \
    ./cmd/ftcm
done
