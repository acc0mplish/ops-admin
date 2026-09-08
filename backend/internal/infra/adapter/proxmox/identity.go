// identity.go — /cluster/status에서 클러스터 신원을 해독하는 등록 스모크 표면
// (plan phase5 §3.3·A11, ④리뷰 LOW). register-pve CLI의 신원 probe가 이전까지
// 자체 HTTP+헤더 조립을 둔 것이 이 표면의 존재 이유다 — 조립을 어댑터 런타임
// 클라이언트(newClient·buildClient) 한 곳으로 합쳐 등록 probe와 런타임 전송
// posture(deployment_mode·TLS)의 드리프트를 없앤다.
package proxmox

import (
	"context"
	"fmt"

	"ops-admin/backend/internal/infra/contract"
)

// ClusterIdentity — /cluster/status가 말하는 등록 대상의 신원: 클러스터명,
// 또는 standalone(A11)일 때 유일 노드명. register-pve 리포트와
// provider_context ExternalID의 출처다.
type ClusterIdentity struct {
	Name       string
	Standalone bool
	Nodes      int
	Quorate    bool
}

// ResolveIdentity — GET /cluster/status 1회를 런타임과 동일한 클라이언트
// 조립(buildClient — 자격 헤더·{data} 래핑·TLS posture 공유)으로 조회해
// ClusterIdentity로 해독한다. cluster행이 있으면 그 name이 신원이고 quorate
// 성분이 Quorate가 된다. cluster행이 없고 node행이 정확히 1개면 standalone
// 폴백(A11) — 노드명이 신원이 된다. 그 외(cluster행 부재 + node 0개 또는
// 2개 이상)는 가짜 클러스터 신원을 만들지 않고 에러로 끝낸다 — Health의
// 보고 폴백(Healthy:true)과 다른 등록 계약이다. 실패는 §9.3 신호 어휘로
// 분류된다(classify — 응답 본문·자격 물질 불노출, claim 13).
func (a *Adapter) ResolveIdentity(ctx context.Context, conn contract.ConnectionView) (ClusterIdentity, error) {
	client, err := a.buildClient(conn)
	if err != nil {
		return ClusterIdentity{}, err
	}
	// quorum 성분은 clusterStatusEntry(전송 소유)에 없다 — Health와 같은
	// 별도 디코드 형상(clusterHealthEntry)을 쓴다. 신원 조회는 1회 단발이므로
	// get의 페일오버 장부(연속 2회 임계)는 발화하지 않는다.
	var entries []clusterHealthEntry
	if err := client.get(ctx, "/cluster/status", "identity", &entries); err != nil {
		return ClusterIdentity{}, err
	}
	var identity ClusterIdentity
	firstNode := ""
	for i := range entries {
		switch entries[i].Type {
		case "cluster":
			identity.Name = entries[i].Name
			identity.Quorate = entries[i].Quorate != nil && *entries[i].Quorate != 0
		case "node":
			identity.Nodes++
			if firstNode == "" {
				firstNode = entries[i].Name
			}
		}
	}
	if identity.Name != "" {
		return identity, nil
	}
	// A11 standalone 폴백: cluster행 부재 + node행 1개 → node명으로 신원 확정.
	if identity.Nodes == 1 && firstNode != "" {
		identity.Name = firstNode
		identity.Standalone = true
		return identity, nil
	}
	return identity, fmt.Errorf("proxmox: /cluster/status reported %d node rows without a cluster row — cannot resolve a cluster identity", identity.Nodes)
}
