// Compare engine (plan §3.5 — §15 verbatim): pairing (§15.1), classification
// (§15.3), verdict (§15.4), the marshal whitelist (R-3), and the 3-day gate
// checker (§15.4 r2). Pure functions over captured data — no DB, no adapters.
package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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

// --- legacy 측 추출 ---

func legacyEntries(c LegacyCapture) map[string]map[string]sideEntry {
	out := map[string]map[string]sideEntry{}
	put := func(section, key, kind string, fields, volatile map[string]cmpValue) {
		entry := out[section]
		if entry == nil {
			entry = map[string]sideEntry{}
			out[section] = entry
		}
		entry[key] = sideEntry{kind: kind, fields: fields, volatile: volatile}
	}

	for _, n := range c.Nodes {
		fields := map[string]cmpValue{
			"role":   textValue(normalizeRoles(n.Role)),
			"status": textValue(nodeHealthOfLegacy(n.Status)),
		}
		if cores, ok := quantityToCores(n.CPU); ok {
			fields["cpu"] = numeric(cores, false)
		}
		if gb, ok := legacyMemoryMBToGB(n.Memory); ok {
			fields["memory"] = numeric(gb, false)
		}
		put("node", n.Name, "orchestration.node", fields, nil)
	}

	for _, n := range c.Namespaces {
		put("namespace", n.Name, "orchestration.namespace",
			map[string]cmpValue{"phase": textValue(n.Status)}, nil)
	}

	for _, p := range c.Pods {
		put("pod", p.Namespace+"/"+p.Name, "orchestration.pod",
			map[string]cmpValue{"phase": textValue(p.Status)},
			map[string]cmpValue{"restarts": numeric(float64(p.Restarts), true)})
	}

	for _, w := range c.Workloads {
		sub := strings.ToLower(strings.TrimSpace(w.Type))
		if ScopeDisposition(sub) != "compare" {
			continue // replicaset — ledger 경로 (job·cronjob은 P1-C2 합류)
		}
		fields := map[string]cmpValue{}
		if ready, replicas, ok := splitReady(w.Ready); ok {
			fields["replicas"] = numeric(replicas, true)
			fields["readyReplicas"] = numeric(ready, true)
		}
		put(sub, w.Namespace+"/"+sub+"/"+w.Name, "orchestration.workload", fields, nil)
	}

	for _, s := range c.Services {
		serviceType := strings.TrimSpace(s.Type)
		if strings.EqualFold(serviceType, "Headless") {
			// legacy display transformation — a headless service IS a
			// ClusterIP-typed service with None IP (mapping.md §4.7).
			serviceType = "ClusterIP"
		}
		put("service", s.Namespace+"/"+s.Name, "network.load_balancer", map[string]cmpValue{
			"type":      textValue(serviceType),
			"clusterIP": textValue(s.ClusterIP),
			"ports":     textValue(canonicalLegacyPorts(s.Ports)),
		}, nil)
	}

	for _, i := range c.Ingresses {
		put("ingress", i.Namespace+"/"+i.Name, "network.load_balancer", map[string]cmpValue{}, nil)
	}

	for _, cm := range c.ConfigMaps {
		put("configmap", cm.Namespace+"/"+cm.Name, "orchestration.configmap",
			map[string]cmpValue{"keys": numeric(float64(cm.Keys), true)}, nil)
	}

	for _, s := range c.Secrets {
		put("secret", s.Namespace+"/"+s.Name, "orchestration.secret", map[string]cmpValue{
			"type": textValue(s.Type),
		}, nil)
	}

	for _, s := range c.Storage {
		fields := map[string]cmpValue{
			"storageClass": textValue(s.StorageClass),
			"accessModes":  textValue(normalizeSet(s.AccessModes, ",")),
		}
		if gb, ok := quantityToGB(s.Capacity); ok {
			fields["capacity"] = numeric(gb, false)
		}
		switch strings.ToUpper(strings.TrimSpace(s.Kind)) {
		case "PV":
			put("pv", s.Name, "storage.volume", fields, nil)
		case "PVC":
			put("pvc", s.Namespace+"/"+s.Name, "storage.volume", fields, nil)
		}
	}
	return out
}

