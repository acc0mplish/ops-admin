// Whitelist injection test (plan T50 / R-3) — must live in the internal test
// package to inject a rogue key past the typed constructor.
package inventory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestArtifactKeyValidationRejectsInjectedKeys(t *testing.T) {
	// A well-formed artifact passes its own whitelist.
	artifact := CompareArtifact{
		ArtifactSchema: CompareArtifactSchema, ClusterName: "kind-seed", Verdict: VerdictPass,
		Report: CompareReport{Blockers: []Mismatch{{Section: "pod", Key: "kube-system/a", Field: "identity"}}},
	}
	b, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := validateArtifactKeys(parsed, artifactScopeRoot); err != nil {
		t.Fatalf("legitimate artifact rejected: %v", err)
	}

	// Injecting a credential-shaped key anywhere in the tree fails.
	for _, injection := range []struct {
		scope string
		key   string
	}{{artifactScopeRoot, "kubeConfig"}, {artifactScopeRoot, "ciphertext"}, {artifactScopeReport, "rawLegacy"}, {artifactScopeMismatch, "data"}} {
		var tampered map[string]any
		if err := json.Unmarshal(b, &tampered); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		switch injection.scope {
		case artifactScopeReport:
			report := tampered["report"].(map[string]any)
			report[injection.key] = "leak"
		case artifactScopeMismatch:
			blockers := tampered["report"].(map[string]any)["blockers"].([]any)
			entry := blockers[0].(map[string]any)
			entry[injection.key] = "leak"
		default:
			tampered[injection.key] = "leak"
		}
		err := validateArtifactKeys(tampered, artifactScopeRoot)
		if err == nil {
			t.Fatalf("injected key %q at %s passed the whitelist", injection.key, injection.scope)
		}
		if !strings.Contains(err.Error(), injection.key) {
			t.Fatalf("error %v does not name the offending key %q", err, injection.key)
		}
	}
}
