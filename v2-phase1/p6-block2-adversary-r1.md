# Phase 6 블록 2 계획 r4 — 5렌즈 적대검토 병합 보고 (r1)

작성: 2026-09-10 · 대상: `v2-phase1/phase6-plan.md` r4 (529행, HEAD 45671e2) ·
렌즈: plan-adversary-xhigh × 5 직렬 (완전성→기술적오류→위험→단순성→검증성)

## 1. 렌즈 × 심각도 × 발견수

| 렌즈 | CRITICAL | HIGH | MEDIUM | LOW | 판정 |
|---|---|---|---|---|---|
| 완전성 | 0 | 2 | 5 | 3 | 조건부 승인 |
| 기술적 오류 | 0 | 3 | 7 | 6 | 조건부 승인 |
| 위험 | 0 | 3 | 4 | 4 | 조건부 승인 |
| 단순성 | 0 | 1 | 5 | 4 | 조건부 승인 |
| 검증성 | 0 | 5 | 10 | 4 | 조건부 승인 |
| **계 (raw)** | **0** | **14** | **31** | **21** | — |
| **계 (dedupe)** | **0** | **11** | — | — | — |

CRITICAL 0 → 반려 게이트 미발동. 전 렌즈 조건부 승인 — HIGH는 착수 전 수정 필수.

## 2. 교차확정 (독립 2+렌즈 일치)

| 결함 | 렌즈 | 확정 수준 |
|---|---|---|
| C24 sed 앵커 부재 (`authGroup.GET` ↔ 실측 `routes_v1_system.go:19 g.GET`) | 기술#1·위험#1·검증#2 | 3렌즈 — 블록 1 미징수 원인 가능성 |
| C62 기대값 모순 (패턴 11종=삭제 전원인데 기대 1) | 기술#2·검증#1 | 2렌즈 |
| "424 v1 + 13 v2" 오기 3곳 (정답 414 v1) | 기술#3(H)·위험#8(L) | 2렌즈 |
| S5 나열 9≠12 + integration_ai.go:964는 Get이 아닌 List | 완전#4·기술#4·위험#6 | 3렌즈 |
| AI detail 소비자 integration_ai.go:966 누락 | 완전#3·위험#5 | 2렌즈 |
| §6.1 허위 포함관계 (C7⊃C11·C12·C61·C19⊃C20·C52⊂C36②) | 기술#7·단순#1 | 2렌즈 — 시행 시 H0 필수 C11 소거 모순 |
| r3 §3.5 죽은 참조 (working tree 부재) | 완전#7·기술#10 | 2렌즈 |

## 3. 수정 지시 — 필수 (HIGH 11건, dedupe)

