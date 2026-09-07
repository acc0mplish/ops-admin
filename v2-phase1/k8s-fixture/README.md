# v2 Phase 3 Slice A — kind 2호기 fixture (계획 N17/N18/N19)

Slice A 트레이스(리허설 F·Go E2E G1·claim 6/7/8)가 향하는 실클러스터 자산.
**phase2 게이트 클러스터 `v2-p2`는 무변경** (§0.2a 게이트 자산 보호, 보존 제약 #11) —
모든 Phase 3 실클러스터 작업은 `kind create cluster --name v2-p3` 2호기에서만.

## 자산

| 파일 | 역할 |
|---|---|
| `seed.yaml` | 시드 매니페스트 — ns `v3-seed`·deploy `restart-target`(nginx:1.27-alpine 2 replica·`minReadySeconds: 10`). minReadySeconds는 크래시 주입의 킬 창을 결정적으로 만드는 장치(R2/H1) |
| `run-fixture.sh` | 아래 절차 전체의 실행 가능 재현 (L1) |
| `README.md` | 본 문서 |

## 구축·재현 절차 (run-fixture.sh가 수행)

```bash
kind create cluster --name v2-p3 --image kindest/node:v1.34.0   # 2호기 (없을 때만)
docker pull nginx:1.27-alpine && kind load docker-image nginx:1.27-alpine --name v2-p3  # 슬로우 풀 제거(R12)
bash v2-phase1/k8s-fixture/run-fixture.sh                        # 시드→v1 등록→백필→sync→검증
cd backend && go test -tags=e2e ./e2e/slicea/ -count=1           # Go 트레이스 E2E (G1)
```

E2E(`backend/e2e/slicea`)는 전용 스키마 `ops_admin_p3e2e`를 스스로 생성·해제하며
클러스터 등록·백필·sync도 자체 수행한다(VK-7·A11) — dev 스키마(`ops_admin`)를
건드리지 않는다. run-fixture.sh은 dev 스키마 리허설·F 단계용.

## 재시딩 절차

시드 스펙 변경 시: `kubectl --context kind-v2-p3 apply -f seed.yaml` 재실행만으로
수렴(멱등). 클러스터 재생성이 필요하면 2호기를 지우고 위 절차를 다시 — **v2-p3만**.

## dirty runbook (VK-12)

- E2E 실패로 스키마 `ops_admin_p3e2e`가 잔류: E2E는 성공 시에만 해제한다 —
  `docker exec ops-admin-mysql-dev mysql -uroot -p -e 'DROP DATABASE ops_admin_p3e2e'`
- 잔존 running 태스크 정리(dev 전용): `UPDATE provider_task SET status='failed', error_code='lease_expired', finished_at=NOW() WHERE status IN ('queued','running','awaiting_approval');`
- E2E 실행 전후 어느 쪽이든 v2-p2 조작 금지.

## 측정 기구 (§0.1/L3)

클러스터 단얫은 전부 `kubectl --context kind-v2-p3 -o json` 서브프로세스 1회 호출로
JSON을 채취해 Go가 판정한다 — generation 증분은 실행 전후 2회 스냅샷의 차,
신규 RS 판별은 `metadata.creationTimestamp > 테스트 시작 시각` 기준(이름 순서 아님).
