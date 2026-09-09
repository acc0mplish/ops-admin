// k8s_metrics_test.go — Phase Z3 characterization (k8s_metrics 시맨 순수 함수 4개).
// 목적은 "옳음"이 아니라 "불변": 현재 동작을 그대로 기록해 B~D2 파일 분해의 유일한
// 검출기이자 P의 V2 동치 오라클이 되게 한다 (계획 §J0·§12 #16, R18 — legacy 호출 1행).
package service

import (
	"reflect"
	"testing"
	"time"
)

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 파드명 정렬, 타임스탬프 밀리초 변환, 빈 포인트 샘플 제외, nil → 빈 슬라이스가 현재 계약.
func TestCharK8sPodMetricSeries(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    []map[string]any
	}{
		{
			name:    "정렬·변환·최신값",
			payload: `{"data":{"result":[{"metric":{"pod":"b-pod"},"values":[[1600000000,"1.5"],[1600000060,"2"]]},{"metric":{"pod":"a-pod"},"values":[[1600000000,0.5]]}]}}`,
			want: []map[string]any{
				{"name": "a-pod", "latest": 0.5, "points": []map[string]any{{"timestamp": int64(1600000000000), "value": 0.5}}},
				{"name": "b-pod", "latest": 2.0, "points": []map[string]any{
					{"timestamp": int64(1600000000000), "value": 1.5},
					{"timestamp": int64(1600000060000), "value": 2.0}}},
			},
		},
		{name: "빈 포인트 샘플 제외", payload: `{"data":{"result":[{"metric":{"pod":"x"},"values":[[100]]}]}}`, want: []map[string]any{}},
		{name: "nil 결과 → 빈 슬라이스", payload: "", want: []map[string]any{}},
	}
	for _, tc := range cases {
		var result *PromQueryResult
		if tc.payload != "" {
			result = &PromQueryResult{}
			charDecode(t, tc.payload, result)
		}
		if series := k8sPodMetricSeries(result); !reflect.DeepEqual(series, tc.want) { // legacy 1행
			t.Errorf("%s: series = %+v, want %+v", tc.name, series, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 미지원 키는 1h로 수렴(key 재할당), span은 정의 duration과 정확히 일치가 현재 계약.
func TestCharResolveK8sPodMetricRange(t *testing.T) {
	cases := []struct {
		rangeKey string
		wantStep int
		wantKey  string
		wantSpan time.Duration
	}{
		{rangeKey: "1h", wantStep: 30, wantKey: "1h", wantSpan: time.Hour},
		{rangeKey: "6h", wantStep: 120, wantKey: "6h", wantSpan: 6 * time.Hour},
		{rangeKey: "24h", wantStep: 300, wantKey: "24h", wantSpan: 24 * time.Hour},
		{rangeKey: "7d", wantStep: 30, wantKey: "1h", wantSpan: time.Hour},
	}
	for _, tc := range cases {
		start, end, step, key := resolveK8sPodMetricRange(tc.rangeKey) // legacy 1행
		if end.Sub(start) != tc.wantSpan || step != tc.wantStep || key != tc.wantKey {
			t.Errorf("%q: = (span %v, step %d, key %q), want (%v, %d, %q)", tc.rangeKey, end.Sub(start), step, key, tc.wantSpan, tc.wantStep, tc.wantKey)
		}
		if end.After(time.Now().Add(2 * time.Second)) {
			t.Errorf("%q: end가 현재 시각보다 미래 = %v", tc.rangeKey, end)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
// 샘플 간 동일 타임스탬프는 값 합산, 밀리초 정렬, 짧은 쌍 무시가 현재 계약.
func TestCharK8sPodMetricPoints(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    []map[string]any
	}{
		{
			name:    "타임스탬프 합산·정렬",
			payload: `{"data":{"result":[{"metric":{"pod":"a"},"values":[[100,"1"],[200,"2"]]},{"metric":{"pod":"b"},"values":[[100,"0.5"]]},{"metric":{"pod":"short"},"values":[[300]]}]}}`,
			want: []map[string]any{
				{"timestamp": int64(100000), "value": 1.5},
				{"timestamp": int64(200000), "value": 2.0}},
		},
		{name: "빈 결과 → 빈 슬라이스", payload: "", want: []map[string]any{}},
	}
	for _, tc := range cases {
		var result *PromQueryResult
		if tc.payload != "" {
			result = &PromQueryResult{}
			charDecode(t, tc.payload, result)
		}
		if got := k8sPodMetricPoints(result); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: points = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.
func TestCharK8sPodMetricLatest(t *testing.T) {
	points := []map[string]any{
		{"timestamp": int64(100000), "value": 1.5},
		{"timestamp": int64(200000), "value": 2.0},
	}
	cases := []struct {
		name   string
		points []map[string]any
		want   any
	}{
		{name: "마지막 값", points: points, want: 2.0},
		{name: "빈 입력 → nil", points: nil, want: nil},
	}
	for _, tc := range cases {
		if got := k8sPodMetricLatest(tc.points); !reflect.DeepEqual(got, tc.want) { // legacy 1행
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}
