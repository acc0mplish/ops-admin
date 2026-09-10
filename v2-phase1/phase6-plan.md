# V2 Phase 6 계획 — Legacy cutover and God-object decomposition (r4)

작성: 2026-09-09 (r1) · r2 (전제 변경 2건) · r3 (4렌즈 상환: CRITICAL 10·HIGH 17·MEDIUM 22·LOW 8) ·
개정 r4 (2026-09-10 — I2 상환: 블록 2 G·H·I 파일 단위 상세 분해. 블록 1·선행 P 완결 반영,
전 수치 현재 HEAD `45671e2` 재실측) · **개정 r4.1 (2026-09-10 — 5렌즈 적대검토 상환:
CRITICAL 0·HIGH 11(dedupe)·MEDIUM 31·LOW 21 — `p6-block2-adversary-r1.md`. 필수 F1~F11 전부 적용·
권고 R-A~R-S 처분표 §15)** · 티어 **XL**

- 증거층: `v2-phase1/phase6-plan-evidence.md`(E1~E10) + `v2-phase1/phase6-coupling-census.md`(E12·E13) · 인계 문서:
  `v2-phase1/phase6-parity-handover.md` (**P**n) · P 계획: `v2-phase1/p-parity-plan.md` · 런북: `v2-phase1/phase6-cutover-runbook.md`
- 검토 원천: `docs/task-id/v2-phase6-cutover/gaps-r1.md` (부호 C-n·H-n·V-n·T-n·S-n·M-n)
- 원천 스펙: `docs/architecture/multi-infrastructure-control-plane-v2.md` — §19.1·§19·§3.2/§3.3·§5.4·§15·§16.1·**§17.2·§17.3**·§18.2·**§20**·**§21**·§24
- 상태: `v2-phase1/p6-state.json` — 블록 1 PR #52~#61 · P 트랙 PR #62~#74 전부 완결 머지

**r4 변경 요약**: ① 블록 1(Z·A·BCD·D2·F·E0·E2·E1)과 선행 P(PP-0~P1-G2·P2-A~E) 완결 기록으로 압축
— 상세는 PR·state.json·증거층에 영구 기록됨. ② §4.1 골격을 G0/G1·H0/H1/H2·I-a/I-b 파일 단위 배치표로 확장.
③ **S5 신설** — r3 §J5가 세지 않은 `GetK8sCluster` k8s_cluster 조회 소비 12곳(터미널·메트릭·진단·ops·AI)을
Phase I 선행 작업으로 등재(실측 2026-09-10). ④ create 라우트(`POST /k8s/resource/yaml/create`) 처분 판정
(I-P1 블로커 승계) — H2 기본선을 12건 삭제로 확정·create 존치 이월(I10). ⑤ C39 재설계(G1의 compare 종결로
왕복 판정 수단 변경). ⑥ claims·보존 제약·위험·이월 대장 블록 2판 갱신.

---

## 0. 결론 먼저 (BLUF)

