# V2 §18.2 메트릭 전면 계측 계획 — M1 14종 계약 완성 (r1)

- 원천: `docs/architecture/multi-infrastructure-control-plane-v2.md` **§18.2**(:1461-1481) — 표 16행 중 **M1=yes 14종**이 계약(라벨·계기 위치 포함, 문언 그대로). 나머지 2종은 스펙 자체가 deferred로 명시.
- 범위: 신규 계기 12종 + `/internal/metrics` 엔드포인트 + health sweep 루프(신규 최소 회로). **프로덕션 동작(동기화·실행·재큐 로직)은 무변경** — 계기 삽입과 렌더 확장만.
- 실측 기준: main `18e2cb1` 작업 트리, 2026-09-08 전수 확인(§0). 실측 없는 추정 배제 — 12종 계기 위치는 전부 file:line 실측.
- 의존 승계: §10.2(:963) "No new runtime dependencies in M1" → **prometheus/client_golang 도입 안 함**. 표준 라이브러리만으로 text exposition 직접 렌더 유지.

---

## 0. 현재 코드 실측 (계획의 사실 기반)

### 현행 metrics 패키지 — 2종, TYPE 헤더 없음
- `backend/internal/infra/metrics/metrics.go`(143행): `Counters`(mutex) = providers·rateLimits·apiErrors 맵. 메서드 `New`·`RegisterProviders`·`IncRateLimit`·`IncAPIError`·`Render`. 렌더는 `provider_rate_limit_total{provider="…"} N` 형태 bare 라인 — **`# TYPE` 헤더 없음**(exposition상 untyped로 유효하나 표준 관례 아님).
- 패키지 순도 계약(주석 :8-10): **표준 라이브러리만 import** — adapters·contracttest 양쪽에서 import 가능한 어휘 인접 인프라.
- 호출부 전수: 4 어댑터 client.go(아래 §1.1)·`fake/fake.go:120,158`·`compose/compose.go:43-49,105`(공유 Counters + RegisterProviders 4종)·`inventory/sync.go:139-141`(SyncReport.MetricsText = Render()).
- 어댑터 주입 패턴: 각 `NewAdapter` 기본 `metrics.New()` + `WithCounters(counters)` 옵션 — compose가 스택 공유 Counters를 주입(compose.go:54,68,79,106).
- 하위 호환 단얫 실측: `compose_test.go:59,108,264` — `strings.Contains(Render(), …)` **부분 문자열 단얫**(TYPE 헤더 추가에 영향 없음). `sync_test.go:519` TestSyncReportCarriesMetricsRender 역시 Contains(:540). `metrics_test.go:47`만 라인 수 정확 일치 단얫(=2) — **TYPE 헤더 도입 시 깨지는 유일 지점**.
- `metrics_test.go:62` TestNewZeroState: `New().Render() == ""` — 패밀리 부재 시 렌더 공백 계약(헤더는 패밀리 존재 시에만 발행하면 보존됨).

### 엔드포인트·health 루프 — 둘 다 미존재
- `/internal/metrics` 라우트 전역 grep 0건. metrics.go 주석(:2-5)이 "endpoint and the full §18.2 table are explicitly deferred to Phase 3+ (§13 부채 대장)"로 명시 — 본 과제가 그 상환.
- **health check loop 부재**: `Health()`는 4 어댑터+fake에 구현(예: `aliyun/adapter.go:115`, `proxmox/adapter.go:152`)되나 **비테스트 호출부 0건**. `provider_connection.last_health_at` 컬럼은 존재(model/provider.go:22)하나 기록 주체 없음(읽기만: api/v2/infra.go:142,168). 스펙의 유일 언급은 §18.2 표(:1467) — 루프는 본 과제가 최소 형태로 신규 생성(§1.1-A).
- 라우팅 실측: `router.New(cfg, db, v2API)`(router/router.go:29) — 외부 호출부는 `main.go:96` 유일(골든 테스트는 패키지 내 직접 `New`). 전역 미들웨어 = Logger·Recovery·CORS(router.go:47). 라우트 골든: `router/routes_inventory_test.go:123` TestRouteInventoryArtifact — `New()`가 조립한 표면 vs `docs/security/route-inventory.txt` **바이트 골든**(nil v2API 부트 = "unchanged-v1" 단얫, R11).
- 서버 조립: `main.go:96-100` — router.New → http.Server → ListenAndServe. 엔진 레인: `engine_config.go:132` startEngineLane(compose.Build → BuildEngine → Start).

