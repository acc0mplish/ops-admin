# Tencent 매핑 — legacy `cloudInstance` ↔ V2 정규화 (§14.2·§15.2 권위 매핑 표)

> **위치 원칙**: §15.2 — *"the mapping table lives next to the normalizer"* — 에 따라
> `adapter/tencent/normalizer.go` 옆에 둔다.
>
> **coverage rule (§15.2)**: *"the mapping table records, for every field the legacy
> API response serializes, the corresponding V2 normalized field and the transformation
> rule — or a documented reason the field is dropped."* 아래 §3 커버리지 표가 legacy
> `cloudInstance` **11필드 전수**의 처분이며, §15.2 규칙에 따라 전 필드가 mapped /
> dropped(reason) 둘 중 하나로 처분되어야 claim이 성립한다(계획 판정 J7 — r2 L1).
> 비교 엔진(cloudcompare — Phase E1)이 이 표를 소비한다.

- 원천(legacy): `backend/service/service.go` `cloudInstance` 11필드 — 채움 경로는
  `fetchCloudInstances(provider, ak, sk, region)` → `util/tencentcloud.go`
  `TencentCloudService.GetInstances`(v1 읽기 전용 — §3 disposition, M2까지 v1 권위).
  SDK는 common+cvm(v1.1.31) — 어댑터는 TC3 직구현(판정 J2(c))으로 **동일 CVM API
  DescribeInstances**를 호출하므로 원천 데이터는 동일하다.
- 대상(V2): `internal/infra/adapter/tencent/normalizer.go` — `DiscoveredResource`
  (Kind `compute.vm`·ExternalID·ExternalURN·Raw·Normalized)
- 스펙: §8.1 신원·§8.2 관측·§14.2 Tencent 매핑·§15.2 coverage rule·계획 §3.2/판정 J3·J7

---

## 1. 신원 — URN 규칙 (§8.1, 가정 A2)

URN 형식: `urn:tencent:{context}:{kind}:{external_id}` — `{context}` = provider_context의
`ContextID`(10진). 페어링 키(§15 클라우드 변형 — 판정 J7)는 **`instance_id`**(계정 스코프)
= `ExternalID`와 동일 성분.

| Kind | Subtype | ExternalID | ExternalURN | Raw 상한 |
|---|---|---|---|---|
| compute.vm | 인스턴스 패밀리(InstanceType 첫 접두 — `S5.LARGE8`→`S5`, 부재 시 "") | `InstanceId` | `urn:tencent:{ctx}:compute.vm:{InstanceId}` | 64KiB(초과 시 base64 절단 + `truncated` 마커) |

## 2. 단위·집계 규칙

- **Memory**: CVM `DescribeInstances.Memory`의 API 단위는 **GB** — V2 `memoryGB`는
  API 값을 그대로 기록한다. legacy v1 표시는 `Memory/1024`로 변환하는데
  (`service.go` fetchCloudInstances — "MB 전제" 표시 규격), 이는 legacy 표시 규격과
  API 단위의 불일치다. 비교 시 cloudcompare가 QuantityEpsilon 관례(k8s
  formatMemoryMB 왕복 올림 허용 승계 — 판정 J7)로 흡수할 변환 규칙으로 기록해 둔다:
  `legacy 표시 GB = Memory/1024`, `V2 memoryGB = Memory`.
- **Disk**: `SystemDisk.DiskSize` + `DataDisks[].DiskSize` 합계(GB) — legacy 집계 규칙과
  동일(`util/tencentcloud.go`). nil `DiskSize` 스킵.
- **CPU**: `Cpu`(코어 수) 그대로 — legacy 표시는 `"N cores"` 문자열, V2는 수치.
- **Raw**: 프로바이더 응답 중 최소 필드 집합만(instanceId·instanceName·instanceType·
  instanceState·zone·region·cpu·memory·osName) — **Tags는 아예 디코드하지 않는다**
  (§4 tag 값 폐기). 초과 시 kubernetes 어댑터와 동일 절단 규약.

