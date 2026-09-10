# Kubernetes 매핑 — legacy `K8sClusterDetail` ↔ V2 정규화 (§15.2 권위 매핑 표)

> **위치 원칙**: 스펙 §15.2는 매핑 표 위치의 예시로 `inventory/k8s/mapping.md`를 든다. 본 표는
> §15.2 자신의 원칙 — *"lives next to the normalizer"* — 에 따라 `adapter/kubernetes/`
> normalizer(`normalizer.go`) 옆에 둔다. 편차는 계획 §2 각주(r2)에 명시됐다.
>
> **coverage rule (§15.2)**: *"the mapping table records, for every field the legacy
> API response serializes, the corresponding V2 normalized field and the transformation
> rule — or a documented reason the field is dropped."* 아래 §4 커버리지 표가 그 전필드
> 대응이며, `mapping_test.go`(PR 22, T51)가 이 문서와 Go 매핑 테이블의 동치를 기계 검증한다.

- 원천(legacy): `backend/model/k8s.go` `K8sClusterDetail` 9섹션 + `backend/service/k8s.go`
  `fetchK8sData` 직렬화 경로 (v1 읽기 전용 — §3 disposition row 11 SUPERSEDE, M2까지 v1 권위)
- 대상(V2): `internal/infra/adapter/kubernetes/normalizer.go` — `DiscoveredResource`
  (Kind·Subtype·ExternalID·ExternalURN·Raw·Normalized)
- 스펙: §8.1 신원·§8.2 관측·§14.1 K8s 매핑·§15.2 coverage rule·계획 §3.1/§3.2/§3.5

---

## 1. 신원 — URN 규칙 (§8.1 "enough native scope to avoid collisions")

URN 형식: `urn:k8s:{context}:{종}:{식별 성분}` — `{context}` = provider_context의
`ContextID`(10진).

| 종 | Subtype | URN 식별 성분 | ExternalID | 비고 |
|---|---|---|---|---|
| node | — | `{metadata.uid}` | uid | UID 신원 — name은 display 전용 |
| namespace | — | `{name}` | name | 클러스터 스코프 고유 |
| workload | deployment / statefulset / daemonset / replicaset / job / cronjob | `{namespace}/{k8s종}/{name}` | 동일 성분 | k8s종 = 소문자 subtype. replicaset·job·cronjob은 P 계획 J-P1-1(P1-A) 수집 확장 — job·cronjob 비교 합류는 P1-C2 착지(§3.2), replicaset은 스코프 외(I-P5) |
| pod | — | `{metadata.uid}` | uid | pod 이름은 재생성된다 — UID 신원, name은 display. 페어링 키(§3)는 name 정합 |
| service | service | `{namespace}/{name}` | 동일 성분 | |
| ingress | ingress | `{namespace}/{name}` | 동일 성분 | |
| endpoint | — | `{namespace}/{name}` | 동일 성분 | P1-A v2-only 보조종 — `network.endpoint`(Phase6ResourceKindExtensions), service.endpoints 집계 원천 |
| gateway | — | `{namespace}/{name}` | 동일 성분 | P1-C1 — `network.gateway`(Phase6ResourceKindExtensions), GatewayAPI Gateway(v1 선호·v1beta1 폴백 — J-P1-1). 비교 집합 불참(§3.2 — I-P5 이월) |
| httproute | — | `{namespace}/{name}` | 동일 성분 | P1-C1 — `network.http_route`, GatewayAPI HTTPRoute(동일 폴백·불참) |
| virtualservice | — | `{namespace}/{name}` | 동일 성분 | P2-D — `network.virtual_service`(Phase6ResourceKindExtensions), istio 오퍼레이션(`k8s.istio.traffic_update`) uid 앵커(J-P1-1). 앵커 최소형 — Raw metadata 신원만, Normalized 키 없음. 비교 집합 불참(§3.2 — I-P5 이월·R-P6) |
| configmap | — | `{namespace}/{name}` | 동일 성분 | 어휘 확장 2종(J9) |
| secret | — | `{namespace}/{name}` | 동일 성분 | 어휘 확장 2종(J9) — **metadata 전용** |
| pv | persistent_volume | `{name}` | name | |
| pvc | pvc | `{namespace}/{name}` | 동일 성분 | |
| storageclass | storage_class | `{name}` | name | V2 단독 종(§3 스코프 표) |

