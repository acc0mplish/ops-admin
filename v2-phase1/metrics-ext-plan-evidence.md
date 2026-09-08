# metrics-ext-plan 부록 — 실측 증거·파괴 범위 전수·r1 갭 상환 (r2)

본 파일은 `v2-phase1/metrics-ext-plan.md`의 부록이다. 계획 본문이 650줄 한도를 넘어 **증거·전수 목록·검토 대조** 세 층을 분리했다. 절 번호는 본문의 것을 그대로 쓴다(§1 · §4 · §14).

- 본문: `v2-phase1/metrics-ext-plan.md`
- r1 갭 원본: `docs/task-id/otel-m18-ext/gaps-r1.md`
- 실측 기준: main `30856a3`, 2026-09-08

---

## 1. 현재 코드 실측 (계획의 사실 기반)

### 1.1 메트릭 패키지 현황
| 파일 | 줄 수 | 역할 |
|---|---|---|
| `backend/internal/infra/metrics/metrics.go` | 185 | 라벨 키 struct 6종(`:18-52`), `Counters`(`:56-80`), `New`(`:83-99`), counter Inc 메서드 9종 |
| `backend/internal/infra/metrics/gauge.go` | 63 | `SetHealth`(`:10`), `RemoveHealth`(`:19`), `SetQueueDepth`(`:27`), `renderHealth`(`:36`), `renderQueueDepth`(`:57`) |
| `backend/internal/infra/metrics/histogram.go` | 163 | 버킷 상수(`:10-13`), `histSample`, Observe 3종, 히스토그램 렌더 3종, `writeHistogram`(`:154`) |
| `backend/internal/infra/metrics/render.go` | 277 | `Render()` 디스패치(`:28-48` — 14종 고정 순서), counter 렌더 9종, 원시 헬퍼(`writeHeader:197`, `writeSampleLine:207`, `labelPairs:222`, `joinLabels:231`, `escapeLabel:240`, `sortedCounterKeys:248`, `itoa:259`, `formatFloat:275`) |
| `backend/internal/infra/metrics/metrics_test.go` | 281 | `TestRenderFullFamilyGolden`(`:75-223` — 14종 전문 byte 골든 `:118-219`), `TestRenderTypeHeaders`(`:227-266`), 이스케이프·결정성·빈 상태 |

- **순도 계약**: `metrics.go:8-10` "imports the standard library only". 현재 import는 `sync`(metrics.go:14), `sort`/`strings`(gauge.go:3-6), `sort`/`strconv`/`strings`(render.go:3-7), `sort`/`strings`/`time`(histogram.go:3-7). **전부 stdlib** — 확장도 stdlib만 쓴다.
- **빈 상태 계약**: `New().Render() == ""`(`metrics_test.go:66-70`). 각 렌더 함수가 `len(map)==0` 조기 return으로 이행(예 `gauge.go:37-39`, `render.go:100-102`). 무라벨 gauge만 예외적으로 `queueDepthSet` bool을 별도로 든다(`metrics.go:79`, `gauge.go:58`) — 맵 기반 신규 패밀리는 이 bool이 **필요 없다**.
- **렌더 순서**는 §18.2 표 순서와 1:1이다(`render.go:33-46`의 주석 열이 곧 표의 순서). 신규 패밀리는 표 삽입 위치와 렌더 삽입 위치가 일치해야 한다.
- **float 렌더 실측**: `formatFloat`(`render.go:275-277`)는 `strconv.FormatFloat(v,'g',-1,64)`다. epoch 초(≈1.757e9)를 통과시키면 `1.7573184e+09`가 나온다 — exposition상 유효하나 골든 가독성과 대시보드 표시가 나빠진다. **타임스탬프는 float 경로를 타지 않는다**(§2.3.1).

### 1.2 동기화 계기 현황 — 확정점 1곳, 사각지대 7곳
`backend/internal/infra/inventory/sync.go` (488줄) `RunSync`(`:64-151`):

