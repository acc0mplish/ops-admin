// compare_entries.go — legacy·V2 사이드 엔트리 추출과 정규화·수량·포트·json·
// 정렬·해시 헬퍼. 원본 compare.go :420-950 바이트 보존 이동(계획 :422-952 —
// 섹션 주석 동행 경계 미세조정).

package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

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