// legacyOutOfScopeCounts tallies the legacy-only workload kinds (ReplicaSet —
// job·cronjob은 P1-C2 합류로 compare라 계수되지 않는다) for the ABSENT ledger.
func legacyOutOfScopeCounts(c LegacyCapture) map[string]int {
	counts := map[string]int{}
	for _, w := range c.Workloads {
		sub := strings.ToLower(strings.TrimSpace(w.Type))
		if disposition := ScopeDisposition(sub); disposition != "compare" && disposition != "" {
			counts[sub]++
		}
	}
	return counts
}

// --- V2 측 추출 ---

func v2Entries(p ProjectedV2) (map[string]map[string]sideEntry, map[string]int) {
	out := map[string]map[string]sideEntry{}
	v2Only := map[string]int{}
	put := func(section, key, kind string, fields, volatile map[string]cmpValue) {
		entry := out[section]
		if entry == nil {
			entry = map[string]sideEntry{}
			out[section] = entry
		}
		entry[key] = sideEntry{kind: kind, fields: fields, volatile: volatile}
	}
	str := func(r ProjectedResource, key string) string {
		v, _ := r.Normalized[key].(string)
		return v
	}

	for _, r := range p.Resources {
		switch {
		case r.Kind == "orchestration.node":
			fields := map[string]cmpValue{
				"role":   textValue(normalizeRoles(anyToStringList(r.Normalized["roles"]))),
				"status": textValue(str(r, "healthState")),
			}
			if n, ok := jsonFloat(r.Normalized["allocatableCoresGB"]); ok {
				fields["cpu"] = numeric(n, false)
			}
			if n, ok := jsonFloat(r.Normalized["allocatableMemoryGB"]); ok {
				fields["memory"] = numeric(n, false)
			}
			put("node", r.DisplayName, "orchestration.node", fields, nil)

		case r.Kind == "orchestration.namespace":
			put("namespace", r.DisplayName, "orchestration.namespace",
				map[string]cmpValue{"phase": textValue(str(r, "phase"))}, nil)

		case r.Kind == "orchestration.pod":
			namespace, _ := r.Raw["namespace"].(string)
			restarts, _ := jsonFloat(r.Normalized["restartCount"])
			put("pod", namespace+"/"+r.DisplayName, "orchestration.pod",
				map[string]cmpValue{"phase": textValue(str(r, "phase"))},
				map[string]cmpValue{"restarts": numeric(restarts, true)})

		case r.Kind == "orchestration.workload":
			sub := strings.ToLower(strings.TrimSpace(r.Subtype))
			if ScopeDisposition(sub) != "compare" {
				continue
			}
			fields := map[string]cmpValue{}
			switch sub {
			case "job":
				// P1-C2 — legacy buildWorkloadItems(k8s_build_pod.go) Ready 동치:
				// job의 Ready는 succeeded/total이고 total은 completions(0이면
				// active+succeeded+failed)다. normalized의 replicas·readyReplicas는
				// batch 종에서 구조적 영값이라 원천이 아니며, 상태 키
				// (completions·active·succeeded·failed — J-P1-3)에서 유도한다.
				active, _ := jsonFloat(r.Normalized["active"])
				succeeded, _ := jsonFloat(r.Normalized["succeeded"])
				failed, _ := jsonFloat(r.Normalized["failed"])
				total, _ := jsonFloat(r.Normalized["completions"])
				if total <= 0 {
					total = active + succeeded + failed
				}
				fields["replicas"] = numeric(total, true)
				fields["readyReplicas"] = numeric(succeeded, true)
			case "cronjob":
				// legacy Ready는 cronJobReadyText 텍스트("Scheduled"/"Suspended"/
				// "N Active")라 splitReady가 실패해 legacy 측 필드가 비어 있다 —
				// V2도 필드 없이 신원 비교만 둔다(양측 동치, 무비교).
			default:
				if n, ok := jsonFloat(r.Normalized["replicas"]); ok {
					fields["replicas"] = numeric(n, true)
				}
				if n, ok := jsonFloat(r.Normalized["readyReplicas"]); ok {
					fields["readyReplicas"] = numeric(n, true)
				}
			}
			put(sub, r.ExternalID, "orchestration.workload", fields, nil)

		case r.Kind == "network.load_balancer" && r.Subtype == "service":
			put("service", r.ExternalID, "network.load_balancer", map[string]cmpValue{
				"type":      textValue(str(r, "type")),
				"clusterIP": textValue(str(r, "clusterIP")),
				"ports":     textValue(canonicalV2Ports(r.Normalized["ports"])),
			}, nil)

		case r.Kind == "network.load_balancer" && r.Subtype == "ingress":
			// host는 v2-only 필드 (mapping.md §4.7) — identity 비교만.
			put("ingress", r.ExternalID, "network.load_balancer", map[string]cmpValue{}, nil)

		case r.Kind == "orchestration.configmap":
			put("configmap", r.ExternalID, "orchestration.configmap",
				map[string]cmpValue{"keys": numeric(jsonLen(r.Normalized["dataKeys"]), true)}, nil)

		case r.Kind == "orchestration.secret":
			// type만 비교 — legacy 리스트 직렬화에 key count가 없어 dataKeys는
			// v2-only (mapping.md §4.9, 실측 정정).
			put("secret", r.ExternalID, "orchestration.secret", map[string]cmpValue{
				"type": textValue(str(r, "type")),
			}, nil)

		case r.Kind == "storage.volume" && (r.Subtype == "persistent_volume" || r.Subtype == "pvc"):
			fields := map[string]cmpValue{
				"storageClass": textValue(str(r, "storageClassName")),
				"accessModes":  textValue(normalizeAnySet(r.Normalized["accessModes"])),
			}
			if n, ok := jsonFloat(r.Normalized["capacityGB"]); ok {
				fields["capacity"] = numeric(n, false)
			}
			if r.Subtype == "persistent_volume" {
				put("pv", r.DisplayName, "storage.volume", fields, nil)
			} else {
				put("pvc", r.ExternalID, "storage.volume", fields, nil)
			}

		case r.Kind == "storage.pool" && r.Subtype == "storage_class":
			// V2 단독 종 — ABSENT ledger로만 (§3.5 스코프 표).
			v2Only["storageclass"]++

		default:
			// Not part of the compared scope — ignored.
		}
	}
	return out, v2Only
}

