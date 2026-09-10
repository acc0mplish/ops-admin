# Phase 6 — 선행 과제 P 인계 명세 (V2 k8s 패리티 폐쇄) · r2

작성: 2026-09-09 (r1) · **개정 r2 (4렌즈 검토 상환: H-1·H-2·H-3·V-4·V-5·V-16·V-17·S-2·M-8)** ·
측정 시점 HEAD `3a3a106` · 측정일 2026-09-09

**성격: 인계 문서.** 본 문서의 작업은 `phase6-plan.md`의 범위가 **아니다**(보존 제약 #6이
`internal/infra/` 변경을 금지). 후속 계획(P)이 **이 문서만 보고 착수하고, §3의 게이트로 끝났음을
기계 판정**할 수 있어야 한다.

- 모계획: `v2-phase1/phase6-plan.md` §8 (요약) · 증거층: `v2-phase1/phase6-plan-evidence.md`
- 처분 원천: `backend/internal/infra/adapter/kubernetes/mapping.md` §3.2·§4 (§15.2 권위 표)

## r1 대비 변경 (검토 상환)

| # | r1 | r2 | 근거 |
|---|---|---|---|
| 모집단 | 엔드포인트 21 / 필드 39 / 수집기 9 — **3문서 3단위** | **`K8sClusterDetail` 1 DTO로 통일** | V-16·H-1 |
| 필드 | 39 | **43** | H-2 (누락 3 · 계수 오류 · 소분류 정정) |
| 도메인 판정 | 2건 | **3건** | H-3 |
| 수집기 | 9종 | **5종** | S-2 (Istio 4종은 v1 참조 0건) |
| 완료 판정 | **0건** | **§3 신설** | V-5·V-4 |
| P2 | 목록만 | + 권한·risk·승인 칸 | M-8 |
| Z 승계 | — | **§5 신설** | 사용자 확정 |

---

## 0. 모집단 확정 (V-16·H-1 상환)

r1은 세 곳에서 서로 다른 것을 셌다. r2는 **하나로 통일한다**.

**대상 = `backend/model/k8s.go:491-501` `K8sClusterDetail` DTO 1개.**

근거: 모계획 §J3이 step 3 전환 대상을 `GET /k8s/cluster/list`·`/cluster/info`·`/cluster/detail`
**3 엔드포인트**로 확정했고, 이 3개가 전부 `K8sClusterDetail`(또는 그 부분집합
`K8sClusterView`)을 직렬화한다. 그리고 `mapping.md`가 덮는 것도 정확히 이 DTO 하나다.

**대상 아닌 것 (명시)**: `pod/logs`·`pod/events`·`pod/containers`·`pod/metrics`·
`workload/metrics`·`namespace/events`·`pod/terminal/ws`·라이브 YAML을 반환하는 `*/detail` 계열
**18 + ws 1**. 인벤토리 투영이 서빙할 수 있는 데이터가 아니다(주기 스냅샷 vs 라이브 조회) →
**v1 무기한 존치**. 모계획 §7 D-12에 이탈로 기록됐다. **P는 이것들을 건드리지 않는다.**

---

## 1. P1 — 읽기 패리티 (수집·정규화)

표기: **집계** = V2 기수집 값에서 계산 · **수집 신규** = 어댑터가 아직 안 가져옴 ·
**조립 신규** = 원천은 있으나 투영이 섹션 형태로 조립 안 함 · **판정 필요** = 도메인 귀속 미정

### 1.1 `overview` 섹션 — `K8sOverview` 8필드

