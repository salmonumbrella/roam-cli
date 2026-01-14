#!/usr/bin/env bash
set -euo pipefail

if [ -z "${ROAM_GRAPH_NAME:-}" ]; then
  echo "ROAM_GRAPH_NAME is required." >&2
  exit 1
fi

echo "Note: Requires Roam desktop app running with Encrypted local API enabled." >&2

BIN="${ROAM_BIN:-./roam}"
if [ ! -x "$BIN" ]; then
  if command -v go >/dev/null 2>&1; then
    tmpdir=$(mktemp -d)
    BIN="$tmpdir/roam"
    go build -o "$BIN" ./cmd/roam
  else
    echo "roam binary not found at $BIN and 'go' is unavailable. Set ROAM_BIN or build first." >&2
    exit 1
  fi
fi

run() {
  ROAM_GRAPH_NAME="$ROAM_GRAPH_NAME" "$BIN" --local "$@"
}

suffix=$(date +"%Y%m%d-%H%M%S")
page="CLI Smoke Test ${suffix}"
mdpage="CLI Smoke MD ${suffix}"

printf "Creating pages: %s / %s\n" "$page" "$mdpage"

run page create "$page"
run page from-markdown "$mdpage" --markdown $'# Heading\n\n- Item 1\n- Item 2'
run block from-markdown --page-title "$page" --markdown $'- A\n- B'
run search ui "CLI Smoke" --limit 5

if command -v python3 >/dev/null 2>&1; then
  uid_of() {
    run page get "$1" --output json | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get(":block/uid") or d.get("block/uid") or d.get("uid") or "")'
  }

  page_uid=$(uid_of "$page" || true)
  md_uid=$(uid_of "$mdpage" || true)

  if [ -n "$page_uid" ]; then
    run page delete "$page_uid" --yes
  else
    echo "Could not resolve UID for $page; delete manually if needed." >&2
  fi

  if [ -n "$md_uid" ]; then
    run page delete "$md_uid" --yes
  else
    echo "Could not resolve UID for $mdpage; delete manually if needed." >&2
  fi
else
  echo "python3 not available; skipping cleanup. Delete these pages manually:" >&2
  echo "- $page" >&2
  echo "- $mdpage" >&2
fi

printf "Smoke test complete.\n"