### 계기 삽입 지점 실측 (12종 대상)
- 어댑터 REST 초이크(에러/ratelimit 계기가 이미 몰려 있는 단일 관문): aliyun `ecsClient.request`(client.go:128), kubernetes `k8sClient.doJSON`(client.go:234), proxmox `Client.do`(client.go:200 — get:307/postForm:429이 경유), tencent `cvmClient.do`(client.go:173).
- sync: `inventory/sync.go:64` RunSync — started:87 / finished:98 / 레포트 확정:137-141. partial 판정:108-112(+rate-limited partial 복귀:227-232). 부재 사다리(=staleness sweeper의 실체): `reconcileAbsences`:390-427 — `noteAbsence`(reconcile.go:165)가 stale_candidate/tombstoned outcome 반환, kind는 루프의 `res.Kind`로 접근 가능. **별도 sweeper 프로세스는 부재** — 스펙의 "staleness sweeper"는 이 reconcile 패스로 매핑.
- tasks: `finishAttempt`(engine.go:509 — Complete/Fail 종단 커밋), `fireTaskTerminal`:559, `requeueForRetry`(engine.go:580 — 재시도), `cancelAtClaimBoundary`(engine.go:398 — 미실행 종단), reaper.go `ReapOnce`:26(만료 리스 처리 :40-95 — 재큐/소진 두 갈래), loops.go `pollLoop`:56(주기 틱). `ClaimedTask.Attempt.StartedAt`(:339-396에서 claim 시각)이 claim→terminal 구간 측정의 원점. **engine.go 898행 — 800 하드캡 초과 상태**(§5 R3 분해 필요).
- secrets: `broker.go` `Resolve`(:66) 성공 반환값이 `binding.Purpose`·`ref.Backend`(model/secretref.go:11, default "internal")를 이미 보유 — 라벨 소스 즉석 확보. Broker 생성부 2곳: compose.go:133, engine 내부(NewEngine).
- 큐 깊이: `ClaimNext`(engine.go:297)는 단일 후보 SELECT — **COUNT 질의 부재**, 신규 메서드로 추가(§1.1).

---

## 1. 계약 요소 결정

### 1.1 측정 대상 12종 → 계기 위치 매핑 (§18.2 표 라벨 그대로)

| # | 메트릭 (타입·라벨) | 계기 위치 (실측) | 비고 |
|---|---|---|---|
| A | `provider_health` gauge 0/1 {connection} | **신규** `compose/healthloop.go` sweep 루프 → `SetHealth(connUID, healthy)` | 루프 미존재 → 최소 신규 회로(아래 가정 A6). 라벨값 = connection UID |
| B | `provider_api_latency_seconds` histogram {provider, op} | 4 어댑터 초이크: aliyun `request`:128, k8s `doJSON`:234, proxmox `do`:200, tencent `do`:173 — `start:=time.Now()`+이연 Observe | 공통화는 metrics 패키지의 `ObserveAPILatency(provider, op, d)` 단일 헬퍼로(계측 호출부만 공통화, HTTP 코드는 어댑터별 유지) |
| C | `inventory_sync_duration_seconds` {connection, mode} | sync.go RunSync 확정점(:137-141 부근) — `finished.Sub(started)` Observe | mode는 RunSync 정규화값(full/incremental/targeted) 그대로 |
| D | `inventory_sync_resource_changes_total` {connection} | 동일 확정점 — `Outcomes[created]+Outcomes[updated]` Inc | 가정 A2: changes = created+updated |
| E | `inventory_sync_partial_total` {connection} | 동일 확정점 — 최종 status == partial일 때 Inc | partial 판정은 기존 분기(:108-112, :227-232) 결과값 사용 |
| F | `provider_task_duration_seconds` histogram {operation, status} | `finishAttempt`(engine.go:509 이동후 terminal.go) 커밋 성공 후 — `now − claim.Attempt.StartedAt`. `cancelAtClaimBoundary`(:398)·reaper 소진 종단(reaper.go:57-68)도 동일 발화 | 가정 A4: 모든 종단 커밋이 발화. 종단 아닌 requeue는 미발화 |
| G | `provider_task_failures_total` {operation, code} | finishAttempt 종단 중 status ∈ {failed, timed_out} — `p.errorCode` 라벨. reaper 소진 경로는 `lease_expired` 코드로 발화 | 가정 A4 |
| H | `provider_task_retries_total` {operation} | `requeueForRetry`(engine.go:580) 트랜잭션 성공 후 | reaper 재큐는 제외 — #J가 담당(가정 A4) |
| I | `worker_queue_depth` gauge (무라벨) | **신규** `Engine.QueueDepth(ctx)` — `COUNT(*) WHERE status='queued' AND next_attempt_at<=now`, `pollLoop`(loops.go:56) 틱마다 Set | 스펙 "poller, COUNT query"의 직접 이행. 측정 질의는 기존 클레임 판정과 무관한 읽기 전용 |
| J | `worker_lease_expired_total` (무라벨) | reaper.go ReapOnce — CAS 승리(RowsAffected>0)한 만료 리스 처리마다 Inc(재큐·소진 양 갈래) | "처리된" 만료 리스 기준(가정 A4) |
| K | `resource_stale_total` {kind} | sync.go `reconcileAbsences` 루프(:421-423 outcome 분기) — outcome == stale_candidate일 때 `res.Kind` 라벨 Inc | 가정 A3: stale_candidate 전환만. tombstoned는 미가산 |
| L | `secret_access_total` {purpose, backend} | broker.go `Resolve` 성공 반환 직전 — `binding.Purpose`·`ref.Backend` Inc | 가정 A5: 성공만. engine 내부 broker의 operations 목적 해석도 동일 계기(목적 라벨로 구분) |

