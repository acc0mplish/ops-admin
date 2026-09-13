# I5 계획 r1 적대검토 병합 — 5렌즈 (2026-09-12)

대상: `v2-phase1/i5-plan.md` r1 (336행) · 판정: **전 렌즈 조건부 승인 — 발견 일괄 반영 후 r2 확정** (반려 0)

## 렌즈 × 심각도 × 발견수

| 렌즈 | 판정 | C | H | M | L |
|---|---|---|---|---|---|
| 1 완전성 | 조건부 | 0 | 1 | 4 | 4 |
| 2 기술오류 | 조건부 | 1 | 4 | 5 | 8 |
| 3 위험 | 조건부 | 0 | 1 | 4 | 3 |
| 4 단순성 | 조건부 | 0 | 1 | 4 | 4 |
| 5 검증성 | 조건부 | 2 | 5 | 5 | 3 |
| **계** | | **3** | **12** | **22** | **22** |

**교차확인(다중 렌즈 독립)** — r2 최우선:
1. **§3 분할 산술 ≤650 자체위반** — 2렌즈(기술 C1·6건 + 단순 H1·4건): ops.go 잔류 666·service_admin ≈740·service_asset_host 722·ops_app_pipeline_exec 680·database.go ~530-900 미배정·ops_schedule 잔류 802. CI-B5가 계획 숫자대로면 PR 강제 재분할
2. **CS-1/CS-2 ↔ §3.2 설계 충돌** — 3렌즈(기술 H1/H2·위험 F3·검증 C2): AssetTerminalSession(:52) 잔류명시 vs service_auth 이동·공용 헬퍼 4심볼(firstNonEmpty :3047·lastField :2926·optionalUint :2650·defaultSSHPort :3056)이 범위열상 타 파일 배정 — 헬퍼 처분표 좌표 단위 확정 필요
3. **pathspec `internal/` vacuous** — 2렌즈(기술 M1·검증 C1): 루트 internal/ 부재 → backend/internal/ 미매치 → 제약 #6·compare.go 제외 게이트 전부 무효
4. **Phase 라벨 오기** — 3렌즈(완전 M2·기술 H4·검증 H1): §3.6 "(Phase H)"→D·§3.8 "I~J"→"H~J"

## 발견 목록 (r2 필수 반영 — 전문 각 렌즈 보고)

### HIGH/C 계 의무분
- [완전 H1] 테스트 >800 4건 §8 등재(engine_test 1,282·proxmox client_test 844·tencent adapter_test 838·kubernetes executor_test 831 — 전수는 30개·"전부 일치" 판정 8 정정)
- [기술 C1·단순 H1] §3 산술 전면 재산출 — database.go 5번째 파일·ops.go 3분할(§2.1 "3"이 원 의도 추정)·ops_schedule 재설계·service_admin/asset_host/pipeline_exec 재배분
- [검증 C2] 헬퍼 처분표 좌표 단위 확정 + CS-1/CS-2 단일 원천화
- [검증 C1] CI-B4·§5 #6 pathspec backend/internal/ 수정
- [기술 H3] 웹 "바이트/원문 보존" 재분류 — composable 자유변수 주입·템플릿 식별자 page.x 재배선은 "행동 보존+재배선"으로 명시·CW-K8s "배선 4줄 원문" 정정
- [위험 F1] 웹 8/9 e2e 미커버 — R6 "기록"에서 승인 게이트로 승격(뷰별 스모크 체크리스트 CI-W claim 또는 명시적 수용 기록)
- [검증 H2] §4 독립검증열 재배선(A에 B산물 CM-2/3 오배선·G에 CC-1 미배선·CO-1~6 심벌 미확정 위임·Phase D/E/F/J/H/I 클레임 공백 — J는 0)
- [검증 H3] 변경파일 집합 동등 클레임 신설(git diff --name-only == 선언 집합) — behavior-neutral 진짜 폐쇄
- [기술 H4·검증 H4] CS-3 소비자 main_compare→main_cloudcompare·CS-2 "k8s_terminal 소비자" 오기(AssetTerminalSession 전후방 service.go 유일)
- [검증 H5·단순 M2] CI-B7 기계 전수 승격 — top-level 선언 블록 멀티셋 diff(~20줄 스크립트·git -C100% --find-copies-harder)를 본체로·인간 3중은 보조 강등
- [위험 F2] 무테스트 영역(controller 2·ops_application·ops_schedule·ops_job·database_multidb) 전수 기계 대조 명시

### MEDIUM 의무분 (요약)
[완전 M1] §5 kubeconfig 평문 계약 추가(opsPipelineKubeconfigFile :1893 — Phase D 범위)·[완전 M3·위험 F2] §2.3 테스트 매핑·무테스트 명시·[완전 M4·검증 H2] CO/CC 배선·[기술 M2] CM-1 67→66·[기술 M4·M-1] CI-B2 30→31·[기술 M3] CS-3 정정·[기술 M5] payload 타입 28→20·[위험 F4] blame 단절 완화(커밋에 원본 파일:라인 좌표 명기·.git-blame-ignore-revs 검토)·[단순 M1] monitor 18→15 병합 기본화(silence+aggregation·query+trace·template+template_prom)·[단순 M3] Phase H 2-PR 허용(H=5,259행 최대·최고위험 R4)·[단순 M4] 파일수 카운트 "잔류 포함" 통일(7·8·6 혼재)·[검증 M1] 수치 5건 재실측 갱신·[검증 M2] 실행 명령+cwd 명기(한자 가드 로컬 절차)·[검증 M3] waiver(v2-phase1/) pathspec 추가·[검증 M4] ^var/^const 커버(H5로 해소)·[검증 M5] Phase 태그 존재 검사·revert 클린 판정

### LOW — 재량 (처분표 형식)
i18n 합 2,069·service 43·경고대 10파일·AssetTerminalWS :980·query_history→trimMonitorQueryHistories·CI-W3 cwd·CM 패키지 한정자·monitor_* 동명 controller/service 충돌 명시·CW-K8s composable 위치(web/src/composables/)·웹 파일명 충돌 0 실측 문구·prometheusRule/monitorLogShortcut 이중배정·BuildMenuTree :1304 귀속·R6 e2e 밖 열거 보완·R8 D 허용·무배포 전제 명문화·[완전 L3] 종결 시 state i5.task 26→25 갱신

## r2 지시 사항
1. 교차확인 4건 최우선 — 산술은 §3 표 전수 재작성(목표 행수=범위 합계+헤더 산출 방식 병기)
2. HIGH/C 의무분 전 반영·MEDIUM 의무분 반영·LOW 처분표(채택/기각 사유)
3. 검증 장치 승격(CI-B7 기계 전수·집합 동등·waiver·태그) — I10 r2 V-1 레슨(존재 증명) 적용
4. r2 상단 개정 이력(발견 번호 ↔ 반영 위치)
5. 미반영 시 사유 명시
