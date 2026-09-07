# Proxmox VE 프로비저닝 런북 (N10 — plan phase5 §5 F 배치)

`ops-admin register-pve`(판정 J6·§3.3)로 PVE 커넥션을 등록하는 운영 절차.
**이 문서는 절차만 기록한다 — 모든 예시 값은 플레이스홀더이며 실접속정보(호스트·토큰
ID·시크릿 값)를 기입하지 않는다**(§E-1 제공 계약 — p3 k8s-fixture/README 선례).
실제 값의 유일한 파일 보관처는 untracked `backend/data/p5-proxmox.env` 1곳이고,
봉인 후 영구적으로 실리는 곳은 런타임 DB(`provider_connection` 행 + `secret_ref`
v2 봉인 행)뿐이다.

## 0. 전제

- **사설망 전용**(r1.4 MEDIUM-8): `--insecure-tls`는 PVE 기본 자가서명 인증서를
  전제로 하며, 사설망(RFC1918 — `10.0.0.0/8`·`172.16.0.0/12`·`192.168.0.0/16`)
  엔드포인트에서만 사용한다. 비-RFC1918 호스트와 조합하면 CLI가 경고를 출력한다
  (TLS 검증 우회가 인터넷 경로에 열리는 것을 막는 방어선). CA를 제공하는 경로는
  SecretRef 참조 재설계 대상(가정 A7).
- v2 스키마(`migrate.Run`)와 시크릿 키 소스(`EnsureSecretKeySource`)가 준비된
  배포 환경 — CLI가 기동 경로와 동일한 초기화를 수행한다.
- `register-pve`는 `--name` 기준 멱등 upsert다. 재실행은 같은 체인(커넥션 1·
  컨텍스트 1·SecretRef 2·바인딩 2) 위에서 엔드포인트·메타·봉인 자격을 갱신한다.

## 1. 토큰 2벌 생성 (PVE 웹 UI 또는 `pveum`)

두 벌은 **서로 다른 SecretRef**로 봉인된다(판정 J5 — 같은 SecretRef 공유 금지,
§14.3 "read-only token for discovery, operations token gated by approval").
`<ro-token-id>`·`<ops-token-id>`·`<ops-user>`는 플레이스홀더다.

### 1-a. 읽기 전용(RO) 토큰 — discovery 전용

1. PVE 웹 UI: Datacenter → Permissions → API Tokens → Add — User `root@pam`,
   Token ID `<ro-token-id>`, **Privilege Separation ON**(기본).
2. 토큰에 감사 역할만 부여:

   ```bash
   pveum acl modify / --tokens 'root@pam!<ro-token-id>' --roles PVEAuditor
   ```

3. 발급 직후 1회만 표시되는 token secret 값을 `p5-proxmox.env`에 기록한다
   (§3 — 파일은 gitignore 봉쇄 확인 후).

### 1-b. 오퍼레이션(OPS) 토큰 — 축소 권한 경로 권장 (r1.4 HIGH-3)

**권장 경로 — 전용 사용자 + 노드 한정 PVEVMAdmin:**

1. 전용 사용자 생성: Datacenter → Users → Add — `<ops-user>@pam` (root 아님).
2. 사용자 권한을 대상 노드 하위로 한정:

   ```bash
   pveum useradd '<ops-user>@pam'
   pveum acl modify /nodes/<node> --users '<ops-user>@pam' --roles PVEVMAdmin
   ```

3. 사용자의 API 토큰 발급: Permissions → API Tokens — User `<ops-user>@pam`,
   Token ID `<ops-token-id>`, **Privilege Separation ON**.
4. 토큰에도 동일 ACL 부여(privsep ON 토큰은 자체 ACL을 갖는다):

   ```bash
   pveum acl modify /nodes/<node> --tokens '<ops-user>@pam!<ops-token-id>' --roles PVEVMAdmin
   ```

   클러스터 전체 게스트를 다뤄야 하면 `/nodes/<node>` 경로를 노드마다 반복
   부여한다(`/` 전역 부여는 축소 전제를 무너뜨리므로 하지 않는다).

**현실 대안 — 기록용, 권장 아님:** root@pam 소유 토큰을 Privilege Separation
**OFF**(privsep 0)로 발급하면 소유자 전권을 승계한다 — 게스트 오퍼레이션 3종
(power·snapshot·config)이 즉시 동작하지만 방어심도가 토큰 1줄과 같아진다.
엔진의 승인 게이트(RequiresApproval)·opdef 화이트리스트(E-5)가 응용측 방어를
제공하므로 동작 자체는 가능하나, 운영 환경에서는 1-b 권장 경로로의 이관을
권고한다. 현 단계 실측 토큰이 이 형상이다(r1.3 §0.1).

