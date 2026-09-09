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
| ReplicaSet | 수집(Deployment 파생 자동 등장) | 수집(P1-A) | 스코프 외 — V2 단독 종 `v2-only`. 구 처분 `dropped(v2-not-collected)`는 비교 엔진이 I-P5 재판정까지 유지(P1-C2는 job·cronjob만 합류 — §9-10 단일 착지점) |
| Job·CronJob | 수집(Workloads 목록 — Type 문자열 실음, main_compare.go:256 실측) | 수집(P1-A) | **비교 집합(P1-C2 착지 — J-P1-2)** — 페어링 키 `{namespace}/{k8s종}/{name}`. job 필드는 buildWorkloadItems Ready 동치 유도(엔진이 succeeded/completions에서 분모·분자 재구성), cronjob은 Ready가 텍스트(cronJobReadyText)라 신원 비교만. 필드 수준 v2-only 키(schedule 등)는 §4.6 참조 |
| endpoints | 미수집(legacy 직렬화에 종 없음) | 수집(P1-A) | 스코프 외 — v2-only 보조종(`network.endpoint`), service.endpoints 집계 원천. 비교 합류는 I-P5 이월 |
| Gateway·HTTPRoute | 수집(advancedNetwork 섹션 — `K8sIstioResourceItem`) | 수집(P1-C1 — `network.gateway`·`network.http_route`, v1→v1beta1 폴백·CRD 부재 시 섹션 스킵) | 스코프 외 — legacy 캡처(LegacyCapture)에 gatewayApiGateways·httpRoutes 섹션 부재(main_compare.go 실측 — J-P1-2 H4)라 합류 시 V2 단독 종·identity-sets-differ BLOCKER. 비교 합류는 I-P5 이월. normalized 키는 §4.8 전환 참조 |
| storageclass | 미수집(속성 필드로만 존재) | 수집 | 스코프 외 — V2 단독 종 `v2-only`, 비교 대상 필드 없음 |

P 계획(J-P1-1)에 따라 P1-A에서 V2 수집을 replicaset·job·cronjob·endpoints로
확장했다 — 비교 엔진(`compare.go`)의 스코프 처분 변경은 P1-C2에서 단일 착지하며
그 전까지 양측 스코프 외 종은 비교 집합에 오염되지 않는다.

## 4. 커버리지 표 — legacy 직렬화 전필드 (coverage rule)

처분 어휘: **mapped** = V2 정규화 필드로 대응 · **dropped(사유)** = 미이관 ·
**v2-only** = V2 단독 필드(비교 집합 외 또는 종 단독).

### 4.1 `cluster` 섹션 (`K8sClusterView`) — 클러스터 자체는 리소스 행이 아니라
connection/context 속성이다(백필 §3.4 매핑의 원천).

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| id | dropped(v2-source-key) | 백필 `source_id`(provider_connection) — 리소스 아님 |
| name | dropped(v2-source-key) | ProviderConnection.Name |
| status / statusText | dropped(v2-source-key) | ProviderConnection 상태 관리로 |
| apiServer | dropped(v2-source-key) | ProviderConnection.Endpoint |
| version | **mapped** | Health/Validate의 `/version` GitVersion — distribution 판정 입력 |
| nodeCount | dropped(v2-derived) | V2는 node 리소스 행 수에서 파생(집계는 read side) |
| env / tags / description | dropped(v2-source-key) | ProviderConnection.ConfigJSON |
| connectionMode / gatewayId / gatewayName | dropped(v2-source-key) | Connection.Config `connection_mode`·`gateway_id` |
| monitorDatasourceId / monitorDatasourceName | dropped(monitoring-domain) | 모니터링 도메인 — V2 인벤토리 밖 |
| lastSyncAt / createTime / updateTime | dropped(volatile) | §15.3 VOLATILE — 타임스탬프 비교 집합 외 |

### 4.2 `overview` 섹션 (`K8sOverview`) — 전부 모니터링·메트릭 파생.

| legacy 필드 | 처분 | 사유 |
|---|---|---|
| healthScore | dropped(monitoring-derived) | 메트릭 스코어 — 인벤토리 아님 |
| cpuUsage / memoryUsage / podUsage / requestRate | dropped(monitoring-derived) | 실시간 사용률 — §18.2 계측 도메인 |
| alertCount | dropped(monitoring-derived) | 알림 도메인 |
| distribution[] | dropped(v2-schema-absent) | 리소스 분포 통계 — read side 집계로 대체 가능 |
| certificates[] | dropped(v2-schema-absent) | 인증서 만료 관측 — Phase 3+ 후보(§13 재검토 여지) |

