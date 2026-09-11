# I10 계획 — create 오퍼레이션화 (§16.1 커넥션-스코프 create + v1 create 면 삭제) — r2

작성: 2026-09-11 (r1) · **개정 r2 (2026-09-11 — 5렌즈 적대검토 상환: `i10-adversary-r1.md` 발견 HIGH 11·MED 17·LOW 18. HIGH 전부·MED 전부·LOW 선택 반영, 기각분 §9 처분표)** · 티어 **XL**
원천: p6-state.json `i10` · phase6-plan.md §J9·§13 I10·§7 D-15·§14.2-3 · 리컨 `v2-phase1/i10-recon.md`
프로세스 계약: plan-xhigh(r2) → adversary 재검(필요 시) → implement → review-pr-xhigh → verify(메인 재실행)

## r2 개정 이력 (발견 번호 ↔ 반영 위치)

| 발견 | 반영 |
|---|---|
| 교차확인 ① WithPreferred 3렌즈(완전H1·기술F6·검증V-6) | §2.5-r1정정3 · §3.7 · CI15(생존 2단) |
| 교차확인 ② operations.go 모순 2렌즈(기술F2·단순F1) | §3.2 인코딩 (i) — **빈 ResourceKinds 채택**, §3.2.2 재작성, §5 #3·#4 |
| 교차확인 ③ char-baseline 2렌즈(완전H2·검증V-7) | §3.7 · CI6(기대 114행·TestChar 25→19) |
| 교차확인 ④ J2 동작 검증 2렌즈(위험F6·검증V-9) | CI14 필수 시도+회피 의무 대체 강화 · CI11 `connectionScoped: true` 4건 |
| 기술3(J1c 상수 무소유) | §2.4 중간열 · §4 J1c 행 · CI7(상수 grep 3건) |
| 기술5(CI8 파일·13→15) | §2.1 정정 · CI8 재작성(contract/operation.go·15) |
| 기술4/V-1/V-2/V-3 | §3.4(핸들 재설계) · 각 CI 존재 증명 · CI13② 2단(resolved:) · CI16 대체 집합 |
| 위험1(멱등키 소각)·위험2(RESOURCE_BUSY) | §3.6 헬퍼 쌍+공유 템플릿 · RI-9 재평가(§7) |
| MED M3~M7·단순2 | §3.7(고아 처분) · §3.8(스펙 배정 통일) · §5 #12·#16·#17 · §7 RI-11·RI-12·롤백 cleanup · §3.2.1 파생 계약 |
| LOW | 반영분 인라인 · 기각분 §9 처분표 |

---

## 0. 판정 요약

| 쟁점 | 판정 | 1줄 근거 |
|---|---|---|
| 1. 라우트 형태 | **(a) §16.1 확장** `POST /api/v2/infra/provider-connections/{uid}/operations/{name}/plan\|execute` | §16.3은 "generic form은 simple low-risk만" — 임의 kind YAML create 부적합·단일 POST라 멱등·승인·태스크 계약 재발명. (a)는 §16.2 기계장치 전부 승계 + §16.1 reviewed 개정 동반. state.json·phase6 §13 I10 행도 (a) 전제 |
| 2. kinds 면 | **매니페스트 신원 매핑 테이블**(v1 face **15 resourceType** 전수 승계〔r1 "13" 오기 정정〕) + **어휘 확장 2종** | 서덨 면 = v1 face 전수(namespace·pod·service·ingress·configmap·secret·pvc·pv·workload·istio 4·gatewayapi·httproute = 15). destinationrule·serviceentry만 §8.5 밖 → `network.destination_rule`·`network.service_entry` 신설(virtual_service "오퍼레이션 앵커 전용·비교 집합 불참" P2-D 선례). 표(contract)가 단일 원천 — def·실행기는 파생하지 않는다(§3.2.1) |
| 3. I3 경계 | **불포함** — I10은 기존 커넥션 대상만 | create는 등록된 커넥션 향해 발사 — POST /provider-connections·validate·sync(I3) 불요. R26(UI 회귀)은 I3 소관 |
| 4. 동기 vs 태스크 | **태스크 모델 채택**(plan→execute(Idempotency-Key)→approve→poll) | §13.4 멱등·§13.3 승인·§18.1 감사의 유일 경로. policy.Evaluate가 ResourceKind 공백 시 fail-closed(VK-4 — 실측 policy.go:58-76)라 매니페스트 매핑 kind가 필수 입력 → plan body 필수 |
| 5. 클러스터 id 공간 | **프론트 resolve 승계** — `resolveK8sClusterConnectionUid(clusterId)`(k8s.js:112, E0) | API는 legacy id 미수용(이중 해상 id 서버 수용은 결합 영구화) |

**인코딩 판정 (r2 신설 — 어댑터리 요청)**

| 항목 | 판정 | 실측 근거 |
|---|---|---|
| (i) 커넥션-스코프 def의 kinds 부호화 | **빈 `ResourceKinds` 등록 — ConnectionScoped 신규 필드 폐지** | registry.go:247-251 검증 루프가 빈 슬라이스에 no-op(등록 성공) · operations.go:341 ListResourceOperations 교집합 루프가 빈 kinds와 미매치 → **리소스-스코프 목록에서 자동 누출 방지** — operations.go·contract/operation.go 무편집. 대안(필드 유지)은 operations.go 소유 지정+§5 #4 예외+스펙 §10.2 구조 확장을 전부 요구 — 우위 없음 |
| (ii) create 핸들 마커 | **`state\|` 마커 + `resource:"manifest"` 기대상태 재사용 — `create\|` 신규 마커 폐지, executor_state.go 무편집** | executor.go:378 Poll이 `state\|` 접두로만 분기(신규 마커는 rollout 디코더로 낙하·에러) · state 판별자 화이트리스트 6종 중 manifest의 판정면(GET 1회·동결 manifest 부분집합 에코)이 create 종단 판정과 동일 · 404→에러 처리도 create 수렴 오보 방지와 정합(§3.4). 핸들: `state\|<connUID>\|<항목경로>\|<base64url({resource:"manifest", manifest:동결})>` |

**골든 목표치(정정 승계)**: 사전 기술 "440→439·280→279"은 v1 삭제분만 계산 — v2 라우트 2 신설 반영 실측 목표 **route-inventory 441(438 라우트+헤더 3)·sensitive 281(278 def+헤더 3)·authz "428 (413 v1 + 15 v2)"·non-GET 233** (§2.4 · CI3/CI4 단일 원천).

---

## 1. 목표 · 비목표

### 목표
1. **§16.1 커넥션-스코프 오퍼레이션 면 신설** — 2 라우트 + 오퍼레이션 def `k8s.resource.create` + 엔진 체인 분기 + 실행기 leg. I-P1 블로커 해소: create의 스코프 앵커를 "존재하지 않는 리소스 uid"에서 "커넥션 uid + 매니페스트 신원"으로 이동.
2. **v1 create 면 삭제** — 라우트(routes_v1_infra.go:241)·컨트롤러(k8s.go:316)·서비스(k8s_mutate.go:14)·opdef 행(defs_monitor.go:77)·k8s_path.go 죽은 헬퍼 6함수·고아 타입 3종·죽은 char 6종·**char-baseline 재생성(120→114행 — C47 불가침의 첫 수정 사례)**. §19.1 step 4 **13/13 완결 — D-15 상환**.
3. **골든 갱신** — J1c 중간값(442·282·"429 (414 v1 + 15 v2)"·234)·최종 441·281·"428 (413 v1 + 15 v2)"·233.
4. **프론트 소비 전환** — K8s.vue create 콜사이트 4곳 → runOpTasks 커넥션-스코프. k8s.js 잔여 v1 k8s 쓰기 0·:65 주석 상환.

