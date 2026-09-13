// Compare engine (plan §3.5 — §15 verbatim): pairing (§15.1), classification
// (§15.3), verdict (§15.4), the marshal whitelist (R-3), and the 3-day gate
// checker (§15.4 r2). Pure functions over captured data — no DB, no adapters.
package inventory

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CountTolerance is §15.3 "count fields differ beyond tolerance" (A11): the
// spec leaves the value open, the plan fixes 0 — a count field must match
// exactly; any drift inside the pairing window is absorbed by the re-pair
// path, not by tolerance. The 0 is inlined at the comparison site
// (valuesEqual — the only count consumer), so no named constant exists to
// drift from it.
//
// QuantityEpsilon absorbs the legacy display round-trip on capacity fields
// (legacy prints decimal MB — formatMemoryMB — while V2 stores GiB-scale GB,
// round3). Compared numerically, not by string.
const QuantityEpsilon = 0.01

// PairingWindow — §15.1 "|Δts| ≤ 60s" before count/identity comparison.
const PairingWindow = 60 * time.Second

// Verdicts (§15.4) and the artifact schema tag (R-3 whitelist).
const (
	VerdictPass    = "pass"
	VerdictBlocker = "blocker"
	VerdictRePair  = "re-pair"

	CompareArtifactSchema = "ops-admin.compare-report/v1"
)

// --- 비교 스코프 규칙 (mapping.md §3.2 — T51 동치 검증 대상). ---

// scopeDispositions maps a legacy/V2 section to its comparison disposition:
// "compare" (both sides collect it), "v2-only" (out of the comparison set —
// storageclass has no compared fields; ReplicaSet is collected by both sides
// since P1-A but legacy appearances are Deployment-derived artifacts, so its
// comparison promotion stays deferred to I-P5 — mapping.md §3.2, P1-F).
// Single-side kinds never pollute the identity sets (§3.5 r2). job·cronjob의
// 비교 집합 합류는 P1-C2 단일 착지점이다(J-P1-2·§9-10 — gateway·httproute·
// replicaset·endpoint·virtualservice는 I-P5 승인 전 불가).
var scopeDispositions = map[string]string{
	"node": "compare", "namespace": "compare", "pod": "compare",
	"deployment": "compare", "statefulset": "compare", "daemonset": "compare",
	"job": "compare", "cronjob": "compare", // P1-C2 합류 — legacy Workloads 목록이 Type과 함께 실는다(main_compare.go:256 실측)
	"service": "compare", "ingress": "compare",
	"configmap": "compare", "secret": "compare", "pv": "compare", "pvc": "compare",
	"replicaset":   "v2-only", // P1-F: P1-A 수집 착지 — 구 dropped(v2-not-collected)는 사실과 어긋난 표기
	"storageclass": "v2-only",
}

// ScopeDisposition reports the comparison scope of a section (mapping.md
// §3.2 동치 — T51).
func ScopeDisposition(section string) string { return scopeDispositions[section] }

// pairingKeyShapes — the pairing key of each section (mapping.md §3.1 동치 —
// T51). legacy serialization carries no metadata.uid (plan §0.6 r2), so the
// engine pairs on namespace/name (cluster-scoped kinds: name); V2 UID identity
// (§15.2) stays internal.
var pairingKeyShapes = map[string]string{
	"node": "name", "namespace": "name",
	"pod":        "namespace/name",
	"deployment": "{namespace}/{k8s종}/{name}", "statefulset": "{namespace}/{k8s종}/{name}", "daemonset": "{namespace}/{k8s종}/{name}",
	"job": "{namespace}/{k8s종}/{name}", "cronjob": "{namespace}/{k8s종}/{name}", // P1-C2 합류 — ExternalID {ns}/{subtype}/{name}과 동일 성분
	"service": "namespace/name", "ingress": "namespace/name",
	"configmap": "namespace/name", "secret": "namespace/name", "pvc": "namespace/name",
	"pv": "name", "storageclass": "name",
}

// PairingKeyShape reports the pairing key shape of a section (mapping.md
// §3.1 동치 — T51).
func PairingKeyShape(section string) string { return pairingKeyShapes[section] }

// --- 입력·출력 형상 ---

// LegacyCapture is the neutral projection of the v1 K8sClusterDetail the
// engine consumes — only the compared sections and fields. main_compare.go
// builds it from model.K8sClusterDetail; the engine never imports v1 types
// and no credential-shaped field exists on it (보존 제약 #7).
type LegacyCapture struct {
	CapturedAt time.Time         `json:"capturedAt"`
	Nodes      []LegacyNode      `json:"nodes"`
	Namespaces []LegacyNamespace `json:"namespaces"`
	Pods       []LegacyPod       `json:"pods"`
	Workloads  []LegacyWorkload  `json:"workloads"`
	Services   []LegacyService   `json:"services"`
	Ingresses  []LegacyIngress   `json:"ingresses"`
	ConfigMaps []LegacyConfigMap `json:"configMaps"`
	Secrets    []LegacySecret    `json:"secrets"`
	Storage    []LegacyStorage   `json:"storage"`
}

