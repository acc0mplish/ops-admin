# C 트랙 인수인계 — ops-admin 자체 UI의 컨트롤 플레인 대시보드 (후속 과제용)

- 출처: `v2-phase1/otel-lgtm-recipe-plan.md` r1 §5. **r2에서 본 파일로 분리**됐다.
- 분리 사유: 사용자 확정(2026-09-08) — **D-1 기각**. C 트랙은 v1 `monitor:query` 권한면을 재사용하지 않는다. `/api/v2` 전용 라우트 설계는 별도 후속 과제다.
- **본 문서는 계획이 아니라 인수인계 자산이다.** 후속 과제가 착수할 때 출발점으로 쓴다. 여기의 어떤 항목도 `otel-lgtm-recipe-plan.md`의 Phase·claim에 포함되지 않는다.

---

## 1. 왜 v1 monitor 재사용이 기각됐는가 (재제안 금지 근거)

r1은 `POST /api/v1/monitor/query/{instant,range}` 재사용을 기본안으로 제시했다. 적대검토가 아래를 실측했고 사용자가 기각을 확정했다.

| # | 사실 (file:line 실측) | 결과 |
|---|---|---|
| 1 | `backend/service/monitor.go:828-831`이 instant 질의마다 `MonitorQueryHistory` 행을 쓰고 `trimMonitorQueryHistories(10)`(`:837-850`)이 **최신 10건만 남기고 전역 DELETE** | 대시보드를 한 번 여는 것만으로 운영자가 `MonitorQuery.vue`에서 쌓은 질의 이력이 소멸한다. **회수 불가** |
| 2 | `middleware.OperationLog`(`backend/middleware/operation_log.go:22-27`)가 GET 아닌 모든 `/api/v1/*`에 감사 행 1건, `auditRequestSummary`(`:59-78`)가 본문 4096B까지 적재 | 패널 N종 × 갱신 주기마다 감사 행 N건 + PromQL 문자열 적재 |
| 3 | `MonitorInstantQuery`(`:800-815`)·`MonitorRangeQuery`(`:774-797`) 질의 검증 0, `GetMonitorDatasource`(`:643-649`) = `db.First(&item, id)` — 소유·활성·범위 검사 0 | 임의 PromQL + 무범위 데이터소스 ID |
| 4 | `fmt.Errorf("Prometheus API returned status %d: %s", …, string(body))`(`:899`·`:945`) → `httpx.Failed(c,400,err.Error())`(`backend/controller/monitor.go:146,158`) | 상류 응답 원문이 API 응답으로 나가고 `error_text` 컬럼에 적재 |
| 5 | `applyMonitorDatasourceAuth`(`:2245-2258`) = basic/bearer/apikey 3분기뿐 | `X-Scope-OrgID` 주입 불가 → 외부 시스템의 테넌트 격리를 끄는 요구로 번역됨 |

**위 5개는 전부 기존 v1 코드의 성질이며 본 계획이 만든 결함이 아니다.** 기각 근거는 이 경로를 채택하면 위 성질을 **V2 관측면이 상속**하고, `monitor:query` 보유자가 커넥션 UID 전수(= V2 인프라 토폴로지 목록)·`secret_access_total{purpose,backend}`·실패 코드를 인증된 v1 API로 읽게 된다는 **권한면 교차**다.

---

## 2. 후속 과제가 만족해야 할 계약

### 2.1 라우트 형상
- 신설 위치: `backend/internal/api/v2/` — `InfraAPI.Register`(`backend/internal/api/v2/infra.go:84-89`)가 조립하는 `/api/v2/infra` 그룹. 그룹은 `backend/router/router.go:535-536`에서 `middleware.Auth(db)` + `middleware.OperationLog(db)`를 이미 두른다.
- **읽기 전용 GET으로 설계한다** — `OperationLog`는 GET에 감사 행을 쓰지 않으므로 위 §1-2 문제가 재발하지 않는다.
- **PromQL을 클라이언트가 보내지 않는다.** 서버가 패널 ID → 사전 정의 식으로 해석한다(§1-3 상속 차단). 예: `GET /api/v2/infra/telemetry/panels/:panel_id?range=6h`.
- 권한: v1 `monitor:query`가 아니라 V2 인프라 읽기 권한 체계를 쓴다. `opdef` 부여 여부는 후속 과제가 판정한다(현행 V2 GET 4종은 opdef 미부착 — `infra.go` 패키지 주석).