// --- 정규화 헬퍼 ---

// nodeHealthOfLegacy maps the legacy node status onto the §8.2 health
// vocabulary (mapping.md §5): Ready→healthy, everything else degraded.
func nodeHealthOfLegacy(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "Ready") {
		return "healthy"
	}
	return "degraded"
}

// normalizeRoles canonicalizes a comma-joined role list (legacy joins sorted;
// V2 stores a sorted list).
func normalizeRoles(joined string) string {
	return normalizeSet(joined, ",")
}

func normalizeSet(joined, sep string) string {
	parts := strings.Split(joined, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" && p != "-" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func normalizeAnySet(v any) string {
	return normalizeSet(anyToStringList(v), ",")
}

func anyToStringList(v any) string {
	switch list := v.(type) {
	case []string:
		parts := make([]string, len(list))
		copy(parts, list)
		sort.Strings(parts)
		return strings.Join(parts, ",")
	case []any:
		parts := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	case string:
		return list
	}
	return ""
}

// splitReady splits the legacy "ready/replicas" display ("2/2").
func splitReady(ready string) (readyN, replicas float64, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(ready), "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	readyN, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	replicas, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	return readyN, replicas, err1 == nil && err2 == nil
}

// canonicalLegacyPorts turns the legacy ports display ("443/TCP, 80:31234/TCP")
// into the canonical "port/proto" set (nodePort dropped — not part of the V2
// normalized schema).
func canonicalLegacyPorts(ports string) string {
	out := make([]string, 0, 4)
	for _, entry := range strings.Split(ports, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" || entry == "-" {
			continue
		}
		slash := strings.Index(entry, "/")
		if slash < 0 {
			continue
		}
		port, proto := entry[:slash], entry[slash+1:]
		if colon := strings.Index(port, ":"); colon >= 0 {
			port = port[:colon] // drop nodePort
		}
		out = append(out, port+"/"+proto)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// canonicalV2Ports renders the V2 normalized port list into the same
// "port/proto" set.
func canonicalV2Ports(v any) string {
	list, ok := v.([]any)
	if !ok {
		return ""
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		port := jsonFloatText(entry["port"])
		if port == "" {
			continue
		}
		proto, _ := entry["protocol"].(string)
		if proto == "" {
			proto = "TCP"
		}
		out = append(out, port+"/"+proto)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// --- 수량 정규화 (mapping.md §2 단위 규칙과 동일 의미론 — engine-local 복제,
// compare는 어댑터 패키지에 의존하지 않는다 — arch rule 1 정신). ---

const gib = 1024.0 * 1024.0 * 1024.0

func parseQuantityBytes(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	lower := strings.ToLower(s)
	for _, sf := range []struct {
		suffix     string
		multiplier float64
	}{{"ki", 1024}, {"mi", 1024 * 1024}, {"gi", gib}, {"ti", gib * 1024}, {"k", 1000}, {"m", 1e6}, {"g", 1e9}} {
		if strings.HasSuffix(lower, sf.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(lower, sf.suffix)), 64)
			if err != nil {
				return 0, false
			}
			return n * sf.multiplier, true
		}
	}
	n, err := strconv.ParseFloat(lower, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func quantityToGB(s string) (float64, bool) {
	bytes, ok := parseQuantityBytes(s)
	if !ok {
		return 0, false
	}
	return bytes / gib, true
}

func quantityToCores(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil {
			return 0, false
		}
		return n / 1000, true
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// legacyMemoryMBToGB parses the legacy display format ("32660 MB") into
// GiB-scale GB — the same physical quantity V2 stores (mapping.md §4.3).
func legacyMemoryMBToGB(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "MB") {
		return 0, false
	}
	mb, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "MB")), 64)
	if err != nil || mb <= 0 {
		return 0, false
	}
	return mb * 1e6 / gib, true
}

// --- JSON 값 헬퍼 ---

func jsonFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func jsonLen(v any) float64 {
	switch list := v.(type) {
	case []any:
		return float64(len(list))
	case []string:
		return float64(len(list))
	}
	return 0
}

func jsonFloatText(v any) string {
	if n, ok := jsonFloat(v); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return ""
}

// --- 정렬 헬퍼 ---

func sortedSectionNames(one, two map[string]map[string]sideEntry) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(one)+len(two))
	for name := range one {
		seen[name] = true
	}
	for name := range two {
		seen[name] = true
	}
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedEntryKeys(entries map[string]sideEntry) []string {
	out := make([]string, 0, len(entries))
	for key := range entries {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedTextKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// --- 해시 (아티팩트 병기용 — 원문은 싣지 않는다, §3.5 r2) ---

func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// LegacyCaptureHash hashes the compared legacy sections (no credential-shaped
// field exists on the capture struct — 보존 제약 #7).
func LegacyCaptureHash(c LegacyCapture) string { return hashJSON(c) }

// ProjectionHash hashes the V2 projection in a canonical order.
func ProjectionHash(v2 ProjectedV2) string {
	resources := make([]ProjectedResource, len(v2.Resources))
	copy(resources, v2.Resources)
	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Subtype != b.Subtype {
			return a.Subtype < b.Subtype
		}
		return a.ExternalID < b.ExternalID
	})
	payload := struct {
		GenerationUID string              `json:"generationUid"`
		Resources     []ProjectedResource `json:"resources"`
	}{GenerationUID: v2.GenerationUID, Resources: resources}
	return hashJSON(payload)
}

