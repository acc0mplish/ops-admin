// opharness_test — P2-A: 왕복 하네스 자체의 반증 가능성(harness_test.go 선례 —
// "the harness must fail LOUDLY (and correctly) on contract breaches, not just
// pass conforming adapters"). 스텁 executor+fixture로 각 단얫 위반이 잡히는 것을
// 단얫한다.
package contracttest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ops-admin/backend/internal/infra/contract"
)

const harnessSecretMarker = "opharness-canary-material"

// stubExecutor — 검증 결과를 스크립트로 흉내 내는 최소 executor+poller.
// counter는 어댑터가 자신의 provider mutation을 기록하는 표면 — fixture의
// Mutations()가 읽는 바로 그 슬라이스다(harness 계약: 어댑터는 변형 수를
// 노출하고 하네스는 delta를 단얫한다).
type stubExecutor struct {
	handle     string
	execErr    error
	pollScript []contract.OperationStatus
	polls      int
	counter    *int
	// mutateEveryExecute — 결함 재현 플래그: 검증 실패(bad) 요청에도 mutation을
	// 기록한다(단얫 4 위반). false면 최초 1회(nominal)만 기록한다.
	mutateEveryExecute bool
	nominalDone        bool
	// validate — 준수 스텁의 검증 훅(비nil이고 에러를 반환하면 mutation 없이
	// 거부). nil이면 무검증 통과(단얫 4 위반 재현).
	validate func(contract.OperationRequest) error
}

func (s *stubExecutor) Execute(_ context.Context, req contract.OperationRequest) (contract.OperationHandle, error) {
	if s.execErr != nil {
		return contract.OperationHandle{}, s.execErr
	}
	if s.validate != nil {
		if err := s.validate(req); err != nil {
			return contract.OperationHandle{}, err
		}
	}
	if s.counter != nil {
		if s.mutateEveryExecute || !s.nominalDone {
			*s.counter++
			s.nominalDone = true
		}
	}
	return contract.OperationHandle{ProviderRef: s.handle}, nil
}

func (s *stubExecutor) Poll(context.Context, contract.PollRequest) (contract.OperationStatus, error) {
	if len(s.pollScript) == 0 {
		return contract.OperationStatus{}, errors.New("stub: no poll script")
	}
	if s.polls < len(s.pollScript) {
		s.polls++
		return s.pollScript[s.polls-1], nil
	}
	return s.pollScript[len(s.pollScript)-1], nil
}

// stubOpFixture — fixture 값 표.
type stubOpFixture struct {
	request   contract.OperationRequest
	bad       []contract.OperationRequest
	mutations *int
	detail    []string
	marker    string
}

func (f *stubOpFixture) Request() contract.OperationRequest { return f.request }

func (f *stubOpFixture) BadRequests() []contract.OperationRequest { return f.bad }

func (f *stubOpFixture) Mutations() int { return *f.mutations }

func (f *stubOpFixture) AllowedDetailKeys() []string { return f.detail }

func (f *stubOpFixture) SecretMarker() string { return f.marker }

func connWithMaterial() contract.ConnectionView {
	return contract.ConnectionView{
		UID:      "conn-opharness-1",
		Material: map[string]string{contract.CredentialPurposeOperations: harnessSecretMarker},
	}
}

// newConformingFixture — 준수 스텁과 그 fixture. pollScript[0]은 running(폴
// 순회 경로), 이후 Succeeded(detail은 generation 1키 — fixture.detail와 1:1).
func newConformingFixture() (*stubExecutor, *stubOpFixture) {
	mutations := 0
	fx := &stubOpFixture{
		request: contract.OperationRequest{
			OperationName: "stub.op",
			ResourceURN:   "urn:stub:1:thing/a",
			Connection:    connWithMaterial(),
		},
		bad: []contract.OperationRequest{
			{OperationName: "stub.op", ResourceURN: "", Connection: connWithMaterial()},
		},
		mutations: &mutations,
		detail:    []string{"generation"},
		marker:    harnessSecretMarker,
	}
	ex := &stubExecutor{
		handle:  "stub|conn-opharness-1|a|2",
		counter: &mutations,
		validate: func(req contract.OperationRequest) error {
			if req.ResourceURN == "" {
				return errors.New("stub: empty URN")
			}
			return nil
		},
		pollScript: []contract.OperationStatus{
			{State: contract.OperationStateSucceeded, Detail: contract.JSONMap{"generation": 2}},
		},
	}
	return ex, fx
}

