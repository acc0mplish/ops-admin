# I10 계획 r1 적대검토 병합 — 5렌즈 (2026-09-11)

대상: `v2-phase1/i10-plan.md` r1 (370행) · 판정: **전 렌즈 조건부 승인 — 발견 일괄 반영 후 r2 확정** (반려 0)

## 렌즈 × 심각도 × 발견수

| 렌즈 | 판정 | CRIT | HIGH | MED | LOW |
|---|---|---|---|---|---|
| 1 완전성 | 조건부 | 0 | 2 | 3 | 4 |
| 2 기술적 오류 | 조건부 | 0 | 4 | 2 | 4 |
| 3 위험 | 조건부 | 0 | 2 | 5 | 2 |
| 4 단순성 | 조건부 | 0 | 1 | 1 | 3 |
| 5 검증성 | 조건부 | 1 | 2 | 6 | 5 |
| **계** | | **1** | **11** | **17** | **18** |

**교차확인(다중 렌즈 독립 발견)** — 반영 우선순위 최상:
- `*WithPreferred` "사용처 소멸" 허위 — **3렌즈**(완전성 H1·기술 F6·검증 V-6): 래퍼(k8s_transport.go:213/:238)가 라이브 읽기(k8sGetIstioJSON 등)에서 생존 호출. 직접 호출(k8s_path.go:157/167/172)만 사멸. 문언대로 J3 실행 시 빌드 붕괴. TestChar*WithPreferred 2종(:438·:467) 처분도 J3 범위 밖.
- operations.go 소유 모순 — **2렌즈**(기술 F2·단순 F1): §3.2.2는 ListResourceOperations 배제 편집 요구하나 §5 #4는 operations.go 수정 명시 금지·어느 Phase도 미소유. 단순 대안: **빈 ResourceKinds 등록**(registry.go:247 no-op·:341 교집합 미매치로 자동 누출 방지 — ConnectionScoped 필드·operations.go 편집 불요) vs 필드 유지 시 operations.go 소유 지정+§5 #4 예외. r2가 인코딩 확정.
- char-baseline/TestChar 감소 계약 — **2렌즈**(완전성 H2·검증 V-7): char-baseline.txt(120행) 재생성 claim·기대 행수 전무. CI6 기대 수치 명기 필요(기저 k8s_path_test.go TestChar 25).
- J2 동작 검증 공백 — **2렌즈**(위험 F6·검증 V-9): e2e create 커버 0(slice-a 2테스트)·grep+build만으로 2,797행 뷰 4콜사이트 재작성 판정.

## 발견 전체 목록 (r2 반영 의무분 위주 — 전문은 각 렌즈 보고)