type LegacyNode struct {
	Name, Role, Status, CPU, Memory string
}

type LegacyNamespace struct {
	Name, Status string
}

type LegacyPod struct {
	Name, Namespace, Status string
	Restarts                int
}

type LegacyWorkload struct {
	Name, Type, Namespace, Ready string
}

type LegacyService struct {
	Name, Namespace, Type, ClusterIP, Ports string
}

type LegacyIngress struct {
	Name, Namespace string
}

type LegacyConfigMap struct {
	Name, Namespace string
	Keys            int
}

// LegacySecret — the legacy list item serializes name/namespace/type only;
// the V2 dataKeys count is v2-only for the pair comparison (mapping.md §4.9).
type LegacySecret struct {
	Name, Namespace, Type string
}

// LegacyStorage — Kind is "PV" or "PVC" (legacy serialization).
type LegacyStorage struct {
	Name, Kind, Namespace, Capacity, StorageClass, AccessModes string
}

// ProjectedV2 is the V2 side: the public generation (projection.go) plus its
// projected resources.
type ProjectedV2 struct {
	SyncedAt      time.Time
	GenerationUID string
	Resources     []ProjectedResource
}

// PairInput is one pairing attempt (§15.1): Attempt 1 is the first capture,
// 2 the single re-pair — a second miss beyond the window is a BLOCKER.
type PairInput struct {
	ClusterID   uint
	ClusterName string
	Attempt     int
	Legacy      LegacyCapture
	V2          ProjectedV2
}

// Mismatch is one classified difference — the only shape that enters the
// artifact (whitelist leaf, R-3).
type Mismatch struct {
	Section string `json:"section"`
	Kind    string `json:"kind"`
	Key     string `json:"key"`
	Field   string `json:"field"`
	Legacy  string `json:"legacy"`
	V2      string `json:"v2"`
	Reason  string `json:"reason"`
}

// CompareReport is the §15.3 classification result.
type CompareReport struct {
	Verdict   string        `json:"verdict"` // pass | blocker | re-pair (§15.4)
	TsDelta   time.Duration `json:"tsDelta"`
	Blockers  []Mismatch    `json:"blockers"`
	Volatiles []Mismatch    `json:"volatiles"`
	Drifts    []Mismatch    `json:"drifts"`
	Absents   []Mismatch    `json:"absents"`
}

// --- 분류 엔진 (§15.3) ---

