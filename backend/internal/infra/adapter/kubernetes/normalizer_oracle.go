// normalizer_oracle.go — Z 오라클 승계용 export 래퍼(계획 P 계획 r3 J-P1-5 — P1-E).
// normalizeSection은 unexported라 service 패키지의 테스트 글루가 프로덕션 디코드를
// 통과할 수 없다. 본 래퍼는 디코드 1종을 그대로 노출하며 로직 추가가 없다 —
// 동치 판정의 대상은 normalizeSection이고 래퍼는 경계만 연다. 프로덕션 호출부는
// 어댑터 내부에 한정된다(변경 불변 목록 외, §5 "승계 유일 허용 변경"의 글루 전제).
package kubernetes

import (
	"encoding/json"

	"ops-admin/backend/internal/infra/contract"
)

// NormalizeSection exposes normalizeSection for the Z oracle glue
// (service/k8s_chartest_helper_test.go): legacy kube 구조체 → json.Marshal →
// 본 함수 → V2 관측 행. ctxID는 URN 성분뿐이라 글루가 고정값을 넣는다.
func NormalizeSection(ctxID uint, section string, raw json.RawMessage) (contract.DiscoveredResource, error) {
	return normalizeSection(ctxID, section, raw)
}
