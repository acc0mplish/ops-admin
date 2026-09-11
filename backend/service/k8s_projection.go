// k8s_projection.go — G0 읽기 3건(cluster/list·info·detail)의 V2 소스 전환
// 착지점, G1에서 단일 경로 고정(phase6-plan §3.1·§J8·C36 2단). P 트랙 산출물인
// inventory.AssembleK8sClusterDetail·inventory.ProjectResources를 소비만
// 한다(보존 제약 #6 — internal/infra 무변경). G0의 전환 검증·롤백용 임시
// 플래그(#12)는 G1에서 legacy 분기와 함께 제거됐다 — V2가 유일 소스다.
//
// ID 공간 판단(I-b — 구현 판정 기록 2026-09-11): 응답 id는 체인 형상별로
// 배정된다. ① source 쌍 체인(source_id>0 — 백필 유산)은 legacy source_id를
// 그대로 노출한다(G0 승인 계약 유지 — 이 집합은 step0007이 stale_source=true로
// 봉인해 동결·소멸 예정이다). ② register-k8s 체인(source_id=0 — source 쌍
// 없는 커넥션)은 conn.ID를 노출한다 — 노출할 source_id가 없고 conn.ID는 이
// 체인의 자연 키다. 두 공간의 수치 충돌 시 source 쌍 체인이 해상 우선권을 갖는다
// (resolveK8sProjectionConnection·GetK8sCluster 이중 해상 — 결정론적). 충돌
// 표면은 유한하다: 백필 source_id 집합은 S1c(I-a) 제거로 더 이상 증가하지
// 않는다. 라우트·골든 무변경 하에 register-k8s 체인이 투영 목록·info·detail에
// 가시화되며, 이로써 H0→I-a 창에서 알려진 제한이었던 source-less 체인 비가시
// 문제가 종결된다(Ia 판정 4 — legacy id 공간 종결의 코드 편).
package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/inventory"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

// k8sProjectionSourceModel — source 쌍 체인(백필 유산)의 식별값. I-b부터
// 목록·단건 해상은 이 값에 한정하지 않는다(register-k8s 체인 가시화 — 패키지
// 문서 ID 공간 판단). 이 상수는 source 쌍 체인의 이중 해상 1순위 조건에만
// 쓰인다.
const (
	k8sProjectionProvider    = "kubernetes"
	k8sProjectionSourceModel = "k8s_cluster"
)

