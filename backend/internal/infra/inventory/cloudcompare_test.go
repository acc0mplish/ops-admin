// Cloud compare engine tests (plan phase4 §3.4 / N15 — §15 클라우드 vm 변형):
// pairing on the instance id, classification per §15.3 over the mapped vm
// fields, the 11-field mapping coverage ledger, the interim-marked artifact
// whitelist (§13-10), and the cloud 3-day gate checker.
package inventory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
)

// cloudVMFieldSet is the authoritative 11-field list of the legacy cloud
// instance serialization (mapping.md §3 — service.go cloudInstance 전수).
var cloudVMFieldSet = []string{
	"instanceID", "hostName", "privateIP", "publicIP", "cpu", "memory",
	"disk", "os", "region", "sshUser", "sshPort",
}

// cloudLegacy returns the legacy capture side of a matching cloud pair.
func cloudLegacy(at time.Time) inventory.CloudLegacyCapture {
	return inventory.CloudLegacyCapture{
		CapturedAt: at,
		Instances: []inventory.CloudLegacyVM{
			{
				InstanceID: "i-mock-1", HostName: "web-1",
				PrivateIP: "10.0.0.1", PublicIP: "203.0.113.7",
				CPU: "4 vCPU", Memory: "8 GB", Disk: "80 GB",
				OS: "Ubuntu 22.04", Region: "cn-hangzhou",
				SSHUser: "root", SSHPort: 22,
			},
			{
				InstanceID: "i-mock-2", HostName: "db-1",
				PrivateIP: "10.0.0.2", PublicIP: "",
				CPU: "2 vCPU", Memory: "16 GB", Disk: "500 GB",
				OS: "Ubuntu 22.04", Region: "cn-hangzhou",
				SSHUser: "root", SSHPort: 22,
			},
		},
	}
}

// cloudProjected builds one projected compute.vm resource the way the V2
// normalizers write Normalized (mapping.md §3 — Normalized 키 이름 그대로).
func cloudProjected(id, display, privateIP, publicIP, os, region string, cpu, memoryGB, diskGB float64) inventory.ProjectedResource {
	privateIPs := []any{}
	if privateIP != "" {
		privateIPs = append(privateIPs, privateIP)
	}
	publicIPs := []any{}
	if publicIP != "" {
		publicIPs = append(publicIPs, publicIP)
	}
	return inventory.ProjectedResource{
		Kind: "compute.vm", Subtype: "", DisplayName: display, ExternalID: id,
		Normalized: contract.JSONMap{
			"displayName": display, "region": region, "zone": "cn-hangzhou-a",
			"cpu": cpu, "memoryGB": memoryGB, "diskGB": diskGB, "os": os,
			"privateIps": privateIPs, "publicIps": publicIPs,
			"instanceType": "ecs.g6.large", "status": "Running",
			"sshHint": map[string]any{"user": "root", "port": 22},
		},
		Raw: contract.JSONMap{"instanceId": id},
	}
}

// cloudV2 returns the V2 projection side matching cloudLegacy.
func cloudV2(at time.Time) inventory.ProjectedV2 {
	return inventory.ProjectedV2{
		SyncedAt:      at,
		GenerationUID: "gen-cloud-1",
		Resources: []inventory.ProjectedResource{
			cloudProjected("i-mock-1", "web-1", "10.0.0.1", "203.0.113.7", "Ubuntu 22.04", "cn-hangzhou", 4, 8, 80),
			cloudProjected("i-mock-2", "db-1", "10.0.0.2", "", "Ubuntu 22.04", "cn-hangzhou", 2, 16, 500),
		},
	}
}

func cloudPair(at time.Time) inventory.CloudPairInput {
	return inventory.CloudPairInput{
		AccountID: 3, AccountName: "acct-1", Attempt: 1,
		Legacy: cloudLegacy(at), V2: cloudV2(at),
	}
}

// N15 — identical cloud inventories pass with zero blockers.
func TestCompareCloudPairPassesOnIdenticalInventories(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	report := inventory.CompareCloudPair(cloudPair(at))
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("verdict = %q, want pass (blockers: %+v)", report.Verdict, report.Blockers)
	}
	if len(report.Blockers) != 0 || len(report.Drifts) != 0 {
		t.Fatalf("unexpected findings: %+v / %+v", report.Blockers, report.Drifts)
	}
}

