# Phase 6 컷오버 런북

작성: 2026-09-09 · 신설 사유: **스펙이 두 곳에서 명명하는데 실물이 없었다** (검토 H-5) —
`docs/architecture/multi-infrastructure-control-plane-v2.md:1425`(§17.2)·`:1840`(§24 C2).
`grep -rn 'cutover runbook'` → 스펙 2행뿐, 실물 0.

- 계획: `v2-phase1/phase6-plan.md` (Phase 부호 Z1~Z3·A·BCD·D2·E·F·G·H·I는 그 §4)
- 증거: `v2-phase1/phase6-plan-evidence.md` · 인계: `v2-phase1/phase6-parity-handover.md`
- **본 런북은 절차만 담는다.** 판정 기준은 계획 §6 claims가 소유한다.

---

## 0. 전제 확인 (매 Phase 착수 전 1회)

```bash
cd /mnt/d/DEV/acc0mplish/ops-admin
git status --porcelain            # 깨끗해야 한다
docker ps | grep -E 'v2-p2-control-plane|ops-admin-mysql-dev'   # 둘 다 Up
git tag p6-<phase>-base           # claims의 diff 기준점 (계획 §6 — HEAD~<n> 대신 태그)
```

**베이스라인 스냅샷** (Phase 착수 직전, 비교용으로 기록):

```bash
cd backend
wc -l service/k8s.go service/monitor.go service/service.go
grep -c . ../docs/security/route-inventory.txt ../docs/security/sensitive-routes.txt
grep -h -c 'opdef.Middleware(' router/*.go | awk '{s+=$1} END{print "opdef.Middleware:", s}'
go test ./opdef/ -cover -count=1 | grep -o 'coverage: [0-9.]*%'
```

---

## 1. 비상 정지 · 롤백

| 상황 | 조치 |
|---|---|
| 어느 Phase든 claims 실패 | 해당 PR을 머지하지 않는다. 이미 머지됐으면 `git revert -m 1 <merge-sha>` |
| **BCD** 되돌림 | 단일 PR이므로 원자 revert. **부분 revert 금지** |
| **D2** 되돌림 | `service.go`·`gateway.go`가 함께 들어 있다. **부분 revert 시 빌드 파손** — 반드시 PR 단위 |
| **E** 되돌림 | 머지 순서가 E2(프론트)→E1(백엔드)이므로 **revert는 역순 E1→E2** |
| **I** (drop) 되돌림 | `mysql -uroot -p123456 ops_admin < /tmp/p6-pre-drop.sql` 후 `go run . migrate`. 실데이터가 없으므로 마이그레이션 재실행만으로도 복구 가능 |
| §15 게이트 판정이 흔들려 보임 | **bare `--gate`를 돌리지 마라.** newest-3 규칙이 기록된 PASS를 뒤집는다. `--gate-waiver ../v2-phase1/compare-gate-waiver-2026-09-08.json`을 반드시 동반 (계획 보존 제약 #5) |

---

## 2. C21 선행조건 — `seed-cron` 제거 (블록 1 착수 시 **1회**)

**왜**: 무변경 HEAD에서 `compare-inventory --cluster 1`이 3회 전부 `blocker`다. 게이트 클러스터
`kind-v2-p2`의 `v2-seed` 네임스페이스에 임시 적용된 `seed-cron` CronJob(`*/10 * * * *`)이 완료
파드를 회전시키는데 V2가 Job/CronJob을 미수집해 identity-sets-differ가 상시 발생한다.
**커밋된 `v2-phase1/k8s-fixture/seed.yaml`에는 없는 객체다**(그 파일은 Namespace·Deployment·
StatefulSet·DaemonSet 4종뿐) — 저장소 픽스처를 고칠 필요가 없다.

```bash
# 1) 제거 전 waiver 게이트 상태 기록 (R21 — 삭제가 기존 증거를 훼손하지 않음을 확인)
cd backend
go run . compare-inventory --cluster 1 --gate \
  --gate-waiver ../v2-phase1/compare-gate-waiver-2026-09-08.json   # "Passed": true 여야 한다

# 2) 제거
kubectl --context kind-v2-p2 -n v2-seed delete cronjob seed-cron
kubectl --context kind-v2-p2 -n v2-seed delete pod --field-selector status.phase=Succeeded

# 3) 베이스라인 재취득
go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124
go run . compare-inventory --cluster 1
python3 -c "import json,glob,os; f=max(glob.glob('data/compare/1/*/*.json'),key=os.path.getmtime); print(json.load(open(f))['verdict'])"
# 기대: pass

# 4) waiver 게이트 재확인 (아티팩트 3개를 이름으로 고정하므로 불변이어야 한다)
go run . compare-inventory --cluster 1 --gate \
  --gate-waiver ../v2-phase1/compare-gate-waiver-2026-09-08.json   # "Passed": true 유지
```

**3)이 `pass`가 아니면**: C21을 그 Phase에서 비활성화하고 **사유를 PR 설명에 적는다**. 탐지기
부재를 조용히 두지 않는다 (계획 §J6).