| legacy 필드 | v1 산출 지점 | V2에 필요한 것 | 처분 |
|---|---|---|---|
| `healthScore` | `getK8sClusterDetailUncached`(k8s.go:823) 경유 | node `healthState` + pod `phase` 집계 규칙 1개 | **집계** (v1 산식 그대로 옮길지 P가 판정) |
| `cpuUsage` | `calculateK8sAggregateMetrics`(k8s.go:4140) | node `capacityCoresGB` 합 + pod requests 합 | **집계** |
| `memoryUsage` | 동일 | node `capacityMemoryGB` 합 | **집계** |
| `podUsage` | 동일 | `allocatable.pods` + pod 행 수 | **집계** + `allocatable.pods` **1필드 수집 확장** (mapping.md §4.3은 allocatableCoresGB·MemoryGB만) |
| `requestRate` | Prometheus (`resolveK8sMonitorDatasource` k8s.go:1253) | — | **판정 필요 ①** |
| `alertCount` | 알림 도메인 | — | **판정 필요 ②** |
| `distribution[]` (`K8sKVTextItem`{label,value,sensitive}) | `buildOverviewDistribution`(k8s.go:2895) + `resolveK8sNetworkCIDRs`(:2910) | node 라벨/어노테이션 + kube-system configmap | **조립 신규** (원천 2종은 기수집) |
| `certificates[]` (`K8sCertificate` 9필드) | `buildOverviewCertificates`(k8s.go:2963) → `parseOverviewCertificate`(:2974)·`certificateCommonName`(:3011)·`k8sCertificateStatus`(:3018) | 인증서 파싱 산물 | **수집 신규 + 보안 판정 필요** (아래 §1.6) |

### 1.2 `advancedNetwork` — 수집기 **2종** (S-2 상환: r1은 6종)

legacy: `gatewayApiGateways[]`·`httpRoutes[]` (`model/k8s.go:480-483` — 2리스트뿐).
v1 산출: `buildAdvancedNetworkSection`(k8s.go:3651) + `k8sGetGatewayAPIJSON`(:4423) +
경로 빌더(:4453·:4457).

| 필요한 V2 수집기 | legacy 타입 | 헬퍼 |
|---|---|---|
| GatewayAPI Gateway | `kubeGatewayAPI`(k8s.go:556) | `collectGatewayAPIHosts`(:3818)·`Ports`(:3826)·`Addresses`(:3834)·`resolveGatewayAPIAddress`(:3842) |
| HTTPRoute | `kubeHTTPRoute`(:577) | `collectHTTPRouteParents`(:3876)·`Targets`(:3888)·`buildHTTPRouteTrafficItems`(:3927) |

**Istio 4종은 대상이 아니다 (S-2 실측)**: `service/k8s.go:630-633`의 `IstioGateways`·
`IstioVirtualServices`·`IstioDestinationRules`·`IstioServiceEntries`는 **백엔드 전체 참조가
선언 4행뿐**이고 `advancedNetwork`에 실리지 않으며 프론트도 `'gatewayapi'`·`'httproute'`로만
호출한다. i18n 키 4개는 미사용. → **신규 kind 어휘 6종 → 2종.**

다중 API 버전 폴백(`buildGatewayAPIResourcePathsWithPreferred` :4457)은 함께 옮긴다.

### 1.3 종 수집 확장 3종

| 종 | legacy 타입 | 상세 빌더 | mapping.md §3.2 |
|---|---|---|---|
| ReplicaSet | `kubeReplicaSet`(k8s.go:220) | — | `dropped(v2-not-collected)` |
| Job | `kubeJob`(:414) | `buildJobDetail`(:2827) | 동일 |
| CronJob | `kubeCronJob`(:435) | `buildCronJobDetail`(:2855) | 동일 |

2026-09-08 비교 아티팩트에 흔적: `data/compare/1/2026-09-08/104530.json`의 `report.absents[]`에
`{"section":"job","kind":"orchestration.workload","field":"count","legacy":"4"}`.

**부수 효과**: 이 3종 수집이 모계획 §J6의 `seed-cron` 문제(C-4)를 근본 해소한다 — Job/CronJob이
수집되면 identity-sets-differ가 사라진다.

### 1.4 개별 필드 **35건** (H-2 상환 — r1은 31, 누락 3 + 계수 정정)

**node — `K8sNodeItem`(model/k8s.go:164)** — 4건