| # | 위치 | 수정 내용 | 근거 렌즈 |
|---|---|---|---|
| F1 | §6 C24 | sed 앵커를 실측 형태로: `routes_v1_system.go:19`의 `g.GET("/profile", ctl.Profile)` — 변수명 포함 재작성 (또는 앵커 독립 식). 수정 후 dry-run으로 PLANT 성립 확인 | 기술·위험·검증 |
| F2 | §6 C62 | 패턴에 `CreateK8sResourceYAML` 추가+기대값 1 유지, 또는 패턴 그대로+기대값 0+별도 create 존재 증명 grep 신설 — 둘 중 택일 후 산술 성립 | 기술·검증 |
| F3 | §J9:152-153·§3.2:250-251·§12#2:460-461 | "424 v1 + 13 v2" → "414 v1 + 13 v2" (426−12=414) 3곳 일괄 | 기술·위험 |
| F4 | §3.3 vs §4·§J5·§14.2-1 | Phase I 승인 게이트 위치 통일 — §3.3 "I 착수 전 재상신 문서에서 확정" ↔ 타 조항 "I-a 후·I-b 전" 상충 해소. I-a 착수 절차 한 문장 못박기 | 완전#1 |
| F5 | §J5 S표·§3.3 | S6 신설 — 테스트 인프라 결합: `e2e/slicea/harness_test.go:454` raw INSERT INTO k8s_cluster·`:467` backfill UID 도출(S1b가 끊는 결합)·runSyncCLI의 RunK8sBackfill 의존(S1c 대상) + `backfill_test.go`·`secretv2_test.go:280`(Row 8)·`internal/api/v2/infra_test.go:77` — I-a/I-b 처분(register-k8s 경유 또는 provider_connection 직접 삽입 재설계) 편입 | 완전#2 |
| F6 | §J5 S5 | 필드 조달 계약 추가 — GetK8sCluster 교체 구현이 `ConnectionMode`·`GatewayID`(및 소비자 실사용 필드 전수)를 provider_connection에서 조달할 것 명시. 누락 시 gateway 모드 direct 강등·`k8s.go:198` 금지 사설 dial 발생 (`k8s_transport.go:163-176`·`k8s_terminal.go:248` 실측) | 위험#2 |
| F7 | §12 #16·§3.3 | `ops_application.go:1893` `opsPipelineKubeconfigFile` 평문 임시파일 기록 — §12 #16에 임시 파일 예외(수명·권한·cleanup 계약) 명시하거나 I-a 배치 편입. 방치 시 구현자가 #16 위반/범위 위반 강제 선택 | 위험#3 |
| F8 | §6.1 | 허위 포함관계 전부 삭제 또는 실제 포함만 재기술 — "C7·C19·C65만 돌리면 자동 충족" 산정 재작성. C11(H0 필수)이 소거되는 모순 해소 | 단순#1·기술#7 |
| F9 | §J5·§3.3·§6 C64 | 미배치 결합 등재 — `gateway.go:109,169`·`monitor.go:698,4510`·`platform.go:108`의 `&model.K8sCluster{}` 직접 gorm 조회 5곳. census(S6 앞)에 추가+I-a 배치 반영+C64 기대값 정합화. 현행대로면 C64 "0행" 도달 불가 | 검증#3 |
| F10 | §6 C64 | 존재 증명 단계 추가: `grep -c 'func TestGetK8sCluster' backend/service/ → 1` — `-run` 무매칭 ok(vacuous) 방지. S5 전환 무테스트 통과 차단 | 검증#4 |
| F11 | §6 C53·§J8·§10 R27 | "G1 종료 시 1회" 실행 불가(G1의 산출물이 CLI 종결 자체) — 삭제하거나 실행 가능 시점 재정의. 단순#3(2회째 0정보)과 통합 판정: G0 전환 시점 단일 1회 권장 | 검증#5·단순#3 |

## 4. 권장 (MEDIUM — 채택/기각 사유 기록 후 처리)

