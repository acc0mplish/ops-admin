// Package tasks — engine_resolve_test.go (I10 J1a)는 §3.3 conn: 분기의
// 단얫을 소유한다: 합성 uid → 커넥션 직조립·실 uid → 기존 infra_resource
// 조인 2경로, 합성 uid 포맷(접두·콜론 분절 3개·namespaced/cluster-scope
// 2형태), 합성 uid의 목표 단위 직렬화(RESOURCE_BUSY), 그리고
// infra_resource.uid 전수가 conn: 비접두임을 가드하는 예약 단얫(위험7 —
// 접두 판별의 전제).
package tasks

import (
	"context"
	"errors"
	"testing"

	"ops-admin/backend/internal/infra/model"
)

// seedBareConnection — 커넥션-스코프 검증 fixture: infra_resource 행 없이
// 커넥션만 심는다. create 대상 리소스는 제출 시점에 아직 존재하지 않는다
// (§3.3 — 스코프 앵커가 리소스 uid에서 커넥션 uid+매니페스트 신원으로 이동).
func seedBareConnection(t *testing.T, f *engineFixture, uid, providerType string) {
	t.Helper()
	conn := model.ProviderConnection{
		UID:          uid,
		ProviderType: providerType,
		Name:         "conn-" + providerType,
		Endpoint:     "https://" + providerType + ".invalid",
	}
	if err := f.db.Create(&conn).Error; err != nil {
		t.Fatalf("seed bare connection: %v", err)
	}
}

// §3.3 J1a — 2경로 단얫: conn: 접두 합성 uid는 infra_resource 조인을
// 건너뛰고 provider_connection을 직조립한다(ConnectionUID·ProviderType 조달,
// ExternalURN="" — create 대상엔 URN이 없다). 실 uid는 기존 T-7 조인을
// 그대로 밟는다(바이트 불변 승계 — ExternalURN까지 조달).
func TestResolveExecutionChainConnectionScoped(t *testing.T) {
	f := newEngineFixture(t, testConfig())
	ctx := context.Background()
	seedChain(t, f.db, "fake", "res-uid-1") // 실 uid 경로용 체인
	seedBareConnection(t, f, "c0ffee00", "kubernetes")

	namespaced := "conn:c0ffee00:namespace:team-a/web" // namespaced형 — target <ns>/<name>
	chain, err := f.eng.resolveExecutionChain(ctx, namespaced)
	if err != nil {
		t.Fatalf("conn-scoped resolve(%q): %v", namespaced, err)
	}
	if chain.ConnectionUID != "c0ffee00" {
		t.Errorf("namespaced ConnectionUID = %q, want %q", chain.ConnectionUID, "c0ffee00")
	}
	if chain.ProviderType != "kubernetes" {
		t.Errorf("namespaced ProviderType = %q, want %q", chain.ProviderType, "kubernetes")
	}
	if chain.ExternalURN != "" {
		t.Errorf("namespaced ExternalURN = %q, want \"\" — create 대상은 infra_resource 행이 없다", chain.ExternalURN)
	}

	clusterScoped := "conn:c0ffee00:namespace:the-namespace" // cluster-scope형 — target <name>
	chain, err = f.eng.resolveExecutionChain(ctx, clusterScoped)
	if err != nil {
		t.Fatalf("conn-scoped resolve(%q): %v", clusterScoped, err)
	}
	if chain.ConnectionUID != "c0ffee00" || chain.ProviderType != "kubernetes" || chain.ExternalURN != "" {
		t.Errorf("cluster-scope chain = %+v, want {c0ffee00 kubernetes \"\"}", chain)
	}

	// 실 uid → 기존 infra_resource→context→connection 조인(T-7 승계).
	chain, err = f.eng.resolveExecutionChain(ctx, "res-uid-1")
	if err != nil {
		t.Fatalf("real-uid resolve: %v", err)
	}
	if chain.ConnectionUID != "conn-fake" || chain.ProviderType != "fake" {
		t.Errorf("real-uid chain = %+v, want connection conn-fake/fake", chain)
	}
	if chain.ExternalURN != "urn:fake:workload:res-uid-1" {
		t.Errorf("real-uid ExternalURN = %q, want the infra_resource URN (T-7)", chain.ExternalURN)
	}
}

