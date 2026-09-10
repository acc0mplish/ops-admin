package inventory

import (
	"encoding/json"

	"ops-admin/backend/internal/infra/contract"
	v1model "ops-admin/backend/model"
)

// k8sassembly_certificates.go — P1-G2 (판정 ④ A′-1 결착 — 사용자 승인
// 2026-09-10): provider_connection.ConfigJSON에 기록된 health_observation 관측
// (어댑터가 도출한 파생 메타데이터 — P1-G1, compose/healthloop.go가 기록)을
// legacy certificates[] 형상으로 조립한다.

// BuildOverviewCertificates — conn.ConfigJSON["health_observation"]["certificates"]
// 관측을 v1model.K8sCertificate 9필드로 디코드한다. 도출·만료 상태 판정은
// 어댑터 소관이고(adapter/kubernetes certificates.go — v1
// parseOverviewCertificate 동치), 조립은 JSON 어휘(name·type·…·statusText —
// K8sCertificate 태그와 1:1)의 통과가 전부다. 관측 부재(스윕 전·파생 실패·
// 관측 없는 어댑터)는 v1 buildOverviewCertificates 동치의 빈 슬라이스다.
// JSON 왕복 디코드는 DB 관측 형상([]any·map[string]any·float64)과 스윕 직후
// 형상([]contract.JSONMap)을 모두 흡수한다(k8s_v2glue_test v2Row 선례).
func BuildOverviewCertificates(config contract.JSONMap) []v1model.K8sCertificate {
	certificates := make([]v1model.K8sCertificate, 0, 2)
	observation, ok := config["health_observation"]
	if !ok {
		return certificates
	}
	blob, err := json.Marshal(observation)
	if err != nil {
		return certificates
	}
	var decoded struct {
		Certificates []v1model.K8sCertificate `json:"certificates"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		return certificates
	}
	return append(certificates, decoded.Certificates...)
}
