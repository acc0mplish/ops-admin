// Compare engine tests (plan §6 T50 + T54, §3.5): classification per §15.3,
// pairing keys per mapping.md §3.1, scope rules per §3.2, CountTolerance=0
// (A11), the marshal whitelist (R-3), and the 3-day gate checker (§15.4 r2).
package inventory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/inventory"
)

// sampleLegacy returns the legacy capture side of a matching pair.
func sampleLegacy(at time.Time) inventory.LegacyCapture {
	return inventory.LegacyCapture{
		CapturedAt: at,
		Nodes: []inventory.LegacyNode{
			{Name: "kind-control", Role: "control-plane", Status: "Ready", CPU: "8", Memory: "32660 MB"},
		},
		Namespaces: []inventory.LegacyNamespace{{Name: "default", Status: "Active"}},
		Pods: []inventory.LegacyPod{
			{Name: "coredns-abc", Namespace: "kube-system", Status: "Running", Restarts: 0},
		},
		Workloads: []inventory.LegacyWorkload{
			{Name: "coredns", Type: "Deployment", Namespace: "kube-system", Ready: "2/2"},
			{Name: "seed-job", Type: "Job", Namespace: "batch", Ready: "1/1"}, // 스코프 외 종
		},
		Services:   []inventory.LegacyService{{Name: "kubernetes", Namespace: "default", Type: "ClusterIP", ClusterIP: "10.96.0.1", Ports: "443/TCP"}},
		Ingresses:  []inventory.LegacyIngress{{Name: "demo", Namespace: "default"}},
		ConfigMaps: []inventory.LegacyConfigMap{{Name: "kube-root-ca", Namespace: "kube-system", Keys: 1}},
		Secrets:    []inventory.LegacySecret{{Name: "bootstrap-token", Namespace: "kube-system", Type: "bootstrap.kubernetes.io/token"}},
		Storage: []inventory.LegacyStorage{
			{Name: "pvc-abc", Kind: "PV", Namespace: "Cluster-scoped", Capacity: "10Gi", StorageClass: "standard", AccessModes: "RWO"},
			{Name: "data-0", Kind: "PVC", Namespace: "default", Capacity: "10Gi", StorageClass: "standard", AccessModes: "RWO"},
		},
	}
}

// v2res builds one projected resource entry.
func v2res(kind, subtype, display, externalID string, normalized, raw contract.JSONMap) inventory.ProjectedResource {
	return inventory.ProjectedResource{
		Kind: kind, Subtype: subtype, DisplayName: display, ExternalID: externalID,
		Normalized: normalized, Raw: raw,
	}
}

// sampleV2 returns the V2 projection side matching sampleLegacy.
func sampleV2(at time.Time) inventory.ProjectedV2 {
	return inventory.ProjectedV2{
		SyncedAt:      at,
		GenerationUID: "gen-1",
		Resources: []inventory.ProjectedResource{
			v2res("orchestration.node", "", "kind-control", "node-uid-1", contract.JSONMap{
				"roles": []string{"control-plane"}, "healthState": "healthy",
				"allocatableCoresGB": 8, "allocatableMemoryGB": 30.417,
			}, nil),
			v2res("orchestration.namespace", "", "default", "default", contract.JSONMap{"phase": "Active"}, nil),
			v2res("orchestration.pod", "", "coredns-abc", "pod-uid-1",
				contract.JSONMap{"phase": "Running", "restartCount": 0}, contract.JSONMap{"namespace": "kube-system"}),
			v2res("orchestration.workload", "deployment", "coredns", "kube-system/deployment/coredns",
				contract.JSONMap{"replicas": 2, "readyReplicas": 2, "image": "registry/coredns:v1"}, nil),
			v2res("network.load_balancer", "service", "kubernetes", "default/kubernetes",
				contract.JSONMap{"type": "ClusterIP", "clusterIP": "10.96.0.1",
					"ports": []any{map[string]any{"port": 443, "protocol": "TCP"}}}, nil),
			v2res("network.load_balancer", "ingress", "demo", "default/demo", contract.JSONMap{"hosts": []string{}}, nil),
			v2res("orchestration.configmap", "", "kube-root-ca", "kube-system/kube-root-ca",
				contract.JSONMap{"dataKeys": []string{"ca.crt"}}, nil),
			v2res("orchestration.secret", "", "bootstrap-token", "kube-system/bootstrap-token",
				contract.JSONMap{"type": "bootstrap.kubernetes.io/token", "dataKeys": []string{"token-id", "token-secret"}}, nil),
			v2res("storage.volume", "persistent_volume", "pvc-abc", "pvc-abc",
				contract.JSONMap{"capacityGB": 10, "storageClassName": "standard", "accessModes": []string{"RWO"}}, nil),
			v2res("storage.volume", "pvc", "data-0", "default/data-0",
				contract.JSONMap{"capacityGB": 10, "storageClassName": "standard", "accessModes": []string{"RWO"}}, nil),
			// v2 단독 종 — 스코프 외 (비교 집합 제외).
			v2res("storage.pool", "storage_class", "standard", "standard",
				contract.JSONMap{"provisioner": "rancher.io/local-path"}, nil),
		},
	}
}