| 필드 | V2에 필요한 것 | 처분 |
|---|---|---|
| `version` | `status.nodeInfo.kubeletVersion` | 수집 신규 |
| `internalIP` | `status.addresses[type=InternalIP]` | 수집 신규 |
| `os` | `status.nodeInfo.osImage` | 수집 신규 |
| `pods` | node별 pod 수 / `allocatable.pods` | 집계 (§1.1 `podUsage`와 분모 공유) |

**namespace — `K8sNamespaceItem`(:182)** — 3건: `pods`·`services`·`workloads` 전부 **집계**
(종별 리소스 행을 namespace로 group by. v1은 `buildNamespaceCounts` k8s.go:3065).

**pod — `K8sPodItem`(:191)** — 5건

| 필드 | V2에 필요한 것 | 처분 |
|---|---|---|
| `workloadName`·`workloadType` | `ownerReferences` → ReplicaSet 경유 Deployment 역추적 | 집계 2건 (v1 `buildPodItemsWithWorkloads`:3142·`podWorkloadRef`:3133). **§1.3 ReplicaSet 수집에 의존** · **모계획 I8-a(`phase4-plan.md:167,586` normalizer 관계 링크)에도 의존** |
| `nodeIP` | `status.hostIP` | 수집 신규 |
| `ip` | `status.podIP` | 수집 신규 |
| `age` | creationTimestamp (Raw 기보유) | 조립 신규 (VOLATILE) |

**workload — `K8sWorkloadItem`(:281)** — 5건: `updated`(`status.updatedReplicas` 수집) ·
`available`(`status.availableReplicas` 수집) · `requests`·`limits`(컨테이너 자원 요약 수집 —
v1 `formatWorkloadResourceSummary`:3578·`formatCPUMilli`:3608·`formatMemoryBytes`:3615) ·
`age`(조립, VOLATILE).

**service — `K8sServiceItem`** — 3건: `externalIP`(수집 — v1 `serviceExternalIP`:3410) ·
`endpoints`(수집 — `kubeEndpoints`(:265) 객체 필요, v1 `buildEndpointCounts`:3377) · `age`(조립).

**ingress — `K8sIngressItem`** — 3건: `address`(수집) · `tls`(수집) · `age`(조립).

**storage — `K8sStorageItem`** — 6건: `namespaceScope`(수집 — 어노테이션
`ops-admin.io/namespace-scope` k8s.go:4046, `storageNamespaceScope`:4051) · `status`(수집) ·
`sourceType`·`path`·`nfsServer`(수집 — `persistentVolumeSource`:4036) · `reclaimPolicy`(수집).

**configStorage age 2건 〔r2 신설 — H-2 누락〕**: `configMaps[].age` · `secrets[].age` — 조립
(VOLATILE).

**cluster — `K8sClusterView`(:114)** — 4건 〔r2 정정 — r1은 3필드를 번호 2개로 셌다〕

| 필드 | 처분 |
|---|---|
| `nodeCount` 〔r2 신설 — H-2 누락〕 | **집계** (node 리소스 행 수) |
| `statusText` | 조립 (연결 상태 문자열) |
| `gatewayName` | 조립 (`asset_gateway` 조인) |
| `monitorDatasourceId`·`monitorDatasourceName` | **판정 필요 ③** 〔r2 신설 — H-3〕 |

나머지 `K8sClusterView` 필드는 `provider_connection`/`ConfigJSON` 속성이라 투영 조립에서 붙는다
(mapping.md §4.1 `dropped(v2-source-key)`).

### 1.5 합계 (H-2 상환 — 소분류 정정)

| 분류 | 건수 |
|---|---|
| overview 8 (집계 4 · 조립 1 · 수집+보안판정 1 · **판정 필요 2**) | 8 |
| node 4 · namespace 3 · pod 5 · workload 5 · service 3 · ingress 3 · storage 6 · configStorage age 2 · cluster 4 | 35 |
| **필드 합계** | **43** |
| 소분류 | **수집 신규 19 · 집계 6 · 조립 6** (+ 판정 필요 3 + 보안 판정 1 = 위 43에 포함) |
| 수집기 확장 | **5종** — GatewayAPI·HTTPRoute 2 + ReplicaSet·Job·CronJob 3 |
| 신규 kind 어휘 | **2종** (GatewayAPI·HTTPRoute) |