func ComparePair(in PairInput) CompareReport {
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

	legacy := legacyEntries(in.Legacy)
	v2, v2Only := v2Entries(in.V2)

	// ABSENT ledger (§15.3 — 한쪽만 + mapping 표 dropped → 로그만). Aggregated
	// per out-of-scope kind so the report stays bounded. P1-C2로 job·cronjob이
	// 비교 집합에 합류해 남은 스코프 외 워크로드는 replicaset뿐이다(J-P1-2).
	for _, section := range []string{"replicaset"} {
		if n := legacyOutOfScopeCounts(in.Legacy)[section]; n > 0 {
			report.Absents = append(report.Absents, Mismatch{
				Section: section, Kind: "orchestration.workload", Key: fmt.Sprintf("%d entries", n),
				Field: "count", Legacy: strconv.Itoa(n), V2: "0",
				Reason: ScopeDisposition(section),
			})
		}
	}
	if n := v2Only["storageclass"]; n > 0 {
		report.Absents = append(report.Absents, Mismatch{
			Section: "storageclass", Kind: "storage.pool", Key: fmt.Sprintf("%d entries", n),
			Field: "count", Legacy: "0", V2: strconv.Itoa(n),
			Reason: ScopeDisposition("storageclass"),
		})
	}

	for _, section := range sortedSectionNames(legacy, v2) {
		if ScopeDisposition(section) != "compare" {
			continue
		}
		legacyKeys := legacy[section]
		v2Keys := v2[section]

		// Identity sets (§15.3 BLOCKER — pairing-key 집합 차이).
		for _, key := range sortedEntryKeys(legacyKeys) {
			if _, ok := v2Keys[key]; !ok {
				report.Blockers = append(report.Blockers, Mismatch{
					Section: section, Kind: legacyKeys[key].kind, Key: key,
					Field: "identity", Legacy: "present", V2: "absent",
					Reason: "identity-sets-differ",
				})
			}
		}
		for _, key := range sortedEntryKeys(v2Keys) {
			if _, ok := legacyKeys[key]; !ok {
				report.Blockers = append(report.Blockers, Mismatch{
					Section: section, Kind: v2Keys[key].kind, Key: key,
					Field: "identity", Legacy: "absent", V2: "present",
					Reason: "identity-sets-differ",
				})
			}
		}

		// Field comparison on the intersection only.
		for _, key := range sortedEntryKeys(legacyKeys) {
			le, lok := legacyKeys[key]
			ve, vok := v2Keys[key]
			if !lok || !vok {
				continue
			}
			compareFields(&report, section, key, le, ve, beyond, attempt)
		}
	}

	// 2회차 초과는 BLOCKER 리포트 — 무한 재캡처 루프 금지 (§15.1).
	if beyond && attempt >= 2 {
		report.Blockers = append(report.Blockers, Mismatch{
			Section: "pairing", Kind: "pairing", Key: "-",
			Field:  "pairing-window",
			Legacy: in.Legacy.CapturedAt.Format(time.RFC3339Nano), V2: in.V2.SyncedAt.Format(time.RFC3339Nano),
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

// compareFields classifies every compared field of one paired entry.
func compareFields(report *CompareReport, section, key string, legacyEntry, v2Entry sideEntry, beyond bool, attempt int) {
	names := make(map[string]bool, len(legacyEntry.fields)+len(v2Entry.fields))
	for name := range legacyEntry.fields {
		names[name] = true
	}
	for name := range v2Entry.fields {
		names[name] = true
	}
	// Volatile fields are compared too — their differences are sampled, never
	// blocking (§15.3 VOLATILE 무조건 허용).
	for name := range legacyEntry.volatile {
		names[name] = true
	}
	for name := range v2Entry.volatile {
		names[name] = true
	}
	for _, field := range sortedTextKeys(names) {
		legacyValue, lok := legacyEntry.volatile[field]
		if !lok {
			legacyValue, lok = legacyEntry.fields[field]
		}
		v2Value, vok := v2Entry.volatile[field]
		if !vok {
			v2Value, vok = v2Entry.fields[field]
		}
		if !lok || !vok {
			// Extractable on one side only — never blocks (defensive: both
			// extractors are symmetric over a healthy cluster).
			report.Volatiles = append(report.Volatiles, Mismatch{
				Section: section, Kind: legacyEntry.kind, Key: key, Field: field,
				Legacy: boolText(lok), V2: boolText(vok), Reason: "value-missing-one-side",
			})
			continue
		}
		if valuesEqual(legacyValue, v2Value) {
			continue
		}
		mismatch := Mismatch{
			Section: section, Kind: legacyEntry.kind, Key: key, Field: field,
			Legacy: valueText(legacyValue), V2: valueText(v2Value),
		}
		// VOLATILE — 무조건 허용, 샘플 리포트 (§15.3).
		_, legacyVolatile := legacyEntry.volatile[field]
		_, v2Volatile := v2Entry.volatile[field]
		if legacyVolatile || v2Volatile {
			mismatch.Reason = "volatile"
			report.Volatiles = append(report.Volatiles, mismatch)
			continue
		}
		if beyond {
			if attempt <= 1 {
				// DRIFT — Δ>60s count/status: 미평가, 재페어링 (§15.3).
				mismatch.Reason = "drift-window-exceeded"
				report.Drifts = append(report.Drifts, mismatch)
			} else {
				mismatch.Reason = "pairing-window-exceeded"
				report.Blockers = append(report.Blockers, mismatch)
			}
			continue
		}
		if legacyValue.isCount || v2Value.isCount {
			// CountTolerance = 0 (A11) — 정확 일치, 1 차이도 BLOCKER.
			mismatch.Reason = "count-mismatch"
		} else {
			mismatch.Reason = "field-mismatch"
		}
		report.Blockers = append(report.Blockers, mismatch)
	}
}

// --- 비교 값 형상 ---

type cmpValue struct {
	text    string
	num     float64
	isNum   bool
	isCount bool
}

func textValue(s string) cmpValue {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "-"
	}
	return cmpValue{text: s}
}

func numeric(n float64, count bool) cmpValue {
	return cmpValue{num: n, isNum: true, isCount: count}
}

// valuesEqual — counts compare exactly (CountTolerance=0), numeric capacity
// fields within QuantityEpsilon (legacy prints decimal MB, V2 stores GiB GB),
// text values after trim/empty normalization.
func valuesEqual(legacyValue, v2Value cmpValue) bool {
	if legacyValue.isNum && v2Value.isNum {
		if legacyValue.isCount || v2Value.isCount {
			return legacyValue.num == v2Value.num
		}
		return math.Abs(legacyValue.num-v2Value.num) <= QuantityEpsilon
	}
	return normalizeText(legacyValue.text) == normalizeText(v2Value.text)
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func valueText(v cmpValue) string {
	if v.isNum {
		return strconv.FormatFloat(v.num, 'f', -1, 64)
	}
	return normalizeText(v.text)
}

func boolText(b bool) string {
	if b {
		return "present"
	}
	return "absent"
}

type sideEntry struct {
	kind     string
	fields   map[string]cmpValue
	volatile map[string]cmpValue
}
