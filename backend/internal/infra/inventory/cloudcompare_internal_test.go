// Cloud whitelist injection test (plan phase4 N14 / §13-10 / R-3) — must live
// in the internal test package to inject a rogue key past the typed
// constructor of the cloud compare artifact.
package inventory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCloudArtifactKeyValidationRejectsInjectedKeys(t *testing.T) {
	// A well-formed cloud artifact (interim-marked — §13-10) passes its own
	// whitelist.
	artifact := CloudCompareArtifact{
		ArtifactSchema: CloudCompareArtifactSchema, AccountName: "acct-1", Verdict: VerdictPass,
		Interim: true, InterimReason: "real-account proof pending", Scope: "mock-endpoint",
		Report: CompareReport{Blockers: []Mismatch{{Section: "compute.vm", Key: "i-mock-1", Field: "identity"}}},
	}
	b, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := validateArtifactKeys(parsed, artifactScopeCloudRoot); err != nil {
		t.Fatalf("legitimate cloud artifact rejected: %v", err)
	}

	// Injecting a credential-shaped key anywhere in the cloud tree fails.
	for _, injection := range []struct {
		key  string
		path func(map[string]any)
	}{
		{"accessKey", func(tampered map[string]any) { tampered["accessKey"] = "leak" }},
		{"ciphertext", func(tampered map[string]any) { tampered["ciphertext"] = "leak" }},
		{"secretKey", func(tampered map[string]any) { tampered["secretKey"] = "leak" }},
		{"rawLegacy", func(tampered map[string]any) {
			tampered["report"].(map[string]any)["rawLegacy"] = "leak"
		}},
		{"data", func(tampered map[string]any) {
			blockers := tampered["report"].(map[string]any)["blockers"].([]any)
			blockers[0].(map[string]any)["data"] = "leak"
		}},
	} {
		var tampered map[string]any
		if err := json.Unmarshal(b, &tampered); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		injection.path(tampered)
		err := validateArtifactKeys(tampered, artifactScopeCloudRoot)
		if err == nil {
			t.Fatalf("injected key %q passed the cloud whitelist", injection.key)
		}
		if !strings.Contains(err.Error(), injection.key) {
			t.Fatalf("error %v does not name the offending key %q", err, injection.key)
		}
	}
}