### 4.3 `nodes` 섹션 (`K8sNodeItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name | **mapped** | DisplayName + 페어링 키(§3.1) |
| role | **mapped** | `normalized.roles` (`node-role.kubernetes.io/*` 라벨 파생) |
| status | **mapped** | `normalized.healthState` (conditions Ready=True→healthy, 그 외 degraded) |
| version | **mapped** | `normalized.kubeletVersion` (status.nodeInfo.kubeletVersion — 부재 시 키 생략) |
| internalIP | **mapped** | `normalized.internalIP` (status.addresses 중 type=InternalIP 첫 주소 — 부재 시 키 생략, legacy "-" 포맷은 조립 P1-D 소유) |
| (node) podCIDRs | **mapped** | `normalized.podCIDRs` (spec.podCIDRs + 단일 spec.podCIDR 폴백 — v2-only 필드, P1-A 수집) |
| os | **mapped** | `normalized.osImage` (status.nodeInfo.osImage — 부재 시 키 생략) |
| cpu | **mapped** | `normalized.capacityCoresGB` (quantityToCores) |
| memory | **mapped** | `normalized.capacityMemoryGB` (Ki→GB) |
| pods | dropped(v2-derived) | 파생 카운트 — read side 집계 |

V2-only: `capacityMemoryGB` 외 `allocatableCoresGB`·`allocatableMemoryGB`·`allocatablePods`
(allocatable["pods"] 개수 정수 — P1-B)·`podCIDRs`·`taints`·`healthState` (Raw의 uid·labels·
creationTimestamp 포함 — 비교 집합은 §3.2 규칙 따름).

### 4.4 `namespaces` 섹션 (`K8sNamespaceItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name | **mapped** | DisplayName + 페어링 키 |
| status | **mapped** | `normalized.phase` (Active/Terminating) |
| pods / services / workloads | dropped(v2-derived) | 종별 리소스 행에서 파생 |
| createdAt | **mapped** | Raw `creationTimestamp` (VOLATILE — §15.3) |

### 4.5 `pods` 섹션 (`K8sPodItem`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name / namespace | **mapped** | DisplayName + 페어링 키 `{namespace}/{name}` (URN은 uid — §1) |
| workloadName / workloadType | dropped(v2-derived) | ownerReferences 파생 — 원천 Raw는 P1-A 수집(`Raw.ownerReferences` uid/kind/name), 집계는 P1-D(관계 데이터 도메인) |
| status | **mapped** | `normalized.phase` 그대로(상태 어휘: pod phase) + `normalized.lifecycleState` 동일값 |
| node | **mapped(relationship)** | pod→node `runs_on` 관계 — PR 21 relationship 적재 (리소스 필드 아님) |
| nodeIP / ip | **mapped** | `normalized.hostIP` / `normalized.podIP` (status.hostIP·status.podIP — 부재 시 키 생략) |
| restarts | **mapped** | `normalized.restartCount` (containerStatuses 합계 — VOLATILE) |
| age | dropped(volatile) | §15.3 VOLATILE |

### 4.6 `workloads` 섹션 (`K8sWorkloadItem` — deploy/statefulset/daemonset + P1-A 확장 replicaset/job/cronjob 동일 아이템 형면)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| name / namespace / type | **mapped** | DisplayName + Subtype(deployment/statefulset/daemonset) + 페어링 키 |
| ready | **mapped** | `normalized.readyReplicas` ("1/3" 형식 → 수치 분리: legacy는 `Ready` 문자열, V2는 수치 — 변환 규칙: 분모=replicas·분자=readyReplicas) |
| updated / available | **mapped** | `normalized.updatedReplicas` / `normalized.availableReplicas` (status — apps 계열 전용. batch 종은 구조적 부재로 키 생략 — legacy는 job의 active·succeeded에서 파생, 조립 P1-D) |
| age | dropped(volatile) | |
| requests / limits | **mapped** | `normalized.containers`[{name, image, requests{cpuMilli, memBytes}, limits{…}}] — **양은 milli·bytes 정수로 저장, legacy "500m / 1.0Gi" 포맷은 조립 P1-D의 포맷터 오라클**(`formatWorkloadResourceSummary`·`formatCPUMilli`·`formatMemoryBytes`) |

**batch 고유 키는 v2-only**(legacy 직렬화에 부재 — job: `completions`·`parallelism`·`active`·
`succeeded`·`failed`, cronjob: `schedule`·`active`(JobReference 배열 카운트)·jobTemplate 경계의
completions·parallelism·containers. 구조적 부재는 키 생략으로 구분 — P1-B 수집, Z 테이블
최소면은 buildWorkloadItems 동치). **P1-C2 비교 합류 후 필드 유도 규칙**: job의 ready는
legacy `succeeded/total`(total = completions, 0이면 active+succeeded+failed)이므로 비교 엔진이
normalized 상태 키에서 이를 재구성한다 — normalized의 `replicas`·`readyReplicas`는 batch 종에서
구조적 영값이라 원천이 아니다. cronjob은 legacy Ready가 텍스트(cronJobReadyText —
"Scheduled"/"Suspended"/"N Active")라 양측 모두 수치 필드 없이 신원 비교만 한다.

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
| ports | **mapped** | `normalized.ports` (port·protocol·name 목록) |
| externalIP | **mapped** | `normalized.externalIP` (spec.externalIPs + status.loadBalancer.ingress — IP 우선·hostname 대체, ", " 결합. legacy `serviceExternalIP` 동치 — 공백 시 키 생략, "<none>" 포맷은 조립 P1-D) |
| endpoints | dropped(v2-derived) | endpoints 객체 파생 카운트 — 보조종 수집은 P1-A 착수(`network.endpoint` `normalized.readyAddresses`), 집계는 P1-D |
| age | dropped(volatile) | |
| (ingress) host | **mapped** | `normalized.hosts` (rules[].host 목록) — v2-only 필드 |
| (ingress) address | **mapped** | `normalized.address` (status.loadBalancer.ingress[0] IP 우선 — 부재 시 키 생략, legacy "-" 포맷은 조립 P1-D) |
| (ingress) tls | **mapped** | `normalized.tls` (spec.tls 유무 → "Enabled"/"Disabled" — legacy 상태 어휘 그대로) |
| (ingress) age | dropped(volatile) | |