| return | 위치 | 조건 | 현재 계기 |
|---|---|---|---|
| ① | `:72` | `loadConnection` 실패(행 부재/DB 오류) | **없음** |
| ② | `:76` | `resolveDiscoverer` 실패(미등록 provider type / Discoverer 미구현) | **없음** |
| ③ | `:80` | `resolveContext` 실패(context 행 부재) | **없음** |
| ④ | `:84` | `resolveConnectionView` 실패(broker 해석 실패) | **없음** |
| ⑤ | `:93` | run 행 `Create` 실패 | **없음** |
| ⑥ | `:105` | `reconcileAbsences` 실패 | **없음** |
| ⑦ | `:134` | run 행 `Updates`(확정) 실패 | **없음** |
| ⑧ | `:150` | 정상 종단 | duration·changes·partial (`:139-149`) |

- `mode`는 `:65-68`에서 **첫 return보다 먼저** 정규화된다 — 8개 지점 전부 `mode` 라벨을 확보한다.
- 확정점 블록(`:139-149`)은 `if r.counters != nil` 가드 안에 있고, 끝에서 `report.MetricsText = r.counters.Render()`(`:148`)로 CLI 산출물을 굳힌다 — **신규 계기는 이 Render 호출보다 앞에 놓여야** 산출물이 자기 일관성을 유지한다.
- 종단 상태 어휘: `reconcile.go:45-48` — `running`/`partial`/`succeeded`/`failed`. `:137`에서 `report.Status = status`.
- 성공 갈래에서 `finished`(`:98`)는 세 곳에 같은 값으로 쓰인다: `report.CommittedAt`(`:107`), `updates["finished_at"]`(`:128`), `updates["committed_at"]`(`:131`). → **DB 복원값과 라이브 발화값이 동일**하다(가정 A4).
- `RunSync` 비테스트 호출부는 2곳: `backend/main_sync.go:97`(CLI — 사용자 입력 UID), `backend/internal/infra/compose/compose.go:204`(태스크 종단 관측 갱신 — UID는 `:190-197` join 결과이고 `:201`에서 공백 가드). **장기 상주 스크랩 프로세스에서 UID를 만드는 유일 경로는 후자**이므로 사전 단계 계기의 카디널리티는 DB 행 수로 유계다(§5).

### 1.3 provider_health 계기 현황
`backend/internal/infra/compose/healthloop.go` (219줄):
- `SweepOnce`(`:121`)가 `s.db.Find(&conns)`(`:123`)로 `[]model.ProviderConnection` 전체를 로드 → `:130-139` 루프에서 `s.counters.SetHealth(conn.UID, healthy)`(`:133`).
- **provider_type은 이미 손에 있다**: `model.ProviderConnection.ProviderType`(`backend/internal/infra/model/provider.go:13`), `probe`가 `:179`에서 `s.registry.ProviderType(conn.ProviderType)`로 소비 중.
- `markObservedUnhealthy`(`:207-213`)는 `s.observed []string`(UID만)을 돌며 `SetHealth(uid, false)`(`:211`) — provider_type을 모른다. **여기가 라벨 추가의 유일한 구조적 마찰점**이다(§2.2.2가 해소).
- `setObserved`(`:215-219`)는 **호출부 0건**(`grep -rn "setObserved" internal/` → 정의 1줄뿐). 미사용 메서드는 Go에서 컴파일 오류가 아니므로 잔존한다. `observed`의 타입을 바꾸면 이 죽은 코드가 컴파일을 깬다 — §2.2.2 권고안은 타입을 바꾸지 않으므로 무영향이다.
- gauge 렌더: `gauge.go:36-53` — `sort.Strings(conns)` 후 `labelPairs([2]string{"connection", conn})`. 정렬 키는 connection 하나이고 그것이 곧 맵 키다.

### 1.4 타임스탬프 원장 실측
- `model.InventorySyncRun`(`backend/internal/infra/model/inventory.go:58-74`): `ConnectionID`(`:61`), `Status`(`:64`), `CommittedAt *time.Time`(`:72`), `FinishedAt *time.Time`(`:73`). **connection UID는 없다** — `provider_connection` 조인 필요.
- 저장소 안의 "성공한 run" 술어 선례: `backend/internal/infra/inventory/projection.go:32`·`:98` — `status = ? AND committed_at IS NOT NULL` with `RunStatusSucceeded`. **복원 질의는 이 술어를 그대로 쓴다**(어휘 일관성).
- 부팅 경로: `backend/engine_config.go:138-156 startEngineLane` — `compose.Build`(`:139`) 실패 시 4-nil 반환(R11 경로, `:142`), 성공 시 `taskEngine.Start(baseCtx)`(`:152`)·`healthSweep.Start(baseCtx)`(`:154`)·`stack.Counters.Render` 반환(`:155`). 이 함수의 호출부는 서버 부트뿐이며 마이그레이션 이후다.
- `engine_config.go`의 현재 import(`:3-17`)에 `inventory`는 없다. 다만 **`package main`은 이미 `inventory`를 import 한다**(`main_sync.go:97` `inventory.SyncInput`) — 신규 모듈 경계가 생기지 않는다.

