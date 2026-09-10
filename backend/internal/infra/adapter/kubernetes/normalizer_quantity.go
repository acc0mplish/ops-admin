package kubernetes

// 단위 정규화 전담 분할(R-P3 — normalizer.go 650행 계약): §3.2 Ki→GB·코어
// 소수 파서와 P1-D 조립 원천의 원문량 헬퍼를 한 몸으로 둔다. mapping.md §2
// 단위 규칙의 코드화다.

import (
	"math"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// --- 단위 정규화 (§3.2 — Ki→GB, 코어 소수) ---

const (
	kib = 1024.0
	mib = 1024.0 * 1024.0
	gib = 1024.0 * 1024.0 * 1024.0
)

// parseQuantityBytes parses a Kubernetes storage quantity (Ki/Mi/Gi/Ti or
// plain bytes) into bytes.
func parseQuantityBytes(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)
	suffixes := []struct {
		suffix     string
		multiplier float64
	}{
		{"ki", kib}, {"mi", mib}, {"gi", gib},
		{"ti", gib * 1024}, {"k", 1000}, {"m", 1e6}, {"g", 1e9},
	}
	for _, sf := range suffixes {
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

// quantityToGB converts a storage quantity to GB (GiB-scale, 3 decimals) —
// mapping.md 단위 규칙과 동치(T51 검증 대상).
func quantityToGB(s string) (float64, bool) {
	bytes, ok := parseQuantityBytes(s)
	if !ok {
		return 0, false
	}
	return round3(bytes / gib), true
}

// quantityToCores parses a CPU quantity ("8", "7500m") into cores.
func quantityToCores(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil {
			return 0, false
		}
		return round3(n / 1000), true
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return round3(n), true
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// firstQuantityText returns the first non-blank quantity string — the legacy
// firstNonEmpty semantics the storage capacity display uses.
func firstQuantityText(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// --- 판정 ⑤ (b) — kubeadm-config 2키 화이트리스트 (보존 제약 #7의 유일 예외) ---

// kube-system/kubeadm-config의 네트워크 위상 2키만 data 값에서 추출한다(사용자
// 승인 2026-09-10). 소스 YAML 키 → normalized 키 매핑이 화이트리스트의 전부다 —
// 이 외의 어떤 data 값도 정규화에 진입하지 않으며, 키 확장은 리뷰 승인을 전제로
// 한다(mapping.md §2·§6 봉인 문언).
const (
	kubeadmConfigNamespace = "kube-system"
	kubeadmConfigName      = "kubeadm-config"
)

var kubeadmConfigValueKeys = map[string]string{
	"serviceSubnet": "serviceCIDR",
	"podSubnet":     "podSubnetCIDR",
}

// applyKubeadmNetworkKeys scans the kubeadm-config data values line-by-line —
// legacy resolveK8sNetworkCIDRs(k8s_overview.go:31)의 수집측 승계: `key: value`
// 행에서 주석·따옴표를 벗겨 화이트리스트 2키만 채운다.
func applyKubeadmNetworkKeys(n contract.JSONMap, data map[string]string) {
	for _, content := range data {
		for _, line := range strings.Split(content, "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
			if !ok {
				continue
			}
			value = strings.Trim(strings.TrimSpace(strings.Split(value, "#")[0]), "\"'")
			normalizedKey, allowed := kubeadmConfigValueKeys[strings.TrimSpace(key)]
			if !allowed || value == "" {
				continue
			}
			n[normalizedKey] = value
		}
	}
}