### 2.2 라우트 골든 — 후속 과제 범위에 반드시 포함할 것
신규 `GET /api/v2/infra/...` 행은 **`docs/security/route-inventory.txt`(453줄, v2 행 254-261)를 반드시 변경시킨다.**
`otel-lgtm-recipe-plan.md`의 보존 제약 #5("라우트 골든 무변경")는 **그 계획에만 구속력이 있다** — 후속 과제는 골든 재생성을 자기 범위에 포함해야 한다. 이 문장이 없으면 후속 과제가 상속받지 않아도 될 제약에 막힌다.

### 2.3 상류 오류 리댁션
Mimir 응답 본문을 그대로 에코하지 않는다(§1-4 상속 차단). 상태 코드와 분류된 오류 코드만 반환하고 원문은 서버 로그에만 남긴다.

### 2.4 데이터소스 주소
`otel-lgtm-recipe-plan.md` §4.3 확정 형상 기준:
- 질의: `http://mimir:9009/prometheus/api/v1/query{,_range}` (Mimir monolithic HTTP 포트 **9009**)
- 멀티테넌시: 기본 비활성. 활성 시 `X-Scope-OrgID` 헤더 필요 — 후속 과제의 HTTP 클라이언트는 임의 헤더를 실을 수 있어야 한다(v1 코드가 못 하는 지점).

---

## 3. 패널 목록과 PromQL (자산 — 골든 대조 불일치 0 확인됨)

현행 §18.2 M1 14종만 전제한다. 적대검토 기술 렌즈가 **14식 전부 ↔ `backend/internal/infra/metrics/metrics_test.go:118-218` 골든 대조 불일치 0**을 확인했다. 아래 표의 "실측"은 2026-09-08 스모크 스택(`grafana/alloy:v1.19.2` + `grafana/mimir:3.2.0`)에서 실제 질의해 응답을 받은 항목이다.

| # | 패널 | 타입 | 엔드포인트 | PromQL |
|---|---|---|---|---|
| P1 | 커넥션 헬스 | 상태 그리드 | instant | `provider_health` **(실측 — `{connection,instance,job}` 라벨 보존 확인)** |
| P2 | 프로바이더 API p95 지연 | 시계열 | range | `histogram_quantile(0.95, sum by (provider, op, le) (rate(provider_api_latency_seconds_bucket[5m])))` **(실측 — 평가 성공)** |
| P3 | 프로바이더 API 에러율 | 시계열 | range | `sum by (provider, code) (rate(provider_api_errors_total[5m]))` |
| P4 | 레이트리밋 발생 | 시계열 | range | `sum by (provider) (rate(provider_rate_limit_total[5m]))` |
| P5 | 인벤토리 동기화 소요 p95 | 시계열 | range | `histogram_quantile(0.95, sum by (connection, mode, le) (rate(inventory_sync_duration_seconds_bucket[30m])))` |
| P6 | 동기화 변경 건수 | 시계열 | range | `sum by (connection) (rate(inventory_sync_resource_changes_total[15m]))` |
| P7 | 부분 동기화 비율 | 단일값 | instant | `sum(increase(inventory_sync_partial_total[1h])) / clamp_min(sum(increase(inventory_sync_duration_seconds_count[1h])), 1)` **(실측 — 평가 성공, 0 반환)** |
| P8 | 태스크 소요 p95 (성공/실패) | 시계열 | range | `histogram_quantile(0.95, sum by (operation, status, le) (rate(provider_task_duration_seconds_bucket[15m])))` |
| P9 | 태스크 실패 | 시계열 | range | `sum by (operation, code) (rate(provider_task_failures_total[15m]))` |
| P10 | 태스크 재시도 | 시계열 | range | `sum by (operation) (rate(provider_task_retries_total[15m]))` |
| P11 | 워커 큐 깊이 | 시계열 | range | `worker_queue_depth` |
| P12 | 리스 만료 | 시계열 | range | `rate(worker_lease_expired_total[15m])` |
| P13 | Stale 리소스 (kind별) | 시계열 | range | `sum by (kind) (increase(resource_stale_total[1h]))` |
| P14 | 시크릿 접근 | 시계열 | range | `sum by (purpose, backend) (rate(secret_access_total[1h]))` |