// §3.3 — 커넥션-스코프 경로의 하드 에러 2계열: 미등록 커넥션(unknown
// resource 계열 — 재시도 없음)과 포맷 계약 위반(분절 3개 아님·빈 커넥션
// 분절·분절 초과). k8s 이름/네임스페이스 어휘에 콜론이 없어 4분절 이상은
// 곧 잘못된 조립이다.
func TestSyntheticUIDResolutionErrors(t *testing.T) {
	f := newEngineFixture(t, testConfig())
	seedBareConnection(t, f, "c0ffee00", "kubernetes")
	ctx := context.Background()

	cases := []string{
		"conn:deadbeef:namespace:team-a/web",   // 미등록 커넥션
		"conn:c0ffee00",                        // 분절 1 — 커넥션만 있음
		"conn:c0ffee00:namespace",              // 분절 2 — target 누락
		"conn::namespace:team-a/web",           // 빈 커넥션 분절
		"conn:c0ffee00:namespace:ns/web:extra", // 분절 4 — 초과
	}
	for _, uid := range cases {
		if _, err := f.eng.resolveExecutionChain(ctx, uid); err == nil {
			t.Errorf("resolveExecutionChain(%q) err = nil, want hard error", uid)
		}
	}
}

// r3 LOW①(I10 J1c) — 빈 singular/target 분절도 거부한다: 3분절이라도
// singular·target이 공백이면 목표 신원이 성립하지 않고, 목표 단위 직렬화
// ((resource_uid, active_flag) 유니크)를 무의미하게 만든다. 본 단얫은
// connectionUIDOfSynthetic 자신을 직접 검증한다 — 가드와 같은 패키지의
// 유닛테스트라서 가능한 소재다(v2 측 왕복 단얫은
// operations_connection_test.go가 소유).
func TestConnectionUIDOfSyntheticRejectsEmptySegments(t *testing.T) {
	for _, uid := range []string{
		"conn:c0ffee00::team-a/web", // 빈 singular
		"conn:c0ffee00:namespace:",  // 빈 target
		"conn:c0ffee00::",           // 둘 다 공백
	} {
		if _, err := connectionUIDOfSynthetic(uid); err == nil {
			t.Errorf("connectionUIDOfSynthetic(%q) err = nil, want the malformed-uid hard error (r3 LOW①)", uid)
		}
	}
}

// §3.3 — 합성 uid는 (resource_uid, active_flag) 유니크의 목표 신원 단위로
// 직렬화한다: 동일 목표 2번째 활성 제출은 ErrResourceBusy(N11 fast-fail),
// 상이 목표(다른 이름)는 독립 제출된다. 실제 스키마(step0003 가드) 위에서
// 단얫한다 — bare AutoMigrate엔 이 유니크가 없다(guards_test 참조).
func TestSyntheticUIDSerializesTarget(t *testing.T) {
	f := newGuardedFixture(t, false)
	ctx := context.Background()

	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "conn:c0ffee00:namespace:team-a/web",
	}); err != nil {
		t.Fatalf("first synthetic submit: %v", err)
	}
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "conn:c0ffee00:namespace:team-a/web",
	}); !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("second submit on the same target err = %v, want ErrResourceBusy (N11)", err)
	}
	if _, _, err := f.eng.Submit(ctx, SubmitInput{
		OperationName: "fake.workload.restart",
		ResourceUID:   "conn:c0ffee00:namespace:team-a/other",
	}); err != nil {
		t.Fatalf("distinct-target submit must be independent: %v", err)
	}
}

// 예약 단얫(위험7 채택) — infra_resource.uid 전수가 conn: 비접두임을
// 가드한다. conn: 접두가 합성 uid 판별의 유일한 근거이므로, 실 리소스 uid가
// 이 접두를 가져오면 판별이 오염된다. 커넥션·리소스 uid 어휘(32자 hex
// newSyncUID·SourceKeyUID·seed 계열)는 콜론을 쓰지 않는다 — 이 단얫이 그
// 계약을 잠근다(행이 있는 한 전수 검사).
func TestInfraResourceUIDsNeverUseConnPrefix(t *testing.T) {
	f := newEngineFixture(t, testConfig())
	seedChain(t, f.db, "fake", "res-uid-1")
	seedBareConnection(t, f, "c0ffee00", "kubernetes")

	var seeded int64
	if err := f.db.Model(&model.InfraResource{}).Count(&seeded).Error; err != nil {
		t.Fatalf("count infra_resource: %v", err)
	}
	if seeded == 0 {
		t.Fatal("fixture sanity: no infra_resource rows to guard")
	}
	var polluted int64
	if err := f.db.Model(&model.InfraResource{}).
		Where("uid LIKE ?", "conn:%").Count(&polluted).Error; err != nil {
		t.Fatalf("guard count: %v", err)
	}
	if polluted != 0 {
		t.Fatalf("infra_resource rows with conn: prefix = %d, want 0 — the synthetic-uid discriminator is reserved", polluted)
	}
}