---

---

## 4. 파괴 범위 실측 — 깨지는 단얫 / 안 깨지는 단얫

### 4.1 `metrics_test.go` — 유일한 골든 파일 (깨짐, 필수 수정)

**(1) 골든 본문 `:119-120`**
```diff
-provider_health{connection="conn-aliyun"} 1
-provider_health{connection="conn-k8s"} 0
+provider_health{connection="conn-aliyun",provider="aliyun"} 1
+provider_health{connection="conn-k8s",provider="kubernetes"} 0
```
정렬은 connection 키 그대로이므로 **행 순서 불변**.

**(2) 골든 본문 `:174` 직후 삽입 — 다중 시리즈** (r2 강화 — M-7)

r1은 `inventory_sync_total`을 **단일 시리즈**로만 골든에 넣었다. 그러면 결정성 검사(`metrics_test.go:113-116` — `got != gotAgain`)가 **정렬 누락을 잡지 못한다**: 시리즈가 하나면 맵 순회 순서가 무의미하기 때문이다. 3라벨 패밀리의 정렬 버그는 정확히 이 검사가 잡아야 하는 것이므로, **세 라벨 각 자리에서 갈리는 시리즈**를 넣는다.

```diff
 inventory_sync_partial_total{connection="conn-a"} 1
+# TYPE inventory_sync_total counter
+inventory_sync_total{connection="conn-a",mode="full",status="failed"} 1
+inventory_sync_total{connection="conn-a",mode="full",status="succeeded"} 2
+inventory_sync_total{connection="conn-a",mode="incremental",status="succeeded"} 1
+inventory_sync_total{connection="conn-b",mode="full",status="partial"} 1
+# TYPE inventory_sync_last_success_timestamp_seconds gauge
+inventory_sync_last_success_timestamp_seconds{connection="conn-a"} 1757318400
+inventory_sync_last_success_timestamp_seconds{connection="conn-b"} 1757318401
 # TYPE provider_task_duration_seconds histogram
```
- 4개 시리즈가 **connection**(a/b)·**mode**(full/incremental)·**status**(failed/succeeded/partial) 세 자리 전부에서 갈린다 → 정렬 3단계 중 어느 하나가 빠져도 반복 렌더가 흔들리거나 순서가 어긋난다.
- last-success도 2 시리즈로 늘려 gauge 쪽 정렬(`sortedCounterKeys` 계열 단일 키)도 실증한다.

**정렬 계약 명시 (M-7)**: `sortedCounterKeys`(`render.go:248`)는 시그니처가 `map[string]uint64`라 **3라벨 복합 키에 쓸 수 없다**. `renderSyncTotal`은 `renderAPIErrors`(`render.go:77-85`)·`renderTaskFailures`(`:130-135`)와 같은 형상 — 키 슬라이스를 만들고 `sort.Slice`로 **`connection` → `mode` → `status` 3단 비교**를 직접 쓴다. 라벨 렌더 순서도 같은 순서다(`labelPairs([2]string{"connection",…}, {"mode",…}, {"status",…})`).