### 1-c. 토큰 보관 — `backend/data/p5-proxmox.env` (untracked)

```bash
# backend/data/p5-proxmox.env — gitignore 봉쇄 대상(§E-1 계약 2·M10).
# 값은 플레이스홀더 — 실제 값은 이 파일에만 기록한다.
export OPS_ADMIN_PVE_RO_TOKEN_SECRET='<ro-token-secret-value>'
export OPS_ADMIN_PVE_OPS_TOKEN_SECRET='<ops-token-secret-value>'
```

커밋 전 항상 확인:

```bash
git check-ignore backend/data/p5-proxmox.env   # exit 0이어야 한다
```

## 2. register-pve 실행

```bash
cd backend
set -a; source data/p5-proxmox.env; set +a   # 시크릿 2종을 env로 주입 (셸 히스토리 회피)

go run . register-pve \
  --name '<pve-connection-name>' \
  --endpoint 'https://<pve-host>:8006' \
  --token-user 'root@pam' \
  --ro-token-id '<ro-token-id>' \
  --ops-token-id '<ops-token-id>' \
  --insecure-tls            # 사설망 자가서명 전제 — 공개 경로면 사용 금지(MEDIUM-8)
  # [--reverse-proxy]       # 고정 리버스 프록시 배치 — 어댑터 노드 페일오버 비활성
```

- 시크릿은 env(`OPS_ADMIN_PVE_RO_TOKEN_SECRET`·`OPS_ADMIN_PVE_OPS_TOKEN_SECRET`)
  우선, 미설정 시 `--ro-token-secret`/`--ops-token-secret` 인자 폴백 — 인자는
  셸 히스토리에 남으므로 env 경로를 기본으로 쓴다(claim 13).
- CLI가 내장 Validate(§13-1 — GET `/version` + GET `/cluster/status`, **RO 토큰
  사용**)를 통과해야 등록이 진행된다. 실패 시 DB 기입 없이 종료(exit 1).
- 성공 출력(stdout JSON): `connectionUid`·`contextUid`·`clusterName`·
  `standalone`·`nodes`. standalone 노드(A11 — `/cluster/status`가 cluster행 없이
  node행만 반환)에서는 `standalone:true`·`clusterName`이 유일 노드명으로 온다.
- 등록되는 체인: `provider_connection`(provider_type=proxmox) 1행 →
  `provider_context`(Kind=cluster) 1행 → `secret_ref` 2행(EncryptSecretV2 봉인
  JSON `{tokenUser,tokenID,tokenSecret}`·KeyID 기록) →
  `provider_credential_binding` 2행(purpose `inventory`=RO·`operations`=OPS).

## 3. 검증

### 3-a. 싱크·리소스 노출 (claim 10 경로)

```bash
go run . sync-inventory --connection '<출력의 connectionUid>'
```

- 리포트 아티팩트: `data/sync/<connectionUid>/<날짜>/<시각>.json`
- 게스트 0 환경(실측 r1.3)이면 `compute.hypervisor_node`·`storage.pool` 등
  `kindPrefix=compute.`·storage/network 종의 items>0으로 discovery를 증명한다.
  `compute.vm`·`compute.system_container` 열은 게스트 생성 시점에 보완 종결.

```bash
# 프로바이더 커넥션 등록 확인
mysql -u <user> -p <db> -e \
  "SELECT uid,name,endpoint,status FROM provider_connection WHERE provider_type='proxmox';"
# 1행 — uid가 register-pve 출력과 일치
```

### 3-b. 권한 시드 (claim 11 — E-2 승인분)

부팅 시 `store.Seed`가 `infra:pve:guest:operate` 메뉴 행을 만들고 super-admin에
부여한다(멱등 — 재부팅 시 행 수 불변).

```sql
-- 메뉴 행 (>= 1)
SELECT COUNT(*) FROM sys_menu
 WHERE value='infra:pve:guest:operate' AND menu_status=1;
-- super-admin 부여 (>= 1)
SELECT COUNT(*) FROM sys_role_menu rm
  JOIN sys_role r ON r.id=rm.role_id
  JOIN sys_menu m ON m.id=rm.menu_id
 WHERE r.role_key='super-admin' AND m.value='infra:pve:guest:operate';
```

## 4. 롤백 SQL (r1.4 §11 HIGH-2 — 삭제 순서는 참조 방향 역순)