### HIGH (11)
1. **[완전성1]** §3.7 WithPreferred 허위 — 교차확인 참조. → "직접 호출 사멸·함수 잔존" 정정 + transport_test 2종 J3 편입
2. **[완전성2]** char-baseline 재생성 계약 공백 — 재생성·커밋 의무 + 기대 행수 claim(C47 불가침 첫 수정 사례 명시)
3. **[기술1]** J1c 중간 하드코딩 무소유 — routes_inventory_test:130·authz_replay:235/:353 상수 3곳이 +2 라우트 시점 즉시 붉음(437→439·427→"429 (414 v1 + 15 v2)"·232→234). J1c 소유 추가 + CI7 상수 grep
4. **[기술2]** operations.go 모순 — 교차확인 참조. r2 인코딩 확정(권고: 빈 ResourceKinds — 단순·계약 무위반 실측)
5. **[기술3]** CI8 grep 오류 파일 — ConnectionScoped 실소재는 contract/**operation.go**:4(adapter.go 아님). CIm "13 resourceType" 산수도 오류(실측 고유값 15 — 블록12/값15/하위형19). 패리티 단얫 명세 수치 정정
6. **[기술4]** `create|` 핸들 마커 Poll 미라우팅 — executor_state.go:38 `state|` 접두만 분기·state 화이트리스트 6종(executor_state.go:128-138)에 create 부재·`state|`+manifest는 404→에러(§3.4 "404→Running" 불일치). 핸들 형식 재설계 + executor_state.go J1b 소유 + §5 #4 예외
7. **[위험1]** 멱등키 소각 배선 모순 — 계획 맵 키(`${connectionUid}:${op}:...`) vs 소각 템플릿(`${resourceUid}:${op}` k8s.js:170) 인자 비정합 → 키 미소각 → 삭제 후 재생성이 낡은 SUCCEEDED 재생(N6 replay wins) = **UI 성공·클러스터 미생성 silent 무결성 위반**. 배선 정합화 + 종단 후 재발화→신규 태스크 uid 단얫 claim
8. **[위험2]** RESOURCE_BUSY 무기한 잠금 — 승인 만료 부재·cancel 라우트 503 미착지(tasks.go:20-21)·키 맵 메모리성(새로고침=새 키=409). 탈출=승인자 Reject뿐. RI-9 재평가(중/중)+탈출 경로 기술
9. **[단순1]** ConnectionScoped 인코딩 — 교차확인(기술2) 참조
10. **[검증1=CRITICAL 강등 아님·V-1]** 신규 테스트 존재 증명 전무 — `go test -run 무매치 → ok exit 0` 실증. 501 stub이 전 필수 claim 통과. CIm·CI9·CI10에 `grep -c 'func Test<이름>' → 1` 추가
11. **[검증2·V-2]** CI13② vacuous — 스펙 1556행에 I10 이미 존재("carried as I10 (D-15)") → J3 무작업 통과. 'resolved:' 고유 패턴 또는 매치수 델타로 교체
12. **[검증3·V-3]** CI16 대체 증거가 J1c 본체(operations_connection.go — policy 입력·합성 uid·400/404/409) 미포함. 대체 집합에 operations_connection_test 지정 단얫(plan 200·execute 201·키 무지정 400·불소속 kind 400) 포함

### CRITICAL 근거 원문 (반려 게이트 기록용)
V-1: "신규 테스트 존재 증명 전무 — no-op 구현이 전 필수 claim 통과 가능. 실증: `go test ./internal/infra/contract/ -run TestNoSuchTestExists` → `ok [no tests to run]` exit 0." — 검증성 렌즈 단독(구조 파괴 아닌 게이트 보강 사안·다른 렌즈 미확인) → **단독 반려 없음, r2 필수 반영**

### MEDIUM (17) — r2 반영
- M3 [완전3] parseK8sManifestIdentity·friendlyK8sYAMLError 처분 확정(유일 호출자 create 본체)
- M4 [완전4] 고아 타입 3종(K8sResourceYAMLPayload·K8sResourceDeletePayload·k8sManifestIdentity) 처분/이월
- M5 [완전5·검증4] 스펙 §10.2 개정(ConnectionScoped 시)·§8.5 개정 Phase 배정 모순(J0 vs J1c 3곳 상충) 통일
- [기술5=F5] "13 resourceType" 산수 정정(15)·ReplicaSet 미공개 초과분
- [위험3] 롤백 런북: 비종단 conn: 행=활성 잠금 — cleanup(SQL) 단계 추가("잔여 무해"는 종단 행만 참)
- [위험4] J1c~J3 승인 우회 창 — deprecation 통지·운영 공지 완화책
- [위험5] Secret 매니페스트 payload_json 영속+taskView 전문 노출 — 봉인 계약 확장(반식별화 또는 명시적 수용 기록)
- [위험6=검증9] J2 동작 검증 — CI14 필수화 또는 등가 자동 단얫 의무화
- [위험7] conn: 접두 예약 단얫 추가(uid 생성기 비충돌 불변식)
- [단순2] kinds 3중 열거 파생 계약 명문화(contract 표 → createServedKinds/실행기 파생)

### LOW (18) — r2 재량 반영
완전 L6~L9(e2e 모드 규율 serve-b/c·골든 -update 절차 본문·k8s.js 226행·§14.2-3 인용) · 기술 F7~F10(12스위트→실측 51 Test·CI9 알파벳 단얫·낡은 주석 grep 불가·226행) · 위험 F8~F9(RFC1123 charset 가드·repo 외 census 통지창) · 단순 F3~F5(J0+J1a 병합 여지·grep 3곳 보강·§10.2 브리프 불일치) · 검증 V-10~V-14(CI4 3분리 grep·CI12 경로·CI15 web/src 전역·CI13① 절 근거·태그 존재 검사)

## r2 지시 사항
1. HIGH 11건 전부 반영(교차확인 4건 우선) — 인코딩 결정 필요분: ConnectionScoped(빈 ResourceKinds 권고 vs 필드+소유 지정)·핸들 마커 형식
2. MEDIUM 17건 반영 · LOW 재량
3. 발견 미반영 시 사유 명시(처분표 형식 — phase6 §15 R-* 선례)
4. 수치·좌표 정정은 전부 실측값으로(13→15 resourceType·k8s.js 226·기저 TestChar 25·스펙 1556행)
5. r2 상단에 개정 이력(본 문서 발견 번호 ↔ 반영 위치 역주)