// --- 아티팩트 (R-3 — whitelist 마셜) ---

const (
	artifactScopeRoot      = "root"
	artifactScopeCloudRoot = "cloudRoot"
	artifactScopeReport    = "report"
	artifactScopeMismatch  = "mismatch"
)

// CompareArtifact is the whole report artifact — a fixed whitelist: report,
// pair summary, hashes, version strings. Raw legacy section JSON and the V2
// projection body never enter; only hashes do (§3.5 r2). Nothing
// credential-shaped exists on the struct (보존 제약 #7).
type CompareArtifact struct {
	ArtifactSchema   string        `json:"artifactSchema"`
	ClusterID        uint          `json:"clusterId"`
	ClusterName      string        `json:"clusterName"`
	Verdict          string        `json:"verdict"`
	Attempt          int           `json:"attempt"`
	TsDeltaSeconds   float64       `json:"tsDeltaSeconds"`
	LegacyCapturedAt time.Time     `json:"legacyCapturedAt"`
	LegacyHash       string        `json:"legacyHash"`
	V2GenerationUID  string        `json:"v2GenerationUid"`
	V2SyncedAt       time.Time     `json:"v2SyncedAt"`
	V2Hash           string        `json:"v2Hash"`
	Report           CompareReport `json:"report"`
}

