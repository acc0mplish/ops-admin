// Inventory backfill shared helpers. The §5.4 k8s_cluster propagation that
// lived in this file was retired in V2 Phase 6 I-a (phase6-plan §J5
// S1a-S1c) — register-k8s (backend/main_register_k8s.go) is the only k8s
// registration path now. The cloud propagation (backfill_cloud.go) keeps
// consuming the envelope and id helpers below.
package inventory

import (
	"fmt"
	"strings"
)

// v2EnvelopePrefix — the §4.2 envelope version tag (util's unexported
// constant, mirrored here as a read-only shape check). Values carrying the
// prefix are copied verbatim; anything else is treated as P-class plaintext
// by the cloud backfill's sealing path.
const v2EnvelopePrefix = "v2:"

// envelopeKeyID parses the v2 envelope shape "v2:<key_id>:<payload>" and
// returns its key id — the only part of the envelope the backfill family
// reads.
func envelopeKeyID(value string) (string, bool) {
	if !strings.HasPrefix(value, v2EnvelopePrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(value, v2EnvelopePrefix)
	keyID, payload, ok := strings.Cut(rest, ":")
	if !ok || keyID == "" || payload == "" {
		return "", false
	}
	return keyID, true
}

func strconvID(id uint) string {
	return fmt.Sprintf("%d", id)
}
