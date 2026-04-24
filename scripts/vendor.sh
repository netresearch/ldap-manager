#!/usr/bin/env bash
# Refreshes pinned frontend vendor files from scripts/vendor.lock.
# Aborts if SHA256 verification fails.

set -euo pipefail

LOCK="$(dirname "$0")/vendor.lock"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [[ ! -f "$LOCK" ]]; then
  echo "missing $LOCK" >&2
  exit 1
fi

fail=0
while IFS=' ' read -r dest url sha; do
  [[ -z "$dest" || "$dest" == \#* ]] && continue
  sha="${sha%$'\r'}"

  if [[ -z "$url" || -z "$sha" ]]; then
    echo "malformed lock line for $dest" >&2
    fail=1
    continue
  fi

  abs="$ROOT/$dest"
  mkdir -p "$(dirname "$abs")"
  echo "fetching $url"
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  curl --fail --silent --show-error --location "$url" -o "$tmp"

  got="$(sha256sum "$tmp" | awk '{print $1}')"
  if [[ "$got" != "$sha" ]]; then
    echo "SHA256 mismatch for $dest" >&2
    echo "  expected: $sha" >&2
    echo "  got:      $got" >&2
    fail=1
    rm -f "$tmp"
    trap - EXIT
    continue
  fi

  mv "$tmp" "$abs"
  chmod 0644 "$abs"
  trap - EXIT
  echo "  -> $dest"
done < "$LOCK"

if [[ $fail -ne 0 ]]; then
  echo "one or more vendor files failed SHA verification" >&2
  exit 1
fi

echo "vendor refresh OK"
