# Aliyun 매핑 — legacy `cloudInstance` ↔ V2 정규화 (§15.2 권위 매핑 표)

> **위치 원칙**: 스펙 §15.2 — *"lives next to the normalizer"* — 에 따라
> `adapter/aliyun/normalizer.go` 옆에 둔다(kubernetes mapping.md 선례 승계).
>
> **coverage rule (§15.2)**: *"the mapping table records, for every field the
> legacy API response serializes, the corresponding V2 normalized field and the
> transformation rule — or a documented reason the field is dropped."* 아래 §3
> 커버리지 표가 legacy `cloudInstance` **11필드 전수**(계획 r2 L1 — §0.2
> service.go:2913 실측)를 매핑 또는 dropped(reason)로 처분한다. 커버리지 규칙상
> 11필드 전부가 둘 중 하나로 처분되어야 claim이 성립한다(계획 §6·J7).

- 원천(legacy): `backend/service/asset_cloud_aliyun.go` `fetchAliyunCloudInstances`
  → `aliyunCloudInstances` (v1 raw ECS RPC — HMAC-SHA1·DescribeRegions/
  DescribeInstances·ecs.<region>.aliyuncs.com. §3 disposition — v1 무변경,
  M2까지 v1 권위)
- 원천(provider): Aliyun ECS `DescribeInstances` 응답의 `Instances.Instance`
  요소 (V2 클라이언트 `client.go` — 같은 RPC 규약의 어댑터 재구현, 판정 J2)
- 대상(V2): `internal/infra/adapter/aliyun/normalizer.go` — `DiscoveredResource`
  (Kind·Subtype·ExternalID·ExternalURN·Raw·Normalized)
- 스펙: §8.1 신원·§8.2 관측·§14.2 클라우드 매핑·§15.2 coverage rule·계획
  §3.1/§3.2/판정 J7

---

## 1. 신원 — URN 규칙 (§8.1·가정 A2)

URN 형식: `urn:aliyun:{context}:compute.vm:{instance_id}` — `{context}` =
provider_context의 `ContextID`(10진). k8s normalizer URN 관례 승계
(`UNIQUE(context,kind,urn)` 성분).

| 성분 | 값 |
|---|---|
| ExternalID | `InstanceId` (예: `i-xxxx`) |
| ExternalURN | `urn:aliyun:{context}:compute.vm:{InstanceId}` |
| Kind | `compute.vm` (어휘 기존 — 확장 0) |
| Subtype | 인스턴스 패밀리 — `InstanceType`에서 추출 (`ecs.g6.large` → `g6`). 없으면 `""` (§3.2) |

## 2. 단위 정규화 · Raw

- memory: ECS `Memory`(MB) → `memoryGB` = MB/1024 (legacy `formatAliyunMemory`의
  정수 나눗셈과 같은 스케일 — V2는 float).
- disk: `SystemDisk.Size` + `DataDisks.Disk[*].Size` 합산(GB) — legacy
  `diskGB` 합산과 동일.
- cpu: `Cpu`(vCPU 정수) — legacy 표시 문자열 `"N vCPU"`의 원값.
- Raw: `{instanceId, instanceName, regionId, zoneId, instanceType, status}`만 —
  자격·서명 파라미터는 구조적으로 진입 불가(§3.1 redaction, 하네스 단얫 4).
  크기 상한 `MaxRawBytes = 64KiB`(§8.2 — k8s 어댑터와 동일 계획 도입값,
  가정 A12). 초과 시 `{"rawB64":…, "truncated":true}` 절단 보존 +
  `normalized["truncated"]=true`.

## 3. 커버리지 표 — legacy `cloudInstance` 11필드 전수 (§15.2)

페어링 키(§15 비교 엔진 identity 대조 키) = **`InstanceID`**(계정 스코프).

