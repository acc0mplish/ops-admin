# 선행 과제 P 구현 계획 — V2 k8s 패리티 폐쇄 (P1 읽기 + P2 쓰기) · r3

작성: 2026-09-10 (r1) · 개정 r2 (3렌즈 반려 상환: CRITICAL 1 · HIGH 8 중 유효 6 · MEDIUM 15 · LOW 11) ·
**개정 r3 (2026-09-10 — r2 검증 조건부 승인 마무리: M-P1·M-P2·M-P3 + LOW 6건. 새 구조 없음 — 서술·게이트 보강)** · 티어 **XL** ·
측정 HEAD `620bee2` (블록 1 완결 후 — p6-state.json "done")
원천 스펙: `v2-phase1/phase6-parity-handover.md` **r2** (유일 스펙) · 모계획 `phase6-plan.md` §8·§J0·§J3·§J5·§J6
실측 원천: `contract/resource_kind.go`·`contract/adapter_test.go`·`adapter/kubernetes/{adapter,normalizer,client,executor}.go`·
`compose/{compose,healthloop}.go`·`inventory/{projection,compare}.go`·`contracttest/harness.go`·`api/v2/{infra,operations}.go`·
`main_compare.go`·`service/k8s*.go`(13파일)·`service/k8s_*_test.go`(마커 120)·`main_register_pve.go`·`sensitive-routes.txt`

## r1 대비 변경 (검토 상환)

| 결함 | r1 | r2 | 근거 |
|---|---|---|---|
| C1/H-1 | kind 어휘를 compose readKinds만 건드림 | **`Phase6ResourceKindExtensions` 슬라이스**(contract/resource_kind.go) 신설 — 4종, M1 19종 불변 단얫 무충돌 | H1·기술 C1 교차 |
| H-2/H-5 | ④ A안 "어댑터 Health()가 DB 기록" | **arch rule 2 위반이었다.** HealthResult.Observation 확장 + healthloop 기록 2변형으로 재서술, P1-G 2 Phase 분할 | H5·H-2 교차 |
| H-3 | Z 원장 31종·"집계 6" 애매 | **33종 재산출**(주소 헬퍼 2종 추가)·**43필드↔Phase 대응표 신설**·버킷 산술 재정의(집계 13) | H2·H3 |
| H-4 | gateway·httproute 비교 집합 합류 | **스코프 외 이월(I-P5)** — LegacyCapture에 섹션 부재(main_compare.go 실측)라 합류는 identity-sets-differ BLOCKER. 합류는 job·cronjob만(Workloads Type 실림 실측) | H4·M-6 |
| H-5 | workload template 컨테이너 양만 수집 | **pod 실행 컨테이너 requests/limits 원시량 수집 추가**(①② 분자)·milli/bytes 정수 저장·podCIDR 단일 필드 폴백 | H5 |
| H-6 | "Job 수집이 C-4 근본 해소" | **인과 정정** — 근본 해소는 fixture 정리(완료 파드 회전 제거·state.json 증거), 수집은 비교 합류의 필요조건. PC-5 연속 3회 pass 추가 | H7 |
| H-7 | 게이트 명령 4건 공허/불가능 | PC-4(원장 불식식)·PC-7(`TestExecute|TestPoll`)·PC-8(TestOperationDefTable 일원화)·PC-A(실존명) — 신규 테스트명 전부 Phase deliverable에 못 박음 | H8·셀프 검증성 |
| M군 9건 | — | §4·§7·§10에 반영 (M-1 라인 재인용·M-2 승계 경계·M-3 gw 키·M-4 G-P2c 미해결 이동·M-5 권한 주석·M-6 CRD 부재·M-7 ≥13·M-8 spike·M-9 계수) | M-1~9·L-1 |
| r3 | — | PC-6 exact 테스트명 매치(M-P3) · `main_compare.go` 불변+PC-V 추가(M-P1) · J-P1-9 계상 규칙(M-P2) · 인용 드리프트 4건(probe :174·markLastHealthAt :195·restart 승인 `compose.go:287`·adapter_test :45)·E1/E2 계열 마커 소속 실측 정정(metrics·detail 0)·PC-L 확대·원장 주석행 금지·§9-7 각주 | r2 검증 M-P1~3·LOW |

---

## 0. BLUF

**P1** = `K8sClusterDetail` **43필드** 중 `certificates[]` 1엔트리 분리 계상 → **42필드 즉시** + 수집기 6섹션(RS·Job·CronJob·Gateway·HTTPRoute·Endpoints) + kind 어휘 **4종**(`Phase6ResourceKindExtensions` — 제안명 `network.gateway`·`network.http_route`·`network.endpoint`·`network.virtual_service`) + 조립 라이브러리(`internal/infra/inventory/k8sassembly.go` 신규) + **Z 오라클 승계 스왑 29마커**(④ 대기 4 제외).
**P2** = 오퍼레이션 **신규 9종**(restart 포함 총 10종) — `internal/api/v2/` **무변경 설계**(payload는 실행기 자체 검증).
**Phase 15개**: PP-0 → P1-A·B·C1·C2·D·E1·E2·F(→ G1·G2는 ④ 결착 후) ∥ P2-A~E. 각 ≤5파일.

판정 제안 요약(§2·§3 — 결정권은 리뷰/사용자):

| # | 항목 | 권고 |
|---|---|---|
| ①② | `requestRate`·`alertCount`(및 `healthScore`·usage 3종) | **(가) V2 인벤토리 흡수 — 전부 집계.** 실측상 모니터링 도메인이 아니다(§1-1) |
| ③ | `monitorDatasourceId`·`Name` | **혼합** — Id는 `provider_connection.ConfigJSON` 흡수(등록 경로), Name은 조립 시 monitor_datasource 조인(§3.2 row 19 REMAIN 무충돌) |
| ④ | `certificates[]` 보안 경계 | **분석만 제공(§3)** — 권고 A′안 2변형(HealthResult.Observation + healthloop 기록). ④ 결착 전 착수 금지 유지 |
| ⑤〔신설〕 | `distribution[]`의 serviceCIDR 원천 | kubeadm-config **data 값**이 필요한데 보존 제약 #7이 configmap 값 수집을 금지 — 2키 화이트리스트 예외(권고) 또는 Unknown 이관 안 함 |

---

## 1. 사실 정정 (인계 r2·지시문 대비 — 전부 행번호 실측)