**근본 해소**: 선행 과제 P1의 Job/CronJob 수집(인계 문서 §1.3)이 착지하면 이 조치가 불필요해진다.

---

## 3. Phase별 절차

### Z1·Z2·Z3 — characterization test

1. `cd backend && echo $(( $(grep -c '^func ' service/k8s.go) - $(grep -c '^func (s \*Service)' service/k8s.go) ))` → `120` 확인 (C44)
2. Z1에서 `*Service` 메서드 41개 이름 목록을 `service/testdata/char-exclude.txt`로 생성·커밋 (C46 참조). Z3 종료 시 `service/testdata/char-baseline.txt`도 생성·커밋 (C47 참조)
3. 시맨별로 테스트 작성 — **테이블 주도**, legacy 호출을 **1행으로 격리**(인계 문서 §5 승계 규약)
4. 각 테스트 최상단에 주석: `// characterization: 현재 동작 고정. 정합성 판정 아님 — 버그도 그대로 기록한다.`
5. C45·C46·C16 → C7

### A — 라우트 등록 분할

1. `router.go`의 등록 블록을 도메인별 `registerXxx(g, db, ctl)`로 이동. **그룹 생성·`.Use()`는
   `router.go`에 남긴다** (계획 §J1)
2. v2는 전용 시그니처 `registerV2(engine, db, v2API)` — nil 폴백(`router.go:537-539`) 포함
3. **핸들러를 클로저로 감싸지 않는다** (감싸면 골든이 즉시 깨진다 — 내장 카나리)
4. C4 → C1 → C2 → C48 → C6 → C7 → C16

### BCD — DTO·메서드·빌더 이관 (10파일, 단일 PR)

1. E5 경계표(증거층)대로 심볼 단위 이동. **로직 수정 금지 — 순수 이동**
2. **커밋을 시맨별로 분리**한다(리뷰는 커밋 단위 — R20)
3. `k8s_fetch.go`가 650 초과면 `k8s_fetch.go`/`k8s_overview.go`로 조건부 재분할
4. **C47**(Z 전량 통과 — B~D2의 유일한 검출기) → C15 → C16 → C21 → C19 → C20 → C7

### D2 — 트랜스포트 + 상태 캡슐화

1. `k8sClientState`를 신설하고 `Service`의 5필드(`service.go:49-56`)를 흡수.
   **반드시 포인터(`*k8sClientState`)로 보유** — 값 복사 시 뮤텍스·singleflight가 복제돼
   상호배제가 깨진다
2. `service.go:72` `New`에서 초기화, `gateway.go`를 캡슐 경유로
3. C13(`go vet` copylocks) → C14 → C47 → C15 → C16 → C21 → C7

### E0 → E2 → E1 — step 4a (E-1 (가) 확정)

**순서 역전 금지**: E0(필터) → E2(프론트가 V2 경로 사용) → E1(백엔드 경로 삭제). revert는 역순.

E0 — `internal/api/v2/infra.go` 1파일:
1. `ListResources`에 `connectionUid` 쿼리 파라미터 파싱 추가
2. `liveResources`의 기존 `provider_connection pconn` JOIN에 `pconn.uid = ?` WHERE 한 줄
3. **응답 형상·기존 4개 필터·다른 핸들러는 건드리지 않는다** — C51이 잠근다
4. C51 → C19 → C7

E2 — 계약은 계획 §3.5:
1. restart를 plan → execute(`Idempotency-Key`) → approve → 폴링(2초, 5분 상한)으로 전환
1b. uid 해석: `GET /provider-connections`에서 `sourceModel=="k8s_cluster" && sourceId==clusterId` → uid → `GET /resources?kind=orchestration.workload&connectionUid=<uid>&pageSize=100` → `externalUrn` 꼬리 일치
2. `K8s.vue:693`의 벌크 `Promise.all`을 **N개 승인 대기 태스크**로 분해. 건별 Idempotency-Key,
   승인 대기 큐 표시, 폴링 주기, 부분 실패 표시
