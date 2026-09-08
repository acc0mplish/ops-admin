# 리소스 UID ↔ OTel semconv 매핑 테이블 (semconv v1.44.0 고정)

- 출처: `v2-phase1/otel-lgtm-recipe-plan.md` §6. **r2에서 본 파일로 분리**(문서 길이 계약 ≤650줄).
- 본 파일의 표가 **P6 산출물의 정본**이다 — P6은 이 내용을 스펙 `docs/architecture/multi-infrastructure-control-plane-v2.md` **§18.5 "리소스 텔레메트리 semconv 매핑"** 절로 착지시킨다.
- 좌변(우리 어휘·URN)의 실측 근거는 계획 §1.4.

## 0. URN 형상 (매핑의 좌변 — 테스트 실측)

- k8s: `urn:k8s:{ctx}:node:{uid}` · `:namespace:{name}` · `:pod:{uid}` · `:workload:{ns}/{kind}/{name}` · `:service:{ns}/{name}` — `backend/internal/infra/adapter/kubernetes/adapter_test.go:284-293`
- proxmox: `urn:proxmox:{ctx}:hypervisor_node:{node}` · `:vm:{node}/{vmid}` · `:system_container:{node}/{vmid}` · `:pool:{node}/{storage}` · `:interface:{node}/{iface}` — `backend/internal/infra/adapter/proxmox/discovery_mock_test.go:276-288`, 파서 계약 `adapter/proxmox/executor.go:55,84-86`
- 어휘 원천: `backend/internal/infra/contract/resource_kind.go:4-24`(M1 19종) + `:32-35`(Phase2 확장 2종) = **21종**

## 1. 매핑 규칙과 전수 표

**규칙 4가지**:
1. registry에 실재하는 속성만 매핑한다. **대응이 없으면 발명하지 않고 `opsadmin.*`로 적는다.**
2. 사설 네임스페이스는 **`opsadmin.`** — `otel.`·`opentelemetry.` 및 semconv 예약 루트(`k8s.`·`host.`·`cloud.`·`container.`·`service.`·`network.`·`process.`)를 침범하지 않는다.
3. 전 kind 공통: `opsadmin.resource.uid`(URN 전문)·`opsadmin.connection.uid`·`opsadmin.context.uid`·`opsadmin.resource.kind`·`opsadmin.resource.subtype`.
4. **속성이 질의 가능해지는 경로**: OTLP 리소스 속성은 Mimir에서 **`target_info` 시리즈의 라벨**로 착지하고, 메트릭과는 **`on (job, instance)`** 로 조인한다. 스모크 실측에서 `job="ops-admin-control-plane"`·`instance="stub:80"`이 targets-map의 `job` 키로부터 그대로 착지함을 확인했다. `up`·`scrape_duration_seconds`·`scrape_samples_scraped`·`scrape_samples_post_metric_relabeling`·`scrape_series_added`·`target_info`는 **정상적으로 함께 생기는 합성 시리즈**이며 드리프트가 아니다.

