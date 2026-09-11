// executor_create.go — I10 §3.4: 커넥션-스코프 create leg(k8s.resource.create)의
// OperationExecutor leg. uid-스코프 오퍼레이션 모델과 다른 면이다 — create 대상은
// 아직 존재하지 않아 URN이 없고(엔진 체인이 ExternalURN=""으로 공급 — engine_resolve.go),
// 목표 신원은 매니페스트 자신이 동결한다(§3.5 — create의 동결 앵커는 매니페스트).
//
//	wire:  POST <컬렉션 경로> 1회 — 본문은 동결 매니페스트 그대로(v1 create 와이어,
//	       주입 없음 — apply의 resourceVersion 주입은 update 전용 와이어다).
//	handle: state|<connUID>|<항목 경로>|<base64url({resource:"manifest", …동결
//	        manifest})> — apply·delete와 동일 state 가족 인코딩(executor_state.go
//	        무편집 — §0 인코딩 (ii)). Poll은 기존 pollStateRef → pollManifestRef
//	        (GET 1회·부분집합 에코)로 종단을 판정한다.
//
// 3분기(§3.4 — RI-5): POST 201 = 종단 진입(핸들 반환). 409 AlreadyExists는
// 유일하게 GET 판정으로 진입하는 상태다 — 항목 경로 GET 1회의 부분집합 에코가
// 일치하면 수렴 종단(크래시 → 리퍼 재실행이 이미 도달한 목표를 실패로 오보하지
// 않는다), 불일치면 즉시 충돌 실패(타인의 객체 — v1 409 실패 UX 패리티). 그 외
// 오류는 POST 에러를 그대로 돌려 엔진 재시도(MaxAttempts 3)에 맡긴다.
//
// 서빙 면은 매핑 표 단일 원천(internal/infra/contract/k8s_create_face.go — §3.2.1
// 파생 계약)에서 파생된다: 본 leg는 표의 Plural·APIVersions·Namespaced·APIGroup
// 소유값으로 컬렉션 경로를 조립할 뿐 종 어휘를 소유하지 않는다(이중 열거 금지).
// APIVersions는 표에 기본 순서(v1→v1beta1)로 담기고, 실행 시 매니페스트 선언
// 버전을 1순위로 재정렬한다(J0 리뷰 LOW-1 — v1 buildIstioResourcePathsWithPreferred
// 패리티). 재정렬은 항상 신규 슬라이스로 반환한다(J0 리뷰 LOW-2 — 표의 공유
// 백킹 배열 보존).
package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"ops-admin/backend/internal/infra/contract"

	ksyaml "sigs.k8s.io/yaml"
)

// CreateOperationName — compose_k8s_ops.go opdef 등록과 dispatch(executor.go)가
// 공유하는 단일 원천(§J-P1-6 — descriptors are code). 리터럴은 J0가 등록한
// compose def·opdef_table_test 행과 동일 문자열이다.
const CreateOperationName = "k8s.resource.create"

// createTarget — 신원 가드가 확정한 create 목표. 경로 조립과 핸들 부호화의
// 유일 입력이다(가드 통과 후 재파싱 없음).
type createTarget struct {
	Manifest        map[string]any              // 동결 매니페스트 — POST 본문이자 핸들 기대상태
	Entry           contract.K8sCreateFaceEntry // 매핑 표 행 — 경로 성분의 단일 원천
	Name            string                      // metadata.name(trimmed)
	Namespace       string                      // namespaced 종의 metadata.namespace(trimmed) — 클러스터 스코프는 공백
	DeclaredVersion string                      // apiVersion의 버전 성분("networking.istio.io/v1beta1" → "v1beta1")
}