## 2. 단위 정규화 · Raw 상한

- 저장용량: Kubernetes quantity(Ki/Mi/Gi/Ti/바이트) → **GB(GiB 스케일), 소수 3자리**
  (`bytes / 1024³` — 예: `31457280Ki` → `30`). 코드: `quantityToGB`.
- CPU: `"8"` → `8`, `"7500m"` → `7.5` (코어, 소수 3자리). 코드: `quantityToCores`.
- Raw 크기 상한: `MaxRawBytes = 64 << 10` (64KiB — **계획 도입값, 스펙 §8.2는 수치 미정,
  가정 A12**). 초과 시 Raw는 `{"rawB64": <원본 JSON의 base64 접두>, "truncated": true}`로
  절단 보존 + `normalized["truncated"]=true` 마커. 코드: `applyRawLimit`.
- Raw는 항상 metadata 부분집합(name·namespace·uid·creationTimestamp·labels·
  ownerReferences의 uid/kind/name 3성분 — P1-A)만 담는다. secret·configmap의
  data **값**은 Raw에 절대 진입하지 않는다(§5 보존 제약 #7).
- **판정 ⑤ (b) 예외(사용자 승인 2026-09-10 — 보존 제약 #7의 유일 예외)**:
  `kube-system/kubeadm-config` 1종의 data 값 중 소스 YAML 키 `serviceSubnet`·
  `podSubnet` 2키(네트워크 위상 — 비밀 아님)만 파싱해
  `normalized.serviceCIDR`·`normalized.podSubnetCIDR`로 수집한다. 이 **2키
  한정**이며, 키 확장은 리뷰 승인을 전제로 한다(§6 봉인 문언·어댑터
  `kubeadmConfigValueKeys` 상수 — `TestKubeadmConfigWhitelist` 카나리).
  주석·따옴표 제거는 legacy `resolveK8sNetworkCIDRs`(k8s_overview.go:31)의
  수집측 승계다.

## 3. 비교 프로토콜 표 (§3.5 — T51 동치 검증 대상)

### 3.1 종별 페어링 키 (비교 엔진의 identity 대조 키)

legacy 직렬화에 `metadata.uid`가 전무(계획 §0.6 r2 실측)이므로, 비교 페어링 키는
**`namespace/name`(클러스터 스코프 종은 `name`)** 이다. V2의 UID 신원(§1)은 내부
신원 원칙이고 페어링 키와 분리된다.

| 종 | 페어링 키 | V2 URN 성분과의 관계 |
|---|---|---|
| node | `name` | URN은 `{uid}` — 매핑 키로 name ↔ V2 DisplayName 대조 |
| namespace | `name` | URN `{name}` — 일치 |
| workload(deployment/statefulset/daemonset/job/cronjob) | `{namespace}/{k8s종}/{name}` | URN과 동일 성분 — job·cronjob은 P1-C2 합류(P1-C2 pairingKeyShapes 착지) |
| pod | `{namespace}/{name}` | URN은 `{uid}` — 매핑 키로 name 대조(재생성 pod은 name 정합) |
| service/ingress/configmap/secret/pvc | `{namespace}/{name}` | URN과 동일 성분 |
| pv/storageclass | `{name}` | URN과 동일 성분 |

### 3.2 비교 스코프 표 — 양측 매핑 종·단독 종 처분

"매핑 표에 양측 매핑된 종·필드만 비교 집합" (계획 §3.5 r2). 단독 종은 count·identity
비교 집합에서 제외되며(집합 차이 BLOCKER의 오염원이 아님), coverage rule(§4)의 검증
대상에는 포함된다.

| 종 | legacy | V2 discoverer | 처분 |
|---|---|---|---|
| node·namespace·pod·deployment·statefulset·daemonset·service·ingress·configmap·secret·pv·pvc | 수집 | 수집 | **비교 집합** — §4 coverage rule 적용 |
| ReplicaSet | 수집(Deployment 파생 자동 등장) | 수집(P1-A) | 스코프 외 — 비교 승격 **이관 안 함**, I-P5 재판정까지 보류. 양측 수집이나 legacy측 등장은 Deployment 파생이라 독립 신원 부재 → 처분 `v2-only`(P1-A 수집 착지에 따른 P1-F 처분 정정. P1-C2는 job·cronjob만 합류 — §9-10 단일 착지점) |
| Job·CronJob | 수집(Workloads 목록 — Type 문자열 실음, main_compare.go:256 실측) | 수집(P1-A) | **비교 집합(P1-C2 착지 — J-P1-2)** — 페어링 키 `{namespace}/{k8s종}/{name}`. job 필드는 buildWorkloadItems Ready 동치 유도(엔진이 succeeded/completions에서 분모·분자 재구성), cronjob은 Ready가 텍스트(cronJobReadyText)라 신원 비교만. 필드 수준 v2-only 키(schedule 등)는 §4.6 참조 |
| endpoints | 미수집(legacy 직렬화에 종 없음) | 수집(P1-A) | 스코프 외 — v2-only 보조종(`network.endpoint`), service.endpoints 집계 원천. 비교 합류는 I-P5 이월 |
| Gateway·HTTPRoute | 수집(advancedNetwork 섹션 — `K8sIstioResourceItem`) | 수집(P1-C1 — `network.gateway`·`network.http_route`, v1→v1beta1 폴백·CRD 부재 시 섹션 스킵) | 스코프 외 — legacy 캡처(LegacyCapture)에 gatewayApiGateways·httpRoutes 섹션 부재(main_compare.go 실측 — J-P1-2 H4)라 합류 시 V2 단독 종·identity-sets-differ BLOCKER. 비교 합류는 I-P5 이월. normalized 키는 §4.8 전환 참조 |
| storageclass | 미수집(속성 필드로만 존재) | 수집 | 스코프 외 — V2 단독 종 `v2-only`, 비교 대상 필드 없음 |

P 계획(J-P1-1)에 따라 P1-A에서 V2 수집을 replicaset·job·cronjob·endpoints로
확장했다 — 비교 엔진(`compare.go`)의 스코프 처분 변경은 P1-C2에서 단일 착지하며
그 전까지 양측 스코프 외 종은 비교 집합에 오염되지 않는다.

## 4. 커버리지 표 — legacy 직렬화 전필드 (coverage rule)

처분 어휘: **mapped** = V2 정규화 필드로 대응 · **dropped(사유)** = 미이관 ·
**v2-only** = V2 단독 필드(비교 집합 외 또는 종 단독) · **pending(사유)** = 도메인
판정 대기 보류(현재 1건 — certificates ④ 보안판정, 결착 전 커밋 부재가 G-P1f).

### 4.1 `cluster` 섹션 (`K8sClusterView`) — 클러스터 자체는 리소스 행이 아니라
connection/context 속성이다(백필 §3.4 매핑의 원천).

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| id | dropped(v2-source-key) | 백필 `source_id`(provider_connection) — 리소스 아님 |
| name | dropped(v2-source-key) | ProviderConnection.Name |
| status / statusText | **mapped(조립·P1-D)** | Connection.Status + 표시 어휘(Running/Partial Alerts/Offline — `StatusText`), AlertCount>0이면 warning 승격(v1 detail 계약) |
| apiServer | dropped(v2-source-key) | ProviderConnection.Endpoint |
| version | **mapped** | Health/Validate의 `/version` GitVersion — distribution 판정 입력 |
| nodeCount | **mapped(조립·P1-D)** | Connection.ConfigJSON `node_count`(백필 승계), 부재 시 node 행 수 폴백 — distribution "N nodes". v1 표시 계약(TestCharBuildOverviewDistribution NodeCount=3 vs 노드 1행)이 저장 속성을 고정한다(P1-D 판단 기록) |
| env / tags / description | dropped(v2-source-key) | ProviderConnection.ConfigJSON |
| connectionMode / gatewayId / gatewayName | **mapped(조립·P1-D)** | Config `connection_mode`(정규화)·Connection.GatewayID + gateway 이름 읽기 조인(조립) |
| monitorDatasourceId / monitorDatasourceName | **mapped(판정③ — 조립 P1-D)** | Id는 등록 경로가 ConfigJSON `monitor_datasource_id` 흡수, Name은 조립이 monitor_datasource 읽기 조인(개체 소유는 모니터링 도메인 — REMAIN 정합) |
| lastSyncAt / createTime / updateTime | dropped(volatile) | §15.3 VOLATILE — 타임스탬프 비교 집합 외(조립의 LastSyncAt는 영값 — P1-D 판단 기록) |

### 4.2 `overview` 섹션 (`K8sOverview`) — 판정 ①② (가) 확정(사용자 승인
2026-09-10): 전 필드 V2 인벤토리 집계로 흡수한다(§1 실측 — 모니터링 도메인이
아니라 인벤토리 파생). 집계는 조립 P1-D(`inventory/k8sassembly.go`)가 v1 산식
오라클과 동치로 소유한다.

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| healthScore | **mapped(집계·P1-D)** | `CalculateHealthScore(alertCount)` — 100−8n·하한 40(v1 `calculateHealthScore` 동치) |
| cpuUsage / memoryUsage | **mapped(집계·P1-D)** | pod 컨테이너 requests 합 ÷ node allocatable 합(`FormatUsagePercent` — GB 3소수 저장의 역변환은 round로 정확 복원) |
| podUsage / requestRate | **mapped(집계·P1-D)** | pod 행 수 "N Pods"·워크로드(5종) 행 수 "N Workloads" |
| alertCount | **mapped(집계·P1-D)** | not-Ready(`readyCondition`≠True)·`unschedulable` 노드 + failed/pending/unknown pod(v1 `calculateK8sAggregateMetrics` 동치) |
| distribution[] | **mapped(조립·판정⑤)** | 5행 고정 라벨 — Service CIDR은 kubeadm-config 2키(§2 예외), Pod Network은 node `podCIDRs` 폴백(정렬·"、"), 부재는 Unknown(추측 금지) |
| certificates[] | **pending(④-보안판정)** | 인증서 만료 관측 — ④ 결착 대기로 분리 계상(선행조건: 결착 전 착수·커밋 금지 G-P1f — P1-F 전환, 착수는 P1-G1·G2) |

### 4.3 `nodes` 섹션 (`K8sNodeItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name | **mapped** | DisplayName + 페어링 키(§3.1) |
| role | **mapped** | `normalized.roles` — 수집측이 v1 어휘를 확정한다(`node-role.kubernetes.io/*` 접미, 빈 접미 키→"worker", 역할 부재→["worker"], 정렬·중복 허용 — TestCharJoinNodeRoles 계약). 조립은 "," 결합만(`JoinNodeRoles`) |
| status | **mapped** | `normalized.healthState` (conditions Ready=True→healthy, 그 외 degraded). 표시 3치(Ready/NotReady/**Unknown**)는 조립이 `normalized.readyCondition`(True/False/부재 — P1-D 조립 원천)으로 복원한다 — healthState 2치로는 v1 "조건 없음 → Unknown" 계약(TestCharNodeReadyStatus)이 재현 불가라 상태 원천을 별도 보존(P1-D 판단 기록) |
| version | **mapped** | `normalized.kubeletVersion` (status.nodeInfo.kubeletVersion — 부재 시 키 생략) |
| internalIP | **mapped** | `normalized.internalIP` (status.addresses 중 type=InternalIP 첫 주소 — 부재 시 키 생략, legacy "-" 포맷은 조립 P1-D 소유) |
| (node) podCIDRs | **mapped** | `normalized.podCIDRs` (spec.podCIDRs + 단일 spec.podCIDR 폴백 — v2-only 필드, P1-A 수집) |
| os | **mapped** | `normalized.osImage` (status.nodeInfo.osImage — 부재 시 키 생략) |
| cpu | **mapped** | `normalized.allocatableCoresGB` (v1 표시 원천은 **allocatable** — 구 표의 capacityCoresGB 표기 정정). 표시는 GB 역변환 compact 문자열("4") — 원문 m 접미("4000m")는 GB에서 복원 불가인 왕복 한계(§2 단위 규칙 고유) |
| memory | **mapped** | `normalized.allocatableMemoryGB` (v1 표시 원천은 **allocatable** — Ki→GB). 표시는 bytes 역변환(round) 후 10⁶ MB 반올림("8590 MB") |
| pods | **mapped(집계·P1-D)** | pod `nodeName`별 행 수 / 분모 `capacityPods`(Status.Capacity["pods"] — allocatablePods와 다른 원천, TestCharBuildNodeItems "2/110") |

V2-only: `capacityMemoryGB` 외 `allocatableCoresGB`·`allocatableMemoryGB`·`allocatablePods`
(allocatable["pods"] 개수 정수 — P1-B)·`podCIDRs`·`taints`·`healthState`·
`readyCondition`·`unschedulable`·`capacityPods`(P1-D 조립 원천) (Raw의 uid·labels·
creationTimestamp 포함 — 비교 집합은 §3.2 규칙 따름).

### 4.4 `namespaces` 섹션 (`K8sNamespaceItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name | **mapped** | DisplayName + 페어링 키 |
| status | **mapped** | `normalized.phase` (Active/Terminating) |
| pods / services / workloads | **mapped(집계·P1-D)** | 종별 행 수 파생 — pods·service 행과 워크로드 5종 행(deployment/statefulset/daemonset/job/cronjob; replicaset 제외 — legacy도 5종만 셈)을 namespace별 계수 |
| createdAt | **mapped** | Raw `creationTimestamp` (VOLATILE — §15.3). 표시 "2006-01-02 15:04" 포맷은 조립 P1-D 소유 |

### 4.5 `pods` 섹션 (`K8sPodItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name / namespace | **mapped** | DisplayName + 페어링 키 `{namespace}/{name}` (URN은 uid — §1) |
| workloadName / workloadType | **mapped(집계·P1-D)** | ownerReferences 역추적 체인(RS→Deployment·Job→CronJob·직접 소유자 + 셀렉터 폴백 + 이름 접두 최장 매칭 — v1 `buildPodItemsWithWorkloads` 동치). 원천은 Raw `ownerReferences`(P1-A)·워크로드/Job `normalized.selector`(P1-D 조립 원천), 조립 P1-D |
| status | **mapped** | `normalized.phase` 그대로(상태 어휘: pod phase) + `normalized.lifecycleState` 동일값 |
| node | **mapped(relationship)** | pod→node `runs_on` 관계 — PR 21 relationship 적재 (리소스 필드 아님). 표시·노드별 파드 카운트의 원천은 `normalized.nodeName`(P1-D 조립 원천 — 조립은 관측 행만으로 순수하게 돈다, R-P8) |
| nodeIP / ip | **mapped** | `normalized.hostIP` / `normalized.podIP` (status.hostIP·status.podIP — 부재 시 키 생략) |
| restarts | **mapped** | `normalized.restartCount` (containerStatuses 합계 — VOLATILE) |
| age | **mapped(조립·P1-D)** | Raw `creationTimestamp` → `HumanizeAge`(RFC3339 → Just now/m/h/d/날짜 — §15.3 VOLATILE 비결정 구간은 비교 집합 밖) |

### 4.6 `workloads` 섹션 (`K8sWorkloadItem` — deploy/statefulset/daemonset + P1-A 확장 replicaset/job/cronjob 동일 아이템 형면)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name / namespace / type | **mapped** | DisplayName + Subtype(deployment/statefulset/daemonset) + 페어링 키 |
| ready | **mapped** | `normalized.readyReplicas` ("1/3" 형식 → 수치 분리: legacy는 `Ready` 문자열, V2는 수치 — 변환 규칙: 분모=replicas·분자=readyReplicas) |
| updated / available | **mapped** | `normalized.updatedReplicas` / `normalized.availableReplicas` (status — apps 계열. **daemonset만 원천 필드가 다르다**: UpdatedNumberScheduled·NumberAvailable을 동일 키로 수집 — legacy `buildWorkloadItems` 동치. batch 종은 구조적 부재로 apps 키 생략 — legacy는 job의 active·succeeded에서 파생, cronjob은 active 카운트·suspend 표시. 유도는 조립 P1-D) |
| age | **mapped(조립·P1-D)** | Raw `creationTimestamp` → `HumanizeAge` |
| requests / limits | **mapped** | `normalized.containers`[{name, image, requests{cpuMilli, memBytes}, limits{…}}] — **양은 milli·bytes 정수로 저장, legacy "500m / 1.0Gi" 포맷은 조립 P1-D의 포맷터 오라클**(`formatWorkloadResourceSummary`·`formatCPUMilli`·`formatMemoryBytes`) |

**batch 고유 키는 v2-only**(legacy 직렬화에 부재 — job: `completions`·`parallelism`·`active`·
`succeeded`·`failed`·`selector`, cronjob: `schedule`·`active`(JobReference 배열 카운트)·
`suspend`·jobTemplate 경계의 completions·parallelism·containers. 구조적 부재는 키 생략으로
구분 — P1-B 수집, `selector`·`suspend`는 P1-D 조립 원천(셀렉터 폴백 매칭·cronJobReadyText
"Suspended" 분기). Z 테이블 최소면은 buildWorkloadItems 동치). **P1-C2 비교 합류 후 필드
유도 규칙**: job의 ready는 legacy `succeeded/total`(total = completions, 0이면
active+succeeded+failed)이므로 비교 엔진이 normalized 상태 키에서 이를 재구성한다 —
normalized의 `replicas`·`readyReplicas`는 batch 종에서 구조적 영값이라 원천이 아니다.
cronjob은 legacy Ready가 텍스트(cronJobReadyText — "Scheduled"/"Suspended"/"N Active")라
양측 모두 수치 필드 없이 신원 비교만 한다.

**`workload.image`는 v2-only**: legacy **리스트** 직렬화(`K8sWorkloadItem`)에 image 필드가
없다(계획 §0.6 r2 실측 — §15.3 BLOCKER 예시의 "image tag"는 상세 DTO
`K8sWorkloadDetail.Containers[].Image` 기준 예시로, 페어 비교 대상(9섹션 리스트)에서는
비교 불가한 죽은 예시). V2는 `spec.template.spec.containers[0].image`를
`normalized.image`로 수집하되 **비교 집합 외**(스코프 규칙 §3.2)로 둔다.

### 4.7 `network` 섹션 (`K8sNetworkSection.services` = `K8sServiceItem`,
`.ingresses` = `K8sIngressItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name / namespace | **mapped** | DisplayName + 페어링 키 |
| type | **mapped** | `normalized.type` (Subtype "service") |
| clusterIP | **mapped** | `normalized.clusterIP` (None이면 생략) |
| ports | **mapped** | `normalized.ports` (port·protocol·name·nodePort 목록 — nodePort는 P1-D 조립 원천: legacy "80:30080/TCP" 표시의 원천이며 비교 엔진은 port/proto만 소비(compare `canonicalV2Ports`)해 무영향) |
| externalIP | **mapped** | `normalized.externalIP` (spec.externalIPs + status.loadBalancer.ingress — IP 우선·hostname 대체, ", " 결합. legacy `serviceExternalIP` 동치 — 공백 시 키 생략, "<none>" 포맷은 조립 P1-D) |
| endpoints | **mapped(집계·P1-D)** | `network.endpoint` 보조종의 `normalized.readyAddresses`를 "ns/name" 키로 대조(v1 `buildEndpointCounts` 동치 — 보조종 수집은 P1-A) |
| age | **mapped(조립·P1-D)** | Raw `creationTimestamp` → `HumanizeAge` |
| (ingress) host | **mapped** | `normalized.hosts` (rules[].host 목록) — v2-only 필드. 표시 ", " 결합은 조립 P1-D |
| (ingress) address | **mapped** | `normalized.address` (status.loadBalancer.ingress[0] IP 우선 — 부재 시 키 생략, legacy "-" 포맷은 조립 P1-D) |
| (ingress) tls | **mapped** | `normalized.tls` (spec.tls 유무 → "Enabled"/"Disabled" — legacy 상태 어휘 그대로) |
| (ingress) age | **mapped(조립·P1-D)** | Raw `creationTimestamp` → `HumanizeAge` |

### 4.8 `advancedNetwork` 섹션 (`K8sAdvancedNetworkSection` — GatewayAPI/Istio)

P1-C1에서 GatewayAPI 2종(`network.gateway`·`network.http_route`)의 V2 수집이 착지했다 —
P1-C2 단일 착지점에서 커버리지 처분을 `mapped`로 전환한다. 비교 집합 불참은 §3.2 스코프 표의
소관(legacy 캡처에 섹션 부재 — I-P5 이월)이고, 본 표는 필드 수준 대응만 담는다.

| legacy 필드 (`K8sIstioResourceItem`) | 처분 | V2 대응 |
|---|---|---|
| gatewayApiGateways[] | **mapped** | `network.gateway` 종 — `normalized.gatewayClassName`·`hosts`·`addresses`·`ports`(J-P1-3). item.hosts→hosts, item.ports→ports, item.address→addresses(`JoinAndLimit` 포맷·서비스 폴백은 조립 P1-D — `resolveGatewayAPIAddress` 동치) |
| httpRoutes[] | **mapped** | `network.http_route` 종 — `normalized.parents`·`targets`·`hostnames`(J-P1-3 + P1-D 조립 원천 확장). item.gateways→parents, item.target→targets, item.hosts→hostnames(`JoinAndLimit` 포맷은 조립 P1-D — legacy `collectHTTPRouteParents/Targets` 동치) |
| (item) age | **mapped(조립·P1-D)** | Raw `creationTimestamp` → `HumanizeAge` |
| (item) name / namespace / kind | **mapped** | DisplayName + Raw namespace + 종 판정(`network.gateway`·`network.http_route`) |

hostnames는 J-P1-3의 parents·targets 2키에 대한 **P1-D 조립 원천 확장**이다(판단 기록
P1-D-2): legacy `K8sIstioResourceItem.Hosts = joinAndLimit(spec.hostnames, 3)`가 v1 특성
표로 고정돼 있어 2키만으로는 조립 동치가 불성립한다.

비교 집합 불참 사유(§3.2): legacy 캡처(`LegacyCapture`)에는 gatewayApiGateways·httpRoutes
섹션이 없다(main_compare.go 실측 — J-P1-2 H4). 합류는 I-P5(LegacyCapture 섹션 확장 + GatewayAPI
CRD 실클러스터 설치와 함께)로 이월된다.

### 4.9 `configStorage` 섹션 (`K8sConfigStorageSection`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| configMaps[].name / namespace | **mapped** | DisplayName + 페어링 키 |
| configMaps[].keys | **mapped** | `normalized.dataKeys` (개수 카운트 → 키 이름 목록; Data+binaryData 키 합산 — legacy Keys=len(Data)+len(Binary) 동치. **data 값 미수집** — §2 판정 ⑤ 예외는 kubeadm-config 2키 한정) |
| secrets[].name / namespace | **mapped** | DisplayName + 페어링 키 |
| secrets[].type | **mapped** | `normalized.type` |
| secrets[].keys | **mapped** | `normalized.dataKeys` (키 이름만 — **§14.1 "secret metadata", 데이터 값 절대 미수집**) |
| storage[].name / kind / namespace | **mapped** | pv→`persistent_volume`·pvc→`pvc` Subtype + 페어링 키 |
| storage[].namespaceScope | **mapped** | `normalized.namespaceScope` — pv: annotation `ops-admin.io/namespace-scope`, 기본 "Cluster-scoped"(legacy `storageNamespaceScope` 동치). pvc 행은 legacy 공란 — pvc는 키 생략(행 형상은 조립 P1-D 소유) |
| storage[].status | **mapped** | `normalized.phase` (pv/pvc status.phase) |
| storage[].capacity | **mapped** | `normalized.capacityGB` + `normalized.capacityRaw`(P1-D 조립 원천 — legacy 표시 원문량 문자열 "5Gi"/"10Gi": pvc는 **requests 우선**, pv는 status 우선. GB 3소수는 원문량을 복원하지 못하는 왕복 한계의 보완 — 판단 기록 P1-D-3. capacityGB의 pvc 우선순위도 requests 우선으로 정정해 표시·비교 원천을 일치) |
| storage[].storageClass | **mapped** | `normalized.storageClassName` |
| storage[].sourceType / path / nfsServer | **mapped** | `normalized.sourceType`·`normalized.sourcePath`·`normalized.nfsServer` — spec.persistentVolumeSource(hostPath→"hostPath"+path, nfs→"NFS"+path+server, 그 외 키 생략). legacy `persistentVolumeSource` 동치(pv 한정 — pvc는 소스 없음). "-" 포맷은 조립 P1-D |
| storage[].accessModes | **mapped** | `normalized.accessModes` (표시 ", " 결합은 조립 P1-D) |
| storage[].reclaimPolicy | **mapped** | `normalized.reclaimPolicy` (spec.persistentVolumeReclaimPolicy — pv 한정, 부재 시 키 생략) |
| configMaps[].age / secrets[].age / storage 상세(YAML 등) | **mapped(조립·P1-D)** | age는 Raw `creationTimestamp` → `HumanizeAge`. 상세(YAML 등)는 상세 DTO 소관 — 본 표(리스트 직렬화) 밖 |

**storageclass는 v2-only 종**: legacy는 storageclasses 객체 목록을 수집하지 않는다
(`K8sStorageItem.StorageClass`는 PV/PVC 행의 속성 필드일 뿐 — 계획 §0.6 r2 실측). V2는
`storage.pool` Subtype `storage_class`로 수집하며 `normalized.provisioner`는 v2-only
필드. 비교 대상 필드 없음(§3.2 스코프 표).

## 5. 상태 어휘 (§8.2 lifecycle_state·health_state — 1페이지 요약)

| 대상 | 규칙 | 값 |
|---|---|---|
| node `healthState` | conditions `Ready` | `True`→`healthy`, 그 외(부재 포함)→`degraded` |
| pod `lifecycleState` | `status.phase` 그대로 | `Running`·`Pending`·`Succeeded`·`Failed`·`Unknown` |
| pod `phase` | `status.phase` 그대로 | 동일 어휘 (비교는 VOLATILE 아님 — §15.3 count/status 규칙 따름) |
| namespace `phase` | `status.phase` 그대로 | `Active`·`Terminating` |

## 6. 자격·보안 경계 (보존 제약 #7)

- kubeconfig는 `DiscoverRequest.Connection.Material["inventory"]`(브로커 purpose
  `inventory` Resolve 산물)로만 진입 — 로그·에러·Raw·Normalized·아티팩트 어디에도
  노출 금지. 에러 메시지는 신호 종류+상태 코드만 담는다.
- secret은 metadata(type + data **키 이름 목록**)만, configmap은 data **키 이름 목록**만
  정규화에 진입한다 — 값은 어댑터 경계에서 폐기. **유일한 예외는 판정 ⑤ (b)의
  kubeadm-config 2키다**(§2 — `serviceSubnet`·`podSubnet`, 사용자 승인
  2026-09-10): 이 **2키 한정**이며, 어떤 추가 키·값도 리뷰 승인 없이는 수집하지
  않는다. 어댑터는 화이트리스트 상수(`kubeadmConfigValueKeys`)로만 접근하고
  `TestKubeadmConfigWhitelist` 카나리가 화이트리스트 밖 값의 잔류를 기계 판정한다.
- 게이트웨이 모드는 `Connection.Config{"connection_mode","gateway_id"}` + 주입 dialer로만
  구성(A4) — Phase 2 게이트 증명은 직접 연결, 실환경 홉은 M2.

## 7. 쓰기 오퍼레이션 등재처 — 종결 표기 (P2-E)

본 표는 **읽기 패리티**(legacy `K8sClusterDetail` 직렬화 ↔ V2 정규화)의 권위 표다 —
coverage rule(§15.2)도 응답 직렬화 필드를 대상으로 한다. 쓰기 오퍼레이션 10종
(`k8s.workload.restart`·`scale`·`image_update`·`resources_update`·
`k8s.node.labels_update`·`k8s.service.update`·`k8s.resource.apply`(update 한정)·
`k8s.resource.delete`·`k8s.istio.traffic_update`·`k8s.httproute.traffic_update`)의
등재처는 본 표가 아니라 **opdef 코드**(descriptors are code — `compose_k8s_ops.go`
opdef 변수 + `compose.go` `restartOperation`)와 그 1:1 잠금 테스트
`contracttest/opdef_table_test.go`(`TestOperationDefTable` — 권한·risk·승인·
capability·ResourceKinds·IdempotencyPolicy 전칸럼)다(J-P1-7 재해석 확정 — plan·execute는
파라미터 라우트라 sensitive-routes 골든에 10행 실기가 물리적 불가, 계획 §10 미해결 5의 확정 판정).

- plan·execute 파라미터 라우트 2개는 골든(`docs/security/sensitive-routes.txt:157-158`)
  에 대표 권한(`assets:k8s:workload:restart`)으로 실리고, 요청별 권한 분해는
  `opdef.V2DynamicMiddleware` + `ResolveOperationPermission`(`routes_v2.go:33-34`)이
  집행한다. 대표행 ↔ restart def 정합은 `TestOperationDefTable`이 단얫한다.
- 골든 292·452 불변은 유지 원칙 — 오퍼레이션 추가는 골든이 아니라 opdef 코드·표 테스트
  쪽 착지다(신규 권한 문자열 0 — 보존 제약 #6 유지).