func matchingPair(at time.Time) inventory.PairInput {
	return inventory.PairInput{
		ClusterID: 7, ClusterName: "kind-seed", Attempt: 1,
		Legacy: sampleLegacy(at), V2: sampleV2(at),
	}
}

// blockerKeys collects the pairing keys named in the blocker list.
func blockerKeys(ms []inventory.Mismatch) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Section+"/"+m.Key)
	}
	return out
}

func hasReason(ms []inventory.Mismatch, reason string) bool {
	for _, m := range ms {
		if m.Reason == reason {
			return true
		}
	}
	return false
}

// T50 — identical inventories pass with zero blockers.
func TestComparePairPassesOnIdenticalInventories(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	report := inventory.ComparePair(matchingPair(at))
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("verdict = %q, want pass (blockers: %+v)", report.Verdict, report.Blockers)
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("blockers on identical inventories: %v", blockerKeys(report.Blockers))
	}
	if len(report.Volatiles) != 0 {
		t.Fatalf("volatiles on identical inventories: %+v", report.Volatiles)
	}
}

// T50 — identity set difference is a BLOCKER (N12 — the gate rejects it).
func TestComparePairFlagsIdentitySetDifferenceAsBlocker(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	// V2 loses the pod — one-sided legacy entry on a comparable kind.
	in.V2.Resources = in.V2.Resources[:2]
	report := inventory.ComparePair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker", report.Verdict)
	}
	if !hasReason(report.Blockers, "identity-sets-differ") {
		t.Fatalf("identity-sets-differ reason missing from blockers: %+v", report.Blockers)
	}
}

// T50 — pairing key is namespace/name: same name in another namespace is a
// different identity (mapping.md §3.1).
func TestComparePairPairingKeyIsNamespaceName(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	// Legacy pod sits in kube-system; V2 pod now reports default.
	for i := range in.V2.Resources {
		if in.V2.Resources[i].Kind == "orchestration.pod" {
			in.V2.Resources[i].Raw = contract.JSONMap{"namespace": "default"}
		}
	}
	report := inventory.ComparePair(in)
	if !hasReason(report.Blockers, "identity-sets-differ") {
		t.Fatalf("namespace change must split the identity set: %+v", report.Blockers)
	}
}

// T50 — VOLATILE fields (restart counts) are unconditionally allowed.
func TestComparePairAllowsVolatileFields(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	for i := range in.V2.Resources {
		if in.V2.Resources[i].Kind == "orchestration.pod" {
			in.V2.Resources[i].Normalized["restartCount"] = 5
		}
	}
	report := inventory.ComparePair(in)
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("verdict = %q, want pass (volatile only)", report.Verdict)
	}
	if len(report.Volatiles) == 0 {
		t.Fatalf("volatile restart difference must be sampled, got none")
	}
}

// T50 — count fields: CountTolerance = 0 (A11), one unit of difference is a
// BLOCKER, not a warning.
func TestComparePairCountToleranceIsZero(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	in.Legacy.Workloads[0].Ready = "1/2"
	report := inventory.ComparePair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker (count off by one)", report.Verdict)
	}
	if !hasReason(report.Blockers, "count-mismatch") {
		t.Fatalf("count-mismatch reason missing: %+v", report.Blockers)
	}
}

// T50 — non-volatile mapped field difference is a BLOCKER.
func TestComparePairFlagsNonVolatileFieldDifference(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	for i := range in.V2.Resources {
		if in.V2.Resources[i].Kind == "orchestration.secret" {
			in.V2.Resources[i].Normalized["type"] = "Opaque"
		}
	}
	report := inventory.ComparePair(in)
	if report.Verdict != inventory.VerdictBlocker {
		t.Fatalf("verdict = %q, want blocker (field mismatch)", report.Verdict)
	}
	if !hasReason(report.Blockers, "field-mismatch") {
		t.Fatalf("field-mismatch reason missing: %+v", report.Blockers)
	}
}

