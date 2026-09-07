#!/usr/bin/env bash
# Architectural boundary checks for V2 (Phase -1 Task 6, plan D-3).
# v0 rules, all green on the current tree; each rule fails loudly, not
# silently, when the boundary it guards is crossed.
#
#   R1 import : internal/domain/dnsserver must not directly import
#               internal/domain/provider (direct imports only — -deps would
#               flag the opdef→middleware→store transit and false-alarm).
#   R2 name   : core packages must not reference provider product identifiers
#               (aliyun/tencent/alicloud/tencentcloud); capability names are
#               fine. v2 (phase4 J8/D2): the guarded set covers the V2 core
#               packages too, *_test.go is excluded (core-test mock identifiers
#               are not product branches), and the §7.1 vocabulary declaration
#               lines in contract/provider_type.go are the single exception —
#               *declaring* a provider name is the spec's own design, *branching*
#               on it is not.
#   R3 purity : opdef's direct imports are limited to {middleware, gin,
#               gorm.io/gorm, stdlib} — opdef stays a pure vocabulary table
#               and may not reach into service/controller/router/store/model.
#               gorm is required: opdef.Middleware(db *gorm.DB, ...).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$ROOT/backend"

# Core packages guarded by R1/R2 (script constant; extend deliberately).
# phase4 J8 defines the full V2 core set (infra/{contract,registry,inventory,
# secrets,model,policy,metrics} + internal/tasks + internal/api/v2); the ④
# review of Phase 4 (HIGH-1) enrolls the remaining packages now that the
# backfill's provider branch moved into the contract alias table. infra/compose
# stays outside on purpose: it is the assembly root that §11.1 "registered
# explicitly" requires to import adapter packages, and "core orchestration"
# (spec §25) is not what it is.
CORE_PACKAGES=(internal/domain/dnsserver internal/infra/contract internal/infra/registry internal/infra/inventory internal/infra/secrets internal/infra/model internal/infra/policy internal/infra/metrics internal/tasks internal/api/v2)

# R2 v2 (phase4 J8): scan the full provider-name vocabulary, exclude core test
# files, and pin the single allowed exception — the §7.1 vocabulary declaration
# lines (M1ProviderTypeNames / ReservedProviderTypeNames) and the §5.4 legacy
# alias table (ProviderTypeAliases, ④review HIGH-1) in provider_type.go.
# A hit on the exception file counts only when the line names one of those
# symbols, and even then fails if the line branches on a provider name
# (declaration vs branch, J8-c). The alias table is declared as a single-line
# literal so every alias literal stays under the line-wise symbol pin.
R2_PATTERN='\b(aliyun|tencent|alicloud|tencentcloud)\b'
R2_VOCAB_EXCEPTION_FILE=internal/infra/contract/provider_type.go
R2_VOCAB_EXCEPTION_SYMBOLS='M1ProviderTypeNames|ReservedProviderTypeNames|ProviderTypeAliases'
R2_BRANCH_PATTERN='== *"(aliyun|tencent|alicloud|tencentcloud)"|case "(aliyun|tencent|alicloud|tencentcloud)"|ProviderType.*=='

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
# v2: *_test.go is excluded from the scan (phase4 J8-d — core tests legitimately
# carry provider mock identifiers; product code still requires zero). The
# canary plants the same branch in both a product file and a test file, so the
# exclusion rule itself stays falsifiable.
for pkg in "${CORE_PACKAGES[@]}"; do
  hits=0
  while IFS= read -r hit; do
    [[ -z "$hit" ]] && continue
    file="${hit%%:*}"
    if [[ "$file" == "$R2_VOCAB_EXCEPTION_FILE" ]] \
      && grep -qE "$R2_VOCAB_EXCEPTION_SYMBOLS" <<<"$hit"; then
      # The allowed vocabulary declaration line — unless it branches.
      if grep -qE "$R2_BRANCH_PATTERN" <<<"$hit"; then
        echo "R2 FAIL: $hit"
        hits=$((hits + 1))
      fi
      continue
    fi
    echo "R2 FAIL: $hit"
    hits=$((hits + 1))
  done < <(grep -rnE "$R2_PATTERN" "$pkg" --include='*.go' --exclude='*_test.go' || true)
  if [[ "$hits" -eq 0 ]]; then
    echo "R2 PASS: $pkg references no provider product identifier (product code, vocabulary exception aside)"
  else
    echo "R2 FAIL: $pkg references a provider product identifier (aliyun/tencent)"
    fail=1
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
