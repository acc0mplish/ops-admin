# Phase 6 — 전제 변경·결합 census 실측 (r3)

작성: 2026-09-09 · 성격: **증거층 분책**. `phase6-plan-evidence.md`(E1~E10 — 트리 현황 실측)의
연속이며, 여기에는 r2·r3에서 새로 측정한 두 갈래만 담는다:

- **E12** — 운영 배포본 부재(전제 A)가 r1의 어떤 실측을 무의미하게 만들었나 + step 5의 새 선행조건
- **E13** — 계획 §2.0의 6축 결합 census를 r2 배치표 전체에 소급 적용한 결과 (CRITICAL 4건의 원자료)

모계획: `v2-phase1/phase6-plan.md` · 측정 시점 HEAD `3a3a106` · 측정일 2026-09-09

## E12. 운영 배포본 부재가 바꾼 것 (r2 전제 1 — 실측 재배치)

r1의 E6은 "관찰 기간을 무엇으로 수행하는가"를 답하려 측정했다. 전제 1로 그 질문이 사라졌으므로
**측정 자체는 유효하되 결론이 이동**한다. 원 측정은 E6에 그대로 두고 이동만 기록한다.

| E6 항목 | r1 결론 | r2 결론 |
|---|---|---|
| E6.1 GET이 `sys_operation_log`에 안 남음 (`middleware/operation_log.go:25`) | step 5 읽기 착수 불가의 **결정적 근거** | **무관** — 관찰할 트래픽이 없다. 미래 배포 시 복원되는 문제 (본문 §7 D-6 조건화 개정이 복원 지점) |
| E6.2 비-GET은 `sys_operation_log` 전건 기록·자동 retention 없음 | step 5 쓰기 착수의 **기계 판정 수단** | **불요** — 관찰 조건 자체가 소멸. 측정 사실(자동 정리 없음·관리자 수동 삭제 3종 `service.go:1204-1213`)은 미래 재사용 대비 보존 |

### E12.1 step 5의 새 선행조건 — 실측 (본문 J5 S1~S3의 원자료)

**S1 — V2 백필이 `k8s_cluster`를 직접 읽는다**

`internal/infra/inventory/backfill.go:69`:
```go
err := db.WithContext(ctx).Table("k8s_cluster").
```
그 밖에 `:74`(에러 문언)·`:158`·`:180`·`:196`·`:204`·`:211`·`:224`·`:226`·`:300` 이 `k8s_cluster`를
소스 키로 쓴다. 테이블 drop 시 `RunK8sBackfill` 전체가 무효.

**S2 — `compare-inventory`의 legacy 측**

`main_compare.go:37`(`--cluster` = "v1 k8s_cluster id")·`:76`(load)·`:83`
(`source_model = "k8s_cluster"` 조인). drop 시 legacy 캡처 불가.

**S3 — REMAIN 도메인의 논리 참조 (DB FK 제약은 없음)**

인바운드 FK 실측 (2026-09-09, `information_schema.key_column_usage`):
```
asset_database        gateway_id            -> asset_gateway
asset_gateway         credential_id         -> asset_credential
asset_host            cloud_account_id      -> asset_cloud_account
asset_host            credential_id         -> asset_credential
asset_host            gateway_id            -> asset_gateway
asset_host            group_id              -> asset_host_group
asset_host_group_rel  host_id               -> asset_host
asset_host_group_rel  group_id              -> asset_host_group
asset_service_workload service_id           -> asset_service
k8s_cluster           gateway_id            -> asset_gateway
k8s_cluster           monitor_datasource_id -> monitor_datasource
```
→ **`referenced_table_name = 'k8s_cluster'` 인 행은 0건.** `k8s_cluster`는 나가는 FK 2건만 갖는다.

논리 참조 2곳 (gorm `index`, FK 제약 아님):
- `model/asset.go:12` `AssetService.K8sClusterID uint` (§3.2 row 9 — REMAIN)
- `model/ops.go:383` `OpsApplicationEnvironmentBinding.K8sClusterID uint` (§3.2 row 22 — REMAIN,
  "FK converts to a typed binding at Milestone 2 cutover")

행수 실측 (2026-09-09): `k8s_cluster` 2 · `asset_service` **0** · `ops_application_env_binding` **0**.
→ 참조 무결성 파손 데이터는 현재 없다. 남는 것은 **코드 경로 정리**(`service/asset_service.go:18,71`·
`service/asset_service_diagnosis.go:50` 등).

### E12.2 `k8s_cluster` 컬럼 전수 (drop 대상 — 실측)

