// opharness.go — P2-A: 오퍼레이션 왕복 하네스(계획 r3 §J-P1-6·§6 P2-A —
// "오퍼레이션 왕복 하네스(adapter 비의존 seam)"). harness.go가 §23.1 디스커버리
// 계약 4단얫을 운영하는 것과 같은 형상으로, Execute→Poll의 provider 왕복을
// 운영한다. Z 동치가 아닌 **쓰기 경험식의 대체 판정**(§J-P1-5 — replaceImageVersion
// 계열은 Z 원장에서 제외되고 본 왕복이 동치를 판정한다)이 이 하네스의 존재 이유다.
//
// opharness imports contract only — never an adapter package. Adapter test
// files (kubernetes, proxmox, …) call RunOperationRoundtrip with their own
// OperationFixture implementation.
package contracttest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"ops-admin/backend/internal/infra/contract"
)

// pollRoundsBound — 왕복 단얫 2의 수렴 상한. rollout 수렴 폴은 엔진이 매 폴
// 재호출하는 구조라 하네스도 유한 상한 내 순회로 종결을 판정한다(무한 폴 방지).
const pollRoundsBound = 8

// OperationFixture is the operation roundtrip seam (harness.go의 Fixture와
// 대칭). An adapter test supplies it; the harness drives the
// executor+poller through it.
type OperationFixture interface {
	// Request is the nominal valid request the harness executes and polls to
	// convergence.
	Request() contract.OperationRequest
	// BadRequests are validation negatives: each must fail Execute with a
	// non-empty error AND leave the provider untouched (see Mutations).
	BadRequests() []contract.OperationRequest
	// Mutations is the provider side-effect counter — the harness asserts a
	// validation failure never mutates. Reads (GET) are not side effects;
	// adapters count mutating requests (PATCH/POST/DELETE) only.
	Mutations() int
	// AllowedDetailKeys is the §10.2 redaction allowlist: a Succeeded detail
	// must carry exactly this key set (1:1 — restart 선례 compose_test의
	// reflection 단얫과 동일 계약).
	AllowedDetailKeys() []string
	// SecretMarker is a credential-material value the fixture plants on the
	// connection; it must never surface in handle·detail·error strings
	// (보존 제약 #7의 실행 경로 절반 — harness.go 단얫 2의 Poll 대응). ""
	// skips the scan.
	SecretMarker() string
}

// executorPoller — 본 하네스가 구동하는 최소 면. contract.OperationExecutor·
// contract.TaskPoller의 조합 별칭이 아니라 두 인터페이스를 모두 만족하는 값의
// 이형명 — registry V5가 만족을 강제하는 바로 그 쌍이다.
type executorPoller interface {
	contract.OperationExecutor
	contract.TaskPoller
}

// RunOperationRoundtrip runs the operation roundtrip assertions against ex,
// driven by fx. Every violation is reported on t — the suite NEVER passes
// silently.
func RunOperationRoundtrip(t interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Helper()
}, ex executorPoller, fx OperationFixture) {
	t.Helper()
	for _, f := range OperationRoundtripFailures(ex, fx) {
		t.Errorf("%s", f)
	}
}

