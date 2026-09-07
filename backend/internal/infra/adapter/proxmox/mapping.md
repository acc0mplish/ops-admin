# Proxmox VE 매핑 — PVE API2 응답 ↔ V2 정규화 (§15.2 권위 매핑 표)

> **위치 원칙**: 스펙 §15.2 — *"lives next to the normalizer"* — 에 따라
> `adapter/proxmox/normalizer.go` 옆에 둔다(kubernetes·aliyun mapping.md 선례).
>
> **zero prior code — legacy comparison set is empty by spec**: PVE는 v1 소스
> 테이블이 전무하다(계획 §0.2 실측 — `%proxmox%`·`%pve%` 0행). §25 Slice B
> 재증명의 대상이 "a provider with zero prior code in the tree"이므로 **legacy
> 비교 집합은 스펙상 공집합**이다. §15 비교 프로토콜·페어링 키 표는 대상이
> 없어 이 문서에 존재하지 않고, 커버리지 표는 **어댑터가 실제 디코드하는 PVE
> 응답 필드 전수**를 mapped / dropped(reason)로 처분한다(claim 17 —
> TestProxmoxMappingCoverage가 `reflect`로 정규화기 디코드 구조체와 기계 동치
> 단얫).

- 원천(provider): PVE API2 `/api2/json` GET — `/cluster/status`·`/nodes`·
  `/nodes/{n}/qemu|lxc|storage|network` (V2 클라이언트 `client.go` — Phase B,
  직접 HTTP·PVEAPIToken 헤더, 판정 J1)
- 대상(V2): `internal/infra/adapter/proxmox/normalizer.go` —
  `DiscoveredResource` (Kind·Subtype·ExternalID·ExternalURN·Raw·Normalized)
- 스펙: §8.1 신원·§8.2 관측·§14.3 Proxmox 매핑·§15.2 coverage rule·계획
  §3.1/판정 J3·가정 A2/A10/A11

---

## 1. 신원 — kind/Subtype/URN 규칙 (§8.1·판정 J3)

URN 형식: `urn:proxmox:{context}:{kind세그}:{node}/{성분}` — `{context}` =
provider_context의 `ContextID`(10진), `{kind세그}`는 kind의 마지막 성분
(executor URN 파싱 계약 — 판정 J7의 `(vm|system_container)` 문언). k8s
normalizer URN 관례 승계(`UNIQUE(context,kind,urn)` 성분).

| PVE 표면 | kind (기존 어휘 — 확장 0) | Subtype | URN 세그 | ExternalID / URN 성분 |
|---|---|---|---|---|
| `/cluster/status` **cluster행** | — (리소스 행 아님 — ProviderContext 성격, 등록 CLI 소관) | — | — | normalizer 미산출 |
| `/cluster/status` **node행** | `compute.hypervisor_node` | `""` | `hypervisor_node` | `{nodeName}` → `urn:proxmox:{ctx}:hypervisor_node:{node}` |
| `/nodes/{n}/qemu` | `compute.vm` | `qemu` | `vm` | `{node}/{vmid}` → `urn:proxmox:{ctx}:vm:{node}/{vmid}` |
| `/nodes/{n}/lxc` | `compute.system_container` | `lxc` | `system_container` | `{node}/{vmid}` → `urn:proxmox:{ctx}:system_container:{node}/{vmid}` |
| `/nodes/{n}/storage` | `storage.pool` | PVE type (`zfs`·`dir`·`lvm`…) | `pool` | `{node}/{storeid}` → `urn:proxmox:{ctx}:pool:{node}/{storeid}` |
| `/nodes/{n}/network` | `network.interface` | PVE type (`bridge`·`eth`·`bond`…) | `interface` | `{node}/{iface}` → `urn:proxmox:{ctx}:interface:{node}/{iface}` |

**QEMU와 LXC는 다른 kind다 — 붕괴 금지(§14.3)**: qemu → `compute.vm`, lxc →
`compute.system_container`. Subtype만 프로바이더 세부를 담는다(§8.5
"Provider-native details remain subtypes").

**클러스터 쿼럼의 처분 (A10)**: `/cluster/status` cluster행의 quorate·nodes는
**리소스 정규화 경로에 진입하지 않는다** — ProviderContext.Status/MetadataJSON의
성분(등록 CLI 소관)이며, 노드 리소스·VM 리소스 어느 상태 필드로도 붕괴되지
않는다. Health 메시지만이 쿼럼을 보고한다(adapter.go Health — 클러스터명·노드
수·quorum).

