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
# The hash names the logic that decides. The web view (internal/ui, cmd/hc/ui.go) only
# reads run logs, so changing it does not change the hash.
logic=$(echo "$files" | grep -v -e '^internal/ui/' -e '^cmd/hc/ui\.go$')
hash=$( (cat go.mod; for f in $logic; do echo "== $f"; cat "$f"; done) | shasum -a 256 | cut -c1-12)
module=$(sed -n 's/^module[[:space:]]*//p' go.mod | head -1)
version=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\(.*\)"$/\1/p' internal/engine/version.go | head -1)

mkdir -p runs/engines bin
snapshot="runs/engines/${version}-${hash}.tar.gz"
if [ ! -f "$snapshot" ]; then
  tar -czf "$snapshot" go.mod scripts internal/ui/static $files
  printf '{"version":"%s","hash":"%s","builtAt":"%s"}\n' "$version" "$hash" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> runs/engines/builds.jsonl
fi

# Build beside the target and rename: a rename leaves running copies of the old binary
# alone, while writing over bin/hc in place gets them killed (macOS code signing).
go build -ldflags "-X ${module}/internal/engine.SourceHash=${hash}" -o bin/hc.new ./cmd/hc
mv -f bin/hc.new bin/hc
echo "built bin/hc  engine ${version}  source ${hash}  snapshot ${snapshot}"