**(3) 골든 setup 호출부**
- `:80-81` → `c.SetHealth("conn-k8s", "kubernetes", false)` / `c.SetHealth("conn-aliyun", "aliyun", true)`
- `:94`(`IncSyncPartial`) 뒤에 (2)의 4+2 시리즈를 만드는 호출을 추가한다. 순서를 **일부러 역순·뒤섞어** 넣어야 정렬이 실증된다 — 예: `c.IncSyncRun("conn-b","full","partial")` → `c.IncSyncRun("conn-a","incremental","succeeded")` → `c.IncSyncRun("conn-a","full","succeeded")` ×2 → `c.IncSyncRun("conn-a","full","failed")`, `c.SetSyncLastSuccess("conn-b", time.Unix(1757318401,0))` → `c.SetSyncLastSuccess("conn-a", time.Unix(1757318400,0))`. 고정 상수라 골든은 결정적이다.
- **주의(단조 계약과의 상호작용)**: `SetSyncLastSuccess`는 단조이므로(§2.3.1), 같은 커넥션에 더 작은 값을 나중에 넣는 setup은 **적용되지 않는다**. 골든 setup은 커넥션별로 값을 1회씩만 넣는다.

**(4) `TestRenderTypeHeaders`**
- `:230` → `c.SetHealth("conn", "p", true)`
- `:243` 뒤 2줄 추가: `c.IncSyncRun("conn", "full", "succeeded")`, `c.SetSyncLastSuccess("conn", time.Unix(1, 0))`
- `:253` 뒤 기대 문자열 2개 추가: `"# TYPE inventory_sync_total counter"`, `"# TYPE inventory_sync_last_success_timestamp_seconds gauge"`

**(5) 무영향인 같은 파일 내 테스트**: `TestRenderLabelShapes`(`:10-34`), `TestRenderDeterministicAndSorted`(`:36-56` — rate-limit 3줄 정확 일치. **본 과제는 health/sync 패밀리를 건드리지 않는 이 테스트를 깨지 않는다**), `TestRegisterProvidersIgnoresEmpty`(`:58`), `TestNewZeroState`(`:66`), `TestRenderLabelValueEscaping`(`:269`).

### 4.2 `healthloop_test.go` — 2곳 (r2 정정 — H-7)

**(1) helper (`:69-71`) — 명시적 실패로 깨짐**
```diff
 func healthLine(render, uid, value string) string {
-	return `provider_health{connection="` + uid + `"} ` + value
+	return `provider_health{connection="` + uid + `",provider="` + fake.ProviderName + `"} ` + value
 }
```
- `fake`는 `:11`에 이미 import되어 있다 — 신규 import 없음.
- 소비처 `:86`·`:111`·`:142`·`:180` 4곳은 **helper 수정만으로 전부 복구**된다.

