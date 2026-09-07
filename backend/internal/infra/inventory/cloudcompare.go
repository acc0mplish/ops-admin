// Cloud compare engine (plan phase4 §3.4 / N14 — §15 클라우드 vm 변형): one
// paired run over one cloud account — the legacy capture (the v1
// fetchCloudInstances path, wrapped for compare in the service layer) against
// the V2 compute.vm projection — paired on the instance id, classified per
// §15.3 over the mapped fields with the same exact-count semantics and
// QuantityEpsilon the K8s engine uses (§15.2 mapping.md coverage rule), plus
// the §13-10 interim-marked artifact and the cloud 3-day gate checker. Pure
// functions over captured data — no DB, no adapters, no endpoint knowledge.
package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CloudCompareArtifactSchema is the cloud paired-run artifact schema — a
// sibling of the K8s compare schema, versioned separately because the cloud
// artifact carries the §13-10 interim marking.
const CloudCompareArtifactSchema = "ops-admin.compare-cloud-report/v1"

// CloudSectionVM is the report section key of the compared cloud family
// (§15 "mapped family = vm"; adapter mapping.md — the §15.2 pairing key is the
// instance id within the account scope).
const CloudSectionVM = "compute.vm"

// CloudVMFieldDispositions is the §15.2 coverage ledger over the legacy cloud
// instance 11-field serialization (adapter mapping.md §3 — 전수 처분): every
// legacy field is either compared or dropped with its documented reason. The
// instance id is the pairing key, not a compared field.
var CloudVMFieldDispositions = map[string]string{
	"instanceID": "pairing-key",
	"hostName":   "compare", "privateIP": "compare", "publicIP": "compare",
	"cpu": "compare", "memory": "compare", "disk": "compare",
	"os": "compare", "region": "compare",
	// display-only constants — the v1 host screen renders them from literals;
	// they are not observations and have no V2 normalized counterpart
	// (§14.2, mapping.md §3 rows 10-11).
	"sshUser": "dropped(display-only-constant)",
	"sshPort": "dropped(display-only-constant)",
}

// CloudVMFieldDisposition reports the §15.2 disposition of one legacy cloud
// instance field ("" — undisposed, a mapping gap).
func CloudVMFieldDisposition(field string) string { return CloudVMFieldDispositions[field] }

// ValidateCloudVMFieldCoverage enforces the §15.2 coverage rule over the given
// legacy field list: every field must carry a disposition. The compare tests
// feed it the authoritative 11-field set, so a new unmapped legacy field
// fails the suite instead of silently skipping comparison (N14 처분 완결).
func ValidateCloudVMFieldCoverage(fields []string) error {
	undisposed := make([]string, 0, len(fields))
	for _, field := range fields {
		if CloudVMFieldDispositions[field] == "" {
			undisposed = append(undisposed, field)
		}
	}
	if len(undisposed) > 0 {
		return fmt.Errorf("cloud compare: legacy vm fields without a mapping disposition: %s",
			strings.Join(undisposed, ", "))
	}
	return nil
}

// CloudLegacyCapture is the neutral projection of one v1 cloud account's
// instance list — the compared fields of the mapping table plus the two
// display-only constants it disposes as dropped. Nothing credential-shaped
// exists on it (보존 제약 #7): the caller resolves credentials, the engine
// never sees them.
type CloudLegacyCapture struct {
	CapturedAt time.Time       `json:"capturedAt"`
	Instances  []CloudLegacyVM `json:"instances"`
}

// CloudLegacyVM is one legacy cloud instance — the service layer's
// cloudInstance serialized into the engine's vocabulary (field names follow
// the mapping table's legacy column).
type CloudLegacyVM struct {
	InstanceID string
	HostName   string
	PrivateIP  string
	PublicIP   string
	CPU        string // legacy display, e.g. "4 vCPU"
	Memory     string // legacy display, e.g. "8 GB"
	Disk       string // legacy display, e.g. "80 GB"
	OS         string
	Region     string
	SSHUser    string // display-only — dropped disposition, carried for the ledger only
	SSHPort    int    // display-only — dropped disposition
}