### 비목표 (경계)
- **I3 — §16.1 POST /provider-connections·{uid}/validate·{uid}/sync 불포함**(GET만 존재 — infra.go:84-89 실측). 등록은 register-k8s CLI. R26(UI 회귀) I3 소관.
- 기존 10종 오퍼레이션 def·실행기·라우트 동작 변경 금지(§5 #4 — executor.go dispatch case 1행·tasks.go 주석 1행만 예외).
- 라이브 패스스루 읽기 17 + `pod/terminal/ws` 무변경(§J3·D-12 승계).
- istio kinds의 apply/delete 면 확장 — create만(現狀 승계).
- DB 스키마 변경 0(provider_task.resource_uid에 합성 uid 문자열만).
- e2e 신규 슬라이스 창설 불가 — slice-a 보강만(모드 규율: slice-b/c는 serve-b/c 전용 fixture — 병렬 레인 금지, web-e2e-stack-modes 계약).

---

## 2. 현재 상태 실측 (2026-09-11 HEAD — r2 재실측 포함)

### 2.1 v1 create 면 (삭제 대상 — 6축 census)

| 축 | 실측 |
|---|---|
| ① 라우트·골든 | routes_v1_infra.go:241 · route-inventory.txt **:331** · sensitive-routes.txt **:177**(assets:k8s:workload:yaml·mutating·risk=medium) |
| ② 내부 호출자 | controller/k8s.go:316(핸들러)·:322(호출)·service/k8s_mutate.go:14(본체)뿐 — **서비스 내부 호출자 0** |
| ③ 프론트 | k8s.js:200 함수 + K8s.vue 콜사이트 **4곳** — :1715 PV(cluster-scope)·:1886 configmap/secret 생성(edit 분기는 이미 V2 resourceApply)·:1904 namespace·:2099 istio(**템플릿 6종**: gatewayapi·httproute·gateway·virtualservice·destinationrule·serviceentry — buildIstioTemplate :810). 타 뷰 소비 0 |
| ④ DB | 무관(k8s_cluster drop 완결) |
| ⑤ 테스트 | routes_inventory_test.go:130 `437`·authz_replay_test.go:235 `427 (414 v1 + 13 v2)`·:353 `232` · **char-baseline `service/testdata/char-baseline.txt` 120행** — 죽는 char 6종(:16·:17·:18·:19·:73·:104)·생존 char 확정(§3.7) |
| ⑥ CI | 원격 대기 금지 — 로컬 claims 전부 |

서비스 흐름(k8s_mutate.go:14-53): payload{ClusterID, ResourceType, Namespace, Name, YAML} → k8sClientForCluster → YAMLToJSON → parseK8sManifestIdentity → buildK8sCreateResourcePaths(k8s_path.go:90 — **resourceType 어휘 15종**〔r1 "13" 정정〕: namespace·pod·service·ingress·configmap·secret·pvc·pv·workload(하위 5형)·istio 4(gateway·virtualservice·destinationrule·serviceentry)·gatewayapi·httproute) → 컬렉션 POST → `created:true` 동기 응답.

### 2.2 V2 면 (확장 기반)

- **라우트**: infra.go:84 Register — GET 4종. operations.go:44 RegisterOperations — 감사(`group.Use(AuditMiddleware)`) 선행 → route별 `grants(opdef.Must(POST, path))` = V2DynamicMiddleware(opdef.go:96 — `c.Param("name")` → registry def 권한). 배선: router.go:86-88 → routes_v2.go:32-35.
- **plan/execute 핸들러**: PlanOperation(operations.go:153 — 무상태·무body)·ExecuteOperation(:197 — Idempotency-Key 필수·RESOURCE_BUSY 409·재생 200+`Idempotency-Replayed`·첫 제출 201).
- **엔진 제약 (r1 신규 실측·r2 보강)**: SubmitInput.ResourceUID `''` 하드 거부(T-5, engine.go:97) · resolveExecutionChain이 `infra_resource.uid` 조인(T-7) → 합성 uid+분기 필수(§3.3) · executeClaimed가 chain→def→adapter→capability→broker→Execute — Connection이 클러스터 주소 결정(arch rule 2), URN은 목표 파생만 → create는 URN 불요. **cancel 능력 착지 완료**(cancel.go:58 `Engine.RequestCancel` — infra.go:74 타입 단언 성공 경로. operations.go:22-25·tasks.go:20-21의 "503 미착지" 주석은 낡은 문언 — RI-9 탈출 경로 산정에 반영).
- **오퍼레이션 def 등록부**: compose_k8s_ops.go(309행) 10종. resourceApplyOperation(:234) update 한정 주석(I-P1). DefTable 잠금 테스트: **contracttest/opdef_table_test.go:157** — `len(ops) == len(k8sOpDefTable)+len(pveDefNames)`(현재 13 = k8s 10 + pve 3) — create 추가 시 테이블 +1행(14) 필요(contracttest는 P 산물 — §5 #3 예외 확장).
- **def 타입 소재**: `contract/operation.go:4`(`type OperationDefinition` — r1이 adapter.go로 오기〔CI8 교정〕. adapter.go는 OperationRequest/PollRequest 소유).
- **state handle 가족 (r2 핵심 실측)**: executor_state.go — `stateHandleMarker = "state"`(:38 부근), 핸들 `state|<connUID>|<apiPath>|<base64url(expectation)>`, 기대상태 판별자 **6종**(node·service·manifest·delete·traffic_istio·traffic_httproute — 미지 판별자 파싱 거부). Poll 분기: executor.go:378 `strings.HasPrefix(ProviderRef, "state|")` → pollStateRef — **접두 밖 마커는 rollout 디코더로 낙하(에러)**. manifest 판정면: GET 1회·관측이 동결 manifest를 포함하는지(부분집합 에코)·404→에러.
- **apply 실행기 신원 가드 선례**: executor_resource.go `applyManifestOf` — kind·metadata.name·namespace 정합 사전 거부(cross-kind 차단). create leg가 승계할 모델.
- **경로 원형**: resourceCandidatePaths(항목 경로·gwapi 2후보)·k8s_path.go:90 컬렉션 경로·`buildIstioResourcePathsWithPreferred`(k8s_transport.go:217 — apiVersion v1/v1beta1 선호 순서).
- **registry 등록 검증**(registry.go:228): ResourceKinds 전종 IsKnownResourceKind 소속 — **빈 슬라이스 no-op**(§0 인코딩 (i) 근거).
- **policy.Evaluate**(policy.go:58-76): ProviderType·ContextKind·ResourceKind·Risk 공백 시 평가 불가 → fail-closed. k8s 컨텍스트 Kind="cluster"(main_register_k8s.go:247).
- **taskView.Payload 노출 (r2 실측 — RI-12)**: tasks.go:25-28 주석 "Payload is included … carries no credential material" + `Payload contract.JSONMap \`json:"payload"\`` — **Secret 생성 매니페스트(stringData 평문)는 이 가정을 위반**한다. J1c에서 주석 수식+수용 기록(§3.5·§5 #16).

### 2.3 프론트 자산

- k8s.js(**226행** — r1 "227" 정정): :65-66 주석 "stays on v1 by design…(D-15, carried as I10)" · :200 createK8sResourceYAML(잔여 v1 k8s 쓰기 = 이 1개) · :112 resolveK8sClusterConnectionUid · :132 resolveK8sResourceUid · :160-177 멱등키 맵 — **생성 템플릿 `${resourceUid}:${operation}`(:163)과 소각 템플릿(:170)이 인자 순서까지 동일**(커넥션-스코프 헬퍼도 동일 구조 강제 — §3.6) · :183 createK8sOperationTask.
- useK8sOperationProgress.js(203행) run(): 커넥션 uid 해상 → 행별 리소스 uid 해상 → 제출 → 권한 보유 시 자동 approve → 2초 폴·5분 예산·종단 멱등키 소각(:91 `clearK8sOperationIdempotencyKey(row.resourceUid, row.op)`).
- api/infra.js: planInfraOperation은 body {} 고정 — 커넥션-스코프 plan은 yaml body(신규 함수).
- i18n: k8s-extra-i18n.js(46행) ko/en 이중 사전 — op 명칭 1쌍 추가.
- K8sDialogs.vue(568행)은 마크업만 — J2 무변경. K8s.vue 2,797행 ±수십(4 치환).

### 2.4 골든 산술 (실측 전개 — 본문 수치는 전부 CI3/CI4/CI7로 귀결)

실측: route-inventory `grep -c .` → **440**(헤더 3 + 라우트 437 = 1 ping + 7 public + authGroup 427(v1 414 + v2 13) + uploads GET/HEAD 2) · sensitive → **280**(헤더 3 + opdef 277, v2는 `/infra/...` 경로 5행 :155-159).

| 단계 | route-inventory | sensitive | authz authGroup | non-GET |
|---|---|---|---|---|
| 현재 | 440 (437) | 280 (277) | 427 (414 v1 + 13 v2) | 232 |
| J1c 후 (+2 v2 라우트·+2 opdef) — **테스트 상수 3곳 동반 갱신 의무**(routes_inventory_test:130 → 439·authz :235 → `"429 (414 v1 + 15 v2)"`·:353 → 234) | 442 (439) | 282 (279) | 429 (414+15) | 234 |
| **J3 후 (−1 v1 create)** | **441 (438)** | **281 (278)** | **428 (413 v1 + 15 v2)** | **233** |

routes_inventory_test.go:123-125 주석 "428 authGroup·415 v1"은 실측(427·414)과 어긋나는 낡은 산문 — 단얫값만 유효, J1c 상수 갱신 시 바로잡는다(산문 수정은 grep claim 불가 — 리뷰 확인 항목).

### 2.5 정정 이력

**리컨 번들 정정(r1 승계)**: ① 골든 439/279 → **441/281**(v2 2신설 미계산) ② k8s_mutate.go:81 → **:14** ③ 골든파일 헤더 3행 구성 명시 ④ 엔진 T-5/T-7 신규 실측 ⑤ apply 사전 가드·컬렉션 경로 원형 특정 ⑥ destinationrule·serviceentry 어휘 밖 구체화 ⑦ plan 무body·policy ResourceKind 필수 충돌 도출.

**r1 → r2 정정(적대검토)**: ① §3.7 "k8s_transport.go *WithPreferred 변형(사용처 소멸)" **허위 폐기** — 래퍼·WithPreferred 전부 생존(k8s_transport.go:204/:209 라이브 읽기 → :213/:238 래퍼 → :217/:242 본체). 사멸은 k8s_path.go 직접 호출 3곳(:157/:167/:172)뿐. J3에서 transport 함수·`TestChar*WithPreferred` 2종(k8s_transport_test.go:438/:467) **존치** ② ConnectionScoped 필드 폐지 → 빈 ResourceKinds(§0 인코딩 (i)) ③ `create|` 핸들 마커 폐지 → state\| 재사용(§0 (ii)) ④ "13 resourceType" → **15** ⑤ k8s.js 227 → **226행** ⑥ CI8 grep 대상 adapter.go → **contract/operation.go:4**(폐지와 함께 클레임 재작성) ⑦ 엔진 테스트 "12 스위트" → **51 Test 함수**(실측 grep 합산) ⑧ J1c 중간 상수 3곳 소유 누락 보완.

---

## 3. 설계

### 3.1 라우트 형태 (쟁점 1 — (a) 채택)

```text
POST /api/v2/infra/provider-connections/{uid}/operations/{name}/plan      → PlanConnectionOperation
POST /api/v2/infra/provider-connections/{uid}/operations/{name}/execute   → ExecuteConnectionOperation
```

- 등록: `InfraAPI.RegisterConnectionOperations(group, grants)`(RegisterOperations 미러 — 감사는 그룹 선행 장착) — routes_v2.go 1블록.
- opdef 대표 행 2종(defs_v2infra.go): `POST /infra/provider-connections/:uid/operations/:name/plan|execute` — Permission 대표값 `assets:k8s:workload:yaml`(실제는 V2DynamicMiddleware가 `:name` 해석 — 기존 5행과 동일 구조).
- **기각 (b) §16.3**: "generic form allowed only for simple low-risk operations"에 정면 배치(임의 kind YAML create). 단일 POST라 멱등·승인·태스크·감사 재발명.
- **기각 (c) 기존 §16.2 라우트에 합성 uid 재사용**: §8.1 신원 위반(resources 경로 :uid에 비-리소스 값)·loadLiveResource 의미론 오염. 골든 산술 2행이 설계를 지배하지 않는다.

### 3.2 오퍼레이션 정의 `k8s.resource.create` — 빈 ResourceKinds 부호화

```go
var resourceCreateOperation = contract.OperationDefinition{
    Name:               kubernetes.CreateOperationName, // "k8s.resource.create"
    Version:            "1",
    RequiredPermission: "assets:k8s:workload:yaml", // v1 create 행 재사용 — 신규 권한 문자열 0
    RequiredCapability: "orchestration.kubernetes.apply",
    ResourceKinds:      nil, // 커넥션-스코프 — 빈 슬라이스(§0 인코딩 (i)). 서빙 면은 §3.2.1 표가 소유
    Mutating:           true,
    RiskLevel:          "high",       // 임의 kind면 — apply/delete 상향 선례(P2-C §10-4). v1 medium→상향 판단 기록
    RequiresApproval:   true,          // 전 오퍼레이션 승인 필수 posture 승계
    IdempotencyPolicy:  "provider_create_convergent", // POST 201 = 종단; 409+동결 manifest 부분집합 에코 = 수렴 성공(§3.4)
    TimeoutSeconds:     30,
    RetryPolicy:        contract.RetryPolicy{MaxAttempts: 3, BackoffSeconds: 5},
    Redaction:          func() any { return stateResultRedaction{} }, // generation·serverURL — apply 승계
}
```

**빈 ResourceKinds 부호화의 효과 (실측 근거)**: registry.go:247-251 소속 검증 루프 no-op(등록 성공) · operations.go:341 ListResourceOperations 교집합 루프가 빈 kinds와 미매치 → **존재 리소스의 오퍼레이션 목록에서 자동 누출 방지** — operations.go·contract/operation.go·스펙 §10.2 구조 확장 불요. 스펙 개정은 §10.2에 각주 1행("커넥션-스코프 오퍼레이션은 ResourceKinds를 갖지 않는다 — kind 면은 매니페스트 매핑 표가 소유", J0 착지).

#### 3.2.1 kinds 면 — 매핑 테이블 단일 원천 (쟁점 2)

신규 `internal/infra/contract/k8s_create_face.go` — **apiVersion group × kind → (§8.5 kind, namespaced)** 정적 표 + 조회 함수. **파생 계약(단순2 채택)**: 이 표가 create 서빙 면의 유일 원천이다 — def의 ResourceKinds는 공란(§3.2)·실행기의 컬렉션 경로 표도 본 표 소속 성분에서 조립한다(이중 열거 금지 — CI10 단얫이 표 소속성과 경로 조립의 정합을 잠금).

| apiVersion group | kind | → kind | namespaced |
|---|---|---|---|
| (core) | Namespace | orchestration.namespace | ✗ |
| (core) | PersistentVolume | storage.volume | ✗ |
| (core) | Pod | orchestration.pod | ✓ |
| (core) | Service | network.load_balancer | ✓ |
| (core) | ConfigMap / Secret | orchestration.configmap / .secret | ✓ |
| (core) | PersistentVolumeClaim | storage.volume | ✓ |
| networking.k8s.io | Ingress | network.load_balancer | ✓ |
| apps | Deployment·StatefulSet·DaemonSet·ReplicaSet | orchestration.workload | ✓ |
| batch | Job·CronJob | orchestration.workload | ✓ |
| networking.istio.io | Gateway | network.gateway | ✓ |
| networking.istio.io | VirtualService | network.virtual_service | ✓ |
| networking.istio.io | **DestinationRule** | **network.destination_rule** 〔신설〕 | ✓ |
| networking.istio.io | **ServiceEntry** | **network.service_entry** 〔신설〕 | ✓ |
| gateway.networking.k8s.io | Gateway | network.gateway | ✓ |
| gateway.networking.k8s.io | HTTPRoute | network.http_route | ✓ |

- `contract/resource_kind.go`에 `I10ResourceKindExtensions = []string{"network.destination_rule", "network.service_entry"}` 추가(IsKnownResourceKind 조회부 확장 — Phase2·6과 동일 reviewed-diff 경로·주석 "create 전용 앵커·수집·비교 집합 불참(virtual_service P2-D 선례)").
- **검증은 매니페스트 구조 검증**: kind·apiVersion·metadata.name 필수 + 표 소속 + namespace 규칙(namespaced 종은 manifest metadata.namespace 필수·클러스터 스코프 종은 공백). v1 resourceType 어휘 스위치 폐지 — 표가 어휘 소유. **v1 패리티 계약**: 표가 v1 face 15 resourceType 전수(§2.1)를 덮는다 — UI 4 콜사이트 전종 이행이 D-15 완전 상환 조건.
- resourceMutationKinds(8종)와의 관계: create 면 = 8종 ∪ {pod, virtual_service, destination_rule, service_entry}. apply 면 무변경.

### 3.3 엔진 — 합성 resource_uid와 체인 분기

- **합성 uid**: `conn:<connectionUID>:<singular>:<namespace>/<name>`(클러스터 스코프는 `conn:<connUID>:<singular>:<name>`) — execute 시점 매니페스트 신원에서 결정적 도출. T-5 충족 · `(resource_uid, active_flag)` 유니크(§13.4)가 **목표 신원 단위 직렬화**(동일 목표 동시 create 2건째 RESOURCE_BUSY 409 fast-fail·상이 목표 독립). `conn:` 단순 접두(커넥션 전체 직렬화)는 무관 create 충돌로 기각.
- **예약 단얫(위험7 채택)**: engine_resolve_test에 ① 생성된 infra_resource.uid 전수가 `conn:` 비접두임을 조건부로 단얫(가드 쿼리) ② 합성 uid 포맷 단얫(접두·콜론 세그먼트 수·namespaced/cluster-scope 2형) — r2 "알파벳 단얫".
- **엔진 변경**: resolveExecutionChain에 `conn:` 접두 분기 — infra_resource 조인 대신 `provider_connection.uid = ?` 직접 조회(ConnectionUID·ProviderType 조달·ExternalURN=""). executeClaimed 이하 무변경.
- **라인캡 동반 분할**: engine.go 696행(650 초과) — resolveExecutionChain·connectionView·capabilityServed를 `engine_resolve.go`(신규)로 추출, engine.go ~600행 복귀. 기존 테스트 **51 Test 함수**(실측) 전수 보존 + 신규 2경로 단얫.

### 3.4 실행기 leg — `executor_create.go` (신규) — state\| 핸들 재사용 (인코딩 (ii))

- `CreateOperationName = "k8s.resource.create"` 상수 + executor.go dispatch switch case 1행(**executor.go 유일 편집**).
- **Execute(payload, Connection)**: `payloadDecode{yaml}` → YAMLToJSON(v1 동일 변환) → 신원 가드(§3.2.1 규칙 — applyManifestOf 강도: 표 소속 group×kind·name·namespace 정합) → 컬렉션 경로 후보(실행기 소유 표 — §3.2.1 파생 계약, istio/gateway API는 apiVersion 선호 v1→v1beta1 2후보) → **POST 1회(동결 매니페스트 body)** 후 3분기:
  - **201** → 핸들 반환: `state|<connUID>|<항목경로>|<base64url({"resource":"manifest","manifest":동결})>` — **apply와 동일 인코딩**(항목경로 = 컬렉션 경로 + "/" + name). 엔진이 같은 사이클에 Poll 1회 → pollStateRef(manifest 판정면: GET 1회·부분집합 에코) **무편집 재사용** → Succeeded{generation, serverURL}.
  - **409 AlreadyExists** → GET 항목경로 1회·동결 manifest 부분집합 에코 판정: **일치** → 핸들 반환(폴이 수렴 종단 — crash-retry가 이미 도달한 목표를 실패로 오보하지 않는다) / **불일치** → executor error "already exists with different content"(v1 409 실패 UX 패리티 — 타인의 객체 충돌은 즉시 실패).
  - **그 외 오류** → executor error(재시도 MaxAttempts 3).
- **404 처리**: manifest 판정면의 기존 동작(404→에러)을 승계 — create 후 GET 404는 "수렴 주장 반증"(삭제 경쟁)이므로 Running이 아니라 에러·재시도가 정확하다(r1 "404→Running" 폐기 — 적대검토 지적 수용).
- **executor_state.go·executor.go(Poll) 무편집** — 접두 분기(executor.go:378)·판별자 화이트리스트·부분집합 에코 판정 전부 기존 자산 재사용(§0 (ii) 실측 근거).
- 어댑터는 req.Connection만 소비(arch rule 2) — kubeconfig 물질은 ConnectionView.Material로만.

### 3.5 API 핸들러 — `operations_connection.go` (신규, internal/api/v2)

- **plan(body 필수 `{yaml}`)**: 커넥션 로드(stale_source 제외)·컨텍스트 로드(Kind="cluster") → def 해석(resolveOperationDefinition 재사용) → 매니페스트 파싱·매핑 표 조회(불소속·구조 위반 → **400 INVALID_OPERATION_PAYLOAD** — policy 평가 불가 입력 사전 차단, VK-4 정신) → policy.Evaluate(매핑 kind·ContextKind·ProviderType·Risk·Mutating) → `{operation, version, connectionUid, targetKind, requiresApproval, riskLevel, permission, policyVersion}`. restartedAt/resourceRevision 미발급(create의 동결 앵커는 매니페스트 자신).
- **execute(body `{yaml}`·Idempotency-Key 필수)**: 동일 검증·policy → 합성 uid 도출 → `engine.Submit({OperationName, ResourceUID: 합성, Payload: {yaml} 그대로})` → 201/200 재생. 오류 코드: IDEMPOTENCY_KEY_REQUIRED·OPERATION_NOT_REGISTERED·**CONNECTION_NOT_FOUND(신규 404)**·INVALID_OPERATION_PAYLOAD·POLICY_EVALUATION_FAILED·POLICY_DENIED·ENGINE_UNAVAILABLE·RESOURCE_BUSY 409·SUBMIT_FAILED.
- 감사: auditInfo에 ProviderConnectionUID·Operation·(execute)ResourceUID=합성·PolicyVersion·ErrorCode — AuditMiddleware 그룹 선행 승계.
- **taskView.Payload 주석 수식(J1c — tasks.go 1행)**: "carries no credential material" 가정이 Secret 생성 매니페스트로 위반됨을 주석에 명기 + 수용 기록(§5 #16 — RI-12).

### 3.6 프론트 전환 — 멱등키 배선 정합 (위험1 채택)

- k8s.js: createK8sResourceYAML·:65-66 주석 삭제 · `K8S_OPERATIONS.resourceCreate` 추가 · 신규 헬퍼 쌍 **`k8sConnectionIdempotencyKey(connectionUid, op, targetKey)` / `clearK8sConnectionIdempotencyKey(connectionUid, op, targetKey)`** — **맵 키 템플릿 `${connectionUid}:${op}:${targetKey}`를 두 함수가 공유 상수로 소유**(기존 쌍이 resourceUid:op 템플릿을 생성·소각 양쪽에 동일 소유하는 구조 승계 — k8s.js:160-177 실측). targetKey = 프론트가 조립한 매니페스트의 `kind:ns/name`(프론트 신원 인지). **소각 누락 = 낡은 SUCCEEDED 재생(N6 replay wins)에 의한 UI 성공·클러스터 미생성 silent 위반** — CI11 grep·CI14/CI16 재발화 단얫으로 잠금.
- `createK8sConnectionOperationTask(connectionUid, operation, payload, targetKey)` — plan(body=yaml) → execute(공유 템플릿 키) → {taskUid, permission} 반환 + composable이 소각용 targetKey를 row에 보관.
- useK8sOperationProgress.js run(): target 부재(**`connectionScoped: true`**) 행은 resolveK8sResourceUid 건너뛰고 커넥션-스코프 제출 — approve 자동·폴·키 소각(공유 템플릿)·타임아웃 무변경 승계.
- K8s.vue 4 콜사이트(:1715·:1886 create 분기·:1904·:2099) → runOpTasks `{op: K8S_OPERATIONS.resourceCreate, connectionScoped: true, payload: {yaml}, display}` 치환(성공 후 refreshCurrentClusterData는 기존 finishHook 패턴). K8sDialogs.vue 무변경.
- i18n: `k8sOpResourceCreate` ko/en 1쌍 추가(패리티 게이트).

### 3.7 v1 면 삭제 (J3 — 원자) — 죽음/생존 경계 확정 (교차확인 ①·M3·M4 채택)

**사멸(6함수 + 3타입 + 6 char)**: k8s_path.go의 buildK8sYAMLResourcePath(:21)·buildK8sCreateResourcePaths(:90)·buildK8sYAMLResourcePaths(:178)·buildK8sDeleteResourcePaths(:214)·parseK8sManifestIdentity(:223)·friendlyK8sYAMLError(:242) — 유일 호출자가 create 본체/사멸 가족(실측 §2.1②·§2.5-r2①). 고아 타입 3종: `K8sResourceYAMLPayload`(model/k8s.go:57)·`K8sResourceDeletePayload`(:66 — H2 리뷰 이월 "model payload 고아" 상환의 일부)·`k8sManifestIdentity`(k8s_types_mesh.go:186). char 6종: TestCharBuildK8sCreateResourcePaths·…DeleteResourcePaths·…YAMLResourcePath·…YAMLResourcePaths·TestCharParseK8sManifestIdentity·TestCharFriendlyK8sYAMLError.

**생존(존치 — J3 밖)**: `isK8sNotFoundError`(k8s_path.go:234 — 라이브 호출자 k8s_transport.go:190/:292) · **`buildIstioResourcePaths`/`buildGatewayAPIResourcePaths` 래퍼(k8s_transport.go:213/:238)와 `*WithPreferred` 본체(:217/:242)** — 라이브 읽기(:204/:209)가 호출 · `TestChar*WithPreferred` 2종(k8s_transport_test.go:438/:467) · k8s_path.go 나머지 19함수(GetText·StatusText·건강 점수 계열 — 512행→~330행 존치) · k8s_mutate.go의 GetK8sNamespaceEvents(:55-61).

**char-baseline 재생성(교차확인 ③ 채택 — C47 불가침의 첫 수정 사례)**: `service/testdata/char-baseline.txt`을 C47 기존 재생성 절차(시간 문자열 sed 절단 포함 — 원문 명령은 phase6 §6 C47 계열)로 재생성·커밋. **기대 행수 120 → 114**(−6). CI6이 잠금.

**골든·상수**: 골든 2종 재생성(`go test ./router/ -run 'TestRouteInventoryArtifact|TestSensitiveRoutesArtifact' -update` — 생성기 헤더 원문)·테스트 상수 3곳 J1c 중간값 → J3 최종값(§2.4)·routes_inventory_test 낡은 산문 주석 바로잡음(리뷰 확인 — grep 불가).

### 3.8 스펙 문서 영향 (Phase 배정 통일 — M5 채택)

| 스펙 편집 | Phase | 내용 |
|---|---|---|
| §8.5 계열 + §10.2 각주 | **J0** | I10 어휘 확장 2종 서술(virtual_service 선례 인용) · "커넥션-스코프 오퍼레이션은 ResourceKinds를 갖지 않는다 — kind 면은 매니페스트 매핑 표가 소유" |
| §16.1 | **J1c** | provider-connections 하위 operations 2 endpoint + 커넥션-스코프 계약(매핑 표·합성 uid·plan body 필수) |
| §19.1 step 4 | **J3** | D-15 이탈 기록에 **상환 주석 추가**(`resolved: 2026-09-… — connection-scoped create landed (I10); step 4 is 13/13` — 역사 문언 삭제 아님. 실측: 스펙에 `resolved:` 현재 0매치 → CI13② 2단 성립) |

---

## 4. 구현 Phase 분해

```
[착수 승인(§14.2-3)] ─▶ J0 ─▶ J1a ─▶ J1b ─▶ J1c ─▶ J2 ─▶ J3
                        def·어휘  엔진 체인  실행기 leg  라우트+상수+스펙  프론트  원자 삭제
```

J0·J1a 상호 무의존(병렬 가능)이나 XL 직렬 프로세스 계약상 순차 PR(리뷰 단순성 — 처분표 §9-D3). J1b는 J0·J1a 선행. J2는 J1c 후. **J3는 J2 후**(E-1 승계 — J2 이전 v1 삭제 금지).

| Phase | 파일 (소유) | 성격 | 독립 검증 |
|---|---|---|---|
| **J0** 〔임계〕 | ① contract/resource_kind.go(+I10 슬라이스) ② contract/k8s_create_face.go〔신규〕+테스트 ③ compose/compose_k8s_ops.go(+def 1·등록 1행) ④ **contracttest/opdef_table_test.go(테이블 +1행 — P 산물 예외 §5 #3)** ⑤ 스펙 §8.5·§10.2 개정 | additive(라우트·골든 무변경) | CI1·CI2·CI8·CIm |
| **J1a** 〔임계〕 | ① tasks/engine.go(체인 계열 추출 ~600행) ② tasks/engine_resolve.go〔신규〕(+conn: 분기) ③ tasks/engine_resolve_test.go〔신규〕 | additive(도달 불가 — 라우트 미착지) | CI1·CI2·CI9 |
| **J1b** | ① adapter/kubernetes/executor_create.go〔신규〕 ② executor_create_test.go〔신규〕 ③ executor.go(dispatch case 1행 — 유일 편집) | additive | CI1·CI2·CI10 |
| **J1c** | ① api/v2/operations_connection.go〔신규〕 ② operations_connection_test.go〔신규〕 ③ opdef/defs_v2infra.go(+2행) ④ router/routes_v2.go(등록 1블록) ⑤ api/v2/tasks.go(주석 1행 — §3.5) ＋ **테스트 상수 3곳 갱신(routes_inventory_test:130→439·authz:235→"429 (414 v1 + 15 v2)"·:353→234)** ＋ 골든 2종 재생성(442·282) ＋ 스펙 §16.1 | additive(라우트 신설 — v1 무변경) | CI1·CI2·CI7·CI13① |
| **J2** | ① api/k8s.js ② composables/useK8sOperationProgress.js ③ views/assets/K8s.vue ④ utils/k8s-extra-i18n.js ⑤ (보강) e2e/slice-a.spec.js create 단얫 | behavior-changing(프론트 — v1 존치 병행) | CI11·CI12·CI14 |
| **J3** 〔임계·원자 ⚠〕 | routes_v1_infra.go·controller/k8s.go·service/k8s_mutate.go·service/k8s_path.go(6함수)·model/k8s.go(2타입)·service/k8s_types_mesh.go(1타입)·opdef/defs_monitor.go·service/k8s_path_test.go(char 6종)·**service/testdata/char-baseline.txt(재생성 114행)**·routes_inventory_test.go·authz_replay_test.go·스펙 §19.1 ＋ 골든 2종 재생성(441·281) — **~12파일(≤5 의도된 편차 — H2·E1 선례 원자 단위)** | behavior-changing(삭제 원자) | CI1~CI6·CI13②·CI15 |

임계경로(직렬 신중): **J0 매핑 표**(서빙 면 정확성이 전체 전제) → **J1a 엔진 체인**(합성 uid 분기 — 태스크 엔진 코어) → **J3 원자 삭제**. J1b·J1c는 매핑 표·체인에 결합 — 표가 단일 원천이므로 정합은 유닛테스트 2중(표 소속성·경로 조립)이 잠금.

라인캡: 신규 k8s_create_face.go(~120)·engine_resolve.go(~120)·executor_create.go(~260)·operations_connection.go(~200) + 테스트 — 전부 650 이내. engine.go 696→~600(CI9). compose_k8s_ops.go 309→~350. k8s_path.go 512→~330. controller/k8s.go 438−13. k8s.js 226±.

---

## 5. 보존 제약 (③구현 프롬프트에 verbatim 복사 — r2)

1. **v1 create 라우트는 J3까지 존지한다** — J2(프론트 전환) 머지 전에 v1 create 면을 삭제하지 않는다(E-1 승계).
2. **골든 2종·테스트 상수의 허용 변경은 J1c(+2·중간 상수 3곳)·J3(−1·최종 상수 3곳) 선언분뿐**이다. J0·J1a·J1b·J2는 골든 diff 0. 최종 기대값 route-inventory **441**·sensitive **281**·authz **"428 (413 v1 + 15 v2)"**·non-GET **233** — 단일 원천은 CI3·CI4·CI7.
3. **P 산출물 무변경 — 단 I10 명시적 예외 2건**: (a) compose_k8s_ops.go 신규 def 추가·등록 1행 한정(기존 10종 def 바이트 불변) (b) contracttest/opdef_table_test.go k8sOpDefTable +1행(def 집합 잠금 단얫의 대상 확장). `k8sassembly*`·`compose.go`·`mapping.md`·`projection.go`·`internal/infra/metrics/**`·`internal/infra/inventory/**`·**char-baseline은 J3 재생성분 외 무변경**.
4. **기존 10종 오퍼레이션·핸들러·실행기 동작 무변경** — create는 신규 파일·신규 행으로만 추가. `operations.go`·`infra.go`·`executor_state.go`·`executor_resource.go`·`executor.go`의 Poll은 수정하지 않는다. 유일 예외 2편집: executor.go dispatch case 1행(J1b)·tasks.go 주석 1행(J1c).
5. **kubeconfig 평문 금지(봉인 계약 승계)** — create 실행기·핸들러는 ConnectionView·매니페스트만 다룬다. 시크릿 물질의 로그·에러·감사·ProviderRef 노출 금지.
6. **어댑터의 DB·브로커 직접 접근 금지**(arch rule 2) — create leg는 req.Connection만 소비. 합성 uid 도출은 API 계층(v2)과 엔진뿐.
7. **Z 테스트**: k8s_path_test.go에서 죽은 6종 char만 제거 — **isK8sNotFoundError char·k8s_transport_test.go 전부(WithPreferred 2종 포함)·살아남은 19종 char는 바이트 불변**. char-baseline 재생성은 C47 절차 원문(시간 문자열 sed 절단 포함)으로만 — 기대 114행. char-exclude·C47 불가침의 첫 수정 사례임을 PR에 명기.
8. **라이브 패스스루 읽기 17 + `pod/terminal/ws` 무변경**(§J3·D-12).
9. **state.json 쓰기 금지** — 메인 세션 단일 writer. 구현은 코드·문서만.
10. **파일 라인캡** — 신규·성장 파일 전부 ≤650(800 경고). engine.go는 J1a에서 650 미만 분할 복귀.
11. **원격 CI 대기 금지** — claims가 전부 로컬 대응.
12. **승인 우회 창(J1c~J3)** — v1 create(무승인)과 V2 create(승인 필수)가 병존하는 기간은 J1c 머지부터 J3 머지까지로 최소화한다. J1c PR 설명에 창 존재·J2/J3 종결 예정을 명기하고, J2가 창의 사용자 노출을 제거한다(프론트 전환). 창 연장 금지.
13. **i18n ko/en 쌍 패리티·한자 혼입 금지** — 신규 문구 양 사전 동시 추가.
14. **J3는 6축 census를 PR 설명에 표로 첨부**(create 라우트 1건 — §2.1 갱신판). 죽음/생존 경계(§3.7)가 표의 전제다.
15. **스펙 편집은 조용히 두지 않는다** — §8.5·§10.2(J0)·§16.1(J1c)·§19.1 step 4 상환 주석(J3)이 각 PR에 동반. 역사 이탈 문언 삭제 금지.
16. **Secret 매니페스트 payload 노출 수용 기록(RI-12)** — create 태스크의 payload_json은 사용자 작성 매니페스트를 그대로 동결하며 taskView.Payload로 노출된다(§13 내구성 계약·재생·디버깅 원천). Secret 생성 매니페스트의 stringData가 포함될 수 있음을 tasks.go 주석 수식·PR 설명에 명시적으로 기록한다. 권한 게이트(Auth+task grant) 뒤 관제 면이라 **수용**한다 — payload 반식별화는 재생 계약을 깨뜨리므로 불채택.
17. **멱등키 소각 배선** — 커넥션-스코프 키 생성·소각은 하나의 공유 템플릿 상수를 소비한다(템플릿 이원화 금지). 종단 전환 시점 소각을 runOpTasks 폴 콜백이 수행한다(기존 패턴 승계).

---

## 6. 검증 요구 (claims — vacuity 금지 2단·존재 증명 포함, 전제 `cd /mnt/d/DEV/acc0mplish/ops-admin`, Phase 개시 시 `git tag i10-<phase>-base && git rev-parse --verify i10-<phase>-base`〔존재 검사〕)

```
--- 공통 (매 Phase) ---
CI1  [전 Phase] cd backend && go test ./... -race -count=1 → ok
CI2  [전 Phase] python3 scripts/secret-scan.py → exit 0
CIf  [전 Phase] wc -l backend/internal/tasks/engine.go backend/internal/tasks/engine_resolve.go
       backend/internal/infra/contract/k8s_create_face.go backend/internal/infra/compose/compose_k8s_ops.go
       backend/internal/infra/adapter/kubernetes/executor_create.go backend/internal/api/v2/operations_connection.go
       2>/dev/null | grep -v total | awk '$1>800' → 0행 · awk '$1>650' → 초과 파일은 같은 Phase 분할

--- J0 (def·어휘·매핑 표) ---
CI8  [J0] 2단+부호화: ① grep -c 'network.destination_rule\|network.service_entry' backend/internal/infra/contract/resource_kind.go → 2 이상
       ② cd backend && go test ./internal/infra/contract/ ./internal/infra/registry/ ./internal/infra/contracttest/ -count=1 → ok(DefTable 14종 갱신 포함)
       그리고 grep -c 'k8s.resource.create' backend/internal/infra/compose/compose_k8s_ops.go → 2 이상(def+등록) ·
       grep -c 'k8s.resource.create' backend/internal/infra/contracttest/opdef_table_test.go → 1 이상(테이블 행) ·
       grep -rn 'ConnectionScoped' backend/internal/infra/ → 0행(폐지 부호화 증명 — 빈 ResourceKinds 채택) ·
       awk '/^### 8.5/,/^## 9\./' docs/architecture/multi-infrastructure-control-plane-v2.md | grep -c 'destination_rule' → 1 이상(§8.5 개정 동반 — 헤더는 3중 해시 실측)
CIm  [J0] 2단: ① grep -c 'func TestK8sCreateFace' backend/internal/infra/contract/k8s_create_face_test.go → 1 이상(존재 증명 — V-1)
       ② cd backend && go test ./internal/infra/contract/ -run TestK8sCreateFace -count=1 -v 2>&1 | grep -c '^--- PASS' → 1 이상
       그리고 grep -c 'networking.istio.io\|gateway.networking.k8s.io' backend/internal/infra/contract/k8s_create_face.go → 2 이상
       (테스트는 v1 face 15 resourceType 전수 대응·그룹 구분 Gateway 이중·namespaced 플래그 단얫)

--- J1a (엔진 체인) ---
CI9  [J1a] wc -l backend/internal/tasks/engine.go → 650 이하(분할 계약) 이고
       grep -c 'func TestResolveExecutionChainConnectionScoped' backend/internal/tasks/engine_resolve_test.go → 1(존재 증명) 이고
       grep -c 'conn:' backend/internal/tasks/engine_resolve.go → 1 이상(분기 증명) 이고
       cd backend && go test ./internal/tasks/ -race -count=1 → ok — 신규 단얫: 합성 uid → 커넥션 직조립·실 uid → 기존 조인 2경로 +
       uid 포맷(접두·세그먼트·cluster-scope형) + 생성 uid 전수 'conn:' 비접두 가드(예약 단얫)

--- J1b (실행기 leg) ---
CI10 [J1b] test -f backend/internal/infra/adapter/kubernetes/executor_create.go → exit 0 이고
       grep -c '^func Test' backend/internal/infra/adapter/kubernetes/executor_create_test.go → 1 이상(존재 증명) 이고
       grep -n 'CreateOperationName' backend/internal/infra/adapter/kubernetes/executor.go → 1행(dispatch case — 유일 편집) 이고
       git diff --stat i10-j1b-base -- backend/internal/infra/adapter/kubernetes/executor_state.go → 출력 없음(무편집 증명 — state| 재사용) 이고
       cd backend && go test ./internal/infra/adapter/kubernetes/ -count=1 → ok
       (신원 가드·컬렉션 경로 15면·POST 201/409-에코일치/409-불일치 3분기·state| 핸들 부호화 단얫 — mock client 선례 executor_*_mock_test.go)

--- J1c (라우트+상수+스펙) ---
CI7  [J1c] grep -c 'POST /api/v2/infra/provider-connections/:uid/operations' docs/security/route-inventory.txt → 2 이고
       grep -c . docs/security/route-inventory.txt → 442 · grep -c . docs/security/sensitive-routes.txt → 282 그리고
       grep -n 'contract is 439' backend/router/routes_inventory_test.go → 1건 ·
       grep -n '429 (414 v1 + 15 v2)' backend/router/authz_replay_test.go → 1건 ·
       grep -n '!= 234' backend/router/authz_replay_test.go → 1건 (중간 상수 3곳 소유 증명)
CI13① [J1c] awk '/^### 16.1/,/^### 16.2/' docs/architecture/multi-infrastructure-control-plane-v2.md | grep -c 'provider-connections/{uid}/operations' → 1 이상(§16.1 근거 한정 — §8.5 확인은 CI8〔J0〕가 소유)

--- J2 (프론트) ---
CI11 [J2] 2단: ① 착수 전 grep -cE "http\.(post|put|delete)\('/api/v1/k8s" web/src/api/k8s.js → 1(존재 증명)
       ② 완료 후 같은 명령 → 0 이고
       grep -c 'createK8sResourceYAML\|stays on v1 by design' web/src/api/k8s.js → 0 이고
       grep -c 'createK8sConnectionOperationTask' web/src/api/k8s.js → 1 이상 이고
       grep -c 'k8sConnectionIdempotencyKey' web/src/api/k8s.js → 2 이상(생성+소각 헬퍼 쌍) 이고
       grep -c 'connectionScoped: true' web/src/views/assets/K8s.vue → 4(콜사이트 4곳 배선 증명) 이고
       grep -rn 'createK8sResourceYAML' web/src → 0행
CI12 [J2] cd web && bun install --frozen-lockfile && bun run build → exit 0 ·
       node scripts/check-i18n-parity.mjs (repo root에서 — 스크립트 경로 root 기준) → exit 0 ·
       한자 가드(korean-localization-guard.yml 인라인 python) → PASS
CI14 [J2 — 필수 시도] cd web && bun run e2e → exit 0. 대상 슬라이스는 slice-a(serve-a) — slice-b/c는 serve-b/c 전용 fixture라
       본 변경(k8s 면)의 레인 아니다(모드 규율). kind 미가용으로 회피 시: CI11 grep(4콜사이트 배선) + CI12 build를
       필수로 돌리고 재발화 단얫(종단 후 동일 버튼 재클릭 → 신규 태스크 uid ≠ 이전 — 가짜 재생 성공 방지)을
       코드 계약(§3.6 공유 템플릿·종단 소각) 리뷰 확인으로 대체, 사유·판정 경로 PR 기재(C61 선례 강화)

--- J3 (v1 원자 삭제·골든·baseline 종결) ---
CI3  [J3] grep -c . docs/security/route-inventory.txt → 441 · grep -c . docs/security/sensitive-routes.txt → 281
CI4  [J3] 하드코딩 음의 증명(3분리 — 최종): cd backend &&
       grep -n '!= 437\|!= 439\|contract is 437\|contract is 439' router/routes_inventory_test.go → 0행(438로) ·
       grep -n '!= 427\|!= 429\|414 v1 + 13 v2\|429 (414 v1 + 15 v2)' router/authz_replay_test.go → 0행(428·'413 v1 + 15 v2'로) ·
       grep -n '!= 232\|!= 234\|232 baseline\|234 baseline' router/authz_replay_test.go → 0행(233으로) 그리고
       grep -n 'contract is 438\|428 (413 v1 + 15 v2)\|233 baseline' router/routes_inventory_test.go router/authz_replay_test.go → 3건 양성
CI5  [J3] v1 create 면 소멸: grep -rn 'CreateK8sResourceYAML' backend/ --include=*.go → 0행 이고
       grep -rn 'yaml/create' backend/router backend/controller backend/service backend/opdef --include=*.go → 0행 이고
       cd backend && go build ./... → ok
CI6  [J3] opdef·char·baseline: grep -c 'yaml/create' backend/opdef/defs_monitor.go → 0 이고
       grep -c '^func TestChar' backend/service/k8s_path_test.go → 19(25−6) 이고
       wc -l backend/service/testdata/char-baseline.txt → 114 이고
       grep -c 'TestCharBuildK8sCreateResourcePaths\|TestCharBuildK8sYAMLResourcePath\|TestCharBuildK8sDeleteResourcePaths\|TestCharParseK8sManifestIdentity\|TestCharFriendlyK8sYAMLError' backend/service/testdata/char-baseline.txt → 0 이고
       grep -c 'PASS: TestCharIsK8sNotFoundError' backend/service/testdata/char-baseline.txt → 1(생존 증명) 이고
       cd backend && go test ./service/ -count=1 → ok
CI13② [J3] 2단: ① 착수 전 grep -c 'resolved:' docs/architecture/multi-infrastructure-control-plane-v2.md → 0(신규 패턴 증명)
       ② J3 후 → 1 이상(D-15 상환 주석) 이고
       grep -c 'carried as I10' docs/architecture/multi-infrastructure-control-plane-v2.md → 1 유지(역사 문언 보존 증명)
CI15 [J3] 잔여·생존 경계: grep -rn "http\.(post\|put\|delete)\('/api/v1/k8s" web/src → 0행(web/src 전역) 이고
       grep -rn 'buildK8sCreateResourcePaths\|buildK8sYAMLResourcePath\|buildK8sDeleteResourcePaths\|parseK8sManifestIdentity\|friendlyK8sYAMLError' backend/service --include=*.go → 0행(사멸 6함수·단수형 패턴이 복수형 포섭) 이고
       grep -c 'func buildIstioResourcePathsWithPreferred\|func buildGatewayAPIResourcePathsWithPreferred' backend/service/k8s_transport.go → 2(생존 증명 — 라이브 읽기 호출) 이고
       grep -c 'func isK8sNotFoundError' backend/service/k8s_path.go → 1(생존 증명) 이고
       grep -c 'K8sResourceYAMLPayload\|K8sResourceDeletePayload' backend/model/k8s.go → 0(고아 타입 처분) 이고
       grep -c 'k8sManifestIdentity' backend/service/k8s_types_mesh.go → 0(동일)

--- 종합 (구현 완료 후 1회 — 선택, 판정 경로 기재 의무) ---
CI16 [종합·선택] V2 왕복: kind kubeconfig → register-k8s → 서버 기동 →
       POST /api/v2/infra/provider-connections/{uid}/operations/k8s.resource.create/plan(body {yaml: Namespace 매니페스트}) → 200 ·
       execute(Idempotency-Key) → 201 → approve → 폴 succeeded → kubectl --context kind-v2-p2 get ns <name> → Found ·
       동일 key 재execute → 200 + Idempotency-Replayed: true ·
       종단 후 신규 key 재execute(재발화) → 신규 태스크 uid ≠ 이전(소각·재발화 계약 실증).
       미가용 시 대체 집합: CI9(체인 2경로) + CI10(실행기 3분기·핸들 부호화) + CI7(라우트 골든) +
       **operations_connection_test 지정 단얫: go test ./internal/api/v2/ -run TestPlanConnectionOperation -count=1 → ok
       (plan 200·execute 201·키 무지정 400·불소속 kind 400·CONNECTION_NOT_FOUND 404 — CI13① 대신이 아니라 J1c 본체 검증으로 J1c에도 적용)**
```

최소 회전 세트(Phase별): CI1·CI2·CIf (+J2: CI12). 존재 증명(V-1)은 CIm·CI9·CI10·(CI16 대체 집합)에 인라인.

---

## 7. 위험 매트릭스 · 롤백 (r2 — RI-9 재평가·신설 2건)

| # | 위험 | 가능 | 파급 | 완화 | 검출 |
|---|---|---|---|---|---|
| RI-1 | 엔진 체인 분기(conn:)가 태스크 엔진 불변식(T-5~T-7·유니크·재생)을 흔든다 | 중 | 높 | additive 분기(실 uid 경로 바이트 불변) + 2경로 단얫 + 기존 51 Test -race | CI9·CI1 |
| RI-2 | 골든 목표 정정(441/281 ≠ 사전 기술 439/279)이 리뷰·검증 충돌 | 높 | 중 | §0·§2.4 산술 전개 선제 문서화·CI3/CI4/CI7 단일 원천·중간 상수 3곳 소유 명시 | CI3·CI4·CI7 |
| RI-3 | 어휘 확장 2종 정당성 논쟁 | 중 | 중 | virtual_service P2-D 선례 원용 + §8.5 reviewed-diff 경로 + 스펙 서술 동반. 기각 시 UI istio 2종 기능 소실 | CI8·CIm·CI13① |
| RI-4 | istio Gateway vs Gateway API Gateway 혼동 | 중 | 높 | 매핑 표가 apiVersion group까지 키로 소비 — 그룹 불일치 사전 400·실행기 경로 후보도 그룹별 분기 + 단얫 | CI10·CIm |
| RI-5 | 409 재시도 의미론(크래시 재시도가 성공을 실패로 오보) | 중 | 중 | 409 시 동결 manifest 부분집합 에코 판정 — 일치=수렴 종단·불일치=즉시 충돌 실패(v1 패리티). delete terminal_404 대칭 | CI10 |
| RI-6 | UX 변화(동기 토스트→승인+폴 다이얼로그) | 높 | 저 | H1 9종과 동일 UX로 통일 — create가 마지막 예외였음. 문구 기존 사전 재사용(신규 1쌍) | CI12·리뷰 |
| RI-7 | J3 원자 삭제(~12파일) 폭발 반경 — H2 동일 성격 | 중 | 높 | E-1 순서·원자 편차 사전 승인·6축 census·죽음/생존 경계 확정(§3.7)·char-baseline 재생성 계약 | CI5·CI6·CI15 |
| RI-8 | 매핑 표 이중 구현 표류(API vs 실행기) | 중 | 중 | 표를 contract에 단일 배치·파생 계약 명문(§3.2.1) — CIm·CI10이 각각 잠금 | CIm·CI10 |
| **RI-9** (재평가) | **동일 목표 create의 무기한 잠금** — awaiting_approval 태스크가 active_flag=1을 쥔 채 방치되면 동일 목표 신규 create 전부 RESOURCE_BUSY 409. 키 맵은 메모리성(새로고침=새 키)이지만 유니크는 서버측 — 새로고침으로도 탈출 불가 | **중** | **중** | **탈출 경로 2개 실측 확보**: (1) RejectTask — awaiting_approval 전용 종단화·active_flag 해제 (2) CancelTask — Engine.RequestCancel 착지 완료(cancel.go:58 — operations.go/tasks.go "503 미착지" 주석은 낡은 문언이며 J1c tasks.go 주석 수식 시 함께 정정). 폴 5분 예산 다이얼로그의 /infra/tasks 탐색 링크가 방치 태스크 발견 경로. 리퍼·승인 만료는 M1 범위 밖(§13.6 이월 계열) — 계획은 탈출 경로 문서화로 대응 | CI16·리뷰 |
| RI-10 | K8s.vue 4 콜사이트 치환 리뷰 불가 규모 | 중 | 중 | op별 매핑표 PR 첨부·`connectionScoped: true` 4건 grep·runOpTasks target 치환은 기계적(R29 선례) | CI11 |
| **RI-11** (신설·위험4) | J1c~J3 승인 우회 창 — v1 create(무승인) 병존 기간에 무승인 경로 사용 | 중 | 저 | 창 최소화(J1c 즉시 J2·J3)·J1c PR 명기·J2가 사용자 노출 제거. M1 무배포 단계라 실 트래픽 부재 — §5 #12로 계약화 | 리뷰·CI11 |
| **RI-12** (신설·위험5) | Secret 생성 매니페스트의 payload_json 영속·taskView 노출 | 중 | 중 | **명시적 수용 기록**(§5 #16): §13 동결 계약·재생 원천이라 반식별화 불채택. 관제 면은 Auth+task grant 게이트 뒤. tasks.go 주석 가정 수식(J1c)·PR 설명 첨부 | 리뷰·CI13① |

**롤백**: 앵커 = I10 착수 전 커밋(`i10-j0-base`부터 Phase별 태그 — 존재 검사 CI 전문). J0·J1a·J1b·J1c additive — 개별 revert 무함(J1c revert 시 골든 중간값·상수 3곳도 함께 복귀). J2 revert 시 v1 create가 J3 이전엔 생존 — 무해(§5 #1 순서 계약). **J3 원자 revert**(부분 revert 불가). **cleanup 단계(위험3 채택)**: revert 후 잔여 `conn:` 접두 **비종단** 태스크 행은 active 잠금으로 남는다 — 롤백 절차에 "비종단 conn: 태스크를 실패 종단화하는 SQL 1문(`UPDATE provider_task SET status='failed' WHERE resource_uid LIKE 'conn:%' AND status NOT IN (종단 5종)` — 정확 종단 집합은 구현 시 §13.5 어휘 원문 대조) 또는 reject 라우트 순회"를 포함한다. "잔여 무해"는 **종단 행만** 참이다. 전체 역순: J3→J2→J1c→J1b→J1a→J0. DB 스키마 무변경 — 백업·복원 불요.

---

## 8. 이탈 기록 (D-15 상환 갱신 — 기존 deviation 표와 정합)

| # | 조항 | 적용 | 사유 | 처분 |
|---|---|---|---|---|
| **D-15** (phase6 §7) | §19.1 step 4 "legacy write paths stop" | 12/13 → **13/13 완결** | I10이 I-P1 블로커를 구조적으로 해소 — 스코프 앵커를 리소스 uid에서 커넥션 uid+매니페스트 신원으로 이동 | J3 착지 시 상환 — 스펙 §19.1 step 4에 `resolved:` 주석 추가(삭제 아님 — CI13② 2단)·phase6 §13 I10 행 폐쇄는 p6-state 기록 |
| **D-16** (승계) | §19.1 "§15 comparisons re-run" | 변경 없음 | compare k8s 페어링 종결(G1)과 무관 — I10 검증은 V2 왕복 claims(CI16·유닛테스트)이 대체 | 유지 — 신규 이탈 0 |
| 신규 없음 | §16.1·§8.5·§10.2 개정 | 이탈 아님 | reviewed-diff 개정 경로(스펙은 저장소 내 살아있는 문서) | — |

**리컨 정정 승계**: 골든 441/281(§2.4)은 과업 기술의 산술 오류 정정 — state.json task 문구 수치는 완료 보고에서 메인이 갱신(본 에이전트 state.json 무기록).

---

## 9. r2 처분표 (LOW 18 — 반영분/기각분)

| LOW (병합 보고 분류) | 처분 | 사유·반영 위치 |
|---|---|---|
| 완전 L6 e2e 모드 규율 | **채택** | CI14 — slice-a(serve-a) 한정·serve-b/c 병렬 레인 금지 명시(web-e2e-stack-modes 계약) |
| 완전 L7 골든 -update 절차 본문 | **채택** | §3.7 재생성 명령 원문·CI 전문 태그 존재 검사 |
| 완전 L8 k8s.js 226행 | **채택** | §2.3 정정(227→226) |
| 완전 L9 §14.2-3 인용 | **채택** | 헤더 원천 란 |
| 기술 F7 "12스위트"→실측 | **채택** | §3.3·CI9 — 51 Test 함수(실측 합산) |
| 기술 F8 CI9 알파벳(포맷) 단얫 | **채택** | CI9 — uid 포맷·예약 가드 단얫 포함 |
| 기술 F9 낡은 주석 grep 불가 | **채택(절차 조정)** | §2.4·§3.7 — 산문 수정은 리뷰 확인 항목으로 명시(grep claim 포기) |
| 기술 F10 226행(중복) | **채택** | L8과 동일 |
| 위험 F8 RFC1123 charset 가드 | **기각** | 이름 검증의 권위는 k8s API 서버 — 게이트 이중화는 검증 드리프트만 추가(§3.2.1 가드는 비empty·표 소속·namespace 규칙으로 충분) |
| 위험 F9 repo 외 census 통지창 | **기각** | M1 무배포 — 통지 대상 트래픽 부재. J2 프론트 전환·J1c PR 명기(§5 #12)가 실질 통지 |
| 단순 F3 J0+J1a 병합 | **기각** | 병합 시 6~7파일로 ≤5 위반·직렬 PR이 리뷰 단위 최소(§4 논거 r1 승계) |
| 단순 F4 grep 3곳 보강 | **채택** | CI4 3분리·CI11 헬퍼 쌍·CI6 생존 증명 |
| 단순 F5 §10.2 브리프 불일치 | **채택** | §3.8 배정 통일(J0)로 해소 |
| 검증 V-10 CI4 3분리 | **채택** | CI4 재작성 |
| 검증 V-11 CI12 경로 | **채택** | CI12 "repo root에서" 명시 |
| 검증 V-12 CI15 web/src 전역 | **채택** | CI15 재작성 |
| 검증 V-13 CI13① 절 근거 | **채택** | CI13① awk §16.1/§8.5 범위 한정 |
| 검증 V-14 태그 존재 검사 | **채택** | §6 전문 `git rev-parse --verify` |

---

## 10. 완료 판정 (구현·리뷰에 인계)

- J3 머지 시점에 §19.1 step 4는 13/13 — D-15 상환·I-P1 폐쇄·phase6 §13 I10 폐쇄(메인 기록).
- 골든 최종 441·281·"428 (413 v1 + 15 v2)"·233 — 이후 골든 논의의 기준점(CI3·CI4 원천).
- char-baseline 114행 — C47 불가침 계약의 갱신 선례.
- 다음 이탈 후보: I3(§16.1 POST /provider-connections — R26 해소 경로)·I5(hardcap) — 본 계획 범위 밖.