// createManifestOf — Payload["yaml"] 검증과 신원 가드(§3.2.1 규칙 —
// applyManifestOf 강도 승계): 공백 아닌 YAML 문자열 → JSON 객체 디코드
// (sigs.k8s.io/yaml — v1과 동일 변환·동일 "invalid yaml content" 문언) →
// apiVersion·kind 필수(v1 parseK8sManifestIdentity 문언 계승) → 매핑 표 소속
// (group×kind 불일치는 교차-kind 사전 거부 — RI-4) → metadata.name 필수 →
// namespace 규칙(namespaced 종은 필수, 클러스터 스코프 종은 공백). create에는
// URN 목표가 없으므로 kind·namespace 정합의 기준은 표 자신이다.
func createManifestOf(payload contract.JSONMap) (createTarget, error) {
	var spec struct {
		YAML string `json:"yaml"`
	}
	if err := payloadDecode(payload, &spec); err != nil {
		return createTarget{}, err
	}
	if strings.TrimSpace(spec.YAML) == "" {
		return createTarget{}, errors.New(`kubernetes: payload carries no "yaml" — the plan request must freeze the resource manifest (v1 "invalid yaml payload")`)
	}
	raw, err := ksyaml.YAMLToJSON([]byte(spec.YAML))
	if err != nil {
		return createTarget{}, errors.New("kubernetes: invalid yaml content")
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest == nil {
		return createTarget{}, errors.New("kubernetes: invalid yaml content")
	}
	apiVersion, _ := manifest["apiVersion"].(string)
	apiVersion = strings.TrimSpace(apiVersion)
	if apiVersion == "" {
		return createTarget{}, errors.New("kubernetes: resource apiVersion is required")
	}
	kind, _ := manifest["kind"].(string)
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return createTarget{}, errors.New("kubernetes: resource kind is required")
	}
	entry, ok := contract.K8sCreateFace(contract.K8sAPIGroupOf(apiVersion), kind)
	if !ok {
		return createTarget{}, fmt.Errorf("kubernetes: kind %q in apiVersion %q is not a k8s.resource.create face (mapping table contract)", kind, apiVersion)
	}
	metadata, _ := manifest["metadata"].(map[string]any)
	name, _ := metadata["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return createTarget{}, errors.New("kubernetes: resource name is required")
	}
	namespace, _ := metadata["namespace"].(string)
	target := createTarget{
		Manifest:        manifest,
		Entry:           entry,
		Name:            name,
		DeclaredVersion: apiVersion[strings.IndexByte(apiVersion, '/')+1:],
	}
	if !strings.Contains(apiVersion, "/") {
		// core apiVersion("v1") — 버전 성분은 문자열 자신이다.
		target.DeclaredVersion = apiVersion
	}
	if entry.Namespaced {
		target.Namespace = strings.TrimSpace(namespace)
		if target.Namespace == "" {
			return createTarget{}, fmt.Errorf("kubernetes: create of namespaced kind %q carries no metadata.namespace", kind)
		}
		return target, nil
	}
	if strings.TrimSpace(namespace) != "" {
		return createTarget{}, fmt.Errorf("kubernetes: cluster-scoped kind %q carries namespace %q", kind, namespace)
	}
	return target, nil
}

// realignAPIVersions — 매니페스트 선언 버전을 1순위로 재정렬한다(J0 리뷰 LOW-1 —
// v1 buildIstioResourcePathsWithPreferred의 선언 v1beta1 → [v1beta1, v1] 패리티).
// 선언 버전이 후보 안에 없거나 공백이면 기본 순서다. 출력은 항상 신규 슬라이스다
// (J0 리뷰 LOW-2 — apiV1Only·apiV1Preferred 공유 백킹 배열 보존).
func realignAPIVersions(declared string, candidates []string) []string {
	first := make([]string, 0, len(candidates))
	rest := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if declared != "" && candidate == declared {
			first = append(first, candidate)
			continue
		}
		rest = append(rest, candidate)
	}
	return append(first, rest...)
}

// createCollectionPaths — 표 소유 성분에서 조립한 컬렉션 경로 후보(선호 순).
// core(APIGroup "")는 /api/v1, 그룹은 /apis/<group>/<version>이고 namespaced
// 종은 /namespaces/<ns> 세그먼트를 끼운다(삭제된 v1 create 경로 빌더의
// 표 파생 일반화 — I10 J3 사멸 분계).
func createCollectionPaths(entry contract.K8sCreateFaceEntry, declaredVersion, namespace string) []string {
	versions := realignAPIVersions(strings.TrimSpace(declaredVersion), entry.APIVersions)
	paths := make([]string, 0, len(versions))
	for _, version := range versions {
		var b strings.Builder
		if entry.APIGroup == "" {
			b.WriteString("/api/v1")
		} else {
			b.WriteString("/apis/" + entry.APIGroup + "/" + version)
		}
		if entry.Namespaced {
			b.WriteString("/namespaces/" + namespace)
		}
		b.WriteString("/" + entry.Plural)
		paths = append(paths, b.String())
	}
	return paths
}