// CloudFamily carries the family-specific legacy display-unit rule the
// compare consumes (each adapter's mapping.md §2 단위·집계 규칙). The rules
// travel as data on purpose: the engine is a core package and must not branch
// on provider product identifiers (V2 arch R2) — the caller resolves the
// provider type to this rule set and hands it in with the pair.
type CloudFamily struct {
	// LegacyMemoryDivisor is how much the family's legacy v1 memory display
	// divides the V2 memoryGB scale by. A legacy display on the divided scale
	// is normalized back by multiplication (legacy display GB × divisor).
	// A divisor ≤ 1 (the zero value included) means the legacy display already
	// is the V2 scale and must not be rescaled.
	LegacyMemoryDivisor float64
}

// MemoryGB normalizes one legacy memory display value (already parsed to its
// GB number) into the V2 memoryGB scale for this family. A divided display is
// lossy (integer division floors), so the normalization recovers the V2 scale
// only up to that display rounding — a genuine hardware drift past the floor
// still surfaces as a field mismatch; the rule exists to absorb the family's
// documented display convention, not to widen the QuantityEpsilon.
func (f CloudFamily) MemoryGB(legacyGB float64) float64 {
	if f.LegacyMemoryDivisor > 1 {
		return legacyGB * f.LegacyMemoryDivisor
	}
	return legacyGB
}

// CloudPairInput is one cloud pairing attempt (§15.1): Attempt 1 is the first
// capture, 2 the single re-pair — a second miss beyond the window is a
// BLOCKER. The V2 side reuses the public projection shape (ProjectedV2).
type CloudPairInput struct {
	AccountID   uint
	AccountName string
	Attempt     int
	Family      CloudFamily
	Legacy      CloudLegacyCapture
	V2          ProjectedV2
}

// CompareCloudPair classifies one cloud paired run (§15.3): identity-set
// comparison on the instance id, field comparison on the mapped intersection,
// the 60s pairing window and the single re-pair (§15.1), verdict per §15.4.
// The K8s engine's count exactness (§15.3 — counts compare with zero
// tolerance) carries over: the vm capacity fields are quantities (epsilon),
// everything else is exact text.
func CompareCloudPair(in CloudPairInput) CompareReport {
	delta := in.Legacy.CapturedAt.Sub(in.V2.SyncedAt)
	if delta < 0 {
		delta = -delta
	}
	beyond := delta > PairingWindow
	attempt := in.Attempt
	if attempt < 1 {
		attempt = 1
	}

	report := CompareReport{
		TsDelta:  delta,
		Blockers: []Mismatch{}, Volatiles: []Mismatch{},
		Drifts: []Mismatch{}, Absents: []Mismatch{},
	}

	legacy := cloudLegacyEntries(in.Legacy, in.Family)
	v2 := cloudV2Entries(in.V2)

	// Identity sets (§15.3 BLOCKER — the instance-id sets must match).
	for _, id := range sortedEntryKeys(legacy) {
		if _, ok := v2[id]; !ok {
			report.Blockers = append(report.Blockers, Mismatch{
				Section: CloudSectionVM, Kind: CloudSectionVM, Key: id,
				Field: "identity", Legacy: "present", V2: "absent",
				Reason: "identity-sets-differ",
			})
		}
	}
	for _, id := range sortedEntryKeys(v2) {
		if _, ok := legacy[id]; !ok {
			report.Blockers = append(report.Blockers, Mismatch{
				Section: CloudSectionVM, Kind: CloudSectionVM, Key: id,
				Field: "identity", Legacy: "absent", V2: "present",
				Reason: "identity-sets-differ",
			})
		}
	}

	// Field comparison on the pairing intersection only.
	for _, id := range sortedEntryKeys(legacy) {
		le, lok := legacy[id]
		ve, vok := v2[id]
		if !lok || !vok {
			continue
		}
		compareFields(&report, CloudSectionVM, id, le, ve, beyond, attempt)
	}

	// 2회차 초과는 BLOCKER 리포트 — 무한 재캡처 루프 금지 (§15.1).
	if beyond && attempt >= 2 {
		report.Blockers = append(report.Blockers, Mismatch{
			Section: "pairing", Kind: "pairing", Key: "-",
			Field:  "pairing-window",
			Legacy: in.Legacy.CapturedAt.Format(time.RFC3339Nano),
			V2:     in.V2.SyncedAt.Format(time.RFC3339Nano),
			Reason: "pairing-window-exceeded",
		})
	}

	switch {
	case len(report.Blockers) > 0:
		report.Verdict = VerdictBlocker
	case beyond && attempt <= 1:
		report.Verdict = VerdictRePair
	default:
		report.Verdict = VerdictPass
	}
	return report
}