// T50 — DRIFT: beyond the 60s window the first attempt re-pairs; the second
// miss is a BLOCKER (no unbounded recapture loop).
func TestComparePairDriftRepairsOnceThenBlocks(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	in.V2.SyncedAt = at.Add(-90 * time.Second)

	first := inventory.ComparePair(in)
	if first.Verdict != inventory.VerdictRePair {
		t.Fatalf("attempt 1 verdict = %q, want re-pair", first.Verdict)
	}

	in.Attempt = 2
	in.Legacy.CapturedAt = at.Add(30 * time.Second)
	second := inventory.ComparePair(in)
	if second.Verdict != inventory.VerdictBlocker {
		t.Fatalf("attempt 2 verdict = %q, want blocker", second.Verdict)
	}
	if !hasReason(second.Blockers, "pairing-window-exceeded") {
		t.Fatalf("pairing-window-exceeded reason missing: %+v", second.Blockers)
	}
}

// T50 — the 60s boundary: exactly 60s evaluates normally, one nanosecond
// beyond triggers the drift path.
func TestComparePairSixtySecondBoundary(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	in.V2.SyncedAt = at.Add(-60 * time.Second)
	if report := inventory.ComparePair(in); report.Verdict != inventory.VerdictPass {
		t.Fatalf("exactly 60s must evaluate: verdict = %q", report.Verdict)
	}
	in.V2.SyncedAt = at.Add(-60*time.Second - time.Nanosecond)
	if report := inventory.ComparePair(in); report.Verdict != inventory.VerdictRePair {
		t.Fatalf("61s must drift to re-pair: verdict = %q", report.Verdict)
	}
}

// T50 — 스코프 외 종 (legacy 단독 Job, V2 단독 storageclass) never pollute the
// identity sets — they are logged as ABSENT only.
func TestComparePairExcludesOutOfScopeKinds(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	report := inventory.ComparePair(matchingPair(at))
	if report.Verdict != inventory.VerdictPass {
		t.Fatalf("out-of-scope kinds must not block: verdict = %q, blockers %v",
			report.Verdict, blockerKeys(report.Blockers))
	}
	if len(report.Absents) == 0 {
		t.Fatalf("out-of-scope kinds must be logged as ABSENT, got none")
	}
	sawJob, sawStorageClass := false, false
	for _, a := range report.Absents {
		if a.Section == "job" {
			sawJob = true
		}
		if a.Section == "storageclass" {
			sawStorageClass = true
		}
	}
	if !sawJob || !sawStorageClass {
		t.Fatalf("absent ledger incomplete: %+v", report.Absents)
	}
}

// T50 (R-3) — the artifact marshal is a fixed whitelist: a key injected
// outside it fails the marshal (internal test injects the key).
func TestMarshalArtifactWhitelist(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	artifact := inventory.CompareArtifact{
		ArtifactSchema:   inventory.CompareArtifactSchema,
		ClusterID:        7,
		ClusterName:      "kind-seed",
		Verdict:          inventory.VerdictPass,
		Attempt:          1,
		LegacyCapturedAt: at,
		V2SyncedAt:       at,
	}
	b, err := inventory.MarshalArtifact(artifact)
	if err != nil {
		t.Fatalf("MarshalArtifact: %v", err)
	}
	// No credential-shaped material may exist in the serialized form.
	for _, needle := range []string{"kubeconfig", "kubeConfig", "ciphertext", "Ciphertext"} {
		if strings.Contains(string(b), needle) {
			t.Fatalf("artifact leaks credential-shaped key %q", needle)
		}
	}
}