부수 요구: `kubeEndpoints` 수집 · `allocatable.pods` 1필드 · GatewayAPI 다중 버전 폴백 이관.

### 1.6 P가 **먼저** 판정해야 할 4건

| # | 항목 | 쟁점 |
|---|---|---|
| ① | `overview.requestRate` | mapping.md `dropped(monitoring-derived)`. §3.2 row 19(Monitor=REMAIN) 소유일 수 있다 |
| ② | `overview.alertCount` | 동일 |
| ③ 〔r2 신설〕 | `cluster.monitorDatasourceId`·`Name` | mapping.md §4.1 `dropped(monitoring-domain)`. row 19와의 정합 |
| ④ | `overview.certificates[]` **보안 경계** | **결착: A′-1 채택(사용자 승인 2026-09-10 — P1-G1·G2 착지).** v1은 **kubeconfig의 인증서 성분을 파싱**한다(k8s.go:2963). mapping.md §6이 "kubeconfig는 Material로만 진입 — 로그·에러·Raw·Normalized 어디에도 노출 금지"라고 못 박았다. **보안 계약 위반 가능성**. → 판정: 봉인 대상은 재사용 가능한 **물질**(원본 인증서·개인키·kubeconfig 문자열)이고 만료일·CN 등 파생 메타데이터는 관측이다. 어댑터는 `HealthResult.Observation`(contract additive 필드)으로 **9필드 파생 메타데이터만** 반환(v1 `parseOverviewCertificate` 동치 — arch rule 2 준수, 어댑터 DB 무소속)·sweep이 `provider_connection.ConfigJSON["health_observation"]`에 기록(markLastHealthAt 패턴)·조립이 소비. `client-key-data`는 v1과 동일하게 미취급. 착지 검증: PC-6(`TestHealthSweepRecordsObservation` exact)·PC-4(원장 28행·92+28=120·char baseline 동일)·G-P1f 조건 소멸(mapping.md pending(④) 0행) |

> **선행조건 (팀리드 지시 2026-09-09)**: **④ 판정이 끝나기 전에는 P1에서 `certificates[]`를
> 착수하지 않는다.** 다른 42개 필드는 ④와 무관하게 진행할 수 있다. 게이트 `G-P1c`의 dropped
> 잔여 0 판정에서 `certificates[]`는 ④ 결착 시까지 "판정 대기"로 분리 계상한다.

①②③은 공통 선택지가 있다: **(가) V2 인벤토리가 흡수** vs **(나) 투영 조립 단계에서 v1
모니터링 도메인을 조회해 합침**. (나)면 §3.2 row 19 REMAIN 처분과 충돌하지 않는다.

---

## 2. P2 — 쓰기 패리티 (오퍼레이션 10종) + 권한·risk·승인 (M-8 상환)

선례: restart는 `RequiresApproval: true`(`compose.go:287`)·권한 `assets:k8s:workload:restart`·
risk=medium(`sensitive-routes.txt:184`). **`resource.apply`/`delete`는 임의 kind YAML을 적용·
삭제하므로 restart 선례로 환원할 수 없다** — 아래 제안값은 P가 확정해야 한다.