**P7 분모 주의**: 14종에는 "동기화 총 실행 횟수" 카운터가 없다. `inventory_sync_duration_seconds_count`(histogram `_count`)를 총 실행 수로 쓴다. `clamp_min(…,1)`은 0 분모 방어다.

---

## 4. "미상" 렌더 계약 (필수 — H-4 상환)

`/internal/metrics`는 **빈 문자열일 수 있다.** 렌더러의 10개 패밀리가 조기 return한다: `backend/internal/infra/metrics/gauge.go:36-39`(`len(c.health)==0`)·`gauge.go:55-56`(`!c.queueDepthSet`), `render.go:53,70,100,111,123,145,156,165,176`. 커넥션 미등록·sweep 미실행 스택에서 패밀리가 통째로 생략된다.

따라서 UI 계약:
1. **패밀리 부재는 "비정상"이 아니라 "미상"이다.** P1 상태 그리드는 `provider_health` 무결과일 때 커넥션을 **회색/미상**으로 렌더한다. 빨강(비정상)으로 칠하면 안 된다.
2. P7은 패밀리 부재 시 무결과다 — `clamp_min` 분모 방어와 무관하다. 무결과는 "0%"가 아니라 **"—"** 로 렌더한다.
3. 데이터소스는 정상인데 패밀리만 없는 상태를, 데이터소스 장애와 **구분해서** 표시한다.
4. 패널 단위 degrade: 한 패널 실패가 화면 전체를 실패시키지 않는다.

---

## 5. 확장 착지 후 변경 (`otel-m18-ext`)

`otel-m18-ext`가 착지하면 패밀리가 **14종 → 16종**이 된다(신설 `inventory_sync_total{connection,mode,status}` · `inventory_sync_last_success_timestamp_seconds{connection}`, `status` 어휘 = `succeeded|partial|failed`). 그때:
- **P7 대체 가능**: `sum(rate(inventory_sync_total{status="partial"}[1h])) / clamp_min(sum(rate(inventory_sync_total[1h])), 1)`. **현행 P7 식도 계속 유효하다** — `inventory_sync_partial_total`은 §18.2 M1 계약 행이라 제거되지 않는다.
- **P1 롤업 신설 가능**: `provider_health`에 `provider` 라벨이 붙으므로 `sum by (provider) (provider_health)`. 기존 P1 식은 그대로 유효하다(라벨 추가는 기존 식을 깨지 않는다).
- **P15 신설 가능**: `time() - inventory_sync_last_success_timestamp_seconds` — 마지막 성공 이후 경과.

**주의 — 존재하지 않는 라벨 매처는 오류가 아니라 빈 결과다.** 확장 착지 전에 `{status="partial"}` 매처를 쓰면 알람이 조용히 침묵한다. 착지 확인 전에는 쓰지 않는다.

---

## 6. 되돌림 비용

본 문서는 문서일 뿐 스펙 조항이 아니다 — 폐기 비용 0. r1이 이 계약을 스펙 §18.5에 박으려 했던 Phase(P6)는 **삭제**됐다(H-11 상환): D-1을 뒤집을 때 되돌릴 대상이 스펙 조항과 그 인용 문서가 되는 상황을 만들지 않는다.