// T54 — the gate checker: newest three artifacts must all pass, on three
// distinct days, for one cluster (§15.4 r2).
func TestEvaluateGateRequiresThreeDistinctDays(t *testing.T) {
	dir := t.TempDir()
	write := func(cluster string, verdict string, at time.Time) string {
		artifact := inventory.CompareArtifact{
			ArtifactSchema: inventory.CompareArtifactSchema, ClusterName: cluster,
			Verdict: verdict, LegacyCapturedAt: at, V2SyncedAt: at,
		}
		b, err := inventory.MarshalArtifact(artifact)
		if err != nil {
			t.Fatalf("MarshalArtifact: %v", err)
		}
		path := filepath.Join(dir, "compare", cluster, at.Format("2006-01-02"), at.Format("150405")+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return path
	}

	day := func(d int) time.Time { return time.Date(2026, 9, d, 8, 0, 0, 0, time.UTC) }

	// 같은 날 3회 → 거부 (E-3/A2).
	sameDay := []string{
		write("kind-seed", inventory.VerdictPass, day(1)),
		write("kind-seed", inventory.VerdictPass, day(1).Add(time.Hour)),
		write("kind-seed", inventory.VerdictPass, day(1).Add(2*time.Hour)),
	}
	if res := inventory.EvaluateGate(sameDay); res.Passed {
		t.Fatalf("same-day triple must be rejected: %+v", res.Reasons)
	}

	// 서로 다른 날 3회 pass → 통과.
	distinct := []string{
		write("kind-seed", inventory.VerdictPass, day(1)),
		write("kind-seed", inventory.VerdictPass, day(2)),
		write("kind-seed", inventory.VerdictPass, day(3)),
	}
	if res := inventory.EvaluateGate(distinct); !res.Passed {
		t.Fatalf("distinct-day triple pass must clear the gate: %+v", res.Reasons)
	}

	// streak 차단 (§3.5 r2): [pass, fail, pass, pass] — 최신 3개 창에 fail → 거부.
	withFail := []string{
		write("kind-seed", inventory.VerdictPass, day(1)),
		write("kind-seed", inventory.VerdictBlocker, day(2)),
		write("kind-seed", inventory.VerdictPass, day(3)),
		write("kind-seed", inventory.VerdictPass, day(4)),
	}
	if res := inventory.EvaluateGate(withFail); res.Passed {
		t.Fatalf("fail inside the newest-3 window must be rejected")
	}

	// streak 리셋 (§3.5 r2): [fail, pass, pass, pass] — fail이 창 밖 → 통과.
	failOutside := []string{
		write("kind-seed", inventory.VerdictBlocker, day(1)),
		write("kind-seed", inventory.VerdictPass, day(2)),
		write("kind-seed", inventory.VerdictPass, day(3)),
		write("kind-seed", inventory.VerdictPass, day(4)),
	}
	if res := inventory.EvaluateGate(failOutside); !res.Passed {
		t.Fatalf("fail outside the newest-3 window must not block: %+v", res.Reasons)
	}

	// 동일 클러스터 단얫: 3 pass·3일이지만 클러스터가 섞이면 거부.
	mixed := []string{
		write("kind-seed", inventory.VerdictPass, day(1)),
		write("other-cluster", inventory.VerdictPass, day(2)),
		write("kind-seed", inventory.VerdictPass, day(3)),
	}
	if res := inventory.EvaluateGate(mixed); res.Passed {
		t.Fatalf("mixed clusters must be rejected")
	}

	// 3개 미만 → 거부.
	if res := inventory.EvaluateGate(distinct[:2]); res.Passed {
		t.Fatalf("fewer than three artifacts must be rejected")
	}
}

// Hash functions are deterministic and input-sensitive.
func TestCaptureAndProjectionHashes(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	h1 := inventory.LegacyCaptureHash(sampleLegacy(at))
	if h1 != inventory.LegacyCaptureHash(sampleLegacy(at)) {
		t.Fatalf("legacy hash not deterministic")
	}
	if h1 == inventory.LegacyCaptureHash(sampleLegacy(at.Add(time.Second))) {
		t.Fatalf("legacy hash ignored the capture timestamp")
	}
	v2 := sampleV2(at)
	if inventory.ProjectionHash(v2) != inventory.ProjectionHash(sampleV2(at)) {
		t.Fatalf("projection hash not deterministic")
	}
	v2.Resources = v2.Resources[:1]
	if inventory.ProjectionHash(v2) == inventory.ProjectionHash(sampleV2(at)) {
		t.Fatalf("projection hash ignored the resource set")
	}
}

// CompareReport survives a JSON round trip with its classification intact.
func TestCompareReportJSONRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	in := matchingPair(at)
	in.V2.Resources = in.V2.Resources[:2]
	report := inventory.ComparePair(in)
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var back inventory.CompareReport
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if back.Verdict != inventory.VerdictBlocker || len(back.Blockers) != len(report.Blockers) {
		t.Fatalf("round trip lost classification: %+v", back)
	}
}
