// k8s_detail_test.go — Phase Z3 characterization (k8s_detail 시맨 순수 함수 1개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"testing"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 빈 값은 숫자 fallback, 숫자 문자열은 int, 그 외는 문자열 보존이 현재 계약.
func TestCharServiceTargetPort(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		fallback int
		want     any
	}{
		{name: "빈 값 → fallback", value: "", fallback: 80, want: 80},
		{name: "공백 → fallback", value: "   ", fallback: 443, want: 443},
		{name: "숫자 문자열 → int", value: "8080", fallback: 80, want: 8080},
		{name: "선행 0 숫자 → int", value: "080", fallback: 80, want: 80},
		{name: "음수 → int", value: "-1", fallback: 80, want: -1},
		{name: "이름 → 문자열 보존", value: "web", fallback: 80, want: "web"},
	}
	for _, tc := range cases {
		if got := serviceTargetPort(tc.value, tc.fallback); got != tc.want { // legacy 1행
			t.Errorf("%s: = %v(%T), want %v", tc.name, got, got, tc.want)
		}
	}
}
