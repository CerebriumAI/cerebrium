#!/usr/bin/env bash
set -euo pipefail

out=THIRD_PARTY_NOTICES
go run github.com/google/go-licenses/v2@v2.0.1 report ./cmd/cerebrium \
  --ignore github.com/cerebriumai/cerebrium \
  --template .github/third-party-notices.tpl > "$out"

modcache="$(go env GOMODCACHE)"
go list -deps -f '{{with .Module}}{{.Dir}}{{end}}' ./cmd/cerebrium | sort -u | while read -r dir; do
  for f in "$dir"/NOTICE*; do
    [ -f "$f" ] || continue
    printf '\n================================================================================\nNOTICE: %s\n================================================================================\n\n' "${dir#"$modcache"/}"
    cat "$f"
  done
done >> "$out"
