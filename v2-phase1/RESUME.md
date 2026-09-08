# V2 재개 문서 (RESUME) — 2026-09-08 기준

세션·컨텍스트가 새로 시작될 때 이 문서부터 읽는다. state 파일들(`p2~p5-state.json`)이 상세 상태고, 본 문서는 **재개 시 실행 순서·대기 항목·확정 결정**의 요약이다.

## 현재 위치

| 페이즈 | 상태 |
|---|---|
| Phase -1~4 | ✅ 종결 (interim — 실계정 증명 일부 보류) |
| Phase 5 Proxmox | ✅ **완전 종결** — read-only+guarded ops·실엔드포인트 게이트 전 항 실증·실게스트 power/snapshot 라이브 실증 (PR #42~#49) |
| §18.2 메트릭 | ✅ 14종 전부 (PR #48·#49) — M1의 마지막 기술 병목 해소 |
| Phase 6 (legacy cutover) | ⏸ **진입 불가 — 조건부 대기** (`phase6-entry-assessment.md`) |
| Phase 7-9 (vCenter 등) | 예약 — M3 (진입 전제 미충족) |

## 확정 결정 (사용자)

- **Aliyun (및 Tencent 신규 확장) 지원 계획 없음 — 2026-09-08 사용자 확정.** 클라우드 family의 실계정 승격(p4 E-1·§13-10)·§15.4 cloud family 비교는 **보류 확정**. 현시점 유지: aliyun/tencent 어댑터 코드는 존치(읽기 전용·mock 증명 상태), 신규 투자 없음. 이 결정으로 §19.1의 cloud family 행은 Phase 6 범위에서 **제외**된다 — cutover 대상 family는 k8s만.
- Phase 2 게이트 3일차·M5 CLI freeze는 시간 의존으로 진행(아래 대기 참조).

## 대기 항목 (재개 시 체크)

| 항목 | 예정 | 재개 시 할 일 |
|---|---|---|
| **Phase 2 게이트 3일차** (§15.4 3연속·3 상이 일자) | **2026-09-09** — 원샷 cron 예약(a903b7b9, 세션 생존 시 자동). 미발화 시 수동 | 아래 "day-3 수동 실행" 블록 |
| **M5 ceefc27 CLI freeze 해소** | 2026-09-09 | freeze 해소 확인만 — 별도 작업 없음(해소 = freeze 만료 경과) |
| day-3 PASS 후 | — | `phase6-entry-assessment.md` §15.4 행 갱신 → k8s family 조건 충족 |

### day-3 수동 실행 (게이트기가 3 상이 일자를 기계 강제)

```bash
cd /mnt/d/DEV/acc0mplish/ops-admin/backend
docker ps | grep v2-p2-control-plane        # kind 클러스터 생존 확인
go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124
go run . compare-inventory --cluster 1      # verdict: pass (blocker면 20s 간격 sync+compare 재시도 — 사다리 수렴)
go run . compare-inventory --cluster 1 --gate   # 3 distinct days PASS 확인
```

### 잠재 함정 (재발 시 런북)

- **v2-p2/p3 sync가 "unknown v2 master key id k20260906"로 실패** → 자격이 로테이션 구키 봉인. 해소: `UPDATE k8s_cluster SET updated_at=NOW() WHERE id=1;` 후 sync 재실행(백필이 현키로 재봉인).
- **compare가 전부 identity-sets-differ blocker** → v2 세대가 stale. sync를 compare 직전에 실행(|Δ|≤60s). fixture cron 포드 사다리 수렴까지 2~3 사이클 소요 정상.
- 서버 미기동 시: `cd backend && go build -o /tmp/p5-backend . && (setsid nohup /tmp/p5-backend > /tmp/p5-server.log 2>&1 &)` — 포트 8082. admin 비밀: dev DB 리셋값 `P5-Gate-2026!` (sys_admin.password bcrypt).

## Phase 6 진입 조건 (§19.1 — assessment 기준)

- ✅ 복구 리허설 (2026-09-08 — 13테이블 COUNT·CHECKSUM 일치)
- ✅ dual-write authority rule 공개 (§19.1 본문)
- ⏳ §15.4 k8s family 3일차 (09-09)
- ➖ ~~cloud family~~ — **범위 제외 확정**(위 결정)
- k8s family 충족 시: **Phase 6 계획(plan-high) 착수** — cutover 대상 family = k8s(호스트 계열 asset은 §3 row 4 M2 cutover 표기 유지, 별도 판정)

## 진행 중 브랜치/PR

- 없음 (전부 병합·삭제). p5 잔여 3건도 PR #50(c4813ca)으로 소화 — **기술 잔여 0**. 잔여는 day-3 게이트(09-09 예약)만.

## 원천 인덱스

- 스펙: `docs/architecture/multi-infrastructure-control-plane-v2.md` (§14.3 PVE·§18.2 메트릭·§19.1 cutover·§20 게이트·§25 Slice B)
- 계획: `v2-phase1/phase{0..5}-plan.md`·`metrics-plan.md`·`task{1,3..6}-plan.md`
- 상태: `v2-phase1/p{2..5}-state.json`·`phase6-entry-assessment.md`
- 런북: `v2-phase1/pve-provisioning.md`·`k8s-fixture/README.md`