| V2 operation | v1 원천 (`service/k8s.go`) | v1 권한 | 제안 risk | 승인 |
|---|---|---|---|---|
| `k8s.workload.scale` | `ScaleK8sWorkload`:1658 | `assets:k8s:workload:scale` | medium | 필요 |
| `k8s.workload.image_update` | `UpdateK8sWorkloadImages`:1746 (+`replaceImageVersion`:3622·`buildWorkloadImagePatchBody`:3558) | `assets:k8s:workload:image` | medium | 필요 |
| `k8s.workload.resources_update` | `UpdateK8sWorkloadResources`:1829 (+`buildWorkloadContainerPatchBody`:3570) | `assets:k8s:workload:yaml` | medium | 필요 |
| `k8s.node.labels_update` | `UpdateK8sNodeLabels`:947 | `assets:k8s:workload:yaml` | medium | 필요 |
| `k8s.service.update` | `UpdateK8sService`:1970 (+`serviceTargetPort`:2078) | `assets:k8s:workload:yaml` | medium | 필요 |
| **`k8s.resource.apply`** | `CreateK8sResourceYAML`:1430 · `UpdateK8sResourceYAML`:1364 (+경로 빌더 :4571·:4640·:4728) | `assets:k8s:workload:yaml` | **high — 임의 kind 생성/변경** | **필요 · 신규 권한 검토** |
| **`k8s.resource.delete`** | `DeleteK8sResource`:1471 (+`buildK8sDeleteResourcePaths`:4764) | `assets:k8s:resource:delete` | **high — 임의 kind 삭제, 비가역** | **필요 · 신규 권한 검토** |
| `k8s.istio.traffic_update` | `UpdateK8sIstioTraffic`:1500 | `assets:k8s:advancednetwork` | medium | 필요 |
| `k8s.httproute.traffic_update` | `UpdateK8sHTTPRouteTraffic`:1551 | `assets:k8s:advancednetwork` | medium | 필요 |

**cluster add/update/delete 3건은 오퍼레이션이 아니라 등록 경로다** → §4.

**V-17 상환 — 커버리지 판정 수단**: §15 비교 프로토콜은 **읽기 전용**이라 오퍼레이션 정의를
검증하지 않는다. P2는 오퍼레이션↔실행기 대응을 `contracttest` 하네스
(`internal/infra/contracttest/harness.go`)로 잠그고, §3 게이트 G-P2b가 그것을 요구한다.

---

## 3. 완료 판정 게이트 (V-5·V-4 상환 — r1에 0건이었다)

**전부 기계 판정 가능해야 한다.** 모계획 §4.2가 이 게이트를 블록 2 진입 조건으로 인용한다.

```
G-P1a  대상 DTO 전 필드 분류: cd backend && go test ./internal/infra/adapter/kubernetes/ \
         -run TestMappingCoverage -count=1 → ok
G-P1b  **텍스트 편집 방어 (V-4 상환)** — 현행 mapping_test.go:153-159 는
         strings.Contains(doc, field) + strings.Contains(doc, disposition) 로
         **문서 전체 부분문자열**만 본다. Go coverageTable 값과 .md 텍스트를 함께
         고치면 수집기 0개로도 녹색이 된다.
       → P는 이 테스트를 강화해야 한다: disposition == "mapped" 인 필드마다
         **실제 normalized 키가 정규화 산출물에 존재함**을 단얫할 것.
       판정: cd backend && go test ./internal/infra/adapter/kubernetes/ \
         -run TestMappingCoverage -count=1 && \
         grep -c 'normalizedKeyFor\|assertNormalizedKey' \
           internal/infra/adapter/kubernetes/mapping_test.go → 1 이상
G-P1c  dropped 잔여 0: cd backend && grep -c 'dropped(v2-schema-absent)\|dropped(v2-not-collected)' \
         internal/infra/adapter/kubernetes/mapping.md → 0
         (또는 잔여 항목이 본 문서 §1 표에서 "이관 안 함" 결정으로 명시 전환됐음을 PR이 보임)
G-P1d  수집기 5종 등재: cd backend && for k in gatewayapi httproute replicaset job cronjob; do \
         grep -qi "$k" internal/infra/adapter/kubernetes/adapter.go || echo "MISSING $k"; done → 출력 0행
G-P1e  §15 비교 유지: 모계획 C21 (seed-cron 선행조건 포함) → verdict pass
G-P1f  판정 필요 4건 결착: 본 문서 §1.6 표의 4행이 전부 "결정: (가)/(나)/이관 안 함"으로
         갱신되고 그 근거가 커밋 메시지에 있음. **④(certificates) 미결착 상태에서
         `certificates[]` 관련 커밋이 있으면 게이트 실패** — 선행조건 위반
G-P2a  오퍼레이션 10종 등록: cd backend && go run . --list-operations 2>/dev/null | wc -l
         (또는) grep -c 'RegisterOperation' internal/infra/compose/compose.go → 11 이상 (restart 포함)
G-P2b  실행기 대응: cd backend && go test ./internal/infra/contracttest/ -count=1 → ok,
         10종 전부 plan→execute 왕복
G-P2c  권한·risk·승인 확정: docs/security/sensitive-routes.txt 에 10종 def가 등재되고
         §2 표의 risk/승인 칸이 실제 등록값과 일치
```