## 3. 커버리지 표 — legacy `cloudInstance` 11필드 전수 (coverage rule)

처분 어휘: **mapped** = V2 정규화 필드로 대응 · **dropped(사유)** = 미이관.

| legacy 필드 | legacy 형태 | 처분 | V2 대응 / 사유 |
|---|---|---|---|
| InstanceID | string | **mapped(페어링 키)** | ExternalID + ExternalURN 성분 — 페어링 키 `instance_id`(판정 J7) |
| HostName | string | **mapped** | DisplayName(legacy는 `firstNonEmpty(InstanceName, InstanceID)` 폴백 — V2 동일 규칙) |
| PrivateIP | string(첫 주소) | **mapped** | `privateIps[0]` — V2는 `PrivateIpAddresses` 전체 목록 수집 |
| PublicIP | string(첫 주소) | **mapped** | `publicIps[0]` — 동일 |
| CPU | `"N cores"` 표시 문자열 | **mapped** | `cpu` 수치(코어) — 비교는 QuantityEpsilon(J7) |
| Memory | `"N GB"` 표시 문자열 | **mapped** | `memoryGB` — 단위 규칙 §2 참조(legacy `/1024` 표시 변환) |
| Disk | `"N GB"` 표시 문자열 | **mapped** | `diskGB` — §2 집계 규칙 |
| OS | string | **mapped** | `os` (`OsName`) |
| Region | string(계정 리전 설정) | **mapped** | `region` — 리전 출처는 `Connection.Config["regions"]`(판정 J3), Placement.Region 우선 |
| SSHUser | 상수 `"root"` | **dropped(display-only)** | v1 호스트 표시 전용 필드 — 관측값이 아니라 v1 화면 관례 상수. V2 정규화 스키마에 대응 필드가 없다(§3.2의 `sshHint`는 관측값이 아닌 **프로바이더 기본 힌트**) → §15.2 규칙에 따라 사유 기록 후 dropped. mapping.md·cloudcompare 양쪽에 기록(판정 J7) |
| SSHPort | 상수 `22` | **dropped(display-only)** | 동일 |

## 4. 비교 스코프 비고 (§15.2·판정 J7)

- **인스턴스 상태**: legacy `cloudInstance`에는 상태 필드가 없어 비교 불가 — V2
  `status`(`InstanceState`: RUNNING/STOPPED/REBOOTING…)은 **ABSENT-dropped**로
  명시(관측 차이·휘발 아님). 상태 비교 도입은 cloudcompare 설계(Phase E1)에서 재검.
- **`sshHint`**: V2 정규화에는 존재하지만 프로바이더 기본 힌트(user root·port 22)라
  비교 집합에서 제외 — legacy SSHUser/SSHPort dropped 처분과 한 쌍이다.
- **ip 목록**: legacy는 첫 주소만 담으므로 페어 비교는 `privateIps[0]`/`publicIps[0]`
  성분으로 한다(V2가 추가 수집한 2번째 이후 주소는 v2-only — 비교 집합 외).
- **tag**: CVM `Tags`는 V2가 수집하지 않는다(디코드 단계에서 폐기 — §2) — legacy도
  수집하지 않으므로 양측 부재, coverage 대상 외.

## 5. 자격·보안 경계 (보존 제약 3·7)

- 자격은 `DiscoverRequest.Connection.Material["inventory"]`(브로커 purpose `inventory`
  Resolve 산물 JSON `{"accessKey","secretKey"}`)로만 진입 — 로그·에러·Raw·Normalized·
  아티팩트 어디에도 노출 금지. 에러 메시지는 신호 종류·상태 코드·API 오류 코드만 담는다.
- 서명은 in-tree TC3-HMAC-SHA256(`client.go` tc3Authorization — finops
  `finOpsTencentRequest` 선례 승계, 가정 A6) — 새 SDK 의존 없음(보존 제약 4, §10.2).
- 리전은 `Connection.Config["regions"]`로만(계정 리전 필수 — legacy
  `GetInstances` "at least one Tencent Cloud region is required" 계약 승계).
