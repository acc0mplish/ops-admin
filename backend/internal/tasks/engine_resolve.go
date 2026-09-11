package tasks

import (
	"context"
	"fmt"
	"strings"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
)

// 실행 체인 결정면 (§3.3, I10 J1a) — resource_uid의 두 경로를 여기서
// 결정한다: 실 uid는 infra_resource→context→connection 조인(T-7 바이트
// 승계), conn: 접두 합성 uid는 커넥션 직조립(커넥션-스코프 create).
// executeClaimed 이하는 이 결정면만 소비한다.

// executionChain is the T-7 join result: resource_uid → infra_resource →
// provider_context → provider_connection → ProviderType.
type executionChain struct {
	ConnectionUID string
	ProviderType  string
	ExternalURN   string
}

// connUIDPrefix — §3.3 합성 resource_uid 접두부. 커넥션-스코프 create(I10)가
// "아직 존재하지 않는 리소스" 대신 커넥션+매니페스트 신원을 앵커할 때
// provider_task.resource_uid에 `conn:<connectionUID>:<singular>:<target>`가
// 들어온다. infra_resource.uid는 이 접두로 시작하지 않는다(예약 단얫 —
// engine_resolve_test 가드) — 접두가 곧 합성 uid의 판별자다.
const connUIDPrefix = "conn:"

// syntheticUIDSegments — 합성 uid의 콜론 분절 수(conn: 접두 제거 기준).
// `<connectionUID>:<singular>:<target>`의 3분절이 계약이다 — target은
// namespaced `<ns>/<name>` 또는 cluster-scope `<name>` 2형태(§3.3).
// k8s 이름·네임스페이스 어휘에 콜론이 없어 분절 초과는 곧 잘못된 조립이다.
const syntheticUIDSegments = 3

// resolveExecutionChain walks the two §3.3 paths: a `conn:`-prefixed synthetic
// uid resolves the connection directly, everything else walks the three-level
// join. Soft-deleted resources are outside the default scope (deleted_at IS
// NULL) — a miss (unknown or deleted resource, or a broken chain) is an
// `unknown resource` hard error the caller terminal-fails without retry (T-7).
func (e *Engine) resolveExecutionChain(ctx context.Context, resourceUID string) (executionChain, error) {
	if strings.HasPrefix(resourceUID, connUIDPrefix) {
		return e.resolveConnectionScopedChain(ctx, resourceUID)
	}
	var row executionChain
	res := e.db.WithContext(ctx).
		Table("infra_resource").
		Select("provider_connection.uid AS connection_uid, provider_connection.provider_type AS provider_type, infra_resource.external_urn AS external_urn").
		Joins("JOIN provider_context ON provider_context.id = infra_resource.context_id").
		Joins("JOIN provider_connection ON provider_connection.id = provider_context.connection_id").
		Where("infra_resource.uid = ? AND infra_resource.deleted_at IS NULL", resourceUID).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return executionChain{}, fmt.Errorf("tasks: execution chain join for %q: %w", resourceUID, res.Error)
	}
	if row.ConnectionUID == "" {
		return executionChain{}, fmt.Errorf("tasks: unknown resource %q — no live resource→context→connection chain (T-7)", resourceUID)
	}
	return row, nil
}

// resolveConnectionScopedChain — §3.3 conn: 분기: 합성 uid의 커넥션 분절로
// provider_connection을 직접 조회해 ConnectionUID·ProviderType를 조달한다.
// ExternalURN은 공백 — create 대상은 아직 infra_resource 행이 없어 URN이
// 없고, 실행기 leg(J1b)은 req.Connection만 소비한다(arch rule 2). 미등록
// 커넥션은 실 uid 경로의 unknown resource와 같은 하드 에러 계열(재시도 없음).
func (e *Engine) resolveConnectionScopedChain(ctx context.Context, resourceUID string) (executionChain, error) {
	connUID, err := connectionUIDOfSynthetic(resourceUID)
	if err != nil {
		return executionChain{}, err
	}
	var row executionChain
	res := e.db.WithContext(ctx).
		Table("provider_connection").
		Select("uid AS connection_uid, provider_type AS provider_type").
		Where("uid = ?", connUID).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return executionChain{}, fmt.Errorf("tasks: connection-scoped chain lookup for %q: %w", resourceUID, res.Error)
	}
	if row.ConnectionUID == "" {
		return executionChain{}, fmt.Errorf("tasks: unknown connection in resource_uid %q — no provider_connection row (§3.3)", resourceUID)
	}
	return row, nil // ExternalURN은 zero value ""
}

// connectionUIDOfSynthetic — 합성 uid에서 커넥션 분절을 추출한다. 커넥션 uid
// 어휘는 32자 hex(newSyncUID·SourceKeyUID — 콜론 불포함)이라 첫 분절이 곧
// uid다. 분절 수가 3이 아니거나 커넥션 분절이 빈 uid는 포맷 계약 위반의
// 하드 에러다.
func connectionUIDOfSynthetic(resourceUID string) (string, error) {
	segs := strings.Split(strings.TrimPrefix(resourceUID, connUIDPrefix), ":")
	if len(segs) != syntheticUIDSegments || segs[0] == "" {
		return "", fmt.Errorf(
			"tasks: malformed synthetic resource_uid %q — want conn:<connectionUID>:<singular>:<target> (§3.3)",
			resourceUID)
	}
	return segs[0], nil
}

// connectionView assembles the §7.2-derived view the execution path hands to
// adapters, filling Material through the §7.4/§3.5 broker. A connection with
// NO operations binding (the fake / Phase 1 gate path) runs with nil Material
// — "fake는 Config만으로 동작" (§3.5). A binding that exists but fails to
// resolve is a hard error: executing without the configured credential would
// be the silent hole §3.5 closes.
func (e *Engine) connectionView(ctx context.Context, chain executionChain) (contract.ConnectionView, error) {
	var conn model.ProviderConnection
	if err := e.db.WithContext(ctx).Where("uid = ?", chain.ConnectionUID).First(&conn).Error; err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: load connection %q: %w", chain.ConnectionUID, err)
	}
	view := contract.ConnectionView{
		UID:          conn.UID,
		ProviderType: conn.ProviderType,
		Endpoint:     conn.Endpoint,
		Config:       conn.ConfigJSON,
	}
	var bindings int64
	if err := e.db.WithContext(ctx).Model(&model.ProviderCredentialBinding{}).
		Where("provider_connection_id = ? AND purpose = ?", conn.ID, contract.CredentialPurposeOperations).
		Count(&bindings).Error; err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: probe operations bindings for %q: %w", chain.ConnectionUID, err)
	}
	if bindings == 0 {
		return view, nil
	}
	resolved, err := e.broker.Resolve(ctx, conn.UID, contract.CredentialPurposeOperations)
	if err != nil {
		return contract.ConnectionView{}, fmt.Errorf("tasks: %s: %w", ErrorCodeCredentialError, err)
	}
	view.Material = map[string]string{contract.CredentialPurposeOperations: resolved.Value}
	return view, nil
}

// capabilityServed — the adapter choice is a registry lookup only (arch
// rule 1): the connection's provider type must have declared the operation's
// required capability.
func capabilityServed(reg *registry.Registry, providerType, capability string) bool {
	for _, c := range reg.Capabilities(providerType) {
		if c.Name == capability {
			return true
		}
	}
	return false
}