3. `web/e2e/slice-a.spec.js`(:15,:45)·`slice-c.spec.js`(:116) 갱신
4. C28 ② → C12 → C42 → C43 → C50

E1:
1. 라우트·컨트롤러·opdef def 삭제. **`service/k8s.go`의 `RestartK8sWorkload`는 남긴다** —
   `integration_ai.go:1732`가 호출하며 그 도메인은 OUT-OF-SCOPE다
2. **테스트 하드코딩 3곳을 같은 PR에서 함께 수정**:
   `routes_inventory_test.go:130` 450→449 · `authz_replay_test.go:234` 440→439(실패 메시지
   `(427 v1 + 13 v2)`→`(426 v1 + 13 v2)`) · `:353` 245→244
3. 골든 재생성: `cd backend && go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact' -update`
   후 **diff를 반드시 검토** (손으로 고치지 않는다)
4. C49 → C1' → C2' → C27 → C8 → C6 → C7

### F — 스펙 조건화 개정

1. §19.1에 영문 문단 `Applicability (M1 development stage)` 신설
2. D-2·D-4~D-7·D-10~D-12 각각에 `deviation recorded: 2026-09-09 — <사유>` 한 줄
3. §20 Phase 6 게이트 문언에 동일 참조
4. C33 → C42 → C43

### G·H·I — 블록 2 (선행 과제 P 완료 후)

인계 문서 §3 게이트 전항 통과가 진입 조건. 상세 분해는 P 산출 후 계획 r4가 소유한다(이월 I2).
**Phase I는 별도 사용자 승인 없이 실행하지 않는다.**

Phase I 직전 필수:
```bash
mysqldump --single-transaction -h127.0.0.1 -uroot -p123456 ops_admin > /tmp/p6-pre-drop.sql
test -s /tmp/p6-pre-drop.sql && echo BACKUP_OK        # C34
docker exec ops-admin-mysql-dev mysql -uroot -p123456 -N -e \
  "select count(*) from information_schema.key_column_usage \
   where table_schema='ops_admin' and referenced_table_name='k8s_cluster';"   # C35 → 0
```

---

## 4. 삭제·이동 시 6축 census (계획 §2.0 — PR 설명에 표로 첨부 필수)

```bash
X=<심볼>   R=<라우트>   T=<테이블>
grep -n "$R" docs/security/route-inventory.txt docs/security/sensitive-routes.txt   # ① 라우트
grep -rn "s\.$X(\|svc\.$X(" backend/ --include=*.go | grep -v _test                # ② 서비스 내부
grep -rniln "$X" web/src                                                            # ③ 프론트 (대소문자 무시)
grep -rn "$T" backend/ --include=*.go | grep -v _test; grep -n "$T" backend/util/secret_registry.go   # ④ DB
grep -rn "<하드코딩 수치>" backend/ web/e2e --include=*_test.go --include=*.spec.js  # ⑤ 테스트 단얫
ls .github/workflows/                                                               # ⑥ CI 2파일 12잡
```

축이 하나라도 미조사면 PR 반려다. **"선언된 표면이 아니라 실제 결합을 센다."**

---

## 5. 알려진 함정

| 증상 | 원인 · 조치 |
|---|---|
| `sync-inventory` 실패 `unknown v2 master key id k20260906` | 자격이 로테이션 구키로 봉인됨. `UPDATE k8s_cluster SET updated_at=NOW() WHERE id=1;` 후 재실행(백필이 현키로 재봉인) |
| `compare`가 전부 `identity-sets-differ` | ① §2의 `seed-cron` 미제거 ② v2 세대 stale — sync를 compare 직전에 실행(\|Δ\|≤60s) |
| bare `--gate`가 `Passed: false` | 정상이다. C21 실행이 아티팩트를 쌓아 newest-3가 바뀐 결과. **waiver 동반 실행만 유효** |
| `go test ./router/` 가 450/440/245로 실패 | Phase E의 하드코딩 3곳 수정 누락 — C49 |
| arch-boundary 카나리가 10분 넘게 안 끝남 | WSL 마운트 전체 복사가 느리다. 배경 실행하거나 `node_modules`·`web/e2e/.stack` 제외 복사 |
| 서버 수동 기동 필요 | `cd backend && go build -o /tmp/p6-backend . && (setsid nohup /tmp/p6-backend > /tmp/p6-server.log 2>&1 &)` — 포트 8082 |
