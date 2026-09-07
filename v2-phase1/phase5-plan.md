# V2 Phase 5 계획 — Proxmox VE read-only + guarded operations (r1)

작성: 2026-09-07 (r1) · 티어 L (zero-prior-code 프로바이더에서 Slice B 기준 재증명 + UPID dual-mode·자격 2종 분리가 게이트 직결 — review-pr-xhigh 병행 권장) · 앵커(실측 시점 HEAD): `18e2cb1` (main, PR #41 merge)

**갱신 r1.1 (2026-09-07 — 게이트 판정 업데이트)**: 사용자가 실 Proxmox 엔드포인트 보유·제공을 확정(접속정보는 수령 대기). E-1은 **(a)안으로 귀결** — 게이트 (a)는 interim 아닌 스펙 문언 그대로 종결이 목표. mock PVE는 게이트 대체가 아니라 **테스트 하니스로만 한정**(단위·회귀·계약). §13-10 interim 마킹 규격은 불필요해져 제거 — 실엔드포인트 증명 절차만 계획. 접속정보 제공 방식 계약 신설(§E-1 — 저장소 커밋 전면 금지·런타임 프로비저닝).

**갱신 r1.2 (2026-09-07 — 엔드포인트 실측 결과)**: 도달성 증거 확보 — 호스트 도달·TLS 핸드셰이크 정상(`pve-api-daemon/3.0`, `https://192.168.250.43:8006`). 그러나 토큰 인증 **401 Authentication failed** — 사용자 재확인 대기(토큰 값 재생성 가능성). 게이트 (a) 실계정 증명 절차는 **토큰 재수령·검증 통과(curl 200) 후 실행**으로 계약(§13-1 0단계). 접속정보 보관처는 untracked `backend/data/p5-proxmox.env` 1곳으로 확정 — 계획 문서에는 경로만 기록. **실측 결함 1건**: `.gitignore`에 `backend/data/` 부재(`git check-ignore` exit 1) — env 파일이 커밋 가능 상태로 노출 → M10(.gitignore 1라인)으로 봉쇄.

**갱신 r1.3 (2026-09-07 — 토큰 검증 통과·렌즈1 발견 반영)**: **자격 2벌 검증 통과로 §14.3 진입 전제조건 전량 충족 — E-1 종결**(블로커 해소). 토큰: RO `root@pam!testtest`(PVEAuditor 권한)·OPS `root@pam!opsadmin-ops`(privsep 0 전권) — tokenid는 헤더의 공개 성분이므로 기록 가능(시크릿 값은 미기입). 선행 curl 200 검증은 메인 세션이 수행·종결(F8 — §13-1의 0단계는 register-pve 내장 Validate로 재계약). `.gitignore` 봉쇄도 메인 완료(`.gitignore:25`·`git check-ignore` exit 0 실증 — M10은 "메인 완료·PR 편입"으로 재서술). 실측 형상 2건: (a) `/cluster/status`가 **standalone 노드에서 cluster행 없이 node행만** 반환 → J3 폴백 판정·A11 신설 (b) **qemu/lxc 게스트 0** → claim 10 증명 대체 선택지 명시. 렌즈1(완전성) 발견 반영: F2(reserved 픽스처 t.Fatal — registry_test.go Phase A 편입)·F4(§0.8 이월 6항 판정 보충·클라우드 소관 정정)·F5(§22-17 화해 문단)·F6(claim 8 경로 정정)·F7(증류 계약 인덱스 오기 2건)·F8(§13-1 스모크 수단). **상태계약 동기 규칙 신설(F1)**: r1.3부터 게이트 상태(E-1 등)는 state.json과 계획 서술이 동일 시점 기준으로 일치한다 — 어느 한쪽만 갱신된 상태로 두지 않는다.

**갱신 r1.4 (2026-09-07 — 3렌즈 병합 최종 개정)**: 완전성·기술오류·위험 3렌즈 병합 15건(HIGH 4·MEDIUM 5·LOW 6). **HIGH-1 시더 재계약(착수 전 필수)**: 메인 실증 — `grantRoutePermissionMenus`(seed.go:531)가 `value IN opdef.PermissionStrings()` 한정 조회라 opdef 무변경이면 신규 권한의 Menu 행·RoleMenu 부여가 불가하고 권한 검사(permission.go:64 sys_menu.value 조인)가 전원 403 → M8을 신규 시더 함수(Menu 행 생성 + super-admin 부여·멱등 upsert)로 재계약. **HIGH-2 롤백 SQL 보강**(관측·리소스·싱크런 + 태스크 이력 삭제 순서 명시), **HIGH-3 OPS 토큰 방어심도**(런북 축소 권한 가이드 + executor URN 파싱 방어 단얫), **HIGH-4 31b 실엔드포인트 증명 이월 행**(게스트 생성 시점 트리거). MEDIUM: 배치표 보완(client_test.go·secret-scan.py 행)·M5 경로 정정·페일오버 발화 조건 한계·insecure-tls 사설망 전제·claim 13 하니스 단얫. LOW: §0.5 패턴 2곳 정정·Phase A 3파일(provider_type_test 제외 — 승격 무영향 실측)·Phase G 문구·§3.1 보기·claim 5 재현 절차·§12 공유 파일 인지.

- 원천: `docs/architecture/multi-infrastructure-control-plane-v2.md` r2+r2.1–r2.5 — 핵심 절: **§14.3 (Proxmox 범위·UPID 매핑·자격 분리·r2.2 ProxCenter 증류 계약 5개)**·**§20 Phase 5 게이트 문언**·**§21 PR 31**·**§25 (Slice B 성공 기준 — 재증명 대상)**·**§22 (금지 목록 — r1.3 추가)**·§16 (API V2 — 신규 라우트 0으로 충족)·§17 (UI)·§12.2 (VNC 콘솔 이월)·§10.2 (신규 의존 금지)·§4.1/§4.4 (secret 처리·P-class 서순)·§3 (처분 row — PVE는 해당 v1 모델 없음)·§5.3 (M2 밴드 5–7주 중 Phase 5분 2–3주)·§23 (테스트 전략 — Proxmox는 "no tests until the environment exists").
- **§22 금지 문언과의 화해 (r1.3 — F5)**: §22-17 "Scheduling code for a provider with no environment to test against"·§20 "a real Proxmox endpoint **to develop against**"·§23.2 "Proxmox+ no tests until the environment exists"는 전부 **환경 부재를 전제로 한 금지**다. r1.3 시점에 환경이 존재하고(엔드포인트 도달·토큰 2벌 검증 통과 — §0.1) 자격은 이미 검증됐으므로 금지의 전제가 소멸한다 — Phase 5의 착수·테스트 작성은 §22-17 위반이 아니라 그 전제의 해소다. 본 계획은 이 화해를 근거로 실엔드포인트 검증 경로(§13-1·claim 10)를 유일한 게이트 경로로 유지한다(mock은 하니스 — J8).
- 선행 Phase 자산: Phase -1~4 전부 종결(interim). 특히 Phase 3의 실행 회로(`OperationExecutor`/`TaskPoller`/`PollRequest.Connection`·승인 체인·`OnTaskTerminal`)와 Phase 4의 클라우드 어댑터·R2 v2 증명기구·mock 엔드포인트 패턴.
- 범위 (§21): **PR 31** `feat(proxmox): read-only adapter … (Phase 5)` — §21은 "Milestone 2+ (scoped at entry; shape for reference)"로 참고 형상이므로 본 계획은 PR 31을 31a(read-only)/31b(guarded ops)/31c(UI·E2E·종결) 서브 PR로 분해한다(p4의 PR 27–30 4분해 선례 승계).
- 출구 게이트 (§20 Phase 5 문언 전문): "**Slice B criteria re-proven on a provider with zero prior code in the tree**". §25 Slice B 요소 4개 — (a) 동일 계약을 통한 read-only discovery (b) 공통 인벤토리 UI가 compute kinds 렌더 (c) **공유 스키마 변경 없음(diff 증명)** (d) **코어에 provider명 분기 없음(arch-boundary 증명)**. 스펙이 "comparison vs legacy"를 요구하지 않는다(PVE는 legacy 뷰 부재 — §0.2 실측).
- 계획 계약: `phase4-plan.md` r2 구조 승계 (게이트 실현가능성 판정 → 에스컬레이션 E → §0 실측 → 판단 J → 배치표 → 인터페이스 계약 → Phase 분해 ≤5파일 → 테스트 계약 → claims → 보존 제약 → 리스크 → 롤백 → 소유권 → 이월 대장 → 가정 A).

---

## 게이트 실현가능성 판정 (최상단 의무 — §0 실측 근거)

| 게이트 (§20 Phase 5 = §25 Slice B 재증명) | 판정 | 근거(§0 실측) | 전제 |
|---|---|---|---|
| (a) 동일 계약 read-only discovery — 실엔드포인트 | **실현 가능 (§14.3 진입 전제조건 전량 충족 — E-1 종결, r1.3)** | 엔드포인트 도달·TLS 정상(`pve-api-daemon/3.0`·`https://192.168.250.43:8006`·§0.1)·**토큰 2벌 검증 통과**(RO `root@pam!testtest` PVEAuditor·OPS `root@pam!opsadmin-ops` privsep 0 — §0.1). §14.3 entry precondition 3성분(엔드포인트 도달·API token·realm) 전부 충족·§20 "endpoint is not negotiable — no invented provider contracts" (a) 경로 | register-pve(내장 Validate — §13-1) → sync → 리소스 노출 (claim 10). 잔여는 게이트 실행뿐 — **interim 경로 없음** |
| (a) 계약·코드 경로 자체 | **실현 가능 — 코어 확장 거의 0** | `DiscoverRequest.Connection`(Material["inventory"])·섹션 커서·contracttest 하네스·SyncRunner가 프로바이더 무관(p4 §0.4 승계). kind 어휘가 PVE 리소스를 이미 커버(compute.hypervisor_node·compute.vm·compute.system_container·storage.pool·network.interface — §0.3) | 어휘 승격 판정 J2(provider type만) |
| (b) 공통 인벤토리 UI가 compute kinds 렌더 | **실현 가능** | ComputeInventory.vue가 `kindPrefix=compute.`로 compute 전 family 렌더·상세 드로어·ResourceOperations.vue가 registry opdef×kind 교집합을 provider 무관 조회(§0.6). PVE 오퍼레션이 opdef 등록 즉시 UI에 자동 등장 | kindOptions 배열 2항 추가(hypervisor_node·system_container — 필터 선택지 확장, 뷰 구조 변경 아님) |
| (c) 공유 스키마 변경 없음 (diff 증명) | **실현 가능 — 마이그레이션 step 0** | PVE는 v1 소스 테이블이 없어 백필 대상 부재(§0.2) → step0007 불필요. 커넥션 등록은 CLI(판정 J6)가 provider_connection·provider_context·secret_ref·provider_credential_binding **기존 테이블**에만 기입 | 없음 |
| (d) 코어 provider명 분기 없음 | **실현 가능하나 증명기구 확장이 선행** | R2 v2 패턴이 `\b(aliyun\|tencent\|alicloud\|tencentcloud)\b` — **proxmox 미포함**(§0.5). 또한 `provider_type.go`의 proxmox 승격(M1 목록 이동)이 어휘 예외 심볼 라인 안에서 일어나므로 기존 예외 장치가 그대로 커버 | R2 v3: 패턴에 proxmox 추가 + 카나리 플레인트(판정 J9) |

### 에스컬레이션 (사용자 승인 대상 — Phase 4 E-1/E-2 선례 형식)

- **E-1 (실 Proxmox 엔드포인트·토큰 — 종결, r1.3)**: §14.3 진입 전제조건이 **전량 충족됐다**. 엔드포인트 도달·TLS 정상(`https://192.168.250.43:8006`·`pve-api-daemon/3.0`)·**토큰 2벌 검증 통과** — RO `root@pam!testtest`(PVEAuditor 권한 — discovery용)·OPS `root@pam!opsadmin-ops`(privsep 0 전권 — operations용, §14.3 자격 분리 그대로). tokenid는 `PVEAPIToken=<user>@<realm>!<tokenid>=<secret>` 헤더의 공개 성분이므로 본 기록이 합법(시크릿 값은 미기입). 선행 curl 200 검증은 메인 세션이 수행·종결(F8). **블로커 해소 — 게이트 (a)는 이제 실행만 남은 상태**(§13-1·claim 10).
  - **접속정보 제공 방식 계약 (저장소 무오염 — 지속 계약)**:
    1. **자격 물질(토큰 시크릿·전체 PVEAPIToken 문자열)은 저장소에 절대 커밋 금지** — 소스·문서·계획 문서·테스트 픽스처·런북 예시 전부. 커밋되는 문서에는 파라미터 이름과 형식만 기재한다.
    2. **env 파일·config.yaml 커밋도 금지** — `backend/config.yaml`은 이미 untracked(§4.6)·`deploy/config.yaml`은 .gitignore 대상. 접속정보의 유일한 파일 보관처는 **untracked `backend/data/p5-proxmox.env` 1곳**(계획·문서에는 경로만 기록하고 값은 기입하지 않는다). **봉쇄 완료(r1.3 — 메인 세션 실행)**: `.gitignore:25`에 `backend/data/` 추가·`git check-ignore backend/data/p5-proxmox.env` exit 0 실증 — M10은 "메인 완료·PR 31a 편입"으로 재서술(Phase A 검증의 check-ignore 단얫은 유지 — 회귀 방지). 접속정보가 영구적으로 실리는 곳은 **런타임 DB뿐**: `provider_connection` 행 + `secret_ref` 봉인 행(v2 envelope — §4.2).
    3. **프로비저닝 경로**: `ops-admin register-pve`(판정 J6)이 유일한 등록 경로 — 시크릿은 env(`OPS_ADMIN_PVE_RO_TOKEN_SECRET`·`OPS_ADMIN_PVE_OPS_TOKEN_SECRET` — 셸에서 `backend/data/p5-proxmox.env`를 source해 주입) 또는 일회성 인자로 전달되고 CLI가 즉시 EncryptSecretV2 봉인. 런북 문서(N10)는 절차만 기록(예시 값은 플레이스홀더). **Phase 2 k8s-fixture 선례 승계** — kind 시드 클러스터 등록(k8s-fixture/README)도 "매니페스트는 커밋·kubeconfig는 로컬 런타임"으로 분리했다.
    4. **전달 채널**: 접속정보는 대화·보안 채널로 수령하고 세션 로그에 시크릿 값이 남지 않도록 값 자체는 생략(수령 사실·엔드포인트·tokenid 등 비밀 아닌 성분만 기록 — 본 항목이 그 준례).
- **E-2 (PVE 오퍼레이션 권한 어휘 — r1.4 시더 경로 재계약)**: registry opdef의 `RequiredPermission`은 기존 시드 권한 재사용이 원칙(p3 J4 — "role grants carry over")이나, 실측 v1 권한 175종 중 PVE guest 전원/스냅샷/구성에 부합하는 권한이 없다(§0.4 — `assets:k8s:workload:restart`는 k8s 전용 명명, `assets:host:terminal`은 콘솔). 선택지: (a) **권장 — 신규 권한 1종** `infra:pve:guest:operate`(4세그 — registry permissionPattern 충족)를 opdef 3종이 공유 + **신규 시더 함수가 Menu 행 생성 후 super-admin 역할에 부여**(M8 재계약 — 아래). (b) `assets:k8s:workload:restart` 재사용 — 라우트·시드 변경 0이나 k8s 전용 명명을 PVE에 적용하는 어휘 붕괴. p3 J4의 근거("기존 역할 보조")는 기존 도메인 확장에 대한 것이고 PVE는 zero-prior-code 신규 도메인이므로 신규 권한 1종이 정직하다 — 승인 요청. 라우트 레벨 대표 권한(defs_v2infra.go)은 변경 불필요(동적 조회 — §0.4).
  - **시더 경로 재계약 (r1.4 HIGH-1 — 메인 seed.go:531·permission.go:64 실증)**: 기존 서술의 "seedSuperAdminRoutePermissions 관례"는 **라우트 권한(value ∈ opdef.PermissionStrings())만 커버**한다 — `grantRoutePermissionMenus`(seed.go:531)가 Menu 행을 opdef 어휘 한정 조회하므로, opdef 무변경인 본 계획에서 신규 권한은 **Menu 행 자체가 존재하지 않고** RoleMenu 부여 대상이 없으며, 권한 검사(`adminHasAnyPermission` — sys_admin_role×sys_role_menu×sys_menu.value 조인, permission.go:64)가 **전원 403**이 된다. 따라서 M8은 seed.go에 **신규 시더 함수**를 만든다: (1) Menu 행 생성(value=`infra:pve:guest:operate`·menu_type 3·menu_status 1 — 기존 라우트 권한 메뉴 형상 준용·RoutePermissionsRootValue 하위) (2) super-admin 역할 role_menus 부여 — 전부 멱등 upsert(존재 시 skip). 기존 함수·시드 무변경(보존 제약 승계).
- **E-3 (PVE 등록 경로 CLI — §16.1 POST 라우트 이월 유지)**: PVE 커넥션은 v1 소스가 없어 백필 불가(§0.2). `POST /api/v2/infra/provider-connections`는 p3 §13-8 이월("커넥션 수명주기 UI 수요 시")으로 유지하고, 본 Phase는 CLI 서브커맨드 `register-pve`(판정 J6)로 등록한다. UI 등록 화면 수요 시 이월 대장 재검.
- **E-4 (arch-boundary-canary 잡 확장)**: R2 v3 proxmox 플랜트 추가 — p4 E-4와 동일 형태(보존 관례 "기존 CI 잡 무변경"의 명시적 예외). 게이트 (d) 증명기구 요건.
- **E-5 (config 오퍼레이션의 필드 화이트리스트)**: `pve.guest.config`가 PUT config 전체를 개방하면 파괴적 구성 변경이 가능해진다. 본 Phase는 **안전한 최소 필드 집합**(cores·memory[v2 style은 구현 시점 PVE 문법 확인 — 가정 A6)만 수용하고 그 외 필드는 거부하는 화이트리스트로 구현한다는 설계 승인. 전체 필드 개방은 이월.

이 외 에스컬레이션 없음. 판정 J2(kind 어휘 재사용)·J5(토큰 2종 SecretRef)·J6(등록 CLI)은 계획 승인 시 확인 요망.

---

## 0. 현재 코드·자산 실측

### 0.1 게이트 전제 — PVE 엔드포인트·자격 존재 여부 (전수)

실측 (2026-09-07, dev MySQL `ops_admin`·리포·환경):

```text
provider_connection            2행  전부 kubernetes (kind-v2-p2·kind-v2-p3) — proxmox 0행
SHOW TABLES LIKE %proxmox%     0행  (%pve% 동일 — PVE v1 소스 테이블 부재)
provider_credential_binding    inventory 2행·operations 1행 (k8s) — PVE 바인딩 0
환경변수                        PROXMOX*/PVE* 0건
backend/config.yaml·deploy/     proxmox·pve·8006 포트 그 무엇도 0건
코드                            backend 전 .go에서 "proxmox" 등장 4곳 (r1.3 정정 —
                               렌즈1 F2: r1의 "1곳뿐"은 제품 코드만 센 오기):
                               contract/provider_type.go:55 (ReservedProviderTypeNames — 제품)
                               contract/contract_test.go:66 (wantReserved 단얫)
                               registry/registry_test.go:139 (reserved 거부 픽스처 — F2 임계)
                               secrets/broker_test.go:18 (ProviderConnection 시드 문자열)
web/src                         proxmox 0건
go.mod                          PVE SDK 없음 (관련 모듈 0건)
```

**4곳의 승격 파급 판정 (F2)**: `provider_type.go:55`(승격 대상 본체 — M1)·`contract_test.go:66`(집합 단얫 갱신 — Phase A 소유)·`registry_test.go:139`(**TestRegisterProviderRejectsReservedAndUnknown이 proxmox을 reserved 거부 픽스처로 사용 — 승격 시 proxmox이 M1 어휘로 등록 가능해져 t.Fatal("reserved provider type registered without error") 발화. Phase A에서 픽스처를 vcenter로 교체**)·`broker_test.go:18`(ProviderType 문자열만 사용·등록 경로 아니므로 무영향 — 갱신 불요).

**엔드포인트 실측 (r1.3 — 자격 검증 통과로 갱신)**:

```text
호스트 도달                     OK — TLS 핸드셰이크 정상
서버 배너                       pve-api-daemon/3.0
엔드포인트                      https://192.168.250.43:8006 (사설망 — 비밀 아닌 성분)
토큰 인증                       검증 통과 (r1.3) — 2벌 모두 200:
                               RO  root@pam!testtest     (PVEAuditor 권한 — discovery용)
                               OPS root@pam!opsadmin-ops (privsep 0 전권 — operations용)
클러스터 형상                    standalone 노드 — /cluster/status가 cluster행 없이
                               node행만 반환 (J3 폴백 판정·A11)
게스트                          qemu/lxc 0 — compute.vm 증명 전제 깨짐 (claim 10 대체)
접속정보 보관처                  backend/data/p5-proxmox.env (untracked — 값은 미기입·
                               경로만. .gitignore:25 봉쇄 완료·check-ignore exit 0)
```

결론(r1.3 갱신): §14.3 entry precondition **전량 충족** — 게이트 (a)는 실행만 남은 상태(E-1 종결). Phase 4와의 차이는 유효 — (1) PVE는 **zero prior code**(§25 재증명의 요건 자체), (2) k8s E-2류 국소 대체물은 존재하지 않는다. **mock PVE는 게이트 대체가 아니라 테스트 하니스로만 사용**(판정 J8) — 게이트 (a) 증명은 전부 실엔드포인트.

### 0.2 백필·커넥션 소스 부재 (마이그레이션 step 불필요의 근거)

- k8s 백필(`inventory/backfill.go` — k8s_cluster 소스)·클라우드 백필(`inventory/backfill_cloud.go` 640줄 — asset_cloud_account·finops 소스) 모두 **v1 소스 테이블을 순회**하는 구조. PVE는 해당 v1 모델이 없다(§0.1) → 백필 경로 불성립 → **step0007 신설 불필요**(게이트 (c) 근거).
- 커넥션 등록은 CLI 신규(판정 J6). `main_sync.go`는 `RunK8sBackfill`+클라우드 백필을 호출하지만(§0.7) PVE분 추가 불필요 — PVE 커넥션도 등록 후에는 기존 `sync-inventory --connection <uid>`로 싱크된다(SyncRunner가 registry 조회만 — provider 무관).

### 0.3 어휘·계약 자산 (PVE가 재사용하는 형상 — 전부 실측)

```text
provider_type.go:42   M1ProviderTypeNames = [kubernetes, aliyun, tencent, fake]
         :55          ReservedProviderTypeNames = [proxmox, vcenter, cloudstack, openstack]
         — registry.RegisterProviderType V1이 reserved 이름을 거부(registry.go:60-66):
           "provider type %q is reserved; descriptor lands with its milestone (§7.1)"
           → Phase 5 첫 과제 = 어휘 승격(M1에 추가·Reserved에서 제거 — 판정 J2)
         :58          ProviderContextKinds에 "cluster" 이미 존재 (PVE 클러스터 컨텍스트 재사용)
resource_kind.go      M1ResourceKinds 19종 + Phase2 확장 2종 — PVE 리소스 매핑에 필요한
                      compute.hypervisor_node·compute.vm·compute.system_container·
                      storage.pool·network.interface 전부 이미 존재 → kind 어휘 확장 0(판정 J3)
capability.go         M1CapabilityVocabulary 7종 — mutation은 orchestration.kubernetes.apply뿐
                      (k8s 전용 명명). §10.1 "r1's list included vnc/spice console, system-container
                      power, and volume/snapshot manage — capabilities of providers not in M1.
                      They move to their owning phases." → Phase 5가 소유(판정 J2)
         CapabilityInterfaceMap 7행 — 신규 capability마다 매핑 표 행이 필수(registry.go:156-159
         가 "no §3.7 mapping-table row"를 fail-closed 거부)
contract/adapter.go   DiscoverRequest{ContextID,Cursor,Connection}·OperationRequest{...Connection}·
                      PollRequest{Handle,Connection}·OperationHandle{ProviderRef}·
                      OperationStatus{State: running|succeeded|failed}·ProviderSignalError 3종 —
                      전부 기존, PVE에 계약 변경 불요
engine.go executeClaimed  handle.ProviderRef == "" → e.Complete(claim, nil) — 주석 실측:
                      "§14.3 PVE shape: null-with-exit-status → single-attempt success"
                      **엔진이 UPID dual-mode의 null 반환 경로를 이미 구현 중** (판정 J4)
engine.go pollAsyncAttempts handle_ref <> ''인 시도만 폴 — UPID 폴링이 그대로 여기에 착지
```

### 0.4 권한·승인 표면

- registry `RegisterOperation` V6: permissionPattern `^[a-z0-9_-]+(:[a-z0-9_-]+){1,3}$` — `infra:pve:guest:operate` 충족(4세그).
- 라우트 레벨: `defs_v2infra.go`가 plan/execute 5종 non-GET의 대표 권한 `assets:k8s:workload:restart`·`ops:job:approve`를 가지고 **실제 권한은 V2DynamicMiddleware가 registry opdef에서 요청별 해석**(operations.go:67-76 `ResolveOperationPermission`). → PVE opdef가 신규 RequiredPermission을 가져도 라우트·골든·sensitive-routes 변경 0. 다만 그 권한을 보유한 역할이 없으면 누구도 실행 불가 → E-2 시드.
- 승인 체인: `provider_task` approval 컬럼 + `POST /tasks/:uid/approve|reject`(권한 `ops:job:approve`) — provider 무관, 재사용만으로 충분.
- v1 권한 175종 실측 스캔: PVE guest 전원/스냅샷/구성에 부합하는 기존 권한 없음(§E-2).

### 0.5 R2 v2 증명기구 (arch-boundary — 게이트 (d))

```text
scripts/check-arch-boundary.sh (p4 J8 산출물 — 실측):
  CORE_PACKAGES 10종 (dnsserver + infra/{contract,registry,inventory,secrets,model,
                 policy,metrics} + internal/tasks + internal/api/v2) — compose는 조립 루트로 제외
  R2_PATTERN='\b(aliyun|tencent|alicloud|tencentcloud)\b'  ← proxmox 부재
  R2_VOCAB_EXCEPTION_FILE=contract/provider_type.go + 심볼 핀포인트
                 (M1ProviderTypeNames|ReservedProviderTypeNames|ProviderTypeAliases)
  R2_BRANCH_PATTERN= (== "aliyun"|case ...|ProviderType.*==)  ← proxmox 부재
  *_test.go 제외(J8-d)·카나리 플레인트는 aliyun만
→ R2 v3 필요: 패턴 2곳에 proxmox 추가 — R2_PATTERN(스크립트 44행)·R2_BRANCH_PATTERN(47행)만(r1.4 LOW-10 정정: "3곳"은 오기. 어휘 예외 심볼·파일은 동일해서 provider_type.go의
  proxmox 승격 라인이 자동 커버)+ 카나리 proxmox 플레인트(E-4)
```

### 0.6 UI·E2E 표면

- `ComputeInventory.vue`(122줄): `kindPrefix='compute.'` 목록 + `kindOptions=['compute.vm','compute.volume','compute.snapshot','compute.image']`(63행) — **hypervisor_node·system_container 누락**(테이블 자체는 kind 문자열을 그대로 렌더하므로 행은 보이나 필터 선택지에 없다) → 1행 추가.
- `ResourceOperations.vue`(113줄)·`TaskDetail.vue`(171줄)·`InfraTasks.vue`(117줄): `GET /resources/:uid/operations`가 registry opdef × kind 교집합(operations.go:337-349 — provider 무관)을 반환하므로 PVE opdef 등록 즉시 자동 등장. **UI 신규 뷰 0·라우트 0·메뉴 0**(PVE는 compute kinds만 게이트 대상 — 스토리지/네트워크 뷰는 기존 공백, 이월).
- i18n: `infra-i18n.js`(76줄) ko/en — 신규 키 발생분은 kindOptions 라벨 정도(파리티 게이트 `node scripts/check-i18n-parity.mjs`).
- E2E: `web/e2e/stack.sh` + slice-a/b 패턴 — V2 테이블 시드 기반으로 실엔드포인트 불요(slice-c 안).

### 0.7 CLI·Phase 2 게이트 머지 순서 (p4 §4 freeze 승계 검토)

- `main_sync.go`(sync-inventory)·`main_compare.go`: p4 종결로 클라우드 백필·비교가 이미 머지됨. **본 Phase는 두 파일 모두 수정하지 않는다**(§0.2 — PVE 백필·§15 비교 대상 부재) → p4 §4 freeze 같은 머지 순서 제약이 새로 발생하지 않는다.
- 단, Phase 2 게이트(p2-state: "2·3일차 수동 보류"·1일차 pass만)는 **미종결** — kind 클러스터 v2-p2 보존 지침·비교 아티팩트 창구는 계속 유효. 본 Phase는 v2-p2·비교 아티팩트에 무접촉(보존 제약 9).
- anchor `18e2cb1` 시점 CI 잡 실측: **10잡**(backend-test·frontend-build·migration-test·route-coverage·route-coverage-canary·secret-scan·secret-scan-canary·arch-boundary·arch-boundary-canary·slice-a-e2e).

### 0.8 Phase 4 이월 대장 — 본 계획 처리 여부 판정 (과제 의무)

| 이월 (p4-state m1Remaining·§13) | 판정 | 근거 |
|---|---|---|
| Phase 2 게이트 2·3일차 수동 | **무관 — 이월 유지** | 본 Phase는 v2-p2·비교 아티팭트 무접촉. 사용자 수동 창구 그대로 |
| 클라우드 실계정 승격 (p4 m1Remaining "§13-10") | **클라우드 소관 — 원 소관 유지 (r1.3 정정)** | p4의 잔여 블로커는 **Aliyun/Tencent 실계정 증명**(클라우드)이다 — 본 계획이 흡수하는 게 아니라 M1 선언의 전제로 원 소관(p5-state의 추적 아님)에 남는다. PVE 자체의 증명 절차는 별도 행(아래) |
| 실엔드포인트 증명 절차 (PVE분) | **구조 승계·r1.1 격상** | p4 §13-10 승격 절차 형식을 실엔드포인트 단일 경로로 계획에 흡수(§13-1·claim 10) — interim 마킹 분은 폐기 |
| M5 ceefc27 freeze 해소 | **무관** | 클라우드 CLI freeze — 본 Phase 미접촉 |
| §18.2 메트릭 앵커 (M1 선언 전 필수) | **부분 승계** | PVE 어댑터가 기존 Counters(IncRateLimit/IncAPIError) 승계(p4와 동일 패턴). 나머지 전면 계측은 M1 준비 태스크 이월 유지 |
| p4 §13-1 vm 외 클라우드 family | **무관 — 이월 유지** | 클라우드 소관(disk·snapshot·eip/lb·vpc·sg·image — 재입구 M2 cutover 전 또는 운영 요구) |
| p4 §13-3 커버리지 래치 | **무관 — 불변 확인** | 래치는 opdef 커버리지 기준(v2-ci backend-test baseline 65.4/gate 60.0). PVE는 opdef를 추가하나 래치 측정 대상 패키지 집합이 아니라 opdef 테이블 — 수치 불변 예상, claim 2 전패키지 통과로 회귀 확인 |
| p4 §13-4 normalizer 관계 링크 | **무관 — 이월 유지** | K8s family 소관(p4 §0.7-3 승계). PVE 관계(VM→node runs_on)는 본 Phase 디스커버리가 정의하지 않는다 — 관계 링크 확장과 무관 |
| p4 §13-6 stale_source 상세 노출 | **무관 — 이월 유지** | v1 삭제 시나리오가 실제 발생하는 클라우드/K8s 운용 분. PVE는 v1 소스 부재로 stale_source 경로 자체가 없음(§0.2) |
| p4 §13-7 incremental 싱크 모드 | **무관 — 이월 유지** | phase2 §3.3 이월 승계. PVE는 단일 노드·리소스 수 작음 — full 모드로 충분(게이트 무요구) |
| p4 §13-8 R2 크래시 인젝션 안정화 | **무관 — 조건부 이월 유지** | 플레이크 관찰 시에만 착수. 본 Phase의 R2 v3 확장(M5)은 스캔 규칙 변경이지 잡 안정화 아님 — 별개 |
| p4 §13-9 asset_cloud_account 평문 P-class 잔존 | **무관 — 이월 유지** | 클라우드 v1 테이블 소관(M2 cutover에 폐기 예정). PVE 자격은 신규 SecretRef봉인 경로뿐 — 평문 잔존 경로 0 |
| POST /provider-connections 등 6종 (p3 §13-8) | **본 Phase에서도 이월 유지 (E-3)** | CLI 등록으로 충족. UI 등록 화면 수요 시 재검 |

---

## 1. 아키텍처 판단 (선택지 비교 + 선택 근거)

### J1 — PVE 클라이언트: 직접 HTTP, 신규 의존 0 〔임계〕

- **선택**: `adapter/proxmox/client.go`가 PVE REST(`/api2/json`)를 직접 호출. 인증은 헤더 1개 `Authorization: PVEAPIToken=<user>@<realm>!<tokenid>=<secret>`(§14.3 증류 계약 2) — aliyun HMAC-SHA1(p4 J2)보다 경량. JSON 디코드는 표준 라이브러리.
- **기각: Go PVE SDK 도입** — §10.2 "No new runtime dependencies in M1" 정신(계획 문언은 M1이나 §21 Phase 5는 같은 계약 위). SDK가 주는 이점(타입 완성도)이 토큰 헤더+JSON 디코드로 충족되는 범위를 초과하지 않고, mock 하니스 호환성은 직접 HTTP가 완전 자유. `git diff main -- backend/go.mod` 빈 diff가 claim 12.
- **기각: ProxCenter(TS) 코드 이식** — AGPL 라이선스·언어 상이. 스펙 §14.3 r2.2가 "distill behavior, don't copy code"로 못박음. 본 계획은 5개 증류 계약만 반영(각 J4·J1·J6·J6·J6 — r1.3 F7 정정: 계약 3 페일오버 비대칭의 착지는 J5가 아니라 J6(2)).

### J2 — 어휘 승격·확장의 최소집합 〔임계〕

```text
provider_type.go  M1ProviderTypeNames += "proxmox"·ReservedProviderTypeNames -= "proxmox"
                  (§7.1 "Reserved (descriptors land with their milestone)"의 착지 —
                   Phase 5가 그 마일스톤. R2 어휘 예외 심볼 라인 안의 변경이라 증명기구
                   재설계 불요 — 판정 J9)
capability.go     신규 3종 (§10.1 "they move to their owning phases"의 Phase 5분):
                    compute.power.manage      ReadOnly=false  OwnerPhase "Phase5"
                    storage.snapshot.manage   ReadOnly=false  OwnerPhase "Phase5"
                    compute.config.apply      ReadOnly=false  OwnerPhase "Phase5"
                  + CapabilityInterfaceMap 3행 (전부 RequiredInterface "OperationExecutor"·
                    OptionalInterfaces ["TaskPoller"] — k8s apply 행과 동일 형상: §14.3 UPID는
                    "핸들 반환 시 폴 시도, 널 핸들은 동기 완료(이중 모드 r2.2)"가 이미 k8s 행의
                    Reason 문언으로 존재함 — 실측 capability.go:68-70)
resource_kind.go  변경 0 — 기존 어휘 재사용 (판정 J3)
```

- 3종 vs 1종(`compute.proxmox.apply`) 절충: capability는 opdef의 위험·승인 정책 상위 묶음이다. power/snapshot/config는 리스크 프로파일이 달라(전원 즉시 영향·스냅샷 저장소 소비·구성은 지속 상태 변경) opdef RiskLevel이 분기하므로 capability도 분리한다. 스펙 §10.1이 "system-container power, volume/snapshot manage"를 별개로 언급한 것과 정합.
- 기각: kind 어휘 신규(pve.node 등) — 기존 compute.hypervisor_node 등이 정확 대응(§8.5가 이미 프로바이더 횡단 공통 종으로 설계). Subtype(qemu/lxc)으로 프로바이더 세부를 담는다(§8.5 "Provider-native details remain subtypes").

### J3 — 디스커버리 범위·정규화: §14.3 리소스 전수, kind 기존 어휘 재사용

```text
PVE 표면                 kind(기존)         Subtype        URN 성분 (§14.3 "node name + VMID")
/cluster/status(cluster행) ProviderContext   Kind="cluster" ExternalID=<cluster name>
                                          (k8s 선례 — 리소스 행 아님; 쿼럼·HA 헬스는
                                           context Status/MetadataJSON — VM 헬스에 붕괴 금지
                                           §14.3 Note의 계약화)
/cluster/status(node행)  compute.hypervisor_node ""          <nodeName> (IP는 Normalized —
                                           오프라인 멤버 IP 보존이 §14.3 증류 계약 4)
/nodes/{n}/qemu          compute.vm         "qemu"         <node>/<vmid>
/nodes/{n}/lxc           compute.system_container "lxc"     <node>/<vmid>
                                          (QEMU와 LXC는 다른 kind — 붕괴 금지 §14.3)
/nodes/{n}/storage       storage.pool       PVE type(zfs·dir·lvm…) <node>/<storeid>
/nodes/{n}/network       network.interface  PVE type(bridge·vlan·bond…) <node>/<iface>
URN: urn:proxmox:{ctxID}:{kind세그}:{...} — k8s/클라우드 normalizer 관례 승계
섹션 커서: status→nodes→qemu→lxc→storage→network 순 고정, 노드×섹션 순회
  (k8s 섹션 커서 관례 — PVE API는 자체 페이징 없음, 페이지 상한은 어댑터 pageSize로
   페이지당 리소스 수 상한을 하네스 단얫과 동일하게 유지)
standalone 폴백 (r1.3 실측 반영 — A11): /cluster/status가 standalone 노드에서
  cluster행 없이 node행만 반환한다(실측 §0.1). 컨텍스트 생성 판정:
  cluster행 존재 → ExternalID=<cluster name>(기존 가정)
  cluster행 부재(standalone) → ExternalID=<유일 node행의 name>·MetadataJSON
    {"standalone":true} 마커 — 클러스터명 허위 구성을 피하고 재실행 멱등 유지
```

- `mapping.md`(N5)는 §15.2 형상을 유지하되 **legacy 열이 전부 부재**임을 헤더에 명시("zero prior code — legacy comparison set is empty by spec")하고 표는 PVE 응답 필드 → Normalized 키 처분(mapped/dropped)만 기재. §15 비교 프로토콜은 대상 없음(§0 게이트 판정) — 비교 기구 코드 무변경.
- 오프라인 노드: `/nodes/{n}/qemu` 호출이 노드 다운 시 실패하면 해당 노드의 리소스는 그 세대에서 결번 — §9.2 "provider outage is not resource deletion"(N9)가 지키는 기존 회로. 노드 행 자체는 /cluster/status에서 online=false로 관측된다.

### J4 — UPID dual-mode 착지: 엔진 무변경, 어댑터가 두 형상을 인코딩 〔임계〕

엔진 실측(§0.3) — `executeClaimed`가 이미 `handle.ProviderRef == ""` → `e.Complete(claim, nil)`(주석 "§14.3 PVE shape: null-with-exit-status → single-attempt success"), `pollAsyncAttempts`가 `handle_ref <> ''`인 시도만 폴. **따라서 PVE executor는 계약만 지키면 된다(엔진·tasks 패키지 수정 0)**:

```text
Execute(POST …?background_delay=<N>):
  응답 data == null (에러 없음)         → OperationHandle{ProviderRef: ""} 반환
                                        → 엔진이 단일 attempt 성공으로 종단(동기 완료 창)
  응답 data == "UPID:…"                 → OperationHandle{ProviderRef:
      "upid|<connUID>|<node>|<rawUPID>"} 반환 (k8s rollout 핸들 인코딩 관례 승계 —
      connUID 자기서술(J12 승계)·node는 UPID 파싱 성분)
  동기 에러(5xx·에러 data)              → error 반환 → 엔진 executor_error/operation_failed
Poll(GET /nodes/{node}/tasks/{upid}/status):
  status=running                        → OperationStateRunning (다음 폴 사이클)
  status=stopped && exitstatus=="OK"    → OperationStateSucceeded{detail}
  status=stopped && exitstatus==에러문  → OperationStateFailed
UPID 파싱: "UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:"
  — node 추출로 타겟 폴링(§14.3 증류 계약 1의 문언 그대로). 핸들 인코딩에 raw UPID를
  통째로 실어 재폴 시 파싱 중복을 없게 한다(node는 어차피 필요 — 인코딩·디코딩 정합 단얫).
background_delay: executor가 요청에 명시적으로 첨부(1–30s 창 — 증류 계약 1). 기본값은
  구현 상수(A4) — 창 내 완료 시 PVE가 null을 돌려주는 것이 계약이므로 값 자체는
  증명 대상이 아니다.
멱등: power start/reboot 등 PVE 연산의 재실행 안전성은 opdef RetryPolicy 보수값
  (MaxAttempts 낮게·재시도는 승인 후 폴 회로에 한정)과 execute 멱등키(엔진 §13.4 —
  중복 제출 차단)로 통제. k8s J1(동결 restartedAt)에 대응하는 PVE측 동결 재료는
  없다(전원 상태는 idempotent — 이미 켜진 VM의 start는 PVE가 오류 반환).
  재시도 파괴 가능성은 RequiresApproval=true + 낮은 MaxAttempts로 완화(R3).
```

### J5 — 자격: 토큰 2종 SecretRef 분리 + 브로커 목적 어휘 재사용 〔임계〕

- SecretRef 재질 JSON(브로커 Value 단일 문자열 계약 — p4 J4 승계): `{"tokenUser":"root@pam","tokenID":"opsadmin-ro","tokenSecret":"…"}` — 클라이언트가 헤더 `PVEAPIToken=<tokenUser>!<tokenID>=<tokenSecret>`로 조립. 성분 저장이 전체 헤더 문자열 저장보다 나은 점: Health 메시지의 계정 식별 마스킹(tokenUser 접두)·토큰 분리 표시가 명시적.
- **바인딩 2행 (§14.3 "read-only token for discovery, operations token gated by approval")**: `Purpose="inventory"`(RO 토큰 SecretRef 지목)·`Purpose="operations"`(별도 RW 토큰 SecretRef 지목). purpose 어휘는 contract 상수 그대로(CredentialPurposeInventory/Operations — 확장 0). **같은 SecretRef를 공유하지 않는다**(클라우드 finops와 다름 — §14.3이 권한 분리를 명시).
- 실행 회로: 엔진 `connectionView`가 operations 바인딩을 Resolve해 `Material["operations"]` 주입(기존 회로 — 변경 0). discovery는 SyncRunner가 `Material["inventory"]`(기존 회로).
- 자격 평문 취급: p4 보존 제약 3 승계 — 프로세스 메모리 외 기록 금지·로그·감사·task_event·API 응답 미포함(claim 13).

### J6 — 커넥션 등록 CLI + 페일오버 비대칭 + reverse-proxy 플래그

**(1) 등록 — `ops-admin register-pve`(N8·E-3)**: `--endpoint --name --token-user --ro-token-id --ro-token-secret --ops-token-id --ops-token-secret [--realm 기본 pam] [--reverse-proxy] [--insecure-tls]`. 절차: 커넥션(provider_type=proxmox·ConfigJSON에 배치 모드·TLS posture)→컨텍스트(Kind="cluster" — Validate가 /version·/cluster/status로 클러스터명 확정)→SecretRef 2건(EncryptSecretV2 봉인)→바인딩 2행(inventory·operations). 멱등: 같은 `--name` 재실행은 갱신(upsert — k8s 백필 멱등 관례). 시크릿은 인자 대신 env(`OPS_ADMIN_PVE_RO_TOKEN_SECRET` 등) 수용을 기본으로 하고 인자는 폴백(셸 히스토리 노출 최소화 — claim 13).

**(2) 페일오버 증거 비대칭 (§14.3 증류 계약 3)** — 클라이언트 내부 규칙:

```text
실패 분류:
  쓰기(POST/PUT) 타임아웃/에러  → 페일오버 후보 수집·전환 금지 (착지 가능성·재시도 멱등성이
                                  지배 — "A timed-out write is never failover evidence")
  읽기(GET) 실패·연결 클래스 실패 → 임계치(연속 N회 — A4) 도달 시 후보로 전환
후보 목록: 전환 직전 /cluster/status 재조회로 node 행의 ip 전부(online·offline 포함 —
  증류 계약 4: /cluster/status가 오프라인 멤버 IP를 보고하는 유일한 표면이므로 후보
  목록이 클러스터 저하 시 멤버를 잃지 않는다). 조회 자체 실패 시 기존 엔드포인트 유지.
reverse-proxy 모드: ConfigJSON {"deployment_mode":"reverse_proxy"} — 플래그 시 후보
  전환 자체를 금지(엔드포인트 고정 — 증류 계약 5: 내부 node IP로 전환하면 안 되는 배치).
stateless 유지: 후보 목록을 어댑터 인스턴스에 캐시하지 않는다(요청마다 재조회 — k8s
  "HTTP clients are per-request" 원칙). 임계 카운터는 클라이언트 인스턴스 수명(요청
  스코프)에 둔다 — 프로세스 재시작 시 리셋이 허용되는 운영 등급(완화 R4).
발화 조건의 한계 (r1.4 MEDIUM-7 정합): 전환은 **Discover 경로 내 다중 GET 순회에서만
  발화 가능**하다 — 단일 GET(Validate/Health/Poll)는 임계(연속 2회)에 도달하지 않고,
  엔드포인트 전체 장애 시에는 후보 재조회(/cluster/status) 자체가 실패해 비발동,
  standalone 노드에서는 후보가 자기 자신 1개라 전환 대상이 없다(모두 비발동이 정상
  거동 — 정합성에 영향 없음, 가용성 최적화일 뿐). 카운터 스코프는 요청 스코프 유지(R4).
```

**(3) TLS**: PVE 기본은 자가서명 — `--insecure-tls` 시 ConfigJSON에 posture 기록 후 클라이언트가 InsecureSkipVerify. TLSProfile은 CA/verify posture 전용(§7.2 — 시크릿 없음). CA 본문은 SecretRef 참조가 정석이나 본 Phase는 사전 제공 CA 없이 플래그만(실엔드포인트 제공 시 요구 재검 — 가정 A7).

### J7 — 오퍼레이션 정의 3종 (guarded)

```text
pve.guest.power     ResourceKinds [compute.vm, compute.system_container]
                    RequiredCapability compute.power.manage
                    Payload {action: start|shutdown|stop|reboot}
                    Mutating·Risk medium·RequiresApproval true
                    IdempotencyPolicy "provider_state_convergent" (A4)
                    TimeoutSeconds 30·RetryPolicy{MaxAttempts:2, BackoffSeconds:10}
                    — POST /nodes/{n}/qemu|lxc/{vmid}/status/{action}
pve.guest.snapshot  [compute.vm, compute.system_container]·storage.snapshot.manage
                    Payload {snapname} — POST …/snapshot
                    Risk medium·RequiresApproval true
pve.guest.config    [compute.vm, compute.system_container]·compute.config.apply
                    Payload {cores?, memoryMB?} — 화이트리스트(E-5) 외 키 거부
                    PUT …/config — Risk high·RequiresApproval true
공통 RequiredPermission: infra:pve:guest:operate (E-2 승인분 — 1종 공유)
Redaction: 성공 detail 허용 필드 타입 명시(k8s restartResultRedaction 선례) —
  {node, vmid, action, upid?, taskStatus, exitStatus}
```

- power의 4 액션을 1 opdef로 묶은 근거: k8s restart 1종 선례·승인 UX(동일 승인자 인구)·액션은 Payload 검증으로 통제. start는 파괴성 낮음·stop/shutdown은 서비스 영향 — 리스크 세분화 필요 시 이월(가정 A8).
- URN 파싱: `urn:proxmox:{ctx}:(vm|system_container):{node}/{vmid}` → 게스트 타입(QEMU/LXC API 경로 분기)·node·vmid 추출(k8s parseWorkloadURN 선례).

### J8 — mock PVE 엔드포인트 (테스트 하니스 전용 — r1.1 한정)

- **위치**: 단위·계약·회귀 테스트의 측정 기구일 뿐 게이트 증명 대체가 아니다(r1.1 — 게이트 (a)는 전부 실엔드포인트). mock이 게이트 아티팩트를 산출하지 않는다.
- Go `httptest` — `mockProxmox`가 PVE API2 JSON 응답 구조 재현: `/api2/json/cluster/status`(cluster행+node행 2-3·오프라인 멤버 1 포함)·`/version`·`/nodes`·`/nodes/{n}/qemu|lxc`(시드 게스트 — nil 포인터 시나리오 포함, p4 R8 교훈)·`/storage`·`/network`·mutation POST(모드: 즉시 완료 → data:null / UPID 반환 / 동기 에러)·`/nodes/{n}/tasks/{upid}/status`(running→OK·OK→exitstatus 에러 전이).
- 토큰 헤더 검증 포함(모의 자격 리터럴 → secret-scan allowlist 사전 명시 — p4 claim 14 형식 승계).
- 엔드포인트 주입: `Connection.Endpoint`(계약 기존 필드 — 테스트·실엔드포인트 공통 경로라는 점이 하니스의 가치: mock을 통과한 클라이언트 코드가 실엔드포인트에서 동일 경로로 동작).

### J9 — R2 v3: 게이트 (d) 증명기구 확장

- `R2_PATTERN`·`R2_BRANCH_PATTERN`에 proxmox 추가(각 1곳씩 패턴 문자열 수정 — 스크립트 2라인). 어휘 예외 장치(파일·심볼 핀포인트·_test.go 제외)는 그대로 재사용 — provider_type.go의 proxmox 승격이 M1ProviderTypeNames/ReservedProviderTypeNames 라인 안에서 일어나므로 예외가 자동 커버(선언은 통과·분기는 FAIL — J8-c 장치 승계).
- 카나리(E-4): `inventory/sync.go` 제품 코드에 `if conn.ProviderType == "proxmox" {}` 플랜트 → FAIL 발화 + _test.go 플레인트 통과(제외 규칙 자기 검증).

### J10 — UI: 기존 뷰 재사용, 변경 2라인급

- ComputeInventory.vue `kindOptions`에 `compute.hypervisor_node`·`compute.system_container` 추가(63행 — 필터 선택지 확장). 테이블·드로어·정규화/Raw 탭은 kind 무관 렌더(§0.6 실측).
- 오퍼레이션 UI: ResourceOperations.vue가 opdef 등록 즉시 PVE 오퍼레이션 노출(서버 산출 — UI 권한 필터링 없음은 기존 설계). plan 다이얼로그·승인·태스크 상세 전부 기존 회로.
- 메뉴·라우트·신규 뷰 0. i18n 신규 키는 kindOptions 라벨이 이미 kind 문자열을 그대로 쓰는지 구현 시 확인 — 문자열 그대로면 키 0건(가정 A9).

---

## 2. 수정 대상 (배치표 — 전수)

### 신규 (12)

| # | 파일 | PR | 요약 |
|---|---|---|---|
| N1 | `backend/internal/infra/adapter/proxmox/client.go` | 31a | PVE API2 HTTP 클라이언트 — PVEAPIToken 헤더 조립·`background_delay` 파라미터·페일오버 비대칭(쓰기 타임아웃 증거 불가·읽기·연결류만 임계 집계)·/cluster/status 후보 재조회(오프라인 멤버 포함)·reverse-proxy 플래그 시 전환 금지·TLS posture |
| N2 | `backend/internal/infra/adapter/proxmox/upid.go` | 31a | UPID 파싱(`UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:` — node 추출)·핸들 인코딩/디코딩 `upid\|<connUID>\|<node>\|<rawUPID>`(Phase B에서 클라이언트와 동시 착지 — executor는 Phase D가 소비) |
| N3 | `backend/internal/infra/adapter/proxmox/adapter.go` | 31a | BaseAdapter+Discoverer — Descriptor{Type:"proxmox", ContextKinds:["cluster"]}·섹션 커서 디스커버리·Validate(/version)/Health(/cluster/status) |
| N3b | `backend/internal/infra/adapter/proxmox/executor.go` | 31b | OperationExecutor+TaskPoller — URN 파싱·Payload 검증·dual-mode 3분기(null/UPID/동기 에러)·Poll(§3.2 — k8s adapter.go+executor.go 분리 선례) |
| N4 | `backend/internal/infra/adapter/proxmox/normalizer.go` | 31a | PVE 응답 → DiscoveredResource — 기존 kind 어휘 재사용(J3)·URN 빌더·클러스터 쿼럼/노드 온라인 상태 매핑 |
| N5 | `backend/internal/infra/adapter/proxmox/mapping.md` | 31a | 매핑 표 — legacy 집합 공백 명시(§25 zero-prior-code)·PVE 필드 → Normalized 처분 전수·kind/Subtype 표 |
| N6 | `backend/internal/infra/adapter/proxmox/discovery_test.go` | 31a | mockProxmox(J8) + contracttest.RunContractSuite 4단얫 + 노드 다운 시나리오(결번 리소스·tombstone 부재)·**standalone /cluster/status 시나리오(cluster행 없음 — A11 폴백 단얫)**·redaction 카나리 |
| N7 | `backend/internal/infra/adapter/proxmox/executor_test.go` | 31b | UPID dual-mode 3경우(null→단일 시도 성공·UPID→폴 수렴·동기 에러→실패)·핸들-connUID 정합·페일오버 비대칭 단얫·power 액션 4종·config 화이트리스트 거부·토큰 2종 분리(inventory 자격으로 실행 거부) |
| N8 | `backend/main_register_pve.go` | 31c | `register-pve` 서브커맨드(J6 — 커넥션+컨텍스트+SecretRef 2+바인딩 2 upsert·시크릿 env 우선) |
| N9 | `web/e2e/slice-c.spec.js` | 31c | 시드 PVE 리소스(compute.vm·system_container·hypervisor_node) 렌더·상세 드로어·오퍼레이션 목록 노출 |
| N11 | `backend/internal/infra/adapter/proxmox/client_test.go` | 31a | mockProxmox 기본 서버·클라이언트 계약 — 토큰 헤더 조립·background_delay·페일오버 비대칭 4시나리오·UPID 파싱·**모든 error 문자열에 `PVEAPIToken=` 접두 부재 단얫**(r1.4 MEDIUM-9 — grep 허점 폐쇄).(r1.4 MEDIUM-5: Phase B 검증이 이미 이 파일을 소유로 명시했으나 배치표 누락이었음 — 보완 등재) |
| N10 | `v2-phase1/pve-provisioning.md` | 31c(운영자산) | 실엔드포인트 프로비저닝 런북 — 토큰 2벌 생성 가이드(PVE 권한 분리)·**OPS 토큰 축소 권한 경로(r1.4 HIGH-3: 전용 사용자 예 opsadmin@pam + PVEVMAdmin을 /nodes 하위 한정 — 현재 privsep0 전권은 사용자 확정 현실값이나 런북이 축소 경로를 제시·운영 권고)**·register-pve 절차·검증 명령·**롤백 SQL 전체 순서(§11 보강분 — task_event→…→커넥션 + 권한 行)**·**사설망 전용 전제**(비-RFC1918 엔드포인트 + --insecure-tls 조합 경고 — r1.4 MEDIUM-8)(§E-1 제공 계약 준수 — **예시 값은 전부 플레이스홀더, 실접속정보 기입 금지**. p3 k8s-fixture/README 선례) |

### 수정 (11)

| # | 파일 | PR | 요약 |
|---|---|---|---|
| M1 | `backend/internal/infra/contract/provider_type.go` | 31a | M1ProviderTypeNames에 "proxmox" 추가·ReservedProviderTypeNames에서 제거(판정 J2 — R2 어휘 예외 심볼 라인 안) |
| M2 | `backend/internal/infra/contract/capability.go` | 31b | M1CapabilityVocabulary 3종 추가(compute.power.manage·storage.snapshot.manage·compute.config.apply — OwnerPhase "Phase5")+CapabilityInterfaceMap 3행(OperationExecutor 필수·TaskPoller 옵션) |
| M3 | `backend/internal/infra/compose/compose.go` | 31a/31b | 31a: proxmox 어댑터 등록 + 읽기 capability(`inventory.full` 1종 — ResourceKinds는 디스커버리 종 전수 선언). 31b: mutation capability 3종 + opdef 3종(J7) |
| M4 | `backend/internal/infra/compose/compose_test.go` | 31a | 등록 수 단얫 갱신(기존 단얫의 want 목록에 proxmox 추가 — p4 M2 명시적 예외 선례 동일 취급) |
| M5 | `scripts/check-arch-boundary.sh` | 31b | R2 v3 — R2_PATTERN·R2_BRANCH_PATTERN에 proxmox 추가 2라인(r1.4 MEDIUM-6 경로 정정: 스크립트는 repo root `scripts/` 소재 — `backend/scripts/` 아님) |
| M6 | `.github/workflows/v2-ci.yml` (arch-boundary-canary) | 31b | proxmox 코어 분기 플랜트 추가(제품+_test 양측 — E-4) |
| M7 | `backend/main.go` | 31c | dispatch에 register-pve 1블록(기존 서브커맨드 관례) |
| M8 | `backend/store/seed.go` | 31c | **신규 시더 함수(r1.4 HIGH-1 재계약)** — (1) Menu 행 생성(value=infra:pve:guest:operate·menu_type 3·menu_status 1·라우트 권한 메뉴 형상 준용) (2) super-admin role_menus 부여. 전부 멱등 upsert·기존 시더 함수 무변경. 근거: grantRoutePermissionMenus가 opdef 어휘 한정이라 신규 권한은 Menu 행 자체가 없어 전원 403(seed.go:531·permission.go:64 실증) |
| M9 | `web/src/views/infra/ComputeInventory.vue` | 31c | kindOptions 2항 추가(hypervisor_node·system_container) — 필요 시 infra-i18n 키 병행 |
| M10 | `.gitignore` | 31a | `backend/data/` 1라인 — **메인 세션 완료(r1.3: .gitignore:25 반영·`git check-ignore` exit 0 실증)·PR 31a 편입만 남음**. 원 결함: `p5-proxmox.env` 커밋 노출(§E-1 계약 2의 강제 장치) |
| M11 | `scripts/secret-scan.py` | 31a | mock PVE 토큰 리터럴 allowlist 사전 명시(p4 claim 14 형식 승계 — value∥path glob·테스트 경로 한정). r1.4 MEDIUM-5: 부수 문단에만 있어 보존 제약 10("배치표 외 금지")과 자기모순이었음 — 배치표 행으로 승격 |

부수 갱신: r1.4 MEDIUM-5로 `scripts/secret-scan.py` allowlist는 배치표 M11행으로 승격(이 문단 소유 아님). route-inventory 골든·migration step·sensitive-routes **변경 0**(라우트·스키마·non-GET 신규 없음 — 게이트 (c)의 diff 증명이 그대로 빈 diff로 성립).

총산: 신규 12 + 수정 11(유니크). 기존 v1 코드 수정 0(service·controller·model·internal/domain 무접촉). internal/tasks·internal/api/v2·internal/infra/{inventory,secrets,model,migrate,registry,policy,metrics} **전부 무수정** — Slice B 기준의 핵심 주장(claim 3·4가 기계 증명).

---

## 3. 인터페이스 계약 (스펙 어휘의 정확한 코드화)

### 3.1 어댑터 공개면 (PR 31a — 계약 변경 0)

```go
// adapter/proxmox/adapter.go
const ProviderName = "proxmox"   // Prometheus provider 라벨 + §7.1 어휘(승격분)
type Adapter struct { metrics *metrics.Counters; pageSize int }
func NewAdapter(opts ...Option) *Adapter          // WithCounters·WithPageSize(하네스)
func (*Adapter) Descriptor() contract.ProviderTypeDescriptor
    // {Type:"proxmox", AdapterVersion:"1", ProtocolVersion:"1",
    //  ContextKinds:[]string{"cluster"}, BuiltIn:true,
    //  ConfigSchema: func() contract.ConfigSpec { return contract.ConfigSpec{} }}  // 빈 spec —
    //  aliyun/tencent Descriptor와 동일 형상 승계(r1.4 LOW-13: 스케치 누락 보기)
func (*Adapter) Close() error { return nil }   // 무상태 — 장수 상태 없음(클라우드 어댑터 승계)
func (a *Adapter) Validate(ctx, conn contract.ConnectionView) error
    // Material["inventory"] JSON 파싱 + GET /version (realm·클러스터명은 /cluster/status)
func (a *Adapter) Health(ctx, conn contract.ConnectionView) contract.HealthResult
    // GET /cluster/status — Message: 클러스터명·노드 수·quorum 상태(토큰 물질 미포함)
func (a *Adapter) Discover(ctx, req contract.DiscoverRequest) (contract.DiscoverPage, error)
    // 자격: req.Connection.Material["inventory"] JSON {tokenUser,tokenID,tokenSecret}
    //       파싱 실패/빈 값 → "missing credential material"(k8s/클라우드 문언 승계)
    // 커서: "<section>|<node>|<index>" — 섹션 고정 순서(J3)·페이지 상한 pageSize
var _ contract.BaseAdapter = (*Adapter)(nil); var _ contract.Discoverer = (*Adapter)(nil)
```

- 에러 매핑: PVE 5xx/501→`ProviderSignalError{SignalUnreachable}`·권한(401/403 — 토큰 권한 부족)→`SignalPermissionDenied`·쓰로틀 →`SignalRateLimited`(하네스 단얫 2와 1:1).
- Raw/Normalized redaction: Raw에서 토큰 헤더·자격 성분 제외(응답 본문에는 없으나 마커 카나리로 하네스가 단얫)·Normalized는 식별·용량·상태만. Raw 64KiB 상한(p4 A12 승계).

### 3.2 executor (PR 31b)

```go
// adapter/proxmox/executor.go (N3b — k8s adapter.go+executor.go 분리 선례 승계)
func (a *Adapter) Execute(ctx, req contract.OperationRequest) (contract.OperationHandle, error)
    // Material["operations"](RW 토큰) — inventory 자격이면 즉시 에러(자격 분리 단얫)
    // OperationName: pve.guest.power|pve.guest.snapshot|pve.guest.config 유일 수용
    // ResourceURN 파싱 → (guestType qemu|lxc, node, vmid)
    //   방어 단얫(r1.4 HIGH-3): vmid은 숫자 전체(^[0-9]+$)·node는 ^[a-zA-Z0-9_-]+$
    //   문자셋 검증을 통과한 뒤에만 경로 조립 — executor 버그·변조 URN의 경로 오염
    //   (경로 순회·인접 리소스 침범)을 조립 직전에 차단. 검증 실패는 즉시 에러(재시도 없음)
    // POST/PUT + background_delay → null|UPID|에러 3분기(판정 J4)
func (a *Adapter) Poll(ctx, req contract.PollRequest) (contract.OperationStatus, error)
    // 핸들 디코딩 → UPID node로 GET /nodes/{node}/tasks/{upid}/status
    // req.Connection.UID == 핸들 connUID 정합 가드(k8s Poll 승계)
var _ contract.OperationExecutor = (*Adapter)(nil); var _ contract.TaskPoller = (*Adapter)(nil)
```

- 성공 detail(redaction 허용 필드와 1:1): `{node, vmid, guestType, action?, snapname?, upid?, exitStatus, cores?, memoryMB?}`.

### 3.3 등록 CLI (PR 31c)

```text
ops-admin register-pve --name <pve-cluster> --endpoint https://192.168.250.43:8006
  [--token-user root@pam] --ro-token-id <> --ops-token-id <>
  (시크릿 2종은 env OPS_ADMIN_PVE_RO_TOKEN_SECRET / OPS_ADMIN_PVE_OPS_TOKEN_SECRET 우선 —
   셸에서 `source backend/data/p5-proxmox.env` 후 실행(§E-1 계약 2·유일한 파일 보관처),
   미설정 시 --ro-token-secret/--ops-token-secret 인자 폴백)
  [--reverse-proxy] [--insecure-tls]
  (r1.4 MEDIUM-8: --insecure-tls는 사설망(RFC1918) 엔드포인트 전제 — 비-RFC1918 호스트와
   조합하면 CLI가 경고 출력. TLS 검증 우회가 인터넷 경로에 열리는 것을 차단하는 방어선)
  → 커넥션(provider_type=proxmox·ConfigJSON{deployment_mode?,tls_insecure?}·source 없음)
  → 컨텍스트(Kind="cluster"·ExternalID=<cluster name>·MetadataJSON{nodes:n,quorum:bool})
  → SecretRef 2건(v2 봉인 JSON·KeyID 기록) → 바인딩 2행(inventory·operations)
  멱등: --name 기준 upsert(재실행 시 자격·메타 갱신 — k8s 백필 멱등 관례)
  이후: ops-admin sync-inventory --connection <uid> (기존 CLI — 무변경)
```

### 3.4 어휘·등록 (PR 31a/31b — compose)

```go
// compose.go (31a)
px := proxmox.NewAdapter(proxmox.WithCounters(counters))
reg.RegisterProviderType(px.Descriptor(), px)   // J2 승격 후 V1 통과
reg.RegisterCapabilities("proxmox",
    contract.Capability{Name:"inventory.full", Version:"1",
        ResourceKinds: proxmox.DiscoveryKinds, ReadOnly:true})
// compose.go (31b) — executor 구현과 같은 Phase(V5 요구)
reg.RegisterCapabilities("proxmox", /* compute.power.manage·storage.snapshot.manage·
    compute.config.apply — ResourceKinds [compute.vm, compute.system_container] */)
reg.RegisterOperation(powerOperation); /* snapshot·config 동형 */
```

---

## 4. 임계경로 식별 (직렬) + 시퀀스 제약

```text
J2 어휘 승격(M1·테스트) ─→ PVE 클라이언트·UPID(N1·N2) ─→ 디스커버리(N3·N4·N5·N6·M3/M4 31a분)
                                        │
                        executor·opdef(N7·M2·M3 31b분) ─→ R2 v3·카나리(M5·M6)
                                        │
                        등록 CLI(N8·M7·M8) ─→ UI·E2E·종결(M9·N9)
```

- 최장 직렬: 어휘 → 클라이언트 → 디스커버리 → 실행자 → CLI → E2E. R2 v3은 31b 착구간 병렬 가능(opdef 등록 전 확정이 카나리 단얿 조건일 뿐).
- **접속정보는 확보 완료(r1.3 — E-1 종결)·임계경로 밖의 대기 조건 소멸** — 코드 전량을 mock 하니스 검증으로 완성하는 즉시 게이트 실행(register-pve 내장 Validate 스모크→sync. p4 Path R/M 판정의 승계·r1.1부터 게이트 경로는 실엔드포인트 단일).
- **시퀀스 제약**: 없음(신규). main_sync.go·main_compare.go 무수정(§0.7)이므로 p4 §4 freeze류 머지 제약이 발생하지 않는다. 단 E-2 시드·어휘 승격은 각각 승인 후 착수.

## 5. Phase 분해 (≤5파일·독립 검증 단위 — read-only → guarded ops 순서)

| Phase | 소관 | 파일 (≤5) | 검증 |
|---|---|---|---|
| **A (PR 31a·커밋1)** | 어휘 승격·픽스처 교체 | M1 + `contract_test.go` 갱신(집합 단얫) + `registry/registry_test.go` 갱신(**F2: TestRegisterProviderRejectsReservedAndUnknown의 reserved 픽스처 proxmox→vcenter 교체 — 승격 시 t.Fatal 발화 방지**) — 3파일(r1.4 LOW-11: `provider_type_test.go` 제외 — 실측 결과 이 파일은 ProviderTypeAliases 단얫만 담고 승격 무영향. M10은 메인 완료분이라 파일 목록 외) | `go test ./internal/infra/contract/ ./internal/infra/registry/ -race` — proxmox가 M1 어휘·reserved 제거·등록 가능성 단얫 GREEN·reserved 거부 단얫이 vcenter 픽스처로 유지 GREEN·`git check-ignore backend/data/p5-proxmox.env` exit 0(M10 메인 완료분의 회귀 방지 단얫 — 유지) |
| **B (PR 31a·커밋2)** | PVE 클라이언트·UPID | N1 + N2 + N11(`client_test.go`) — 3파일 | `go test ./internal/infra/adapter/proxmox/ -run 'Client|UPID|Failover' -race` — 토큰 헤더 조립·UPID 파싱 전성분·페일오버 비대칭(쓰기 타임아웃 전환 부재·읽기 연속 실패 전환·오프라인 멤버 IP 후보 보존·reverse-proxy 고정)·**error 문자열 `PVEAPIToken=` 부재 단얫** GREEN |
| **C (PR 31a·커밋3)** | read-only 디스커버리 | N3 + N4 + N5 + N6 — 4파일 | `go test ./internal/infra/adapter/proxmox/ -race` — 하네스 4단얫·섹션 커서 종결·오프라인 노드 결번·tombstone 부재 |
| **C2 (PR 31a·커밋4)** | compose 등록 | M3(31a분 등록) + M4 — 2파일 | `go test ./internal/infra/compose/ -race` — proxmox 등록 단얫·읽기 capability·기존 단얫 무회귀(등록 수 want 갱신은 이 커밋 소유) |
| **D (PR 31b)** | guarded operations | N3b(executor.go) + N7 + M2 + M3(31b분) — 4파일 | `go test ./internal/infra/adapter/proxmox/ ./internal/infra/compose/ -race` — UPID dual-mode 3경우·자격 분리·config 화이트리스트 거부·opdef 등록 V5/V6 통과 |
| **E (PR 31b·커밋2)** | R2 v3·카나리 | M5 + M6 + 카나리 플레인트 검증 — 2파일 | `bash scripts/check-arch-boundary.sh` 전 PASS·카나리 잡 proxmox 플레인트 FAIL 발화(_test 플레인트 통과) |
| **F (PR 31c·커밋1)** | 등록 CLI·시드·런북 | N8 + M7 + M8 + N10 — 4파일 | mock 엔드포인트 대상 `register-pve` 실행 → DB 5행(커넥션·컨텍스트·SecretRef 2·바인딩 2)·멱등 재실행·권한 시드 부여 단얫·런북의 플레이스홀더 전용 단얫(secret-scan 사전 통과) |
| **G (PR 31c·커밋2)** | UI·E2E·종결 | M9 (+infra-i18n 필요분) + N9 + 게이트 아티팩트 — 3파일 | `bun run build`·파리티·slice-c Playwright passed·게이트 실행 단계(접속정보 확보 완료 — r1.4 LOW-12 정정) 실엔드포인트 Validate/Health→register-pve→sync→리소스 노출(claim 10) |

순서: A→B→C→C2→D→E→F→G. E는 D 직렬(카나리가 등록 코드 요구). F는 D와 병렬 가능(C2 이후 — 등록 CLI가 디스커버리 종점을 소비). read-only(C/C2 머지)가 guarded ops(D)에 선행 — §14.3 Sequence 문언 승계.

## 6. 테스트 계약 (TDD — §23)

```text
계약(contracttest)  proxmox RunContractSuite — 페이징(섹션 커서 종결·무중복·총수==시드)·
                    에러 분류(신호 3종)·rate-limit 카운터·redaction 카나리
유닛+통합           UPID 파싱(전 성분·변형 거부)·핸들 인코딩 왕복·normalizer(URN 유니크·
                    nil-safe — PVE 응답의 포인터 필드·오프라인 노드 IP 보존)·
                    클라이언트(토큰 헤더·background_delay 파라미터·페일오버 비대칭 4시나리오)·
                    executor(dual-mode 3경우·폴 전이 running→OK/에러·자격 분리 거부·
                    config 화이트리스트 외 키 400·poll-connUID 정합)·등록 CLI(upsert 멱등·
                    시크릿 env 우선·바인딩 2행 목적 분리·standalone /cluster/status 폴백 A11)
엔진 회귀           internal/tasks 무수정의 기계 확인 — claim 4(빈 diff)
E2E                 slice-c.spec.js — 시드 PVE kinds 3종 렌더(kindPrefix)·상세 드로어·
                    오퍼레이션 목록(opdef×kind 교집합에 pve.guest.power 노출)
음성                코어 proxmox 분기 플레인트(제품) FAIL·_test 플레인트 통과·
                    inventory 자격으로 Execute 거부·config 미허용 필드 거부·
                    reverse-proxy 시 후보 전환 부재·쓰기 타임아웃 후보 전환 부재
```

기존 테스트 무변경 — 신규 파일·기존 파일 추가 단얫만. **명시적 예외 2건(골든 재생성과 동일 취급·커밋 근거)**: (a) compose_test.go 등록 수 단얫 want 목록 갱신(어댑터 등록의 필연 — p4 M2 선례), (b) contract 어휘 테스트의 집합 단얫 갱신(승격의 필연).

## 7. 검증 요구 (claims — ④리뷰 주입용·로컬 명령)

1. `cd backend && go test ./internal/infra/adapter/proxmox/ -race -count=1` — 하네스 4단얫+실행자 계약 GREEN.
2. `cd backend && go test ./... -race -count=1` — 전 패키지 GREEN(기존 회귀 0).
3. (게이트 c) `git diff main -- backend/internal/infra/model/ backend/internal/infra/migrate/ backend/internal/infra/inventory/ backend/internal/infra/secrets/ backend/internal/tasks/ backend/internal/api/v2/` — **빈 diff**(공유 스키마·코어 패키지 무변경 — §25 Slice B (c)·(a)의 기계 증명).
4. (게이트 d) `bash scripts/check-arch-boundary.sh` — R1/R2 v3/R3 전부 PASS(코어 제품 코드에 proxmox 식별자 0 — 어휘 예외 라인·_test.go 제외만 통과).
5. (게이트 d 반증) arch-boundary-canary — proxmox 코어 플랜트(제품 코드) FAIL 발화 + _test 플레인트 통과 로그. **로컬 재현 절차(r1.4 LOW-14 — §10 로컬 판정 정합)**: 워크트리 스크래치 카피에서 `internal/infra/inventory/sync.go`(제품 코드)에 `if conn.ProviderType == "proxmox" {}` 플랜트 삽입 → `bash scripts/check-arch-boundary.sh` R2 FAIL 발화 확인 → 같은 플랜트를 `_test.go`에 삽입 → 통과 확인(제외 규칙 자기 검증) → 카피 폐기. CI 잡 로그는 원격 증거로 병행(대기하지 않음).
6. (UPID dual-mode) `go test ./internal/infra/adapter/proxmox/ -run 'DualMode|Poll' -v` — null 반환 단일 시도 성공·UPID 반환 폴 수렴·동기 에러 실패 3경우의 === RUN 출력.
7. (자격 분리) executor_test 출력 — inventory Material로 Execute가 에러·operations Material만 수용·`SELECT purpose, COUNT(*) FROM provider_credential_binding GROUP BY purpose`에 PVE 커넥션의 inventory·operations 2행(등록 후).
8. (게이트 b 전제) `cd web && bun run build` GREEN + `node scripts/check-i18n-parity.mjs`(스크립트는 **repo root `scripts/` 소재 — r1.3 F6 정정: `cd web` 하위 경로 아님**, 루트에서 실행) — GREEN.
9. (게이트 b) `cd web && bun run e2e` — slice-c passed(시드 PVE 3종 kinds 렌더·오퍼레이션 목록). 백엔드 `curl '.../api/v2/infra/resources?kindPrefix=compute.'` items>0.
10. (게이트 a — 실엔드포인트) 선행 curl 200 검증은 메인이 종결(r1.3 — 재수행 불요). `register-pve`(§E-1 제공 계약 — 시크릿은 `backend/data/p5-proxmox.env` source 후 env 전달·내장 Validate가 /version+/cluster/status 스모크) → `go run . sync-inventory --connection <uid>` → 리포트 outcomes·`GET /api/v2/infra/resources?kind=compute.vm` items>0·`SELECT COUNT(*) FROM provider_connection WHERE provider_type='proxmox'` = 1. **게스트 0 실측 대응(r1.3)**: 현재 qemu/lxc가 0이라 `kind=compute.vm items>0`의 전제가 성립하지 않는다 — 증명은 택일: (a) 사용자가 테스트 게스트 1개 생성(qemu 우선) 후 원문안대로 (b) 게스트 없이 진행 시 **증명 종을 대체** — `kindPrefix=compute.`·`kind=compute.hypervisor_node`·`kind=storage.pool`의 items>0으로 discovery·UI 렌더를 증명하고 `compute.vm`·`compute.system_container` 열은 게스트 생성 시점에 보완 종결(게이트의 본질은 계약·UI·diff·분기 증명이지 특정 종의 존재가 아님 — §25 문언 정합). 실행 경로(31b) 검증은 실게스트 생성 후 claim 6·7로 별도 종결.
11. (권한 — E-2 승인분·r1.4 HIGH-1 기능 단얫 보강) (a) 어휘: `grep -rn "infra:pve:" backend/internal/infra/compose/compose.go backend/store/seed.go` — opdef 3종의 RequiredPermission 1종·시더 함수 존재. (b) 라우트: `git diff main -- backend/opdef/` — 빈 diff(대표 권한 무변경). **(c) 기능(핵심 — 라우트 어휘 무변경이라도 Menu 행 없으면 전원 403)**: 시더 1회 실행 후 `SELECT COUNT(*) FROM sys_menu WHERE value='infra:pve:guest:operate' AND menu_status=1` ≥1·super-admin의 부여 확인(`SELECT COUNT(*) FROM sys_role_menu rm JOIN sys_role r ON r.id=rm.role_id JOIN sys_menu m ON m.id=rm.menu_id WHERE r.role_key='super-admin' AND m.value='infra:pve:guest:operate'` ≥1)·시더 재실행 시 행 수 불변(멱등). executor 라우팅 성공은 claim 6·7이 커버(자격 분리·dual-mode 단얫이 권한 통과 이후 경로 증명).
12. `git diff main -- backend/go.mod backend/go.sum` — 빈 diff(신규 의존 0).
13. (자격 비로그) `grep -rn "tokenSecret\|tokenUser" backend/internal/infra/adapter/proxmox/ backend/main_register_pve.go --include="*.go" | grep -iE "log\.|print|fmt\.print"` — 0행. **보강(r1.4 MEDIUM-9)**: grep은 로그 호출 라인만 잡는다 — 하니스가 **모든 error 문자열·ProviderSignalError.Message·OperationStatus.Detail에 `PVEAPIToken=` 접두 부재를 단얫**(N11 계약 — 에러 래핑 과정에서 자격이 문자열에 묻어 들어가는 경로의 폐쇄. grep 허점의 테스트측 보완).
14. `python3 scripts/secret-scan.py; echo $?` — exit 0(모의 토큰 리터럴 allowlist 사전 명시분 포함).
15. `cd backend && go test ./internal/infra/adapter/proxmox/ -cover` — 신규 패키지 커버리지 ≥80%.
16. (VNC 콘솔 부재 — §12.2) `grep -rniE "vnc|spice" backend/internal/infra/adapter/proxmox/ web/src/views/infra/` — 0행.
17. (mapping.md 처분 완결) `go test ./internal/infra/adapter/proxmox/ -run Mapping` — mapping.md ↔ normalizer 동치·legacy 집합 공백 명시 단얫(p4 11필드 처분 완결 형식 승계).
18. (게이트 a 증명 경로 단일성 — r1.1) `grep -rn "interim" backend/internal/infra/adapter/proxmox/ backend/main_register_pve.go v2-phase1/pve-provisioning.md` — **0행**(interim 마킹 경로 폐기 — 게이트 증명은 실엔드포인트 단일 경로).

## 8. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **v1 무변경**: `backend/service/**`·`backend/controller/**`·`backend/model/**`·v1 라우트 그룹·`internal/domain/**` 무접촉. PVE는 기존 v1 코드가 전무하다(실측 §0.1) — 발견 즉시 계획 이탈로 보고.
2. **엔진·코어 회로 무변경**: `internal/tasks/**`·`internal/api/v2/**`·`internal/infra/{inventory,secrets,model,migrate,registry,policy,metrics}/**` 수정 금지(claim 3 빈 diff). UPID dual-mode는 기존 executeClaimed/pollAsyncAttempts 회로의 계약 준수로 완성 — 엔진 변경 요구가 발견되면 설계 실패로 에스컬레이션.
3. **코어 스키마·분기 금지 (§25)**: 마이그레이션 step 신설 금지·모델 변경 금지. 코어 패키지 제품 코드(_test.go 제외)에 proxmox 식별자 분기 추가 금지 — R2 v3가 거부. 수정 허용 코어 파일은 `contract/provider_type.go`(어휘 승격 2라인)·`contract/capability.go`(어휘 3종+매핑 표 3행)·`compose/compose.go`(등록·opdef)뿐 — 배치표와 동일 집합.
4. **신규 런타임 의존성 0**: `backend/go.mod`·`go.sum` 무변경(§10.2). PVE는 직접 HTTP(토큰 헤더·JSON 디코드 — 표준 라이브러리).
5. **기존 테스트 무변경**: 기존 단얫·기대값 변경 금지 — 신규 파일·추가 단얫만. 명시적 예외 3건: (a) compose_test.go 등록 수 want 갱신(어댑터 등록의 필연), (b) contract 어휘 집합 단얫 갱정(승격의 필연), **(c) `registry/registry_test.go:139` reserved 거부 픽스처 proxmox→vcenter 교체(r1.3 F2 — proxmox가 M1에 승격되는 순간 "reserved로 거부된다"는 단얫 자체가 모순이 되는 필연. vcenter는 여전히 reserved라 단얫의 의미를 보존한다)** — 전부 커밋 메시지에 근거.
6. **어댑터 결계 (arch rule 2)**: 어댑터는 req.Connection만 읽는다 — DB·브로커·service import 금지. 자격은 Material["inventory"]/["operations"] JSON blob으로만. 후보 노드 목록의 어댑터 인스턴스 캐시 금지(요청마다 /cluster/status 재조회).
7. **자격 물질 보호**: 토큰 시크릿·브로커 평문은 로그·감사·task_event·API 응답·테스트 출력 미포함(claim 13). SecretRef 외 신규 저장 위치 금지. 등록 CLI는 env 우선·인자 폴백. **파일 보관처는 untracked `backend/data/p5-proxmox.env` 1곳뿐**(r1.2) — M10(.gitignore)이 커밋 노출을 봉쇄하며 그 전에 어떤 커밋도 `backend/data/`를 포함하지 않는다(git add 시 경로 확인 의무).
8. **CI 기존 10잡 무변경**: v2-ci.yml 수정은 arch-boundary-canary의 proxmox 플레인트 추가만(E-4). merge 판정은 로컬.
9. **게이트 자산 보호**: kind 클러스터 v2-p2·phase2 비교 아티팩트·클라우드 게이트 자산 무변경. main_sync.go·main_compare.go 무수정(§0.7 — 본 Phase 소유 아님).
10. **배치표 외 금지·VNC 금지**: §2 배치표 외 파일 착수 금지. VNC/SPICE 콘솔 구현 금지(§12.2 — 커넥터 에이전트 ADR 이월). config 전체 필드 개방 금지(E-5 화이트리스트만).

## 9. 리스크 매트릭스 (가능성 × 파급) + 완화

| # | 리스크 | 가능성 | 파급 | 완화 |
|---|---|---|---|---|
| R1 | 게이트 (a) 종결이 코드 완성 시점에 의존(자격·엔드포인트는 충족 — 남은 병목은 구현 진행뿐) | 낮음(r1.3 — 진입 전제 전량 충족) | 중 | 코드 전량을 mock 하니스 검증으로 완성 후 §13-1 절차 즉시 실행(register-pve 내장 Validate가 첫 스모크 겸용 — F8)·상태계약 동기 규칙(r1.3 헤더)로 계획-state 일치 유지 |
| R2 | PVE API 응답 형상 가정 오류(실엔드포인트에서만 발현 — /api2/json 래핑·background_delay 시맨틱) | 중 | 높음 | mock이 PVE 공개 문서 형상을 충실히 재현·실엔드포인트 증명 절차의 Validate/Health 선행 스모크(§13-1)·증류 계약 5개는 문서화된 동작 기반이라 표면 수 최소·**게스트 0 claim 10 (b)안 채택 시 실행 경로(dual-mode)의 실게스트 검증은 게이트 이후 잔류 — §13-9 이월 창구가 추적**(r1.4 HIGH-4) |
| R3 | 전원 재시도의 파괴적 이중 실행(stop→start 경합 등) | 중 | 높음 | RequiresApproval=true·MaxAttempts 2 보수·엔진 멱등키(중복 제출 차단)·이중 실행 단얫은 mock의 전이 추적으로 실증 |
| R4 | 페일오버 임계 카운터의 요청 스코프 리셋이 과도/과소 전환 | 중 | 중 | J6 stateless 절충 문서화·전환 실패 시 기존 엔드포인트 유지(전환은 최적화 not 정확성)·reverse-proxy 모드로 비활성 경로 제공·**발화 조건 한계 명시(r1.4 MEDIUM-7: Discover 다중 GET에서만 발화·전체 장애·standalone은 비발동이 정상)** — 카운터는 요청 스코프 유지 |
| R5 | R2 v3 위음성(어휘 예외 라인 남용·_test.go 제외 남용) | 낮음 | 높음(게이트 (d) 신뢰) | p4 R5 승계 — 심볼 핀포인트·카나리 양측 발화·예외 2라인 변경은 커밋 근거 |
| R6 | kindOptions 누락·오퍼레이션 UI 미노출로 게이트 (b) 논쟁 | 낮음 | 중 | claim 9 E2E가 렌더를 기계 증명·ListResourceOperations 교집합은 서버 산출 |
| R7 | PVE 토큰 권한 분리가 실제 토큰 권한과 불일치(RO 토큰이 쓰기 가능 등) | 중 | 중 | 등록 CLI Validate가 RO 토큰으로 읽기 스모크만 수행(쓰기 시도 안함 — 부작용)·operations 토큰 검증은 실행 시점 401/403 매핑·실엔드포인트 승격 절차에서 토큰 권한 확인 항목 |
| R8 | 신규 권한(E-2) 미승인·시드 누락으로 PVE 오퍼레이션 전면 거부 | 중 | 중 | E-2 계획 승인 시점 확정·claim 11 시드 부여 단얫·(b) 안(기존 권한 재사용)은 1줄 교체 가능 |
| R9 | config 화이트리스트 필드 문법 오류(cores·memory PVE 형식)로 실제 적용 실패 | 중 | 중 | E-5 승인 시점 PVE 문법 확정(가정 A6)·PUT 전 계약 테스트가 요청 파라미터 형상 단얫·실패는 동기 에러로 안전 종단 |
| R10 | slice-c E2E용 V2 시드 PVE 행 작성 오류(URN·kind 어긋남) | 낮음 | 낮음 | normalizer를 경유한 시드 생성기(테스트 헬퍼)로 우회 — 손작성 JSON 금지 |
| R11 | Phase 2 게이트 수동 창구와 PVE 작업 충돌(오염) | 낮음 | 중 | 무접촉 원칙(보존 제약 9)·본 Phase가 건드리는 DB는 V2 테이블 신규 행뿐 |

## 10. CI 로컬 판정 방침

- merge 판정: **로컬** — `go test ./... -race`·`bun run build`·본 계획 claim 명령. 원격 CI 대기 금지(p3/p4 관례 승계·기억 정책 정합).
- CI 변경은 E-4 승인분 1건(arch-boundary-canary proxmox 플레인트). slice-c E2E는 신규 CI 잡 추가 없음(로컬 판정 — slice-b와 동일 취급).
- 라이브 PVE 토큰을 CI에 두지 않는다(§23.2 "No live cloud credentials in CI, ever" 정신 승계). claim 14 secret-scan을 merge 전 필수 관문으로 편입.

## 11. 롤백 가능성 판정

- **PR 31a/31b (완전 롤백 가능)**: `adapter/proxmox/` 패키지 삭제 + compose 등록 제거 + 어휘 승격 revert(proxmox를 M1→Reserved 원복) — 코어 무영향. 지속물 없음(스키마 0).
- **PR 31c (조건부 — 데이터 잔여·r1.4 HIGH-2 롤백 SQL 보강)**: CLI·시드·UI는 revert로 코드 복귀. 잔여물은 수동 SQL 정리 — **삭제 순서(참조 방향 역순)**: `task_event` → `task_attempt` → `provider_task`(실행 이력이 있는 경우) → `resource_observation` → `infra_resource` → `inventory_sync_run` → `provider_credential_binding` → `secret_ref` → `provider_context` → `provider_connection` + 권한(`sys_role_menu` 부여 행 → `sys_menu`의 infra:pve:guest:operate 행). **infra_resource까지 지우는 이유**: 신원 unique key가 `(context_id, kind, external_urn)`이라 컨텍스트만 지우고 재등록하면 구 resource 행이 고아화되어 새 체인과 재수습(연결) 불가 — 관측·리소스·싱크런을 함께 지워야 재등록이 깨끗하다. super-admin 권한 부여 행은 시더 재실행으로는 소거 불가(upsert-only — 수동 delete).
- 게이트 실패 시 착구간 복귀: Phase 역적용(G→A). A/B(read-only)와 D(guarded ops)는 독립 롤백 가능 — read-only만 남겨두는 중단점도 유효(§5.4 부분 채택 정합).

## 12. 파일 소유권 — 구현 스폰 분리

| 스폰 | 소유 | 접근 금지 |
|---|---|---|
| impl-P31a1 (임계) | M1 + `contract_test.go` 집합 단얫 갱신 + `registry_test.go` 픽스처 교체 (Phase A — 3파일) | adapter·compose·tasks |
| impl-P31a2 (임계) | N1·N2·client_test.go (Phase B) | adapter.go·compose |
| impl-P31a3 (임계) | N3·N4·N5·N6 (Phase C) + M3(31a분)·M4 (Phase C2) | executor·opdef·main |
| impl-P31b (임계) | N3b(executor.go)·N7·M2·M3(31b분) (Phase D) | adapter.go Discoverer 본체(읽기만)·CLI |
| impl-P31b2 | M5·M6 (Phase E) | Go 소스 전부(스크립트·CI만) |
| impl-P31c | N8·M7·M8·N10 (Phase F) + M9·N9 (Phase G) | internal/infra 하위·adapter 패키지 |

교차 소유: `compose.go`(31a 등록→31b opdef 직렬 누적 — p4 M1/M2 aliyun분/tencent분 선례). `adapter/proxmox/`는 패키지 내 파일 분할 소유(디렉터리 단위 스폰 경계 유지를 위해 31a2→31a3→31b 순 직렬).

**타 레인 공유 파일 인지 (r1.4 LOW-15)**: `store/seed.go`(M8)과 `.github/workflows/v2-ci.yml`(M6)은 다른 진행 레인이 동시에 수정할 수 있는 핫 파일이다 — 충돌 시 리베이스 비용이 발생한다는 것을 인지만 한다(독점 배타 금지·컨플릭트 시 조정). seed.go는 p3 이후 모든 Phase가 시더를 확장해 온 공용 확장점이며 v2-ci.yml은 E-4 외 불변이 원칙이라 실제 충돌 표면은 좁다.

## 13. 산출 외 확인사항 (이월 대장 증보)

1. **실엔드포인트 증명 절차 (r1.3 — 진입 전제 충족 상태로 재계약·interim 없음)**: 선행 curl 200 자격 검증은 **메인 세션이 수행·종결**(RO·OPS 2벌 모두 통과 — §0.1). 잔여 절차: `register-pve`(**0단계 스모크 내장** — CLI의 Validate가 GET /version·GET /cluster/status로 도달·자격·클러스터 형상을 검증 후 등록, F8) → `sync-inventory --connection <uid>` → 게이트 (a) 종결(claim 10 — 게스트 0 대체 선택지 포함). 접속정보는 이미 확보 상태 — 코드 완성 즉시 실행 가능.
2. **VNC/SPICE 콘솔**: §12.2 커넥터 에이전트 + ADR 이월 — 재입구는 "ADR accepted AND a NAT-resident provider in scope"(§5.5). PVE가 NAT-resident 실례가 되는 시점이 ADR 착수 트리거 후보임을 기록.
3. **POST /provider-connections 등 §16.1 미구현 6종**: p3 §13-8 이월 승계 — 본 Phase는 CLI로 충족(E-3). 커넥션 수명주기 UI 수요 시 재검.
4. **PVE 스토리지·네트워크 전용 인벤토리 뷰**: discovery는 수집하나 UI는 compute 뷰만 존재(기존 공백) — kinds 도착 시 뷰 확장은 §17.1 "…kinds arrive with providers"의 잔여분. 수요 명명 시 재검.
5. **§18.2 메트릭 전면 계측**: M1 선언 준비 태스크 이월 유지(p4 §13-2 승계) — PVE 어댑터는 기존 Counters 승계만.
6. **config 오퍼레이션 전체 필드 개방·전원 액션별 리스크 세분화**: E-5 화이트리스트 외 필드·opdef 분할(power start/stop 별도 RiskLevel)은 운영 요구 시.
7. **모의 서명 3중 병존의 PVE 유사분 없음**: PVE는 v1 구현이 없어 어댑터 1벌뿐(신규) — p4 §13-11과 달리 수렴 대상 없음. 기록 생략.
8. **Power 액션의 게스트 상태 기반 거부(이미 실행 중 start 등)**: PVE가 동기 에러를 반환하면 실패 종단이 정상 경로 — 상태 사전 확인(GET 후 조건부 실행)은 요구 트레이스에 없어 미구현. 운영 UX 요구 시 재검.
9. **31b 실엔드포인트 증명 이월 (r1.4 HIGH-4)**: 게이트 ①이 claim 10의 (b)안(게스트 0 — hypervisor_node/storage.pool 대체 증명)으로 종결된 경우, 실행 경로(dual-mode·자격 분리)의 **실게스트 실증은 게이트 이후 잔류**한다. 소관: 게이트 종결 창구·트리거: 테스트 게스트 생성 시점. 내용: 게스트 생성 → claims 6·7(UPID dual-mode 3경우·자격 분리)을 실게스트에서 재실행 + qemu/lxc 디스커버리의 실엔드포인트 형상 보완 종결. R2 리스크 표의 "게이트 이후 잔류" 성격과 동일 창구에서 추적.

## 14. 가정 명세 (불확실 요소의 명시적 처리)

| # | 가정 | 검증 시점 |
|---|---|---|
| A1 | E-1 종결(r1.3): §14.3 진입 전제조건 전량 충족 — 엔드포인트 도달·토큰 2벌 검증 통과(RO·OPS). 게이트 경로는 실엔드포인트 단일·mock은 하니스만. 잔여는 게이트 실행(코드 완성 대기) | 게이트 (a) 종결 시(claim 10) |
| A2 | PVE REST 응답 래핑이 `{data: …}`(JSON API2 표준)·UPID가 응답 data 문자열·태스크 상태가 `/nodes/{node}/tasks/{upid}/status`의 `{status, exitstatus}` — §14.3 증류 계약과 PVE 공개 문서 기반 | Phase B mock 설계·실엔드포인트 승격 시 재확인 |
| A3 | `ProviderConnection.Endpoint`에 `https://host:8006` 전체 베이스(클라이언트가 `/api2/json` 접미 조립) — mock은 httptest URL 주입 | Phase B |
| A4 | background_delay 기본값·RetryPolicy(MaxAttempts 2·Backoff 10s)·페일오버 임계(연속 2회)는 계획 도입값 — 스펙이 수치를 정하지 않음 | Phase D 구현 시 상수화·주석 출처 명시 |
| A5 | SecretRef 재질 JSON 키 `{tokenUser, tokenID, tokenSecret}` — 브로커 Value 단일 문자열 계약 준수(p4 A4 승계) | Phase F |
| A6 | config 화이트리스트 초기 집합 `{cores, memoryMB}`의 PVE PUT config 파라미터 명칭·단위 — E-5 승인 시점 PVE 문서로 확정 | 계획 승인(E-5)·Phase D |
| A7 | 실엔드포인트 TLS: 핸드셰이크는 정상 실측(r1.2)이나 **신뢰 체인 여부는 미확인**(사설망 인증서 가능성) — `--insecure-tls` 플래그 유지·curl 스모크의 `-k`와 동일 posture. **r1.4 MEDIUM-8 갱신: --insecure-tls는 사설망(RFC1918) 전제 — 엔드포인트(192.168.250.43)가 조건 충족, 비-RFC1918 호스트 조합은 CLI가 경고(N10 런북 "사설망 전용" 전제와 정합)**. CA 제공 시 SecretRef 참조 경로는 재설계 대상 | register-pve 실행 시·엔드포인트 변경 시 |
| A8 | power 4액션 1 opdef·권한 1종 공유(E-2)가 승인됨 — 분할 요구 시 opdef/권한 추가는 이월 | 계획 승인 시 |
| A9 | kindOptions 라벨이 kind 문자열 그대로면 i18n 신규 키 0건 — 라벨 번역 요구 시 키 추가(파리티 게이트) | Phase G |
| A10 | 클러스터 쿼럼/HA 헬스의 표현 위치: ProviderContext.Status/MetadataJSON(리소스 행 아님) — VM 헬스와의 붕괴 금지 준수의 구체 착지 | Phase C 매핑 리뷰 |
| A11 | **(r1.3 신설 — 실측 형상)** `/cluster/status`가 standalone 노드에서 cluster행 없이 node행만 반환(§0.1 실측). 컨텍스트 ExternalID 폴백: cluster행 부재 시 유일 node행의 name + MetadataJSON `{"standalone":true}` 마커(클러스터명 허위 구성 회피·재실행 멱등 유지). 멀티노드 클러스터에서의 형상은 게스트·클러스터 확장 시 재검 | Phase C(mock 시나리오 standalone·cluster 2케이스)·실엔드포인트 sync 시점 |
