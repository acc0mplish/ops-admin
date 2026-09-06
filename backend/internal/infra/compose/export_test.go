package compose

// export_test — 테스트 전용 공개면(표준 Go 관용구). registerKubernetes의
// V4 오류 경로(공급자 유형 등록 선행 계약)는 Build의 정상 조립으로는 도달
// 불가 — 등록 순서 계약의 음성 단얫이 직접 호출을 필요로 한다.
var RegisterKubernetesForTest = registerKubernetes