// projectK8sClusterList — cluster/list의 V2 소스. live kubernetes 커넥션을
// conn.ID 오름차순으로 열거한다(I-b: source_model 필터 제거 — register-k8s
// 체인 가시화. post-drop live 집합은 register-k8s 체인이고 노출 id == conn.ID이므로
// conn.ID 정렬이 legacy Order("id asc") 대응을 계승한다). 공개 generation이
// 없는 커넥션(동기화 전)은 공개 읽기면(§5.4c·N8)에서 부재로 읽힌다 — 목록
// 전체의 오류로 승격하지 않는다.
func (s *Service) projectK8sClusterList() ([]model.K8sClusterView, error) {
	var conns []infraModel.ProviderConnection
	if err := s.db.Where("provider_type = ? AND stale_source = ?",
		k8sProjectionProvider, false).
		Order("id asc").Find(&conns).Error; err != nil {
		return nil, fmt.Errorf("k8s projection: list connections: %w", err)
	}
	result := make([]model.K8sClusterView, 0, len(conns))
	for i := range conns {
		view, err := s.projectK8sClusterView(&conns[i])
		if err != nil {
			if errors.Is(err, errK8sProjectionNotPublished) {
				continue
			}
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

// k8sProjectionViewID — 커넥션의 노출 id. source 쌍 체인은 legacy source_id,
// register-k8s 체인은 conn.ID(패키지 문서 ID 공간 판단).
func k8sProjectionViewID(conn *infraModel.ProviderConnection) uint {
	if conn.SourceID > 0 {
		return conn.SourceID
	}
	return conn.ID
}

// ProjectK8sClusterInfo — cluster/info의 V2 착지점(컨트롤러 k8s.go:32가 직접
// 호출 — §J8). G1 단일 경로: legacy 행 위임 분기는 제거됐다. V2 소스에는
// 평문 kubeconfig를 채울 수단이 없다(봉인 계약 §12 #16) — 해당 필드는
// 영값(빈 문자열)으로 직렬화된다(C69·R-I — §12 #1의 유일 예외).
func (s *Service) ProjectK8sClusterInfo(clusterID uint) (model.K8sCluster, error) {
	view, err := s.projectK8sClusterInfoView(clusterID)
	if err != nil {
		return model.K8sCluster{}, err
	}
	return s.k8sClusterFromProjection(view)
}

// projectK8sClusterDetail — cluster/detail의 V2 소스(GetK8sClusterDetail의
// singleflight 본문이 호출). 캐시 구조·TTL은 무변경(§J8 캐시 승계)이며
// 캐시 키는 legacy 클러스터 id다.
func (s *Service) projectK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, error) {
	conn, err := s.resolveK8sProjectionConnection(clusterID)
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	contextID, err := s.projectK8sContextID(conn)
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	rows, err := s.projectK8sRows(contextID)
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	detail, err := inventory.AssembleK8sClusterDetail(s.db, conn, rows)
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	detail.Cluster.ID = k8sProjectionViewID(conn)
	return detail, nil
}

// projectK8sClusterInfoView / projectK8sClusterView — 조립은 P 산출물에 위임해
// 매핑의 단일 원천을 유지하고, 응답 id만 legacy 공간으로 되돌린다(패키지 문서
// 판정 기록).
func (s *Service) projectK8sClusterInfoView(clusterID uint) (model.K8sClusterView, error) {
	conn, err := s.resolveK8sProjectionConnection(clusterID)
	if err != nil {
		return model.K8sClusterView{}, err
	}
	return s.projectK8sClusterView(conn)
}

func (s *Service) projectK8sClusterView(conn *infraModel.ProviderConnection) (model.K8sClusterView, error) {
	contextID, err := s.projectK8sContextID(conn)
	if err != nil {
		return model.K8sClusterView{}, err
	}
	rows, published, err := s.projectK8sPublishedRows(contextID)
	if err != nil {
		return model.K8sClusterView{}, err
	}
	if !published {
		return model.K8sClusterView{}, errK8sProjectionNotPublished
	}
	detail, err := inventory.AssembleK8sClusterDetail(s.db, conn, rows)
	if err != nil {
		return model.K8sClusterView{}, err
	}
	view := detail.Cluster
	view.ID = k8sProjectionViewID(conn)
	return view, nil
}

// k8sClusterFromProjection — 조립된 cluster 뷰를 info 응답 DTO로 옮긴다.
// 해당 필드는 항상 영값으로 둔다(C69 — 리터럴 금지)·LastSyncAt은 VOLATILE
// 제외(P1-D-5 판정)로 nil이다. gateway·monitor_datasource는 legacy Preload에
// 대응해 id로 조회하며, 행 부재(ErrRecordNotFound)만 Preload와 같이 영값으로
// 용인하고 그 외 DB 에러는 호출자로 전파한다.
func (s *Service) k8sClusterFromProjection(view model.K8sClusterView) (model.K8sCluster, error) {
	cluster := model.K8sCluster{
		ID:                  view.ID,
		Name:                view.Name,
		Status:              view.Status,
		APIServer:           view.APIServer,
		Version:             view.Version,
		NodeCount:           view.NodeCount,
		Env:                 view.Env,
		Tags:                view.Tags,
		ConnectionMode:      view.ConnectionMode,
		GatewayID:           view.GatewayID,
		MonitorDatasourceID: view.MonitorDatasourceID,
		Description:         view.Description,
		CreatedAt:           view.CreatedAt,
		UpdatedAt:           view.UpdatedAt,
	}
	if cluster.GatewayID != nil && *cluster.GatewayID > 0 {
		if err := s.db.First(&cluster.Gateway, *cluster.GatewayID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return model.K8sCluster{}, fmt.Errorf("k8s projection: gateway load: %w", err)
			}
		}
	}
	if cluster.MonitorDatasourceID != nil && *cluster.MonitorDatasourceID > 0 {
		if err := s.db.First(&cluster.MonitorDatasource, *cluster.MonitorDatasourceID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return model.K8sCluster{}, fmt.Errorf("k8s projection: monitor datasource load: %w", err)
			}
		}
	}
	return cluster, nil
}

