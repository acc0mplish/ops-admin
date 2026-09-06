// N4 (plan §1 J5, §6) — 정책 엔진 계약: §10.3 policy_input 어휘(Tenant 부재 —
// r2), builtin:default-allow 판정(전부 허용 + 판정 근거 + 버전 문자열),
// 미조립 입력 = 평가 불가 에러(fail-closed의 호출자 계약, VK-4).
package policy

import (
	"strings"
	"testing"
)

func validInput() PolicyInput {
	return PolicyInput{
		ProviderType: "kubernetes",
		ContextKind:  "cluster",
		ResourceKind: "orchestration.workload",
		Risk:         "medium",
		Mutating:     true,
	}
}

// builtin:default-allow — M1 규칙집합은 전부 허용. 판정은 버전 문자열과
// 근거를 동반한다(감사 policy_version의 정준원천 — A4).
func TestEvaluateAllowsAllUnderBuiltinDefaultAllow(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   PolicyInput
	}{
		{"mutating medium risk", validInput()},
		{"read-only low risk", func() PolicyInput {
			in := validInput()
			in.Mutating = false
			in.Risk = "low"
			return in
		}()},
		{"other provider high risk", func() PolicyInput {
			in := validInput()
			in.ProviderType = "fake"
			in.ContextKind = "region"
			in.Risk = "high"
			return in
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := Evaluate(tc.in)
			if err != nil {
				t.Fatalf("Evaluate: %v — builtin:default-allow permits every assembled input", err)
			}
			if !dec.Allow {
				t.Error("Allow = false, want true (M1 ruleset allows all)")
			}
			if dec.PolicyVersion != BuiltinDefaultAllow {
				t.Errorf("PolicyVersion = %q, want %q", dec.PolicyVersion, BuiltinDefaultAllow)
			}
			if strings.TrimSpace(dec.Reason) == "" {
				t.Error("Reason is empty — every decision carries its rationale (J5)")
			}
		})
	}
}

// 미조립 입력은 deny가 아니라 평가 불가다 — Evaluate는 에러를 내고 호출자는
// 403으로 거부한다(VK-4 fail-closed). 최소 허용 엔진에서 "평가 불가=통과"는
// 정책 계층의 부재를 무효화한다.
func TestEvaluateRejectsUnassembledInput(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PolicyInput)
		field  string
	}{
		{"empty provider type", func(in *PolicyInput) { in.ProviderType = "" }, "provider_type"},
		{"empty context kind", func(in *PolicyInput) { in.ContextKind = "" }, "context_kind"},
		{"empty resource kind", func(in *PolicyInput) { in.ResourceKind = "" }, "resource_kind"},
		{"empty risk", func(in *PolicyInput) { in.Risk = "" }, "risk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput()
			tc.mutate(&in)
			dec, err := Evaluate(in)
			if err == nil {
				t.Fatal("Evaluate succeeded on an unassembled input, want an evaluation error (VK-4)")
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error = %v, want it to name the missing field %q", err, tc.field)
			}
			if dec.Allow || dec.PolicyVersion != "" {
				t.Errorf("decision = %+v on error, want the zero decision — no usable verdict may leak", dec)
			}
		})
	}
}
