#!/usr/bin/env bash
# CI-B7: top-level declaration-block multiset diff (i5-plan.md §6.1 spec,
# materialized in Phase A per §10-4; consumed by every I5 phase A~G).
#
# Contract:
#   go-decl-blocks-diff.sh [--blocks-only|--lines-only|--body-only]
#                          <original.go> <new1.go> [<new2.go> ...]
#     Compares ORIGINAL against the union of the NEW+REMAINING files with three
#     order-free multiset checks (all must pass in "all" mode):
#       blocks - byte-identical declaration blocks (a `^(func|type|var|const) `
#                line through its closing `^}`), sorted, diffed  [CI-B7 core]
#       lines  - declaration-line multiset (`^func|^type|^var|^const`)  [CI-B6
#                supplement: catches one-line declarations (`type X int`) the
#                block scan cannot close — documented plan limitation]
#       body   - non-blank line multiset below the package/import header
#                (catches `const (...)` group internals and comments that the
#                other two checks are blind to)
#   go-decl-blocks-diff.sh --dump-blocks <file.go> [<file.go> ...]
#     Emit the sorted block multiset (PR baseline attachment per plan §6.1).
#
# Known limits (by design, symmetric on both sides): blank-line-only changes
# and repositioning of inter-block comments pass; raw strings containing
# `^}$` shift block boundaries identically in original and moved copies.
#
# Exit: 0 = all requested checks identical, 1 = any mismatch, 2 = usage error.
set -u
export LC_ALL=C

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | tail -n +2 >&2
  exit 2
}

extract_blocks() {
  awk '
    FNR == 1 { inblk = 0 }
    /^(func|type|var|const) / { buf = $0; inblk = 1; next }
    inblk {
      buf = buf "\n" $0
      if ($0 ~ /^}$/) { print buf; inblk = 0 }
      next
    }
  ' "$@"
}

extract_lines() {
  grep -hE '^(func|type|var|const) ' "$@"
}

extract_body() {
  awk '
    FNR == 1 { seenpkg = 0; inimp = 0 }
    !seenpkg { if ($0 ~ /^package /) seenpkg = 1; next }
    inimp { if ($0 ~ /^\)$/) inimp = 0; next }
    /^import \(/ { inimp = 1; next }
    /^import[ \t]/ { next }
    /^[ \t]*$/ { next }
    { print }
  ' "$@"
}

MODE="all"
case "${1:-}" in
--blocks-only | --lines-only | --body-only)
  MODE="${1#--}"; MODE="${MODE%-only}"
  shift
  ;;
--dump-blocks)
  shift
  [ $# -ge 1 ] || { echo "usage: $0 --dump-blocks <file.go>..." >&2; exit 2; }
  for f in "$@"; do
    [ -f "$f" ] || { echo "not a file: $f" >&2; exit 2; }
  done
  extract_blocks "$@" | sort
  exit 0
  ;;
esac

[ $# -ge 2 ] || usage
ORIG=$1
shift
[ -f "$ORIG" ] || { echo "not a file: $ORIG" >&2; exit 2; }
for f in "$@"; do
  [ -f "$f" ] || { echo "not a file: $f" >&2; exit 2; }
done

TMP="$(mktemp -d)" || exit 2
trap 'rm -rf "$TMP"' EXIT

count_blocks() { grep -cE '^(func|type|var|const) ' "$1"; }
count_lines() { wc -l < "$1"; }

run_check() { # name extractor counter files...
  local name="$1" fn="$2" ctr="$3"
  shift 3
  "$fn" "$ORIG" | sort > "$TMP/a"
  "$fn" "$@" | sort > "$TMP/b"
  local na nb
  na=$("$ctr" "$TMP/a")
  nb=$("$ctr" "$TMP/b")
  if diff -u "$TMP/a" "$TMP/b" > "$TMP/d"; then
    echo "[$name] OK ($na units identical)"
    return 0
  fi
  echo "[$name] DIFF (original $na units vs new $nb units):" >&2
  if [ "$(wc -l < "$TMP/d")" -gt 40 ]; then
    sed -n '1,40p' "$TMP/d" >&2
    echo "... (truncated; full diff at $TMP/d)" >&2
  else
    cat "$TMP/d" >&2
  fi
  return 1
}

rc=0
case "$MODE" in
all | blocks) run_check blocks extract_blocks count_blocks "$@" || rc=1 ;;
esac
case "$MODE" in
all | lines) run_check lines extract_lines count_lines "$@" || rc=1 ;;
esac
case "$MODE" in
all | body) run_check body extract_body count_lines "$@" || rc=1 ;;
esac
exit $rc