### 4.8 `advancedNetwork` 섹션 (`K8sAdvancedNetworkSection` — GatewayAPI/Istio)

P1-C1에서 GatewayAPI 2종(`network.gateway`·`network.http_route`)의 V2 수집이 착지했다 —
P1-C2 단일 착지점에서 커버리지 처분을 `mapped`로 전환한다. 비교 집합 불참은 §3.2 스코프 표의
소관(legacy 캡처에 섹션 부재 — I-P5 이월)이고, 본 표는 필드 수준 대응만 담는다.

| legacy 필드 (`K8sIstioResourceItem`) | 처분 | V2 대응 |
|---|---|---|
| gatewayApiGateways[] | **mapped** | `network.gateway` 종 — `normalized.gatewayClassName`·`hosts`·`addresses`·`ports`(J-P1-3). item.hosts→hosts, item.address→addresses, item.ports→ports 유도(legacy `collectGatewayAPIHosts/Ports/Addresses` 동치) |
| httpRoutes[] | **mapped** | `network.http_route` 종 — `normalized.parents`·`targets`(J-P1-3). item.gateways→parents, item.target→targets 유도(legacy `collectHTTPRouteParents/Targets` 동치) |
| (item) age | dropped(volatile) | §15.3 VOLATILE |
| (item) name / namespace / kind | **mapped** | DisplayName + Raw namespace + 종 판정(`network.gateway`·`network.http_route`) |

비교 집합 불참 사유(§3.2): legacy 캡처(`LegacyCapture`)에는 gatewayApiGateways·httpRoutes
섹션이 없다(main_compare.go 실측 — J-P1-2 H4). 합류는 I-P5(LegacyCapture 섹션 확장 + GatewayAPI
CRD 실클러스터 설치와 함께)로 이월된다.

### 4.9 `configStorage` 섹션 (`K8sConfigStorageSection`)

| legacy 필드 | 처분 | V2 대응 / 사유 |
|---|---|---|
| configMaps[].name / namespace | **mapped** | DisplayName + 페어링 키 |
| configMaps[].keys | **mapped** | `normalized.dataKeys` (개수 카운트 → 키 이름 목록; **data 값 미수집**) |
| secrets[].name / namespace | **mapped** | DisplayName + 페어링 키 |
| secrets[].type | **mapped** | `normalized.type` |
| secrets[].keys | **mapped** | `normalized.dataKeys` (키 이름만 — **§14.1 "secret metadata", 데이터 값 절대 미수집**) |
| storage[].name / kind / namespace | **mapped** | pv→`persistent_volume`·pvc→`pvc` Subtype + 페어링 키 |
| storage[].namespaceScope | **mapped** | `normalized.namespaceScope` — pv: annotation `ops-admin.io/namespace-scope`, 기본 "Cluster-scoped"(legacy `storageNamespaceScope` 동치). pvc 행은 legacy 공란 — pvc는 키 생략(행 형상은 조립 P1-D 소유) |
| storage[].status | **mapped** | `normalized.phase` (pv/pvc status.phase) |
| storage[].capacity | **mapped** | `normalized.capacityGB` |
| storage[].storageClass | **mapped** | `normalized.storageClassName` |
| storage[].sourceType / path / nfsServer | **mapped** | `normalized.sourceType`·`normalized.sourcePath`·`normalized.nfsServer` — spec.persistentVolumeSource(hostPath→"hostPath"+path, nfs→"NFS"+path+server, 그 외 키 생략). legacy `persistentVolumeSource` 동치(pv 한정 — pvc는 소스 없음) |
| storage[].accessModes | **mapped** | `normalized.accessModes` |
| storage[].reclaimPolicy | **mapped** | `normalized.reclaimPolicy` (spec.persistentVolumeReclaimPolicy — pv 한정, 부재 시 키 생략) |
| configMaps[].age / secrets[].age / storage 상세(YAML 등) | dropped(volatile) | |

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
  정규화에 진입한다 — 값은 어댑터 경계에서 폐기.
- 게이트웨이 모드는 `Connection.Config{"connection_mode","gateway_id"}` + 주입 dialer로만
  구성(A4) — Phase 2 게이트 증명은 직접 연결, 실환경 홉은 M2.