// N15 — the pairing key is the instance id: a one-sided instance is an
// identity-set BLOCKER on both directions.
func TestCompareCloudPairInstanceIDPairing(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)

	in := cloudPair(at)
	in.V2.Resources = in.V2.Resources[:1] // V2 loses i-mock-2
	report := inventory.CompareCloudPair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker (legacy-only instance)", report.Verdict)
	}
	if !hasReason(report.Blockers, "identity-sets-differ") {
		t.Fatalf("identity-sets-differ reason missing: %+v", report.Blockers)
	}

	in = cloudPair(at)
	in.V2.Resources = append(in.V2.Resources,
		cloudProjected("i-mock-3", "extra", "10.0.0.3", "", "Ubuntu 22.04", "cn-hangzhou", 1, 1, 20))
	report = inventory.CompareCloudPair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker (v2-only instance)", report.Verdict)
	}
	if !hasReason(report.Blockers, "identity-sets-differ") {
		t.Fatalf("identity-sets-differ reason missing: %+v", report.Blockers)
	}
}

// N15 — a renamed host is a mapped-field BLOCKER.
func TestCompareCloudPairFlagsFieldDifference(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	in := cloudPair(at)
	in.V2.Resources[0].Normalized["displayName"] = "web-1-renamed"
	report := inventory.CompareCloudPair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker (field mismatch)", report.Verdict)
	}
	if !hasReason(report.Blockers, "field-mismatch") {
		t.Fatalf("field-mismatch reason missing: %+v", report.Blockers)
	}
}

// N15 — the memory/disk/cpu quantities ride the QuantityEpsilon display
// round-trip (legacy prints integer GB; V2 stores the MB/1024 float).
func TestCompareCloudPairQuantityEpsilon(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	in := cloudPair(at)
	in.V2.Resources[0].Normalized["memoryGB"] = 8.005 // same physical quantity
	report := inventory.CompareCloudPair(in)
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("verdict = %q, want pass within QuantityEpsilon (blockers %+v)", report.Verdict, report.Blockers)
	}
	in.V2.Resources[0].Normalized["memoryGB"] = 8.5 // a real drift
	report = inventory.CompareCloudPair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker beyond QuantityEpsilon", report.Verdict)
	}
}

// N15 — DRIFT beyond the 60s window: attempt 1 re-pairs, attempt 2 blocks
// (§15.1 — no unbounded recapture loop).
func TestCompareCloudPairDriftRepairsOnceThenBlocks(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	in := cloudPair(at)
	in.V2.SyncedAt = at.Add(-90 * time.Second)

	first := inventory.CompareCloudPair(in)
	if first.Verdict != inventory.VerdictRePair {
		t.Fatalf("attempt 1 verdict = %q, want re-pair", first.Verdict)
	}

	in.Attempt = 2
	in.Legacy.CapturedAt = at.Add(30 * time.Second)
	second := inventory.CompareCloudPair(in)
	if second.Verdict != inventory.VerdictBlocker {
		t.Fatalf("attempt 2 verdict = %q, want blocker", second.Verdict)
	}
	if !hasReason(second.Blockers, "pairing-window-exceeded") {
		t.Fatalf("pairing-window-exceeded reason missing: %+v", second.Blockers)
	}
}

// N14 — §15.2 coverage rule: every legacy cloudInstance field must carry a
// disposition (compare or dropped-with-reason); an undisposed field is a
// mapping gap, not a silent comparison hole.
func TestCloudVMFieldCoverageIsComplete(t *testing.T) {
	if err := inventory.ValidateCloudVMFieldCoverage(cloudVMFieldSet); err != nil {
		t.Fatalf("11-field coverage incomplete: %v", err)
	}
	compared := 0
	for _, field := range cloudVMFieldSet {
		disposition := inventory.CloudVMFieldDisposition(field)
		switch {
		case disposition == "compare":
			compared++
		case disposition == "pairing-key":
			// the §15.2 identity — not a compared field
		case len(disposition) > len("dropped(") && disposition[:len("dropped(")] == "dropped(":
			// dropped fields must carry their reason — a bare "dropped" is not
			// a documented disposition (§15.2 coverage rule).
		default:
			t.Fatalf("field %q has an unusable disposition %q", field, disposition)
		}
	}
	if compared != 8 {
		t.Fatalf("compared field count = %d, want 8 (11 = 8 compared + pairing key + 2 display-only dropped)", compared)
	}
	if disposition := inventory.CloudVMFieldDisposition("instanceID"); disposition != "pairing-key" {
		t.Fatalf("instanceID disposition = %q, want pairing-key", disposition)
	}
	if err := inventory.ValidateCloudVMFieldCoverage([]string{"instanceID", "bogusField"}); err == nil {
		t.Fatalf("an undisposed field must fail the coverage check")
	}
}