// resolveK8sProjectionConnection — 노출 id를 live 커넥션으로 이중 해상한다.
// 1순위는 source 쌍 체인(source_model/source_id — C53 페어링과 동일 조건, 구
// main_compare.go:83의 계승), 2순위는 register-k8s 체인(source_id=0 — 노출 id
// == conn.ID). 순서가 곧 충돌 우선권이다(패키지 문서 ID 공간 판단). 이 이중
// 해상 순서는 GetK8sCluster(k8s.go)의 2단 조회와 동일 조건이다 — 양측을 함께
// 갱신할 것(L1). stale 행은 부재다.
func (s *Service) resolveK8sProjectionConnection(clusterID uint) (*infraModel.ProviderConnection, error) {
	var conn infraModel.ProviderConnection
	err := s.db.Where("provider_type = ? AND source_model = ? AND source_id = ? AND stale_source = ?",
		k8sProjectionProvider, k8sProjectionSourceModel, clusterID, false).First(&conn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = s.db.Where("provider_type = ? AND source_id = ? AND id = ? AND stale_source = ?",
			k8sProjectionProvider, 0, clusterID, false).First(&conn).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("k8s cluster %d has no live V2 source — run sync-inventory", clusterID)
		}
		return nil, fmt.Errorf("k8s projection: resolve connection for cluster %d: %w", clusterID, err)
	}
	return &conn, nil
}

// projectK8sContextID — 커넥션과 컨텍스트는 1:1이다(§7.3·A5).
func (s *Service) projectK8sContextID(conn *infraModel.ProviderConnection) (uint, error) {
	var pctx infraModel.ProviderContext
	if err := s.db.Where("connection_id = ?", conn.ID).First(&pctx).Error; err != nil {
		return 0, fmt.Errorf("k8s projection: context for connection %d: %w", conn.ID, err)
	}
	return pctx.ID, nil
}

// projectK8sPublishedRows — 공개 generation 존재를 확인하고 관측 행을 투영한다.
// published=false는 "동기화 전 커넥션"이며 호출자가 처분을 결정한다(목록은
// 생략, 단건은 안내 에러).
func (s *Service) projectK8sPublishedRows(contextID uint) ([]inventory.ProjectedResource, bool, error) {
	var published int64
	if err := s.db.Model(&infraModel.InventorySyncRun{}).
		Where("context_id = ? AND status = ? AND committed_at IS NOT NULL", contextID, inventory.RunStatusSucceeded).
		Count(&published).Error; err != nil {
		return nil, false, fmt.Errorf("k8s projection: published check for context %d: %w", contextID, err)
	}
	if published == 0 {
		return nil, false, nil
	}
	rows, err := s.projectK8sRows(contextID)
	if err != nil {
		return nil, false, err
	}
	return rows, true, nil
}

func (s *Service) projectK8sRows(contextID uint) ([]inventory.ProjectedResource, error) {
	generation, err := inventory.LatestAuthoritativeGeneration(s.db, contextID)
	if err != nil {
		return nil, fmt.Errorf("k8s projection: %w", err)
	}
	rows, err := inventory.ProjectResources(s.db, contextID, generation.GenerationUID)
	if err != nil {
		return nil, fmt.Errorf("k8s projection: %w", err)
	}
	return rows, nil
}

// errK8sProjectionNotPublished — 목록에서 생략할 미공개 커넥션 표식.
var errK8sProjectionNotPublished = errors.New("k8s projection: no published generation")
