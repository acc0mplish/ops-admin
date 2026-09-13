# I5 하드캡 분할 리컨 (2026-09-12, 메인 직접)

task: I5·I6 하드캡 분할 · tier XL(정정 — L에서 상향: 26파일·~40k줄·전체 시스템 면) · 원천: p6-state next + phase6-plan.md :585(I5·I6 — "monitor.go 4,798 등 하드캡 14건·§3.2 row 4·7·22·OTEL 후속·M2 재판정")

## 현황 실측 (2026-09-12 HEAD d0f5cd3)

### backend Go >800 (17파일)
| 행수 | 파일 |
|---|---|
| 4,801 | service/monitor.go |
| 3,061 | service/service.go |
| 2,488 | service/ops_application.go |
| 2,230 | service/database.go |
| 1,783 | service/integration_ai.go |
| 1,338 | controller/controller.go |
| 1,336 | service/integration_finops.go |
| 1,243 | internal/infra/inventory/compare.go |
| 1,242 | service/database_multidb.go |
| 1,197 | service/database_backup.go |
| 1,169 | service/notify.go |
| 1,112 | service/ops.go |
| 1,023 | service/ops_schedule.go |
| 1,016 | service/ops_job.go |
| 1,007 | service/ssl_certificate.go |
| 1,001 | service/domain.go |
| 831 | controller/monitor.go |

### web src >800 (9파일)
| 행수 | 파일 |
|---|---|
| 2,803 | views/assets/K8s.vue |
| 2,600 | views/monitor/MonitorDashboard.vue |
| 2,456 | views/assets/DatabaseWorkbench.vue |
| 1,397 | layouts/MainLayout.vue |
| 1,076 | views/applications/AppPipelineCenter.vue |
| 1,070 | views/assets/Host.vue |
| 1,037 | views/monitor/MonitorAlertRule.vue |
| 1,009 | views/ops/OpsJobDesigner.vue |
| 849 | views/assets/AssetOverview.vue |

### 650~800 경고대 (참고 — 본 과업 밖)
OpsScriptLibrary 698·AppBuildTaskList 683·model/ops.go 658

## 계획이 판단할 사항
1. **범위 확정**: 계획 문구 "14건"과 실측 26건의 차이 — 전수(backend+web) 1계획 vs I5(backend)·I6(web) 분리. 계획 :585는 I5·I6 병기 — §3.2 row 4·7·22 원문 확인 후 경계 확정
2. **분할 패턴**: BCD 선례(k8s.go 5,062→1,265+신규 10파일·시맨별 커밋·C47 CHAR_IDENTICAL·심볼 보존 3중 증명) 승계 — 각 파일의 시맨 경계·목표 파일 수·≤650
3. **검증 계약**: 파일별 behavior-neutral 증명 수단 — Go: C7 -race·기존 테스트·(순수함수 char는 monitor 영향 확인 필요 — char-baseline은 k8s 소관·Z 오라클과 무관한지) / Vue: bun build·e2e slice-a·i18n 패리티·C12·C61. 골든·상수 무변경(라우트 무변경)
4. **Phase 분해**: 도메인별 배치(예: monitor 백+프론트 / service 코어 / database군 / ops군 / web 자산군...) — Phase당 ≤5파일·직렬 PR·각 PR 독립 검증
5. **조건 승계**: OTEL 후속(메모리 otel-telemetry-decisions — A트랙만·C트랙 기각·별도 Mimir) — monitor 분할 형상이 OTEL A트랙·메트릭 수집기와 충돌하지 않게. M2 재판정(monitor 미이관·재등록 --monitor-datasource-id — I-a 기록) 참조
6. **리스크**: service.go(God object—§2.5)·controller.go 미들웨어 체인·MainLayout(전 라우트 공유)·e2e 커버 밖 뷰 분할 회귀 탐지 수단

## 프로세스
XL: plan-xhigh → 5렌즈 직렬 → implement(Phase당 ≤5파일·도메인별 배치) → review-pr-xhigh → verify. 상태: p6-state.json `i5` 섹션. 보존: behavior-neutral(라우트·골든·opdef·권한·API 형상 무변경)·waiver 무삭제·kubeconfig 평문 금지·라인캡 ≤650