**(2) `:191` — 침묵으로 깨짐. r1이 "안 깨진다"고 오판한 지점.**
```go
if render := counters.Render(); strings.Contains(render, `provider_health{connection="conn-vanish"}`) {
    t.Fatalf("vanished connection series must be pruned:\n%s", render)
}
```
- 리터럴이 닫는 `}`로 끝나므로 라벨 append 후 **절대 매치되지 않는다**. 부정 단얫이라 컴파일도 실행도 통과하지만, **prune 회로를 통째로 지워도 초록**이다 — 회귀 검출력 0.
- r1이 이걸 "무수정 통과"로 분류한 것이 §0.1이 말하는 실패 유형 그 자체다: "기존 테스트가 통과한다"를 안전망으로 신뢰한 결과 안전망이 비어 있었다.
- **처분: 긍정+부정 쌍으로 재작성한다**(T-8). prune 전 라인 존재를 **긍정 단얫**으로 먼저 고정하고, prune 후 부재를 `healthLine(...)` helper 기반으로 확인한다 — helper를 쓰면 라벨 형상이 바뀔 때 컴파일/단얫이 함께 따라간다:
```go
// prune 전: 시리즈가 실제로 있다는 것을 먼저 증명한다(부정 단얫의 전제).
if want := healthLine(render, "conn-vanish", "1"); !strings.Contains(render, want) { t.Fatalf(...) }
// prune 후: 그 커넥션 스코프 라인이 사라졌다.
if strings.Contains(render, `provider_health{connection="conn-vanish",`) { t.Fatalf(...) }
```
  두 번째 리터럴이 `,`로 끝나는 것이 요점 — 닫는 `}` 없이 라벨 경계에서 끊어 **향후 라벨이 더 붙어도 계속 매치**된다.
- 같은 테스트에 **last-success 소거 단얫을 함께 넣는다**(§2.3.4·T-8): prune 후 `inventory_sync_last_success_timestamp_seconds{connection="conn-vanish"}` 라인 부재.

**(3) 무영향**: `TestHealthSweeperStartStopCancelsLoop`(`:149`)·`TestStopBeforeStartIsSafe`(`:198`).

### 4.3 `sync_metrics_test.go` — 증설 (기존 단얫은 무영향)
기존 단얫은 전부 통과한다: `:54`·`:81`(duration `{connection,mode}` — 라벨 무변경), `:58`·`:87`(changes), `:62`(succeeded run이 `inventory_sync_partial_total` 문자열을 만들지 않아야 함 — 신규 패밀리 이름 `inventory_sync_total`은 이 부분 문자열을 포함하지 않는다), `:84`, `:119`·`:126`·`:129`(stale). **증설만** 한다(§8 T-2/T-3/T-4).

### 4.4 안 깨지는 단얫 — 전수 확인 목록 (근거 포함)
| 지점 | 단얫 형태 | 안 깨지는 이유 |
|---|---|---|
| `compose/compose_test.go:59,108,264` | `Contains(Render(), 'provider_rate_limit_total{provider="…"} 0')` | rate-limit 패밀리 무변경 |
| `compose/compose_proxmox_test.go:53` | 동일 | 동일 |
| `contracttest/harness.go:183,193` | `countLineValue(Render(), "provider_rate_limit_total", provider)` | 동일 |
| `inventory/sync_test.go:537-541` | `MetricsText != ""` + rate-limit Contains | 렌더는 더 길어질 뿐, 두 단얫 다 성립 |
| `adapter/{aliyun,kubernetes,proxmox,tencent}/latency_test.go` | `provider_api_latency_seconds` 버킷 라인 | latency 패밀리 무변경 |
| `adapter/tencent/adapter_test.go:563-566` | `provider_api_errors_total{provider="tencent",op="discover",code="InternalError"}` Contains | **실측 확인** — health/sync 패밀리 미참조 |
| `tasks/engine_metrics_test.go` (313줄) | task·worker 패밀리(F/G/H/I/J) + `worker_queue_depth` | 본 과제가 손대지 않는 패밀리 |
| `secrets/broker_metrics_test.go:40,48,58` | `secret_access_total` | 동일 |
| `backend/main_metrics_test.go` (54줄) | 핸들러 200/404·Content-Type·body verbatim | 렌더 내용 무단얫 |
| `backend/main_sync.go:35,106` | `MetricsText` 필드 통과 | 골든 없음 |
| `router/routes_inventory_test.go` 라우트 골든 2종 | `route-inventory.txt`·`sensitive-routes.txt` byte 비교 | 신규 라우트 0 (§3.4 실측) |
| `metrics_test.go:36-56` `TestRenderDeterministicAndSorted` | rate-limit 3줄 정확 일치 | 본 과제는 rate-limit 무접촉 |

**r2 정정**: r1은 이 목록에 `healthloop_test.go:191`을 넣었다. **오분류다** — 컴파일·실행은 통과하지만 검출력을 잃는다. §4.2(2)로 이동했고, 갱신 대상이다. 이 사례가 §0.1이 요구하는 규율의 근거다: **"통과함"과 "검증함"은 다르다.** 위 표의 각 행은 그 단얫이 **본 과제와 무관한 패밀리를 본다**는 이유로 통과하는 것이지, 본 과제의 변경을 검증하지 않는다 — 그 검증은 §8이 새로 만드는 테스트가 한다.

---

---

## 14. r1 갭 상환 대조표

| 갭 | 등급 | 해소 위치 | 방식 |
|---|---|---|---|
| H-1 ⑧ 상태 매핑 오류 | HIGH | §2.1.3 표, §3.2 | ⑧ 값역 3종으로 정정 + `failed` 6갈래 명기 + 스펙 문장 재작성 |
| H-2 불변식 8경로 중 3개만 검증 | HIGH | §8 T-3 표, claim 6 | 경로별 델타 단얫 10케이스 + 총합 단얫. claim은 `grep -c` 폐기, 코드 검수 + 테스트 2단 |
| H-3 last-success 소거 부재 | HIGH | §2.3.4, §6 P3a, T-8, claim 3 | `RemoveSyncLastSuccess` + `pruneVanished` 1줄 |
| H-4 복원 문 위치 미지정 → 역행 | HIGH | §2.3.1 단조, §6 P4, T-10, claim 7·9 | setter 단조 규정(주 방어) + 호출 위치 `BuildEngine` 앞 고정(부 방어) |
| H-5 P3b "1줄" 거짓 | HIGH | §6 P3b, §2.3.5 | 술어 `status == RunStatusSucceeded` 확정, 최소 3줄로 정정 |
| H-6 claim 14 자기모순 | HIGH | §3.3, claim 14 | `metrics.go:54`의 숫자 분해 삭제 → "16종"만 |
| H-7 `:191` 접두 논거 거짓 | HIGH | §2.2.1, §4.2(2), §4.4 말미, 보존 제약 19 | 논거 재작성(스펙 라벨 열 일치), `:191`을 갱신 대상으로 재분류, 긍정/부정 쌍 규율 |
| H-8 zero → `18446744011573954816` | HIGH | §0.1, §2.3.1, R8, T-9, claim 7·10 | setter zero 가드 |
| H-9 partial 의미 괴리 | HIGH | §2.3.5, T-4, claim 8 | succeeded 전용 확정 + partial 케이스 테스트 |
| H-10 forward-stop 비용 누락 | HIGH | §9.2 표, §6 P4, claim 20 | P3b·P4 분리 착지 금지 + 멈춤 지점별 표 |
| H-11 파급 열거 불완전 | HIGH | §3.4 표 | 5문서 전수 + RESUME.md를 수정 대상에 편입 |
| H-12 착지 순서 무소유 | HIGH | §0.2-5, §9.1 R1, claim 19, §13-5 | P1 < recipe P2 제약 명문화 + recipe 전달 요청 |
| M-1 로컬 게이트 부재 | MED | §8 게이트 3종, R11, 보존 제약 16, claim 17 | `check-arch-boundary.sh`·`secret-scan.py` 추가 |
| M-2 파일 수 3중 불일치 | MED | §7 | 13개(수정 11 + 신규 2)로 통일, 번호 부여 |
| M-3 `Name` 미검토 | MED | §13-9 | 미채택 + 3가지 사유 명기 |
| M-4 Build 호출부 오인용 | MED | §2.3.2 | 14건 전수 재열거 |
| M-5 마이그레이션 선행 근거 거짓 | MED | §2.3.2 | 근거를 "의미론적 층위"로 교체(결론 유지) |
| M-6 `infra.go:54` 누락 | MED | §2.3.2, 보존 제약 15 | 프로덕션 3곳으로 정정 |
| M-7 정렬 계약·골든 무장해제 | MED | §4.1(2), R10, claim 4 | 3단 `sort.Slice` 명시 + 골든 다중 시리즈화 |
| M-8 스펙-코드 게이트 0건 | MED | §2.4 말행, §3.4 말미, §13-6 | 인정된 구멍으로 명기 |
| M-9 노출은 필드가 아니라 결합 | MED | §13-3 | "같은 등급" 판정 철회 + 증분 명시 + 3선택지 |
| M-10 partial_total 폐기 비용 상승 | MED | §13-10 | 유지 판정 + 비용 상승 기록 |
| M-11 recipe 골든 범위 리터럴 | MED | §3.4 표, §13-5 | recipe 경고로 전달 |
| L-1 `metrics_test.go:73`→`:72` | LOW | §3.3 | 정정 |
| L-2 assessment 범위 `:60`부터 | LOW | §3.4 | 정정 |
| L-3 restore_test 하네스 미지목 | LOW | §6 P4, claim 11 | `sync_test.go` 하네스 재사용 명시 |
| L-4 `mode` 자유 문자열 | LOW | §5 (나), A3, §13-7 | 카디널리티 절에 편입 |
| L-5 `SetHealthUnhealthy` no-op 미검증 | LOW | §8 T-6 | 갈래 추가 |
| L-6 ⑥⑦ 메트릭/원장 비대칭 | LOW | §2.1.3 말미, A9, §13-8 | 의도된 비대칭으로 명기 |
| L-7 부팅 hang 배제 근거 | LOW | §2.3.2 | 그대로 유지(정확) |
| L-8 `epic.md` 기존 불일치 | LOW | §3.4 표 | 조치 불요로 명기 |