코드는 revert로 복귀한다. 데이터 잔여물은 아래 순서로 수동 삭제한다.
**`infra_resource`까지 지우는 이유**: 리소스 신원 unique key가
`(context_id, kind, external_urn)`이라 컨텍스트만 지우고 재등록하면 구 resource
행이 고아화되어 새 체인과 재수습 불가 — 관측·리소스·싱크런을 함께 지워야
재등록이 깨끗하다. super-admin 권한 부여 행은 시더 재실행으로는 소거 불가
(upsert-only)이므로 마지막에 수동 delete한다. `<connection-uid>`는
register-pve 출력의 `connectionUid`로 치환한다.

**SecretRef 주의**: 바인딩(`provider_credential_binding`)을 먼저 지우면 어느
SecretRef가 이 커넥션 것이었는지 역추적할 수 없다 — 그래서 0단계에서 id 목록을
**먼저** 확정하고, 확정한 목록으로 뒤에서 삭제한다(순서는 plan §11 문언 그대로
바인딩 → SecretRef).

```sql
SET @conn_uid = '<connection-uid>';
SET @conn_id = (SELECT id FROM provider_connection WHERE uid = @conn_uid);

-- (0) 삭제 대상 SecretRef id 목록을 먼저 확정 — 결과를 보관한다
SELECT b.secret_ref_id FROM provider_credential_binding b
 WHERE b.provider_connection_id = @conn_id;

-- (1) 실행 이력 — 태스크 체인 (있는 경우). provider_task에는 커넥션 컬럼이
--     없다(모델 실측 — resource_uid 키, model/task.go:18) — 컨텍스트→리소스
--     uid→resource_uid 경유로 조인한다. 순서상 태스크가 (2)의 리소스 삭제보다
--     선행하므로 서브쿼리가 유효하다.
DELETE te FROM task_event te
  JOIN provider_task pt ON pt.id = te.task_id
 WHERE pt.resource_uid IN (
   SELECT ir.uid FROM infra_resource ir
   WHERE ir.context_id IN (SELECT id FROM provider_context WHERE connection_id = @conn_id));
DELETE ta FROM task_attempt ta
  JOIN provider_task pt ON pt.id = ta.task_id
 WHERE pt.resource_uid IN (
   SELECT ir.uid FROM infra_resource ir
   WHERE ir.context_id IN (SELECT id FROM provider_context WHERE connection_id = @conn_id));
DELETE FROM provider_task WHERE resource_uid IN (
   SELECT ir.uid FROM infra_resource ir
   WHERE ir.context_id IN (SELECT id FROM provider_context WHERE connection_id = @conn_id));

-- (2) 관측·리소스·싱크런 — 컨텍스트 종속 데이터 전량
DELETE ro FROM resource_observation ro
  JOIN infra_resource ir ON ir.id = ro.resource_id
 WHERE ir.context_id IN (SELECT id FROM provider_context WHERE connection_id = @conn_id);
DELETE ir FROM infra_resource ir
 WHERE ir.context_id IN (SELECT id FROM provider_context WHERE connection_id = @conn_id);
DELETE FROM inventory_sync_run WHERE connection_id = @conn_id;

-- (3) 바인딩 → SecretRef → 컨텍스트 → 커넥션
DELETE FROM provider_credential_binding WHERE provider_connection_id = @conn_id;
DELETE FROM secret_ref WHERE id IN (/* (0)에서 확정한 id 목록 */);
DELETE FROM provider_context WHERE connection_id = @conn_id;
DELETE FROM provider_connection WHERE id = @conn_id;

-- (4) 권한 — 부여 행 먼저, 메뉴 행은 마지막
DELETE rm FROM sys_role_menu rm
  JOIN sys_menu m ON m.id = rm.menu_id
 WHERE m.value='infra:pve:guest:operate';
DELETE FROM sys_menu WHERE value='infra:pve:guest:operate';
```

원문 순서 요약(task_event → task_attempt → provider_task → resource_observation
→ infra_resource → inventory_sync_run → provider_credential_binding →
secret_ref → provider_context → provider_connection + sys_role_menu → sys_menu)
은 plan §11 r1.4 HIGH-2 보강분과 동일하다.

## 5. 재등록

롤백 후 또는 자격 로테이션 시: 동일 `--name`으로 §2를 재실행하면 같은 UID 체인이
갱신된다(멱등 upsert). 롤백 SQL을 전량 실행한 뒤 재등록하면 처음과 동일하게
생성된다.
