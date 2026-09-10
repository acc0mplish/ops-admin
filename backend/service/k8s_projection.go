// k8s_projection.go — G0 읽기 3건(cluster/list·info·detail)의 V2 소스 전환
// 착지점(phase6-plan §3.1·§J8). P 트랙 산출물인
// inventory.AssembleK8sClusterDetail·inventory.ProjectResources를 소비만
// 한다(보존 제약 #6 — internal/infra 무변경). 임시 플래그 V2_READ_SOURCE_K8S는
// 전환 검증·롤백용 장치(#12)로 G1에서 legacy 분기와 함께 제거된다.
//
// ID 공간 판단(구현 판정 기록 — 2026-09-11): V2 투영 뷰의 응답 id는
// conn.ID가 아니라 legacy source_id(k8s_cluster.id)로 되돌려 노출한다. 근거 —
// ① 프론트 무변경(C56)이 G1·H 전 구간에서 유지되려면 list→info/detail→
// 라이브 패스스루(node·pod·terminal·metrics — 보존 제약 #7 v1 무기한 존치)가
// 같은 id 공간을 공유해야 한다. ② §J8이 info를 컨트롤러 착지점으로 분리한
// 이유 자체가 GetK8sCluster 소비 12곳(S5)이 legacy id 공간을 계속 쓰기
// 때문이다. ③ C53 compare의 페어링(--cluster <legacy id>)이
// source_model/source_id 조회(main_compare.go:83)로 성립한다. 조립물 내부의
// Cluster.ID(conn.ID)는 응답 직전 source_id로 되돌린다 — compare 집합은
// cluster 뷰 필드를 제외하므로(legacyCaptureFromDetail은 섹션만 캡처) P가
// 잠근 패리티 계약과 충돌하지 않는다.
//
// register-k8s 체인(H0 신설 — source 쌍 없는 커넥션)은 I-a S5가
// GetK8sCluster를 provider_connection 소스로 전환하기 전까지 본 투영 목록에
// 나타나지 않는다(H0→I-a 창의 알려진 제한 — 리포트 이월 판정).
package service

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/inventory"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

// k8sReadSourceV2 — 전환 플래그 판정의 단일 헬퍼: 값을 trim·소문자 정규화해
// ""·"0"·"false"는 OFF(§11 기본 legacy), 그 외는 ON으로 읽는다. 호출부는 env
// 이름을 인자로 넘겨 3착지점(k8s.go 2분기·info)이 같은 정규화를 공유한다.
func k8sReadSourceV2(env string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(env))) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// G0 투영의 소스 해상은 백필 체인(source_model/source_id 쌍)에 한정한다.
const (
	k8sProjectionProvider    = "kubernetes"
	k8sProjectionSourceModel = "k8s_cluster"
)

// projectK8sClusterList — cluster/list의 V2 소스. 백필 체인 커넥션을
// source_id 오름차순(legacy Order("id asc") 대응)으로 열거한다. 공개
// generation이 없는 커넥션(동기화 전)은 공개 읽기면(§5.4c·N8)에서 부재로
// 읽힌다 — 목록 전체의 오류로 승격하지 않는다.
func (s *Service) projectK8sClusterList() ([]model.K8sClusterView, error) {
	var conns []infraModel.ProviderConnection
	if err := s.db.Where("provider_type = ? AND source_model = ? AND stale_source = ?",
		k8sProjectionProvider, k8sProjectionSourceModel, false).
		Order("source_id asc").Find(&conns).Error; err != nil {
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

// ProjectK8sClusterInfo — cluster/info의 V2 착지점(컨트롤러 k8s.go:32가 직접
// 호출 — §J8). 플래그 기본값은 legacy(§11 롤백 계약)라 분기는 본 함수 안에
// 둔다. V2 소스에는 평문 kubeconfig를 채울 수단이 없다(봉인 계약 §12 #16) —
// 해당 필드는 영값(빈 문자열)으로 직렬화된다(C69·R-I — §12 #1의 유일 예외).
func (s *Service) ProjectK8sClusterInfo(clusterID uint) (model.K8sCluster, error) {
	if k8sReadSourceV2("V2_READ_SOURCE_K8S") {
		view, err := s.projectK8sClusterInfoView(clusterID)
		if err != nil {
			return model.K8sCluster{}, err
		}
		return s.k8sClusterFromProjection(view)
	}
	return s.GetK8sCluster(clusterID)
}

// projectK8sClusterDetail — cluster/detail의 V2 소스(GetK8sClusterDetail의
// singleflight 본문에서 분기). 캐시 구조·TTL은 무변경(§J8 캐시 승계)이며
// 캐시 키는 양 분기 모두 legacy 클러스터 id다.
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
	detail.Cluster.ID = conn.SourceID
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
	view.ID = conn.SourceID
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

// resolveK8sProjectionConnection — legacy 클러스터 id를 백필 체인 커넥션으로
// 해상한다(C53 페어링과 동일 조건 — main_compare.go:83). stale 행은 부재다.
func (s *Service) resolveK8sProjectionConnection(clusterID uint) (*infraModel.ProviderConnection, error) {
	var conn infraModel.ProviderConnection
	err := s.db.Where("provider_type = ? AND source_model = ? AND source_id = ? AND stale_source = ?",
		k8sProjectionProvider, k8sProjectionSourceModel, clusterID, false).First(&conn).Error
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