// cloudLegacyEntries extracts the legacy side: one entry per instance, keyed
// by the pairing key (§15.2), fields per the mapping table's compared rows.
// The family display-unit rules normalize the legacy display strings into the
// V2 scale before the field comparison.
func cloudLegacyEntries(c CloudLegacyCapture, family CloudFamily) map[string]sideEntry {
	out := make(map[string]sideEntry, len(c.Instances))
	for _, vm := range c.Instances {
		id := strings.TrimSpace(vm.InstanceID)
		if id == "" {
			continue // an instance without an id has no pairing key — the
			// capture side filters it before the identity comparison
		}
		fields := map[string]cmpValue{
			"hostName":  textValue(vm.HostName),
			"privateIP": textValue(vm.PrivateIP),
			"publicIP":  textValue(vm.PublicIP),
			"os":        textValue(vm.OS),
			"region":    textValue(vm.Region),
		}
		if cores, ok := cloudCPUCores(vm.CPU); ok {
			fields["cpu"] = numeric(cores, false)
		}
		if gb, ok := cloudGB(vm.Memory); ok {
			// ④review round2 F2 — family display-unit rule (mapping.md §2 per
			// adapter): a divided legacy display is multiplied back to the V2
			// memoryGB scale; a display already on the V2 scale passes through.
			//
			// ④review round2 F3 judgment — the unrescaled family's legacy
			// floor truncation (MB/1024 integer division) is NOT absorbable
			// by QuantityEpsilon: the lost fraction reaches 1023/1024 GB, far
			// beyond 0.01. The float conversion therefore lives on the V2
			// side (the normalizer records memoryGB = float64(MB)/1024), and
			// a legacy display whose source MB was not an exact GB multiple
			// surfaces as a real field mismatch — reported, not masked (the
			// same false-negative philosophy F1 enforces on the CPU display).
			fields["memory"] = numeric(family.MemoryGB(gb), false)
		}
		if gb, ok := cloudGB(vm.Disk); ok {
			fields["disk"] = numeric(gb, false)
		}
		out[id] = sideEntry{kind: CloudSectionVM, fields: fields}
	}
	return out
}