| # | kind (`resource_kind.go`) | 발행 어댑터 | semconv v1.44.0 속성 | 안정성 | 값 출처 |
|---|---|---|---|---|---|
| 1 | `machine.baremetal` | 없음(어휘만) | `host.id`, `host.name` | **Development** | — |
| 2 | `compute.hypervisor_node` | proxmox | `host.id`, `host.name` | **Development** | URN 꼬리 `{node}` = `pve_node_info{name}` |
| 3 | `compute.vm` | proxmox | `host.id`, `host.name`, `host.type` | **Development** | `host.id` ← `pve_up{id}`의 `qemu/{vmid}` |
| 4 | `compute.system_container` | proxmox | `host.id` | **Development** | `lxc/{vmid}`. **`container.*`는 OCI 컨테이너용이라 LXC 게스트에 적용하지 않는다** |
| 5 | `compute.image` | 없음(어휘만) | `host.image.id`, `host.image.name` | **Development** | — |
| 6 | `compute.template` | 없음(어휘만) | **대응 없음** | — | `opsadmin.template.id` |
| 7 | `orchestration.cluster` | 없음(어휘만) | `k8s.cluster.name`, `k8s.cluster.uid` | **Stable** | — |
| 8 | `orchestration.node` | kubernetes | `k8s.node.name`, `k8s.node.uid` | **Stable** | URN 꼬리 = node uid |
| 9 | `orchestration.namespace` | kubernetes | `k8s.namespace.name` | **Stable** | URN 꼬리 = 이름 |
| 10 | `orchestration.workload` | kubernetes | subtype 분기: `k8s.deployment.name` / `k8s.statefulset.name` / `k8s.daemonset.name` (+`k8s.namespace.name`). **어휘 확장 대비**: `k8s.replicaset.name`·`k8s.job.name`·`k8s.cronjob.name`도 전부 Stable이나 **현 어댑터는 발행하지 않는다**(`adapter/kubernetes/adapter.go:196-198` 3종만 열거, `executor.go:66,69`가 그 외를 거부) | **Stable** | URN `{ns}/{kind}/{name}` 3분할 |
| 11 | `orchestration.pod` | kubernetes | `k8s.pod.name`, `k8s.pod.uid` (+`k8s.namespace.name`) | **Stable** | URN 꼬리 = pod uid |
| 12 | `orchestration.configmap` | kubernetes | **대응 없음**(registry 실조회 — `k8s.configmap.name` 미정의) | — | `opsadmin.k8s.configmap.name` + `k8s.namespace.name` |
| 13 | `orchestration.secret` | kubernetes | **대응 없음**(`k8s.secret.name` 미정의) | — | `opsadmin.k8s.secret.name` + `k8s.namespace.name`. **이름만 — 내용은 어떤 경로로도 싣지 않는다** |
| 14 | `network.segment` | 없음(어휘만) | **대응 없음** | — | `opsadmin.network.segment.id` |
| 15 | `network.interface` | proxmox | `network.interface.name`은 **트래픽 맥락용이지 리소스 신원용이 아니다** → 신원은 사설로 | — | `opsadmin.network.interface.name` + `host.id`(소유 노드) |
| 16 | `network.ip` | 없음(어휘만) | **대응 없음**(리소스 신원으로서) | — | `opsadmin.network.ip.address` |
| 17 | `network.load_balancer` | kubernetes | `k8s.service.name` (+`k8s.namespace.name`) — **registry에 실재한다**(r1이 "신원 속성 없음"으로 단정하고 `opsadmin.k8s.service.name`을 발명한 것은 규칙1·2 동시 위반이었다. r2 정정) | `k8s.service.name` = **Development** / `k8s.namespace.name` = Stable | URN `{ns}/{name}` 2분할 |
| 18 | `storage.pool` | kubernetes, proxmox | **대응 없음** | — | `opsadmin.storage.pool.name` (+ PVE는 `host.id`) |
| 19 | `storage.volume` | kubernetes | **대응 없음**(`k8s.volume.*`는 볼륨 **마운트** 맥락용이지 PV 신원이 아니다) | — | `opsadmin.storage.volume.name` |
| 20 | `storage.snapshot` | 없음(어휘만) | **대응 없음** | — | `opsadmin.storage.snapshot.id` |
| 21 | `identity.account` | (aliyun/tencent — 범위 제외) | `cloud.provider`, `cloud.account.id`, `cloud.region` | (미확인 — 범위 제외로 검증 안 함) | 표에만 남긴다 |

**stability 실측**(semconv v1.44.0 `model/k8s/registry.yaml` 원본 파싱, 2026-09-08): **Stable `k8s.*`는 42종**이다 — 본 표가 쓰는 것은 그 부분집합이며 "10종이 전부"가 아니다. `k8s.configmap.name`·`k8s.secret.name`은 **부재**(ABSENT), `k8s.service.name`은 **Development**. `host.*`는 전부 Development. assessment §6의 경고가 확정됐다. **Development 속성에 의존하는 대시보드는 semconv 업그레이드로 깨질 수 있음을 §4.8 런북에 1줄 남긴다.**

**Grafana 딥링크 템플릿**: `{GRAFANA_BASE}/explore?left={"datasource":"ops-admin-mimir","queries":[{"expr":"<식>"}],"range":{"from":"now-6h","to":"now"}}` (URL 인코딩). `{GRAFANA_BASE}`는 `OTEL_GRAFANA_BASE`(§4.2), 데이터소스 uid는 `ops-admin-mimir`(§4.4)로 고정. **`connection` 라벨값은 커넥션 UID이므로 UID→표시명 해석은 딥링크 생성 측이 한다** — Mimir에는 이름이 없다.

---