| # | 항목 | 렌즈 |
|---|---|---|
| R-A | 임시 플래그 `V2_READ_SOURCE_K8S` 삭제 검토 — on 작동 절차 계획 내 전무·C53 compare는 CLI로 플래그 우회·롤백 가치 영. 단 r3 §J3·C36 2단·D-4(플래그 한 사이클 보존 조건화)와의 정합 재판정 필요 — 채택 시 C36 2단→단순화 파급 명시 | 단순#2 |
| R-B | r3 C40 부활 — H2 후 하드코딩 3곳 grep 단얫(`grep -n '!= 449\|!= 439\|!= 244' → 0행`) 승계 또는 폐기 기록 | 완전#5 |
| R-C | S3 칼럼 drop 착지 — §J10 마이그레이션 명세에 `asset_service.k8s_cluster_id`·`ops_application_environment_binding.k8s_cluster_id` 칼럼 drop 추가(`store/migrate.go:52` AutoMigrate 포함 실측)+`asset_service.go:96` map 키 `"k8s_cluster_id"` 잡는 claim 보강 | 완전#6·검증#15 |
| R-D | r3 §3.5 참조 자기완결화 — H1 UI 계약 세부 인라인 또는 `web/src/api/k8s.js:74-112` 주석 좌표 병기 | 완전#7·기술#10 |
| R-E | "내부 호출자 0의 10종" → 11종 정정(§3.2 H2 vs §J9) | 기술#5 |
| R-F | §J8 "컨트롤러·라우트·opdef 무변경" → "라우트·opdef 등록 무변경(컨트롤러 본문 :32는 교체)" 한정 표기 | 기술#6 |
| R-G | §3.4 "절대 건드리지 않는 것"에 §12 #6 (b) 예외 상호참조 추가(I-a backfill.go 수정과 병존 모순 해소) | 기술#8 |
| R-H | seed.go:375 권한 트리 `assets:k8s:cluster:add/edit/delete` 3종·`localization.go:109,199-202` i18n 고아 권한 처방 등재 — 단 `compose_k8s_ops.go:113-299`가 v1 권한(`assets:k8s:workload:scale` 등) 재사용하므로 전면 삭제 불가·선택적 정리로 | 위험#4 |
| R-I | G0 `cluster/info` 응답 형상 계약 — `model/k8s.go` KubeConfig `json:"kubeConfig"` 마스킹 0건 실측. projectK8sClusterInfo의 kubeConfig 필드 처분(마스킹/제외) 못박기+info 동치 판정 claim 신설 | 위험#7·검증#6 |
| R-J | C39′ 판정 입력 계약화 — kubeconfig fixture 경로(저장소 부재 실측)·uid 획득 경로(stdout 사양) 확정 | 검증#7 |
| R-K | C63 'cloud' vacuous 정비 — `backfill_cloud.go`에 'cloud' 부재 불가능한 파일. 실질 존치 증명으로 교체 | 검증#9 |
| R-L | C66 drop 마이그레이션 파일명·러너 진입 명령 확정(자기충족 grep 해소) | 검증#10 |
| R-M | C65 glob `k8s*.go`가 블록 2 비대상 테스트 파일(650초과 652 현존) 포함 — 스코프 한정 또는 처분 계약 명시 | 검증#11 |
| R-N | H1 뷰 배선 검증 — C61 e2e 회피 허용 시 프론트 배선 검증 전무. 최소 grep 단얫 또는 회피 조건 강화 | 검증#14 |
| R-O | S5 소비자 나열 재열거 — 실측 12좌표 전수(controller/k8s.go:32·k8s.go:84,134,185 추가·integration_ai.go:964는 ListK8sClusters 별도 표기). R30이 원문 전수 열거 요구 | 완전#4·기술#4·위험#6 |
| R-P | §14.2-4에 AI detail 경로(`integration_ai.go:966` GetK8sClusterDetail) 추가 인지 요청 | 완전#3·위험#5 |
| R-Q | 되돌림 마이그레이션 삭제 검토 — migrate 러너 순방향 전용·Down 심볼 0건·부활물은 영구 미사용 빈 테이블. 롤백=백업 restore 단일 경로화 | 단순#5 |
| R-R | 수치 6중 명기 → 규범 1곳(C38)+참조화 | 단순#6 |
| R-S | LOW 취사 — 완전#8(7파일→6파일)·#9(verbatim 표장)·#10(21종→20종), 기술#11~16, 위험#9~11, 단순#7~10, 검증#16~19 참조 | 각 |

## 5. 검증 정합 확인 (수정 불요 — 렌즈 실측)

골든 452/292·산술 440/280/437/427(414 v1)/232 · 쓰기 13건 좌표(routes_v1_infra.go:223-253) · 프론트 쓰기 2파일 집중 · cloud compare 분리(G1 종결 파급 0) · 롤백 서술-배치 정합 · P 산출물 접합(k8sassembly:27·projection:68) · register-k8s 인계 §4 7행 이관 · V2_READ_SOURCE_K8S 사전부재 · E0 connectionUid 착지(infra.go:276-283) · k8s.js:74-112 선례 주석 존재.