**블록 1과 선행 P는 완결** — step 1(라우트 분할 #55)·step 2a(k8s.go 5,062→1,265 + D2 캡슐화 #56~#57)·
step 4a(restart V2 전환 #59~#61)·Phase F(스펙 조건화 #58), P 트랙(43필드 패리티·오퍼레션 10종 DefTable
잠금·Z 오라클 28 스왑·compare 3회 pass·판정 ①②③⑤④ 전부 결착 #62~#74). **잔여 step 3·4b·5가 전부 블록 2다.**

| §19.1 step | r4 판정 | Phase | 결정적 근거 |
|---|---|---|---|
| **3** 읽기 → V2 투영 (**3건**) | **착수 가능** (P1 완료 ✓) | **G0→G1** | §J3·§J8 — 접합점 `AssembleK8sClusterDetail`·`ProjectResources` P 완결 |
| **4b** 잔여 쓰기 중단 (**13건 중 12건**) | **착수 가능** (P2 + G 완료 후) | **H0→H1→H2** | §J9 — 오퍼레이션 10종 DefTable 잠금·CLI 규약 인계 §4. create 1건은 I-P1 블로커로 존치 이월(I10) |
| **5** 컬럼/테이블 삭제 | **계획만 수립 — 승인 게이트** | **I-a→I-b** | §J5·§J10 — k8s_cluster 결합 27참조·7파일(재실측) + **S5 신설** |

**블록 2 3대 실측 사실 (2026-09-10, HEAD `45671e2`)**:

1. **쓰기 라우트 13건 전수 확정** — `route-inventory.txt` k8s mutating 13행
   (:28·:29 DELETE, :333~:338 POST, :428~:432 PUT) = cluster CRUD 3 + 오퍼레이션 9 + create 1.
   서비스 내부 호출자는 여전히 **Restart·Scale 2종뿐**(`integration_ai.go:1732,1734` — P 무변경 계약으로 r3와 동일).
2. **프론트 쓰기 소비는 2파일에 집중** — `web/src/views/assets/K8s.vue`(오퍼레이션 9 + create)·
   `K8sClusterManage.vue`(cluster CRUD 3). `api/k8s.js` 쓰기 함수 13개 전부 존재(restart는 이미 V2). 폭발 반경이
   r3 우려보다 작다 — E2(restart) 선례의 기계적 확장으로 H1을 닫는다.
3. **S5 — `GetK8sCluster` 소비 12곳이 k8s_cluster 테이블에 묶임** — 터미널(`k8s_terminal.go:104,129`)·
   메트릭(`k8s_metrics.go:23,94`)·fetch(`k8s_fetch.go:120`)·진단(`asset_service_diagnosis.go:54,151`)·
   ops(`ops_application.go:1893`)·AI(`integration_ai.go:964` ListK8sClusters). r3 §J5는 backfill·compare·
   asset/ops·secret만 세고 이 계층을 놓쳤다. **Phase I의 실질 선행 작업**이다(§J5 S5).

---

## 1. 게이트 실현가능성 (§20 문언 대조)

§20 원문 관찰기간 조항은 운영 배포본 부재로 적용 불가 — §7 D-11로 조건화 개정 완료(Phase F #58 착지,
`grep -c 'deviation recorded'` 8건 이상 C33 완결). §19.1 전제조건 4항 녹색.
**블록 2 추가 이탈**: D-15(create 존치 — §J9)·D-16(compare CLI k8s 페어링 종결 — §J8).

---

## 2. 아키텍처 판정 (J판정)

### 2.0 경계 원칙 — 6축 결합 census 〔블록 2가 주 소비자〕

**원칙: 선언된 표면이 아니라 실제 결합을 센다.** 삭제·이동 대상마다 6축을 전수 조사하고 PR 설명에 표로
첨부한다(보존 제약 #15). 6축: ① 라우트 골든 2종 · ② **서비스 내부 호출자**(`grep -rn "s\.X(\|svc\.X("
backend/ --include=*.go | grep -v _test`) · ③ 프론트(`grep -rniln` — 대소문자 무시) · ④ DB·마이그레이션 ·
⑤ 테스트 단얫(하드코딩 수치) · ⑥ CI(워크플로 2파일 12잡). **r4 재실측으로 ②③이 갱신됐다** — ② 14종 중
내부 호출자 2종(Restart·Scale ← `integration_ai.go:1732,1734`) 불변 · ③ 쓰기 프론트 소비 **2파일로 집중**
(K8s.vue·K8sClusterManage.vue — r3 "9파일"은 읽기 포함 전체 소비 기준).

### 2.1 블록 1·P 판정 완결 요약 (상세는 r3 문서 이력·PR·state.json)

| 판정 | 요약 | 완결 증거 |
|---|---|---|
| J0 Phase Z | 순수 함수 120 characterization — TestChar 120·C47 baseline | #52~#54, 3중 CHAR_IDENTICAL |
| J1 step 1 | 등록 함수 4분할 behavior-neutral — 골든 diff 0·middleware 285 | #55 |
| J2 step 2a | k8s.go 5,062→1,265 + D2 캡슐(포인터 강제 C13) — 2b는 I1 이월 | #56~#57 |
| J4 step 4a | E-1 (가) `connectionUid` 필터(#59)→E2 프론트 V2 4단 흐름(#60)→E1 레거시 경로 삭제(#61). Restart 메서드 존치 | #59~#61 |
| J6 C21 | 폐기 아님 — seed-cron 선행조건화·waiver 3아티팩트 이름 고정 | C22 Passed=true |
| J7 삭감 | BCD 1PR(S-1)·C32 카나리 1회(M-1) 등 | 승인 기록 |
| P 트랙 | 43필드(④ A′-1 포함)·수집기 5+부수·kind 4종·`k8sassembly.go` 4파일·오퍼레이션 10종 DefTable·Z 스왑 28·판정 ①②③⑤④ 결착 | #62~#74 (56b12da) |

### J3 — step 3: 대상은 **3 엔드포인트** 〔r3 판정 승계·좌표 갱신〕

- **전환 3건**: `GET /k8s/cluster/list`(:147)·`/cluster/info`(:146)·`/cluster/detail`(:145) — `mapping.md`가
  덮는 유일 모집단 `K8sClusterDetail` DTO(읽기 21 = 전환 3 + 라이브 패스스루 17 + `pod/terminal/ws` 1).
- **라이브 패스스루 17 + ws 1은 v1 무기한 존치** (§7 D-12) — 하이브리드 아님(r1 판정 승계).
- **플래그**: 임시 env `V2_READ_SOURCE_K8S` (롤백용 보존 폐기·전환 검증용). G1의 마지막 커밋이 플래그와
  legacy 분기를 함께 제거(C36 2단 유지).

### J8 — G 설계: 소스 전환의 3가지 착지점 〔r4 신설·임계〕

**접합점 (P 완결 산출물 — G는 소비만, 수정 금지)**: `inventory.AssembleK8sClusterDetail(db, conn, rows)`
(`k8sassembly_sections.go:27`) · `inventory.ProjectResources(db, contextID, generationUID)`
(`projection.go:68`) · `service.go:22` 이미 `internal/infra/inventory` import — 배선 최소.

**3 엔드포인트는 서비스 계층 구조가 서로 달라 각각 다른 착지점을 쓴다** (실측):

| 엔드포인트 | 컨트롤러 | 현재 소스 | 착지점 | 이유 |
|---|---|---|---|---|
| `cluster/list` | `controller/k8s.go:15` | `ListK8sClusters`(`k8s.go:16`) | **메서드 내부 분기** | 유일한 타 소비자가 AI 도구(`integration_ai.go:964`) — 응답 형상 `[]K8sClusterView` 동일하므로 동반 전환 무해 |
| `cluster/info` | `:24` | `GetK8sCluster`(`k8s.go:29`) | **컨트롤러 교체** — `:32`가 신규 `projectK8sClusterInfo` 직접 호출 | `GetK8sCluster`는 **소비 12곳**(S5)이 k8s_cluster 행·평문 kubeconfig에 의존 — 내부 분기하면 터미널·진단·메트릭이 전부 V2 조립물을 받아 봉인 계약 위험. 골든 핸들러명(`GetK8sClusterInfo-fm`)은 컨트롤러 함수명이라 불변 |
| `cluster/detail` | `:85` | `GetK8sClusterDetail`(`k8s.go:142`)→캐시(:165)→`getK8sClusterDetailUncached`(:184 라이브 k8s API: `parseKubeConfig`→`newK8sHTTPClientForCluster`→`fetchK8sData`) | **메서드 내부 분기** | 타 소비자 2곳 — `main_compare.go:116`(G1에서 S2 종결)·`integration_ai.go:966`(AI 도구 `k8s_cluster_overview` — 응답 형상 동일로 동반 전환 무해, §14.2-4 승계) |

- 신규 파일 `service/k8s_projection.go`: `projectK8sClusterList`·`projectK8sClusterInfo`·
  `projectK8sClusterDetail` — provider_connection 조회 + `ProjectResources` + `AssembleK8sClusterDetail`.
  **라우트·opdef 등록은 무변경(골든 불변) — 컨트롤러 `k8s.go:32` 본문(info 착지점)만 교체다.**
- **〔r4.1·R-I〕 info 응답의 kubeConfig 마스킹 계약**: `model.K8sCluster.KubeConfig`는
  `json:"kubeConfig"`로 **마스킹 없이 직렬화**된다(실측 `model/k8s.go:5-24`) — 현재 v1 info 응답에
  평문이 실린다. 유일 프론트 소비처는 `K8sClusterManage.vue:128` 편집 폼 프리필(**H1에서 폼 제거로
  소멸**)·`AssetDetail.vue` 미소비. **판정: `projectK8sClusterInfo`는 kubeConfig를 항상 `""`로
  반환한다** — S5 전환 이후 V2 소스에는 평문을 채울 수단 자체가 없고(봉인 계약 §12 #16) 유일 소비가
  H1에서 사라진다. G0 명시 변경(응답 형상 불변 원칙의 유일 예외)으로 기록하고 `TestProjectK8sClusterInfo`
  deliverable이 잠근다(C69). G0~H1 사이 편집 폼은 기존 kubeconfig 프리필 대신 재입력을 요구한다.
- **캐시 승계**: `GetK8sClusterDetail`의 `k8sOverviewCache`(`service.go:67`) 구조·TTL 무변경 — 본문 소스만
  교체. 최소 diff 원칙(필요성 하락은 제거 사유가 아니다).
- **동치 판정 수단**: §15 `compare-inventory`가 legacy 캡처(uncached)와 V2 인벤토리를 페어링하므로
  `verdict: pass`가 곧 전환 동치 판정이다(P가 3회 pass로 이미 예증). 별도 왕복 대조기 불요.
  **판정 시점은 G0 종료 단일 1회뿐이다**〔r4.1·F11〕 — G1의 산출물이 compare 종결 자체라 G1 종료 후에는
  CLI가 이미 존재하지 않는다(0정보 재실행).
- **플래그 운용 절차**〔r4.1·R-A 보강〕: G0 검증에서 `V2_READ_SOURCE_K8S=1`로 서버를 기동해 3
  엔드포인트 응답을 확인하는 절차를 선택 절차로 둔다(롤백 실증 — env 변경만으로 legacy 소스 복귀).
- **G1에서 S2(compare k8s 페어링)를 조기 종결한다** — `uncached` 제거가 `main_compare.go:116` 호출을
  깨기 때문에 같은 PR에서 처리한다(§J5 S2 갱신): `main_compare.go`의 k8s 페어링·`legacyCaptureFromDetail`
  제거, `compare-inventory`는 cloud(`compare-inventory-cloud`·별도 dispatch)와 무관하게 CLI 종결.
  `main_compare.go` CLI 동결은 Phase 2 게이트 종결(C22 완결)로 이미 해제됐다(`main.go:57` 주석 참조).
  **파급 — C21·C22 폐기**: G1 이후 §15.4 게이트의 k8s 판정 수단이 소멸한다. 이는 §19.1 "§15 comparisons
  re-run after each step"의 적용 종료 시점이 G1임을 의미 — §7에 D-16으로 기록하고, H·I의 검증은
  claims(C39′·C68 등 V2 왕복)가 대체한다.

### J4 — step 4a 완결 → 4b 승계 〔E-1 (가) 패턴의 H 적용〕

E0→E2→E1 3단(#59~#61)이 확립한 패턴을 H가 승계한다: **대체 경로 먼저(H0 CLI·H1 프론트) → 마지막 삭제(H2)**.
- **(가) 패턴 승계**: 라우트·컨트롤러·opdef def가 "경로"다. 서비스 메서드는 내부 호출자가 있으면 존치 —
  `ScaleK8sWorkload`(`k8s_workload.go:61`)·`RestartK8sWorkload`(`:103`)는 `integration_ai.go:1732,1734`
  (AI 도구, §3.2 row 16 OUT-OF-SCOPE)가 호출하므로 **존치**. 내부 호출자 0인 12종 중 삭제 11종·create
  존치 1종은 §J9에서 종별 확정.
- **E0 자산 승계**: `connectionUid` 필터(`infra.go:276-283`)가 이미 착지 — H1 프론트가 uid를 해석하는
  경로(`GET /provider-connections` → uid → `GET /resources?connectionUid=`)는 재구축 불요.

### J9 — H 설계: 쓰기 13건 종별 처분 + create 블로커 〔r4 신설·임계〕

**13건 전수 (실측 — `route-inventory.txt`·`routes_v1_infra.go:223-253`·`defs_monitor.go:70-89`·`sensitive-routes.txt` 13행)**:

| # | 라우트 | 서비스 메서드 (위치) | opdef 권한 | V2 대체 | 처분 |
|---|---|---|---|---|---|
| 1 | POST `/k8s/cluster/add` | `CreateK8sCluster`(`k8s.go:37`) | `assets:k8s:cluster:add` | `register-k8s` CLI | **H2 삭제** + 메서드 삭제 |
| 2 | PUT `/k8s/cluster/update` | `UpdateK8sCluster`(`k8s.go:83`) | `assets:k8s:cluster:edit` | 동일 | 동일 |
| 3 | DELETE `/k8s/cluster/delete` | `DeleteK8sCluster`(`k8s.go:133`) | `assets:k8s:cluster:delete` high | 동일 | 동일 |
| 4 | POST `/k8s/workload/scale` | `ScaleK8sWorkload`(`k8s_workload.go:61`) | `assets:k8s:workload:scale` | `k8s.workload.scale` op | **H2 경로 삭제·메서드 존치**(② 호출자) |
| 5 | POST `/k8s/workload/images` | `UpdateK8sWorkloadImages`(`k8s_workload.go:149`) | `assets:k8s:workload:image` | `k8s.workload.image_update` | H2 삭제 + 메서드 삭제 |
| 6 | PUT `/k8s/workload/resources` | `UpdateK8sWorkloadResources`(`k8s_workload.go:232`) | `assets:k8s:workload:yaml` | `k8s.workload.resources_update` | 동일 |
| 7 | PUT `/k8s/node/labels` | `UpdateK8sNodeLabels`(`k8s.go:308`) | `assets:k8s:workload:yaml` | `k8s.node.labels_update` | 동일 |
| 8 | PUT `/k8s/service/update` | `UpdateK8sService`(`k8s_detail.go:74`) | `assets:k8s:workload:yaml` | `k8s.service.update` | 동일 |
| 9 | PUT `/k8s/resource/yaml` | `UpdateK8sResourceYAML`(`k8s_mutate.go:15`) | `assets:k8s:workload:yaml` | `k8s.resource.apply`(update 한정) | 동일 |
| 10 | DELETE `/k8s/resource/delete` | `DeleteK8sResource`(`k8s_mutate.go:122`) | `assets:k8s:resource:delete` high | `k8s.resource.delete` | 동일 |
| 11 | POST `/k8s/istio/traffic` | `UpdateK8sIstioTraffic`(`k8s_mutate.go:151`) | `assets:k8s:advancednetwork` | `k8s.istio.traffic_update` | 동일 |
| 12 | POST `/k8s/httproute/traffic` | `UpdateK8sHTTPRouteTraffic`(`k8s_mutate.go:202`) | `assets:k8s:advancednetwork` | `k8s.httproute.traffic_update` | 동일 |
| 13 | POST `/k8s/resource/yaml/create` | `CreateK8sResourceYAML`(`k8s_mutate.go:81`) | `assets:k8s:workload:yaml` | **없음 — I-P1 블로커** | **H2 존치 + 이월(I10)** — 아래 판정 |

**create 처분 판정 (I-P1 승계)**: create는 대상 리소스 uid가 없어 uid-스코프 오퍼레이션 모델에 부적합
(P 계획 §1-3 판정). 대안 3종 — (a) §16.1 커넥션-스코프 오퍼레이션 라우트 신설: `internal/api/v2/` 확장으로
본 계획의 자기한정(#6·C51 승계) 밖, 별도 계획·승인 필요 · (b) **12건만 삭제하고 create 1사이클 존치**
· (c) CLI create 서브커맨드: YAML apply는 UI 상호작용이 본질이라 부적합. **판정: (b)** — 스펙 §19.1 step 4의
완전 시달은 (a) 착지 시점이며, H의 스코프 통제(과대 범위 금지)가 (b)를 지시한다. §7 D-15 이탈 기록 +
I10 이월(§16.1 POST 구현 계획 수립 시 create 오퍼레이션화·라우트 삭제·골든 440→439). **H2 골든 기대값은
12건 기준으로 확정한다 — 기대값의 단일 원천은 claims C38·C40이며 본문의 수치 표기는 전부 이 2개로
귀결한다〔r4.1·R-R〕(13건 전 삭제 시 I10 착지 시점 값으로 C38·C40이 갱신된다).

**register-k8s CLI (H0)**: 신규 `backend/main_register_k8s.go` + `main.go` dispatch(`register-pve` :71-72
직후). **인계 §4 규약 verbatim 이관** — §3.2 배치표. 선례 구조(실측): `pveRegisterUID(name, salt)` =
sha256("register-pve\|name\|salt")[:32] register-scoped 도메인 · `pveUpsertSecretRef`(EncryptSecretV2
즉시 봉인) · `pveUpsertBinding`(purpose 재포인팅) · 검증 실패 시 트랜잭션 롤백. k8s판은
`register-k8s|name|salt` 도메인 + `--kubeconfig <path>` 경로만 수령(값·로그 노출 금지) +
`--monitor-datasource-id`는 `ConfigJSON["monitor_datasource_id"]`(판정 ③ Id 저장처) 기록.

**H1 프론트 전환 (E2 UI 계약 승계 — §3.3)**: 소비 2파일. `K8s.vue`의 오퍼레이션 9종은 restart(#60)가
확립한 V2 4단 흐름(plan→execute(`Idempotency-Key`)→approve→폴링)·`api/infra.js` 재사용. `K8sClusterManage.vue`
cluster CRUD 폼은 **제거 + CLI 안내로 대체**(기능의 CLI 이전 — UX 변화, §10 R26). i18n은 기존 4단 흐름
문구 재사용 + cluster 제거 안내 — 사전 21종 패리티(C42)가 잠근다. 선례 좌표: `web/src/api/k8s.js:74-112`
restart V2 호출·Idempotency-Key 유틸 주석(r3 §3.5 계약의 코드 내 원천).

**〔r4.1·R-H〕 seed 권한 트리 정리(H2 동반)**: `store/seed.go:375` cluster CRUD 권한 매핑 3종
(`assets:k8s:cluster:add`·`:edit`·`:delete`)이 H2 라우트·opdef 삭제로 고아가 된다 — **H2에서 같은 PR에
제거**한다. 단 부모 `assets:k8s:cluster`(seed.go:228·:332)와 `assets:k8s:workload:scale` 등 오퍼레이션
권한은 `compose_k8s_ops.go` V2 def가 재사용 중 — **선택적 정리 3종만**.

### J5 — step 5: `k8s_cluster` 결합 〔r4 재실측·S5 신설〕

r3 표를 현재 HEAD로 재실측 갱신 — **27참조·6코드 파일**(AI 툴 키 문자열 `k8s_cluster_overview` 제외.
`main_sync.go:43`은 주석이라 코드 참조 아님 — r4 초안의 "7파일"에서 정정〔r4.1〕. 코드 파일:
backfill.go·main_compare.go·model/k8s.go·asset_service.go·secret_registry.go·integration_ai.go(문자열)):

| # | 결합 (현재 좌표) | 파괴 결과 | 처분 Phase |
|---|---|---|---|
| **S1a** | `markStaleSources`(`backfill.go:297-303`) — `source_id NOT IN (SELECT id FROM k8s_cluster)` | drop 후 `sync-inventory` 1회 → 전 행 `stale_source=true` → V2 읽기 전면 공백 | **I-a 제거** |
| **S1b** | `SourceKeyUID("k8s_cluster", id)`(`backfill.go:158,196`) — r3의 `sync.go:478`에서 이동 | UID 파생 코드가 죽은 테이블 참조 | **I-a 제거** |
| **S1c** | `RunK8sBackfill`(`backfill.go:57-75` 소스 읽기·전파 전체 — 328행 파일) | 백필 전체 무효 — **H 완료로 해소 조건 성립**(CLI가 등록 대체) | **I-a 제거**(k8s 경로만 — `backfill_cloud.go` 존치) |
| **S2** | `main_compare.go:37,74,76,83,116` legacy 캡처 | ~~Phase I~~ → **G1로 조기 이동**(§J8 — uncached 제거와 충돌) | **G1 제거** |
| **S3** | `AssetService.K8sClusterID`(`model/asset.go:12` — `asset_service.go:96` 쓰기)·`OpsApplicationEnvironmentBinding.K8sClusterID`(`model/ops.go:383`) — 둘 다 §3.2 REMAIN | DB FK 제약 0건(실측 `referenced_table_name='k8s_cluster'` 0행)·dev 행수 0. 코드 경로·칼럼 정리만 | **I-a** |
| **S4** | `util/secret_registry.go:65` Row 8(`K8sCluster.kube_config`, ClassPlaintext) — 87행 파일 | G-1 게이트 모집단 14→13 | **I-a 제거**(C41) |
| **S5** 〔r4 신설·r4.1 R-O 재열거〕 | **`GetK8sCluster` 소비 14좌표** — 외부 라이브 소비 9: `k8s_terminal.go:104,129`·`k8s_metrics.go:23,94`·`k8s_fetch.go:120`·`asset_service_diagnosis.go:54,151`·`ops_application.go:1893` / 내부 3(`k8s.go:84,134,185` — Update·Delete·uncached 경유, **H2·G1에서 소멸**) / info 진입 1(`controller/k8s.go:32` — **G0에서 V2 대체**) / **`ListK8sClusters` AI 소비 1**(`integration_ai.go:964` — List 별도) + `model/k8s.go:5-26`(K8sCluster gorm 매핑) | **drop 시 터미널·메트릭·라이브 패스스루 17건·진단·ops·AI 전부 `ErrRecordNotFound`** — r3가 세지 않은 계층 | **I-a 전환** — `GetK8sCluster` 본체 1착지점을 provider_connection(`sourceModel="k8s_cluster"`) + SecretRef 복호 kubeconfig 소스로 교체. 소비자는 무변경(시그니처·`model.K8sCluster` 반환 유지 — 타입은 존재, gorm 매핑만 제거) |
| **S6** 〔r4.1 신설·F9〕 | **미배치 gorm 직접 조회 5곳** — `gateway.go:109,169`(gateway_id→ClusterCount)·`monitor.go:698`(monitor_datasource_id 카운트)·`:4510`(전체 카운트)·`platform.go:108`(env 필드 엔티티)의 `&model.K8sCluster{}` | drop 시 게이트웨이 클러스터 수·모니터링 참조 카운트·플랫폼 통계가 전부 깨짐 — r4 초안 census 누락 | **I-a 전환** — 5곳을 provider_connection 등가 조회(COUNT)로 교체하거나 통계 소멸 판정(리뷰 확정). `k8s_projection.go`의 conn 조회 재사용 권고 |
| **S7** 〔r4.1 신설·F5〕 | **테스트 인프라 결합** — `e2e/slicea/harness_test.go:454`(raw `INSERT INTO k8s_cluster` 평문 kubeconfig §4.4)·`:462`(backfill UID `sha256("backfill\|k8s_cluster\|id")` 도출 — S1b가 끊는 결합)·`backfill_test.go:22`(k8sClusterDDL)·`:56`(Table("k8s_cluster") Create)·`secretv2_test.go:280`(Row 8 하드코딩)·`internal/api/v2/infra_test.go:77`(SourceModel fixture)·`main_sync.go:91`(**runSyncCLI가 RunK8sBackfill 선행 호출 — S1c 제거 시 sync CLI 수정 필수**) | drop 시 e2e·단위 테스트 전부 실패 — I-b 전에 재설계 없으면 테스트 스위트 붕괴 | **I-a 재설계** — harness 등록을 register-k8s CLI 경유 또는 provider_connection 직접 삽입으로 전환·backfill_test·secretv2·infra_test fixture 갱신·`main_sync.go:91` backfill 호출 제거(§3.3 배치) |

**S5 전환의 보안 계약**: 평문 kubeconfig는 `SecretRef` 복호로만 조달(mapping.md §6) — `GetK8sCluster`
교체 구현은 봉인 해제를 메모리 내 국한·로그·에러 노출 금지. backfill이 만든 기존 커넥션(dev 2건)은
`sourceModel="k8s_cluster"`로 조회 가능하므로 폴백 없는 1:1 대체가 성립한다. 신규 CLI 등록 체인은
`register-k8s` UID 도메인이므로 `GetK8sCluster` 교체 시 **두 체인을 모두 조회**한다(sourceModel 무관
`provider_connection.provider="kubernetes"` 조회 권고 — 구현 시 확정).

**〔r4.1·F6〕 S5 필드 조달 계약**: 교체 구현은 `model.K8sCluster` 반환에 필요한 **전 필드를
provider_connection에서 조달**해야 한다 — 특히 `ConnectionMode`·`GatewayID`(register-k8s 규약의
`--connection-mode`·`--gateway-id` 저장처를 `ConfigJSON` 고정 키로 못박는다). 누락 시
`newK8sHTTPClientForCluster`(`k8s.go:198`)·`dialThroughGateway`(`k8s_transport.go:164` — mode≠gateway
또는 GatewayID nil이면 direct dial)가 **gateway 모드를 direct로 강등해 금지된 사설 kubeconfig 주소
직접 dial**을 유발한다(`k8s_terminal.go:248` 게이트웨이 재작성 동일 의존). 소비자 실사용 필드 전수는
I-a census ②로 재확인 후 계약에 열거한다.

**착수 조건**: H 완료 → I-a(S1a·S1b·S1c·S3·S4·**S5·S6·S7**) → 파괴 전 백업(C34) → **사용자 명시 승인
(재상신 필수 — §14.2-1)** → I-b drop.

### J10 — I 설계: versioned drop + 승인 게이트 〔r4 신설·r4.1 R-C·R-L·F4 보강〕

- **I8-b 상환**: v1 AutoMigrate(`store/migrate.go:52` `&model.K8sCluster{}`)에서 k8s_cluster 제외 +
  **versioned drop 마이그레이션** 작성 — 파일명 확정: `internal/infra/migrate/step0007_drop_k8s_cluster.go`
  + `migrations.go` 등록 1행(선롄 step0006 — `migrations.go:40`). 러너는 별도 CLI가 아니라
  `migrate.Run`(`runner.go:48`)이 `main.go:93`·`main_sync.go:77` 기동 시 자동 순방향 실행 — 적용 확인은
  `go test ./internal/infra/migrate/` 확장(C66). drop DDL은 데이터 마이그레이션과 같은 파일에: backfill
  체인 잔여 커넥션 행의 `stale_source=true` 처리(삭제 아님 — §5.4b 원칙).
- **〔R-C〕 S3 칼럼 drop 동반**: 같은 step0007에 `asset_service.k8s_cluster_id`·
  `ops_application_environment_binding.k8s_cluster_id` 칼럼 drop을 포함한다(FK 0건 실측 — 데이터
  마이그레이션 불요). `asset_service.go:96`의 map 키 `"k8s_cluster_id"` 쓰기 경로는 I-a에서 선행 정리
  (C67′ 단얫).
- **〔F4〕 승인 게이트 위치 — I-b 직전 1곳으로 통일**: I-a(결합 제거·S5~S7 전환)는 **H2 완료 후 착수
  가능한 가역 PR들**이며 별도 승인을 요구하지 않는다. I-b(drop) 착수 전 승인 재상신 문서에서 I-a 배치
  전수를 최종 확정한다(§4 그래프·§14.2-1과 정합 — r4 초안 §3.3의 "I 착수 전" 모호 표기 정정).
- **되돌림 마이그레이션은 작성하지 않는다**〔r4.1·R-Q〕 — 러너는 순방향 전용(Down 심볼 부재 실측)이며
  부활물은 영구 미사용 빈 테이블이 된다. 롤백 = 백업 restore(C34) 단일 경로 + CLI 재등록으로 재생성
  (실데이터 dev 2행). §11 갱신.
- 백업 restore가 유일 수단은 아니다(실데이터 dev 2행 — CLI 재등록으로 재생성 가능).

---

## 3. 변경 범위 (블록 2 배치표 — 6축 census 반영)

### 3.1 Phase G — 읽기 3건 소스 전환

**G0 (플래그 도입·V2 경로 병렬)** — 4파일:

| 파일 | 변경 |
|---|---|
| `backend/service/k8s_projection.go` 〔신규〕 | `projectK8sClusterList`·`projectK8sClusterInfo`·`projectK8sClusterDetail` — provider_connection·ProjectResources·AssembleK8sClusterDetail 소비 (P 산출물 무변경) |
| `backend/service/k8s.go` | `ListK8sClusters`(:16)·`GetK8sClusterDetail`(:142) 내부 `V2_READ_SOURCE_K8S` 분기 (캐시 구조 무변경) |
| `backend/controller/k8s.go` | `:32`(Info)만 `projectK8sClusterInfo` 직접 호출 — `GetK8sCluster` 12소비자 무변경 |
| `backend/service/k8s_projection_test.go` 〔신규〕 | 3함수 단위 테스트 (proj 테스트 계열 신규 — Z 승계 아님) |

**G1 (플래그·legacy 제거·S2 종결)** — 3파일:

| 파일 | 변경 |
|---|---|
| `backend/service/k8s.go` | `V2_READ_SOURCE_K8S` 분기·`getK8sClusterDetailUncached`(:184) 제거 — V2 단일 경로. `GetK8sCluster`(:29)는 **존치**(S5 — I-a 소관) |
| `backend/main_compare.go` | k8s 페어링·`legacyCaptureFromDetail` 제거(S2 조기 상환) — `--cluster` 플래그 폐지·CLI 종결. cloud compare 무관 |
| `backend/main_compare_test.go` | 종결 정합 갱신 |

### 3.2 Phase H — 쓰기 중단 + register-k8s CLI

**H0 (CLI 신설)** — 3파일: `backend/main_register_k8s.go` 〔신규〕·`backend/main.go`(dispatch 1블록,
`register-pve` :71-72 직후)·`backend/main_register_k8s_test.go` 〔신규〕.
**인계 §4 규약 verbatim (구현 프롬프트에 원문 이관)**:

| 항목 | 규약 |
|---|---|
| 입력 | `--name`(upsert 멱등 키·필수) · `--kubeconfig <path>`(필수) · `--env` · `--connection-mode direct\|gateway` · `--gateway-id` · `--monitor-datasource-id` · `--insecure-tls` |
| kubeconfig 봉인 | 파일을 읽어 **즉시** `util.EncryptSecretV2` → `secret_ref` 행. **평문은 어디에도 남기지 않는다**(백필의 P-class 예외 `backfill.go:129`는 승계하지 않는다 — 신규 경로는 처음부터 봉인) |
| `provider_context` | `SourceKeyUIDSalted(name, "context")` 형태로 1:1 생성. **`k8s_cluster` 파생 UID(`SourceKeyUID("k8s_cluster", id)`)를 쓰지 않는다** — S1b가 그 결합을 끊으라고 요구한다. 실제 구현은 pve 선례의 register-scoped 도메인(`register-k8s\|name\|salt` sha256[:32]) |
| `provider_credential_binding` | purpose `inventory` 1건 (오퍼레이션이 같은 자격을 쓰면 재사용) |
| 멱등 | `--name` 동일 시 upsert. 재실행이 SecretRef를 재봉인하되 UID는 유지 |
| 검증 | 등록 직후 `Validate` 내장 호출 → `/version` 200 확인 실패 시 롤백(트랜잭션) |
| 금지 | 시크릿 값을 인자·로그·에러에 노출하지 않는다. `--kubeconfig`는 **경로**만 받는다 |

**H1 (프론트 전환 — E2 UI 계약 승계)** — 3~5파일: `web/src/api/k8s.js`(쓰기 12함수 제거·V2 오퍼레이션
호출 추가 — `api/infra.js` 패턴 재사용. **`createK8sResourceYAML`은 create 라우트 존치(D-15)로 함께
존재** — 실측 v1 쓰기 호출 13개 중 12만 제거)·`web/src/views/assets/K8s.vue`(오퍼레이션 9종 4단 흐름
연결 — restart #60 패턴·create 소비 UI 존치)·`web/src/views/assets/K8sClusterManage.vue`(CRUD 폼
제거·CLI 안내)·
`web/src/utils/*-i18n.js` 해당 사전(4단 흐름 문구 재사용·cluster 안내 신규)·필요 시
`web/e2e/slice-a.spec.js` 보강. **승계 계약**: Idempotency-Key 건별 생성·폴링 2초/종단 정지/5분 탐색 링크
전환·승인 대기 배지·N건 일괄 분해(r3 §3.5 표 준용 — restart 확립 패턴의 기계적 확장).

**H2 (백엔드 삭제 — 원자 단위 ⚠ 9~12파일)**: `router/routes_v1_infra.go`(12행 — create 제외)·
`controller/k8s.go`(12핸들러)·`opdef/defs_monitor.go`(12 def)·`service/k8s.go`·`k8s_workload.go`·
`k8s_mutate.go`·`k8s_detail.go`(내부 호출자 0의 **11종** 메서드 + 전용 헬퍼 — `ScaleK8sWorkload`·
`RestartK8sWorkload` 존치)·`store/seed.go:375` 권한 3종 제거(R-H)·골든 2종(452→**440**·292→**280**)·
`router/routes_inventory_test.go`(:130 449→**437**)·`authz_replay_test.go`(:235 439→**427 (414 v1 +
13 v2)**·:353 244→**232**)·e2e 갱신.
E1 8파일 선례와 같은 성격의 ≤5 규칙 의도된 편차 — 하나라도 빠지면 빌드·골든·테스트가 깨지는 원자 단위.

### 3.3 Phase I — k8s_cluster drop (계획만)

**I-a (결합 제거·S5~S7 전환)** — ≤5파일 규칙 준수 복수 PR: `internal/infra/inventory/backfill.go`(k8s
경로 제거 — RunK8sBackfill·markStaleSources·SourceKeyUID k8s)·`backend/main_sync.go:91`(runSyncCLI의
backfill 선행 호출 제거 — S7)·`backend/service/k8s.go`(GetK8sCluster 본체 provider_connection+SecretRef
전환 — 필드 조달 계약 §J5 F6)·`gateway.go`·`monitor.go`·`platform.go`(S6 5곳 등가 조회 교체)·
`backend/util/secret_registry.go`(Row 8 제거)·`backend/model/asset.go`·`model/ops.go`·`model/k8s.go`
(gorm 매핑)·`e2e/slicea/harness_test.go`(등록 경로 재설계 — S7)·`backfill_test.go`·`secretv2_test.go`·
`internal/api/v2/infra_test.go`(fixture 갱신 — S7) 등. I-a는 **H2 완료 후 착수 가능한 가역 PR** —
최종 파일 배치는 I-b 착수 전 승인 재상신 문서에서 확정한다(F4·§J10).
**I-b (drop 실행)**: step0007 마이그레이션(테이블 + S3 칼럼 2개 drop) + 백업(C34) + FK 확인(C35) — §J10.

### 3.4 절대 건드리지 않는 것

`internal/infra/**`·`internal/tasks/**`(보존 제약 #6 — G의 `k8s_projection.go`는 **service 소유 파일**이
inventory를 **호출만** 한다. **유일 예외는 §12 #6 (a)·(b) 2건** — H0 루트 CLI·I-a backfill.go k8s 경로
제거〔r4.1·R-G〕)·`internal/infra/metrics/**`·`inventory/projection.go` diff 0·
**P 산출물 무변경**(`k8sassembly*`·`compose*`·`mapping.md`·`contracttest` — 소비만)·라이브 패스스루 읽기
17 + `pod/terminal/ws`(§J3 존치)·`internal/api/v2/**`(E0 `connectionUid` 완결 — 추가 변경 금지).
DB 스키마는 I-b까지 불변(G·H·I-a 전부 칼럼 0).

---

## 4. Phase 분해

```
[블록 1·P 완결] ──▶ G0 ─▶ G1 ─▶ H0 ─▶ H1 ─▶ H2 ─▶ [I-a 복수 PR] ─▶ [승인 게이트] ─▶ I-b
                   (step3)      (CLI)  (프론트) (step4b)  (step5 선행)   (사용자)    (step5)
```

| Phase | step | 파일 | 성격 | 독립 검증 |
|---|---|---|---|---|
| **G0** 〔임계〕 | 3 | 4 | behavior-additive(플래그 기본 legacy) | C36①·C52·C53·C57·C69 |
| **G1** | 3 | 3 | behavior-changing(소스 단일화) | C36②·C54·C55·C56 |
| **H0** | 4b 선행 | 3 | additive(CLI) | C39′·C58·C59 |
| **H1** | 4b | 3~5 | behavior-changing(프론트) | C60·C42·C12·C61 |
| **H2** 〔임계〕 | 4b | 9~12 ⚠ | behavior-changing(삭제 원자) | C37·C38·C40·C62·C24·C7 |
| **I-a** | 5 선행 | 복수 PR | 결합 제거·S5~S7 전환 | C63·C64·C41·C19 |
| **I-b** | 5 | 1~2 | 파괴(마이그레이션) | C34·C35·C66 — **승인 게이트 후** |

**임계경로(직렬 신중)**: G0(전환 배선 — 응답 형상 동일성이 프론트 무변경의 전제) → H2(13→12건 동시
삭제·골든·테스트·메서드 처분이 한 PR) → I-a S5(GetK8sCluster 단일 착지점 — 12소비자 전부가 의존).
H0·H1은 G1 완료 후 병렬 가능(H1은 H0 무관, H2는 둘 다 완료 후).

**순서 논거**: E0→E2→E1 승계 — CLI(H0)·프론트(H1)가 먼저 있어야 백엔드 삭제(H2)에 기능 공백이 없다.
revert는 역순(H2→H1→H0→G1→G0). **Phase I는 r4에서 계획만 수립** — 착수 승인은 사용자 몫(§14.2-1).

---

## 5. 파일 크기 (800 하드캡)

블록 2 신규 파일: `k8s_projection.go`(예상 ≤350)·`main_register_k8s.go`(선례 `main_register_pve.go`
447줄 — kubeconfig 단일 자격이라 예상 ≤400)·테스트 2종. 전부 650 경고선 이내 설계.
`service/k8s.go` 390(G0 분기 추가 후 G1 uncached 제거로 ±수십 줄)·`controller/k8s.go` 606(H2 12핸들러
제거로 ~-250)·`routes_v1_infra.go`(현재 wc 실측치에서 -12행) — 모두 cap 하회 유지. C65가 전수 잠금.

---

## 6. 검증 요구 (claims — 블록 2판)

**블록 1 완결 claims(C1~C33·C44~C51·카나리)는 전부 검증 완료 — `p6-state.json` claims 배열 참조.
아래는 블록 2가 승계·신설하는 것만 게재한다.** vacuity 금지(r3 원칙 승계): "대상 존재 증명 → 0" 2단 형태.
전제 `cd /mnt/d/DEV/acc0mplish/ops-admin`. Phase 시작 시 `git tag p6-<phase>-base`.

```
--- 블록 2 공통 ---
C7  [각 Phase 종료] cd backend && go test ./... -race -count=1 → ok
C11 [각 Phase 종료] python3 scripts/secret-scan.py → exit 0 (H0은 kubeconfig 취급 — 필수)
C12 [G0·G1·H1] cd web && bun install --frozen-lockfile && bun run build → exit 0 (G는 프론트 무변경 — 무동작으로 exit 0)
C19 [G·H 전 Phase] git diff --stat p6-<phase>-base -- backend/internal/infra backend/internal/tasks → 출력 없음
      (H0의 main_register_k8s.go는 backend 루트 CLI — #6 허용 예외. internal/api/v2/도 무변경: git diff --stat p6-<phase>-base -- backend/internal/api/v2 → 출력 없음)
C20 [G·H 전 Phase] git diff --stat p6-<phase>-base -- backend/store backend/model backend/internal/infra/migrate → 출력 없음
C42 [H1] node scripts/check-i18n-parity.mjs → exit 0 (repo root — 사전 21종 패리티)
C43 [각 Phase 종료] 한자 혼입 금지(korean-localization-guard.yml:13 인라인 python 원문) → PASS
C65 [블록2 전 Phase — 블록 2가 생성·성장시키는 파일만〔r4.1·R-M〕] wc -l backend/service/k8s_projection.go
      backend/main_register_k8s.go backend/main_compare.go backend/controller/k8s.go backend/service/k8s.go
      | grep -v total | awk '$1>800' → 0행 · awk '$1>650' → 초과 시 같은 Phase에서 분할.
      기존 `k8s_*_test.go` 3종(build_net 652·path 651·fetch 650)은 Z·P 소유로 블록 2 무변경 계약 —
      측정에서 제외하고 diff 0으로 별도 잠금(C19 계열 — git diff --stat p6-<phase>-base -- backend/service/k8s_build_net_test.go backend/service/k8s_path_test.go backend/service/k8s_fetch_test.go → 출력 없음)

--- Phase G (읽기 3건 전환) ---
C36 [G] 2단: ① G0 후 grep -rn 'V2_READ_SOURCE_K8S' backend/ → 1 이상(플래그 존재 증명) ② G1 후 같은 명령 → 0행
      그리고 grep -c 'ProjectResources\|AssembleK8sClusterDetail' backend/service/k8s_projection.go → 2 이상
C52 [G0] V2 경로 착지: wc -l backend/service/k8s_projection.go → 1 이상(파일 존재) 이고
      grep -c 'V2_READ_SOURCE_K8S' backend/service/k8s.go → 2 이상(List·Detail 2분기)
C53 [G0 종료 — 단일 1회] 전환 동치(§15 승계 — **G0 시점에만 실행 가능**: G1의 산출물이 compare 종결
      자체라 그 뒤로는 CLI가 존재하지 않는다〔r4.1·F11〕): kubectl --context kind-v2-p2 -n v2-seed get cronjob seed-cron → NotFound 확인 후
      cd backend && go run . sync-inventory --connection b23f3f6ab673c31ead416fe4948a9124 && go run . compare-inventory --cluster 1; python3 -c "import json,glob,os; f=max(glob.glob('data/compare/1/*/*.json'),key=os.path.getmtime); print(json.load(open(f))['verdict'])" → pass
C54 [G1] legacy 라이브 소스 제거: grep -c 'getK8sClusterDetailUncached' backend/service/*.go → 0 이고
      grep -c 'func (s \*Service) GetK8sClusterDetail' backend/service/k8s.go → 1 (메서드는 V2 단일 경로로 존속)
C55 [G1] S2 종결: grep -c 'legacyCaptureFromDetail\|GetK8sClusterDetail' backend/main_compare.go → 0 이고
      go vet ./... (backend) → ok. compare-inventory-cloud dispatch(main.go:64-66)은 무변경
C56 [G 전 Phase] 프론트 무변경: git diff --stat p6-<phase>-base -- web → 출력 없음
C57 [G 전 Phase] 골든 불변: grep -c . docs/security/route-inventory.txt → 452 · docs/security/sensitive-routes.txt → 292
C68 [G0] 캐시 구조 무변경: git diff p6-G0-base -- backend/service/service.go | grep -E '^[-+]' | grep -vE '^(\+\+\+|---)' → 0행

--- Phase H ---
C37 [H2] 삭제 전 census(리뷰 체크리스트 — 명령 아님. #15 강제): 12라우트 각각 §2.0 6축 표가 PR 설명에 첨부됐는지 리뷰어 확인
C38 [H2] 골든 대조: grep -c . docs/security/route-inventory.txt → 440 · docs/security/sensitive-routes.txt → 280
      (create 존치 — I10 착지 시 439·279)
C39′ [H0] register-k8s 왕복(compare 종결 후 판정 수단 — r3 C39 재설계·r4.1 R-J 입력 계약화):
      kubeconfig fixture는 저장소에 없으므로 kind에서 조달: kind get kubeconfig --name kind-v2-p2 > /tmp/p6-h0-kubeconfig (파일 존재 test -s 확인) ·
      cd backend && go run . register-k8s --name p6-h0-check --kubeconfig /tmp/p6-h0-kubeconfig → exit 0 이고 stdout JSON report(pveRegisterReport 선례 :60,:140 — register-k8s도 report를 stdout에 출력한다)에서 uid 취득: go run . register-k8s ... 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin)['connectionUid'])" ·
      재실행 → exit 0(멱등) 이고 uid 동일 · go run . sync-inventory --connection <uid> → exit 0 ·
      리소스 노출: curl -s 'http://127.0.0.1:8080/api/v2/infra/resources?connectionUid=<uid>&pageSize=1' | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['data']['totalCount'] if 'totalCount' in d.get('data',{}) else len(d.get('data',{}).get('items',[])))" → 1 이상
      (서버 미기동 시: sync exit 0 + provider_connection 행 존재를 main_register_k8s_test.go 단얫으로 대체 — PR에 어느 쪽으로 판정했는지 기재)
C58 [H0] CLI 봉인: grep -c 'EncryptSecretV2' backend/main_register_k8s.go → 1 이상 ·
      grep -rn 'kubeconfig 내용\|KubeConfig:' backend/main_register_k8s.go → 0행 ·
      grep -n 'fmt.Print\|log.Print' backend/main_register_k8s.go 이 중 material 변수 출력 0건 (리뷰 확인 — C11 병행)
C59 [H0] register-scoped UID: grep -c 'SourceKeyUID("k8s_cluster"' backend/main_register_k8s.go → 0 이고
      grep -c 'register-k8s|' backend/main_register_k8s.go → 1 이상
C60 [H1] 프론트 전환 2단: ① 착수 전 grep -cE "http\.(post|put|delete)\('/api/v1/k8s" web/src/api/k8s.js → 13(존재 증명 — 실측 2026-09-10)
      ② 완료 후 같은 명령 → 1(create 존재 증명 — createK8sResourceYAML) 이고 grep -c 'operations/' web/src/api/k8s.js → 2 이상 ·
      grep -rln 'addK8sCluster\|updateK8sCluster\|deleteK8sCluster' web/src/views → 0행
C61 [H1] E2E: cd web && bun run e2e → exit 0 (kind 필요 — 로컬 미가용 시 사유 PR 기재. **회피 시 C60②
      grep 단얫과 C12 build는 필수로 돌려 프론트 배선 검증 공백을 막는다**〔r4.1·R-N〕. slice-a·c의 restart
      단얫은 이미 V2 흐름 — 오퍼레이션 9종 진입 지점 셀렉터 보강 필요 시 같은 PR)
C62 [H2] 서비스 메서드 처분 〔r4.1·F2 — 패턴에 create 포함·합산 산술〕: ① 착수 전
      grep -h -c 'func (s \*Service) \(CreateK8sCluster\|UpdateK8sCluster\|DeleteK8sCluster\|UpdateK8sNodeLabels\|UpdateK8sService\|UpdateK8sResourceYAML\|DeleteK8sResource\|UpdateK8sIstioTraffic\|UpdateK8sHTTPRouteTraffic\|UpdateK8sWorkloadImages\|UpdateK8sWorkloadResources\|CreateK8sResourceYAML\)' backend/service/k8s*.go | awk -F: '{s+=$NF} END {print s}' → **12**(존재 증명 — dry-run 실측 2026-09-10: k8s.go 4+detail 1+mutate 5+workload 2)
      ② 완료 후 같은 명령 → **1**(create 존치 증명) 이고
      grep -c 'func (s \*Service) ScaleK8sWorkload\|func (s \*Service) RestartK8sWorkload' backend/service/k8s_workload.go → 2(존치 증명 — C62① 패턴 외)
      그리고 cd backend && go build ./... → ok (integration_ai.go:1732,1734 존치 메서드 참조 무결)
C24 [H2 1회 재징수] 라우트 카나리(블록 1 미징수 상환 — p6-state.json "next" 승계. **r3 앵커
      `authGroup.GET("/profile"`는 현재 HEAD 0매치 — 실측 형태는 `routes_v1_system.go:19`의
      `g.GET`라 앵커를 교정했다**〔r4.1·F1〕. 앵커는 전 router 디렉터리에서 유일(실측). dry-run PLANT_OK
      2026-09-10 확인):
      S=$(mktemp -d) && cp -r . $S/ && cd $S/backend && sed -i 's|g\.GET("/profile", ctl\.Profile)|g.GET("/profile", ctl.Profile)\n\t\tg.POST("/canary/unlisted", func(c *gin.Context) {})|' router/routes_v1_system.go && { grep -q '/canary/unlisted' router/*.go || { echo PLANT_FAILED; exit 1; }; }; ! go test ./router/ -run TestOperationTableCoversRouter -count=1 && echo CANARY_OK → CANARY_OK

--- Phase I (착수 승인 후) ---
C40 [H2] 하드코딩 수치 재수정·음의 증명〔r4.1 신설 — r3 C40 부활·R-B〕:
      cd backend && grep -n '!= 449\|contract is 449' router/routes_inventory_test.go → 0행(437로 변경) ·
      grep -n '!= 439\|426 v1' router/authz_replay_test.go → 0행(427·414 v1로) ·
      grep -n '!= 244\|244 baseline' router/authz_replay_test.go → 0행(232로). 그리고
      grep -n 'contract is 437\|427 (414 v1 + 13 v2)\|!= 232' router/routes_inventory_test.go router/authz_replay_test.go → 3건 전부 양성 존재
C63 [I-a] backfill k8s 경로 제거: grep -c 'RunK8sBackfill\|markStaleSources\|SourceKeyUID("k8s_cluster"' backend/internal/infra/inventory/backfill.go → 0 이고
      grep -rn 'RunK8sBackfill' backend/main_sync.go → 0행(S7 — runSyncCLI 호출부 제거) ·
      grep -c 'func RunCloudBackfill' backend/internal/infra/inventory/backfill_cloud.go → 1(cloud 존치 증명 — 'cloud' 문자열 grep은 파일명상 항상 매치라 vacuous, 폐기〔r4.1·R-K〕) ·
      cd backend && go test ./internal/infra/inventory/ -count=1 → ok
C64 [I-a] S5·S6 전환: ① 존재 증명 — grep -rc 'func TestGetK8sCluster' backend/service/ | awk -F: '{s+=$NF} END {print s}' → 1(착수 전 deliverable 존재 — 무매칭 ok 방지〔r4.1·F10〕) ·
      grep -n 'provider_connection\|ProviderConnection' backend/service/k8s.go(GetK8sCluster 본문) → 1 이상 ·
      grep -rn 'Table("k8s_cluster")\|&model\.K8sCluster{}' backend/service/ backend/internal/infra/inventory/ → backend/service/k8s_projection.go 제외 0행(S6 — gateway·monitor·platform 5곳 포함 전수〔r4.1·F9〕) ·
      cd backend && go test ./service/ -run TestGetK8sCluster -count=1 → ok (신규 단얫 — 두 체인 조회·필드 조달)
C41 [I-a] S4: grep -c 'k8s_cluster' backend/util/secret_registry.go → 0 이고 cd backend && go test ./util/ -count=1 → ok (모집단 14→13)
C34 [I-b] 파괴 전 백업: mysqldump --single-transaction -h127.0.0.1 -uroot -p123456 ops_admin > /tmp/p6-pre-drop.sql && test -s /tmp/p6-pre-drop.sql && echo BACKUP_OK → BACKUP_OK
C35 [I-b] 인바운드 FK 0: docker exec ops-admin-mysql-dev mysql -uroot -p123456 -N -e "select count(*) from information_schema.key_column_usage where table_schema='ops_admin' and referenced_table_name='k8s_cluster';" → 0
C66 [I-b] AutoMigrate 제외·versioned drop〔r4.1·R-L·R-C〕: grep -n 'K8sCluster' backend/store/migrate.go → 0행(:52 제외) 이고
      ls backend/internal/infra/migrate/step0007_drop_k8s_cluster.go → 파일 존재 이고
      grep -c 'step0007DropK8sCluster' backend/internal/infra/migrate/migrations.go → 1(등록) ·
      cd backend && go test ./internal/infra/migrate/ -count=1 → ok(러너는 main.go:93·main_sync.go:77 기동 시 migrate.Run 자동 실행 — 별도 CLI 진입 없음) ·
      S3 칼럼: grep -c 'k8s_cluster_id' backend/internal/infra/migrate/step0007_drop_k8s_cluster.go → 2 이상(asset_service·ops binding 칼럼 drop) ·
      grep -n 'k8s_cluster_id' backend/service/asset_service.go → 0행(쓰기 경로 선행 정리)
C69 [G0] info 마스킹〔r4.1 신설 — R-I〕: cd backend && go test ./service/ -run TestProjectK8sClusterInfo -count=1 -v → ok ·
      grep -c 'KubeConfig:' backend/service/k8s_projection.go → 0행(kubeConfig 필드에 평문 주입 부재 — 구조체 리터럴이 제로값 유지)
```

### 6.1 포함관계 (리뷰어는 상위 1개만) 〔r4.1·F8 재작성 — 허위 포함 삭제〕

실제 포함만 남긴다: `C19⊃C20`(internal/infra·tasks 무변경이 store·model·migrate 무변경을 포함 — 단
H0 CLI·I-a backfill 예외 시점엔 병행 확인) · `C56⊂C12`(프론트 무변경이면 build은 무동작 통과 — G만).
**r4 초안의 `C7⊃C11·C12·C61`은 허위였다** — `-race` 스위트는 secret-scan·bun build·playwright를
실행하지 않는다(C11·C12·C61은 독립 필수). 각 Phase 종료 최소 회전 세트: **C7·C11·C19·C65** (+H1
C12·C42·C61). C36②가 C52를, C40이 C38의 수치 원천이 된다(규범 수치는 C38·C40이 소유 — 본문
타 참조는 전부 이 2개로 귀결〔r4.1·R-R〕).

---

## 7. §19.1·§20·§17.2·§21 이탈 표 + 개정 판정

r3 D-1~D-14 확정 승계(Phase F #58 착지 — `deviation recorded` 8건). **블록 2 신설**:

| # | 조항 | 적용 | 사유 | 처분 |
|---|---|---|---|---|
| **D-15** 〔r4〕 | §19.1 step 4 "legacy write paths stop" | **부분 적용 12/13** | create 1건은 uid-스코프 모델 부적합(I-P1) — §16.1 커넥션-스코프 라우트가 전제 | 이탈 기록 + **I10 이월**(착지 시 13/13 완결·골든 439) |
| **D-16** 〔r4〕 | §19.1 "§15 comparisons re-run after each step" | G1 이후 적용 종료 | compare k8s 페어링이 legacy 캡처 소스(uncached)에 의존 — 소스 전환 완료로 캡처 불가 | 이탈 기록 — H·I 검증은 V2 왕복 claims(C39′ 등)로 대체 |

§17.2(301 route migration)·§17.3(메뉴 시드·i18n) — **I7 상환**: 실측 결과 읽기 3건은 URL·응답 형상
불변(G 프론트 0)이므로 §17.2의 301 체크리스트는 **k8s 읽기에 적용 없음**. 쓰기의 프론트 전환은 v1 API 호출
제거일 뿐 라우트 301이 아니다(§17.2는 "M2 cutover: views replaced + 301" — 본 계획은 라우트 유지·소스만
전환). 메뉴 시드·i18n 21사전은 H1이 C42·C60으로 잠근다. **§21 PR 34("feat(web): legacy route flips +
301s")의 블록 2 실질은 H1** — 별도 301 이탈행 불요.

---

## 8. 선행 과제 P — 완결 기록

P 트랙 완결(#62~#74, 2026-09-10): 43필드(④ certificates A′-1 포함)·수집기 5종+부수(RS·Job·CronJob·
GatewayAPI·HTTPRoute·Endpoints·VirtualService 앵커)·kind 어휘 4종(`Phase6ResourceKindExtensions`)·
조립 `k8sassembly{,_sections,_workloads,_certificates}.go`·오퍼레이션 10종 DefTable 잠금
(`TestOperationDefTable`·compose 4+9=13)·Z 오라클 스왑 28(원장 `v2-swap-ledger.txt` 28행)·
compare 3회 pass·판정 ①②③⑤④ 전부 결착. **블록 2가 승계 소비할 산출물**: `AssembleK8sClusterDetail`·
`ProjectResources`·`connectionUid` 필터(E0)·V2 오퍼레이션 10종·register CLI 규약(인계 §4). 상세는
`p-parity-plan.md`·state.json `ptrack` 참조.

---

## 9. 관측

G1·G2(v1 라우트 메트릭 0종·GET 미기록)은 관찰 기간 소멸로 결함 아님(§7 D-6·D-11 복원 지점 승계).
G3~G5는 `metrics-ext-plan.md` 소관 유지. **G1의 compare 종결(D-16)로 §15 아티팩트 적립도 종료** —
`data/compare/` 기존 아티팩트·waiver JSON 삭제 금지(보존 제약 #5 승계).

---

## 10. 위험 등록부 (블록 2 신설 — r3 R1~R23 이력 승계)

| # | 위험 | 가능 | 파급 | 완화 | 검출 |
|---|---|---|---|---|---|
| **R24** | H2 12건 동시 삭제의 폭발 반경 (골든 2종·opdef·컨트롤러·서비스 4파일·테스트 수치·프론트 잔여 참조가 한 PR) | 중 | 높 | E0→E2→E1 순서 승계(H0→H1→H2) — 삭제 시점에 대체 경로 확정. 원자 단위 편차 사전 승인. 6축 census(C37) | C38·C40·C62 |
| **R25** | create 존치(D-15)가 "step 4 완결"로 오인됨 | 중 | 중 | I10 이월 대장 명기·골든 440 기대값에 주석(439는 I10 후)·§7 이탈 기록 | 리뷰·C38 |
| **R26** | K8sClusterManage.vue CRUD 폼 제거로 클러스터 관리 UX 상실 (CLI 이전) | 높 | 중 | 사용자 확정 사항으로 게이트 — H1 착수 전 안내 문구·CLI 사용법 PR 설명 첨부. §16.1 POST 구현(I3)이 UI 회귀 경로 | C60·리뷰 |
| **R27** | G1 compare 종결이 §15.4 게이트 수단을 소거 — H·I의 동치 판정 공백 | 중 | 중 | D-16 이탈 기록 + V2 왕복 claims(C39′·C63·C64)가 대체. C53을 종결 직전 2회 판정으로 마지막 동치 확정 | C53·C39′ |
| **R28** | I-b drop의 되돌림 불가 | 낮 | 높 | C34 백업 + 마이그레이션 재실행·CLI 재등록 재생성 경로 + **사용자 승인 게이트**(§14.2-1) | C34·C35 |
| **R29** | K8s.vue 1파일에 오퍼레이션 9종 전환 — 리뷰 불가 규모 | 높 | 중 | restart(#60) 확립 패턴의 기계적 확장 — PR 설명에 op별 매핑표(§J9 표 재사용)·커밋 op별 분리 | 리뷰 |
| **R30** | S5 GetK8sCluster 전환이 12소비자 중 미발견 경로를 깨뜨림 | 중 | 높 | 1착지점(함수 본체) 교체 — 소비자 시그니처 무변경. 소비자 목록 12곳을 PR에 전수 열거(census ②원문) | C64·C7 |
| **R31** | H0 CLI가 backfill 체인과 이중 커넥션 생성 (같은 클러스터 2행) | 중 | 저 | CLI는 register UID 도메인 — backfill 체인과 무충돌. I-b 마이그레이션이 backfill 체인 stale 처리(§J10) | C39′ |

**임계경로(블록 2)**: G0 배선(응답 형상 동일성) → H2 원자 삭제 → I-a S5 단일 착지점 전환.
**카나리 재징수 판정**: C24(라우트)는 **H2에서 1회 의무 재징수**(블록 1 미징수 상환 — 라우트 대량 삭제
직후 탐지기 생존 확인). C31(secret-scan)·C32(arch 5종)은 블록 2가 각각 kubeconfig(H0)·새 CLI 파일을
추가하므로 **H0에서 1회 권장 재징수**(미징수 시 PR에 사유 — p6-state "권한 거부" 이력 승계).

---

## 11. 롤백

G0은 플래그 기본 legacy — revert 무해. G1은 소스 단일화+compare 종결이라 원자 revert(비교 CLI가
함께 부활). H0 CLI는 additive — 단독 revert 무해. H1 프론트는 E2 선례 — revert 시 v1 API가 H2 이전엔
살아있으므로 순서만 지키면 무해. **H2는 원자 revert**(골든·테스트·메서드가 같은 PR — 부분 revert 불가,
r3 S-1 논거 승계). I-a 각 PR 독립 revert. **I-b 롤백 = 백업 restore(C34) 단일 경로 + CLI 재등록으로
재생성** — 되돌림 마이그레이션은 작성하지 않는다〔r4.1·R-Q: 러너 순방향 전용·부활물은 영구 미사용
빈 테이블 — §J10〕. 전체 역순: I-b→I-a→H2→H1→H0→G1→G0.

---

## 12. 보존 제약 (③구현 프롬프트에 **verbatim** 복사 — 블록 2판)

1. **읽기 전환(G)은 응답 형상을 바꾸지 않는다** — `K8sClusterDetail`·`K8sClusterView`·`K8sCluster`
   직렬화 결과는 소스와 무관하게 동일하다(운영 부재가 이 요구를 완화하지 않는다 — r3 #1 승계).
   **유일 예외(r4.1·R-I)**: `cluster/info`의 `kubeConfig` 필드는 `projectK8sClusterInfo`가 항상 `""`로
   반환한다(마스킹) — 현재 v1은 평문을 실으나 봉인 계약(#16)이 우선하고 유일 소비처(편집 폼)가 H1에서
   소멸한다. §J8 계약·C69가 잠근다.
2. **골든 2종의 허용 변경은 H2 선언분 12행뿐** (452→440·292→280). G·H0·H1·I-a는 **diff 0**. H2의 테스트
   하드코딩 3곳(437·427/414 v1 + 13 v2·232)은 **같은 PR에서 함께** 고친다.
3. **arch-boundary R2 통과.** 4. **secret-scan 통과.**
5. **§15.4 비교 게이트 — G1까지만 적용**: C53 판정 후 compare k8s 페어링 종결(D-16 기록). 기존
   아티팩트·waiver JSON 삭제 금지. **C53 실행 전 `seed-cron` 선행조건 확인**(§J6 승계).
6. **V2 무변경 — `internal/infra/`·`internal/tasks/`. 단, 예외 2건만 허용한다**: (a) backend 루트 CLI
   `main_register_k8s.go`가 `internal/infra`를 읽어 쓰는 것(선례 `main_register_pve.go`) — 인계 §4
   규약 구현 (b) **I-a의 `backfill.go` k8s 경로 제거**(k8s_cluster 소스 소멸 — drop의 직접 전제).
   **P 산출물(`k8sassembly*`·`compose*`·`mapping.md`·`contracttest`·`projection.go`)은 G·H·I 전
   Phase에서 수정 금지 — 소비만 한다.** `internal/api/v2/`도 무변경(E0 완결 — create 오퍼레이션화가
   필요해지면 별도 계획·승인, I10).
7. **라이브 패스스루 읽기 17건 + `pod/terminal/ws`는 무기한 v1 존치**(§J3·D-12) — G·H·I 어디서도
   건드리지 않는다.
8. **원격 CI 대기 금지** — §6 claims가 워크플로 2파일 12잡 전부의 로컬 대응이다.
9. **롤백 계약** — G·H는 PR revert(H2 원자), I-b는 백업 restore + 마이그레이션 재실행(§11).
10. **DB 파괴 금지** — Phase I-b의 drop은 **별도 승인 사항**(착수 전 재상신 필수). dev DB라도 파괴 전
    백업 필수(C34). I-a까지 스키마 불변.
11. **H2는 레거시 쓰기의 *경로*만 삭제하고 `ScaleK8sWorkload`·`RestartK8sWorkload` 메서드는 남긴다** —
    `integration_ai.go:1732,1734`가 호출하며 그 도메인은 OUT-OF-SCOPE다(J4 가 승계). **H0·H1이 H2보다
    먼저 머지된다.**
12. **전환용 임시 플래그 `V2_READ_SOURCE_K8S`는 블록 2 안에서 생겼다 사라진다**(C36).
13. **스펙 이탈은 조용히 두지 않는다** — D-15·D-16을 Phase F 착지분(§19.1 "Applicability" 문단)에
     추가 등재하는 후속 커밋을 H2·G1 PR에 각각 동반한다.
14. **삭제·이동 대상마다 §2.0의 6축 census를 수행하고 PR 설명에 표로 첨부한다.** 축이 하나라도
     미조사면 반려다. **"선언된 표면이 아니라 실제 결합을 센다."**
15. **Z 테스트·C47 baseline·v2-swap-ledger(28행) 불가침** — 블록 2는 서비스 메서드 삭제 시
     `char-exclude.txt`·baseline과 무관함을 확인 후 진행한다(삭제 메서드는 *Service 메서드 — 원래
     char 대상 아님).
16. **`GetK8sCluster` 전환(I-a S5)은 함수 본체 1착지점에서만** — 소비자의 시그니처·반환 타입은
     무변경(외부 소비 9 + 내부 3 + info 진입 1 + List 1 — §J5 S5 원문 좌표). 평문 kubeconfig는
     SecretRef 복호로만 조달하며 메모리 외 노출 금지(mapping.md §6).
     **임시 파일 예외〔r4.1·F7〕**: `opsPipelineKubeconfigFile`(`ops_application.go:1892`)가 kubectl
     전달용으로 kubeconfig를 `os.CreateTemp` 평문 파일로 기록하는 기존 경로는 존속을 허용하되 계약을
     같은 PR에 명시한다 — 권한 0600(`os.CreateTemp` 기본)·cleanup 함수 반환으로 즉시 삭제·수명은 단일
     kubectl 호출 세션 한정. 신규 임시 파일 경로의 추가는 금지.

---

## 13. 이월 대장 (블록 2판 갱신)

| # | 항목 | 재진입 트리거 | r4 처분 |
|---|---|---|---|
| I1 | 트래픽 중복 제거(step 2b) | 게이트웨이 hop seam 증명 + `internal/infra/` 변경 허용 | 유지 |
| **I2** | 블록 2 파일 단위 분해 | P1·P2 완료 | **본 r4로 상환 완료 — 폐쇄** |
| I3 | §16.1 `POST /provider-connections`·`/validate`·`/sync` 미구현 | H 착수 시 판정 | **판정: 블록 2 범위 밖 확정** — GET만 존재(`infra.go:86` 실측). CLI가 등록 경로 대체. POST 구현은 UI 회귀(R26 해소)와 create 오퍼레이션화(I10)의 전제로 별도 계획 |
| I4 | 읽기 경로 트래픽 계측 | 실사용 배포본 | 유지 |
| I5·I6 | `monitor.go` 4,798 등 하드캡 14건·§3.2 row 4·7·22 | OTEL 후속·M2 재판정 | 유지 |
| **I7** | §21 PR 34 프론트 301·§17.2·§17.3 메뉴 시드·i18n | G 완료 후 | **상환 — §7 재판정**: 읽기는 URL 불변(301 대상 없음)·쓰기는 라우트 유지 소스 전환. 실질(프론트 전환·i18n)은 H1이 흡수. **폐쇄** |
| **I8-a** | `phase4-plan.md:167,586` normalizer 관계 링크 | P1과 | P 완결로 승계 완료 — **폐쇄** |
| **I8-b** | v1 AutoMigrate → versioned(`phase1-plan.md:791` A9) | Phase I와 | **I-b 착지 계획 확정**(§J10·C66) — 유지 |
| **I9** | 쓰기 census 누락 2건 — `POST /console-sessions`(`sensitive-routes.txt:127`)·`GET /k8s/pod/terminal/ws` | H 착수 시 처분 확정 | **판정: k8s 쓰기 13건과 무관·v1 존치** — console-sessions은 console 도메인 공유 쓰기(권한 `assets:host:terminal\|assets:k8s:pod:terminal`)·ws는 GET(exec 채널). §3.2 row 16 OUT-OF-SCOPE 계열. H2에서 6축 census ③프론트 축으로 재확인만. **폐쇄** |
| **I10** 〔r4 신설〕 | create 오퍼레이션화 — §16.1 커넥션-스코프 apply 라우트 신설 + `POST /k8s/resource/yaml/create` 삭제 + 골든 440→439 | §16.1 POST 구현 계획 수립 시(I3와 동일 트리거) | D-15 이탈의 상환 항목 |
| I-P5 | gateway·httproute 등 비교 집합 승격 + CRD 설치 | M2 완전성 | 유지(P 계획 승계) |

---

## 14. 결정 기록 · 미해결

### 14.1 해소 (r4 결정 기록)

블록 1·P 전 판정 완결(§2.1) · **I2 폐쇄(본 r4)** · **I7 폐쇄(§7 재판정 — 읽기 URL 불변·쓰기 소스 전환)** ·
**I9 폐쇄(console-sessions·ws는 k8s 쓰기 13건 밖 — v1 존치)** · **I3 판정(블록 2 범위 밖 — CLI가 대체)** ·
**create 처분 = 12건 삭제 + 1건 존치 이월(D-15·I10)** · **S2 조기 이동(G1)** · **C39 재설계(compare
종결 후 V2 왕복 판정)** · **S5 신설·I-a 편입** · **카나리 C24 H2 재징수·C31/C32 H0 권장 판정**.

### 14.2 미해결 (사용자 에스컬레이션 후보)

1. **[Phase I-b 착수 시 재상신] `k8s_cluster` drop 실행 승인** — 본 r4는 계획만 수립. I-a 완료·백업
   확인 후 별도 재상신.
2. **[H1 착수 전 확인] K8sClusterManage.vue CRUD UI 제거 확정** — 클러스터 등록·편집·삭제가 CLI로
   이전되며 UI에서 사라진다(R26). 복구 경로는 I3(§16.1 POST) 구현.
3. **[I10 트리거] §16.1 POST /provider-connections 구현 승인** — create 오퍼레이화·UI 회귀의 공통
   전제. 본 계획 범위 밖(보존 제약 #6·R19 승계).
4. [G0 착수 전 확인 권고] AI 도구 경로 2건의 동반 전환 — `integration_ai.go:964`(List·`k8s_list_clusters`)
   ·`:966`(Detail·`k8s_cluster_overview`〔r4.1·R-P 추가〕). 응답 형상 동일로 무해 판정이나, AI 도구
   소비 경로의 사전 인지 요청.

---

## 15. r4.1 적대검토 처분표 (F1~F11·R-A~R-S — `p6-block2-adversary-r1.md`)

**필수 F 전부 적용** — F1(C24 앵커 `g.GET` 교정·dry-run PLANT_OK) · F2(C62 패턴 12종+합산+착수 전 12·후 1
— dry-run 실측) · F3(414 v1 3곳) · F4(승인 게이트 I-b 직전 1곳 통일·§J10) · F5(S7 테스트 인프라 신설·
main_sync.go:91 포함) · F6(S5 필드 조달 계약 — ConnectionMode·GatewayID ConfigJSON 조달·gateway 강등
방지) · F7(#16 임시 파일 예외 계약 0600·cleanup·단일 세션) · F8(§6.1 허위 포함 삭제·최소 회전 세트 재산정) ·
F9(S6 gorm 직접 조회 5곳·C64 정합) · F10(C64 존재 증명 ①단) · F11(C53 G0 단일 1회 재정의).

| 권고 | 처분 | 사유 (1행) |
|---|---|---|
| R-A 플래그 삭제 | **기각** | 스펙 §19.1 D-4 조건화("전환 검증용 플래그")의 구현 실체라 삭제 시 §7 재개정+사용자 재확정 파급 · env 기반 복귀(G0~G1 사이 결함 시 revert 없이 소스 복귀) 가치 잔존 · E-1 3단 원칙(필터→전환→삭제)과 동일 구조. 지적한 "on 작동 절차 전무"만 §J8 플래그 운용 절차로 보강 |
| R-B C40 부활 | 채택 | 음의 증명(구 수치 grep 0)이 H2 수치 갱신 누락을 기계 차단 — C40 신설 |
| R-C S3 칼럼 drop | 채택 | FK 0건 실측·같은 step0007에 칼럼 2개 drop·asset_service.go:96 선행 정리 C66 단얫 |
| R-D §3.5 자기완결 | 채택 | §3.2 H1에 `k8s.js:74-112` 선례 좌표 병기 |
| R-E 10종→11종 | 채택 | §3.2 H2 정정(§J9와 정합 — 13−Scale−create=11) |
| R-F 무변경 한정 | 채택 | §J8 "라우트·opdef 등록 무변경(컨트롤러 :32 본문은 교체)" |
| R-G §3.4 상호참조 | 채택 | #6 (a)·(b) 예외 상호참조 추가 |
| R-H seed 권한 | 채택(선택적) | seed.go:375 3종만 H2 동반 제거 — compose_k8s_ops v1 권한 재사용은 존치 |
| R-I kubeConfig 마스킹 | 채택 | 현재 v1이 평문 직렬화(실측)·봉인 계약 우선·유일 소비가 H1 소멸 — §J8 계약·C69·#1 예외 |
| R-J C39′ 입력 계약 | 채택 | kind kubeconfig 조달·stdout JSON report uid 취득 명시 |
| R-K C63 vacuous | 채택 | `func RunCloudBackfill` 존재 증명으로 교체(실측 1) |
| R-L C66 자기충족 | 채택 | step0007_drop_k8s_cluster.go·migrations.go 등록·migrate.Run 자동 실행(main.go:93 실측)·go test 판정 |
| R-M C65 glob | 채택 | 블록 2 생성·성장 파일로 스코프 한정·Z·P 소유 테스트 3종 diff 0 별도 잠금 |
| R-N H1 배선 검증 | 채택 | C61 회피 시 C60②+C12 필수로 강화 |
| R-O S5 재열거 | 채택 | 14좌표 전수(외부 9·내부 3·info 1·List 1)로 재작성 — §J5 S5 |
| R-P AI detail 966 | 채택 | §J8 detail 착지점 표·§14.2-4에 추가 |
| R-Q 되돌림 마이그레이션 삭제 | 채택 | 러너 순방향 전용 실측 — 백업 restore 단일 경로(§J10·§11) |
| R-R 수치 규범 1곳 | 채택 | C38·C40이 단일 원천 — 본문 참조화(§J9) |
| R-S LOW 취사 | 부분 채택 | 완전#8(7→6코드 파일 — main_sync.go:43 주석)·#10(21종은 스펙 §17.3 문언 승계·실측 utils 22파일 각주) 반영 · #9(verbatim 표장)·나머지 LOW는 위 수정에 자연 편입되어 개별 기록 생략 |
