package contract

import (
	"errors"
	"strings"
	"testing"
)

// PR 20 확장면 단언 — ProviderSignalError(§9.3 신호 1:1)와 어휘 확장 2종(J9).
// 기존 contract_test.go(Phase 0 보존 파일)는 무접촉 — 본 파일은 확장분만 검증한다.
func TestProviderSignalErrorShape(t *testing.T) {
	err := &ProviderSignalError{Kind: SignalRateLimited, Message: "429 with Retry-After"}
	if got := err.Error(); got != "provider signal (rate_limited): 429 with Retry-After" {
		t.Errorf("Error() = %q", got)
	}
	bare := &ProviderSignalError{Kind: SignalUnreachable}
	if got := bare.Error(); got != "provider signal: unreachable" {
		t.Errorf("bare Error() = %q", got)
	}

	// errors.As 경로 — sync runner가 신호를 분기하는 방식(계획 §3.3).
	var sig *ProviderSignalError
	wrapped := errors.Join(errors.New("context"), err)
	if !errors.As(wrapped, &sig) || sig.Kind != SignalRateLimited {
		t.Fatalf("errors.As failed through wrap: %v", wrapped)
	}
	if !strings.Contains(sig.Error(), "429") {
		t.Errorf("message lost: %q", sig.Error())
	}
}

func TestSignalKindVocabularyClosed(t *testing.T) {
	want := []string{"rate_limited", "permission_denied", "unreachable"}
	got := []string{SignalRateLimited, SignalPermissionDenied, SignalUnreachable}
	if len(got) != len(want) {
		t.Fatalf("signal kinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("signal kind[%d] = %q, want %q (§9.3 어휘 1:1)", i, got[i], want[i])
		}
	}
}

func TestPhase2ResourceKindExtensionRecognized(t *testing.T) {
	// M1ResourceKinds는 §8.5 19종 verbatim 보존(T1) — 확장은 별도 슬라이스(J9).
	for _, kind := range Phase2ResourceKindExtensions {
		if !IsKnownResourceKind(kind) {
			t.Errorf("extension kind %q not recognized by IsKnownResourceKind", kind)
		}
		if IsKnownResourceKind("orchestration.unknown") {
			t.Errorf("unknown kind recognized — vocabulary is not closed")
		}
	}
	for _, k := range M1ResourceKinds {
		if k == "orchestration.configmap" || k == "orchestration.secret" {
			t.Errorf("extension kind %q leaked into M1ResourceKinds (§8.5 verbatim 계약 위반)", k)
		}
	}
}