**standalone 폴백 (A11 — 실측 형상)**: standalone 노드에서 `/cluster/status`는
cluster행 없이 node행만 반환한다. normalizer는 노드행만 정규화한다 — 클러스터
신원을 허위 구성하지 않는다. 컨텍스트 ExternalID 폴백(유일 node행 name +
`{"standalone":true}` 마커)은 등록 CLI(Phase F)의 소관이다.

## 2. 단위 정규화 · Raw

- maxmem(바이트) → `memoryMB` = 바이트/1048576 (3자리 반올림).
- maxdisk·total·used·avail(바이트) → `*GB` = 바이트/1024³ (GiB 스케일 — k8s
  quantityToGB와 동일).
- maxcpu → `cores` (PVE 보고값 원값 — 공유 슬롯에서 소수).
- uptime(초) → `uptimeSeconds` (정수 반올림).
- content·tags(CSV) → 리스트(공백 trim·빈 성분 제거).
- Raw: 신원 + 프로바이더 네이티브 식별 성분만 — 자격·서명 파라미터는 구조적으로
  진입 불가(§3.1 redaction, 하네스 단얫 4 — 미해독 응답 필드에 심은 카나리로
  단얫). 크기 상한 `MaxRawBytes = 64KiB`(§8.2 — k8s 어댑터와 동일 계획 도입값,
  p4 가정 A12). 초과 시 `{"rawB64":…, "truncated":true}` 절단 보존 +
  `normalized["truncated"]=true`.
- nil-sparse 행: PVE는 성분을 생략하는 응답을 돌려줄 수 있다(p4 R8 교훈) —
  수치·플래그 성분은 포인터 디코드로 통과시키고, 대응 Normalized 키를 생략한다.
  표시명은 name이 없으면 `{node}/{vmid}`·vmid로 폴백한다.

## 3. 커버리지 표 — 정규화기가 디코드하는 PVE 응답 필드 전수 (§15.2 준용)

legacy 비교 집합이 공집합이므로(헤더 참조) 커버리지 대상은 **정규화기 디코드
구조체의 json 필드 전수**다. 처분 등급: `mapped`(정규화 표의 대응 키로 산출) /
`dropped(reason)`.

### 3.1 `/cluster/status` node행 → compute.hypervisor_node

| # | PVE 필드 | V2 처분 | 변환 규칙 / dropped 사유 |
|---|---|---|---|
| 1 | `type` | **dropped(dispatch-only)** | 사유: 행 분기용("node"/"cluster") — cluster행은 미산출(§1 표), node행은 kind가 표에서 고정이라 관측값 불요 |
| 2 | `name` | **mapped** | ExternalID·URN 성분·DisplayName + Raw.name |
| 3 | `ip` | **mapped** | `Normalized.ip` + Raw.ip — **오프라인 멤버 IP 보존**(§14.3 증류 계약 4: /cluster/status가 오프라인 멤버 IP를 보고하는 유일 표면) |
| 4 | `online` | **mapped** | `Normalized.online`(bool) + `Normalized.status`(`online`/`offline`) + Raw.online(원값 0/1) — 노드 상태는 노드 리소스에 귀속(클러스터 쿼럼 붕괴 금지 — A10) |

### 3.2 `/nodes/{n}/qemu|lxc` 게스트 행 → compute.vm(qemu) / compute.system_container(lxc)

| # | PVE 필드 | V2 처분 | 변환 규칙 / dropped 사유 |
|---|---|---|---|
| 1 | `vmid` | **mapped** | ExternalID·URN 성분·`Normalized.vmid` + Raw.vmid |
| 2 | `name` | **mapped** | DisplayName + Raw.name(빈 값 시 `{node}/{vmid}` 폴백) |
| 3 | `status` | **mapped** | `Normalized.status` — VM 헬스는 VM 리소스에만 귀속(A10) |
| 4 | `template` | **mapped** | `Normalized.template`(bool) |
| 5 | `maxmem` | **mapped** | `Normalized.memoryMB`(바이트→MB) |
| 6 | `maxcpu` | **mapped** | `Normalized.cores`(원값) |
| 7 | `maxdisk` | **mapped** | `Normalized.diskGB`(바이트→GB) |
| 8 | `tags` | **mapped** | `Normalized.tags`(CSV→리스트) |
| 9 | `uptime` | **mapped** | `Normalized.uptimeSeconds`(초) |