---

## 4. `register-k8s` CLI 규약 (블록 2 이월 판정 "껍데기 불완전" 상환)

모계획 Phase H의 유일한 cluster CRUD 대체 경로다. 선례 `backend/main_register_pve.go`
(:244-281 커넥션/컨텍스트 upsert · :357-394 SecretRef 봉인·바인딩).

| 항목 | 규약 |
|---|---|
| 입력 | `--name`(upsert 멱등 키·필수) · `--kubeconfig <path>`(필수) · `--env` · `--connection-mode direct\|gateway` · `--gateway-id` · `--monitor-datasource-id` · `--insecure-tls` |
| kubeconfig 봉인 | 파일을 읽어 **즉시** `util.EncryptSecretV2` → `secret_ref` 행. **평문은 어디에도 남기지 않는다**(백필의 P-class 예외 `backfill.go:129`는 승계하지 않는다 — 신규 경로는 처음부터 봉인) |
| `provider_context` | `SourceKeyUIDSalted(name, "context")` 형태로 1:1 생성. **`k8s_cluster` 파생 UID(`SourceKeyUID("k8s_cluster", id)`)를 쓰지 않는다** — 모계획 S1b가 그 결합을 끊으라고 요구한다 |
| `provider_credential_binding` | purpose `inventory` 1건 (restart 등 오퍼레이션이 같은 자격을 쓰면 재사용) |
| 멱등 | `--name` 동일 시 upsert. 재실행이 SecretRef를 재봉인하되 UID는 유지 |
| 검증 | 등록 직후 `Validate` 내장 호출 → `/version` 200 확인 실패 시 롤백(트랜잭션) |
| 금지 | 시크릿 값을 인자·로그·에러에 노출하지 않는다. `--kubeconfig`는 **경로**만 받는다 |

---

## 5. Phase Z 승계 규약 (사용자 확정 — 모계획 §J0 목적 ②)

모계획 Phase Z가 `service/k8s.go` 순수 함수 120개에 characterization test를 만든다. **그 테스트는
P의 동치 판정 오라클로 승계된다** — 버려지지 않는다.

**승계 대상 (본 문서 §1이 이관 원천으로 지목한 함수)**: `buildOverviewDistribution`(2895) ·
`resolveK8sNetworkCIDRs`(2910) · `buildOverviewCertificates`(2963) · `parseOverviewCertificate`(2974) ·
`k8sCertificateStatus`(3018) · `calculateK8sAggregateMetrics`(4140) · `buildNamespaceCounts`(3065) ·
`buildPodItemsWithWorkloads`(3142) · `formatWorkloadResourceSummary`(3578) · `formatCPUMilli`(3608) ·
`formatMemoryBytes`(3615) · `serviceExternalIP`(3410) · `buildEndpointCounts`(3377) ·
`persistentVolumeSource`(4036) · `storageNamespaceScope`(4051) · `collectGatewayAPIHosts`(3818) ·
`collectHTTPRouteParents`(3876) · `buildHTTPRouteTrafficItems`(3927) 외.

**승계 방법**: Z 테스트는 **테이블 주도**로 쓰고 legacy 함수 호출을 **1행으로 격리**한다
(모계획 R18 완화). P는 그 1행을 V2 정규화기 호출로 교체해 **같은 입력 → 같은 출력**을 요구한다.
이것이 "V2 재구현이 legacy와 동치인가"의 유일한 기계 판정 수단이다 — §15 비교는 **인벤토리
수준**의 동치만 보고 필드 산식은 보지 않는다.

**게이트**: `G-P1g` — 승계 대상 함수마다 V2 대응 구현이 Z 테스트의 동일 입력 테이블로 통과.
