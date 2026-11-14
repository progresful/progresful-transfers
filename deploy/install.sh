#!/usr/bin/env bash
# Installs or updates the runner and the board on the server, in the way its other sites
# are set up: an own user and folder (/srv/bahn), systemd units, one Caddy snippet.
#
#   DEPLOY_HOST=user@host deploy/install.sh            binary, units, snippet; restarts the board
#   DEPLOY_HOST=user@host deploy/install.sh -key Team  also installs that team's key from .env
#
# It never starts or stops the runner of a final: that is done by hand (see bahn-final.service).
set -euo pipefail
cd "$(dirname "$0")/.."
host="${DEPLOY_HOST:?set DEPLOY_HOST=user@host}"
team=""
if [ "${1:-}" = "-key" ]; then team="${2:?-key needs a team name as in .env}"; fi

files=$(find cmd internal -name '*.go' -type f | LC_ALL=C sort)
logic=$(echo "$files" | grep -v -e '^internal/ui/' -e '^cmd/hc/ui\.go$')
hash=$( (cat go.mod; for f in $logic; do echo "== $f"; cat "$f"; done) | shasum -a 256 | cut -c1-12)
module=$(sed -n 's/^module[[:space:]]*//p' go.mod | head -1)
version=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\(.*\)"$/\1/p' internal/engine/version.go | head -1)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X ${module}/internal/engine.SourceHash=${hash}" -o "$out/hc" ./cmd/hc
cp deploy/bahn-ui.service deploy/bahn-final.service deploy/bahn-watch.service deploy/bahn.caddy "$out/"
[ -f runs/map.json ] && cp runs/map.json "$out/map.json"
echo "built linux/amd64 hc  engine ${version}  source ${hash}"

stage="/tmp/bahn-deploy-$$"
ssh "$host" "mkdir -m 700 $stage"
scp -q "$out"/* "$host:$stage/"

ssh "$host" "sudo bash -s" <<REMOTE
set -euo pipefail
id bahn >/dev/null 2>&1 || useradd --system --home-dir /srv/bahn --shell /usr/sbin/nologin bahn
install -d -o bahn -g bahn -m 750 /srv/bahn /srv/bahn/runs
# Caddy does not read /srv/bahn; the binary is replaced by a rename, so a running runner keeps its copy.
install -o root -g bahn -m 750 $stage/hc /srv/bahn/hc.new && mv -f /srv/bahn/hc.new /srv/bahn/hc
[ -f $stage/map.json ] && [ ! -f /srv/bahn/runs/map.json ] && install -o bahn -g bahn -m 640 $stage/map.json /srv/bahn/runs/map.json
install -o root -g root -m 644 $stage/bahn-ui.service $stage/bahn-final.service $stage/bahn-watch.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --quiet bahn-ui
systemctl restart bahn-ui
# The snippet: keep the old one until the whole config is known to be valid.
snippet=/etc/caddy/conf.d/bahn.caddy
if ! cmp -s $stage/bahn.caddy \$snippet; then
  [ -f \$snippet ] && cp -p \$snippet $stage/bahn.caddy.old
  install -o root -g root -m 644 $stage/bahn.caddy \$snippet
  if caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >$stage/validate.log 2>&1; then
    systemctl reload caddy
    echo "caddy: snippet installed, config valid, reloaded"
  else
    if [ -f $stage/bahn.caddy.old ]; then mv -f $stage/bahn.caddy.old \$snippet; else rm -f \$snippet; fi
    echo "caddy: the config was NOT valid with the new snippet; it was taken out again, nothing reloaded" >&2
    tail -5 $stage/validate.log >&2
    exit 1
  fi
else
  echo "caddy: snippet unchanged"
fi
REMOTE

if [ -n "$team" ]; then
  # The key goes over the ssh channel on stdin: never in a command line, never printed.
  key_line=$(awk -v team="$team" '
    /^# team:/ { hit = ($3 == team); next }
    hit && /SPREAD_KEY=/ { sub(/^#[[:space:]]*/, ""); print; found = 1; exit }
    END { if (!found) exit 1 }' .env) || { echo "no key of team $team in .env" >&2; exit 1; }
  printf '%s\n' "$key_line" | ssh "$host" "sudo install -o bahn -g bahn -m 600 /dev/stdin /srv/bahn/.env"
  unset key_line
  echo "key of $team installed; fingerprint on the server: $(ssh "$host" "sudo sed -n 's/^SPREAD_KEY=//p' /srv/bahn/.env | tr -d '\n' | sha256sum | cut -c1-8")"
fi

ssh "$host" "rm -rf $stage; systemctl is-active bahn-ui; sudo -u bahn /srv/bahn/hc 2>&1 | head -1; systemctl is-active bahn-final || true"