// OperationRoundtripFailures executes the harness and returns one
// human-readable entry per contract violation (empty = conforming). Exposed
// separately from RunOperationRoundtrip so the harness's own falsifiability
// tests can assert that breaches are CAUGHT (harness_test.go 선례).
func OperationRoundtripFailures(ex executorPoller, fx OperationFixture) []string {
	var failures []string
	fail := func(format string, args ...any) {
		failures = append(failures, fmt.Sprintf(format, args...))
	}

	ctx := context.Background()

	// --- 단얫 1: Execute — 검증 통과 요청은 비어 있지 않은 자기서술 handle을
	// 반환한다(J2 — ProviderRef는 task_attempt.handle_ref에 지속된다). ---
	baseline := fx.Mutations()
	handle, err := ex.Execute(ctx, fx.Request())
	if err != nil {
		fail("assertion 1 (execute): nominal request failed: %v", err)
		return failures
	}
	if strings.TrimSpace(handle.ProviderRef) == "" {
		fail("assertion 1 (execute): nominal request returned an empty ProviderRef — handles persist to task_attempt.handle_ref and must self-describe (J2)")
	}
	if fx.Mutations() <= baseline {
		fail("assertion 1 (execute): nominal Execute issued no provider mutation (counter %d → %d)", baseline, fx.Mutations())
	}

	// --- 단얫 2: Poll — Execute가 발행한 handle은 폴 순회 내 수렴하며, 실행
	// 중 상태는 detail을 싣지 않는다(restart 선례 — detail is terminal-only). ---
	var terminal *contract.OperationStatus
	seen := ""
	for round := 1; round <= pollRoundsBound; round++ {
		status, pollErr := ex.Poll(ctx, contract.PollRequest{Handle: handle, Connection: fx.Request().Connection})
		if pollErr != nil {
			// 자격 부재 등 하네스 조립 결함은 조기 실패 — 폴 자격은 fixture가
			// Request().Connection과 동일하게 조립해야 한다(J12 대칭).
			fail("assertion 2 (poll): round %d failed: %v", round, pollErr)
			return failures
		}
		if status.State == "" {
			fail("assertion 2 (poll): round %d carried an empty state", round)
			return failures
		}
		seen = status.State
		if seen != contract.OperationStateRunning {
			terminal = &status
			break
		}
		if status.Detail != nil {
			fail("assertion 2 (poll): running round %d carried detail %v (detail is terminal-only)", round, status.Detail)
		}
	}
	if terminal == nil {
		fail("assertion 2 (poll): did not converge within %d rounds (last state %q)", pollRoundsBound, seen)
		return failures
	}
	if terminal.State != contract.OperationStateSucceeded {
		fail("assertion 2 (poll): terminal state %q, want succeeded", terminal.State)
	}

	// --- 단얫 3: Succeeded detail 키 집합 == 허용 필드 1:1(§10.2 typed
	// redaction — 성공 detail은 opdef Redaction 스펙 밖의 키를 흘리지 않는다). ---
	if terminal.State == contract.OperationStateSucceeded {
		got := sortedKeys(terminal.Detail)
		want := append([]string(nil), fx.AllowedDetailKeys()...)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			fail("assertion 3 (redaction): succeeded detail keys = %v, want exactly %v (§10.2 allowlist 1:1)", got, want)
		}
		scanForMarker(fx.SecretMarker(), "succeeded detail", jsonMustEncode(&failures, terminal.Detail), &failures)
	}

	// --- 단얫 4: 검증 실패는 provider를 건드리지 않는다 — 전 왕복 0회
	// (executor_test TestExecuteValidationRejectsWithoutSideEffects의
	// 하네스 일반형). ---
	for i, bad := range fx.BadRequests() {
		before := fx.Mutations()
		if _, badErr := ex.Execute(ctx, bad); badErr == nil {
			fail("assertion 4 (validation): bad request #%d (%s op %q) succeeded, want a validation error", i, describeURN(bad.ResourceURN), bad.OperationName)
		}
		if after := fx.Mutations(); after != before {
			fail("assertion 4 (validation): bad request #%d issued %d provider mutation(s), want 0", i, after-before)
		}
	}

	// --- 단얫 5: handle·에러 문자열에 자격 물질 미출현(보존 제약 #7). ---
	scanForMarker(fx.SecretMarker(), "handle ProviderRef", handle.ProviderRef, &failures)
	_, leakErr := ex.Execute(ctx, fx.Request())
	scanForMarker(fx.SecretMarker(), "re-execute error path", describeErr(leakErr), &failures)

	return failures
}

// scanForMarker appends a failure when the planted secret material appears in
// an observable surface. Empty marker = fixture opted out.
func scanForMarker(marker, where, observed string, failures *[]string) {
	if marker == "" {
		return
	}
	if strings.Contains(observed, marker) {
		*failures = append(*failures, fmt.Sprintf(
			"assertion 5 (no leak): %s carries credential material (marker %q):\n%s", where, marker, observed))
	}
}

func jsonMustEncode(failures *[]string, v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		*failures = append(*failures, "assertion 5 (no leak): detail is not JSON-encodable: %v"+err.Error())
		return ""
	}
	return string(raw)
}

func describeErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func describeURN(urn string) string {
	if urn == "" {
		return "<empty URN>"
	}
	return urn
}

// --- 작은 정렬·비교 헬퍼 — contracttest stdlib 순수성(harness.go 선례). ---

func sortedKeys(m contract.JSONMap) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