```
id · name · status · api_server · version · node_count · env · tags · connection_mode
gateway_id · monitor_datasource_id · description · kube_config · last_sync_at
created_at · updated_at
```
(16컬럼. `kube_config`는 P-class 평문 — mapping.md/백필 주석 `backfill.go:129`이 "plaintext는
v1에 KEPT"라고 명시. drop이 곧 그 평문의 소멸이므로 **파괴 전 백업**(C34)이 특히 중요하다.)

### E12.3 §16.1 구현 격차 (본문 J4 4b의 근거)

스펙 §16.1이 정의한 provider 라우트 11개 중 **구현된 것은 GET 4개뿐**:

| 스펙 §16.1 | 구현 |
|---|---|
| `GET /provider-types` | ✅ `internal/api/v2/infra.go:83` |
| `GET /provider-connections` | ✅ `:84` |
| **`POST /provider-connections`** | ✗ **미구현** |
| `GET /provider-connections/{uid}` | ✗ |
| **`POST /provider-connections/{uid}/validate`** | ✗ |
| **`POST /provider-connections/{uid}/sync`** | ✗ |
| `GET /provider-contexts` | ✗ |
| `GET /resources` | ✅ `:85` |
| `GET /resources/{uid}` | ✅ `:86` |
| `GET /resources/{uid}/relationships` | ✗ |
| `GET /resources/{uid}/operations` | ✅ `RegisterOperations` 경유 |

→ cluster add/update/delete를 V2 라우트로 대체할 수 없다. 대안은 `register-k8s` CLI
(선례 `main_register_pve.go` — backend 루트에서 `internal/infra`를 읽어 쓰기만 하고 패키지를
변경하지 않으므로 보존 제약 #6 유지).

---

## E13. 6축 결합 census — 소급 적용 실측 (r3 신설 · 계획 §2.0 원자료)

CRITICAL 4건(C-1~C-4)이 전부 "라우트와 화면만 셌다"는 같은 실패 모드였다. 아래는 계획 §2.0이
의무화한 6축을 r2 배치표 전체에 소급 적용한 결과다. 측정일 2026-09-09.

### E13.1 ② 서비스 내부 호출자 — k8s 쓰기 메서드 14종 전수

```
grep -rn "s\.<M>(\|svc\.<M>(" backend/ --include=*.go | grep -v _test
```

| 메서드 | 내부 호출자 |
|---|---|
| **`RestartK8sWorkload`** | **1 — `service/integration_ai.go:1732`** |
| **`ScaleK8sWorkload`** | **1 — `service/integration_ai.go:1734`** |
| `UpdateK8sWorkloadImages` · `UpdateK8sWorkloadResources` · `UpdateK8sNodeLabels` · `UpdateK8sService` · `CreateK8sResourceYAML` · `UpdateK8sResourceYAML` · `DeleteK8sResource` · `UpdateK8sIstioTraffic` · `UpdateK8sHTTPRouteTraffic` · `CreateK8sCluster` · `UpdateK8sCluster` · `DeleteK8sCluster` | **각 0** |

→ **14종 중 2종만 내부 결합이 있고, 둘 다 같은 파일이다.** `integration_ai.go`는 §3.2 row 16
**OUT-OF-SCOPE** 도메인이므로 계획 §J4 (가)가 "서비스 메서드는 남긴다"로 해소한다.

### E13.2 ⑤ 하드코딩 라우트 수치 — 전수 3곳

| 위치 | 코드 | Phase E 후 값 |
|---|---|---|
| `backend/router/routes_inventory_test.go:130` | `if len(lines) != 450` | **449** |
| `backend/router/authz_replay_test.go:234` | `if len(authRoutes) != 440 { t.Fatalf("… contract is 440 (427 v1 + 13 v2)") }` | **439** · 메시지 `(426 v1 + 13 v2)` |
| `backend/router/authz_replay_test.go:353` | `if count != 245 { t.Fatalf("authGroup non-GET count %d drifted from the 245 baseline") }` | **244** |

r2는 `authz_replay_test.go:47-49`를 "주석·용량 힌트"로 적었다 — **오분류**다. `:45`는 주석,
`:51`은 슬라이스 cap, **`:234`·`:353`이 `t.Fatalf`로 끝나는 하드 단얫**이다 (H-1·H-2·M-1).

**CI 측 하드코딩은 0건** (`grep .github/workflows/ scripts/` — T-1). `opdef/opdef_test.go:74`
`minTotalDefs = 255`는 **하한**이라 289→288에서도 안전.

### E13.3 ④ `k8s_cluster` 결합 — 비테스트 **31참조 · 7파일**

```
grep -rn "k8s_cluster" backend/ --include=*.go | grep -v _test | wc -l   → 31
```

| 파일 | 성격 |
|---|---|
| `internal/infra/inventory/backfill.go` | 소스 읽기 13지점 + **`markStaleSources`:297-303** (C-3의 진원) |
| `main_compare.go` | :37 플래그 설명 · :76 load · :83 `source_model='k8s_cluster'` 조인 |
| `main_sync.go` | :43 주석 |
| `model/k8s.go` | :27 `TableName()` |
| `service/asset_service.go` | :96 `k8s_cluster_id` 컬럼 갱신 |
| **`util/secret_registry.go`** | **:65 — §4.1 secret 인벤토리 14행 중 8행** (`K8sCluster.kube_config`, `ClassPlaintext`). **r2가 놓친 결합** → 계획 S4 |
| `service/integration_ai.go` | :99·:705·:706·:965 — **거짓 양성**. `k8s_cluster_overview`는 AI 툴 키 문자열이지 테이블 참조가 아니다 |

**인바운드 FK 실측** (`information_schema.key_column_usage`, 2026-09-09):
`referenced_table_name = 'k8s_cluster'` → **0행**. `k8s_cluster`는 **나가는** FK 2건만 갖는다
(`gateway_id→asset_gateway` · `monitor_datasource_id→monitor_datasource`).
행수: `k8s_cluster` 2 · `asset_service` 0 · `ops_application_env_binding` 0.

### E13.4 ③ 프론트 k8s API 소비 — **9파일** (r2 배치표는 2)

`AssetDetail.vue` · `AppPipelineCenter.vue` · `Application.vue` · `K8sClusterManage.vue` ·
`K8s.vue` · `ServiceHealthDiagnosis.vue` · `K8sPodTerminal.vue` · `ServicePodMonitor.vue` ·
`k8s/PodMonitor.vue`.

그중 **restart 소비는 2파일**(`web/src/api/k8s.js:73` · `web/src/views/assets/K8s.vue`) —
r2의 2파일은 restart 한정으로는 맞았으나 **블록 2(step 3 전환)의 소비 면적을 9파일로 세지
않았다**(H-6). `K8s.vue:693`은 `Promise.all(targets.map(restartK8sWorkload…))` 벌크 호출.

### E13.5 ⑥ CI — 워크플로 **2파일 12잡** (r2는 `v2-ci.yml` 10잡만)

| 파일 | 잡 |
|---|---|
| `.github/workflows/v2-ci.yml` | 10잡 (E8 표) |
| **`.github/workflows/korean-localization-guard.yml`** | **`no-hardcoded-chinese`(:13) · `i18n-parity`(:80)** — r2 claims에 대응 0 |

`i18n-parity`는 **Phase E2가 `K8s.vue`에 plan→execute 문구를 넣으면 직격**한다 (V-11).
계획 C42·C43이 상환한다.

### E13.6 함수 분류 — Phase Z 모집단 근거

```
grep -c '^func ' service/k8s.go              → 161
grep -c '^func (s \*Service)' service/k8s.go →  41
                                    순수 함수 → 120
```

E5 시맨별 순수 함수 분포 (심볼 시작행을 E5 경계에 대조):

| 시맨 | 순수 함수 |
|---|---|
| `k8s_types` · `k8s_types_mesh` · 잔류 `k8s` · `k8s_mutate` · `k8s_workload` | 0 |
| `k8s_metrics` | 4 |
| `k8s_detail` | 1 |
| `k8s_fetch` | 24 |
| `k8s_build_pod` | 21 |
| `k8s_build_net` | 26 |
| `k8s_transport` | 19 |
| `k8s_path` | 25 |
| **합계** | **120** |

### E13.7 V-4 — P1 게이트의 텍스트 편집 취약점 (실측)

`backend/internal/infra/adapter/kubernetes/mapping_test.go:145-159`:

```go
for _, field := range legacyFieldNames(t, section.item) {
    key := section.path + field
    disposition, ok := coverageTable[key]
    if !ok { t.Errorf("legacy field %s is not classified in the coverage table", key); continue }
    if !strings.Contains(doc, field) { t.Errorf("field %s (%s) is absent from mapping.md", field, key) }
    if !strings.Contains(doc, disposition) { t.Errorf("disposition %q of %s is absent from mapping.md", disposition, key) }
}
```

`doc`은 `mapping.md` **전문 문자열**이고 검사는 `strings.Contains` 2회 — **행 대응도, "mapped"가
실제 V2 정규화 키를 갖는지도 보지 않는다.** Go `coverageTable`(:53) 값과 .md 텍스트를 함께 고치면
**수집기 0개로 게이트가 녹색**이 된다. 인계 문서 §3 `G-P1b`가 이 강화를 요구한다.