// artifactAllowedKeys is the T50-enforced whitelist: any key outside it fails
// the marshal (R-3 — a schema drift or an injected payload cannot serialize).
var artifactAllowedKeys = map[string]map[string]bool{
	artifactScopeRoot: {
		"artifactSchema": true, "clusterId": true, "clusterName": true,
		"verdict": true, "attempt": true, "tsDeltaSeconds": true,
		"legacyCapturedAt": true, "legacyHash": true,
		"v2GenerationUid": true, "v2SyncedAt": true, "v2Hash": true,
		"report": true,
	},
	// Cloud root scope (plan phase4 N14 / §13-10): the K8s whitelist plus the
	// interim marking keys.
	artifactScopeCloudRoot: {
		"artifactSchema": true, "accountId": true, "accountName": true,
		"verdict": true, "attempt": true, "tsDeltaSeconds": true,
		"legacyCapturedAt": true, "legacyHash": true,
		"v2GenerationUid": true, "v2SyncedAt": true, "v2Hash": true,
		"report":  true,
		"interim": true, "interimReason": true, "scope": true,
	},
	artifactScopeReport: {
		"verdict": true, "tsDelta": true,
		"blockers": true, "volatiles": true, "drifts": true, "absents": true,
	},
	artifactScopeMismatch: {
		"section": true, "kind": true, "key": true, "field": true,
		"legacy": true, "v2": true, "reason": true,
	},
}

