package main

import (
	"fmt"
	"os"
)

// runCompareInventory is the dispatch entry of the closed "compare-inventory"
// command (main.go). The CLI was closed in V2 Phase 6 G1 (phase6-plan §J8·
// §12 #5): the §15 paired run ended with the C53 verdict, and G1 removing the
// legacy read source (getK8sClusterDetailUncached) left the capture with
// nothing to pair against — the V2 inventory is the only read source now.
// §15.4's re-run applicability therefore ends at G1 (D-16); the stored
// artifacts and the gate waiver JSON are kept (§12 #5). The cloud variant
// (compare-inventory-cloud) is a separate CLI and is untouched. Any argument
// shape answers with this notice and a non-zero exit.
func runCompareInventory(_ []string) int {
	fmt.Fprintln(os.Stderr, "compare-inventory: closed in V2 Phase 6 G1 — the §15 k8s pairing ended with the C53 verdict (D-16); the V2 inventory is the only read source now, keep it fresh with sync-inventory. Stored artifacts remain under data/compare/.")
	return 1
}
