#!/usr/bin/env bash
# Builds bin/hc stamped with the hash of its source, and keeps a snapshot of that source
# under runs/engines/<version>-<hash>.tar.gz. Every run logs version + hash, so any past
# result can be traced to (and rebuilt from) the logic that produced it:
#
#   mkdir /tmp/old && tar -xzf runs/engines/e1-<hash>.tar.gz -C /tmp/old
#   (cd /tmp/old && go build -o hc-old ./cmd/hc)
set -euo pipefail
cd "$(dirname "$0")/.."

files=$(find cmd internal -name '*.go' -type f | LC_ALL=C sort)
hash=$( (cat go.mod; for f in $files; do echo "== $f"; cat "$f"; done) | shasum -a 256 | cut -c1-12)
version=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\(.*\)"$/\1/p' internal/engine/version.go | head -1)

mkdir -p runs/engines bin
snapshot="runs/engines/${version}-${hash}.tar.gz"
if [ ! -f "$snapshot" ]; then
  tar -czf "$snapshot" go.mod scripts $files
  printf '{"version":"%s","hash":"%s","builtAt":"%s"}\n' "$version" "$hash" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> runs/engines/builds.jsonl
fi

go build -ldflags "-X highwaycoins/internal/engine.SourceHash=${hash}" -o bin/hc ./cmd/hc
echo "built bin/hc  engine ${version}  source ${hash}  snapshot ${snapshot}"