// TestOperationRoundtripPassesConformingExecutor — 준수 executor는 0 위반.
func TestOperationRoundtripPassesConformingExecutor(t *testing.T) {
	ex, fx := newConformingFixture()
	if failures := OperationRoundtripFailures(ex, fx); len(failures) != 0 {
		t.Fatalf("conforming executor reported %d failure(s):\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

// TestOperationRoundtripPassesWithRunningRounds — running 2경유 후 수렴해도 0 위반.
func TestOperationRoundtripPassesWithRunningRounds(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.pollScript = []contract.OperationStatus{
		{State: contract.OperationStateRunning},
		{State: contract.OperationStateRunning},
		{State: contract.OperationStateSucceeded, Detail: contract.JSONMap{"generation": 2}},
	}
	if failures := OperationRoundtripFailures(ex, fx); len(failures) != 0 {
		t.Fatalf("conforming executor reported %d failure(s):\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

// TestOperationRoundtripCatchesEmptyHandle — 단얫 1: 빈 ProviderRef.
func TestOperationRoundtripCatchesEmptyHandle(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.handle = ""
	assertFailure(t, OperationRoundtripFailures(ex, fx), "empty ProviderRef")
}

// TestOperationRoundtripCatchesNoMutation — 단얫 1: mutation 없는 Execute.
func TestOperationRoundtripCatchesNoMutation(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.counter = nil
	assertFailure(t, OperationRoundtripFailures(ex, fx), "no provider mutation")
}

// TestOperationRoundtripCatchesExecError — 단얫 1: nominal 요청 실패는 즉시
// 보고(후속 단얫 생략 — 공허 green 봉쇄).
func TestOperationRoundtripCatchesExecError(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.execErr = errors.New("stub: rejected")
	assertFailure(t, OperationRoundtripFailures(ex, fx), "nominal request failed")
}

// TestOperationRoundtripCatchesNonConvergence — 단얫 2: 상한 내 미수렴.
func TestOperationRoundtripCatchesNonConvergence(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.pollScript = []contract.OperationStatus{{State: contract.OperationStateRunning}}
	assertFailure(t, OperationRoundtripFailures(ex, fx), "did not converge")
}

// TestOperationRoundtripCatchesRunningDetail — 단얫 2: running 상태의 detail.
func TestOperationRoundtripCatchesRunningDetail(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.pollScript = []contract.OperationStatus{
		{State: contract.OperationStateRunning, Detail: contract.JSONMap{"early": "leak"}},
		{State: contract.OperationStateSucceeded, Detail: contract.JSONMap{"generation": 2}},
	}
	assertFailure(t, OperationRoundtripFailures(ex, fx), "detail is terminal-only")
}

// TestOperationRoundtripCatchesDetailKeyMismatch — 단얫 3: 허용 필드 밖 키.
func TestOperationRoundtripCatchesDetailKeyMismatch(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.pollScript = []contract.OperationStatus{
		{State: contract.OperationStateSucceeded, Detail: contract.JSONMap{"generation": 2, "rawSecret": "x"}},
	}
	assertFailure(t, OperationRoundtripFailures(ex, fx), "§10.2 allowlist 1:1")
}

// TestOperationRoundtripCatchesSilentValidation — 단얫 4: 검증 실패가 에러 없이
// 통과한다(검증 훅이 없는 스텁은 bad 요청도 성공시킨다).
func TestOperationRoundtripCatchesSilentValidation(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.validate = nil
	assertFailure(t, OperationRoundtripFailures(ex, fx), "want a validation error")
}

// TestOperationRoundtripCatchesValidationSideEffect — 단얫 4: 검증 실패가
// provider를 변형했다(무검증 + 전 Execute mutation 기록의 결함 조합).
func TestOperationRoundtripCatchesValidationSideEffect(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.validate = nil
	ex.mutateEveryExecute = true
	assertFailure(t, OperationRoundtripFailures(ex, fx), "issued 1 provider mutation")
}

// TestOperationRoundtripCatchesMaterialLeak — 단얫 5: 성공 detail에 자격 물질.
func TestOperationRoundtripCatchesMaterialLeak(t *testing.T) {
	ex, fx := newConformingFixture()
	ex.pollScript = []contract.OperationStatus{
		{State: contract.OperationStateSucceeded, Detail: contract.JSONMap{"generation": 2, "note": harnessSecretMarker}},
	}
	assertFailure(t, OperationRoundtripFailures(ex, fx), "credential material")
}

func assertFailure(t *testing.T, failures []string, want string) {
	t.Helper()
	for _, f := range failures {
		if strings.Contains(f, want) {
			return
		}
	}
	t.Fatalf("failures = %v, want one containing %q", failures, want)
}
