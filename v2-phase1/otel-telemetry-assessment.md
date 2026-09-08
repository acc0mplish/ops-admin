# 멀티 인프라 텔레메트리 — OTel 공식 기준 실측 점검 (r1)

- 일자: 2026-09-08. 기준 트리: main `71d76de`. 모든 코드 근거는 file:line 실측.
- 요청: ① 멀티 인프라로부터 **OTEL로 받기** ② 우리가 **Grafana/모니터링 대시보드 쪽으로 OTEL 지원**.
- 외부 사실 출처: OTel Collector contrib receiver 디렉터리(공식 저장소 실조회), opentelemetry.io Prometheus 호환성 문서, Grafana OTLP ingest 문서. 벤더 블로그는 보조로만 사용하고 본문 결론 근거로 쓰지 않는다.

---

## 0. 결론 먼저 (BLUF)

1. **텔레메트리는 두 평면이고, 답이 서로 다르다.** 요청 문장은 둘을 한 덩어리로 부르고 있다.
   - **A평면 — 컨트롤 플레인 자기 텔레메트리**: §18.2 14종. **이미 구현 완료**(PR #48·#49). 저카디널리티, internal-only.
   - **B평면 — 관리 대상 리소스 텔레메트리**(PVE 게스트 CPU, 파드 재시작 등): **코드베이스에 존재하지 않는다.** 스펙에도 없다. 고카디널리티, 보존기간·소유자 자체가 다르다.
2. **"멀티 인프라로부터 OTEL로 받는다"는 성립하지 않는다 — 우리 4개 프로바이더 중 OTLP를 네이티브로 내보내는 것은 0개다.** OTLP 경계는 **Collector 하류**에 생긴다. 즉 텔레메트리 어댑터는 우리 Go 어댑터가 아니라 **OTel Collector/Alloy**다. (§2 실측)
3. **Grafana 쪽 OTEL 지원은 M1에서 코드 0줄·의존 0개로 이미 가능하다.** `prometheusreceiver`가 `/internal/metrics`를 스크랩 → OTLP로 변환 → Mimir/Tempo. 이게 Prometheus exposition 서비스에 대한 **OTel 공식 권장 경로**다. §10.2 "No new runtime dependencies in M1"(:976)을 문언 그대로 지킨다. (§3)
4. **남은 구멍은 메트릭이 아니라 트레이스와 로그에 있다.** ① 트레이스 컨텍스트가 **아웃바운드로 전파되지 않는다**(어댑터 호출에 `traceparent` 부착 grep 0건) — 프로바이더 측과의 상관관계 불가. ② `traceparent` 부재 시 `randomHex(16)` 대체값(audit.go:106-107)은 외부 트레이스와 이어지지 않는 ID다(의도된 설계 — :107 "presence is the assertion"). ③ **§18.3 필수 로그 필드가 하나도 발행되지 않는다** — 로깅은 `gin.Logger()` 단일(router.go:44). LGTM의 L(Loki) 다리가 비어 있다. (§4)
5. **블로킹 결정 1건**: 토폴로지 A/B/C 중 무엇인가 — 그리고 B를 고르면 §18.2 "Prometheus text exposition" 계약과 §10.2 의존 금지를 **스펙 개정**해야 한다. 이건 사용자 판정 사항이다. (§7)

---

## 1. 현재 위치 실측

### 1.1 A평면은 완료 상태
- `backend/internal/infra/metrics/` — `metrics.go`(185) `histogram.go`(163) `gauge.go`(63) `render.go`(277). §18.2 M1 14종 전부 + 표준 Prometheus text exposition(TYPE 헤더·버킷·_sum/_count).
- **순도 계약**: `metrics.go:8-10` — "this package imports the standard library only". 어댑터·contracttest 양쪽에서 import 가능해야 하므로 유지되는 제약.
- 엔드포인트: `backend/main.go:31` `engine.GET("/internal/metrics", …)`. 엔진 레인 실패 시 미등록(`engine_config.go:132`).
- 인증 없음 — 스펙 §18.2(:1463) "internal-only"는 **배포 경계(망 분리·리버스 프록시 비노출)로 시행**한다는 가정(metrics-plan §1.3 A1). 스크래퍼는 이 신뢰 경계 **안**에 있어야 한다.

### 1.2 B평면은 존재하지 않는다
- 어댑터 계약 전수: `internal/infra/contract/adapter.go:9-30` — `BaseAdapter`·`Discoverer`·`OperationExecutor`·`TaskPoller`·`TaskCanceller`. **텔레메트리/메트릭 능력 인터페이스 없음.** 스펙 §11도 동일(`EventSubscriber`는 r1에서 제거됨, §7.7).
- 어댑터는 요청/응답 + 커서 페이징 모델이고 HTTP 클라이언트는 per-request stateless다(`contract/adapter.go` PollRequest 주석). **텔레메트리는 스트리밍/푸시 모델** — `Discoverer`나 신규 어댑터 능력에 억지로 끼워 넣으면 안 된다.
- 즉 B평면 도입은 "어댑터에 메서드 추가"가 아니라 **새 아키텍처 결정(ADR)** 이다.

---

## 2. 수신(ingest) 실측 — 프로바이더별로 OTEL 경로가 다르다

OTel Collector contrib `receiver/` 디렉터리 실조회 결과(2026-09-08):

| 프로바이더 | contrib 전용 receiver | 실제 OTEL 경로 | 성숙도 |
|---|---|---|---|
| **Kubernetes** | ✅ `k8sclusterreceiver`, `kubeletstatsreceiver`, `hostmetricsreceiver` | Collector가 API server/kubelet을 **풀**해서 OTLP를 **생산**. k8s 자체는 OTLP를 말하지 않는다 | 공식·성숙 |
| **Proxmox VE** | ❌ **없음** (proxmox/libvirt/vsphere 전부 부재) | ① `prometheus-pve-exporter`(커뮤니티) → `prometheusreceiver`, 또는 ② PVE 내장 external metric server(InfluxDB/Graphite 프로토콜) → `influxdbreceiver`(contrib 존재) | 커뮤니티 의존 — ①은 서드파티 exporter 신뢰·운영 부담, ②는 PVE 네이티브지만 메트릭 셋이 노드/게스트 기본치로 제한 |
| **Aliyun / Tencent** | ❌ receiver 없음 (있는 건 `aliyunlogservice`·`tencentcloudlogservice` **exporter** — 방향이 반대) | 클라우드 모니터 API 폴러 자작 필요 | **범위 제외** — RESUME.md 2026-09-08 사용자 확정("신규 투자 없음"). 설계 예산 배정하지 않음 |

**따라서**: "멀티 인프라로부터 OTEL로 받는다"의 실체는 **k8s만 공식 경로가 있고, PVE는 어댑터(exporter) 한 겹을 우리가 운영해야 하며, 클라우드 2종은 범위 밖**이다. 어느 경우든 OTLP를 만드는 주체는 Collector이지 ops-admin이 아니다.

---

## 3. 송출(egress) — Grafana 방향

### 3.1 M1 권장: 코드 0줄·의존 0개
```
ops-admin /internal/metrics  ──scrape──▶  Collector|Alloy(prometheusreceiver)
                                              │ batch + memory_limiter
                                              ▼ otlp exporter
                                          Mimir (metrics)  ─▶ Grafana
```
- OTel 공식 문서가 Prometheus exposition 서비스에 대해 규정한 경로 그대로다.
- **파싱 요건 충족 실측** — 골든 `internal/infra/metrics/metrics_test.go:118-218`(14종 전문 byte 비교)에서 확인:
  - counter 패밀리 **전부 `_total` 접미사**(`provider_api_errors_total`:150, `provider_rate_limit_total`:152, `inventory_sync_resource_changes_total`:171, `inventory_sync_partial_total`:173, `provider_task_failures_total`:206, `provider_task_retries_total`:208, `worker_lease_expired_total`:212, `resource_stale_total`:214, `secret_access_total`:217). `prometheusreceiver`→OTLP의 counter/gauge 판정이 이름 접미사에 걸리므로 이게 핵심 요건이다.
  - `# TYPE` 헤더 **패밀리당 1회·중복 없음·값 라인 직전** 배치(:118·:121·:150·:152·:155·:171·:173·:175·:206·:208·:210·:212·:214·:217).
  - histogram은 `_bucket{…,le=…}` **le 오름차순 + `+Inf` + `_sum` + `_count`** 완비(:121-154 구간 실물).
- **한계 — exemplar 없음**: 메트릭↔트레이스 클릭 네비게이션(Grafana에서 지연 스파이크 → 해당 트레이스 점프)은 **exemplar**에 의존하고 exemplar는 OpenMetrics 포맷을 요구한다. 우리 렌더러는 평문 text exposition이고 `metrics.go:8-10` stdlib 순도 계약상 exemplar 지원은 수작업 추가다. **A안은 Mimir 메트릭 + (추후) Tempo 트레이스를 주지만, 둘 사이의 exemplar 링크 항해는 별도 작업 없이는 얻지 못한다.**
- §10.2(:976) "No new runtime dependencies in M1" 위반 없음. §18.2 렌더 골든 테스트(byte 비교) 무영향.
- Grafana Agent는 지원 종료 — **Alloy** 사용. (Grafana 공식 마이그레이션 고지)
- 운영 조건: 스크래퍼가 internal 신뢰 경계 안에 있어야 한다(§1.1). Collector를 같은 네트워크 세그먼트에 배치하고 `/internal/metrics`는 외부 미노출 유지.

### 3.2 네이티브 OTLP push는 M2+ 결정 사항
- 우리 `Counters`는 손으로 쓴 stdlib 텍스트 렌더러다. **`go.opentelemetry.io/contrib/bridges/prometheus`는 적용되지 않는다** — 그 브리지는 client_golang의 `prometheus.Gatherer`/레지스트리를 소비한다. 우리는 Gatherer도 client_golang 계기도 제공하지 않는다.
- 네이티브 OTLP push 경로는 둘뿐이고 **둘 다 §10.2를 깬다**: (a) client_golang 채택 후 브리지, (b) OTel Go SDK 계기로 14종을 재작성.
- 도입 시 보안 고지: **GHSA-hfvc-g4fc-pqhx / CVE-2026-39883** — `go.opentelemetry.io/otel/sdk` `>= v1.15.0, <= v1.42.0` 영향, **v1.43.0에서 수정**. BSD/Solaris에서 `kenv`를 절대경로 없이 호출해 PATH 하이재킹 → 리소스 초기화 시점 임의 코드 실행(High). 우리 배포는 Linux지만 **버전 하한을 v1.43.0으로 고정**하는 것이 옳다(GHSA 원문 확인 완료 — 2차 집계 사이트가 아닌 공식 advisory 기준).

---

## 4. 트레이스·로그 실측 — 여기가 실제 구멍

### 4.1 trace_id는 "위반"이 아니라 "조인" 상태

| 지점 | 실측 | 판정 |
|---|---|---|
| API 엣지 | `internal/api/v2/audit.go:104` 인바운드 `traceparent` 파싱 → :106-107 부재 시 `randomHex(16)` 대체 | 형식 충족. 단 대체값은 **외부 트레이스와 이어지지 않는 ID** — :107 주석대로 "존재 자체가 단얫"인 의도된 설계지, 오류 아님 |
| 감사 행(요청) | audit.go:110-111 `request_id`·`trace_id` + :123 `task_uid` | ✅ |
| 감사 행(태스크 종단) | `RecordTaskTerminal`(audit.go:163) contextMap(:171-179) — `trace_id`·`request_id`·`actor_id` 없음. 단 **`task_uid`는 있다**(:172) | ⚠️ **스펙 위반 아님.** §18.1 헤더는 "Minimum fields for V2 **operations**" — 요청 행이 그 레코드다. 종단 행은 `Method:"TASK"`·`task://` 경로 공간으로 **의도적으로 구분된 별개 행 클래스**(:160-162). `actor_id` 부재도 같은 이유(엔진이 쓰는 행에는 HTTP 행위자가 없다) |
| 어댑터 호출(아웃바운드) | `traceparent` 부착 grep **0건** | ❌ **실제 결함.** 프로바이더 측 상관관계 불가 — 트레이스가 우리 프로세스 경계에서 끊긴다 |

- 따라서 종단 행의 trace_id 부재는 **`task_uid` 조인 한 번으로 복구 가능**하다(요청 행 ← task_uid → 종단 행). 스펙 상환 과제가 아니다.
- 그럼에도 종단 행에 `trace_id`를 실을 값어치는 있다: **조인 제거 + 비동기 경계를 넘는 span parenting의 전제조건**. OTLP 트레이스를 언젠가 도입한다면 이게 선행 조건이지, 지금 당장의 위반 시정은 아니다.
- **컨트롤 플레인에서 OTEL의 최고 가치는 메트릭이 아니라 트레이스다.** `submit → claim → Execute → Poll → terminal`은 교과서적 분산 트레이스이고 `task_uid`/`attempt_no`가 이미 자연스러운 span 신원이다. 아웃바운드 전파 부재(위 4행)가 그 서사의 실제 구멍이다.

### 4.2 §18.3 필수 로그 필드 — 발행 0건

스펙 §18.3이 요구하는 로그 필드: `request_id`, `trace_id`, `provider_connection_uid`, `provider_context_uid`, `resource_uid`, `task_uid`, `operation`, `attempt_no`, `error_code`.

- 실측: 전역 로깅 미들웨어는 `router/router.go:44` `gin.Logger()` **단 하나**. 구조화 로거·필드 주입 없음. 위 9개 필드 중 **로그로 나가는 것은 0개**다(`trace_id` grep은 `audit.go`에만 적중 — 감사 DB 행이지 로그가 아니다).
- 영향: 우리가 LGTM을 권고하는데 **Loki 다리에 실을 것이 없다**. 감사 행은 DB에 있고 로그는 필드가 없어서, "로그 → 트레이스 → 메트릭" 삼각 항해가 성립하지 않는다.
- r1 범위 판정: **로그 파이프라인은 본 문서 범위 밖으로 두되, 구멍임을 명시**한다. §18.3 이행(구조화 로거 도입 + 필드 주입)은 OTEL 토폴로지 결정과 독립적인 별도 과제이며, 결정되면 Alloy `loki.*` 경로는 코드 0줄로 붙는다.

## 5. 토폴로지 선택지

| | 수신 경로 | 신규 의존 | 충돌 | 카디널리티 소유 |
|---|---|---|---|---|
| **A. 우회 + 상관관계** (권장) | Collector/Alloy → LGTM 직결. ops-admin은 **리소스 UID ↔ semconv 매핑**과 Grafana 딥링크만 제공 | **0** | 없음 | Mimir가 고카디널리티 시리즈 소유, `/internal/metrics`는 저카디널리티 유지 |
| **B. 패스스루** (요청 문장의 액면 해석) | ops-admin이 OTLP receiver 운용 → 보강 → 재송출 | otel SDK + receiver | **ADR 0001 모듈러 모놀리스**, arch rule 2, §10.2, §18.2 렌더 계약 | ops-admin이 전량 통과 — 14종 × 리소스 수만큼 폭증 |
| **C. 쿼리 프록시** | ops-admin이 Mimir에 PromQL 질의해 자체 UI에 렌더 | HTTP 클라이언트만 | 없음 | Mimir |

**권장: A(+필요 시 C를 UI용으로 병행).** 근거는 셋:
1. 어댑터 계약이 요청/응답·stateless인데(§1.2) B는 스트리밍 수집기를 모놀리스 안에 이식한다 — 별도 ADR 없이는 arch rule 위반.
2. B는 §18.2·§10.2 두 계약을 동시에 개정해야 한다. 얻는 것(보강된 리소스 속성)은 A에서 **매핑 테이블 하나**로 얻을 수 있다.
3. 카디널리티: 현재 라벨은 connection/operation 스코프라 시리즈가 수십 개다. 리소스별 라벨이 붙으면 수천 × 14종. 고카디널리티는 Mimir가 소유해야 한다. (contrib의 Cardinality Guardian 프로세서는 아직 development 단계라 방어선으로 삼을 수 없다.)

---

## 6. A안을 실제로 성립시키는 산출물 — semconv 매핑 (이게 "OTEL 공식"의 실질)

리소스 UID로 Grafana에서 Mimir 시리즈를 조인하려면 **정규화 kind → OTel semantic conventions** 계층이 필요하다. 어댑터별 `mapping.md`는 provider→normalized까지만 덮는다. 빠진 층은 이것이다:

| M1 kind (`contract/resource_kind.go:4-24`) | OTel semconv 속성 (registry 그룹) |
|---|---|
| `orchestration.cluster` | `k8s.cluster.name`, `k8s.cluster.uid` |
| `orchestration.node` | `k8s.node.name`, `k8s.node.uid` |
| `orchestration.namespace` | `k8s.namespace.name` |
| `orchestration.pod` | `k8s.pod.name`, `k8s.pod.uid` |
| `orchestration.workload` | `k8s.deployment.name` / `k8s.statefulset.name` / `k8s.daemonset.name` (subtype로 분기) |
| `compute.hypervisor_node` | `host.id`, `host.name` (PVE 노드) |
| `compute.vm` / `compute.system_container` | `host.id` 또는 `vm.*` — PVE의 QEMU/LXC는 semconv에 전용 그룹이 없다. exporter 라벨(`vmid`,`node`)을 우리 쪽에서 `host.id`로 정규화하는 규칙을 명시해야 함 |
| `identity.account` | `cloud.provider`, `cloud.account.id`, `cloud.region` (범위 제외 — 표에만 남김) |
| (전 kind 공통) | 우리 UID는 semconv에 대응 없음 → 사설 네임스페이스 `opsadmin.resource.uid` / `opsadmin.connection.uid`. 예약 네임스페이스 침범 금지 |

주의: `k8s.*`·`host.*`·`cloud.*`의 안정성 등급은 semconv 릴리스마다 다르다. **고정한 semconv 버전의 stability 마커를 확인하고 그 버전을 명시 고정**할 것 — 여기서 "stable"이라고 단정하지 않는다.

---

## 7. 보안 제약 (도입 시 필수)

- **`ConnectionView.Material`은 평문이고 "로그·직렬화 금지" 계약이 붙어 있다**(`contract/adapter.go` Material 주석, 보존 제약 #7). **span 속성과 span 오류 메시지는 직렬화 채널이다.** 어댑터 호출에 트레이싱을 두르는 순간 이 계약이 깨질 수 있으므로, 트레이싱 도입 시 **명시적 리댁션 규칙**을 계약에 추가해야 한다(속성 allowlist 방식 권장 — denylist는 새 필드가 추가될 때 조용히 새어나간다).
- `/internal/metrics`는 무인증이다. Collector를 붙인다는 것은 **스크래퍼가 신뢰 경계 안에 있다는 배포 가정**을 실행에 옮기는 것 — 런북에 경계 조건을 명문화해야 한다.
- 프로바이더 원시 에러는 §18.1상 restricted diagnostics 전용이다. OTLP로 내보내는 span status/message에 원시 에러를 그대로 실으면 이 규정 위반이다.

---

## 8. 권고 실행 순서

1. **(의존 0, OTEL 무관)** `trace_id`/`request_id`를 태스크 제출 시 지속 → `RecordTaskTerminal` echo. **스펙 위반 시정이 아니라** 조인 제거 + 비동기 경계 span parenting의 전제조건. 우선순위는 아래 2·3보다 낮다. — §4.1
2. **(코드 0, M1)** Alloy/Collector `prometheusreceiver` → `otlp` → Mimir 배포 레시피를 런북에 추가. "Grafana 쪽 OTEL 지원" 요구는 여기서 충족된다. — §3.1
3. **(문서, M1)** §6 semconv 매핑 테이블을 스펙 §18에 신설 절로 착지. A안이 실제로 동작하게 만드는 핵심 산출물.
4. **(M2, 결정 후)** k8s는 `k8sclusterreceiver`+`kubeletstatsreceiver`, PVE는 exporter 경로 중 택1로 B평면 수집을 **Collector 측에** 구성. ops-admin 코드 무변경.
5. **(M2+, 스펙 개정 필요)** OTLP 트레이스 네이티브 송출 — §7 리댁션 규칙 + §10.2 개정 + `otel/sdk` ≥ v1.43.0 고정이 전제. 선행으로 **아웃바운드 `traceparent` 전파**(§4.1 4행)가 필요하다.
6. **(별도 과제, 범위 밖)** §18.3 구조화 로그 필드 이행 — 결정되면 Alloy `loki.*` 경로는 코드 0줄로 붙는다. — §4.2

---

## 9. 블로킹 질문 (사용자 판정)

1. **토폴로지 A / B / C 중 무엇인가?** A면 위 순서대로 진행 가능하고 스펙 개정이 필요 없다. B면 §18.2·§10.2 개정 + 신규 ADR이 선행이다.
2. **§10.2 "No new runtime dependencies in M1"을 개정하는가?** 네이티브 OTLP push(메트릭이든 트레이스든)는 전부 이 조항에 걸린다. 유지하면 M1은 Collector 스크랩 경로로 확정된다.
3. B평면 수집 대상 범위 — **k8s만인가, PVE 포함인가?** PVE 포함이면 커뮤니티 exporter 운영 부담(신뢰·패치·가용성)을 인수하는 결정이다.
