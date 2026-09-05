#!/usr/bin/env bash
# Architectural boundary checks for V2 (Phase -1 Task 6, plan D-3).
# v0 rules, all green on the current tree; each rule fails loudly, not
# silently, when the boundary it guards is crossed.
#
#   R1 import : internal/domain/dnsserver must not directly import
#               internal/domain/provider (direct imports only — -deps would
#               flag the opdef→middleware→store transit and false-alarm).
#   R2 name   : core domain packages must not reference provider product
#               identifiers (aliyun/tencent); capability names are fine.
#   R3 purity : opdef's direct imports are limited to {middleware, gin,
#               gorm.io/gorm, stdlib} — opdef stays a pure vocabulary table
#               and may not reach into service/controller/router/store/model.
#               gorm is required: opdef.Middleware(db *gorm.DB, ...).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$ROOT/backend"

# Core packages guarded by R1/R2 (script constant; extend deliberately).
CORE_PACKAGES=(internal/domain/dnsserver)

# R3 allowlist of non-stdlib imports opdef may hold.
OPDEF_ALLOWED_NONSTD=(
  "ops-admin/backend/middleware"
  "github.com/gin-gonic/gin"
  "gorm.io/gorm"
)

cd "$BACKEND"

fail=0

# --- R1: no direct provider import in core packages -------------------------
for pkg in "${CORE_PACKAGES[@]}"; do
  imports="$(go list -f '{{join .Imports "\n"}}' "./$pkg")"
  if grep -q 'ops-admin/backend/internal/domain/provider' <<<"$imports"; then
    echo "R1 FAIL: $pkg directly imports internal/domain/provider"
    fail=1
  else
    echo "R1 PASS: $pkg has no direct provider import"
  fi
done

# --- R2: no provider product identifiers in core sources --------------------
for pkg in "${CORE_PACKAGES[@]}"; do
  if grep -rnE '\b(aliyun|tencent)\b' "$pkg" --include='*.go'; then
    echo "R2 FAIL: $pkg references a provider product identifier (aliyun/tencent)"
    fail=1
  else
    echo "R2 PASS: $pkg references no provider product identifier"
  fi
done

# --- R3: opdef direct imports limited to the pure vocabulary set ------------
# Classification is explicit, not dot-based: the module is `ops-admin/backend`
# (hyphen, no dot), so a dot heuristic lets every internal import bypass the
# check (④review HIGH-1). Internal = ops-admin/backend/*; external module =
# dot in the first path segment; everything else is stdlib.
opdef_imports="$(go list -f '{{join .Imports "\n"}}' ./opdef)" || {
  echo "R3 FAIL: go list could not enumerate opdef imports"
  exit 1
}
violations=0
while IFS= read -r imp; do
  [[ -z "$imp" ]] && continue
  case "$imp" in
    ops-admin/backend/*)
      allowed=0
      for ok in "${OPDEF_ALLOWED_NONSTD[@]}"; do
        if [[ "$imp" == "$ok" ]]; then allowed=1; break; fi
      done
      if [[ "$allowed" -eq 0 ]]; then
        echo "R3 FAIL: opdef imports non-allowlisted module: $imp"
        violations=$((violations + 1))
      fi
      ;;
    *) # external module paths have a dot in the first segment; stdlib does not
      first="${imp%%/*}"
      if [[ "$first" == *.* ]]; then
        allowed=0
        for ok in "${OPDEF_ALLOWED_NONSTD[@]}"; do
          if [[ "$imp" == "$ok" ]]; then allowed=1; break; fi
        done
        if [[ "$allowed" -eq 0 ]]; then
          echo "R3 FAIL: opdef imports non-allowlisted module: $imp"
          violations=$((violations + 1))
        fi
      fi
      ;;
  esac
done <<< "$opdef_imports"

if [[ "$violations" -eq 0 ]]; then
  echo "R3 PASS: opdef imports only {middleware, gin, gorm.io/gorm, stdlib}"
else
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "FAIL: architectural boundary violation(s) detected."
  exit 1
fi
echo "PASS: arch-boundary clean (R1+R2+R3)."
