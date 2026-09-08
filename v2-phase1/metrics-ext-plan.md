# V2 §18.2 메트릭 확장 계획 — 대시보드 구멍 3건 상환 (r2)

- **r2 개정 사유**: ②적대검토 3렌즈 반려(`docs/task-id/otel-m18-ext/gaps-r1.md` — CRITICAL 0 / HIGH 12 / MEDIUM 11 / LOW 8). 상환 대조는 부록 §14.
- **부록**: `v2-phase1/metrics-ext-plan-evidence.md` — §1 현재 코드 실측 / §4 파괴 범위 전수(골든 diff 문언 포함) / §14 갭 상환 대조표. 본문이 650줄 한도를 넘어 증거층을 분리했다. 본문에서 "§1.x"·"§4.x"로 인용하는 것은 전부 부록의 절이다.
- 원천 계약: `docs/architecture/multi-infrastructure-control-plane-v2.md` **§18.2 "Control-plane metrics — instrumented where"**. **인용은 절 제목 기준이 원칙**이다(§3.0) — 병행 과제 `otel-lgtm-recipe`가 같은 파일에 §18.5를 신설하므로 행 좌표는 착지 순서에 따라 어긋난다. 착수 시점 실측 앵커(2026-09-08, main `30856a3`): 절 헤더 `:1461`, 표 헤더 `:1465`, 구분행 `:1466`, 데이터 16행 `:1467-1482`, 후행 문장 `:1484`.
- 승계: `v2-phase1/metrics-plan.md`(§18.2 M1 14종 계측 — 구현 완료, PR #48·#49), `v2-phase1/otel-telemetry-assessment.md`(OTEL 토폴로지 A안 확정), `v2-phase1/otel-lgtm-recipe-plan.md`(병행 과제 — 스크랩·대시보드).
- 실측 기준 트리: main `30856a3` 작업 트리, 2026-09-08 전수 확인. **모든 주장에 file:line**. 실측 없는 추정은 §12 가정으로만 기재한다.
- 범위: 메트릭 3종 확장(라벨 1건 개정 + 패밀리 2종 신설) + 스펙 §18.2 표 개정 + 부팅 복원 회로 1개 + 소거 회로 1개. **스펙에 없는 범위 확장 없음.**

---

## 0. 결론 먼저 (BLUF)

### 0.1 제1원칙 — 이 계획의 산출물은 알람이다

세 확장 전부 최종 소비 형태가 Grafana 알람/패널이다. **알람의 실패 모드는 "터지는 것"이 아니라 "조용히 죽는 것"이다.** r1이 반려된 HIGH 12건 중 4건이 정확히 그 실패였다:

| 실패 | 증상 | 상환 |
|---|---|---|
| zero time → `itoa(uint64(-62135596800))` = **`18446744011573954816`** | `time()-gauge`가 거대 음수 → 알람 **영구 무력화** | §2.3.1 setter zero 가드 |
| 커넥션 삭제 후 gauge 잔존 | `time()-gauge` **영구 오발** + 맵 무한 증가 | §2.3.4 소거 회로 |
| P3b만 착지(복원 없음) | 재시작마다 시리즈 소실 → **조용히 빈칸** | §6·§9.2 forward-stop |
| 복원이 라이브 값을 덮음 | 부팅 직후 값 역행 | §2.3.1 단조 갱신 |

따라서 **본 계획은 각 패밀리마다 "이 시리즈가 잘못됐을 때 무엇이 그것을 알려주는가"를 계약으로 적는다** — §2.4 관측 계약 표가 그 층이고, §8 테스트와 §10 claims가 그 표를 이행한다. "기존 테스트 무수정 통과 = 검증"은 **안전망으로 신뢰하지 않는다**: r1이 그렇게 믿었던 `healthloop_test.go:191`은 부정 단얫이라 prune이 완전히 고장나도 통과한다(§4.2 H-7).

### 0.2 설계 결론

1. **동기화 성공/실패율은 히스토그램 라벨이 아니라 신규 카운터 `inventory_sync_total{connection,mode,status}`로 푼다.** 결정적 근거는 카디널리티가 아니라 **표현 불가능성** — `RunSync` 사전 단계 실패(`sync.go:70-85`)는 run 행조차 만들지 못하므로 관측할 duration이 없다. (a)안은 이 실패를 원리적으로 셀 수 없다. (§2.1)
2. **사각지대는 "범위 밖"이 아니라 계측 대상이다.** `RunSync`의 return 문 8개(`:72 :76 :80 :84 :93 :105 :134 :150`) 중 7개가 현재 무계기다. 8개 전부에 1회씩 발화해 "RunSync 호출 1회 = 증분 정확히 1" 불변식을 만든다. 검증은 `grep -c`가 아니라 **경로별 단얫**이다(§8 T-3, claim 6). (§2.1.3)
3. **`provider_health`에 `provider` 라벨을 append 한다** — `{connection, provider}`. provider_type은 sweep 루프가 이미 순회하는 행에 있다(`healthloop.go:130` → `:179` `conn.ProviderType`). **순서 선정 근거는 r1의 "기존 단얫 보존"이 아니다** — 그 논거는 거짓으로 밝혀졌다(§2.2.1). 근거는 스펙 표의 라벨 열 표기 순서와 일치시키는 것뿐이다. (§2.2)
4. **`inventory_sync_last_success_timestamp_seconds{connection}` gauge를 신설하고, 부팅 시 복원하며, 커넥션 소멸 시 소거한다.** setter는 **단조(monotonic) + zero 가드**를 자체 규정으로 갖는다 — 호출 위치 제약보다 견고하다(§2.3.1). 소거는 `pruneVanished`와 대칭이다(§2.3.4).
5. **본 과제의 유일한 비-추가형 변경은 `provider_health` 라벨 1건이다.** 그리고 이 1건은 **`otel-lgtm-recipe` P2(Alloy 스크랩 개시)보다 반드시 먼저 착지해야 한다** — 지금은 `deploy/alloy/` 미존재(실측: `ls deploy/` = `config.yaml.example` 단일)라 전환 비용 0이지만, 스크랩 개시 후의 TSDB 고아 시리즈는 코드 revert로 회수 불가다. (§9.1 R1)
6. **P3b(라이브 발화)와 P4(복원)는 분리 착지 금지** — 같은 PR 또는 연속. 중간 상태가 착지 전보다 나쁘다(§9.2 forward-stop 표).
7. **깨지는 단얫은 3파일이고, 그중 1건은 "통과하지만 검출력을 잃는" 종류다** — `healthloop_test.go:191`. 이런 단얫은 "무수정 통과"로 처리하지 않고 **적극 갱신**한다(§4.2).

---

## 1. 현재 코드 실측 → **부록 §1**

메트릭 패키지 4파일 현황, `RunSync` 8개 return 전수, `provider_health` 계기 현황, 타임스탬프 원장 실측은 분량상 부록으로 분리했다 — **`v2-phase1/metrics-ext-plan-evidence.md` §1**. 본문 §2 이하는 필요한 file:line을 인용 지점마다 다시 적으므로 부록 없이도 따라갈 수 있다.

---

## 2. 설계 판정 (선택지 비교 → 권고)

### 2.1 확장 ①: 동기화 성공/실패율

#### 2.1.1 선택지 비교
| 기준 | (a) duration에 `status` 라벨 | (b) `inventory_sync_total{connection,mode,status}` 신설 | (c) 둘 다 |
|---|---|---|---|
| **사전 단계 실패 표현** | **불가능** — run이 없어 duration이 없다. 0초를 관측하면 히스토그램 분포를 오염시킨다 | 가능 — 카운터는 duration을 요구하지 않는다 | 가능(b 덕분) |
| **PromQL 성공률** | `count`만 골라 나눠야 함: `sum(rate(..._count{status="succeeded"}[1h])) / sum(rate(..._count[1h]))` — `_count` 접미사를 성공률에 쓰는 건 관용에서 벗어난다 | 교과서형: `sum(rate(inventory_sync_total{status="succeeded"}[1h])) / sum(rate(inventory_sync_total[1h]))` | b와 동일 |
| **시계열 증가** | 기존 duration 시리즈 × status수(≤3). 시리즈당 렌더 라인 15줄 → **라인 수 3배**(§5) | 커넥션당 ≤9 시리즈 × 1줄 | a+b 합 |
| **기존 골든 파괴** | `metrics_test.go` 골든 15줄 재작성 + `ObserveSyncDuration` 시그니처 변경 + `sync_metrics_test.go:54,81` 라벨 문자열 2곳 | 골든에 **블록 삽입만**(기존 라인 byte 불변) | a의 파괴 전부 |
| **스펙 개정** | 기존 행의 라벨 열 수정(계약 변경) | 신규 행 추가(계약 확장) | 둘 다 |
| **`inventory_sync_partial_total` 중복** | 없음 | `{status="partial"}`과 의미 중복 — 그러나 기존 행은 §18.2 계약이므로 유지 | 없음 |

#### 2.1.2 권고: (b) 단독
`inventory_sync_total{connection,mode,status}` counter를 신설한다. (a)를 배제하는 결정적 사유는 위 표 1행 — **요구사항 "실패 run을 세는 카운터가 없다"를 (a)는 원리적으로 충족할 수 없다.** 카디널리티·골든 파괴 최소화는 같은 방향의 보조 근거다. `inventory_sync_duration_seconds`의 라벨은 `{connection, mode}` 그대로 둔다.

중복 판정: `inventory_sync_partial_total`은 `inventory_sync_total{status="partial"}`로 유도 가능해진다. 그럼에도 **제거하지 않는다** — §18.2 표 `:1473`의 M1 계약 행이고, 보존 제약상 기존 라인 형태는 계약이다. 신규 대시보드는 `inventory_sync_total`을 쓰도록 권고만 문서화한다.

#### 2.1.3 사각지대 판정: 8개 return 전부 계측
"어떤 계기도 발화하지 않는" 사전 단계(§1.2 ①-④)를 **명시적 범위 밖으로 두지 않는다.** 이유:
- `mode`와 `ConnectionUID`가 전부 확보된다(`:65-68`이 첫 return보다 앞).
- 사전 단계 실패는 운영자 관점에서 실패한 동기화다 — 성공률 분모에서 빠지면 지표가 낙관 편향된다.
- 확장 ①의 목적 자체가 "실패 run을 세는 것"인데, 가장 조용한 실패를 빼면 목적 미달이다.

동시에 ⑤⑥⑦(run 행 생성/조정/확정 실패)도 같은 이유로 포함한다. 결과로 **불변식**을 얻는다:

> `RunSync` 호출 1회 = `inventory_sync_total` 증분 정확히 1.

이 불변식은 테스트로 반증 가능하고(§8 T-3), 스펙 문장으로 못 박는다(§3.2).

**상태 매핑** (r2 정정 — H-1). r1은 ⑧의 값역을 `succeeded|partial` 2종으로 적었으나 **거짓**이다. `consumePages`는 `RunStatusFailed`를 **6갈래**로 반환한다 — `provider_rate_limited`(`sync.go:238`), `provider_permission_denied`(`:243`), `provider_unreachable`(`:247`), `discover_failed`(`:249`), `cursor_persist_failed`(`:266`), `relationship_failed`(`:275`). 이 값은 `:137`에서 `report.Status`로 그대로 들어가므로 ⑧도 `failed`를 낸다.

| 지점 | 발화 status | 값역 |
|---|---|---|
| ①-⑦ (사전 단계·행 생성·조정·확정 실패) | 상수 `RunStatusFailed` | `failed` |
| ⑧ (정상 종단) | `status`(= `report.Status`, `:137`) | **`succeeded` \| `partial` \| `failed`** — 3종 전부 |

- 따라서 **`status="failed"`는 사전 단계 실패의 전유물이 아니다.** `failed`의 지배적 발생원은 오히려 ⑧의 6갈래(프로바이더 오류·커서 지속 실패·관계 파생 실패)다. §3.2의 스펙 문장이 이 사실을 반영해야 한다.
- `running`(`reconcile.go:45`)은 종단이 아니므로 절대 발화하지 않는다 — ⑧이 `running`을 낼 수 있는 경로는 없다(`consumePages`의 반환 3종에 없음).
- **어휘 상한은 여전히 3종**이다(§5 카디널리티 산정 불변) — 6갈래는 전부 `failed`로 접힌다. 갈래 구분은 `provider_task_failures_total{code}`가 하는 일이고, sync 쪽에는 그에 상응하는 `error_code` 라벨을 **도입하지 않는다**(카디널리티 대비 이득 없음, 스펙 표 라벨 열 밖).
- **L-6 비대칭 기록**: ⑥(`reconcileAbsences` 실패)·⑦(확정 `Updates` 실패)에서 메트릭은 `failed`를 세지만 DB 원장의 `inventory_sync_run.status` 행은 `running`으로 남는다(⑥은 `:133` 갱신 이전, ⑦은 갱신 자체가 실패). 이는 **의도된 비대칭**이다 — 메트릭은 "운영자 관점의 run 결과"를, 원장은 "DB에 확정된 사실"을 말한다. 복원 질의(§2.3.3)는 원장을 읽으므로 이 두 행을 성공으로 오인하지 않는다(`status='succeeded'` 술어). 계획은 이를 은폐하지 않고 가정 A9로 기재한다.

**구현 형상 판정**: `defer` + named return으로 한 곳에서 처리하는 안을 **기각**한다 — defer는 `:148`의 `report.MetricsText = r.counters.Render()`보다 **뒤에** 실행되므로 CLI 산출물(`main_sync.go:35,106`)이 그 run의 증분을 담지 못한다(자기 불일치). 대신 nil 가드를 품은 사설 헬퍼 1개를 만들고 8곳에서 1줄씩 호출한다 — 제어 흐름 무변경(보존 제약 3):

```go
// countRun — §18.2 inventory_sync_total: RunSync의 모든 종단 경로가 정확히
// 1회 호출한다(사전 단계 실패 포함). nil Counters는 무동작.
func (r *SyncRunner) countRun(connection, mode, status string) {
	if r.counters == nil {
		return
	}
	r.counters.IncSyncRun(connection, mode, status)
}
```
**8곳 전부 `r.countRun(...)`를 호출한다 — ⑧도 예외가 아니다.** ⑧은 기존 `if r.counters != nil` 블록(`:139-149`) **안**, `Render()`(`:148`)보다 앞에 놓이므로 nil 검사가 중복되지만, 그 중복은 무해하고 "8개 호출"이라는 기계적 검증 가능성(claim 6·7)이 그 값어치를 넘는다. 블록 안에서 `r.counters.IncSyncRun(...)`을 직접 부르지 **않는다**.

### 2.2 확장 ②: `provider_health`에 provider 라벨

#### 2.2.1 라벨 순서 — `{connection, provider}` (r2: 근거 재작성 — H-7)
**r1의 논거는 거짓이었다.** r1은 "`provider_health{connection="X"`가 접두로 보존되므로 `healthloop_test.go:191`이 무수정 통과한다"고 적었으나, `:191`의 리터럴은 `provider_health{connection="conn-vanish"}`로 **닫는 중괄호까지 포함**한다. 라벨을 append 하면 렌더 라인은 `…conn-vanish",provider="fake"}`가 되어 그 리터럴은 부분 문자열이 아니다. 접두 논거는 성립하지 않는다.

더 나쁜 사실: `:191`은 `if strings.Contains(...) { t.Fatalf(...) }` 형태의 **부정 단얫**이다. 리터럴이 절대 매치되지 않게 되면 **prune 회로가 완전히 고장나도 이 테스트는 통과**한다 — 회귀 검출력이 0이 된다. r1은 이것을 "안 깨지는 단얫" 목록에 넣었는데, 정확히 반대 처분이 필요하다(§4.2).

**순서 자체는 여전히 `{connection, provider}`로 간다.** 남은 근거는 하나뿐이고, 그것으로 충분하다 — **스펙 §18.2 표의 라벨 열 표기 순서(`connection, provider`)와 렌더 라벨 순서를 일치시킨다.** 이는 기존 패밀리 전수의 관례다: `provider_api_errors_total{provider,op,code}`(`render.go:88-92`) ↔ 스펙 표 `provider, op, code`. Prometheus는 라벨 순서에 의미를 두지 않으므로 다른 비용은 없고, 두 순서 중 어느 쪽도 기존 단얫을 구제하지 못한다(둘 다 `:191`과 `healthLine`을 깬다).

#### 2.2.2 저장 형상 — 맵 키는 connection 유지, 값에 provider 동봉
```go
// healthSample — provider_health{connection,provider} 한 시리즈.
type healthSample struct {
	provider string
	healthy  bool
}
// metrics.go:77 교체
health map[string]healthSample // provider_health{connection,provider} 0/1
```
키를 `{connection, provider}` 복합으로 바꾸지 **않는** 이유: `RemoveHealth(connection)`(`gauge.go:19-23`)의 시그니처와 `pruneVanished`(`healthloop.go:158-168`)가 그대로 살고, 정렬 키(`gauge.go:44` `sort.Strings`)도 그대로다. 커넥션 UID는 유일하므로 복합 키는 정보를 더하지 않는다.

`markObservedUnhealthy`(`healthloop.go:207-213`) 해소는 **`observed []string`을 pair로 바꾸지 않고** metrics 쪽에 한 메서드를 더한다:
```go
// SetHealthUnhealthy flips a known connection's health to 0 while keeping its
// provider label. 미관측 커넥션은 provider를 알 수 없으므로 무동작이다.
func (c *Counters) SetHealthUnhealthy(connection string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.health[connection]; ok {
		s.healthy = false
		c.health[connection] = s
	}
}
```
이렇게 하면 `healthloop.go`의 **본 확장분 diff는 2줄**(`:133` 시그니처, `:211` 메서드 교체)로 끝나고, `observed []string`·`swapObserved`·`setObserved`(죽은 코드)는 무변경, `pruneVanished`는 **시그니처와 판정 로직이 무변경**이다. (`pruneVanished`의 본문에는 §2.3.4가 소거 1줄과 doc comment를 더한다 — 그것은 확장 ③ 소관이며 P3b에서 착지한다. 파일 전체 diff는 2줄이 아니라 **2줄 + 1줄 + 주석**이다.) `TestHealthSweepDBFailureMarksObservedZero`(`healthloop_test.go:125-145`)는 1회차 sweep이 엔트리를 만든 뒤 2회차에서 뒤집으므로 provider 라벨이 보존되어 통과한다.

#### 2.2.3 라벨값 어원
`conn.ProviderType`(DB 컬럼 `provider_type`) 그대로다. 기존 `provider_*` 패밀리의 provider 라벨값(어댑터 `ProviderName()` — 예 `fake/fake.go:96,154`)과 같은 어휘 공간이다. 신규 어휘 도입 없음(가정 A6).

### 2.3 확장 ③: 마지막 성공 동기화 시각

#### 2.3.1 이름·타입·렌더 + **setter 계약** (r2 강화 — H-8·H-4)
- 이름: `inventory_sync_last_success_timestamp_seconds{connection}`, type `gauge`. `*_timestamp_seconds` + unix epoch는 Prometheus 관례다.
- 저장: `lastSyncSuccess map[string]int64`(unix seconds). **`float64`를 쓰지 않는다** — `formatFloat`(`render.go:275`)의 `'g'` 포맷이 `1.7573184e+09`를 만들기 때문(§1.1 실측). `itoa(uint64(ts))`로 정수 렌더한다.
- 빈 상태: `len(map)==0` 조기 return — `queueDepthSet` 류 bool 불필요.
- `gauge.go`에 `time` import 추가(stdlib, 순도 유지).

**setter는 두 가지 불변식을 자기 안에 갖는다. 호출부 규율에 맡기지 않는다.**

```go
// SetSyncLastSuccess records one successful sync completion time for
// inventory_sync_last_success_timestamp_seconds{connection}.
//
// 계약 2가지 — 둘 다 호출부가 아니라 여기서 강제한다:
//  1) zero/음수 거부: time.Time zero의 Unix()는 -62135596800이고, itoa는
//     uint64를 받으므로(render.go:259) 18446744011573954816으로 렌더된다.
//     time()-gauge가 거대 음수가 되어 알람이 조용히 영구 무력화된다.
//  2) 단조(monotonic): 과거 값은 현재 값을 덮지 못한다. 부팅 복원이
//     라이브 발화 뒤에 도착해도 게이지가 역행하지 않는다.
func (c *Counters) SetSyncLastSuccess(connection string, at time.Time) {
	ts := at.Unix()
	if ts <= 0 {
		return // 계약 1 — zero time·음수 epoch는 적립하지 않는다
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.lastSyncSuccess[connection]; ok && prev >= ts {
		return // 계약 2 — 단조
	}
	c.lastSyncSuccess[connection] = ts
}
```

**왜 setter인가 (호출부 가드가 아니라):**
- **zero 가드 (H-8)**: r1은 이 가드를 `restore.go`에만 요구했다. 그러면 **라이브 경로가 뚫린다** — `sync.go`의 `finished`(`:98` `time.Now()`)는 정상적으로 zero가 아니지만, 향후 어떤 호출부가 `*time.Time` nil을 역참조하거나 zero struct를 넘기면 방어선이 없다. 실측 위험도: `itoa`가 `uint64`를 받는다는 사실(`render.go:259`)은 컴파일러가 잡아주지 않는 조용한 랩어라운드다.
- **단조 가드 (H-4)**: r1은 복원 호출을 "`startEngineLane` 안"이라고만 적고 **문(statement) 위치를 지정하지 않았다**. `taskEngine.Start(baseCtx)`(`engine_config.go:152`) 뒤에 놓이면, 폴 간격 2초(`engine_config.go:35` `PollInterval: 2 * time.Second`)로 즉시 도는 엔진이 태스크 종단 관측 갱신(`compose.go:204` `RunSync`)을 트리거해 `now`를 쓴 직후, 복원이 과거 `committed_at`으로 **덮어쓴다**. 발동 조건이 "크래시 재시작"인데 그것이 정확히 복원 회로의 존재 이유이므로 최악의 타이밍이다.
  단조 규정은 이 문제를 **호출 순서와 무관하게** 없앤다. 순서 제약(claim으로 강제해야 하는 종류)보다 견고하고, 향후 복원 호출이 이동해도 깨지지 않는다. 그럼에도 §6 P4는 호출 위치를 `stack.BuildEngine`(`:148`) **앞**으로 고정한다 — 두 방어선을 겹치되, 계약의 주체는 setter다.

#### 2.3.2 재시작 소실 문제 — 복원한다 (판정)
in-process 맵이라 프로세스 재시작 시 값이 사라진다. **복원하는 쪽으로 판정**한다:
- 이 gauge의 유일한 소비 형태가 `time() - inventory_sync_last_success_timestamp_seconds > threshold` 알람/패널이다. 시리즈 부재는 알람을 조용히 무력화하고(`absent()` 별도 규칙 없이는), 패널을 빈칸으로 만든다. 재시작 직후가 정확히 관측이 필요한 시점이므로 소실은 치명적이다.
- 원장이 이미 DB에 있다 — `inventory_sync_run`(§1.4). 복원은 신규 데이터가 아니라 이미 쓰고 있는 행의 재해석이다.
- 카운터는 복원하지 않는다 — Prometheus가 counter reset을 다루는 반면, 절대 시각 gauge는 reset 개념이 없다.

**복원 위치 판정: `startEngineLane`(`engine_config.go:138`)**, `compose.Build` 아님. (r2: 근거 문장 교체 — M-4·M-5·M-6. 결론은 불변.)

`compose.Build` 호출부 **전수 실측**(`grep -rn "compose.Build(" backend/ --include=*.go` — 14건):
- **프로덕션 3곳**: `engine_config.go:139`(서버 엔진 레인), `main_sync.go:82`(CLI), **`internal/api/v2/infra.go:54`**(v2 읽기 API — r1이 누락했던 부트 형상).
- **테스트 11곳**: `compose_test.go:36,65,80,121,229,333,451`, `compose_proxmox_test.go:31,66`, `healthloop_test.go:62`, `router/engine_injection_test.go:109`.
- (r1이 Build 호출부로 인용했던 `compose_test.go:59,108,264`·`compose_proxmox_test.go:53`은 **오인용**이다 — 그 행들은 `provider_rate_limit_test` Contains 단얫 라인이다.)

Build에 복원을 넣지 않는 이유 — r1의 "마이그레이션 선행은 `healthloop_test.go:59`뿐"도 **거짓**이었다(`compose_test.go:286`·`:429`의 헬퍼도 선행한다). 정정된 근거는 이것이다:
1. **11개 테스트 호출부 중 마이그레이션이 선행하지 않는 것이 다수**다 — `compose_test.go:36,65,80,121,229`는 스택 조립만 검증하는 형상이다. `inventory_sync_run` 질의를 Build에 넣으면 그 테이블이 없는 경로에서 Build가 실패하거나 에러를 삼켜야 한다. 어느 쪽도 Build의 계약(조립 성공/실패)을 오염시킨다.
2. **의미론적 근거가 더 결정적이다**: 복원은 "이 프로세스가 앞으로 메트릭을 서빙한다"는 전제에서만 옳다. CLI(`main_sync.go:82`)와 v2 API 부트(`infra.go:54`)는 `/internal/metrics`를 서빙하지 않으므로 복원할 이유가 없고, CLI에서는 오히려 리포트의 `MetricsText`에 무관한 과거 값을 섞는다. **Build는 "무엇을 서빙하는가"를 모르는 층이다.**
3. `startEngineLane`은 서버 부트 전용이고 마이그레이션 이후이며(`main.go:92→97→111`, gaps 재검증 완료), 이미 R11 nil-lane 형상을 소유한다(`:140-143`).

실패 처분: **로그 후 계속**. 게이지 부재가 부팅 실패보다 낫다(부팅 hang 위험은 마이그레이션 fail-fast로 배제 — L-7).

#### 2.3.3 복원 질의
신규 파일 `backend/internal/infra/inventory/restore.go`:
```go
// RestoreLastSuccess seeds inventory_sync_last_success_timestamp_seconds from
// the persisted run ledger — 프로세스 재시작이 패널을 비우지 않게 한다.
// 술어는 projection.go:32,98과 동일(succeeded + committed_at NOT NULL).
func RestoreLastSuccess(ctx context.Context, db *gorm.DB, counters *metrics.Counters) error
```
질의(GORM):
```
Table("inventory_sync_run").
Select("provider_connection.uid AS uid, MAX(inventory_sync_run.committed_at) AS at").
Joins("JOIN provider_connection ON provider_connection.id = inventory_sync_run.connection_id").
Where("inventory_sync_run.status = ? AND inventory_sync_run.committed_at IS NOT NULL", RunStatusSucceeded).
Group("provider_connection.uid")
```
- `connection_id` 직결 조인(`model/inventory.go:61`) — `context_id` 경유보다 짧다.
- 읽는 컬럼은 `committed_at`. 성공 갈래에서 `committed_at == finished_at == finished`이므로(§1.2, 가정 A4) 복원값과 라이브 발화값이 같다.
- `counters == nil`이면 즉시 nil 반환(무동작).
- 읽기 전용 단일 집계 질의 — 프로덕션 판정 로직 무접촉.

**스캔 충실도 주의(R7)**: `MAX(committed_at)` 집계값을 Go `time.Time`으로 직접 스캔하면 드라이버 의존이다 — 테스트가 쓰는 SQLite 경로에서 datetime 집계가 텍스트로 돌아와 **에러 없이 zero value**로 착지할 수 있다. 대안 2가지 중 택1하고 T-5로 반증한다: (i) 집계값을 `string`/`int64`로 스캔해 파싱, (ii) `MAX()`를 버리고 커넥션별로 `ORDER BY committed_at DESC LIMIT 1`의 첫 행을 취한다. 어느 쪽이든 zero는 적립되지 않는다 — **§2.3.1의 setter zero 가드가 최종 방어선**이므로 `restore.go`가 실수해도 `18446744011573954816`은 나오지 않는다. 그럼에도 `restore.go`에서 zero를 걸러 로그를 남기는 편이 진단에 낫다(이중 방어).

#### 2.3.4 소거 회로 — `pruneVanished`와의 대칭 (r2 신설 — H-3)
r1은 이 gauge에 **소거 경로를 두지 않았다.** `provider_health`에는 `RemoveHealth`(`gauge.go:19-23`) ↔ `pruneVanished`(`healthloop.go:158-168`) 대칭이 이미 있는데, 신규 gauge만 그 대칭이 깨진 상태였다. 결과:
- 커넥션 행이 DB에서 삭제되어도 마지막 성공 시각이 프로세스 수명 내내 남는다 → `time() - gauge`가 계속 커지며 **알람 영구 오발**.
- 맵이 단조 증가한다(삭제된 커넥션 UID가 영구 누적).
- 재시작해야만 해소되고, 그때도 복원 질의가 `JOIN provider_connection`(INNER)이라 재적립되지 않을 뿐이다 — 즉 "재시작 전까지 오발"이 정확한 상태다.

**해법**: `metrics`에 소거 메서드를 하나 더하고, 이미 소멸을 관측하는 유일한 회로에서 호출한다.
```go
// RemoveSyncLastSuccess drops a connection's last-success series. 커넥션 행이
// 사라진 뒤에도 마지막 값이 잔존해 time()-gauge 알람이 영구 오발하는 것을
// 막는다 — RemoveHealth(gauge.go:19)와 같은 대칭 계약이다.
func (c *Counters) RemoveSyncLastSuccess(connection string) { … }
```
호출부: `healthloop.go:158-168 pruneVanished`의 기존 루프 안, `RemoveHealth(uid)` **바로 다음 줄**(착지 Phase는 P3b). 이 함수는 이미 "이전 관측 집합에 있었으나 이번 sweep에 없는 UID"를 판정하고 있으므로 새 판정 로직이 필요 없다 — **1줄 추가**.

계층 판정: `HealthSweeper`가 health 외의 패밀리를 건드리는 것은 이름과 어긋나 보이지만, 이 회로는 실질적으로 **커넥션 생존 관측기**다(DB 행 전수 순회가 그 실체 — `:123`). 별도 sweeper를 신설하는 것은 §18.2에 없는 회로를 하나 더 만드는 범위 확장이므로 채택하지 않는다. 대신 `pruneVanished`의 doc comment에 "health 시리즈만이 아니라 커넥션 스코프 시리즈 전체를 소거한다"고 명시한다.

**부작용 경고**: 이 배선은 **엔진 레인이 살아 있을 때만** 동작한다(sweeper 기동은 `engine_config.go:154`). R11 부트(스택 실패)에서는 `/internal/metrics` 자체가 미등록이므로(`main.go:28-30` nil 가드) 소비자도 없다 — 정합하다.

#### 2.3.5 partial 갈래의 의미 확정 (r2 신설 — H-9)
r1은 라이브 발화 술어를 "성공 갈래"라고만 적어 **라이브는 succeeded∪partial, 복원은 succeeded 전용**이 될 여지를 남겼다. 이름이 `last_success`이므로 확정한다:

> **`status == RunStatusSucceeded`인 run만 이 gauge를 갱신한다. partial은 갱신하지 않는다.**

- 근거: `partial`은 §9.2상 **generation을 발행하지 못한 run**이다(`sync.go:103-112` — `reconcileAbsences`·`CommittedAt` 둘 다 succeeded 갈래 전용). "마지막으로 인벤토리가 온전히 동기화된 시각"이라는 패널 의미에 partial은 들어가지 않는다.
- 이 술어는 복원 질의의 `status='succeeded' AND committed_at IS NOT NULL`(§2.3.3)과 **정확히 일치**한다 — 라이브와 복원이 같은 집합을 본다.
- 구현 술어는 `status == RunStatusSucceeded`를 쓴다. `!report.CommittedAt.IsZero()`도 동치이지만(succeeded 갈래에서만 `:107`이 대입), **상태 상수 비교가 의도를 직접 말한다**. `:145`의 기존 `if status == RunStatusPartial`과 같은 어휘 층이다.
- 검증: T-4가 partial 케이스를 포함한다 — `sync_metrics_test.go:68-79`에 rate-limited partial 픽스처가 **이미 있다**(신규 픽스처 불요).

---

### 2.4 관측 계약 — "이 시리즈가 틀렸을 때 무엇이 알려주는가" (r2 신설)

§0.1 제1원칙의 이행층이다. 본 과제가 만들거나 바꾸는 시리즈마다, 그 시리즈가 조용히 잘못될 수 있는 구체적 방식과 그것을 잡는 기구를 짝짓는다. **테스트가 없는 칸이 있으면 그 칸은 미완성이다.**

| 시리즈 | 조용한 오류 방식 | 이것을 잡는 기구 | claim |
|---|---|---|---|
| `provider_health{connection,provider}` | provider 라벨이 빈 문자열로 발화(`conn.ProviderType`가 아닌 값 전달) | T-6이 fake 어댑터의 `ProviderName` 리터럴을 단얫 | 2 |
| 〃 | 커넥션 삭제 후 시리즈 잔존 | **T-8**(신설) — prune 후 **긍정 단얫**으로 잔존 라인 부재 확인. 기존 `:191` 부정 단얫은 검출력 0(§4.2) | 3 |
| 〃 | DB 실패 회에서 provider 라벨 유실 | T-6 두 번째 갈래(`healthloop_test.go:125`) | 2 |
| `inventory_sync_total{…,status}` | 특정 return 경로에서 미발화(성공률 분모 왜곡) | **T-3 경로별 단얫** — `grep -c`가 아니라 8경로 각각의 델타 | 6 |
| 〃 | 한 경로에서 2회 발화(중복 계수) | T-3의 "총합 == 호출 횟수" 단얫 | 6 |
| 〃 | 라벨 3개 중 하나가 정렬 누락 → 렌더 비결정성 | **골든 setup 다중 시리즈화**(§4.1) + `metrics_test.go:113-116` 결정성 검사 | 4 |
| `inventory_sync_last_success_timestamp_seconds` | zero time → `18446744011573954816` → 알람 영구 무력화 | **T-9**(신설) — setter에 zero 직접 주입 후 라인 부재 단얫 | 7 |
| 〃 | 복원이 라이브 값을 과거로 덮음 | **T-10**(신설) — 단조 단얫(큰 값 후 작은 값 → 불변) | 7 |
| 〃 | 커넥션 삭제 후 잔존 → 알람 영구 오발 | **T-8**(신설, 위와 같은 테스트) | 3 |
| 〃 | partial run이 갱신 → 이름과 의미 괴리 | **T-4 partial 케이스**(기존 픽스처 재사용) | 8 |
| 〃 | 복원만 착지·라이브 미착지(또는 역) | 착지 단위 계약(§9.2 forward-stop) + T-5·T-4 | 9 |
| 전 패밀리 | 스펙 표와 코드가 어긋남 | **자동 게이트 없음**(M-8 실측 — `.github/workflows/` 2파일·required 9종에 스펙-코드 대조 0건). P5 누락은 claim 13·14의 **수동 검증**이 유일한 방어선이다 | 13·14 |

마지막 행은 계약이 아니라 **인정된 구멍**이다. 자동 게이트 신설은 본 과제 범위 밖이므로(스펙에 없는 CI 잡 추가) §13에 미해결로 올린다.

---

## 3. 스펙 §18.2 개정 — 정확한 diff 문언

### 3.0 인용 정책 — 절 제목 우선 (r2 신설, 팀리드 확정 4)
병행 과제 `otel-lgtm-recipe`가 **같은 파일에 §18.5를 신설**한다. 두 계획의 착지 순서에 따라 행 좌표가 서로 어긋나므로:
- **구현자는 행 번호가 아니라 절 제목·표 행의 메트릭 이름으로 대상을 찾는다.** 본 문서의 행 번호는 착수 시점(main `30856a3`) 스냅숏이며, 불일치 시 **절 제목이 정본**이다.
- 대상 절: `### 18.2 Control-plane metrics — instrumented where`. 대상 행: 표에서 `` `provider_health` ``로 시작하는 행, `` `inventory_sync_partial_total` ``로 시작하는 행.
- **역으로 본 계획이 남에게 주는 파급을 기록할 의무가 있다**: 표에 2행을 삽입하므로 그 아래 전부가 2행씩 밀린다 — deferred 2행 `agent_heartbeat_age_seconds` `:1481→:1483`, `outbox_backlog` `:1482→:1484`, 후행 문장 `:1484→:1487`(§3.2 문단까지 더하면 추가로 밀린다). §3.4에 파급 목록으로 정리한다.

### 3.1 표 행 (`docs/architecture/multi-infrastructure-control-plane-v2.md`)

**변경 1 — `:1467` 라벨 열 수정**
```diff
-| `provider_health` | health check loop, gauge 0/1 | connection | yes |
+| `provider_health` | health check loop, gauge 0/1 | connection, provider | yes |
```

**변경 2 — `:1473` 직후 2행 삽입**
```diff
 | `inventory_sync_partial_total` | sync runner | connection | yes |
+| `inventory_sync_total` | sync runner, every RunSync return | connection, mode, status | yes |
+| `inventory_sync_last_success_timestamp_seconds` | sync runner, gauge unix epoch seconds | connection | yes |
 | `provider_task_duration_seconds` | task engine (claim→terminal) | operation, status | yes |
```

삽입 후 표는 데이터 18행(`:1467-1484`)이 되고, deferred 2행은 `:1481→:1483`, `:1482→:1484`로 밀린다. 후행 문장("No alert thresholds…")은 `:1484→:1487`.

### 3.2 표 뒤 문장 삽입 (r2 정정 — H-1)
후행 문장("No alert thresholds are committed here…") **직전**에 한 문단을 넣는다 — §2.1.3 불변식과 §2.3.5 술어를 계약으로 승격. r1 초안은 "failed = pre-flight failures"라고 한정해 **거짓 서술**이었다(⑧의 6갈래가 `failed`의 지배적 발생원, §2.1.3):
```
`inventory_sync_total.status` uses the terminal run-status vocabulary — `succeeded`, `partial`, `failed`. Every `RunSync` invocation increments exactly one series, so the counter's total equals the number of sync attempts. `failed` covers both provider- and persistence-side run failures (rate limiting, permission denial, unreachability, discovery, cursor persistence, relationship derivation) and pre-flight failures that never create a run row.

`inventory_sync_last_success_timestamp_seconds` advances only on a `succeeded` run — a `partial` run publishes no generation and does not update it. The gauge is monotonic (a later write never moves it backwards), is restored from the `inventory_sync_run` ledger at engine-lane startup so a process restart does not blank the series, and is dropped when its connection row disappears.
```

### 3.3 코드 내 종수 문언 (r2 정정 — H-6·L-1)
r1은 `metrics.go:54`를 "(2종 기존 + **14종** 신규)"로 고치라 하면서 claim 14로 `grep '14종' == 0`을 요구해 **자기모순**이었다. 해소 방향은 "숫자 분해를 지우고 총수만 남긴다":
- `backend/internal/infra/metrics/metrics.go:4` — "the full §18.2 M1 family set (14종)" → **"(16종)"**.
- `backend/internal/infra/metrics/metrics.go:54` — "§18.2 M1 family set (2종 기존 + 12종 신규)" → **"§18.2 M1 family set (16종)"** — 분해 표기를 **삭제**한다. "14종"이라는 문자열이 남지 않아야 claim 14가 성립한다.
- `backend/internal/infra/metrics/metrics_test.go:72`(r1이 `:73`으로 잘못 적었다 — L-1)·`:225` 주석의 "14종" → **"16종"**.
- 변경 전 현재 적중은 3건(`metrics.go:4`, `metrics_test.go:72`·`:225`)이며 P5 후 0건이어야 한다.

### 3.4 파급·비개정 목록 (r2 확장 — H-11·M-8·M-11·L-2·L-8)

**본 과제 착지가 어긋나게 만드는 문서 — 전수 5건**

| 문서·행 | 어긋나는 내용 | 처분 |
|---|---|---|
| `v2-phase1/otel-lgtm-recipe-plan.md:455` | "inventory sync에 `status` 라벨 추가 → P7을 `inventory_sync_duration_seconds_count{status="partial"}`로 단순화 가능"이라 적었는데, **본 계획은 (a)안을 기각**했다. 착지 후 그 PromQL은 **에러가 아니라 빈 결과**를 낸다 — 가장 조용한 실패 | recipe 소관. 정정 문언: `inventory_sync_total{status="partial"}`. **본 계획은 recipe 파일을 수정하지 않는다**(recipe §경계 규칙) — 팀리드에 전달 필요(§13-5) |
| `v2-phase1/otel-lgtm-recipe-plan.md:454,456` | provider 라벨·last_success gauge 서술 — **정확하다**. 조치 불요 | — |
| `v2-phase1/otel-lgtm-recipe-plan.md:25,336,458,549` + `otel-telemetry-assessment.md:60` | 골든 범위를 `metrics_test.go:118-218` 리터럴로 고정. 본 과제가 `:174` 직후 4줄을 삽입하므로 `:214-218`이 `:218-222`로 밀려 `sed -n '118,218p'` 형태 인용은 `resource_stale_total`·`secret_access_total`을 **놓친다**(M-11) | recipe 소관 경고. 권고: 행 범위 고정 대신 백틱 경계 탐색. §13-5 |
| `v2-phase1/RESUME.md:11` | "§18.2 메트릭 ✅ **14종** 전부" — 착지 후 16종 | **본 과제가 갱신한다**(P5). 저장소 재개 문서라 어긋나면 다음 세션이 오판한다 |
| `v2-phase1/phase6-entry-assessment.md:14,18,27` | "14종" 문언 3개소 | 진입 판정 이력 문서 — **무변경**. 그 시점의 사실 기록이다 |
| `docs/architecture/multi-infrastructure-control-plane-epic.md:731,734-735` | 메트릭 이름 목록 | **이미 현행 스펙과 불일치**(L-8 실측) — 본 과제가 만든 문제가 아니다. 조치 불요 |

**개정하지 않는 것**
- `v2-phase1/metrics-plan.md` — 완료 과제의 이력 문서. 재작성하지 않는다.
- `v2-phase1/otel-telemetry-assessment.md:60-63`(r1이 `:61-63`으로 적었다 — L-2, `:60`이 시작) — 조사 산출물이지 계약이 아니다. 골든 행 번호 인용이 어긋나지만 재작성하지 않고 §13-1에 올린다.
- `docs/security/route-inventory.txt`·`sensitive-routes.txt` — 신규 엔드포인트 없음(`grep -rn "internal/metrics" docs/security/*.txt` → **0건**, 실측). 애초에 미수록이라 무변경이 자명하다. **다만 이는 §2.4 마지막 행·M-9의 문제이기도 하다** — 노출 증분을 기록할 아티팩트 자체가 없다(§13-3).

**자동 대조 게이트 부재 (M-8, 인정된 구멍)**: `.github/workflows/` 2파일 + ci-baseline required 9종 전수 확인 결과 **스펙-코드 대조 잡이 0건**이다. P5(문서 개정)를 통째로 빠뜨려도 CI는 녹색이다. 방어선은 claim 13·14의 수동 검증뿐이며, 이를 §13-6에 미해결로 올린다.

---

## 4. 파괴 범위 실측 → **부록 §4**

깨지는 단얫(골든 diff 문언 포함)과 안 깨지는 단얫의 전수 목록은 **`v2-phase1/metrics-ext-plan-evidence.md` §4**에 있다. 요지만 옮기면:

- **깨진다(수정 필수)**: `metrics_test.go`(골든 본문·setup 호출·TYPE 목록·주석), `healthloop_test.go`(`healthLine` helper `:69-71` **그리고** `:191` — 후자는 "통과하지만 검출력을 잃는" 종류라 T-8로 재작성한다), `sync_metrics_test.go`(증설).
- **안 깨진다**: `compose_test.go:59,108,264`·`compose_proxmox_test.go:53`·`contracttest/harness.go:183,193`·`sync_test.go:537-541`·4개 `adapter/*/latency_test.go`·`tencent/adapter_test.go:563`·`engine_metrics_test.go`·`broker_metrics_test.go`·`main_metrics_test.go`·라우트 골든 2종·`metrics_test.go:36-56`. 전부 **본 과제와 무관한 패밀리를 본다**는 이유로 통과하는 것이지, 본 과제를 검증하지 않는다.
- 골든의 `inventory_sync_total`은 **4개 이상 다중 시리즈**로 넣는다 — 단일 시리즈면 결정성 검사(`metrics_test.go:113-116`)가 3라벨 정렬 누락을 잡지 못한다(부록 §4.1(2), M-7).

---

## 5. 카디널리티 영향

기호: `C` = provider_connection 행 수(가정 A1: 코드상 상한 없음, 운영 규모 수십), `M` = mode 어휘 3(`full`/`incremental`/`targeted`, `sync.go:48` 주석), `S` = 종단 status 3(`succeeded`/`partial`/`failed`).

| 패밀리 | 시계열 | 렌더 라인 | 비고 |
|---|---|---|---|
| `inventory_sync_duration_seconds` (현행, 무변경) | C×M | C×M×15 | 버킷 12 + `+Inf` + `_sum` + `_count` |
| (a)안 채택 시 duration | C×M×S | **C×M×S×15** | 라인 수 3배 — 채택하지 않음 |
| **`inventory_sync_total`** (신설) | C×M×S | **C×M×S** = 최대 9C | 시리즈당 1줄 |
| **관측 상한(현실)** | C×1×S = **3C** | 3C | `§3.3 r2`상 Phase 2 실행 경로는 full 전용이고 `compose.go:204`는 Mode 미지정→`full` 정규화(`sync.go:65-68`). `incremental`/`targeted`는 현재 발화 경로가 없다 |
| **`provider_health`** | C (**불변**) | C | 라벨 추가는 시계열 수를 늘리지 않는다 — provider_type은 connection의 함수(1:1 종속 라벨) |
| **`inventory_sync_last_success_timestamp_seconds`** | ≤C | ≤C | 성공 이력이 있는 커넥션만 |

**총 증가 상한**: `9C + C = 10C` 시계열, 같은 수의 라인. C=50이면 500라인 — 현행 duration 단독 라인 수(C×M×15 = 2250, 관측 기준 C×1×15 = 750)와 견주어 저비용이다. **provider_health의 라벨 추가는 카디널리티 증가 0**이라는 점이 (a)안 대비 결정적 차이다.

카디널리티 폭발 위험 지점 **2곳**:

**(가) `connection` 라벨** — 사전 단계 ①(`loadConnection` 실패)의 값은 **DB에 없는 UID**일 수 있다. 실측상 그 입력을 만드는 경로는 CLI(`main_sync.go:97`)뿐이고, CLI 프로세스는 리포트를 찍고 종료하므로 `/internal/metrics`를 서빙하지 않는다. 장기 상주 프로세스의 유일한 `RunSync` 호출부 `compose.go:204`는 join 결과 UID(`:190-197`) + 공백 가드(`:201`)라 DB 행 수로 유계다. → **폭발 없음**(가정 A2).

**(나) `mode` 라벨 (r2 신설 — L-4)** — `SyncInput.Mode`는 **검증 없는 자유 문자열**이다. `sync.go:65-68`은 빈 문자열만 `"full"`로 정규화하고, 어휘 검증(`full|incremental|targeted` 소속 확인)은 **어디에도 없다**. CLI `main_sync.go:97`의 `*mode`는 플래그 값 그대로다. 따라서 CLI에서 임의 문자열이 라벨이 될 수 있다.
- **장기 프로세스에서는 유계**다: `compose.go:204`는 `Mode`를 지정하지 않아 항상 `"full"`로 정규화된다. 스크랩 대상 프로세스의 mode 어휘는 **사실상 1종**이다.
- 그럼에도 (가)와 같은 구조적 결함이므로 함께 기록한다. **본 과제는 mode 검증을 추가하지 않는다** — 프로덕션 판정 로직 변경이고(보존 제약 3), 스펙에 없는 범위 확장이다. §13-7에 미해결로 올린다.
- 두 라벨 모두 "CLI는 스크랩되지 않는다"는 같은 전제에 기대고 있다. 그 전제가 깨지는 유일한 방식은 장기 프로세스에 새 `RunSync` 호출부가 생기는 것이며, **claim 12가 호출부 수를 고정**한다.

---

## 6. Phase 분해 (각 ≤5파일, 독립 검증 단위)

의존: **P1 → P2 → P3a → P3b → P4 → P5** (직렬). P1·P2·P3는 실질적으로 독립 기능이지만 `metrics.go`·`metrics_test.go` 골든이라는 **공유 편집면**을 갖기 때문에 직렬화한다(`metrics-plan.md:112`와 같은 순서 계약). 각 Phase 종료 시점에 `go build ./...`와 해당 패키지 테스트가 **녹색**이어야 한다 — 계층별이 아니라 **기능별 분해**인 이유가 이것이다(`SetHealth` 시그니처만 바꾸고 `healthloop.go`를 다음 Phase로 미루면 compose 패키지가 컴파일되지 않는다: `healthloop.go:133`·`:211`).

### P1 — `provider_health`에 provider 라벨 (5파일)
- `backend/internal/infra/metrics/metrics.go` — `healthSample` struct 신설, `health` 필드 타입 교체(`:77`), `New`의 맵 초기화 타입 정정(`:97`)
- `backend/internal/infra/metrics/gauge.go` — `SetHealth(connection, provider string, healthy bool)`, `SetHealthUnhealthy(connection string)` 신설, `renderHealth`에 provider 라벨 append(`:45-52`)
- `backend/internal/infra/metrics/metrics_test.go` — §4.1(1)(3-부분)(4-부분)
- `backend/internal/infra/compose/healthloop.go` — `:133` 시그니처, `:211` `SetHealthUnhealthy` 교체 (**2줄**)
- `backend/internal/infra/compose/healthloop_test.go` — §4.2 helper 1줄
- 검증: `cd backend && go test ./internal/infra/metrics/ ./internal/infra/compose/ -race -count=1`

### P2 — `inventory_sync_total` 신설 (5파일)
- `backend/internal/infra/metrics/metrics.go` — `connModeStatusKey` struct, `syncRuns` 맵 필드, `New` 초기화, `IncSyncRun` 메서드 (종수 주석은 **P5 소관** — P2 종료 시점의 패밀리 수는 15종이라 여기서 "16종"을 쓰면 일시적 거짓이 된다)
- `backend/internal/infra/metrics/render.go` — `renderSyncTotal` 신설 + `Render()` 디스패치 삽입(`:39` `renderSyncPartial` 직후)
- `backend/internal/infra/metrics/metrics_test.go` — §4.1(2 앞 2줄)(3)(4)
- `backend/internal/infra/inventory/sync.go` — `countRun` 헬퍼 + 8개 return 지점 1줄씩(§2.1.3)
- `backend/internal/infra/inventory/sync_metrics_test.go` — T-2/T-3 증설
- 검증: `cd backend && go test ./internal/infra/metrics/ ./internal/infra/inventory/ -race -count=1`

### P3a — last-success gauge 패밀리 신설·렌더 (4파일)
- `backend/internal/infra/metrics/metrics.go` — `lastSyncSuccess map[string]int64` 필드 + `New` 초기화
- `backend/internal/infra/metrics/gauge.go` — `SetSyncLastSuccess`(**zero 가드 + 단조**, §2.3.1)·`RemoveSyncLastSuccess`(§2.3.4)·`renderSyncLastSuccess`, `time` import
- `backend/internal/infra/metrics/render.go` — `Render()` 디스패치 1줄(`renderSyncTotal` 직후)
- `backend/internal/infra/metrics/metrics_test.go` — 부록 §4.1(2 뒤 2줄)(3)(4) + **T-9**(zero 거부)·**T-10**(단조)
- 검증: `cd backend && go test ./internal/infra/metrics/ -race -count=1` — 패밀리가 렌더되고 두 setter 계약이 성립한다. `RemoveSyncLastSuccess`는 이 시점에 호출부가 없다(다음 Phase가 배선) — 미호출 공개 메서드는 컴파일·vet 무해다.

### P3b — 라이브 발화 + 소거 배선 (4파일)
- `backend/internal/infra/inventory/sync.go` — **r2 정정(H-5)**: r1의 "성공 갈래 1줄"은 거짓이었다. `:139-149` 블록은 succeeded·partial·failed **전부** 통과하고 내부 분기는 `if status == RunStatusPartial`(`:145`) 하나뿐이다. 실제 삽입은 **최소 3줄** — 신규 `if status == RunStatusSucceeded { r.counters.SetSyncLastSuccess(in.ConnectionUID, finished) }`. 술어는 §2.3.5 확정대로 `status == RunStatusSucceeded`(`!report.CommittedAt.IsZero()` 아님). 위치는 `Render()`(`:148`)보다 앞.
- `backend/internal/infra/inventory/sync_metrics_test.go` — **T-4**: succeeded → gauge == `report.FinishedAt.Unix()`; **partial → gauge 불변**(`:68-79` 기존 rate-limited 픽스처 재사용); failed → 불변.
- `backend/internal/infra/compose/healthloop.go` — `pruneVanished`에 `RemoveSyncLastSuccess(uid)` **1줄** + doc comment 갱신(§2.3.4)
- `backend/internal/infra/compose/healthloop_test.go` — **T-8** 재작성(긍정/부정 쌍 + last-success 소거 갈래, 부록 §4.2(2))
- 검증: `cd backend && go test ./internal/infra/inventory/ ./internal/infra/compose/ -race -count=1`
- **P3a/P3b 분리 사유**: P3a는 "패밀리와 그 계약이 존재하는가", P3b는 "실제로 발화·소거되는가"를 각각 독립 검증한다. 한 Phase에 묶으면 6파일이고, 테스트를 뒤로 미루면 `sync.go` 발화가 **무검증 상태로 착지**한다(§6 서두 기준 위반). P3b는 `RemoveSyncLastSuccess`·`SetSyncLastSuccess`에 의존하므로 **P3a 이후**다.
- (`healthloop.go`·`healthloop_test.go`는 P1에서도 손댄 파일이다 — 같은 파일을 두 Phase가 편집하는 것은 §6 서두의 순서 계약으로 처리하며, `metrics.go`·`metrics_test.go`가 이미 같은 형태다.)

### P4 — 부팅 복원 (3파일) — **P3b와 분리 착지 금지**
- `backend/internal/infra/inventory/restore.go` — **신규**, `RestoreLastSuccess`(§2.3.3, R7 스캔 주의)
- `backend/internal/infra/inventory/restore_test.go` — **신규**, T-5. 하네스는 **`sync_test.go`의 기존 것을 재사용**한다(L-3): `package inventory_test`, `newInventoryDB`(`sync_test.go:75` `migrate.Run`), `seedConnection`. 신규 하네스를 만들지 않는다.
- `backend/engine_config.go` — `startEngineLane`에 복원 호출 1줄 + 실패 로그 1줄 + `inventory` import
- **호출 문 위치 고정 (H-4)**: `compose.Build` 성공 직후, **`stack.BuildEngine`(`engine_config.go:148`)보다 앞**. 근거 — `taskEngine.Start`(`:152`) 이후에 두면 폴 간격 2초(`:35`)로 도는 엔진이 `compose.go:204 RunSync`를 트리거해 쓴 `now`를 과거 `committed_at`이 덮을 수 있다. §2.3.1의 단조 계약이 이미 이를 막지만 **두 방어선을 겹친다**. 순서는 claim 9가 검증한다.
- 검증: `cd backend && go test ./internal/infra/inventory/ ./ -race -count=1`
- **착지 단위 계약**: P3b와 P4는 **같은 PR 또는 연속 착지**한다. 근거는 §9.2 forward-stop 표 — P3b만 착지한 중간 상태는 착지 전보다 나쁘다.

### P5 — 계약 문서 개정 (3파일)
- `docs/architecture/multi-infrastructure-control-plane-v2.md` — §3.1 표 3행 + §3.2 문단
- `backend/internal/infra/metrics/metrics.go` — §3.3 종수 주석 2곳(`:4`, `:54`). **여기서만** 손댄다 — 패밀리 수가 16종으로 확정되는 유일한 시점이다
- `backend/internal/infra/metrics/metrics_test.go` — §3.3 주석 종수 2곳(`:73`, `:225`)
- 검증: `cd backend && go test ./... -race -count=1` (전체 게이트) + claim 14

---

## 7. 수정 파일 전수 — **고유 13개** (수정 11 + 신규 2)

r1은 표제 "10개" / 하위 "수정(8)" / 실제 12로 3중 불일치였다(M-2). r2에서 `RESUME.md`가 더해져 총 13이다.

**수정(11)**
| # | 경로 | 변경 요약 | Phase |
|---|---|---|---|
| 1 | `backend/internal/infra/metrics/metrics.go` | `healthSample`·`connModeStatusKey` struct, 필드 3건, `New` 초기화 3건, `IncSyncRun`, 종수 주석 | P1·P2·P3a·P5 |
| 2 | `backend/internal/infra/metrics/gauge.go` | `SetHealth` 시그니처, `SetHealthUnhealthy`, `SetSyncLastSuccess`(zero+단조), `RemoveSyncLastSuccess`, `renderHealth` 라벨, `renderSyncLastSuccess`, `time` import | P1·P3a |
| 3 | `backend/internal/infra/metrics/render.go` | `renderSyncTotal`(3단 `sort.Slice`), `Render()` 디스패치 2줄 | P2·P3a |
| 4 | `backend/internal/infra/metrics/metrics_test.go` | 골든 다중 시리즈 갱신, setup 호출, TYPE 목록 2줄, T-9·T-10, 주석 | P1·P2·P3a·P5 |
| 5 | `backend/internal/infra/compose/healthloop.go` | `:133`·`:211` 2줄(P1) + `pruneVanished` 소거 1줄 + doc comment(P3b) | P1·P3b |
| 6 | `backend/internal/infra/compose/healthloop_test.go` | `healthLine` 1줄(P1) + T-8 재작성(긍정/부정 쌍 + last-success 갈래, P3b) | P1·P3b |
| 7 | `backend/internal/infra/inventory/sync.go` | `countRun` 헬퍼, 8개 return 계기, succeeded 갈래 gauge 3줄 | P2·P3b |
| 8 | `backend/internal/infra/inventory/sync_metrics_test.go` | T-2·T-3(경로별)·T-4(partial 포함) 증설 | P2·P3b |
| 9 | `backend/engine_config.go` | 복원 호출(위치 고정)·로그·import | P4 |
| 10 | `docs/architecture/multi-infrastructure-control-plane-v2.md` | §18.2 표 3행 + 문단 2개 | P5 |
| 11 | `v2-phase1/RESUME.md` | `:11` "14종" → "16종"(§3.4) | P5 |

**신규(2)**: `backend/internal/infra/inventory/restore.go`, `backend/internal/infra/inventory/restore_test.go`

**무변경 확인 대상**: `backend/go.mod`, `backend/go.sum`, `docs/security/route-inventory.txt`, `docs/security/sensitive-routes.txt`, `backend/internal/infra/metrics/histogram.go`, `backend/main.go`, `backend/router/**`.

### 7.1 줄 수 (650 경고선·800 하드캡)

**소스 파일 — 예상치가 신뢰할 만하다** (변경분이 열거 가능):
| 파일 | 현재 | 예상 | 판정 |
|---|---|---|---|
| `metrics.go` | 185 | ~222 | 여유 |
| `gauge.go` | 63 | ~120 | 여유 (setter 2종 + Remove + render) |
| `render.go` | 277 | ~312 | 여유 |
| `sync.go` | 488 | ~510 | 여유(경고선까지 140) |
| `healthloop.go` | 219 | ~223 | 여유 |
| `restore.go` (신규) | — | ~70 | 여유 |
| `engine_config.go` | 156 | ~161 | 여유 |

**테스트 파일 — 예상치를 제시하지 않는다.** r2가 더한 테스트 표면(T-3의 **경로별 9케이스**(테이블 drop 픽스처 포함), T-8 긍정/부정 재작성, T-9, T-10, 다중 시리즈 골든)은 폭이 커서 사전 추정이 빗나가기 쉽다 — 특히 `sync_metrics_test.go`는 DB 픽스처를 동반한 9케이스라 **현재 150줄에서 350~450줄 규모**로 커질 수 있다. r1의 "~235"는 명백히 과소였다.
- 따라서 이 4개 파일(`metrics_test.go` 281, `sync_metrics_test.go` 150, `healthloop_test.go` 212, `restore_test.go` 신규)은 **구현 후 `wc -l` 실측을 완료 보고에 적는다**(보존 제약 14).
- **판정 규칙**: 650 도달 시 같은 작업 안에서 분할한다. 지정 seam —
  - `sync_metrics_test.go` → `sync_run_metrics_test.go`(T-3 경로별 + T-4 타임스탬프) / `sync_metrics_test.go`(T-2 duration·changes·stale). 헬퍼 `renderLineValue`(`:136`)는 남는 쪽에 두고 다른 쪽에서 참조한다(같은 `package inventory_test`).
  - `metrics_test.go` → `render_golden_test.go`(T-1 골든 + TYPE 목록) / `metrics_test.go`(T-9·T-10·이스케이프·결정성·빈 상태).
- 소스 파일에는 분할 seam이 필요하지 않다.

---

## 8. 테스트 계약

- **T-1 렌더 골든**(P1·P2·P3a): `TestRenderFullFamilyGolden`(`metrics_test.go:75`)이 16종 전문 byte 비교로 갱신된다. 확인 요소 — provider_health 2라벨, `inventory_sync_total` 3라벨, 타임스탬프 정수 렌더(지수 표기 아님), 신규 패밀리의 렌더 위치(표 순서), 기존 12종 라인 **byte 불변**.
- **T-2 계기 델타**(P2): `sync_metrics_test.go` 증설 — 성공 run 1회 → `inventory_sync_total{connection,mode,status="succeeded"} 1`; rate-limited partial run 1회 → `{status="partial"} 1`; 두 run 후 duration `_count`는 여전히 2(기존 단얫과 정합).
- **T-3 불변식 — 경로별 단얫**(P2). r1의 T-3은 8경로 중 3개만 덮었고 claim 6은 `grep -c == 8`이라 **경로별 배치를 판정하지 못했다**(H-2: 한 경로 2회 + 다른 경로 0회도 8, 9번째 return이 생겨도 통과). r2는 **경로마다 델타 단얫 1개**를 요구한다. 각 케이스는 "그 경로를 타는 입력 → error 반환 확인 → `inventory_sync_total` 해당 라벨 조합 +1 → 다른 조합 불변":

  | 경로 | 유도 입력 | 기대 라벨 |
  |---|---|---|
  | ① `loadConnection` | 존재하지 않는 UID | `{status="failed"}` |
  | ② `resolveDiscoverer` | registry에 미등록 provider_type의 커넥션 | 〃 |
  | ③ `resolveContext` | `provider_context` 행 없는 커넥션 | 〃 |
  | ④ `resolveConnectionView` | 바인딩 없는 커넥션(broker 해석 실패) | 〃 |
  | ⑤ run `Create` | `inventory_sync_run` 테이블 drop 후 호출 | 〃 |
  | ⑥ `reconcileAbsences` | `infra_resource` 테이블 drop 후 성공 경로 진입 | 〃 |
  | ⑦ 확정 `Updates` | **테이블 drop으로는 격리 불가** — `consumePages`가 페이지마다 같은 테이블에 커서를 쓰므로(`sync.go:264`) drop은 `cursor_persist_failed`(`:266`)에 먼저 걸려 ⑧-failed로 빠진다. 격리하려면 문 단위 GORM 콜백이 필요하다 | 〃 |
  | ⑧-succeeded / ⑧-partial / **⑧-failed** | 정상 / rate-limited(`:68-79` 픽스처) / `SignalPermissionDenied` 반환 스텁 | `succeeded` / `partial` / **`failed`** |

  ⑧-failed는 **반드시 포함**한다 — §2.1.3이 정정한 핵심(`failed`가 사전 단계 전유물이 아님)을 실증하는 유일한 케이스다.
  추가로 **총합 단얫**: 위 시나리오를 N회 돌린 뒤 `inventory_sync_total` 전 라인 값의 합 == N. 이것이 "한 경로 2회" 중복을 잡는다.
  ⑤·⑥은 테이블 drop 기법을 쓴다 — 선례 `healthloop_test.go:134` `db.Migrator().DropTable(...)`. **⑦은 9/10 케이스로 남긴다**: 위 사유(커서 지속이 먼저 걸린다)를 테스트 주석에 적고, claim 6(b)의 코드 검수가 ⑦의 `countRun` 배치를 대신 보증한다. GORM 콜백을 심는 것은 테스트 인프라 신설이라 본 과제 범위 밖이다.
- **T-4 타임스탬프 — partial 포함**(P3b, H-9): ① succeeded run → gauge == `report.FinishedAt.Unix()`. ② **partial run → gauge 불변**(`sync_metrics_test.go:68-79`의 rate-limited 픽스처 재사용 — 신규 픽스처 불요). ③ failed run → 불변. ②가 §2.3.5 술어(`status == RunStatusSucceeded`)의 반증 지점이다.
- **T-5 복원**(P4): 픽스처는 **`provider_connection` 행을 반드시 함께 시드한다** — 복원 질의가 `connection_id`로 조인해 UID를 얻으므로(§2.3.3), run 행만 시드하면 결과 집합이 비어 원인 불명 실패가 난다. 시드 형상: 커넥션 2개(A: succeeded 오래된 run + succeeded 최신 run, B: partial run만) → `RestoreLastSuccess` → A의 gauge = **최신 succeeded의 committed_at**, B는 라인 미발행. 추가 갈래 — 빈 원장 → `Render()`에 패밀리 미등장; `counters == nil` → 무동작 nil 반환; 스캔이 zero time을 낼 경우 적립하지 않음(R7).
- **T-6 health 라벨**(P1): `healthloop_test.go` 기존 4개 테스트가 helper 1줄 수정으로 통과하고, provider 라벨값이 `fake.ProviderName` 리터럴과 일치한다. DB 실패 회(`:125`)에서 provider 라벨이 **보존된 채** 0으로 뒤집힌다. **L-5 추가 갈래**: `SetHealthUnhealthy`를 미관측 커넥션에 호출 → 라인이 **생기지 않는다**(no-op 분기 단얫 — r1은 이 분기를 무검증으로 뒀다).
- **T-8 소거 대칭**(P3b, H-3·H-7): `TestSweepOnceRemovesVanishedSeries`(`healthloop_test.go:171`)를 **긍정/부정 쌍**으로 재작성한다 — (i) prune 전 `healthLine(...)` 긍정 단얫으로 시리즈 존재 확정, (ii) 커넥션 행 삭제 + 재 sweep, (iii) `provider_health{connection="conn-vanish",`(라벨 경계 `,`로 끝남 — 향후 라벨 추가에도 매치 유지) 부재, (iv) **`inventory_sync_last_success_timestamp_seconds{connection="conn-vanish"}` 부재**. (i)이 없으면 (iii)(iv)는 무의미하다(§4.2(2)).
- **T-9 zero 거부**(P3a, H-8): `c.SetSyncLastSuccess("conn", time.Time{})` → `Render()`에 `inventory_sync_last_success_timestamp_seconds` **문자열 미등장**. 음수 epoch(`time.Unix(-1,0)`)도 같다. 이 테스트가 없으면 `18446744011573954816` 회귀가 조용히 통과한다.
- **T-10 단조**(P3a, H-4): `SetSyncLastSuccess("conn", time.Unix(2000,0))` → `SetSyncLastSuccess("conn", time.Unix(1000,0))` → 렌더 값이 **2000 유지**. 역순(1000 → 2000)은 2000으로 전진. 복원-라이브 경합의 축소 모형이다.
- **T-7 하위 호환**(전 Phase): §4.4 목록 전체 **무수정 통과**. 단 §4.4 말미 경고대로, 이는 "본 과제 무관 패밀리 확인"이지 본 과제의 검증이 아니다.

**로컬 완료 게이트 (r2 확장 — M-1).** 이 저장소는 원격 CI 대기 금지 정책이므로 **로컬 게이트가 유일한 방어선**이다. 세 명령을 전부 통과해야 완료다:
```
cd backend && go test ./... -race -count=1
./scripts/check-arch-boundary.sh
python3 ./scripts/secret-scan.py
```
- `check-arch-boundary.sh`의 R2 `CORE_PACKAGES`(`:35`)에 **`internal/infra/metrics`와 `internal/infra/inventory`가 둘 다 포함**되고, R2 패턴(`:49` `\b(aliyun|tencent|alicloud|tencentcloud|proxmox)\b`)은 `--exclude='*_test.go'`로 **비테스트 `.go`만** 훑는다. 예외는 `contract/provider_type.go`의 어휘 선언 라인뿐(`:50-52`).
- **실질 위험**: 본 과제가 만지는 `gauge.go`·`render.go`·`metrics.go`·`sync.go`·`restore.go`의 **주석에 provider 제품명을 한 줄만 써도 R2 FAIL**이다. 골든 예시를 주석에 옮겨 적는 것이 전형적 사고 경로다 — 보존 제약 16이 이를 금지한다.
- 테스트 파일은 제외되므로 `metrics_test.go` 골든의 `provider="aliyun"`·`"kubernetes"`는 안전하다. `healthloop_test.go`의 `fake.ProviderName`도 안전(패턴에 `fake` 없음, 게다가 테스트 파일).

---

## 9. 위험 지점과 롤백

### 9.1 위험
- **R1 `provider_health` 시계열 신원 변경 + 착지 순서 제약** — 유일한 비-추가형 변경. 기존 대시보드·알람이 없다는 전제(가정 A7, 실측 확인) 하에 수용. 검증: T-1·T-6. 완화: P1 단독 revert.

  > **착지 순서 제약 (팀리드 확정 3, H-12).** **본 계획 P1(`provider_health` 라벨)은 `otel-lgtm-recipe` P2(Alloy 스크랩 개시)보다 반드시 먼저 착지한다.** 현재 `deploy/alloy/`는 존재하지 않으므로(실측: `ls deploy/` → `config.yaml.example` 단일) 스크랩 개시 전 라벨 변경의 전환 비용은 **0**이다. 스크랩이 개시된 뒤 라벨을 바꾸면 이전 라벨 집합의 시계열이 TSDB에 고아로 남고, **이것은 코드 revert로 회수되지 않는다**(코드는 되돌아가도 저장된 시리즈는 남는다). 제약 대상은 **P1뿐** — P2~P5는 recipe와 병행 가능하다. 같은 문장이 `otel-lgtm-recipe-plan.md §8 P2`에도 들어가야 상호 구속이 성립한다(§13-5).
- **R2 사전 단계 계기의 카디널리티** — §5 마지막 문단이 유계임을 실측으로 보였으나, 향후 새 `RunSync` 호출부가 외부 입력 UID를 넘기면 전제가 깨진다. 완화: 보존 제약 10으로 못 박고, claim 12가 호출부 수를 검증한다.
- **R3 복원 질의가 부팅을 막음** — 로그 후 계속 처분으로 차단. 검증: T-5의 nil/에러 갈래 + claim 9.
- **R4 `Render()` 디스패치 순서 오배치** — 표 순서와 어긋나면 골든이 표와 불일치한다. 검증: T-1 골든이 기계적으로 잡는다.
- **R5 `New()` 맵 미초기화 → 런타임 panic** — 테스트 실패가 아니라 **크래시**다. `metrics.go:84-98`에 `syncRuns`·`lastSyncSuccess` 초기화를 반드시 추가하고, `health`는 값 타입 변경에 맞춰 `make(map[string]healthSample)`로 정정한다. 검증: claim 5(코드 검수) + T-1(초기화 누락 시 즉시 panic).
- **R6 exposition 유효성 회귀** — `inventory_sync_total`은 counter이므로 `_total` 접미사 유지(후속 `otel-lgtm-recipe`의 전제, `otel-telemetry-assessment.md:61`). 타임스탬프 gauge는 `_total`을 **붙이지 않는다**(gauge에 `_total`을 붙이면 `prometheusreceiver`가 counter로 오판정). 검증: T-1 + claim 10.
- **R7 복원 집계값 스캔 실패의 무증상 착지** — §2.3.3 말미 참조. `MAX(committed_at)`이 드라이버에 따라 텍스트로 돌아와 zero `time.Time`으로 조용히 착지할 수 있다. 완화: 대안 2안 중 택1 + **setter zero 가드가 최종 방어선**(§2.3.1). 검증: T-5·T-9.
- **R8 zero epoch의 uint64 랩어라운드** (r2 신설 — H-8) — `itoa(v uint64)`(`render.go:259`)에 `time.Time{}.Unix() = -62135596800`을 넣으면 **`18446744011573954816`**이 렌더된다(실행 확인). `time()-gauge`가 거대 음수 → 알람이 터지지 않고 **영구 침묵**한다. R7보다 나쁜 실패다(R7은 1970년 값이라 최소한 눈에 띈다). 완화: setter 자체 가드. 검증: T-9.
- **R9 커넥션 소멸 후 gauge 잔존** (r2 신설 — H-3) — 소거 없으면 `time()-gauge` **영구 오발** + 맵 단조 증가. `provider_health`에는 이미 있는 대칭(`RemoveHealth`↔`pruneVanished`)이 신규 gauge에만 없던 상태. 완화: §2.3.4 1줄 배선. 검증: T-8(iv).
- **R10 라벨 정렬 누락 → 렌더 비결정성** (r2 신설 — M-7) — 3라벨 패밀리는 `sortedCounterKeys` 재사용이 불가하고 `sort.Slice` 3단 비교를 손으로 써야 한다. 한 단계를 빠뜨리면 맵 순회 순서가 새어 나온다. **단일 시리즈 골든으로는 잡히지 않는다.** 완화: §4.1(2) 다중 시리즈 골든. 검증: T-1 + `metrics_test.go:113-116` 결정성 검사.
- **R11 로컬 아키텍처 게이트 위반** (r2 신설 — M-1) — 신규/수정 비테스트 `.go`의 **주석 한 줄**에 provider 제품명이 들어가면 `check-arch-boundary.sh` R2가 FAIL한다. 원격 CI 대기 금지 정책상 이 게이트를 로컬에서 돌리지 않으면 착지 후에야 발견된다. 완화: 보존 제약 16 + 완료 게이트 3종(§8).

### 9.2 롤백 가능성 + **forward-stop 비용** (r2 확장 — H-10)

r1의 §9.2는 revert 비용만 다뤘다. 그러나 이 계획의 실제 위험은 **되돌리는 것이 아니라 중간에 멈추는 것**이다 — Phase가 부분 착지한 상태가 착지 전보다 나쁠 수 있다.

**forward-stop 표 — "여기서 멈추면 어떻게 되는가"**

| 멈춤 지점 | 상태 | 착지 전 대비 | 처분 |
|---|---|---|---|
| P1 후 | `provider_health` 2라벨. 소비자 없음(스크랩 미개시) | 중립 | 허용 |
| P2 후 | `inventory_sync_total` 발화 중, 소비자 없음 | 중립(추가형) | 허용 |
| P3a 후 | gauge 패밀리 존재하나 **발화 0** → 렌더에 미등장 | 중립(빈 상태 계약) | 허용 |
| **P3b 후 (P4 없이)** | gauge 라이브 발화 + **복원 없음**. 재시작마다 시리즈 소실 → `time()-gauge` 알람이 **조용히 빈칸**. 계획 스스로 §2.3.2에서 "치명적"이라 부른 상태이고, 이제는 **소비자가 생긴다** | **착지 전보다 나쁨** — 착지 전에는 패널이 아예 없었다 | **금지.** P3b·P4는 같은 PR 또는 연속 착지 |
| P4 후 | 완결. 문서만 미개정 | 코드는 정상, 스펙 불일치 | 허용하되 P5 필수(§13-6) |
| P5 후 | 완료 | — | — |

**revert 등급**
- **P2·P3a·P3b·P4 — 완전 추가형.** 패밀리 신설·계기 추가·복원/소거 회로뿐이라 revert 시 잔여 효과 0. 스키마·시드·제어흐름 무변경. (P3b는 P3a에, P4는 P3a에 의존하므로 revert는 역순.)
- **P1 — 비-추가형.** revert하면 시계열 신원이 원복되며 그 사이 수집된 2라벨 시리즈는 TSDB에 고아로 남는다. **스크랩 개시 전이면 이 비용이 0**이므로 R1의 착지 순서 제약이 곧 롤백 비용 통제책이다.
- **P5 — 문서.** 코드 무영향.
- 최소 롤백 단위: health 라벨 = `gauge.go` 시그니처 + `healthloop.go` 2줄. sync 카운터 = `sync.go` 9줄. 복원 = `engine_config.go` 2줄. 소거 = `healthloop.go` 1줄.
- **DB 롤백 불필요** — 마이그레이션·컬럼 추가 0건, 신규 질의는 전부 읽기 전용.

주의: `metrics-plan.md:135`의 "전 Phase 추가형이라 롤백 안전" 판정을 **그대로 승계하지 않는다** — P1이 비-추가형이고, P3b가 forward-stop 금지 구간이다.

---

## 10. 검증 요구 (claims — 구현 완료 후 ④리뷰가 검증)

1. `provider_health` 렌더 라인이 `{connection,provider}` 2라벨이고 connection이 앞이다 — `cd backend && go test ./internal/infra/metrics/ -run TestRenderFullFamilyGolden -race -count=1` 통과 + `grep -n 'provider_health{connection=' internal/infra/metrics/metrics_test.go`가 `,provider="` 를 포함하는 2줄을 낸다.
2. `SetHealth`의 provider 인자는 `conn.ProviderType`에서 온다 — `grep -n 'SetHealth(' backend/internal/infra/compose/healthloop.go`가 `conn.UID, conn.ProviderType, healthy` 형태 1줄을 낸다.
3. **소거 대칭이 성립한다** — `pruneVanished`(`healthloop.go`)가 `RemoveHealth`와 `RemoveSyncLastSuccess`를 **둘 다** 호출하고, `TestSweepOnceRemovesVanishedSeries`가 (i)prune 전 긍정 단얫 (ii)`provider_health{connection="conn-vanish",` 부재 (iii)`inventory_sync_last_success_timestamp_seconds{connection="conn-vanish"}` 부재를 전부 단얫한다 — 테스트 본문 검수 + `cd backend && go test ./internal/infra/compose/ -run TestSweepOnceRemovesVanishedSeries -race -count=1`. **부정 단얫만 있는 형태면 불합격**(리터럴이 절대 매치되지 않아도 통과하므로).
4. `inventory_sync_total`이 counter로 렌더되고 `_total` 접미사를 가지며 `{connection,mode,status}` 3라벨이다. **정렬은 `sort.Slice` 3단 비교이고 골든이 4개 이상의 다중 시리즈를 담는다** — `cd backend && go test ./internal/infra/metrics/ -run 'TestRenderFullFamilyGolden|TestRenderTypeHeaders' -race -count=1` + 골든의 `inventory_sync_total` 라인 수 ≥ 4이며 connection·mode·status 세 자리에서 각각 갈리는 조합을 포함한다.
5. `New()`가 `syncRuns`·`lastSyncSuccess`를 초기화하고 `health`를 `map[string]healthSample`로 초기화한다 — `sed -n '/func New()/,/^}/p' backend/internal/infra/metrics/metrics.go`에 3개 항목이 전부 보인다(누락 시 nil map write panic).
6. **`inventory_sync_total`이 `RunSync`의 모든 종단 경로에서 정확히 1회 발화한다 — 경로별로 판정한다.** `grep -c`는 근거로 쓰지 않는다(한 경로 2회 + 다른 경로 0회도 8을 낸다). 검증 2단:
   (a) `cd backend && go test ./internal/infra/inventory/ -run TestSyncTotal -race -count=1` — §8 T-3 표의 **9개 케이스**(①②③④⑤⑥, ⑧-succeeded/⑧-partial/⑧-failed)가 전부 존재하고 통과한다. **⑧-failed 케이스 포함이 필수**다. ⑦은 격리 불가로 미덮음이며 그 사유가 테스트 주석에 적혀 있다.
   (b) 코드 검수 — `sed -n '/func (r \*SyncRunner) RunSync/,/^}/p' backend/internal/infra/inventory/sync.go`의 **모든 `return` 문 직전**에 `r.countRun(...)`이 정확히 1회 있고, 그 함수의 `return` 개수와 `countRun` 개수가 같다.
7. **`SetSyncLastSuccess`가 zero 가드와 단조 계약을 setter 자체에 갖는다** — `sed -n '/func (c \*Counters) SetSyncLastSuccess/,/^}/p' backend/internal/infra/metrics/gauge.go`에 ① `ts <= 0` 조기 return ② 기존값과의 비교 후 조기 return이 **둘 다** 보인다. 검증: `cd backend && go test ./internal/infra/metrics/ -run 'TestSyncLastSuccess' -race -count=1`(T-9 zero 거부 + T-10 단조).
8. **`inventory_sync_last_success_timestamp_seconds`는 succeeded run만 갱신한다** — `grep -n 'SetSyncLastSuccess' backend/internal/infra/inventory/sync.go`의 호출이 `status == RunStatusSucceeded` 분기 안에 있고, T-4의 partial 케이스(`sync_metrics_test.go:68-79` 픽스처 재사용)가 gauge 불변을 단얫한다.
9. **복원 호출이 `startEngineLane`에만 있고 `stack.BuildEngine`보다 앞이며 실패 시 로그 후 진행한다** — `grep -rn 'RestoreLastSuccess' backend/ --include=*.go`가 정의 1건 + `engine_config.go` 1건 + 테스트만 낸다(`compose.Build` 안에는 없다). 그리고 `grep -n 'RestoreLastSuccess\|BuildEngine\|taskEngine.Start' backend/engine_config.go`의 행 번호가 **RestoreLastSuccess < BuildEngine < Start** 순서다.
10. 타임스탬프 gauge가 정수로 렌더되고 `_total` 접미사가 없다 — 골든 라인이 `inventory_sync_last_success_timestamp_seconds{connection="conn-a"} 1757318400` 형태로 지수 표기 없이 존재한다. **`18446744` 문자열이 렌더 어디에도 없다**(T-9).
11. 복원 술어가 `projection.go:32,98`과 같다 — `grep -n "committed_at IS NOT NULL" backend/internal/infra/inventory/*.go`가 `projection.go` 2건 + `restore.go` 1건을 낸다. `restore_test.go`는 `sync_test.go`의 기존 하네스(`newInventoryDB`·`seedConnection`, `package inventory_test`)를 재사용하고 신규 하네스를 만들지 않는다.
12. `RunSync`의 비테스트 호출부는 여전히 2곳뿐이다(§5 (가)(나) 전제) — `grep -rn 'RunSync(' backend/ --include=*.go | grep -v _test`가 정의 1 + `main_sync.go:97` + `compose/compose.go:204` = 3줄.
13. 스펙 §18.2 표가 18 데이터행이고 `provider_health` 라벨 열이 `connection, provider`이며 신규 2행이 `inventory_sync_partial_total` 직후에 있고, §3.2 두 문단이 표 뒤에 있다 — **행 번호가 아니라 절 제목으로 찾는다**(§3.0): `awk '/^### 18.2 /,/^### 18.3 /' docs/architecture/multi-infrastructure-control-plane-v2.md`. 그 출력의 `failed` 서술이 "pre-flight 전용"으로 한정되어 있지 **않아야** 한다.
14. "14종" 문언이 코드와 재개 문서에 남아 있지 않다 — `grep -rn '14종' backend/internal/infra/metrics/ v2-phase1/RESUME.md` == 0건. (`phase6-entry-assessment.md`·`otel-*` 문서의 잔존은 §3.4 판정상 의도된 것이며 이 claim의 범위 밖이다.)
15. `metrics` 패키지는 여전히 stdlib만 import 한다 — `cd backend && go list -f '{{join .Imports "\n"}}' ./internal/infra/metrics/`의 출력이 `sync`·`sort`·`strings`·`strconv`·`time` 만이다(주석·문자열 리터럴을 훑는 grep이 아니라 실제 import 그래프를 본다). 아울러 `git diff --name-only`에 `backend/go.mod`·`backend/go.sum`·`docs/security/route-inventory.txt`·`docs/security/sensitive-routes.txt`가 없다.
16. 수정된 기존 테스트 파일은 `metrics_test.go`·`healthloop_test.go`·`sync_metrics_test.go` 3개뿐이다 — `git diff --stat -- '*_test.go'`. (r1의 "`healthloop_test.go` diff 1줄"은 T-8 재작성으로 **폐기**한다.)
17. **로컬 아키텍처·시크릿 게이트 통과** — `cd /mnt/d/DEV/acc0mplish/ops-admin && ./scripts/check-arch-boundary.sh && python3 ./scripts/secret-scan.py` 둘 다 종료 코드 0. 특히 신규/수정 **비테스트** `.go`(`gauge.go`·`render.go`·`metrics.go`·`sync.go`·`healthloop.go`·`restore.go`)의 주석·문자열에 `aliyun|tencent|alicloud|tencentcloud|proxmox`가 **0건**이다(R2 패턴 `check-arch-boundary.sh:49`).
18. 전체 테스트 게이트 통과 — `cd backend && go test ./... -race -count=1`.
19. **착지 순서 준수** — P1 착지 시점에 `deploy/alloy/`가 존재하지 않는다(`ls deploy/`). 존재한다면 recipe P2가 먼저 착지한 것이므로 R1 제약 위반이고, 라벨 변경 전에 팀리드 재판정이 필요하다.
20. **P3b·P4 동시 착지** — `git log --oneline`에서 `sync.go`의 `SetSyncLastSuccess` 추가 커밋과 `restore.go` 추가 커밋이 같은 PR에 있거나 연속이다(§9.2 forward-stop).

---

## 11. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **`metrics` 패키지 순도** — 표준 라이브러리만 import 한다(`backend/internal/infra/metrics/metrics.go:8-10` 주석 계약). client_golang·otel SDK를 도입하지 않는다.
2. **§10.2 "No new runtime dependencies in M1"** — `backend/go.mod`·`backend/go.sum`은 무변경이다.
3. **프로덕션 동작 무변경** — 동기화·태스크 실행·재큐·health sweep의 판정 로직을 바꾸지 않는다. 계기 삽입과 렌더 확장만 한다. `RunSync`의 제어 흐름(분기·return 조건·순서)은 그대로 두고, 각 return 앞에 계기 호출 1줄만 더한다.
4. **`New().Render() == ""` 빈 상태 계약**(`metrics_test.go:66` TestNewZeroState)을 보존한다 — 패밀리 부재 시 `# TYPE` 헤더를 발행하지 않는다.
5. **렌더 결정성** — 패밀리·라벨 정렬과 라벨값 이스케이프(`\`·`"`·개행) 계약을 보존한다(`TestRenderDeterministicAndSorted`·`TestRenderLabelValueEscaping`).
6. **nil Counters 무동작** — 계기 부재가 동기화 결과·종단·재큐를 바꾸지 않는다. `SyncRunner.countRun`은 `r.counters == nil`에서 즉시 반환한다.
7. **기존 12종 라인 byte 불변** — `provider_rate_limit_total`·`provider_api_errors_total`을 포함한 기존 패밀리의 라인 형태는 계약이다(`metrics-plan.md §1.2`). `compose_test.go:59,108,264`·`compose_proxmox_test.go:53`·`contracttest/harness.go:183,193`·`sync_test.go:537-541`·`metrics_test.go:36-56`은 **수정하지 않는다**.
8. **`inventory_sync_duration_seconds`의 라벨은 `{connection, mode}` 그대로다** — status 라벨을 추가하지 않는다. `ObserveSyncDuration` 시그니처 불변, `sync_metrics_test.go:54,81` 무수정.
9. **라우트 골든 무변경** — `docs/security/route-inventory.txt`·`sensitive-routes.txt`를 재생성하지 않는다. 신규 엔드포인트는 없다.
10. **Prometheus text exposition 유효성** — counter 패밀리는 `_total` 접미사를 유지하고 gauge에는 붙이지 않는다(`prometheusreceiver`→OTLP의 counter/gauge 판정이 이름 접미사에 걸린다). `# TYPE` 헤더는 패밀리당 1회, 값 라인 직전에 둔다. histogram의 `le` 오름차순 + `+Inf` + `_sum` + `_count` 형상을 유지한다. 이는 후속 과제 `otel-lgtm-recipe`의 전제다.
11. **`Render()`의 패밀리 순서는 §18.2 표 순서와 1:1을 유지한다**(`render.go:33-46`). 신규 패밀리는 표 삽입 위치와 같은 자리에 디스패치한다.
12. **`tasks`·`adapter`·`secrets` 패키지는 본 과제에서 무변경이다** — task·worker·secret 패밀리에 손대지 않는다.
13. **`agent_heartbeat_age_seconds`·`outbox_backlog`는 §18.2 deferred 그대로** 계기하지 않는다.
14. **파일 줄 수** — 650줄 경고선·800줄 하드캡. 본 과제가 성장시키는 모든 파일에 `wc -l`을 돌리고 완료 보고에 수치를 적는다.
15. **복원 회로는 `compose.Build`에 넣지 않는다** — Build 호출부는 프로덕션 3곳(`engine_config.go:139`·`main_sync.go:82`·`internal/api/v2/infra.go:54`)과 테스트 11곳이고, 그중 `/internal/metrics`를 서빙하는 것은 첫 번째뿐이다. Build는 "무엇을 서빙하는가"를 모르는 층이다. 복원은 `engine_config.go startEngineLane` 전용이며 `stack.BuildEngine`보다 앞이고, 질의 실패는 로그 후 계속이다.
16. **신규·수정 비테스트 `.go` 파일의 주석·문자열에 provider 제품명을 쓰지 않는다** — `aliyun`·`tencent`·`alicloud`·`tencentcloud`·`proxmox`. `scripts/check-arch-boundary.sh` R2가 `internal/infra/metrics`·`internal/infra/inventory`를 `CORE_PACKAGES`(`:35`)로 스캔하며 `--exclude='*_test.go'`(`:49`)이므로 **주석 한 줄이면 FAIL**이다. 골든 예시를 소스 주석으로 옮겨 적지 않는다. 예시가 필요하면 `conn-a`·`p`·`stub` 같은 중립 리터럴을 쓴다. 테스트 파일은 제외 대상이므로 골든의 `provider="aliyun"`은 그대로 둔다.
17. **`SetSyncLastSuccess`의 zero 가드와 단조 계약은 setter 안에 있어야 한다** — 호출부 가드로 대체하지 않는다. `restore.go`나 `sync.go` 쪽 가드는 이중 방어일 뿐이다.
18. **소거 대칭을 깨지 않는다** — 커넥션 스코프 gauge를 추가하면 `pruneVanished`에도 소거를 넣는다. `provider_health`의 `RemoveHealth`↔`pruneVanished` 대칭이 그 선례다.
19. **부정 단얫만으로 소거·부재를 검증하지 않는다** — `strings.Contains`가 거짓이면 통과하는 형태는 리터럴이 형상 변경으로 영영 매치되지 않게 되어도 통과한다. 부재 단얫에는 **직전에 존재를 확정하는 긍정 단얫**을 짝지운다. 부재 리터럴은 닫는 `}`가 아니라 라벨 경계 `,`에서 끊는다.

---

## 12. 가정 (불확실 요구사항의 명시적 처리)

- **A1**: `provider_connection` 행 수에 코드상 상한이 없다(실측: 스키마·검증 로직에 제한 없음). 운영 규모는 수십으로 가정하고 §5의 카디널리티 산정 기준으로 삼는다.
- **A2**: 장기 상주(스크랩 대상) 프로세스에서 `RunSync`를 호출하는 유일 경로는 `compose.go:204`이며 그 UID는 DB join 산물이다(실측 claim 12). CLI 경로의 임의 UID는 프로세스 종료로 소멸하므로 시계열을 남기지 않는다.
- **A3**: `mode` 어휘는 `full`/`incremental`/`targeted` 3종(`sync.go:48` 주석)이며, 현재 발화되는 값은 `full` 뿐이다(§3.3 r2 — Phase 2 실행 경로 full 전용, `compose.go:204` Mode 미지정→`sync.go:65-68` 정규화). **단 이는 어휘 검증이 아니라 관례다** — `SyncInput.Mode`는 검증 없는 자유 문자열이고 CLI는 임의 값을 넘길 수 있다(§5 (나), L-4). 카디널리티 유계성은 "CLI는 스크랩되지 않는다"에 기댄다.
- **A4**: 성공 갈래에서 `committed_at == finished_at`이다(`sync.go:107,128,131` 실측 — 셋 다 `finished` 변수). 따라서 복원값(`committed_at`)과 라이브 발화값(`finished`)이 같다.
- **A5**: `status` 라벨 어휘는 `reconcile.go:45-48`의 종단 3종(`succeeded`/`partial`/`failed`)이다. `running`은 종단이 아니므로 발화하지 않는다.
- **A6**: `provider` 라벨값은 `provider_connection.provider_type` 컬럼값 그대로이며, 기존 `provider_*` 패밀리의 provider 어휘(어댑터 `ProviderName()`)와 같은 공간이다. 신규 어휘를 만들지 않는다.
- **A7**: 기존 Grafana 대시보드·알람 규칙이 없다(과제 지시문). 따라서 `provider_health`의 비-추가형 라벨 변경이 외부 소비자를 깨지 않는다.
- **A8**: 타임스탬프 gauge는 초 단위 정수로 충분하다(패널 용도 — 밀리초 정밀도 불요). 단조 계약상 같은 초 안의 두 성공은 뒤엣것이 반영되지 않는데(`prev >= ts`), 초 단위 해상도에서 의미 있는 차이가 아니므로 수용한다.
- **A9** (r2 신설 — L-6): ⑥`reconcileAbsences` 실패·⑦확정 `Updates` 실패에서 **메트릭은 `failed`를 세고 DB 원장 `inventory_sync_run.status`는 `running`으로 남는다**(⑥은 `sync.go:133` 갱신 이전, ⑦은 갱신 자체가 실패). 의도된 비대칭이다 — 메트릭은 운영자 관점의 run 결과를, 원장은 DB에 확정된 사실을 말한다. 복원 질의는 `status='succeeded'` 술어라 이 행들을 성공으로 오인하지 않는다. 고아 `running` 행을 정리하는 회로는 본 과제 범위 밖이다(§13-8).

---

## 13. 미해결 판정 · 사용자 확인 필요

1. **`otel-telemetry-assessment.md:60-63`의 골든 행 번호 인용이 본 과제 후 어긋난다.** 그 문서는 조사 산출물이지 계약이 아니므로 본 계획은 재작성하지 않는다. 행 번호를 지우고 패밀리 이름만 남기는 1줄 정정을 원하면 별도 지시가 필요하다. **기본값: 무변경.**
2. **`inventory_sync_partial_total`의 장기 처분.** `inventory_sync_total{status="partial"}`로 완전히 유도 가능해지므로 중복이다. 본 과제는 §18.2 계약 행이라는 이유로 **유지**한다. deprecate 하려면 스펙 §18.2에서 행을 제거하는 별도 결정이 필요하다.
3. **`/internal/metrics` 노출 증분 — r1의 "같은 등급" 판정을 철회하고 실제 증분을 적는다** (M-9). 무인증 GET 하나로 다음이 **결합되어** 나온다: 커넥션 UID 집합(=커넥션 수) × 각각의 **인프라 종류**(신규 — provider_type) × 건강 상태 × **마지막 성공 동기화 시각**(신규). 개별 필드가 아니라 이 **조인**이 증분이다 — 조직의 인프라 구성(어떤 클라우드/하이퍼바이저를 몇 개 쓰는가)과 운영 상태(어느 커넥션이 며칠째 동기화 실패인가)를 한 번에 읽을 수 있다. r1은 이를 "이미 노출 중인 것과 같은 등급"이라 적었으나 **논증이 없었다**.
   - 여전히 미노출인 것: 자격증명, 엔드포인트 URL, 리소스 식별자·이름, 사용자 정보.
   - **기록할 아티팩트도 게이트도 없다** — `docs/security/route-inventory.txt`·`sensitive-routes.txt`에 `internal/metrics` 0건(실측). 즉 이 증분은 어떤 보안 골든에도 남지 않는다.
   - **판정 요청**: (a) 현행 전제(배포 경계 시행, `metrics-plan.md §1.3` A1)를 유지하고 증분을 수용, (b) 런북에 노출 목록을 명문화, (c) `sensitive-routes.txt`에 `/internal/metrics`를 등재해 이후 변경이 골든 diff로 드러나게 함. **본 계획의 기본값은 (a)** — (b)(c)는 스펙·골든 개정이라 본 과제 범위 밖이다.
4. **복원 대상을 last-success gauge 1종으로 한정한 판정.** counter 패밀리(`inventory_sync_total` 포함)는 복원하지 않는다 — Prometheus가 counter reset을 처리하고, DB 원장에서 정확한 누적을 재구성하려면 run 전수 집계가 필요해 부팅 비용이 커진다. 이견이 있으면 별도 지시가 필요하다.
5. **병행 과제 `otel-lgtm-recipe`에 전달해야 할 3건** (H-11·H-12·M-11). 본 계획은 recipe 파일을 수정하지 않으므로(양 계획의 경계 규칙) **팀리드가 recipe 쪽에 반영해야 한다**:
   - `otel-lgtm-recipe-plan.md:455` — P7 단순화 서술을 `inventory_sync_duration_seconds_count{status="partial"}` → **`inventory_sync_total{status="partial"}`**로 정정. 현행 문언대로 착지하면 그 PromQL은 에러가 아니라 **빈 결과**를 낸다.
   - `otel-lgtm-recipe-plan.md §8 P2` — R1 착지 순서 제약 문장(§9.1) 삽입. 한쪽에만 있으면 구속되지 않는다.
   - `otel-lgtm-recipe-plan.md:25,336,458,549` — 골든 행 범위 리터럴(`metrics_test.go:118-218`)이 본 과제 후 밀린다. 범위 고정 대신 백틱 경계 탐색 권고(M-11).
6. **스펙-코드 대조 자동 게이트 부재** (M-8, 인정된 구멍). `.github/workflows/` 2파일 + ci-baseline required 9종에 대조 잡 0건 — P5를 통째로 빠뜨려도 CI는 녹색이다. 방어선은 claim 13·14의 **수동 검증**뿐. 게이트 신설은 스펙에 없는 CI 잡 추가라 본 과제 범위 밖이며, 별도 과제로 올릴지 판정이 필요하다.
7. **`SyncInput.Mode` 어휘 미검증** (L-4). 자유 문자열이 라벨이 되는 구조적 결함이다. 장기 프로세스에서는 유계지만(§5 (나)), 검증 추가는 프로덕션 판정 로직 변경(보존 제약 3)이라 본 과제가 하지 않는다. 별도 과제 판정 필요.
8. **⑥⑦의 고아 `running` 행** (A9). 메트릭은 `failed`를 세지만 DB 원장에는 `running` 행이 남는다. 정리 회로(예: 부팅 시 stale `running` 행을 `failed`로 마감)는 프로덕션 동작 추가이므로 범위 밖이다.
9. **`ProviderConnection.Name` 미채택 — 사유** (M-3). 요구 2("대시보드에 hex UID가 그대로 노출")의 나머지 절반은 사람이 읽는 이름이고, `model/provider.go:14`에 `Name string`이 실재한다. **채택하지 않는다**: ① `Name`은 사용자 입력 자유 문자열이라 변경 가능하고, 라벨값이 바뀌면 시계열 신원이 갈라진다(같은 커넥션이 두 시리즈가 됨) — Prometheus 라벨로는 부적합한 성질이다. ② 사람이 읽는 이름은 Grafana 쪽에서 붙이는 것이 관례다(라벨 join 또는 대시보드 변수). ③ 스펙 §18.2 라벨 열에 없다. **`provider_type`을 택한 것은 그것이 불변 분류값이기 때문**이다. UID→Name 매핑이 대시보드에 필요하면 recipe 소관이며, 그 경우 별도 `*_info` gauge 패턴(`connection_info{connection,name} 1`)이 정석이나 본 과제는 신설하지 않는다.
10. **`inventory_sync_partial_total` 폐기 비용이 상승한다** (M-10). `inventory_sync_total{status="partial"}`로 유도 가능해져 중복이지만, recipe P7(`otel-lgtm-recipe-plan.md:390`)이 그 메트릭을 **대시보드 질의 계약으로 승격**한다. 폐기 비용이 "§18.2 표 1행 제거"에서 "두 계약 절 개정 + 패널 재작성"으로 오른다. 중복 자체는 **발산 불가**하므로(발화 조건이 동일 — gaps 재검증 완료) 데이터 정합성 위험은 없다. 본 과제는 **유지**하되, 폐기하려면 지금이 가장 싸다는 점을 판정 대상으로 올린다.

---

## 14. r1 갭 상환 대조표 → **부록 §14**

HIGH 12 / MEDIUM 11 / LOW 8 전 항목의 해소 위치·방식 대조표는 **`v2-phase1/metrics-ext-plan-evidence.md` §14**에 있다.