### 3.3 `/nodes/{n}/storage` 행 → storage.pool (Subtype = PVE type)

| # | PVE 필드 | V2 처분 | 변환 규칙 / dropped 사유 |
|---|---|---|---|
| 1 | `storage` | **mapped** | ExternalID·URN 성분(storeid)·DisplayName + `Normalized.storeid` + Raw.storage |
| 2 | `type` | **mapped** | `Subtype` + Raw.type + `Normalized.type` |
| 3 | `content` | **mapped** | `Normalized.content`(CSV→리스트) + Raw.content |
| 4 | `status` | **mapped** | `Normalized.status` |
| 5 | `active` | **mapped** | `Normalized.active`(bool) |
| 6 | `shared` | **mapped** | `Normalized.shared`(bool) |
| 7 | `total` | **mapped** | `Normalized.capacityGB`(바이트→GB) |
| 8 | `used` | **mapped** | `Normalized.usedGB` |
| 9 | `avail` | **mapped** | `Normalized.availGB` |

### 3.4 `/nodes/{n}/network` 행 → network.interface (Subtype = PVE type)

| # | PVE 필드 | V2 처분 | 변환 규칙 / dropped 사유 |
|---|---|---|---|
| 1 | `iface` | **mapped** | ExternalID·URN 성분·DisplayName + `Normalized` 노드 스코프 + Raw.iface |
| 2 | `type` | **mapped** | `Subtype` + Raw.type + `Normalized.type` |
| 3 | `active` | **mapped** | `Normalized.active`(bool) |
| 4 | `autostart` | **mapped** | `Normalized.autostart`(bool) |
| 5 | `vlan-aware` | **mapped** | `Normalized.vlanAware`(bool — 하이픈 필드명의 캐멀 케이스 치환) |
| 6 | `ports` | **mapped** | `Normalized.ports`(존재 시) |
| 7 | `slaves` | **mapped** | `Normalized.slaves`(존재 시) |
| 8 | `address` | **mapped** | `Normalized.address`(존재 시) |
| 9 | `gateway` | **mapped** | `Normalized.gateway`(존재 시) |

### 3.5 미채택 표면

| 표면/필드 | 처분 | 사유 |
|---|---|---|
| `/cluster/status` cluster행 전체(quorate·nodes·version·id) | **dropped(context-scope)** | 리소스 행 아님(§1) — 컨텍스트 성분은 등록 CLI(Phase F)가 Health 스모크로 확정 |
| `/nodes` 행(node·status·cpu·maxmem 등) | **dropped(traversal-only)** | 순회 전용 섹션 — 온라인 노드 열거의 입력. 노드 관측은 /cluster/status node행이 유일 출처(§3.1) — 이중 산출은 URN 중복이 된다 |
| 응답의 미디코드 필드(ha·cpu 사용률·netin/netout 등) | **dropped(not-decoded)** | Raw는 신원 + 프로바이더 네이티브 식별 성분만 수집(§2) — 미해독 필드 유출 부재는 redaction 카나리가 단얫 |

## 4. 디스커버리 순서 · 커서 (판정 J3)

- 커서: `"<section>|<node>|<index>"` — 섹션 고정 순서
  **status→nodes→qemu→lxc→storage→network**, 섹션 우선 노드×섹션 순회(같은
  섹션의 모든 온라인 노드를 지나 다음 섹션). `nodes`는 리소스를 내지 않는 순회
  전용 섹션이다(§3.5). terminal cursor `""`.
- `status`는 노드 무관 1유닛(qemu 이하는 노드 스코프 — 노드 목록은 GET /nodes의
  online 행). offline 노드의 per-node API는 순회에서 제외 — 그 노드의 리소스는
  그 세대에서 결번이고, tombstone은 어댑터가 내지 않는다(§9.2 — provider
  outage is not resource deletion; 노드 자체는 /cluster/status node행으로
  online=false 관측).
- PVE API는 자체 페이징이 없으므로 페이지 상한(`pageSize`)은 어댑터가 클라이언트
  측 슬라이싱으로 집행한다 — 어떤 페이지도 상한을 초과하지 않는다(하네스 단얫 1).
- 엔드포인트: `req.Connection.Endpoint`에 `https://host:8006` 전체 베이스(가정
  A3) — 클라이언트가 `/api2/json` 접미 조립.
