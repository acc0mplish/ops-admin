package proxmox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

// ResolveIdentity(④리뷰 LOW — 등록 probe의 어댑터 조립 재사용) 단위 테스트.
// register-pve가 기존까지 자체 HTTP 조립으로 하던 /cluster/status 신원 해독이
// 런타임 클라이언트와 같은 전송 경로(do→classify)를 타는 것을 판정 J8 mock으로
// 핀다. 신원 해석 규칙(§3.3·A11)은 Health의 보고 폴백과 다른 등록 계약 —
// unresolvable은 에러여야 한다.

// identityMock — /cluster/status 상태 코드와 data 배열 원문을 주입하는 최소
// API2 표면. 요청 수를 세어 신원 조회가 정확히 1회 단발임을 단얫한다.
type identityMock struct {
	status int
	data   string
	hits   int
}

func (m *identityMock) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.hits++
		if !strings.HasSuffix(r.URL.Path, "/api2/json/cluster/status") {
			http.Error(w, `{"data":null}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(m.status)
		_, _ = w.Write([]byte(`{"data":` + m.data + `}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func identityConnection(endpoint string) contract.ConnectionView {
	return contract.ConnectionView{
		ProviderType: "proxmox",
		Endpoint:     endpoint,
		Material:     map[string]string{"inventory": testCredentialMaterial},
	}
}

func TestAdapterResolveIdentityClusterRow(t *testing.T) {
	mock := &identityMock{status: http.StatusOK, data: `[{"type":"cluster","name":"pve-cluster","quorate":1},` +
		`{"type":"node","name":"pve1"},{"type":"node","name":"pve2"}]`}
	srv := mock.serve(t)

	identity, err := NewAdapter().ResolveIdentity(context.Background(), identityConnection(srv.URL))
	if err != nil {
		t.Fatalf("ResolveIdentity: %v", err)
	}
	if identity.Name != "pve-cluster" || identity.Standalone || identity.Nodes != 2 || !identity.Quorate {
		t.Fatalf("cluster identity: %+v", identity)
	}
	if mock.hits != 1 {
		t.Fatalf("/cluster/status hits = %d, want exactly 1 (단발 probe — 페일오버 재조회 없음)", mock.hits)
	}
}

func TestAdapterResolveIdentityStandaloneFallback(t *testing.T) {
	// A11 실측 형상: cluster행 없이 node행 1개.
	mock := &identityMock{status: http.StatusOK, data: `[{"type":"node","name":"pve-solo"}]`}
	srv := mock.serve(t)

	identity, err := NewAdapter().ResolveIdentity(context.Background(), identityConnection(srv.URL))
	if err != nil {
		t.Fatalf("ResolveIdentity: %v", err)
	}
	if identity.Name != "pve-solo" || !identity.Standalone || identity.Nodes != 1 || identity.Quorate {
		t.Fatalf("standalone identity (A11): %+v", identity)
	}
}

func TestAdapterResolveIdentityUnresolvableIsError(t *testing.T) {
	// cluster행 부재 + node 2개 — 가짜 클러스터 신원 대신 에러(등록 계약).
	mock := &identityMock{status: http.StatusOK, data: `[{"type":"node","name":"pve1"},{"type":"node","name":"pve2"}]`}
	srv := mock.serve(t)

	_, err := NewAdapter().ResolveIdentity(context.Background(), identityConnection(srv.URL))
	if err == nil || !strings.Contains(err.Error(), "cannot resolve a cluster identity") {
		t.Fatalf("unresolvable /cluster/status must error, got %v", err)
	}
}

func TestAdapterResolveIdentityClassifiesSignal(t *testing.T) {
	// 실패 분류는 런타임 classify와 동일한 §9.3 신호 어휘 — 자체 HTTP 조립이던
	// 이전 probe의 평문 에러와 다른, 어댑터 조립 재사용의 관측 증거.
	for _, tc := range []struct {
		status int
		kind   string
	}{
		{http.StatusForbidden, contract.SignalPermissionDenied},
		{http.StatusServiceUnavailable, contract.SignalUnreachable},
	} {
		mock := &identityMock{status: tc.status, data: `null`}
		srv := mock.serve(t)
		_, err := NewAdapter().ResolveIdentity(context.Background(), identityConnection(srv.URL))
		var signal *contract.ProviderSignalError
		if !errors.As(err, &signal) {
			t.Fatalf("status %d: err = %v, want a ProviderSignalError", tc.status, err)
		}
		if signal.Kind != tc.kind {
			t.Fatalf("status %d: kind = %s, want %s", tc.status, signal.Kind, tc.kind)
		}
	}
}

func TestAdapterResolveIdentityRejectsBadMaterialBeforeRequest(t *testing.T) {
	// 자격 재질 결함은 와이어 요청 없이 조립 단계에서 실패한다(buildClient —
	// Validate·런타임과 동일한 조립 검증 순서).
	conn := contract.ConnectionView{Endpoint: "https://127.0.0.1:8006"}
	if _, err := NewAdapter().ResolveIdentity(context.Background(), conn); err == nil ||
		!strings.Contains(err.Error(), "missing credential material") {
		t.Fatalf("missing material must fail at assembly, got %v", err)
	}
}