1. **①②의 전제가 현행 코드와 다르다.** 인계 §1.1은 `requestRate`를 "Prometheus(`resolveK8sMonitorDatasource`)"로, `alertCount`를 "알림 도메인"으로 기술했다. 실측: `service/k8s.go:232-243`에서 `RequestRate = fmt.Sprintf("%d Workloads", len(workloads))`, `AlertCount = metrics.AlertCount`이고 원천 `calculateK8sAggregateMetrics`(`k8s_build_net.go:503-525`)는 **not-Ready/unschedulable 노드 + failed/pending/unknown 파드 수**. `HealthScore`(`k8s_path.go:335`)·`cpu/memoryUsage`(`formatUsagePercent` :346)도 인벤토리 파생. `resolveK8sMonitorDatasource`(`k8s_metrics.go:238`)는 클러스터 등록 시 ID 유효성 검사 전용.
2. **`distribution[]`의 원천 2종 중 1종이 수집 금지다.** serviceCIDR 원천은 kube-system/kubeadm-config **data 값**(`k8s_overview.go:31-56`) — V2는 configmap 키 목록만 수집(mapping.md §2·보존 제약 #7). podCIDR 폴백 원천 `node.Spec.PodCIDRs`도 현행 디코드에 없다(`normalizer.go:42-58`). → 판정 ⑤.
3. **`CreateK8sResourceYAML`(create)은 V2 오퍼레이션 모델에 안 맞는다.** 라우트(`POST /infra/resources/:uid/operations/:name/…`)·엔진 `SubmitInput.ResourceUID`·관측 리프레시(`compose.go:188-208`) 전부 **존재하는 리소스의 uid**를 요구하는데 create는 대상이 없다. → `k8s.resource.apply`는 uid-스코프 update-apply로 한정, create는 이월(I-P1·Phase H 블로커).
4. **"오퍼레이션 10종" 계수 정정.** 인계 §2 표는 9행(신규 9종). 10종 = restart 포함 총계. G-P2a `grep -c RegisterOperation → 11 이상`은 현행 4콜사이트(`compose.go:250,343,346,349`) + 신규 9 = 13으로 성립.
5. **G-P1a/G-P1b의 게이트 명령이 공허 통과한다.** `-run TestMappingCoverage`는 현행 테스트명(`TestMappingTableCoversLegacyFields` 등, `mapping_test.go:125,166,201`)과 무매치. §7에서 실명 교정.
6. **Z 마커 총수는 120이다 (지시문 "121"과 불일치 — 실측 2회).** `grep -rc '// legacy 1행'`·`grep -rh '// legacy' | wc -l` 모두 **120**(7 테스트 파일 24+21+26+19+25+4+1, 헬퍼 0). TestChar 함수 수 120과 우연 일치. PC-4는 총수를 하드코딩하지 않는 **불식식**(잔여 + 원장행수 = 총수)으로 판정해 구현 시점에 자기검증한다.
7. **r1의 인용 오류 2건 정정**: compose RegisterOperation 2번째 proxmox 콜사이트는 `:346`(r1 `:345`) · executor operation 검사문은 `executor.go:210`(r1 `:209`).

---

## 2. 판정 제안 ①②③ + ⑤ (결정: 리뷰/사용자)

### ① `overview.requestRate` · ② `overview.alertCount` — 권고: **(가) V2 인벤토리 흡수(집계)**

- **근거 1 (사실)**: §1-1 — 둘 다 인벤토리 파생. (나)는 조회할 모니터링 경로가 v1에 존재하지 않는다.
- **근거 2 (row 19 정합)**: §3.2 row 19(Monitor = REMAIN)와 무충돌 — 흡수 대상은 인벤토리 집계.
- **근거 3 (mapping.md 정정)**: §4.2의 `dropped(monitoring-derived)` 행들은 사실 오기. P1-D가 전부 `mapped(집계)`로 전환하고 Z 오라클(`calculateK8sAggregateMetrics`·`calculateHealthScore`·`formatUsagePercent`)이 산식 동치를 잠근다.
- **파급**: 없음 — 순수 승리.

### ③ `cluster.monitorDatasourceId`·`monitorDatasourceName` — 권고: **혼합 (Id 흡수 + Name 조인)**

- **Id**: `provider_connection.ConfigJSON["monitor_datasource_id"]`(`internal/infra/model/provider.go:18` — JSON 칼럼, 마이그레이션 0). 인계 §4 register-k8s 규약 `--monitor-datasource-id`가 이 형상을 전제. `k8s_cluster` drop(Phase I) 이후 유일 생존 저장소.
- **Name**: 조립 시(`k8sassembly.go`) `monitor_datasource` 읽기 전용 조인 — 개체 소유는 모니터링 도메인(REMAIN 정합).
- **대안 배제**: (가) 전면 흡수는 이름 이중 저장(변경이 sync에 묶임)·(나) 전면 조회는 Id 저장소가 Phase I 이후 소멸.

### ⑤〔신설〕 `distribution[]` serviceCIDR — 권고: **(b) kubeadm-config 1종·2키 한정 예외** (대안 (a) Unknown 이관 안 함)

- **쟁점**: serviceCIDR은 kubeadm-config data 값에서만 나온다(v1 주석 `k8s_overview.go:27-30` 자인). 조립은 저장된 관측에서 일어나므로 저장이 필요 → 보존 제약 #7과 정면 충돌.
- **(b) 권고안**: mapping.md에 예외 조항 신설 — `kube-system/kubeadm-config` 1종, `serviceSubnet`·`podSubnet` 2키(네트워크 위상, 비밀 아님). 어댑터에 **화이트리스트 상수**로 하드코딩, 나머지 data 값은 계속 폐기(contracttest 카나리가 방어).
- **(a) 대안**: serviceCIDR 항상 "Unknown"(k3s에서 v1도 Unknown). podCIDR 성분만 수집으로 동치 — Z 오라클 `buildOverviewDistribution`은 부분 승계로 약화.
- **판정과 무관한 진행분**: `node.spec.podCIDRs` 수집(단일 `podCIDR` 필드 폴백 포함 — v1 `resolveK8sNetworkCIDRs`:57-80 의미론)은 양안 공통 전제로 P1-B에 포함.

---

## 3. 판정 ④ 분석 — `certificates[]` 보안 경계 (착수 금지 유지 · 결정: 리뷰/사용자)

**실측 (경로 전수)**: v1 `buildOverviewCertificates`(`k8s_overview.go:84-93`) → `parseOverviewCertificate`(:95-130): kubeconfig의 `certificate-authority-data`·`client-certificate-data`(base64 PEM)를 `x509.ParseCertificate`로 파싱해 **파생 메타데이터만** 반환(`K8sCertificate` 전필드 — Name·Type·Subject·Issuer·NotBefore·NotAfter·DaysRemaining·Status·StatusText). **원본 인증서·개인키·kubeconfig 문자열은 응답에 실리지 않는다**(`client-key-data` 미취급).

**§6 조항과의 충돌 여부**: mapping.md §6 "kubeconfig는 Material로만 진입 — 로그·에러·Raw·Normalized·아티팩트 노출 금지"의 해석이 대립한다 — 관대(금지는 재사용 가능한 **물질**; 만료일·CN은 관측) vs 보수(**유래 값 전부**; client-cert CN은 관리 인증서 신원 노출·정찰 가치 중간). 본 계획은 보수 해석 기준으로 설계했다.

**r1 A안의 오류 정정 (H5/H-2 상환)**: r1은 "어댑터 `Health()`가 provider_connection에 기록"이라 했다 — **arch rule 2 위반**(어댑터는 DB 핸들 무소유, `adapter.go:22-26`)이고 `contract.HealthResult`(`contract/adapter.go:53`)는 Healthy·Message 2필드뿐이라 담을 곳도 없었다. 실행 가능 변형 2종:

| 변형 | 설계 | 평가 |
|---|---|---|
| **A′-1 (권고)** | **`HealthResult`에 `Observation contract.JSONMap` 필드 확장(additive)** + 어댑터가 Health 내부에서 이미 파싱한 `clusterRuntime`(`client.go:42-51`)으로 인증서 관측을 도출해 실음 + **`compose/healthloop.go`가 기록** — health sweep은 이미 `probe()`(:174)에서 `Material["inventory"]`(상수 `healthPurpose="inventory"`, :28)로 Health를 호출하고 `markLastHealthAt`(:195)으로 provider_connection에 쓰고 있다. 관측도 같은 쓰기 패턴으로 ConfigJSON에 병합 | healthloop는 프로바이더 무관 유지(관측은 contract 필드로 흐른다)·전 변경이 P 소관(contract·adapter·compose)·`markLastHealthAt` 패턴 승계. Raw·Normalized 불침범 → §6 무충돌. 단점: contract 표면 1필드 확장 |
| A′-2 | 어댑터가 **순수 도출 함수 export**(`DeriveCertificateObservation(material)`)·healthloop이 어댑터 타입 스위치로 호출 | contract 무변경. 단점: 제네릭 sweep 계층에 k8s 특화 분기 — **기각 권고**(계층 오염) |
| B·C | r1과 동일 — B(조립 경로 broker 복호) 기각·C(이관 안 함) 패리티 결손 | §3 r1 참조 |

**착수 조건**: ④ 결착 전 `certificates[]` 커밋 금지(G-P1f). A′-1 채택 시 **P1-G1(관측 생산·기록) → P1-G2(조립·스왑)** 2 Phase로 착수(§6).

---

## 4. 아키텍처 (J판정)

### J-P1-0 kind 어휘 — `Phase6ResourceKindExtensions` 슬라이스 신설 〔r2·임계〕

신규 kind 4종은 `contract/resource_kind.go`에 **별도 슬라이스**로 적재한다(선례 `Phase2ResourceKindExtensions` — M1ResourceKinds는 §8.5 19종 verbatim 불변 단얫(T1)이라 직접 수정 불가, `IsKnownResourceKind` 조회부가 확장 슬라이스를 함께 조회하는 구조 승계):

```go
var Phase6ResourceKindExtensions = []string{
    "network.gateway", "network.http_route", "network.endpoint", "network.virtual_service",
}
```

검증은 `contract/adapter_test.go:45`의 확장 순회 패턴(`TestPhase2ResourceKindExtensionRecognized` :45, 순회 :47) 준수 — 4종이 IsKnownResourceKind 참이고 M1 19종과 교차 없음을 단얫. RS·Job·CronJob은 `orchestration.workload` subtype 확장이라 어휘 슬라이스 불필요. 최종 명칭은 리뷰 확정(§10-3). PP-0에 착지(어휘는 reviewed diff — 스펙 §8.5 문언과 정합).

### J-P1-1 수집기 통합 — 기존 Discoverer 패턴 준수

`discoverSections`(`adapter.go:192-206`)는 현재 **13행** — 6행 추가(P1-A·C1) + virtualservice 1행(P2-D) = **20행**:

| 섹션 | 경로 | 비고 |
|---|---|---|
| replicasets | `/apis/apps/v1/replicasets` | 수집 전용(스코프 외) — pod→워크로드 역추적 체인 |
| jobs | `/apis/batch/v1/jobs` | **비교 집합 합류(P1-C2)** — 합류 없이는 identity-sets-differ 상시(필요조건, §1 사실 6) |
| cronjobs | `/apis/batch/v1/cronjobs` | 동일 |
| gateways | `/apis/gateway.networking.k8s.io/{v1,v1beta1}/gateways` | 폴백 v1 404→v1beta1(`buildGatewayAPIResourcePathsWithPreferred` `k8s_transport.go:279` 의미론). 404는 섹션 스킵(신호 아님) |
| httproutes | `/apis/gateway.networking.k8s.io/{v1,v1beta1}/httproutes` | 동일 |
| endpoints | `/api/v1/endpoints` | v2-only 보조종 — service.endpoints 집계 원천 |
| virtualservices | `/apis/networking.istio.io/v1beta1/virtualservices` (P2-D) | 스코프 외 앵커 — istio 오퍼레이션 uid 닻 |

404-스킵은 `client.go doJSON`이 404를 식별 가능한 형태로 반환하게 최소 수정(401/403/429/전송은 기존 신호 분류 그대로).

### J-P1-2 비교 스코프 판정 〔r2 재판정·임계 — H4 상환〕

**LegacyCapture(`main_compare.go`) 실측**: `Workloads`(Name·Type·Namespace·Ready — :256-258)·`Services`·… 섹션만 캡처하고 **gatewayApiGateways·httpRoutes 섹션은 부재**. 따라서 gateway·httproute를 compare로 전환하면 V2 단독 종 → **identity-sets-differ BLOCKER**.

| 종 | 스코프 | 근거 |
|---|---|---|
| job·cronjob | **비교 집합 합류 (P1-C2)** | legacy Workloads 목록이 Jobs·CronJobs를 Type 문자열과 함께 실는다(`buildWorkloadItems` capacity 실측 `k8s_build_pod.go:175-`)·캡처도 Type 실음 → compare.go 스코프 전환만으로 성립 |
| gateway·httproute·replicaset·endpoint·virtualservice | **스코프 외 (v2-only)** — 비교 합류는 **I-P5 이월** | 캡처 부재(위)·replicaset은 legacy가 Deployment 파생 등장이라 실측 불일치·endpoint·virtualservice는 legacy 직렬화 없음 |

파급: 보존 제약(§9-10)·R-P1·PC-5·fixture(§6 P1-C2) 갱신. gateway/httproute 수집 자체는 정상 착지 — 비교 불참일 뿐(M-6 참조).

### J-P1-3 normalized 키 스키마 확장 (mapping.md §4와 1:1 — G-P1b가 기계 검증)

| 종 | 신규 normalized 키 | 원천 | 비고 |
|---|---|---|---|
| node | `kubeletVersion`·`internalIP`·`osImage`·`podCIDRs`(단일 `podCIDR` 폴백 포함)·`allocatablePods` | status.nodeInfo·addresses·spec.podCIDRs/podCIDR·allocatable["pods"] | P1-B |
| pod | `hostIP`·`podIP`·**`containers`**[{name, requests{cpuMilli,memBytes}, limits{…}}] | status·spec.containers.resources | **원시량은 milli·bytes 정수로 저장**(round3 실수 금지 — 포맷 시 왕복 손실 방지). ①② 집계의 분자는 **실행 pod 컨테이너 합**(`calculateK8sAggregateMetrics` pods 루프)이라 template만으론 Z 오라클 스왑이 불성립 |
| pod Raw | `ownerReferences`[{uid,kind,name}] | metadata.ownerReferences | workloadName/Type 집계 체인 (P1-A) |
| workload | `updatedReplicas`·`availableReplicas`·`containers`[{name,image,requests{…},limits{…}}] | status·spec.template.spec.containers | requests/limits 필드(§4 표)는 **양을 저장·조립이 포맷**(`formatWorkloadResourceSummary`·`formatCPUMilli`·`formatMemoryBytes` 오라클) |
| service | `externalIP` | status.loadBalancer.ingress | P1-B |
| ingress | `address`·`tls` | status.loadBalancer·spec.tls | P1-B |
| pv/pvc | `phase`·`namespaceScope`·`sourceType`·`sourcePath`·`nfsServer`·`reclaimPolicy` | status·annotation `ops-admin.io/namespace-scope`·spec.persistentVolumeSource(`k8s_build_net.go:399,414` 의미론) | P1-B |
| gateway | `gatewayClassName`·`hosts`·`addresses`·`ports` | spec.gatewayClassName·listeners(`collectGatewayAPIHosts/Ports/Addresses` `k8s_build_net.go:181,197` 의미론) | P1-C1 |
| httproute | `parents`·`targets` | spec.parentRefs·rules.backends(`collectHTTPRouteParents/Targets` :239,262 계열) | P1-C1 |
| job/cronjob | `completions`·`parallelism`·`schedule`·`active`·`succeeded`·`failed` | spec·status | buildWorkloadItems 동치 최소면(Z 테이블 확정) |
| endpoints | `readyAddresses`(서비스별 subset 주소 수) | subsets | v2-only |
| configmap(kubeadm-config 한정) | `serviceCIDR`·`podSubnetCIDR` | data 2키 | **판정 ⑤ (b) 채택 시만** |

**age 6건·VOLATILE 4건은 수집하지 않는다** — Raw `creationTimestamp`에서 조립 시 포맷(§15.3 정합).

### J-P1-4 집계·조립 계층 — `internal/infra/inventory/k8sassembly.go` 신규 (임계)

- 투영 확장이 아니라 조립 라이브러리 신규: (i) `projection.go`는 metrics-ext 공유 diff-0 제약 승계 (ii) `api/v2` `resourceView`는 §16.1 행 형상으로 목적이 다르다 (iii) C36이 Phase G 형상을 `service`→`inventory.ProjectResources` 호출로 못 박는다 — 호출 가능한 라이브러리의 소속은 P 범위뿐.
- 인터페이스(제안): `inventory.AssembleK8sClusterDetail(db, rows []ProjectedResource, conn 행) (model.K8sClusterDetail, error)` — 섹션별 조립은 순수 함수 export(Z 오라클 승계 지점). db는 ③ Name 조인·cluster 속성 읽기만(쓰기 없음).
- **v1 `model` DTO 재사용 권고**(순수 데이터 구조 — infra→model import 방향은 arch rule 무위반, Phase G 배선 최소화).

### J-P1-5 Z 오라클 승계 구조 (G-P1g) — 원장 33종 〔r2 재산출〕

**승계 원장 33종**(전부 각 1마커 실측): 인계 §5 명기 18종 + r1 계열 헬퍼 13종 + **r2 추가 2종(`collectGatewayAPIAddresses`·`resolveGatewayAPIAddress` — `k8s_build_net.go:197,205`, 마커 각 1)**. 이 중 **④ 대기 4종**(인증서) 제외 **29종**이 P1-E 즉시 스왑. 단일 원천 = `service/testdata/v2-swap-ledger.txt` — **원장은 순수 함수명 행만 기록한다(헤더·주석·빈행 금지 — `wc -l`이 곧 스왑 수 L이 되어 R+L=총수 불식식을 보존한다)**.

**마커 소속 실측 (r3 — E1/E2 배분의 근거)**: fetch 9(④ 4 포함)·build_net 10·build_pod 6·transport 2·path 6 = 33. **metrics·detail 테스트 파일에는 승계 마커가 0개** — 해당 2파일(마커 5개는 전부 비승계 함수)은 P가 건드리지 않는다.

**승계 선정 경계 (M-2)**: §1이 이관 원천으로 지목한 함수와 그 직계 헬퍼만. **§2(오퍼레이션) 헬퍼 5종은 승계에서 제외** — `replaceImageVersion`·`buildWorkloadImagePatchBody`·`buildWorkloadContainerPatchBody`·`serviceTargetPort`·`buildK8sDeleteResourcePaths`: 쓰기 경험식은 Z 동치가 아니라 **P2 contracttest 왕복이 대체 판정**한다.

| 형태 | 대상 | 스왑 방식 |
|---|---|---|
| **직접 대응형** (원시 입출력) | `formatCPUMilli`·`formatMemoryBytes`·`formatWorkloadResourceSummary`·`calculateHealthScore`·`formatUsagePercent`·`k8sCertificateStatus`(④후)·`certificateCommonName`(④후)·`nodeReadyStatus`·`joinNodeRoles`·`formatTimestamp`·`fallbackText` | V2 동일 시그니처 순수 함수 — 1행을 패키지 한정자만 교체. `nodeReadyStatus`·`joinNodeRoles`는 V2 어휘(healthState·roles[])↔v1 어휘 매핑이 글루에 포함 |
| **파이프라인형** (구조체 입출력) | `buildOverviewDistribution`·`resolveK8sNetworkCIDRs`·`buildOverviewCertificates`(④후)·`parseOverviewCertificate`(④후)·`calculateK8sAggregateMetrics`·`buildNamespaceCounts`·`buildPodItemsWithWorkloads`·`serviceExternalIP`·`buildEndpointCounts`·`persistentVolumeSource`·`storageNamespaceScope`·`collectGatewayAPIHosts`·`collectGatewayAPIPorts`·**`collectGatewayAPIAddresses`·`resolveGatewayAPIAddress`**·`flattenGatewayHosts`·`flattenGatewayPorts`·`collectHTTPRouteParents`·`collectHTTPRouteTargets`·`buildHTTPRouteTrafficItems`·`buildJobDetail`·`buildCronJobDetail` | 헬퍼 `k8s_chartest_helper_test.go`에 테스트 전용 글루: legacy 구조체 → `json.Marshal` → `kubernetes.NormalizeSection`(export 래퍼) → 조립. 본문 1행을 `v2X(...)`로 교체 — **입력 테이블·기대값 무수정** |

- **소유권 경계**: v1 프로덕션 무변경. `service/k8s_*_test.go`·헬퍼는 (i) 1행 교체 (ii) 글루 (iii) 마커 주석 `// legacy 1행`→`// v2 oracle`만. 각 스왑 PR은 원장 적재 + C47 baseline과 PASS 집합 동일을 증명.
- **M-8 spike**: P1-E1 첫 스왑은 `formatCPUMilli`(직접대응형 최소)로 교정 — 글루·원장 패턴이 확정된 뒤 나머지를 진행한다.
- **글루 계층 필요성(셀프 단순성)**: 타입 경계(v1 소문자 구조체 ↔ V2 normalized)를 넘어 "같은 입력→같은 출력"을 보존하는 유일 수단. 대안(Z 테스트의 internal 이전)은 C47 baseline·서비스 테스트 소유권을 깨뜨려 기각.

### J-P1-6 P2 실행기·오퍼레이션 — `internal/api/v2/` 무변경 · 오퍼레이션 10종 확정표

**확정표** (인계 §2 제안값 채택 + 수정 2건: apply를 update 한정 축소·전 opdef 승인 필수. 권한 전부 v1 재사용 — 신규 문자열 0). v1 원천 라인은 **분해 후 실측**(M-1):

| operation | v1 원천 (현행) | RequiredPermission | risk | 승인 | IdempotencyPolicy | handle·poll |
|---|---|---|---|---|---|---|
| `k8s.workload.restart` (기존) | `k8s_workload.go` | `assets:k8s:workload:restart` | medium | 필수 | provider_frozen_annotation | rollout 수렴(현행) |
| `k8s.workload.scale` | `k8s_workload.go:61` | `assets:k8s:workload:scale` | medium | 필수 | provider_state_convergent | rollout 수렴 |
| `k8s.workload.image_update` | `k8s_workload.go:149` | `assets:k8s:workload:image` | medium | 필수 | provider_frozen_payload | rollout 수렴 |
| `k8s.workload.resources_update` | `k8s_workload.go:232` | `assets:k8s:workload:yaml` | medium | 필수 | provider_frozen_payload | rollout 수렴 |
| `k8s.node.labels_update` | `k8s.go:308` | `assets:k8s:workload:yaml` | medium | 필수 | provider_state_convergent | 발행+poll 1회 |
| `k8s.service.update` | `k8s_detail.go:74` | `assets:k8s:workload:yaml` | medium | 필수 | provider_state_convergent | 발행+poll 1회 |
| `k8s.resource.apply` (**update 한정**) | `k8s_mutate.go:15` | `assets:k8s:workload:yaml` | **high** | 필수 | provider_frozen_manifest | 발행+poll 1회 |
| `k8s.resource.delete` | `k8s_mutate.go:122` | `assets:k8s:resource:delete` | **high** | 필수 | provider_terminal_404 | poll 404=Succeeded |
| `k8s.istio.traffic_update` | `k8s_mutate.go:151` | `assets:k8s:advancednetwork` | medium | 필수 | provider_frozen_payload | 발행+poll 1회 |
| `k8s.httproute.traffic_update` | `k8s_mutate.go:202` | `assets:k8s:advancednetwork` | medium | 필수 | provider_frozen_payload | 발행+poll 1회 |

- **dispatch**: `executor.go:210` 검사문을 operation-name switch로 일반화. 9종 파일 분할: `executor_workload.go`·`executor_config.go`·`executor_resource.go`(high)·`executor_traffic.go`(+각 `_test.go` — 현행 `executor_test.go` 831줄이라 신규 분은 신규 파일에).
- **payload 검증은 실행기 자체**(`frozenRestartedAt` `executor.go:191-204` 선례) — `operations.go` `executePayload`는 제네릭 패스스루라 `api/v2` 수정 0. plan 응답의 `restartedAt` 키가 타 op에 붙는 것은 기존 동작(무해).
- **권한**: §10.3 carry-over. apply·delete는 v1이 동일 권한으로 이미 임의 kind를 다뤘다 — **risk·승인만 상향**(high — pve config 선례 `compose.go:395-408`·승인 필수 선례 restart `compose.go:287`). 분리 권한 신설은 차후 하드닝(§10-4).
- **M-5 (per-op 권한의 문서적 소재)**: 골든 292행 동결 유지 시 sensitive-routes.txt에 op별 행을 넣을 수 없고 주석도 아티팩트 테스트 형상을 깨뜨린다 — **def의 소재는 코드(§3.3 descriptors are code) + `TestOperationDefTable` + 본 표**가 단일 권위. 생성기가 주석을 지원하면 재검(I-P).
- **앵커 2건**: istio는 VirtualService 앵커 수집(P2-D)·apply create는 이월(I-P1).

### J-P1-7 G-P2c 등재 — 재해석은 미해결로 이관 (M-4)

plan/execute는 파라미터 라우트 2개(`sensitive-routes.txt:157-158`)에 대표 권한으로 실리고 집행은 레지스트리 def에서 요청별 분해(`opdef.V2DynamicMiddleware` + `ResolveOperationPermission` `routes_v2.go:33-34`). 골든에 10행 실기가 물리적 불가능하므로 def 등재처를 **코드+`TestOperationDefTable`(P2-E deliverable)** 로 하는 재해석을 제안하나 — 인계 게이트 문언 변경이므로 **§10 미해결 5로 승격**(리뷰 승인 후 확정). 골든 292·452 불변은 유지 원칙.

### J-P1-8 GatewayAPI CRD 부재 (M-6) — 픽스처 방침

양 실측 환경 모두 `gateway.networking.k8s.io` CRD 부재(fixture `seed.yaml` kind는 Namespace·Deployment·StatefulSet·DaemonSet뿐). gateway·httproute 수집은 실클러스터에서 **공허 녹색**(0행)이 된다. 방침: (i) 정규화 검증은 **단위 테스트 픽스처**로 충분하게 작성(수집·폴백·정규화 전 경로) (ii) 실클러스터 CRD 설치·시드는 I-P5(비교 합류 재검) 시점에 함께 — 지금 설치하지 않는다(비교 불참인데 인프라만 늘림). **공허 녹성임을 PR에 명시**한다.

### J-P1-9 43필드 ↔ Phase 대응표 (H3) — 버킷 산술 재정의

버킷 재정의(인계 §1.5 "집계 6"은 overview만 센 것): **수집 신규 19 · 집계 13 · 조립 9 · 판정 2엔트리(③ 1엔트리=2필드·④ 1) = 43**.
**계상 규칙**: 이중 처분 행(수집+집계·수집+조립 혼합 — service.endpoints·workload requests/limits)은 **수집 버킷에 1회만 계상**하고 조립·집계 성분은 착지 칸에 병기한다. `certificates[]`는 판정④ 엔트리로 계상한다. 이 규칙으로 19/13/9/2가 재현된다.

| 필드 (엔트리) | 버킷 | 착지 |
|---|---|---|
| overview: healthScore·cpuUsage·memoryUsage·podUsage·requestRate·alertCount (6) | 집계 | P1-D |
| overview: distribution (1) | 조립(⑤) | P1-D |
| overview: certificates (1) | 수집+④ | P1-G1·G2 |
| nodes: version·internalIP·os (3) / pods(1) | 수집 / 집계 | P1-B / P1-D |
| namespaces: pods·services·workloads (3) | 집계 | P1-D |
| pods: workloadName·workloadType (2) / nodeIP·ip (2) / age (1) | 집계(RS 체인—원천 Raw P1-A) / 수집 / 조립 | P1-D / P1-B / P1-D |
| workloads: updated·available (2) / requests·limits (2, 원시량 수집+조립 포맷) / age (1) | 수집 / 수집+조립 / 조립 | P1-B / P1-B·D / P1-D |
| services: externalIP (1) / endpoints (1) / age (1) | 수집 / 수집(endpoints 보조종 P1-A)+집계 / 조립 | P1-B / P1-A·D / P1-D |
| ingresses: address·tls (2) / age (1) | 수집 / 조립 | P1-B / P1-D |
| storage 6 (namespaceScope·status·sourceType·path·nfsServer·reclaimPolicy) | 수집 | P1-B |
| configStorage: configMaps.age·secrets.age (2) | 조립 | P1-D |
| cluster: nodeCount (1) / statusText·gatewayName (2) / monitorDatasourceId·Name (1엔트리) | 집계 / 조립 / 판정③(Id 등록 흡수·Name 조립 조인) | P1-D 전부 |

부수(본표 외): `allocatable.pods` 1키(P1-B)·pod/workload 컨테이너 원시량(J-P1-3)·ownerReferences Raw(P1-A)·endpoints·RS 앵커 수집(P1-A)·gateway/httproute 종(P1-C1·비교 불참).

---

## 5. 변경 범위 전수

**신규**

| 파일 | Phase | 내용 |
|---|---|---|
| `internal/infra/inventory/k8sassembly.go`·`_test.go` | P1-D | 조립 라이브러리(집계 13·조립 9·cluster·overview) — `TestAssembleK8sClusterDetail` 명명 고정 |
| `internal/infra/adapter/kubernetes/normalizer_batch.go` | P1-A | normalizer 분할(611줄 → batch 종 디코드+키) |
| `internal/infra/adapter/kubernetes/normalizer_gw.go` | P1-C1 | gateway/httproute 디코드+키 |
| `internal/infra/adapter/kubernetes/executor_{workload,config,resource,traffic}.go`(+각 _test) | P2-A~D | 오퍼레이션 9종 — `TestOperationContractRoundtrip`(왕복)·`TestExecute*`·`TestPoll*` 명명 고정 |
| `internal/infra/contracttest/opharness.go`(+_test) | P2-A | 오퍼레이션 왕복 하네스(adapter 비의존 seam) |
| `internal/infra/contracttest/opdef_table_test.go` | P2-E | `TestOperationDefTable` — 10종 def 확정표 단얫 |
| `service/testdata/v2-swap-ledger.txt` | P1-E1 | 승계 원장(33종 최종) |
| `v2-phase1/k8s-fixture/seed.yaml` 확장 | P1-C2 | Job 1·CronJob 1 시드(비교 합류 비공허화 — batch CRD는 kind 내장) |

**수정**

| 파일 | Phase | 변경 |
|---|---|---|
| `internal/infra/contract/resource_kind.go`·`contract/adapter_test.go` | PP-0 | `Phase6ResourceKindExtensions` 4종 + 확장 순회 단얫 |
| `adapter/kubernetes/mapping_test.go` | PP-0·P1 전 Phase | G-P1b 강화(`normalizedKeyFor`·`assertNormalizedKey` — `TestMappedFieldsProduceNormalizedKeys` 명명 고정)·coverageTable 확장 |
| `adapter/kubernetes/adapter.go` | P1-A·C1·P2-D | discoverSections 6+1행·폴백·virtualservice (306→~400 예상) |
| `adapter/kubernetes/normalizer.go` | P1-A·B | 디코드·키 확장(분할 후 각 ≤650) |
| `adapter/kubernetes/client.go` | P1-C1 | 404 식별 최소(신호 분류 불변) |
| `internal/infra/compose/compose.go` | P1-A·C1·P2-A~D | readKinds + `registerKubernetesMutations` 9종 |
| `internal/infra/inventory/compare.go`·`compare_test.go` | P1-C2 | job·cronjob 비교 합류(페어링·스코프) |
| `adapter/kubernetes/mapping.md` | P1 전 Phase·P2-E | §1 종표·§2 예외(⑤)·§3.2·§4 전필드 전환·§6 예외 |
| `internal/infra/contract/adapter.go` | P1-G1 | `HealthResult.Observation` 필드(additive) |
| `internal/infra/compose/healthloop.go`(+_test) | P1-G1 | 관측 ConfigJSON 기록(`markLastHealthAt` 패턴) |
| `service/k8s_*_test.go` 5종(fetch·build_pod·build_net·transport·path)·`k8s_chartest_helper_test.go` | P1-E1·E2·G2 | 승계 유일 허용 변경 — 29+4마커 스왑 + 글루. **metrics·detail 테스트는 승계 마커 0 — 무변경** |
| `v2-phase1/RESUME.md` | 각 Phase | 진행 갱신 |

**불변**: `internal/api/v2/**`·`internal/infra/metrics/**`·`inventory/projection.go`·**`backend/main_compare.go`**(I-P5 이월 — 비교 캡처 확장은 P 기간 무변경)·`service/*.go`(비테스트)·`router/**`·골든 2종·DB 스키마(신규 칼럼 0).

---

## 6. Phase 분해 (의존 순서 — 15개)

```
PP-0 ─▶ P1-A ─▶ P1-B ─▶ P1-C1 ─▶ P1-C2 ─▶ P1-D ─▶ P1-E1 ─▶ P1-E2 ─▶ P1-F ─▶ [④ 결착 후] P1-G1 ─▶ P1-G2
   │                      │
   │ (P2-A~C는 P1과 무관 — 워크로드·노드·서비스 종 기수집 — 병행 가능)
   └────────────────────────────▶ P2-A ─▶ P2-B ─▶ P2-C ─▶ P2-D ─▶ P2-E   (P2-D만 P1-C1 후 — 앵커 선행)
```

| Phase | 파일수 | 내용 (신규 테스트명은 deliverable에 고정) | 검증 |
|---|---|---|---|
| **PP-0** 〔임계〕 | 3 | `Phase6ResourceKindExtensions` 4종(resource_kind.go·adapter_test.go) + mapping_test 강화(`TestMappedFieldsProduceNormalizedKeys`) | PC-K·PC-0 |
| **P1-A** | 5 | RS·Job·CronJob·endpoints 4섹션 + ownerReferences·podCIDRs + normalizer 선제 분할(normalizer_batch.go). 신규 종 스코프 외 | PC-1·PC-C |
| **P1-B** | 5 | 필드 수집 19건 + pod/workload 컨테이너 원시량(milli/bytes) — `TestNormalizedKeySchema` 고정 | PC-B·PC-1 |
| **P1-C1** | 5 | gateway·httproute 수집기 + 버전 폴백 + 404-스킵(client.go) + 종 2종 | PC-1·PC-2 |
| **P1-C2** | 4 | **job·cronjob 비교 집합 합류**(compare.go) + fixture Job/CronJob 시드 | PC-A·PC-2 |
| **P1-D** | 3 | 조립 라이브러리 — 집계 13·조립 9·③ 혼합·⑤ 반영 — `TestAssembleK8sClusterDetail` 고정 | PC-3·PC-D |
| **P1-E1** | 5 | Z 스왑 1차 — fetch(④ 4 제외 5)·build_pod(6)·build_net(10) 계열 **21마커** + 원장 신설. **spike: formatCPUMilli 최초 1건으로 패턴 교정(M-8)** | PC-4 |
| **P1-E2** | 3 | Z 스왑 2차 — transport(2)·path(6) 계열 **8마커** → 원장 ≥29. **metrics·detail은 승계 마커 0(실측) — 무변경** | PC-4 |
| **P1-F** | 2 | 게이트 종결 — mapping.md G-P1c 전환 완료(certificates → `pending(④-보안판정)`)·판정 표 갱신 | PC-5 |
| **P1-G1** 〔④ 후〕 | 5 | certificates 관측 생산·기록 — contract/adapter.go(Observation)·k8s adapter.go·adapter_test.go·healthloop.go·healthloop_test.go(`TestHealthSweepRecordsObservation` 고정) | PC-6 |
| **P1-G2** 〔④ 후〕 | 5 | certificates 조립·mapping.md 최종 처분·Z 스왑 4마커 — 원장 ≥33 | PC-6·PC-4·PC-2 |
| **P2-A** | 5 | workload 3종 실행기(+test) + compose 등록 + opharness(+test, `TestOperationContractRoundtrip`) | PC-7·PC-2 |
| **P2-B** | 3 | node labels·service.update(+test) | PC-7 |
| **P2-C** | 3 | resource.apply(update)·delete(+test) — high·권한 검토 결론 반영 | PC-7·PC-8 |
| **P2-D** | 5 | virtualservice 앵커 수집 + istio·httproute traffic(+test) | PC-7·PC-9 |
| **P2-E** | 3 | `TestOperationDefTable`·sensitive 대표 행 정합·mapping.md 종행 | PC-10 |

**임계경로(직렬 신중)**: PP-0(어휘·검출기 — 없으면 이후 전부 공허) → P1-A/B(normalizer 분할 경계) → P1-D(집계 산식 = Z 오라클 대응 구현과 동일 파일) — 세 지점 직렬. P2-A~C 병행, P2-D는 P1-C1 후.
**단순성 셀프검증**: 15 Phase는 병합 시 전부 6파일 초과(P1-C1+C2=9·G1+G2=10·E1+E2=10)하는 경계들 — 분할이 규칙의 결과다. ④ 2분할(G1/G2)은 결착 시점 분리 원칙의 산물.
**착수 대 분리(지시 8호)**: certificates 제외 42필드는 P1-A~F로 ④ 무관 완결. 판정 ①②③⑤는 P1-D 전, ④는 P1-G1 전 결착. G-P1f가 ④ 미결착 상태 인증서 구현 부재를 기계 판정.

---

## 7. 검증 요구 (claims — 실행 명령 원문 + exit code)

전제 `cd /mnt/d/DEV/acc0mplish/ops-admin`. Phase 시작 시 `git tag p-<phase>-base`. **참조 테스트명은 전부 실재(deliverable) — `-run` 무매치 공허 통과 없음(셀프 검증성)**.

```
--- 공통 (전 Phase 종료) ---
PC-C  cd backend && go test ./... -race -count=1 → ok
PC-S  python3 scripts/secret-scan.py → exit 0
PC-L  wc -l backend/internal/infra/adapter/kubernetes/*.go backend/internal/infra/inventory/k8sassembly*.go \
        backend/internal/infra/contracttest/opharness.go backend/internal/infra/compose/healthloop.go \
        | grep -v total | awk '$1>800' → 0행 · awk '$1>650' → 초과 시 같은 Phase에서 분할
PC-V  git diff --stat p-<phase>-base -- backend/internal/infra/metrics backend/internal/infra/inventory/projection.go \
        backend/internal/api/v2 backend/main_compare.go \
        backend/service -- ':!backend/service/k8s_chartest_helper_test.go' ':!backend/service/k8s_fetch_test.go' ':!backend/service/k8s_build_pod_test.go' ':!backend/service/k8s_build_net_test.go' ':!backend/service/k8s_transport_test.go' ':!backend/service/k8s_path_test.go' ':!backend/service/k8s_metrics_test.go' ':!backend/service/k8s_detail_test.go' ':!backend/service/testdata' → 출력 없음
      (main_compare.go 포함 — I-P5 이월로 P 기간 무변경 계약인데 r2까지 게이트가 없었다, M-P1)
PC-N  [P 첫 코드 PR] 블록1 미징수 카나리 C24·C31·C32 징수(p6-state.json "next" 승계 — 지시 아닌 권고, 미징수 시 PR에 사유)

--- PP-0 ---
PC-K  cd backend && grep -c 'Phase6ResourceKindExtensions' internal/infra/contract/resource_kind.go → 1 이상 ·
      go test ./internal/infra/contract/ -count=1 → ok (M1 19종 불변·4종 인지 단얫)
PC-0  cd backend && go test ./internal/infra/adapter/kubernetes/ -run TestMapping -count=1 → ok 이고
      grep -c 'normalizedKeyFor\|assertNormalizedKey' internal/infra/adapter/kubernetes/mapping_test.go → 2 이상
      (G-P1b — `-run TestMappingCoverage` 공허 통과 폐기·실명 교정)

--- P1-A·B·C1 (수집) ---
PC-1  cd backend && for k in gatewayapi httproute replicaset job cronjob endpoint; do \
        grep -qi "$k" internal/infra/adapter/kubernetes/adapter.go || echo "MISSING $k"; done → 0행 (G-P1d + endpoint)
PC-2  cd backend && go test ./internal/infra/contracttest/ ./internal/infra/adapter/kubernetes/ -count=1 → ok
PC-B  cd backend && go test ./internal/infra/adapter/kubernetes/ -run TestNormalizedKeySchema -count=1 → ok (P1-B deliverable)

--- P1-C2 (비교 합류) ---
PC-A  cd backend && go test ./internal/infra/inventory/ ./internal/infra/adapter/kubernetes/ \
        -run 'TestComparePair|TestComparisonScope' -count=1 → ok (실존명 — TestScope 0매치 폐지)
      grep -c '^kind: Job\|^kind: CronJob' ../v2-phase1/k8s-fixture/seed.yaml → 2 (시드 — 비교 비공헌)

--- P1-D (조립) ---
PC-3  cd backend && go test ./internal/infra/inventory/ -run TestAssembleK8sClusterDetail -count=1 → ok (deliverable)
PC-D  grep -c 'dropped(monitoring-derived)' backend/internal/infra/adapter/kubernetes/mapping.md → 0 (판정 ①② 전환)

--- P1-E1·E2 (Z 승계 — G-P1g) ---
PC-4  cd backend && R=$(grep -h -c '// legacy 1행' service/k8s_fetch_test.go service/k8s_build_pod_test.go \
        service/k8s_build_net_test.go service/k8s_transport_test.go service/k8s_path_test.go \
        service/k8s_metrics_test.go service/k8s_detail_test.go | awk -F: '{s+=$NF} END {print s}') ·
      L=$(wc -l < service/testdata/v2-swap-ledger.txt) · echo $((R + L)) → 120
      (잔여+원장=총수 — 계획 실측 120. 구현 최초 스왑 시 재검하여 지시문 121과 상충하면 실측·원장으로 확정 후 본 기대값과 §1-6을 함께 정정)
      L ≥ 29 (E2 종료 시점) · grep -h -c '// v2 oracle' service/k8s_*_test.go | awk -F: '{s+=$NF} END {print s}' → L 과 일치
      cd backend && go test ./service/ -run TestChar -count=1 -v 2>&1 | grep -E '^(--- )?(PASS|FAIL): TestChar' \
        | sed 's/ ([0-9.]*s)$//' | sort > /tmp/char-p.txt && \
        diff service/testdata/char-baseline.txt /tmp/char-p.txt && echo CHAR_IDENTICAL → CHAR_IDENTICAL

--- P1-F / P2-E (게이트 총점검) ---
PC-5  G-P1a: cd backend && go test ./internal/infra/adapter/kubernetes/ -run TestMapping -count=1 → ok (실명)
      G-P1c: cd backend && grep -c 'dropped(v2-schema-absent)\|dropped(v2-not-collected)' \
        internal/infra/adapter/kubernetes/mapping.md → 0
      G-P1e(C21): kubectl --context kind-v2-p2 -n v2-seed get cronjob seed-cron → NotFound 확인 후 \
        cd backend && for i in 1 2 3; do go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124 && \
        go run . compare-inventory --cluster 1; done → 3회 전부 verdict pass (JSON 파서 판정 — 연속성으로 churn 안정 확인) · \
        종료 시 python3 -c "import json,glob,os; f=max(glob.glob('data/compare/1/*/*.json'),key=os.path.getmtime); print(json.load(open(f))['verdict'])" → pass (최신 아티팩트 위생)
      G-P1f: (repo root) grep -rn 'certificateObservation' backend/internal/infra/ → 0행 이고 \
        grep -ci 'certificate' backend/service/testdata/v2-swap-ledger.txt → 0 (④ 결착 전 — 인증서 구현·스왑 부재)
PC-10 G-P2a: cd backend && grep -c 'RegisterOperation' internal/infra/compose/compose.go → 11 이상
      G-P2b: cd backend && go test ./internal/infra/contracttest/ -count=1 → ok 이고 \
        go test ./internal/infra/adapter/kubernetes/ -run TestOperationContractRoundtrip -count=1 → ok (10종 왕복 — P2-A deliverable)
      G-P2c: cd backend && go test ./internal/infra/contracttest/ -run TestOperationDefTable -count=1 → ok (P2-E deliverable — J-P1-6 확정표와 1:1)
      grep -c . docs/security/sensitive-routes.txt → 292 · grep -c . docs/security/route-inventory.txt → 452 (불변)

--- P1-G1·G2 (④ 결착 후) ---
PC-6  grep -c 'pending(④' backend/internal/infra/adapter/kubernetes/mapping.md → 0 · PC-4 재실행(원장 ≥33) · \
      PC-2 재통과 · cd backend && go test ./internal/infra/compose/ -run '^TestHealthSweepRecordsObservation$' -count=1 -v → ok (1 test ran)
      (exact 매치 — 패밀리 접두만으론 기존 5개로 녹색이 되는 공백 봉쇄, M-P3. `-v`의 "1 test ran"으로 매치 1건 확인)

--- P2 (오퍼레이션) ---
PC-7  cd backend && go test ./internal/infra/adapter/kubernetes/ -run 'TestExecute|TestPoll' -count=1 → ok
      (실존 가족명 확장 — `-run TestExecutor` 0매치 폐지)
PC-8  cd backend && go test ./internal/infra/contracttest/ -run TestOperationDefTable -count=1 → ok (risk high 2종·권한 10종 단얫 포함 — grep 다중행 리터럴 폐지) · \
      grep -c 'RequiredPermission' internal/infra/compose/compose.go → 13 이상 (M-7 완화)
PC-9  grep -ci 'virtualservice' backend/internal/infra/adapter/kubernetes/adapter.go → 1 이상 (istio 앵커)
```

---

## 8. 위험 매트릭스 + 임계경로

| # | 위험 | 가능 | 파급 | 완화 | 검출 |
|---|---|---|---|---|---|
| **R-P1** | job/cronjob 비교 합류가 C21을 뒤집는다(합류 초기 드리프트) | 중 | 높 | 합류는 P1-C2 단일 착지점·fixture 시드로 양측 비공헌·드리프트 시 종 스코프 외 강등 후 재판정. gateway/httproute는 애초에 합류 안 함(§J-P1-2) | PC-5 |
| **R-P2** | Z 스왑이 테스트 복사가 된다(기대값 수정) | 중 | 높 | 기대값 무수정 원칙(§9-1)·원장 기록·spike(M-8)로 패턴 선교정·리뷰 확인 | PC-4·리뷰 |
| **R-P3** | normalizer 611줄 + 6섹션·20키 → 800 초과 | 높 | 중 | P1-A 선제 분할(normalizer_batch)·P1-C1 재분할(normalizer_gw) | PC-L |
| **R-P4** | 판정 ⑤ (b) 예외가 configmap 값 금지의 구멍 | 낮 | 높 | 화이트리스트 상수 1종 2키 하드코딩·mapping.md 문언·카나리 유지 | PC-2 |
| **R-P5** | apply(update) 한정이 Phase H의 v1 create 라우트 삭제를 막는다 | 높 | 중 | I-P1 명시 — H 착수 조건에 커넥션-스코프 라우트(§16.1 확정) 선행 | 리뷰 |
| **R-P6** | VirtualService 앵커가 S-2 삭감 잠식 오해 | 중 | 낮 | S-2는 읽기 패리티 제외 — 앵커는 쓰기 uid 닻(스코프 외·비교 불참). PR에 명기 | 문서 |
| **R-P7** | 10종 opdef가 zero-grant replay·authz 재현을 깨뜨린다 | 낮 | 높 | 라우트 0 신설·골든 불변·대표 권한 체계 유지 | PC-C·PC-10 |
| **R-P8** | 조립의 monitor_datasource 조인이 inventory→DB 결합 심화 | 중 | 중 | 읽기 전용 1조인·arch rule 2 무관·conn 행 주입으로 순수 함수는 DB 무지 | PC-3 |
| **R-P9** 〔r2〕 | ④ A′-1의 HealthResult 확장이 다른 어댑터 호흡을 깨뜨린다 | 낮 | 중 | additive 필드(영값 호환)·contract 전체 테스트가 즉시 판정 | PC-K·PC-C |
| **R-P10** 〔r2〕 | job/cronjob 합류 후 seed-cron 재등장이 churn 차이 재발 | 중 | 중 | fixture는 저장소 시드가 아님을 문서화·C21 3회 연속 판정이 churn 안정 확인 | PC-5 |

**임계경로**: PP-0 → P1-A/B → P1-D (직렬 신중). P2-A~C 병행·P2-D는 P1-C1 후.
**롤백**: 전 Phase 독립 PR·revert 역순. Z 스왑은 테스트 전용(원장과 함께 revert → C47 baseline 원복). P1은 additive — 스코프 합류 커밋이 compare.go와 같은 PR이므로 원자 revert. DB 스키마 0. mapping.md는 각 PR에 동반.

---

## 9. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **Z 테스트 120(마커 120)·골든 452/292·C47 baseline 불가침.** 유일 허용 변경은 승계 규약의 (i) legacy 1행의 V2 호출 교체 (ii) 헬퍼 글루 (iii) 마커 주석 교체. **테이블 입력·기대값은 무수정** — V2 구현이 기대값에 맞춰진다.
2. **`service/*.go` 프로덕션 무변경** — P 범위는 `backend/internal/infra/`·`backend/internal/tasks/`. v1 함수는 읽기 원천만. `service/k8s_*_test.go`·헬퍼는 제약 1의 변경만.
3. **`internal/infra/metrics/**` 금지**(metrics-ext 소관) · **`inventory/projection.go` diff 0**(공유 제약 승계 — 조립은 `k8sassembly.go`만) · **`internal/api/v2/` 무변경**(payload는 실행기 자체 검증).
4. **kubeconfig 봉인**(인계 §4·mapping.md §6) — Material 경로만·로그·에러·Raw·Normalized 노출 금지. `certificates[]`는 판정 ④ 결착 전 착수 금지(G-P1f). 판정 ⑤ 예외(채택 시)는 kubeadm-config 1종 2키 화이트리스트만.
5. **원격 CI 대기 금지** — §7 claims가 전부 로컬 판정. arch-boundary·secret-scan 통과.
6. **라우트·권한 문자열 신설 0** — 골든 292/452 불변·오퍼레이션 권한 v1 재사용. 신규 문자열 결론은 리뷰 승인 후만.
7. **파일 크기** — 650 경고·800 하드캡(PC-L). normalizer는 P1-A·C1 선제 분할. 신규 executor 테스트는 신규 파일에(현행 `executor_test.go` 831줄 — 증가 금지). 〔각주: 본 §9의 번호는 P 계획 자체의 것 — 모계획 보존 제약 #6·#7·mapping.md "보존 제약 #7"(kubeconfig 봉인) 인용 시 원문 번호를 명시해 혼동을 피한다.〕
8. **DB 스키마 불변** — 신규 칼럼 0. 커넥션 속성(③ Id·④ 관측)은 기존 ConfigJSON만.
9. **각 Phase는 단독 revert 가능 PR** — 스코프 합류·mapping.md 전환·테스트는 해당 커밋과 같은 PR.
10. **비교 스코프 변경은 P1-C2 단일 착지점** — job·cronjob 외 종의 compare 전환 금지(gateway·httproute는 I-P5 승인 전 불가·LegacyCapture 부재는 H4 실측 근거).

---

## 10. 이월 대장 · 미해결

**이월**

| # | 항목 | 재진입 |
|---|---|---|
| I-P1 | `k8s.resource.apply` create 의미론 — 커넥션-스코프 오퍼레이션 라우트 신설 필요(§16.1 확정)·대안 CLI. **Phase H가 v1 create 라우트 삭제하려면 선행 필수** | Phase H 전(블로커) |
| I-P2 | plan 응답 `restartedAt` 제네릭화 — `api/v2` 변경이라 P 밖 | 블록 2 G·H |
| I-P3 | `workloads.ready` "1/3" 분해의 조립 대응(mapping.md §4.6 변환 규칙)·마커 없는 age 포맷 잔여 | P1-E2 리뷰 |
| I-P4 | register-k8s CLI(인계 §4) — Phase H 소관. 본 계획은 ③ Id 저장처·봉인 규약만 확정 | Phase H |
| **I-P5** 〔r2 확장〕 | gateway·httproute·replicaset·endpoint·virtualservice의 **비교 집합 승격** — LegacyCapture 섹션 확장(main_compare.go·compare.go) + **GatewayAPI CRD 실클러스터 설치·시드**를 포함 | M2 완전성·필요 시 |
| I-P6 | sensitive-routes 생성기의 동적 라우트 주석 지원(M-5 재검) | 생성기 개정 시 |

**미해결 (판정 대기 — 결정권: 리뷰/사용자)**

1. **판정 ①②③⑤ 승인** — P1-D 전. ①② (가)·③ 혼합·⑤는 (b) 예외(보존 제약 인접 — 명시 승인).
2. **판정 ④ 결착** — P1-G1 전. 권고 A′-1(HealthResult.Observation + healthloop 기록).
3. **kind 어휘 최종 명칭** — `network.gateway`·`network.http_route`·`network.endpoint`·`network.virtual_service` 리뷰 확정(PP-0 착지 전).
4. **apply·delete 권한 검토 결론** — 권고: v1 문자열 재사용 + high/승인 상향. 분리 권한은 하드닝. P2-C 전.
5. **〔r2·M-4〕G-P2c 등재처 재해석 승인** — "sensitive-routes.txt 10행 등재"를 코드+`TestOperationDefTable`로 대체하는 인계 게이트 문언 변경. 골든 292 불변 원칙 유지 전제.

---

## 11. 결정 기록

- 모집단 = `K8sClusterDetail` 43필드(§J-P1-9 대응표 — 수집 19·집계 13·조립 9·판정 2엔트리) · certificates 분리 계상 · Z 원장 **33종**(즉시 29·④ 후 4) 실측.
- 사실 정정 7건(§1) — 인계 ①② 전제·distribution 원천·create 부적합·게이트 공허·**마커 총수 120(지시문 121과 불일치 — 불식식으로 자기검증)**·인용 2건.
- kind 어휘 = `Phase6ResourceKindExtensions` 4종(contract 슬라이스 선례 준수) · 비교 합류 = job·cronjob만(gateway 계열은 I-P5) · ④ = A′-1 변형(arch rule 2 정합).
- 집계/조립 = `inventory/k8sassembly.go` 신규 · P2 = `api/v2` 무변경 · 권한·라우트 신설 0(골든 불변) · pod 컨테이너 원시량 수집(milli/bytes 정수).