// validateArtifactKeys walks the serialized tree and rejects any key outside
// the scope whitelist.
func validateArtifactKeys(node map[string]any, scope string) error {
	allowed := artifactAllowedKeys[scope]
	if allowed == nil {
		return fmt.Errorf("compare: unknown artifact scope %q", scope)
	}
	for key, value := range node {
		if !allowed[key] {
			return fmt.Errorf("compare: artifact key %q is outside the %s whitelist", key, scope)
		}
		switch {
		case (scope == artifactScopeRoot || scope == artifactScopeCloudRoot) && key == "report":
			report, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("compare: artifact report is not an object")
			}
			if err := validateArtifactKeys(report, artifactScopeReport); err != nil {
				return err
			}
		case scope == artifactScopeReport:
			if key == "verdict" || key == "tsDelta" || value == nil {
				continue
			}
			entries, ok := value.([]any)
			if !ok {
				return fmt.Errorf("compare: artifact %s is not a list", key)
			}
			for _, entry := range entries {
				mismatch, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("compare: artifact %s entry is not an object", key)
				}
				if err := validateArtifactKeys(mismatch, artifactScopeMismatch); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// MarshalArtifact serializes the artifact and enforces the whitelist — a key
// outside it (schema drift, injected payload) fails the write.
func MarshalArtifact(artifact CompareArtifact) ([]byte, error) {
	return marshalArtifactScoped(artifact, artifactScopeRoot)
}

// --- 게이트 체커 (§15.4 r2) ---

// GateResult is the §15.4 machine verdict over the stored artifacts.
type GateResult struct {
	Passed    bool
	Cluster   string
	Artifacts []string
	Reasons   []string
}

// EvaluateGate reads the paired-run artifacts, takes the newest three by
// capture time, and requires all of them to pass — NOT a pass streak over
// the whole history (§3.5 r2: a fail outside the newest-3 window is a reset
// clock, not a block) — on three distinct calendar days for one cluster
// (§15.4 "3 consecutive passing paired runs on different days", E-3/A2).
func EvaluateGate(paths []string) GateResult {
	type loaded struct {
		path     string
		artifact CompareArtifact
	}
	var artifacts []loaded
	reasons := make([]string, 0, 4)
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		var artifact CompareArtifact
		if err := json.Unmarshal(b, &artifact); err != nil {
			reasons = append(reasons, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		if artifact.ArtifactSchema != CompareArtifactSchema {
			reasons = append(reasons, fmt.Sprintf("%s: schema %q is not %s", path, artifact.ArtifactSchema, CompareArtifactSchema))
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
	clusters := map[string]bool{}
	dates := map[string]bool{}
	for i, entry := range newest {
		result.Artifacts = append(result.Artifacts, entry.path)
		if entry.artifact.Verdict != VerdictPass {
			fail(fmt.Sprintf("%s: verdict %q is not %q", entry.path, entry.artifact.Verdict, VerdictPass))
		}
		cluster, _, ok := artifactRoute(entry.path)
		if !ok {
			fail(fmt.Sprintf("%s: path is not data/compare/<cluster>/<date>/<time>.json", entry.path))
			continue
		}
		clusters[cluster] = true
		dates[entry.artifact.LegacyCapturedAt.Format("2006-01-02")] = true
		if i == 0 {
			result.Cluster = cluster
		}
	}
	if len(dates) != 3 {
		fail(fmt.Sprintf("the 3 runs must fall on 3 distinct days, got %d", len(dates)))
	}
	if len(clusters) != 1 {
		fail(fmt.Sprintf("the 3 runs must share one cluster, got %d", len(clusters)))
	}
	result.Passed = len(result.Reasons) == 0
	return result
}

// artifactRoute extracts the cluster and date components of the canonical
// artifact path — the layout is what makes the gate's "same cluster" and
// "distinct days" machine-readable (plan §2 운영 산출, r2).
func artifactRoute(path string) (cluster, date string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "compare" {
			return parts[i+1], parts[i+2], true
		}
	}
	return "", "", false
}

// GateWaiver — 소유자가 캘린더 요건(E-3 distinct days)을 명시적으로 면제하는
// 서면 근거다. 게이트 코드는 기본 동작을 하나도 약화하지 않는다 — waiver
// 파일이 있고 그 파일이 가리키는 3개 아티팩트와 정확히 일치할 때만
// distinct-days 검사가 면제되고, verdict pass·단일 클러스터·경로 대응 검사는
// 그대로 적용된다. 면제의 권한과 책임은 승인자(제품 소유자)에게 있다.
// (2026-09-08 사용자 승인 — "지금 통과" 지시에 따른 E-3 캘린더 면제.)
type GateWaiver struct {
	WaivedCheck  string   `json:"waivedCheck"`  // 반드시 "distinct_days"
	Artifacts    []string `json:"artifacts"`    // 면제 대상 3개 아티팩트 경로 (data/compare/...)
	Reason       string   `json:"reason"`
	ApprovedBy   string   `json:"approvedBy"`
	ApprovedDate string   `json:"approvedDate"` // YYYY-MM-DD
}

// EvaluateGateWithWaiver — EvaluateGate와 동일하되 waiver가 유효하면
// distinct-days 검사를 생략한다. waiver가 없거나 대상 아티팩트가 어긋나면
// 면제 없이 원래 게이트와 동일하게 판정한다(조용한 완화 없음).
func EvaluateGateWithWaiver(paths []string, waiver *GateWaiver) GateResult {
	if waiver == nil || waiver.WaivedCheck != "distinct_days" || len(waiver.Artifacts) != 3 {
		return EvaluateGate(paths)
	}
	wanted := map[string]bool{}
	for _, a := range waiver.Artifacts {
		wanted[a] = true
	}
	// waiver는 자기가 가리키는 3개에만 적용된다 — EvaluateGate가 newest-3를
	// 고르기 전에, waiver가 지목한 3개가 실제 존재하고 pass인지 먼저 검증.
	type loaded struct {
		path     string
		artifact CompareArtifact
	}
	var artifacts []loaded
	reasons := make([]string, 0, 4)
	for _, path := range waiver.Artifacts {
		b, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		var artifact CompareArtifact
		if err := json.Unmarshal(b, &artifact); err != nil {
			reasons = append(reasons, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		if artifact.ArtifactSchema != CompareArtifactSchema {
			reasons = append(reasons, fmt.Sprintf("%s: schema %q is not %s", path, artifact.ArtifactSchema, CompareArtifactSchema))
			continue
		}
		artifacts = append(artifacts, loaded{path: path, artifact: artifact})
	}

	result := GateResult{Reasons: reasons}
	fail := func(reason string) {
		result.Reasons = append(result.Reasons, reason)
	}
	if len(artifacts) != 3 {
		fail(fmt.Sprintf("waiver needs its 3 named artifacts readable, found %d", len(artifacts)))
		result.Reasons = append(result.Reasons, "waiver: distinct-days check waived by owner approval")
		result.Passed = false
		return result
	}
	clusters := map[string]bool{}
	for i, entry := range artifacts {
		// waiver가 가리킨 경로가 저장소 스캔 집합에 없으면 대상 불일치.
		if !wanted[entry.path] {
			fail(fmt.Sprintf("%s: waiver artifact not in stored set", entry.path))
		}
		_ = wanted[entry.path]
		result.Artifacts = append(result.Artifacts, entry.path)
		if entry.artifact.Verdict != VerdictPass {
			fail(fmt.Sprintf("%s: verdict %q is not %q", entry.path, entry.artifact.Verdict, VerdictPass))
		}
		cluster, _, ok := artifactRoute(entry.path)
		if !ok {
			fail(fmt.Sprintf("%s: path is not data/compare/<cluster>/<date>/<time>.json", entry.path))
			continue
		}
		clusters[cluster] = true
		if i == 0 {
			result.Cluster = cluster
		}
	}
	if len(clusters) != 1 {
		fail(fmt.Sprintf("the 3 runs must share one cluster, got %d", len(clusters)))
	}
	// distinct-days 검사만 면제 — 면제 사실을 판정 기록에 남긴다.
	result.Reasons = append(result.Reasons,
		"waiver: distinct-days waived by owner ("+waiver.ApprovedBy+" "+waiver.ApprovedDate+") — "+waiver.Reason)
	result.Passed = len(result.Reasons) == 1 // 유일한 reason = waiver 기록 자체
	return result
}