// unexpectedStatusOf — doJSON 비-2xx 일반 에러 문언("kubernetes: unexpected
// status: <code>[, 본문 스니펫]")에서 상태 코드를 회복한다. client.go는 404만
// 타입 센티넬(errNotFound)로 노출하고 client.go는 본 leg의 편집 목록 밖이라(§5 #4
// — 판단 기록: 선타입 errAlreadyExists 센티넬이 더 깨끗한 미래 모양이지만 본
// Phase의 산출물 계약이 client.go 무편집을 못 박는다), 선두 앵커 파싱이 409
// 식별 경로다. 앵커 파싱의 안전성: 코드 필드는 메시지 선두에서 시작해 쉼표(또는
// 문자열 끝)로 닫히므로 본문 스니펫 내용이 코드 필드에 들어올 수 없다. 문언
// 포맷이 바뀌면 (0, false) — 비-409로 분류돼 POST 에러가 그대로 반환된다(수렴
// 유실은 있어도 오탐 없음 — fail-safe). 포맷 결합은 409 3분기 테스트가 패키지
// 내에서 잠근다.
func unexpectedStatusOf(err error) (int, bool) {
	const prefix = "kubernetes: unexpected status: "
	rest, ok := strings.CutPrefix(err.Error(), prefix)
	if !ok {
		return 0, false
	}
	end := strings.IndexByte(rest, ',')
	if end < 0 {
		end = len(rest)
	}
	code, convErr := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if convErr != nil {
		return 0, false
	}
	return code, true
}

// executeResourceCreate — 검증(왕복 0회) → POST 1회 → 3분기 → state| 핸들.
// 재실행은 동일 payload의 재 POST다 — 201(신규 생성)이든 409+에코(이미 도달)든
// 동일 handle로 수렴한다(provider_create_convergent — 동일 payload → 동일 동결
// manifest → 동일 ProviderRef).
func (a *Adapter) executeResourceCreate(ctx context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	// 커넥션-스코프 가드 — 엔진 체인(conn: 분기)은 ExternalURN=""을 공급한다.
	// 채워진 URN은 uid-스코프 오퍼레이션과의 배선 교차(오발사)다.
	if strings.TrimSpace(req.ResourceURN) != "" {
		return contract.OperationHandle{}, fmt.Errorf("kubernetes: %s is connection-scoped and carries no resource URN (got %q)", CreateOperationName, req.ResourceURN)
	}
	target, err := createManifestOf(req.Payload)
	if err != nil {
		return contract.OperationHandle{}, err
	}
	client, err := a.executionClient(req)
	if err != nil {
		return contract.OperationHandle{}, err
	}

	collections := createCollectionPaths(target.Entry, target.DeclaredVersion, target.Namespace)
	itemPath := collections[0] + "/" + target.Name
	postErr := client.doJSON(ctx, http.MethodPost, collections[0], nil, "application/json", target.Manifest, "execute", nil)
	if postErr == nil {
		return createStateHandle(req.Connection.UID, itemPath, target)
	}
	if status, ok := unexpectedStatusOf(postErr); ok && status == http.StatusConflict {
		var current map[string]any
		getErr := client.getJSONOp(ctx, itemPath, nil, "execute", &current)
		switch {
		case getErr == nil && manifestSubsumes(target.Manifest, current):
			// 수렴 종단 — 폴이 GET으로 재확인한다(pollManifestRef 무편집 재사용).
			return createStateHandle(req.Connection.UID, itemPath, target)
		case getErr == nil:
			return contract.OperationHandle{}, fmt.Errorf("kubernetes: %s %q already exists with different content — create does not overwrite a foreign object", target.Entry.Kind, itemPath)
		case errors.Is(getErr, errNotFound):
			// 409 → GET 404: 경쟁 소실(삭제 선행) — 판단 기록: 타인 객체 충돌이
			// 아니므로 충돌 실패가 아니라 원 POST 에러로 재시도한다.
			return contract.OperationHandle{}, postErr
		default:
			return contract.OperationHandle{}, getErr
		}
	}
	// 그 외 오류 — executor error(엔진 재시도 MaxAttempts 3 — §3.4). 판단 기록(r3 LOW 이월): 404(네임스페이스 부재 등)에도 차후보(APIVersion 후보 경로) 폴백이 없다 — v1 k8sDoJSONAnyPath의 경로 순회와 달리 POST 1회 단발이며 재시도도 동일 선호 경로를 향한다(§3.4 정합).
	return contract.OperationHandle{}, postErr
}

// createStateHandle — state 가족 부호화(executor_state.go encodeStateRef —
// apply·delete와 동일 인코딩, §0 인코딩 (ii)). 기대상태는 POST 전 동결
// manifest다(주입 없음 — 재실행 byte-identical의 근거).
func createStateHandle(connUID, itemPath string, target createTarget) (contract.OperationHandle, error) {
	ref, err := encodeStateRef(connUID, itemPath, stateExpectation{Resource: "manifest", Manifest: target.Manifest})
	if err != nil {
		return contract.OperationHandle{}, err
	}
	return contract.OperationHandle{ProviderRef: ref}, nil
}
