#!/usr/bin/env bash
# TDD harness for go-decl-blocks-diff.sh (i5-plan.md §10-4, spec §6.1 CI-B7).
#
# Cases pin the script contract used by every I5 phase (A~G):
#   1  identity          - a file compared with itself is identical
#   2  move detection    - byte-identical blocks split across files, in any
#                          order, with redistributed imports -> identical
#   3  body mutation     - one byte changed inside a func body -> detected
#   4  one-line mutation - one-line decl changed -> detected overall (blocks-only
#                          stays green: documented limitation, lines check is
#                          the supplement per plan §6.1)
#   5  dropped block     - a declaration missing from the split -> detected
#   6  duplicated block  - multiset semantics: duplicate copy -> detected
#   7  const-group mutation - internal of `const (...)` -> detected (body check)
#   8  dump-blocks       - deterministic sorted dump; orig dump == split dump
#   9  usage errors      - bad arguments exit 2
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
IMPL="$SCRIPT_DIR/go-decl-blocks-diff.sh"

TMP="$(mktemp -d)" || exit 1
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok() { pass=$((pass + 1)); echo "PASS: $1"; }
bad() { fail=$((fail + 1)); echo "FAIL: $1"; }
expect_exit() { # desc expected actual
  if [ "$3" -eq "$2" ]; then
    ok "$1 (exit=$3)"
  else
    bad "$1 (expected exit=$2, got=$3)"
  fi
}

# --- fixture: original -------------------------------------------------------
cat > "$TMP/orig.go" <<'EOF'
package fixture

import (
	"fmt"
	"net/http"
)

// Doc for Handler.
type Handler struct {
	Name string
}

type Alias = int

var one = 1

const (
	A = 1
	B = 2
)

func oneLine() {}

func big() error {
	if true {
		return fmt.Errorf("x %d", one)
	}
	return nil
}

func usesNet() string {
	_ = http.StatusOK
	return "ok"
}
EOF

# --- fixture: byte-preserving split (order shuffled, imports redistributed) --
cat > "$TMP/split1.go" <<'EOF'
package fixture

import (
	"fmt"
)

var one = 1

type Alias = int

func oneLine() {}

func big() error {
	if true {
		return fmt.Errorf("x %d", one)
	}
	return nil
}
EOF

cat > "$TMP/split2.go" <<'EOF'
package fixture

import (
	"net/http"
)

// Doc for Handler.
type Handler struct {
	Name string
}

func usesNet() string {
	_ = http.StatusOK
	return "ok"
}

const (
	A = 1
	B = 2
)
EOF

# --- fixture variants --------------------------------------------------------
sed 's/"x %d"/"y %d"/' "$TMP/split1.go" > "$TMP/mutated_body.go"
cp "$TMP/split2.go" "$TMP/mutated_oneliner_2.go"
sed 's/var one = 1/var one = 2/' "$TMP/split1.go" > "$TMP/mutated_oneliner.go"
cat "$TMP/split1.go" "$TMP/mutated_oneliner_2.go" > /dev/null
# case 5: oneLine dropped from both files
grep -v '^func oneLine' "$TMP/split1.go" > "$TMP/dropped.go"
# case 6: oneLine present in BOTH files
cat "$TMP/split1.go" > "$TMP/dup1.go"
cat "$TMP/split2.go" > "$TMP/dup2.go"
printf '\nfunc oneLine() {}\n' >> "$TMP/dup2.go"
# case 7: const group internal mutated (lines check blind, body check catches)
sed 's/^	B = 2$/	B = 3/' "$TMP/split2.go" > "$TMP/mutated_const.go"

run_impl() { # args... -> exit code of implementation
  "$IMPL" "$@"
}

# --- case 1: identity --------------------------------------------------------
run_impl "$TMP/orig.go" "$TMP/orig.go" > "$TMP/out1" 2>&1
expect_exit "identity (self compare)" 0 $?

# --- case 2: pure move detection ---------------------------------------------
run_impl "$TMP/orig.go" "$TMP/split1.go" "$TMP/split2.go" > "$TMP/out2" 2>&1
expect_exit "move detection (byte-preserving split, order-free)" 0 $?

# --- case 3: body mutation detected ------------------------------------------
run_impl "$TMP/orig.go" "$TMP/mutated_body.go" "$TMP/split2.go" > "$TMP/out3" 2>&1
expect_exit "body mutation detected (blocks check)" 1 $?

# --- case 4: one-line declaration mutation -----------------------------------
run_impl "$TMP/orig.go" "$TMP/mutated_oneliner.go" "$TMP/mutated_oneliner_2.go" > "$TMP/out4" 2>&1
expect_exit "one-line decl mutation detected (overall)" 1 $?
run_impl --blocks-only "$TMP/orig.go" "$TMP/mutated_oneliner.go" "$TMP/mutated_oneliner_2.go" > "$TMP/out4b" 2>&1
expect_exit "one-line decl invisible to blocks-only (documented limitation)" 0 $?

# --- case 5: dropped declaration ---------------------------------------------
run_impl "$TMP/orig.go" "$TMP/dropped.go" "$TMP/split2.go" > "$TMP/out5" 2>&1
expect_exit "dropped declaration detected" 1 $?

# --- case 6: duplicated declaration (multiset, not set) ----------------------
run_impl "$TMP/orig.go" "$TMP/dup1.go" "$TMP/dup2.go" > "$TMP/out6" 2>&1
expect_exit "duplicated declaration detected (multiset)" 1 $?

# --- case 7: const group internal mutation -----------------------------------
run_impl "$TMP/orig.go" "$TMP/split1.go" "$TMP/mutated_const.go" > "$TMP/out7" 2>&1
expect_exit "const-group internal mutation detected (body check)" 1 $?

# --- case 8: dump-blocks determinism -----------------------------------------
run_impl --dump-blocks "$TMP/orig.go" > "$TMP/dump_orig" 2>&1
rc=$?
if [ $rc -eq 0 ] && [ -s "$TMP/dump_orig" ]; then
  ok "dump-blocks exits 0 with non-empty output"
else
  bad "dump-blocks exits 0 with non-empty output (rc=$rc)"
fi
run_impl --dump-blocks "$TMP/split1.go" "$TMP/split2.go" > "$TMP/dump_split" 2>&1
if diff -q "$TMP/dump_orig" "$TMP/dump_split" > /dev/null; then
  ok "dump-blocks orig == split (deterministic baseline)"
else
  bad "dump-blocks orig == split (deterministic baseline)"
fi

# --- case 9: usage errors ----------------------------------------------------
run_impl > "$TMP/out9" 2>&1
expect_exit "usage error: no arguments" 2 $?
run_impl "$TMP/orig.go" > "$TMP/out9b" 2>&1
expect_exit "usage error: single argument" 2 $?
run_impl "$TMP/orig.go" "$TMP/does-not-exist.go" > "$TMP/out9c" 2>&1
expect_exit "usage error: missing file argument" 2 $?

# --- case 10: real-world identity on repository file -------------------------
REAL="$SCRIPT_DIR/../backend/controller/monitor.go"
if [ -f "$REAL" ]; then
  run_impl "$REAL" "$REAL" > "$TMP/out10" 2>&1
  expect_exit "repository monitor.go self compare" 0 $?
fi

echo "----"
echo "pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