계기 API는 전부 `*Counters` 메서드(Observe/Inc/Set)로 — 어댑터 주입 패턴(`WithCounters`)과 nil 허용 관례(`OnTaskTerminal` nil 필드, engine.go:41-52 선례)를 그대로 따른다.

### 1.2 렌더 형식 — Render() 확장 (재작성 아님)
- Prometheus text exposition 표준 채택: 패밀리별 `# TYPE <name> counter|gauge|histogram` 헤더 + counter/gauge 값 라인 + histogram은 `_bucket`(le 오름차순, `+Inf` 포함)·`_sum`·`_count`.
- 기존 2종 라인 포맷은 **byte 불변**(그 앞에 헤더 라인만 추가) — compose_test·sync_test의 Contains 단얫이 무수정 통과하는 근거. 패밀리 부재 시 헤더도 미발행(TestNewZeroState 공백 계약 보존).
- 버킷 상수: API latency `[.005,.01,.025,.05,.1,.25,.5,1,2.5,5,10]` / task·sync duration `[.05,.1,.25,.5,1,2.5,5,10,30,60,120,300]` (+Inf 공통).
- 라벨값 이스케이프(`\`·`"`·개행)와 패밀리·라벨 정렬(결정성)은 Render 계약에 포함 — TestRenderDeterministicAndSorted 계승.
- 렌더 조립은 파일 분할로 이전(§2 P1: render.go) — 타입명 `Counters`·기존 공개 메서드 시그니처 전부 불변.

### 1.3 엔드포인트 — `/internal/metrics`
- 등록 위치: **main.go에서 router.New 반환 후 `engine.GET("/internal/metrics", h)`**(main.go:96 직후). 근거 2: ① router.New 시그니처·골든 불변 — 라우트 골든(route-inventory.txt)과 R11 "unchanged-v1" 단얫에 영향 0(route-coverage CI 잡 무변경 통과) ② 내부 스크래프 엔드포인트는 /api 트리·opdef·권한 마커 체계 밖이므로 엔진 레벨 등록이 계층 정합.
- 인증: **gin 미들웨어 게이트 없음**(스크래퍼는 JWT 없음). §18.2 문언(:1463)은 "internal-only"만 요구하고 인증 규정 없음 — 내부 전용은 **배포 경계(리버스 프록시 비노출·망 분리)로 시행**을 가정으로 명시(가정 A1, 런북 소관). 전역 CORS·Logger 미들웨어는 경유(읽기 전용 text라 무해).
- 핸들러 소스: startEngineLane이 stack의 Counters Render 함수를 노출(반환 확장) — 스택 실패(R11 경로) 시 nil → 라우트 미등록. 핸들러 자체는 httptest로 직접 단위 검증.
- 트레이드오프 명시: route-inventory.txt 골든은 router.New 조립 표면만 문서화하므로 본 라우트는 골든에 미수록 — 계획 문서로 대체 기록(§7 claim 9).

### 1.4 의존 — 신규 0
- §10.2 "No new runtime dependencies in M1" 승계: client_golang 미도입, metrics 패키지 표준 라이브러리만 import(주석 계약 유지), go.mod·go.sum 무변경.

### 1.5 health sweep 루프 (신규 최소 회로)
- `compose/healthloop.go`: 주기(상수 5분, 가정 A6)마다 provider_connection 행 순회 → registry 어댑터 `Health()` → `SetHealth(uid, result.Healthy)`. ConnectionView 조립은 sync.go:190-201과 동일 형상(broker.Resolve(uid,"inventory") — 가정 A6). 해석 실패 시 gauge 0. **API 변경 없음. DB 기록(last_health_at)은 성공 시만 갱신(계획 r2 승인 — ④리뷰 MEDIUM-2: 고아 컬럼의 최초 기록 주체로서 건전, gauge 0 갈래는 기록하지 않는다).** 실패 시 gauge 유지·기록 없음. startEngineLane에서 taskEngine.Start와 함께 기동/정지(같은 context 파생).

---

## 2. Phase 분해 (각 ≤5파일, 독립 검증 단위)

의존: **P1 → {P2, P3} / P4 → P5 → P6**. P4(순수 이동 리팩터)는 P1과 무관하게 선행 가능. 기본 직렬 순서 P1→P2→P3→P4→P5→P6.

### P1 — metrics 패키지 확장 (12종 패밀리 + 표준 렌더)
파일(5): `backend/internal/infra/metrics/metrics.go`(수정 — 신규 counter 패밀리 필드·Inc 메서드), `…/histogram.go`(신규 — latency·duration Observe + 버킷 렌더), `…/gauge.go`(신규 — health 맵·queue depth + Set/렌더), `…/render.go`(신규 — 통합 Render 조립·TYPE 헤더·이스케이프·정렬; 기존 조립 코드 이전), `…/metrics_test.go`(수정 — 전 패밀리 골든 + 기존 단얫 갱신 1곳).
검증: `go test ./internal/infra/metrics/ -race` — 14종 패밀리 골든(§4 T-1), 기존 2종 라인 byte 불변 단얫, 빈 상태 렌더 "".

### P2 — 어댑터 latency 계기 (4 초이크)
파일(5): `backend/internal/infra/adapter/aliyun/client.go`·`…/kubernetes/client.go`·`…/proxmox/client.go`·`…/tencent/client.go`(각: 초이크 함수 시작 시각 + 이연 `ObserveAPILatency(ProviderName, op, d)` — 기존 IncAPIError/IncRateLimit 호출부와 같은 관문), `backend/internal/infra/adapter/latency_test.go`(신규 — httptest 스텁 엔드포인트로 4종 각각 구동, 공유 Counters 렌더에 bucket 라인 단얫).
검증: 신규 테스트 + 기존 4 어댑터 테스트 무변경 통과(`go test ./internal/infra/adapter/... -race`).

### P3 — inventory·stale·secrets 계기
파일(5): `backend/internal/infra/inventory/sync.go`(C/D/E/K 4종 — RunSync 확정점+reconcileAbsences 분기), `backend/internal/infra/secrets/broker.go`(L — Counters 필드 + `NewBrokerWithCounters` 옵션 추가, 기존 `NewBroker(db)` 시그니처 불변), `backend/internal/infra/compose/compose.go`(broker 주입 교체), `backend/internal/infra/inventory/sync_test.go`(증설 — 델타 단얫), `backend/internal/infra/secrets/broker_test.go`(증설 — purpose/backend 라벨 단얫).
검증: `go test ./internal/infra/... -race` — sync 레포트 델타(성공 run duration/changes, partial run partial+duration, 2회 결어 stale by kind), broker 해석 1회 → secret_access_total{purpose="inventory",backend="internal"} 1.

### P4 — engine.go 분해 (800 하드캡 상환, 순수 이동)
파일(2): `backend/internal/infra/../tasks/engine.go`(종단 경로 이동으로 축소 + `Metrics *metrics.Counters` 필드·`QueueDepth(ctx)` 메서드 추가 — 계기 호출 없는 컴파일 단계), `backend/internal/tasks/terminal.go`(신규 — Complete/Fail/finishAttempt/fireTaskTerminal/requeueForRetry/closeAttempt 이동, byte 동등 이동).
검증: `go test ./internal/tasks/ -race` 기존 9개 테스트 파일 무변경 통과(이동 검증), `wc -l` engine.go ≤ ~720·terminal.go ≤ ~250.

### P5 — task 엔진 계기 (F/G/H/I/J)
파일(5): `backend/internal/tasks/terminal.go`(finishAttempt F/G·requeueForRetry H·cancelAtClaimBoundary F), `backend/internal/tasks/loops.go`(pollLoop 틱 QueueDepth COUNT → SetQueueDepth), `backend/internal/tasks/reaper.go`(J — CAS 승리 처리마다 Inc; 소진 종단 F/G), `backend/internal/infra/compose/compose.go`(BuildEngine에 `engine.Metrics = s.Counters` 1줄), `backend/internal/tasks/engine_metrics_test.go`(신규 — 제출→클레임→실행→Complete/Fail/재큐/reap 경로 델타 단얫, nil Counters 무동작 단얫).
검증: 신규 델타 테스트 + `go test ./... -race`.

### P6 — health sweep + 엔드포인트
파일(5): `backend/internal/infra/compose/healthloop.go`(신규 — §1.5 루프), `backend/internal/infra/compose/healthloop_test.go`(신규 — fake 어댑터 healthy → gauge 1, 바인딩 부재 커넥션 → gauge 0), `backend/engine_config.go`(startEngineLane — Render 함수 노출 + sweep 기동), `backend/main.go`(:96 직후 라우트 등록 + 핸들러), `backend/main_metrics_test.go`(신규 — 핸들러 httptest 200 + text body 단얫, nil 미등록 단얫).
검증: 신규 테스트 + 라우트 골든 무변경 확인(`go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact'`) + 전체 `go test ./... -race`.

---

## 3. 수정 파일 전수 목록 (24개 고유)

**수정(12)**: `backend/internal/infra/metrics/metrics.go`, `backend/internal/infra/adapter/{aliyun,kubernetes,proxmox,tencent}/client.go`, `backend/internal/infra/inventory/sync.go`, `backend/internal/infra/secrets/broker.go`, `backend/internal/infra/compose/compose.go`, `backend/internal/tasks/engine.go`, `backend/internal/tasks/loops.go`, `backend/internal/tasks/reaper.go`, `backend/engine_config.go`, `backend/main.go`
**신규(9)**: `backend/internal/infra/metrics/{histogram.go,gauge.go,render.go}`, `backend/internal/infra/adapter/latency_test.go`, `backend/internal/tasks/terminal.go`, `backend/internal/tasks/engine_metrics_test.go`, `backend/internal/infra/compose/{healthloop.go,healthloop_test.go}`, `backend/main_metrics_test.go`
**테스트 수정(3)**: `backend/internal/infra/metrics/metrics_test.go`, `backend/internal/infra/inventory/sync_test.go`, `backend/internal/infra/secrets/broker_test.go`
(동일 파일 복수 Phase 편집: compose.go P3·P5, terminal.go P4·P5 — 순서 계약은 §2 의존 준수)

---

## 4. 테스트 계약

- **T-1 렌더 골든**(P1): 14종 전 패밀리를 발화시킨 Counters의 Render() 전문을 예상 텍스트와 byte 비교 — TYPE 헤더·버킷 le 오름차순·_sum/_count·라벨 정렬·이스케이프 포함. 기존 2종 라인 형태 그대로 포함.
- **T-2 계기 델타 단얫**(P2/P3/P5/P6): 각 계기 위치에서 "행위 1회 → 해당 패밀리 라인 +1(또는 gauge 값 설정)"을 렌더 파싱으로 단얫 — adapter latency(bucket 라인), sync 3종+stale, engine 5종(duration은 bucket 존재·status 라벨), secret 접근, health gauge, queue depth.
- **T-3 하위 호환**(P1-P6 전 Phase): compose_test:59/108/264·sync_test:519·tasks 9개 테스트 파일·라우트 골든 2종 — **무수정 통과**가 곧 검증(보존 제약 §8).
- **T-4 무동작 안전**(P5): Engine.Metrics nil 시 기존 경과와 동작 동일(계기 부재가 실행·재큐·종단을 바꾸지 않음).
- 로컬 완료 게이트: `cd backend && go test ./... -race -count=1`(CI backend-test 잡과 동일 명령, 원격 대기 없음).

## 5. 위험 지점과 검증 방법

- **R1 엔드포인트 무인증 노출**(P6): §18.2 문언상 요구 없으나 배포 경계 시행이 가정(§1.3). 검증: 핸들러 테스트 + claim 9로 등록 위치·nil 가드 확인. 완화: 1줄 롤백. 배포 측 런북 기재는 본 과제 문서 범위로 하되 시행은 운영 소관임을 계획에 명시.
- **R2 health sweep의 실제 클라우드 API 트래픽**(P6): 5분 주기 전 커넥션 Health probe는 4개 클라우드로 실요청. 완화: 주기 상수 5분(가정 A6)·레이트리밋 신호는 기존 provider_rate_limit_total 계기가 자동 관측. 검증: 루프 테스트는 fake/바인딩 부재로 실API 무호출.
- **R3 engine.go 898행 하드캡**(P4): 계기 추가로 930행 돌파 전에 분해가 선행 조건. 순수 이동 Phase로 기존 테스트 무변경 통과가 이동 무결 증명. (레포 전역 800 초과 파일은 본 과제가 성장시키는 파일만 관할 — engine.go 외 신규 성장 파일은 전부 신규 분할.)
- **R4 TYPE 헤더가 artifact bytes 변경**(P1): SyncReport.MetricsText가 골든 문자열을 포함하는 소비자는 Contains 단얫뿐(실측 §0). 검증: T-3.
- **R5 측정 질의가 클레임 회로에 간섭**(P5): QueueDepth COUNT는 읽기 전용·별질의 — 기존 트랜잭션과 무관함을 코드 리뷰로 확인(claim으로 명시).
- **R6 broker 이중 계기**(P3): compose broker(inventory 목적)와 engine 내부 broker(operations 목적)가 같은 계기에 적립 — 의도된 구분(라벨 purpose). 검증: broker_test가 목적별 라벨 단얫.

## 6. 롤백 가능성 판단

**높음 — 전 Phase 추가형(additive)이다.** 각 Phase 독립 커밋 → 단일 Phase `git revert`로 이후 Phase에 파급 없음(계기·패밀리는 미호출 시 렌더에 미등장). 최악 선택 롤백: 엔드포인트 = main.go 1줄 제거, health sweep = startEngineLane 기동 1줄 제거, 어댑터 계기 = 초이크 함수 2줄(시각+Observe) 제거. 스키마·시드·프로덕션 제어흐름 변경 0이므로 데이터 롤백 불필요.

## 7. 검증 요구 (claims — 구현 완료 후 리뷰가 검증)

1. `backend/internal/infra/metrics/` 4개 소스 파일에 §18.2 M1 14종 각각의 발화 메서드(Inc/Observe/Set)가 존재한다 — `grep -c` 메서드명 12종 신규 + 기존 2종.
2. metrics_test.go에 14종 전 패밀리를 포함하는 Render 전문 골든 테스트(함수명 `TestRenderFullFamilyGolden`)와 `# TYPE` 라인 단얫이 존재한다.
3. TYPE 헤더 추가 후에도 `provider_rate_limit_total{provider="kubernetes"} 0` 라인 byte 불변 — compose_test.go:59·108·264가 무수정 통과로 증명.
4. Render가 `provider_api_latency_seconds_bucket` 라인을 le 오름차순(+Inf)으로, `_sum`·`_count`와 함께 내보낸다 — 골든 테스트 단얫.
5. 4개 client.go 각각 초이크 함수(aliyun `request`·k8s `doJSON`·proxmox `do`·tencent `do`)에 `ObserveAPILatency` 호출이 존재한다 — grep.
6. sync.go RunSync 확정점에 C/D/E 3종, reconcileAbsences의 stale_candidate 분기에 K 계기 호출이 존재한다 — grep.
7. terminal.go finishAttempt(F/G)·requeueForRetry(H)·cancelAtClaimBoundary(F), loops.go pollLoop의 QueueDepth Set(I), reaper.go의 lease_expired Inc(J)·소진 종단 발화(F/G)가 존재한다 — grep.
8. broker.go Resolve 성공 경로에 `secret_access_total`용 Inc가 존재하고 backend 라벨값이 `ref.Backend` 필드에서 온다 — 코드 검수.
9. main.go에 `engine.GET("/internal/metrics"` 등록이 존재하고, engine_config.go가 스택 실패 시 라우트 미등록(nil 가드)을 유지한다 — grep + main_metrics_test nil 단얫.
10. compose/healthloop.go가 존재하고 주기 Health 호출 결과를 `SetHealth(uid, healthy)`로 적립하며 API 변경이 없다(last_health_at은 성공 시만 갱신 — r2 승인) — 파일 + healthloop_test(fake healthy→1, 바인딩 부재→0).
11. 수정된 기존 테스트는 metrics_test.go 1개 파일뿐이며 그 내용은 TestRenderDeterministicAndSorted의 라인 수(2→3)·첫 라인(TYPE 헤더) 단얫 갱신이다 — `git diff --stat` + diff 본문.
12. go.mod·go.sum·docs/security/ 골든 2종은 무변경이다 — `git diff --name-only` 대상 제외 확인.

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

- compose_test.go:59·108·264의 `provider_rate_limit_total{provider="…"} 0` Contains 단얫과 sync_test.go:519 TestSyncReportCarriesMetricsRender는 수정하지 않는다 — 기존 2종 메트릭 라인 포맷은 byte 불변이어야 한다.
- metrics.Counters 타입명과 기존 공개 메서드(New·RegisterProviders·IncRateLimit·IncAPIError·Render) 시그니처는 불변이다 — 4 어댑터·compose·sync·fake의 기존 호출부는 무수정이어야 한다.
- metrics 패키지는 표준 라이브러리만 import한다(패키지 주석 계약 유지). go.mod·go.sum은 무변경이다.
- 라우트 골든은 무변경이다: docs/security/route-inventory.txt·sensitive-routes.txt는 재생성하지 않는다(엔드포인트는 router.New 밖 main.go에 등록한다). router.New 시그니처도 불변이다.
- 엔진 레인 부재 기동(R11 경로 — 스택 빌드 실패)에서 v1 라우트·동작은 불변이며 /internal/metrics는 미등록으로 유지한다.
- P4의 engine.go→terminal.go 이동은 byte 동등 순수 이동이다 — 함수 본문 로직 변경 없이 이동만 한다(계기 삽입은 P5에서).
- tasks 패키지 기존 9개 _test.go 파일은 무수정이다(신규 engine_metrics_test.go로 델타를 단얫한다).
- agent_heartbeat_age_seconds·outbox_backlog는 스펙 §18.2 deferred 그대로 계기하지 않는다.
- tasks 패키지의 계층 결계는 유지한다 — engine은 infra/metrics(표준 라이브러리 전용 리프)만 추가 import하며 inventory·audit은 여전히 import하지 않는다.

## 9. 가정 (불확실 요구사항의 명시적 처리)

- **A1**: "internal-only" = 인증 게이트가 아니라 배포 경계 시행(§18.2에 인증 문언 부재 실측). 스크래퍼 무인증 접근 전제.
- **A2**: resource_changes = created+updated 합산(tombstone·stale 미포함).
- **A3**: resource_stale_total은 stale_candidate 전환만 가산(tombstoned는 별도 미가산).
- **A4**: duration은 모든 종단 커밋(finishAttempt·cancelAtClaimBoundary·reaper 소진)이 {operation,status}로 발화하고 cancelAtClaimBoundary의 미실행 종단도 포함; failures는 failed·timed_out만; retries는 requeueForRetry만(reaper 재큐는 lease_expired 메트릭이 담당).
- **A5**: secret_access_total은 Resolve 성공만 가산(실패는 미가산).
- **A6**: health sweep은 broker "inventory" purpose 사용(실재 바인딩이 있는 유일 목적 — monitoring은 바인딩 부재로 전커넥션 unhealthy가 됨), 해석 실패 시 gauge 0, 주기는 상수 5분.
- **A7**: provider/op/kind/code 라벨값은 기존 IncAPIError 세계의 문자열 상수를 그대로 사용(신규 어휘 도입 없음).
- **A8**: 라우트 골든 미수록의 트레이드오프(§1.3) — route-inventory.txt는 router.New 조립 표면의 계약임을 유지하고 내부 엔드포인트는 본 계획 문서로 기록한다.

## 10. 미계기 2종 — deferred 확인문

`agent_heartbeat_age_seconds`(agents)·`outbox_backlog`(outbox) — §18.2 표가 둘 다 "deferred"로 명시(:1481-1482). 대응 하위시스템(에이전트·아웃박스)이 M1에 부재하므로 계기하지 않는다. 본 계획의 어떤 Phase도 이 2종에 라인을 추가하지 않는다(claim 1·2의 "14종"은 이 2종 제외 기준).