// N14 (§13-10) — the interim artifact carries the marker fields, and the
// cloud gate rejects an interim-marked run (M1 promotion requires non-interim
// artifacts; plan §13-10 / r2 F8).
func TestCloudArtifactInterimMarkingAndGate(t *testing.T) {
	dir := t.TempDir()
	write := func(verdict string, at time.Time, interim bool) string {
		artifact := inventory.CloudCompareArtifact{
			ArtifactSchema: inventory.CloudCompareArtifactSchema, AccountName: "acct-1",
			Verdict: verdict, LegacyCapturedAt: at, V2SyncedAt: at,
		}
		if interim {
			artifact.Interim = true
			artifact.InterimReason = "real-account proof pending (E-1)"
			artifact.Scope = "mock-endpoint"
		}
		b, err := inventory.MarshalCloudArtifact(artifact)
		if err != nil {
			t.Fatalf("MarshalCloudArtifact: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("re-parse: %v", err)
		}
		_, hasInterim := parsed["interim"]
		if hasInterim != interim {
			t.Fatalf("interim key presence = %v, want %v", hasInterim, interim)
		}
		path := filepath.Join(dir, "compare", "cloud", "acct-1", at.Format("2006-01-02"), at.Format("150405")+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return path
	}

	day := func(d int, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.UTC) }

	// Three pass runs on three distinct days clear the formal gate.
	paths := []string{
		write(inventory.VerdictPass, day(1, 8), false),
		write(inventory.VerdictPass, day(2, 8), false),
		write(inventory.VerdictPass, day(3, 8), false),
	}
	if res := inventory.EvaluateCloudGate(paths); !res.Passed {
		t.Fatalf("non-interim distinct-day triple must clear the gate: %+v", res.Reasons)
	}

	// An interim artifact cannot clear the formal gate (§13-10 promotion).
	interim := []string{
		paths[0], paths[1],
		write(inventory.VerdictPass, day(3, 9), true),
	}
	res := inventory.EvaluateCloudGate(interim)
	if res.Passed {
		t.Fatalf("an interim-marked artifact must not clear the formal gate")
	}
	sawInterimReason := false
	for _, reason := range res.Reasons {
		if len(reason) > 6 && reason[:7] == "interim" {
			sawInterimReason = true
		}
	}
	if !sawInterimReason {
		t.Fatalf("interim rejection reason missing: %+v", res.Reasons)
	}

	// Same-day triple and mixed accounts are rejected, mirroring the K8s gate.
	sameDay := []string{
		write(inventory.VerdictPass, day(4, 8), false),
		write(inventory.VerdictPass, day(4, 9), false),
		write(inventory.VerdictPass, day(4, 10), false),
	}
	if res := inventory.EvaluateCloudGate(sameDay); res.Passed {
		t.Fatalf("same-day triple must be rejected")
	}
	mixed := []string{
		write(inventory.VerdictPass, day(1, 8), false),
		func() string {
			p := filepath.Join(dir, "compare", "cloud", "acct-2", day(2, 8).Format("2006-01-02"), day(2, 8).Format("150405")+".json")
			artifact := inventory.CloudCompareArtifact{
				ArtifactSchema: inventory.CloudCompareArtifactSchema, AccountName: "acct-2",
				Verdict: inventory.VerdictPass, LegacyCapturedAt: day(2, 8), V2SyncedAt: day(2, 8),
			}
			b, err := inventory.MarshalCloudArtifact(artifact)
			if err != nil {
				t.Fatalf("MarshalCloudArtifact: %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			return p
		}(),
		write(inventory.VerdictPass, day(3, 8), false),
	}
	if res := inventory.EvaluateCloudGate(mixed); res.Passed {
		t.Fatalf("mixed accounts must be rejected")
	}
	if res := inventory.EvaluateCloudGate(paths[:2]); res.Passed {
		t.Fatalf("fewer than three artifacts must be rejected")
	}
}

// N14 — hashes are deterministic and input-sensitive.
func TestCloudLegacyCaptureHash(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	h1 := inventory.CloudLegacyCaptureHash(cloudLegacy(at))
	if h1 != inventory.CloudLegacyCaptureHash(cloudLegacy(at)) {
		t.Fatalf("cloud legacy hash not deterministic")
	}
	changed := cloudLegacy(at)
	changed.Instances[0].HostName = "web-1-renamed"
	if h1 == inventory.CloudLegacyCaptureHash(changed) {
		t.Fatalf("cloud legacy hash ignored the payload")
	}
	if inventory.ProjectionHash(cloudV2(at)) != inventory.ProjectionHash(cloudV2(at)) {
		t.Fatalf("projection hash not deterministic")
	}
}