| # | legacy 필드 | ECS 원천 | V2 처분 | 변환 규칙 / dropped 사유 |
|---|---|---|---|---|
| 1 | `InstanceID` | `InstanceId` | **페어링 키** + ExternalID + URN 성분 | verbatim |
| 2 | `HostName` | `InstanceName`(빈 값 시 InstanceId 폴백) | `Normalized.displayName` + `DisplayName` | verbatim(공백 trim) |
| 3 | `PrivateIP` | `VpcAttributes.PrivateIpAddress` → `InnerIpAddress` → 첫 NIC `PrimaryIpAddress`/`PrivateIpSets` (legacy 우선순위 동일) | `Normalized.privateIps[]` | legacy는 첫 IP 단일 스트링, V2는 전수 리스트 — 비교 시 first 대응 |
| 4 | `PublicIP` | `PublicIpAddress.IpAddress` 첫값 → `EipAddress.IpAddress` | `Normalized.publicIps[]` | EIP는 리스트 말미에 가산 |
| 5 | `CPU` | `Cpu` | `Normalized.cpu` (int) | legacy 표시 `"{n} vCPU"` ↔ V2 원값 n — §15 QuantityEpsilon 관례로 문자열 왕복 |
| 6 | `Memory` | `Memory`(MB) | `Normalized.memoryGB` | legacy `"{MB/1024} GB"` 정수 나눗셈 ↔ V2 float — QuantityEpsilon |
| 7 | `Disk` | `SystemDisk.Size`+`DataDisks` 합 | `Normalized.diskGB` (int) | legacy 표시 `"{n} GB"` ↔ V2 원값 n |
| 8 | `OS` | `OSName` | `Normalized.os` | verbatim |
| 9 | `Region` | `RegionId`(빈 값 시 질의 리전 폴백) | `Normalized.region` + `Raw.regionId` | verbatim |
| 10 | `SSHUser` | 상수 `"root"` (legacy 하드코딩) | **dropped** | 사유: **display-only** — v1 host 화면 표시 전용 필드(§14.2·계획 r2 L1). 관측값이 아닌 리터럴 상수라 V2 정규화 스키마에 대응 필드가 없다. V2의 `Normalized.sshHint:{user:"root",port:22}`는 **프로바이더 기본 힌트**(§3.2)로, legacy 관측값의 매핑 대상이 아니다. §15 비교 대상 제외 |
| 11 | `SSHPort` | 상수 `22` (legacy 하드코딩) | **dropped** | 사유: SSHUser와 동일 — display-only 상수(§14.2) |

### 3.1 legacy 집합 밖 V2 필드 (v2-only — 비교 집합 밖)

| V2 필드 | ECS 원천 | 비고 |
|---|---|---|
| `Normalized.zone` | `ZoneId` | legacy cloudInstance에 없음 — v2-only |
| `Normalized.instanceType` / `Subtype` | `InstanceType` | legacy에 없음 — v2-only |
| `Normalized.status` | `Status` | legacy에 없음 — v2-only. 인스턴스 상태 변경은 휘발성이 아니라 관측 차이(판정 J7) — legacy에 상태 필드가 없어 **ABSENT-dropped**로 §15 비교 대상 제외 |
| `Normalized.sshHint` | 없음 (프로바이더 기본) | 표시 보조 — 비교 대상 아님(위 10·11 참조) |

## 4. 디스커버리 순서 · 커서 (판정 J3)

- 커서: `"<region>|<pageNumber>"` — 리전×페이지 순회(terminal cursor `""`).
- 리전 출처: 계정 regions 설정(`Connection.Config["regions"]` — 백필이 context
  metadata에서 채움) 우선, 없으면 `DescribeRegions` 열거(legacy와 동일 폴백).
- 페이징: `PageSize=pageSize·PageNumber` — 종료 규칙은 legacy와 동일(빈 배치
  또는 `PageNumber×PageSize >= TotalCount`).
- 엔드포인트: `req.Connection.Endpoint` 우선(mock 주입면 — 판정 A3·J1 Path M),
  빈 값이면 `ecs.<region>.aliyuncs.com`(리전 조합).
