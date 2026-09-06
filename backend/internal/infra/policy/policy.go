// Package policy is the §10.3 policy_evaluate layer (plan §1 J5): a minimal
// built-in engine whose single M1 ruleset — builtin:default-allow — permits
// every assembled input and stamps the decision with the policy version the
// §18.1 audit row records. No policy tables exist (§22-16) and none are read:
// the ruleset is code.
//
// Deny vocabulary and environment blocking are CONTRACT-ONLY in M1 (§5.5
// language as given): a future ruleset produces Decision{Allow: false} with
// its own PolicyVersion; the evaluate/deny seam callers consume does not move.
//
// VK-4 fail-closed contract: Evaluate reports evaluation failures as ERRORS.
// The caller (plan/execute handler) must reject with 403 — never fall through
// to allow. Rationale: in a minimal allow-everything engine, "cannot evaluate
// = pass" would void the policy layer entirely — the composition
// (role_has_permission AND policy_evaluate) degenerates to the first term and
// §10.3's second gate exists only on paper. Evaluation-impossible is therefore
// a distinct outcome from an evaluated deny.
package policy

import (
	"fmt"
	"strings"
)

// PolicyInput — §10.3 policy_input vocabulary. r2: NO Tenant field — tenant is
// server-fixed context (§16.2 "constant default (server-set; no client header
// in M1)"), not a policy input. All string fields are metadata vocabulary
// (provider/context/resource kinds, risk) — no credential material ever
// (보존 제약 #7).
type PolicyInput struct {
	ProviderType string
	ContextKind  string
	ResourceKind string
	Risk         string
	Mutating     bool
}

// Decision is one policy verdict. Allow=false is the deny shape future
// rulesets produce; M1's only ruleset allows. PolicyVersion is the §18.1
// audit field's canonical source. Reason carries no secrets — it is assembled
// from the input vocabulary alone.
type Decision struct {
	Allow         bool
	PolicyVersion string
	Reason        string
}

// BuiltinDefaultAllow — the M1 ruleset version. A4: the spec fixes no version
// format; this string is pinned by the audit-field assertion (claim 8).
const BuiltinDefaultAllow = "builtin:default-allow"

// Evaluate runs the builtin ruleset over one assembled input. An input with a
// missing vocabulary field is an EVALUATION ERROR (zero Decision, no usable
// verdict may leak) — the caller fails closed (VK-4). Presence, not closed-set
// membership, is checked per field: kind/risk vocabulary conformance is the
// registry's definition-time business; evaluation only refuses to vouch for
// inputs the caller did not assemble.
func Evaluate(in PolicyInput) (Decision, error) {
	var missing []string
	if in.ProviderType == "" {
		missing = append(missing, "provider_type")
	}
	if in.ContextKind == "" {
		missing = append(missing, "context_kind")
	}
	if in.ResourceKind == "" {
		missing = append(missing, "resource_kind")
	}
	if in.Risk == "" {
		missing = append(missing, "risk")
	}
	if len(missing) > 0 {
		return Decision{}, fmt.Errorf(
			"policy: input vocabulary incomplete (%s) — evaluation impossible, the caller must fail closed (VK-4)",
			strings.Join(missing, ", "))
	}
	return Decision{
		Allow:         true,
		PolicyVersion: BuiltinDefaultAllow,
		Reason: fmt.Sprintf("%s permits %s on %s (context=%s, risk=%s, mutating=%t)",
			BuiltinDefaultAllow, in.ResourceKind, in.ProviderType, in.ContextKind, in.Risk, in.Mutating),
	}, nil
}