// cloudV2Entries extracts the V2 side from the public projection: the
// compute.vm resources keyed by ExternalID (the instance id), fields per the
// mapping table's Normalized keys. v2-only observation fields (zone,
// instanceType, status, sshHint) are outside the compared set — the mapping
// table disposes them v2-only (mapping.md §3.1).
func cloudV2Entries(p ProjectedV2) map[string]sideEntry {
	out := make(map[string]sideEntry, len(p.Resources))
	str := func(r ProjectedResource, key string) string {
		v, _ := r.Normalized[key].(string)
		return v
	}
	firstIP := func(v any) string {
		list, ok := v.([]any)
		if !ok || len(list) == 0 {
			return ""
		}
		first, _ := list[0].(string)
		return first
	}
	for _, r := range p.Resources {
		if r.Kind != CloudSectionVM {
			continue
		}
		id := strings.TrimSpace(r.ExternalID)
		if id == "" {
			continue
		}
		fields := map[string]cmpValue{
			"hostName":  textValue(firstNonEmptyString(str(r, "displayName"), r.DisplayName)),
			"privateIP": textValue(firstIP(r.Normalized["privateIps"])),
			"publicIP":  textValue(firstIP(r.Normalized["publicIps"])),
			"os":        textValue(str(r, "os")),
			"region":    textValue(str(r, "region")),
		}
		if n, ok := jsonFloat(r.Normalized["cpu"]); ok {
			fields["cpu"] = numeric(n, false)
		}
		if n, ok := jsonFloat(r.Normalized["memoryGB"]); ok {
			fields["memory"] = numeric(n, false)
		}
		if n, ok := jsonFloat(r.Normalized["diskGB"]); ok {
			fields["disk"] = numeric(n, false)
		}
		out[id] = sideEntry{kind: CloudSectionVM, fields: fields}
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// --- 수량 표시 파싱 (mapping.md §3 — legacy 표시 문자열 ↔ V2 원값). ---

// cloudQuantity parses a legacy display value with the given unit suffix
// ("4 vCPU", "8 GB") into its numeric base.
func cloudQuantity(s, suffix string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, suffix) {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, suffix)), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// cloudQuantityFold is cloudQuantity with a case-insensitive unit suffix —
// display strings are presentation, not contract, so "4 Cores" parses like
// "4 cores".
func cloudQuantityFold(s, suffix string) (float64, bool) {
	s = strings.TrimSpace(s)
	if len(s) < len(suffix) || !strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(s[:len(s)-len(suffix)]), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// cloudCPUSuffixes is the family suffix set of the legacy CPU display: one
// family prints "N vCPU", another prints "N cores" (per-adapter mapping.md
// §3 — CPU row). The parser accepts the whole set so a family's display
// convention never degrades into a legacy-side parse failure — which would
// surface as a value-missing-one-side VOLATILE (compare.go) and mask the
// comparison (④review round2 F1).
var cloudCPUSuffixes = []string{"vCPU", "cores"}

// cloudCPUCores parses the legacy CPU display into its core count.
func cloudCPUCores(s string) (float64, bool) {
	for _, suffix := range cloudCPUSuffixes {
		if n, ok := cloudQuantityFold(s, suffix); ok {
			return n, true
		}
	}
	return 0, false
}

// CloudCPUCores is the exported form of the legacy CPU display parser — the
// family suffix set is the mapping table's contract (mapping.md §3 CPU rows),
// so it is testable from the external package too.
func CloudCPUCores(s string) (float64, bool) { return cloudCPUCores(s) }

func cloudGB(s string) (float64, bool) { return cloudQuantity(s, "GB") }

// --- 아티팩트 (§13-10 interim 규격 + R-3 whitelist) ---

// CloudCompareArtifact is the cloud paired-run artifact — the K8s whitelist
// shape plus the §13-10 interim marking: a mock-endpoint proof run is
// machine-detectable, and the formal gate below refuses interim artifacts
// (M1 promotion requires re-running the pairs without the marker). Nothing
// credential-shaped exists on the struct (보존 제약 #7).
type CloudCompareArtifact struct {
	ArtifactSchema   string        `json:"artifactSchema"`
	AccountID        uint          `json:"accountId"`
	AccountName      string        `json:"accountName"`
	Verdict          string        `json:"verdict"`
	Attempt          int           `json:"attempt"`
	TsDeltaSeconds   float64       `json:"tsDeltaSeconds"`
	LegacyCapturedAt time.Time     `json:"legacyCapturedAt"`
	LegacyHash       string        `json:"legacyHash"`
	V2GenerationUID  string        `json:"v2GenerationUid"`
	V2SyncedAt       time.Time     `json:"v2SyncedAt"`
	V2Hash           string        `json:"v2Hash"`
	Report           CompareReport `json:"report"`
	// §13-10 (r2 F8) — interim marking, present only on interim artifacts.
	Interim       bool   `json:"interim,omitempty"`
	InterimReason string `json:"interimReason,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

// MarshalCloudArtifact serializes the cloud artifact and enforces the cloud
// whitelist — a key outside it (schema drift, injected payload) fails the
// write (R-3).
func MarshalCloudArtifact(artifact CloudCompareArtifact) ([]byte, error) {
	return marshalArtifactScoped(artifact, artifactScopeCloudRoot)
}

// marshalArtifactScoped is the shared whitelist marshal: serialize, re-parse,
// and reject any key outside the given scope's whitelist.
func marshalArtifactScoped(artifact any, scope string) ([]byte, error) {
	b, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("compare: marshal artifact: %w", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("compare: re-parse artifact: %w", err)
	}
	if err := validateArtifactKeys(parsed, scope); err != nil {
		return nil, err
	}
	return b, nil
}

// --- 클라우드 게이트 (§15.4 r2 변형) ---

// EvaluateCloudGate reads the cloud paired-run artifacts, takes the newest
// three by capture time, and requires all of them to pass — on three distinct
// calendar days, for one cloud account (§15.4, E-3/A2). An interim-marked
// artifact never clears the formal gate (§13-10): the promotion procedure
// requires re-running the pairs without the marker.
func EvaluateCloudGate(paths []string) GateResult {
	type loaded struct {
		path     string
		artifact CloudCompareArtifact
	}
	var artifacts []loaded
	reasons := make([]string, 0, 4)
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		var artifact CloudCompareArtifact
		if err := json.Unmarshal(b, &artifact); err != nil {
			reasons = append(reasons, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		if artifact.ArtifactSchema != CloudCompareArtifactSchema {
			reasons = append(reasons, fmt.Sprintf("%s: schema %q is not %s", path, artifact.ArtifactSchema, CloudCompareArtifactSchema))
			continue
		}
		artifacts = append(artifacts, loaded{path: path, artifact: artifact})
	}

	result := GateResult{Reasons: reasons}
	fail := func(reason string) {
		result.Reasons = append(result.Reasons, reason)
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].artifact.LegacyCapturedAt.After(artifacts[j].artifact.LegacyCapturedAt)
	})
	if len(artifacts) < 3 {
		fail(fmt.Sprintf("gate needs 3 paired-run artifacts, found %d", len(artifacts)))
		return result
	}

	newest := artifacts[:3]
	accounts := map[string]bool{}
	dates := map[string]bool{}
	for i, entry := range newest {
		result.Artifacts = append(result.Artifacts, entry.path)
		if entry.artifact.Verdict != VerdictPass {
			fail(fmt.Sprintf("%s: verdict %q is not %q", entry.path, entry.artifact.Verdict, VerdictPass))
		}
		if entry.artifact.Interim {
			fail(fmt.Sprintf("interim artifact %s cannot clear the formal gate — re-run without the mock endpoint (§13-10)", entry.path))
		}
		account, _, ok := cloudArtifactRoute(entry.path)
		if !ok {
			fail(fmt.Sprintf("%s: path is not data/compare/cloud/<account>/<date>/<time>.json", entry.path))
			continue
		}
		accounts[account] = true
		dates[entry.artifact.LegacyCapturedAt.Format("2006-01-02")] = true
		if i == 0 {
			result.Cluster = account
		}
	}
	if len(dates) != 3 {
		fail(fmt.Sprintf("the 3 runs must fall on 3 distinct days, got %d", len(dates)))
	}
	if len(accounts) != 1 {
		fail(fmt.Sprintf("the 3 runs must share one cloud account, got %d", len(accounts)))
	}
	result.Passed = len(result.Reasons) == 0
	return result
}

// cloudArtifactRoute extracts the account and date components of the cloud
// artifact path — data/compare/cloud/<account>/<date>/<time>.json — the
// layout that makes the gate's "same account" and "distinct days"
// machine-readable (plan §2 운영 산출, r2).
func cloudArtifactRoute(path string) (account, date string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+3 < len(parts); i++ {
		if parts[i] == "compare" && parts[i+1] == "cloud" {
			return parts[i+2], parts[i+3], true
		}
	}
	return "", "", false
}

// CloudLegacyCaptureHash hashes the compared legacy cloud sections (no
// credential-shaped field exists on the capture struct — 보존 제약 #7).
func CloudLegacyCaptureHash(c CloudLegacyCapture) string { return hashJSON(c) }
