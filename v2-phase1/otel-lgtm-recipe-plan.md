# OTel/Mimir 레시피 계획 — 토폴로지 A 확정 이행 (r2)

- task-id: `otel-lgtm-recipe` · 티어 L · r2 개정 2026-09-08 · 기준 트리 main `30856a3`
- 선행: `v2-phase1/otel-telemetry-assessment.md`(실측 승계) · r1 갭 번들 `docs/task-id/otel-lgtm-recipe/gaps-r1.md`(CRITICAL 2 / HIGH 11)
- **사용자 확정(2026-09-08)**: **D-1 기각** — C 트랙은 v1 `monitor:query` 재사용 금지, `/api/v2` 전용 라우트는 별도 후속 과제. **D-2 = 별도 Mimir 배포**(올인원 아님). 따라서 **본 계획은 A 트랙만 확정한다.** C 트랙 자산은 `v2-phase1/otel-c-track-handoff.md`로 분리 보존.
- **스펙 인용은 행 번호가 아니라 절 제목 기준**이다(H-6 근본 해법 — 병행 계획 `otel-m18-ext`가 같은 파일 §18.2 표에 2행을 삽입해 행 좌표가 착지 순서에 따라 반드시 어긋난다). 코드 file:line은 그대로 쓴다.
- **산출물은 문서·설정 파일이다. `backend/`·`web/` 코드 변경 0.**

---

## 0. 결론 먼저 (BLUF)

1. **r1의 최대 위험(메트릭 이름 OTLP 왕복 비보존)은 실측으로 해소됐다.** 2026-09-08 스모크 스택(`grafana/alloy:v1.19.2` + `grafana/mimir:3.2.0`)에서 §18.2 골든 전문을 스크랩→OTLP→Mimir로 흘린 결과 **14종 패밀리 이름이 전부 그대로 보존**됐다(`_total`·`_bucket`·`_sum`·`_count` 포함). 라벨(`connection`·`provider`·`op` …)과 `job`/`instance`도 보존. 절차·결과는 §9 R-1.
2. **수신 측이 산출물에 들어왔다(H-1).** `docker-compose.otel.yml`에 **Mimir(monolithic, HTTP 9009) + Grafana + Alloy + pve-exporter** 4서비스 + `deploy/mimir/mimir.yaml`·`deploy/grafana/provisioning/`. r1의 존재하지 않던 호스트명 `lgtm:4317`은 전부 제거됐다. k8s는 설정만이 아니라 **RBAC·ConfigMap·Deployment까지** 산출물이다(HIGH-T3).
3. **PVE는 선택 의존이 됐다(H-2 상환).** `profiles: ["pve"]` + `${VAR:-}` 기본값 — PVE 자격이 없는 환경에서 `docker compose config`가 성공함을 실측 확인했다(§9 R-2).
4. **경계는 네트워크로 시행한다(H-8·H-9 상환).** 신규 네트워크 `ops-admin-telemetry`를 만들고 **alloy만 두 네트워크를 잇는다** — `api`·`mysql`·`web`은 pve-exporter·Mimir에 도달할 수 없다. 호스트에 열리는 포트는 **Grafana 하나뿐이며 루프백 바인드**(`127.0.0.1:3000:3000`)다 — `docs/DEPLOY_DOCKER_COMPOSE.md` §7.2가 제시한 `127.0.0.1:8080:80` 선례와 같은 형식.
5. **멀티테넌시 재판정 완료.** r1의 "강제 비활성" 근거(ops-admin v1 인증 코드가 `X-Scope-OrgID`를 못 실음)는 **D-1 기각으로 소멸**했다. 이제 소비자는 Grafana뿐이고 양쪽 다리 모두 헤더를 실을 수 있음을 실측 확인했다(Alloy `headers` 블록 validate 통과 / Grafana 프로비저닝 `jsonData.httpHeaderName1`+`secureJsonData.httpHeaderValue1` 실제 적용 확인). **기본값은 전용 단일 테넌트 배포 자세로서 `multitenancy_enabled: false`, 공유 시 활성화**가 문서화된 탈출구다.
6. **스펙 절 번호는 §18.5 하나뿐이다(C-1 상환).** `### 18.4 AI tool path authorization`가 이미 점유돼 있음이 확인됐다. semconv 매핑은 **§18.5**로 착지하고, r1의 §18.5(질의 계약) Phase는 **삭제**됐다(D-1 기각 — H-11 동시 상환).

---

## 1. 실측 재확인 (본 계획의 사실 기반)

### 1.1 컨트롤 플레인 자기 메트릭 (스크랩 대상)
- 렌더 골든 전문: `backend/internal/infra/metrics/metrics_test.go:118-218` — 14종 패밀리, `# TYPE` 헤더 패밀리당 1회, counter 전부 `_total`, histogram `_bucket{le=…}` 오름차순 + `+Inf` + `_sum` + `_count`. `# HELP` 라인은 없다.
- 엔드포인트: `backend/main.go:31` `engine.GET("/internal/metrics", …)`(`registerMetricsRoute` :27-36). 인증 게이트 없음(:24-26 주석이 가정 A1을 명시), 스택 실패 시 `renderMetrics == nil` → 미등록(:28). 바인드는 `main.go:117` `Addr: ":" + cfg.App.Port` 전 인터페이스, 포트 `8082`(`deploy/config.yaml.example`).
- 렌더 함수 노출: `backend/engine_config.go:155`(실패 경로 `:142` 4-nil). 패키지 순도: `metrics.go:8-10` "imports the standard library only".
- **패밀리 조기 생략(H-4)**: `gauge.go:36-39`(`len(c.health)==0`)·`gauge.go:55-56`(`!c.queueDepthSet`) + `render.go:53,70,100,111,123,145,156,165,176` — 총 **10개 패밀리가 조건부 생략**된다. 커넥션 미등록·sweep 미실행 스택의 `/internal/metrics`는 **빈 문자열일 수 있고 그것이 정상 상태다.**
- 스펙 근거(절 제목 기준): §18.2 `Control-plane metrics — instrumented where` 표의 M1=yes 14종 + deferred 2종. §10.2 `Operation definition` 문단의 "**No new runtime dependencies in M1**".

### 1.2 배포 형상
- `docker-compose.yml` — 서비스 3개(`mysql`·`api`·`web`), 네트워크 `ops-admin`(외부명 `ops-admin-network`, :81-83).
- **`api`에는 `ports:` 블록이 없다**(:28-55). 호스트 포트를 매핑하는 서비스는 `web`(`8080:80`, :66-67) 하나뿐 — `/internal/metrics`의 외부 미노출은 **compose 형상**이 보장한다.
- 저장소 선례 3종: 오버레이 = `docker-compose.dns.yml:1-7` + 운영 문서 §7.3의 `-f` 2개 사용법 / 루프백 바인드 = 운영 문서 §7.2 `"127.0.0.1:8080:80"` / `:ro` 설정 마운트 = `docker-compose.yml:43`.
- **`deploy/`에는 `.env`가 없다**(`.env.example`·`config.yaml.example`뿐) — 운영 문서 §4가 `cp deploy/.env.example deploy/.env`를 지시한다. 모든 claim 명령은 이 복사가 선행됐다고 전제하고 그 명령을 함께 적는다.

### 1.3 `backend/internal/api/v2/` 정합 (H-7 상환)
**A 트랙은 `api/v2`와 접점이 0이다.** 근거 — A 트랙 산출물은 compose 오버레이·Alloy 설정·Mimir/Grafana 설정·문서뿐이고, 다음 표면을 **읽지도 고치지도 않는다**:
- `backend/internal/api/v2/infra.go:84-89` `InfraAPI.Register` — GET 4종(`/provider-types`·`/provider-connections`·`/resources`·`/resources/:uid`).
- `backend/router/router.go:535-536` — `/api/v2/infra` 그룹 + `middleware.Auth(db)`·`middleware.OperationLog(db)`.
- 라우트 골든 `docs/security/route-inventory.txt`(453줄, v2 행 254-261) — 무변경(보존 제약 #5).

**후속 과제(C 트랙)를 위한 계약 이관**: 신규 `GET /api/v2/infra/...` 행은 **route-inventory.txt를 반드시 변경시킨다.** 본 계획의 보존 제약 #5는 **본 계획에만 구속력이 있으며**, 후속 과제는 골든 재생성을 자기 범위에 포함해야 한다. 상세는 `v2-phase1/otel-c-track-handoff.md` §2.2.

### 1.4 어휘·URN 형상 (semconv 매핑의 좌변)
- 어휘 21종: `backend/internal/infra/contract/resource_kind.go:4-24`(M1 19종) + `:32-35`(Phase2 확장 2종). 조회부 `IsKnownResourceKind`(:40-52).
- 어댑터가 실제 발행하는 kind(리터럴 grep): kubernetes = `orchestration.{namespace,node,pod,workload,configmap,secret}`·`network.load_balancer`·`storage.{pool,volume}` / proxmox = `compute.{hypervisor_node,vm,system_container}`·`network.interface`·`storage.pool` / aliyun·tencent = `compute.vm`(범위 제외). **나머지 8종은 어휘만 존재하고 발행 어댑터가 없다.**
- URN 형상 실측은 `v2-phase1/otel-semconv-mapping.md` §0.

---

## 2. assessment 문서 개정 지시

`v2-phase1/otel-telemetry-assessment.md`를 아래 6건으로 개정한다.

| # | 대상 | 개정 내용 |
|---|---|---|
| **R-1** | §5 표 헤더 "(권장)" | `A` 행 → `(확정 — 사용자 2026-09-08)`. `B` 행 → `**기각(2026-09-08)**`. `C` 행 → `**v1 재사용 기각 — /api/v2 전용 라우트로 별도 후속 과제**`(근거: `otel-c-track-handoff.md` §1). 하단 문장을 "확정: A. C는 후속 과제로 분리"로 교체 |
| **R-2** | §9 질문 1·2 | 제목을 `## 9. 블로킹 질문 — 해소 기록(2026-09-08)`로 바꾸고 답 기재. 1 = **A 확정, C는 후속 과제**. 2 = **§10.2 유지**(개정하지 않음) |
| **R-3** | §9 질문 3 | **k8s + PVE 둘 다 포함, PVE는 exporter 경로 ① 채택.** 근거 = 본 계획 §4.7 |
| **R-4** | **§8 항목 4 하나** | **정정 지점은 §8 항목 4 하나다** — §2 표는 헤더가 "contrib 전용 receiver"라 이미 정확하고(해당 문장은 표의 **Kubernetes 행**에 있으며 PVE 행에는 없다) 고칠 것이 없다. §8 항목 4의 "k8s는 `k8sclusterreceiver`+`kubeletstatsreceiver`"는 **Contrib 기준 사실이고 Grafana Alloy에는 두 컴포넌트가 없다**(probe validate가 `cannot find the definition of component name "otelcol.receiver.kubeletstats"` 반환 — 재검증 완료). Alloy 경로는 `discovery.kubernetes`/`discovery.kubelet` + `prometheus.scrape`다 |
| **R-5** | §6 매핑 표 | 본 계획 §6의 21종 전수 표로 교체. semconv **v1.44.0** 고정 명시. `host.*`가 Stable이 아니라 **Development**임을 기록 |
| **R-6** | §3.1 다이어그램·본문 | 수신처를 **Mimir(monolithic, HTTP 9009, `/otlp/v1/metrics` 수집 · `/prometheus` 질의)** 로 확정 표기. exporter는 `otelcol.exporter.otlphttp`(§4.5 근거) |

개정은 문언 교체만 — 기존 file:line 실측·§4 트레이스/로그 분석·§7 보안 제약은 손대지 않는다.

---

## 3. 가정

- **A1** `/internal/metrics`는 무인증이고 "internal-only"는 **배포 경계 시행**이다. 근거: `backend/main.go:24-26`, 스펙 §18.2에 인증 문언 부재. 본 계획은 이 경계를 **compose 네트워크 분리 + `ports:` 블록 부재**로 구체화한다(§4.1).
- **A2** Alloy는 `ops-admin-network` 안에서 `http://api:8082/internal/metrics`를 스크랩한다. 근거: `docker-compose.yml:19-20,45-46,81-83` + `main.go:117` 전 인터페이스 바인드.
- **A3** `alloy`·`pve-exporter`·`mimir`는 **호스트 포트를 하나도 발행하지 않는다**(Alloy UI 12345 포함 — `--server.http.listen-addr=127.0.0.1:12345`로 컨테이너 루프백 바인드). **진단 수단 실측**: Alloy 이미지에는 `wget`·`curl`·`busybox`·`nc`·`python3`가 **전부 없다**(`command -v` → rc=127) — `exec … wget` 류 절차는 성립하지 않는다. 대신 ① `logs`, ② 이미지에 실재하는 **`bash`의 `/dev/tcp`**(실측 동작), ③ 네트워크에 붙인 **일회용 `curlimages/curl` 컨테이너**를 쓴다. `grafana/mimir:3.2.0`은 **셸조차 없는 distroless**(`/bin/mimir` 단독, `docker inspect` 실측)라 `exec`·`CMD-SHELL` 자체가 불가능하다.
- **A4** 수신처는 **Mimir monolithic 단일 인스턴스**(D-2 확정) — 수집 `POST /otlp/v1/metrics`, 질의 접두사 `/prometheus`, HTTP 포트 **9009**. `grafana/otel-lgtm` 올인원은 채택하지 않는다.
- **A5** OTLP 송출은 **`otelcol.exporter.otlphttp`** 다. **Mimir에는 OTLP gRPC 수신 엔드포인트가 없다** — 공식 HTTP API가 `POST /otlp/v1/metrics` 하나만 규정한다. 따라서 r1의 gRPC 기본값은 D-2 확정에 의해 **강제 변경**됐고, 변경 후 형상은 실측으로 대체 확인됐다(§9 R-1).
- **A6** **멀티테넌시 기본값 = `multitenancy_enabled: false`** — 전용 단일 테넌트 배포의 자세이지, r1처럼 ops-admin 코드 한계에 의한 강제가 **아니다**(D-1 기각으로 그 근거 소멸). Mimir를 다른 팀·환경과 공유하면 활성화하고 `X-Scope-OrgID`를 양 다리에 싣는다 — Alloy `otelcol.exporter.otlphttp`의 `headers` 블록(validate 통과 실측) + Grafana 프로비저닝 `jsonData.httpHeaderName1`/`secureJsonData.httpHeaderValue1`(실제 적용 확인 — `secureJsonFields.httpHeaderValue1: true` 응답).
- **A7** k8s 검증 대상은 **kind 픽스처 `v2-p3`** 다(`k8s-fixture/README.md`) — 프로덕션 형상 클러스터는 저장소에 없으므로 k8s claim은 kind 기준으로만 검증 가능하고, **게이트 클러스터 `v2-p2`는 건드리지 않는다**. **A8** PVE exporter에는 `pve-provisioning.md` §1-a의 **RO 토큰(PVEAuditor)** 만 주고 OPS 토큰은 절대 넣지 않는다.
- **A9** PVE 내장 metric server의 **필드 집합은 미확인**(공식 wiki가 `influxdbproto`=`http`/`https`는 명시하나 필드 목록 부재) — 경로 ② 보류 근거. **A10** semconv는 **v1.44.0**(2026-08-04)으로 고정한다.
- **A11** 본 계획은 **현행 14종만** 전제한다(확장 2종은 §7에서만). **A12** `deploy/.env`는 운영자가 `cp deploy/.env.example deploy/.env`로 만든다(운영 문서 §4) — 모든 명령이 그 복사를 선행 단계로 포함한다.

---

## 4. A 트랙 — 배포 레시피

### 4.1 토폴로지와 보안 경계

```text
[ops-admin-network]                    [ops-admin-telemetry]
  mysql ─ api ─ web                      mimir ── grafana
            ▲                              ▲        │
            │ scrape /internal/metrics     │ OTLP   │ 127.0.0.1:3000
            └────────── alloy ─────────────┘ HTTP   ▼
                          │  ▲                    (운영자 브라우저 / 리버스 프록시)
                          │  └── scrape ── pve-exporter  [ops-admin-telemetry]
                          │                (profile: pve)
              alloy만 두 네트워크를 잇는다
```

**보안 경계 — 런북에 그대로 실을 문언**:

> 1. `/internal/metrics`는 인증 게이트가 없다(`backend/main.go:24-26`). 유일한 방어선은 **`api` 서비스가 호스트 포트를 발행하지 않는다는 사실**이다(`docker-compose.yml:28-55`에 `ports:` 없음). `docker-compose.otel.yml`은 `api` 정의를 **일절 수정하지 않으며** 특히 `api`에 `ports:`를 추가하지 않는다.
> 2. `alloy`·`mimir`·`pve-exporter`에 `ports:` 블록을 두지 않는다. Alloy 자체 UI는 `--server.http.listen-addr=127.0.0.1:12345`로 **컨테이너 루프백**에만 바인드한다. **진단은 로그가 1차 창구다** — Alloy 이미지에 HTTP 클라이언트가 없다(A3 실측). 자기 메트릭이 필요하면 컨테이너 안 `bash`의 `/dev/tcp`를 쓴다:
> `docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env exec -T alloy bash -c 'exec 3<>/dev/tcp/127.0.0.1/12345; printf "GET /metrics HTTP/1.0\r\n\r\n" >&3; cat <&3'`
> Mimir는 distroless라 `exec`이 불가능하므로 일회용 컨테이너로 조회한다:
> `docker run --rm --network ops-admin-telemetry curlimages/curl:8.11.1 -s http://mimir:9009/ready`
> 3. **호스트에 열리는 유일한 신규 포트는 Grafana이며 루프백 바인드다** — `127.0.0.1:3000:3000`. 외부 접근이 필요하면 기존 Nginx/HAProxy/Traefik 앞단에서 TLS 종단·인증을 붙인다(운영 문서 §7.2와 동일 방침). 익명 접근은 끈다(`GF_AUTH_ANONYMOUS_ENABLED=false`), 가입은 막는다(`GF_USERS_ALLOW_SIGN_UP=false`), 관리자 비밀번호는 `deploy/.env`에서 필수로 받는다.
> 4. **`pve-exporter`는 자체 인증이 없고 `?target=` 파라미터로 임의 PVE 호스트를 조회할 수 있는 자격 보유 컨테이너다.** 따라서 별도 네트워크 `ops-admin-telemetry`에만 두고 **`alloy`만** 그 네트워크에 함께 참여시킨다 — `api`·`mysql`·`web`은 `http://pve-exporter:9221/pve?target=…`에 도달할 수 없다. (갭 번들이 제시한 3안 중 **네트워크 분리**를 채택했다.)
> 5. `web`(Nginx) 프록시 규칙에 `/internal/` 경로를 **추가하지 않는다**. 현재 프록시 대상은 `/api/v1/`·`/uploads/`뿐이다.
> 6. **새 스택의 `/internal/metrics`가 비어 있는 것은 정상이다** — 커넥션 미등록·health sweep 미실행 시 렌더러의 10개 패밀리가 조기 생략된다(`gauge.go:36-39`·`render.go:53` 외 8곳). 장애로 판정하지 않는다.

### 4.2 compose 오버레이 — `docker-compose.otel.yml` (신규)

`docker-compose.dns.yml` 선례와 동일한 **추가만 하는 오버레이**다. 기존 3서비스 정의는 한 줄도 바꾸지 않는다.

```yaml
# docker-compose.otel.yml — A 트랙(텔레메트리) 선택 오버레이.
# 사용: docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env up -d
# PVE 포함:  ... --profile pve ... up -d
name: ops-admin
services:
  mimir:
    image: grafana/mimir:3.2.0
    container_name: ops-admin-mimir
    restart: unless-stopped
    command: ["-config.file=/etc/mimir/mimir.yaml"]
    environment:
      TZ: ${TZ:-Asia/Shanghai}
    volumes:
      - ./deploy/mimir/mimir.yaml:/etc/mimir/mimir.yaml:ro
      - ops-admin-mimir-data:/data
    networks: [ops-admin-telemetry]
    # healthcheck 없음 — 이 이미지는 distroless다(`/bin/mimir` 단독). CMD/CMD-SHELL 둘 다
    # 실행할 셸·HTTP 클라이언트가 없어 어떤 healthcheck도 영구 unhealthy가 된다(실측).

  grafana:
    image: grafana/grafana:13.2.1
    container_name: ops-admin-grafana
    restart: unless-stopped
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      GF_SECURITY_ADMIN_PASSWORD: ${OTEL_GRAFANA_ADMIN_PASSWORD:?set OTEL_GRAFANA_ADMIN_PASSWORD in deploy/.env}
      GF_USERS_ALLOW_SIGN_UP: "false"
      GF_AUTH_ANONYMOUS_ENABLED: "false"
    ports:
      - "127.0.0.1:3000:3000"        # 루프백 전용 — 운영 문서 §7.2 선례
    volumes:
      - ./deploy/grafana/provisioning:/etc/grafana/provisioning:ro
      - ops-admin-grafana-data:/var/lib/grafana
    networks: [ops-admin-telemetry]
    depends_on: [mimir]      # service_healthy 조건 불가 — 위 주석 참조. Grafana는 부재 데이터소스를 견딘다

  alloy:
    image: grafana/alloy:v1.19.2
    container_name: ops-admin-alloy
    restart: unless-stopped
    command:
      - run
      - --server.http.listen-addr=127.0.0.1:12345
      - --storage.path=/var/lib/alloy/data
      - --disable-reporting           # Grafana 사용 통계 아웃바운드 차단(폐쇄망 경계)
      - /etc/alloy                    # 디렉터리 로드 — PVE 조각을 파일로 켜고 끈다(§4.7)
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      OTLP_ENDPOINT: ${OTEL_OTLP_ENDPOINT:-http://mimir:9009/otlp}
    volumes:
      - ./deploy/alloy:/etc/alloy:ro
      - ops-admin-alloy-data:/var/lib/alloy/data
    networks: [ops-admin, ops-admin-telemetry]   # 두 망을 잇는 유일한 컨테이너
    depends_on:
      api:
        condition: service_healthy

  pve-exporter:
    image: prompve/prometheus-pve-exporter:3.10.0
    container_name: ops-admin-pve-exporter
    restart: unless-stopped
    profiles: ["pve"]                 # --profile pve 없이는 아예 렌더되지 않는다(H-2)
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      PVE_USER: ${OTEL_PVE_USER:-}
      PVE_TOKEN_NAME: ${OTEL_PVE_TOKEN_NAME:-}
      PVE_TOKEN_VALUE: ${OTEL_PVE_TOKEN_VALUE:-}
      PVE_VERIFY_SSL: ${OTEL_PVE_VERIFY_SSL:-false}
    networks: [ops-admin-telemetry]

networks:
  ops-admin-telemetry:
    name: ops-admin-telemetry

volumes:
  ops-admin-mimir-data: { name: ops-admin-mimir-data }
  ops-admin-grafana-data: { name: ops-admin-grafana-data }
  ops-admin-alloy-data: { name: ops-admin-alloy-data }
```

`deploy/.env.example`에 추가할 항목(기존 항목 무수정): `OTEL_GRAFANA_ADMIN_PASSWORD`(필수), `OTEL_OTLP_ENDPOINT`(기본 `http://mimir:9009/otlp`), `OTEL_GRAFANA_BASE`(기본 `http://127.0.0.1:3000` — §6 딥링크의 `{GRAFANA_BASE}` 정의), `OTEL_PVE_USER`·`OTEL_PVE_TOKEN_NAME`·`OTEL_PVE_TOKEN_VALUE`·`OTEL_PVE_VERIFY_SSL`(전부 선택). **스크랩 대상 주소는 환경변수가 아니라 `pve.alloy` 파일에 적는다**(§4.7 — compose `:?` 가드는 profile 제외 서비스에도 적용돼 비-PVE 경로를 깨뜨림을 실측 확인했으므로 fail-fast를 파일 쪽으로 옮겼다).

**태그 실재 확인(2026-09-08 실측)**: `grafana/alloy:v1.19.2`·`grafana/mimir:3.2.0`·`grafana/grafana:13.2.1`·`prompve/prometheus-pve-exporter:3.10.0` 전부 존재. **`prometheus-pve-exporter:v3.10.0`은 존재하지 않는다**(GitHub 릴리스 태그에는 `v`가 붙지만 Docker Hub 태그에는 없다).

### 4.3 Mimir 수신 측 — `deploy/mimir/mimir.yaml` (신규)

```yaml
# 단일 인스턴스(monolithic) Mimir — ops-admin 전용 사설 배포.
target: all
multitenancy_enabled: false        # A6 — 전용 단일 테넌트 자세. 공유 시 true + X-Scope-OrgID

server:
  http_listen_port: 9009
  log_level: warn

common:
  storage:
    backend: filesystem
    filesystem: { dir: /data/common }

blocks_storage:
  backend: filesystem
  filesystem: { dir: /data/blocks }
  bucket_store: { sync_dir: /data/tsdb-sync }
  tsdb: { dir: /data/tsdb }

compactor:
  data_dir: /data/compactor
  sharding_ring: { kvstore: { store: memberlist } }

distributor:
  ring: { kvstore: { store: memberlist } }

ingester:
  ring:
    kvstore: { store: memberlist }
    replication_factor: 1

store_gateway:
  sharding_ring: { replication_factor: 1 }

ruler_storage:
  backend: filesystem
  filesystem: { dir: /data/rules }

limits:
  compactor_blocks_retention_period: 30d    # 보존 기간 — 운영 요구에 맞게 조정
```

- 전 데이터는 `ops-admin-mimir-data` 볼륨의 `/data` 아래 — 컨테이너 재생성으로 소실되지 않는다. 수집 `POST http://mimir:9009/otlp/v1/metrics`, 질의 `http://mimir:9009/prometheus/api/v1/query{,_range}`.
- **단일 인스턴스 filesystem 구성은 고가용성을 제공하지 않는다.** 운영 관측용이며 원본 데이터의 정본이 아니다 — 소실 시 스크랩 재개로 새로 쌓는다.

### 4.4 Grafana — `deploy/grafana/provisioning/datasources/mimir.yaml` (신규)

```yaml
apiVersion: 1
datasources:
  - name: Mimir
    type: prometheus
    access: proxy
    uid: ops-admin-mimir            # {DS_UID} — §6 딥링크가 참조하는 고정값
    url: http://mimir:9009/prometheus
    isDefault: true
    jsonData:
      httpMethod: POST
      prometheusType: Mimir
      timeInterval: 30s             # §4.5 scrape_interval과 일치
    # 멀티테넌시 활성 시에만 아래 2블록을 추가한다(A6 — 실측 확인된 키):
    #   jsonData:       { httpHeaderName1: X-Scope-OrgID }
    #   secureJsonData: { httpHeaderValue1: <tenant-id> }
```

- **`provisioning/dashboards/`·`plugins/` 빈 디렉터리도 함께 만든다**(`.gitkeep`) — 없으면 Grafana가 기동 로그에 `can't read ... provisioning files from directory` 오류를 남긴다(실측). 기능 장애는 아니나 운영자가 진짜 오류와 혼동한다. `{GRAFANA_BASE}` = `OTEL_GRAFANA_BASE`(기본 `http://127.0.0.1:3000`) — §6 딥링크의 유일한 정의처.

### 4.5 Alloy — ops-admin `/internal/metrics` 경로 (`deploy/alloy/config.alloy`)

```alloy
// --- 공통 OTLP 송출 ---
otelcol.exporter.otlphttp "mimir" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
    tls { insecure = true }        // 사설망 전용. 공인 경로면 insecure를 끄고 CA를 지정한다.
  }
}

otelcol.processor.batch "default" {
  output { metrics = [otelcol.exporter.otlphttp.mimir.input] }
}

otelcol.processor.memory_limiter "default" {
  check_interval = "1s"
  limit          = "256MiB"
  output { metrics = [otelcol.processor.batch.default.input] }
}

otelcol.receiver.prometheus "bridge" {
  output { metrics = [otelcol.processor.memory_limiter.default.input] }
}

// --- ops-admin 컨트롤 플레인 자기 메트릭 (§18.2 14종) ---
prometheus.scrape "ops_admin_control_plane" {
  targets = [{
    __address__ = "api:8082",              // 가정 A2 — compose 서비스명 DNS
    job         = "ops-admin-control-plane",
  }]
  metrics_path    = "/internal/metrics"
  scrape_interval = "30s"                  // 저카디널리티 14종 — 15s로 좁힐 이유가 없다
  scrape_timeout  = "10s"                  // 반드시 scrape_interval 이하 (§9 R-4)
  honor_labels    = true
  forward_to = [otelcol.receiver.prometheus.bridge.receiver]
}
```

- **`otlp` → `otlphttp`는 D-2 확정에 따른 강제 변경이다** — Mimir에는 OTLP gRPC 수신 엔드포인트가 없고 공식 HTTP API가 `POST /otlp/v1/metrics` 하나만 규정한다. r1 형상은 validate를 통과했으나 **수신처가 그 프로토콜을 받지 않는다**; 변경 후 형상은 validate + 실제 인제스트 양쪽으로 대체 확인됐다(§9 R-1).
- **채택 근거**: `prometheus.scrape` → `otelcol.receiver.prometheus`는 Prometheus exposition 서비스에 대한 OTel 공식 권장 경로이고 ops-admin 코드는 0줄 바뀐다 — §10.2 `Operation definition`의 "No new runtime dependencies in M1"을 지키는 채택 근거다(보존 제약 #3). **한계**: 평문 text exposition은 exemplar를 담지 못해 메트릭→트레이스 클릭 항해는 얻지 못한다.

### 4.6 k8s 장비 텔레메트리 — `deploy/alloy/config-k8s.alloy` (신규)

**assessment §8-4 정정(R-4)**: Alloy에는 `kubeletstatsreceiver`도 `k8sclusterreceiver`도 없다(재검증 완료). Alloy 네이티브 경로를 쓴다.

**선택 근거**: ops-admin·PVE 트랙이 이미 Alloy를 요구하므로 **에이전트 1종 유지**가 운영상 우위. Contrib Collector를 2호 에이전트로 추가하면 문법·버전·업그레이드 경로가 둘로 갈라진다. 대가는 수집 메트릭이 kubelet/cAdvisor의 Prometheus 계열이라 OTel `k8s.*` 메트릭 이름 규약과 다르다는 점 — semconv는 §6의 **리소스 속성** 층에서 붙이고 메트릭 이름은 kubelet 원본을 쓴다.

클러스터 **내부** 배포용이며 compose 오버레이의 alloy와 **별개 인스턴스**다. 따라서 **자체 OTLP 체인을 갖는다** — `config.alloy`의 `bridge`를 참조하면 단독 검증이 `component … does not exist or is out of scope`로 실패한다(r1에서 실측으로 발견·수정).

```alloy
// 클러스터 내부 전용 독립 인스턴스 — 자체 OTLP 체인을 갖는다.
otelcol.exporter.otlphttp "mimir" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
    tls { insecure = true }
  }
}

otelcol.processor.batch "default" {
  output { metrics = [otelcol.exporter.otlphttp.mimir.input] }
}

otelcol.processor.memory_limiter "default" {
  check_interval = "1s"
  limit          = "256MiB"
  output { metrics = [otelcol.processor.batch.default.input] }
}

otelcol.receiver.prometheus "bridge" {
  output { metrics = [otelcol.processor.memory_limiter.default.input] }
}

discovery.kubernetes "nodes" { role = "node" }

discovery.relabel "kubelet_cadvisor" {
  targets = discovery.kubernetes.nodes.targets
  rule {
    target_label = "__address__"
    replacement  = "kubernetes.default.svc:443"
  }
  rule {
    source_labels = ["__meta_kubernetes_node_name"]
    regex         = "(.+)"
    target_label  = "__metrics_path__"
    replacement   = "/api/v1/nodes/${1}/proxy/metrics/cadvisor"
  }
}

prometheus.scrape "kubelet_cadvisor" {
  targets           = discovery.relabel.kubelet_cadvisor.output
  job_name          = "kubelet-cadvisor"
  scheme            = "https"
  bearer_token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
  tls_config { ca_file = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" }
  scrape_interval = "60s"
  scrape_timeout  = "30s"
  forward_to = [otelcol.receiver.prometheus.bridge.receiver]
}
```

- `role = "node"`는 노드당 대상 1개를 발견한다. 대규모·DaemonSet 배치에서는 API 서버 부하를 피해 `discovery.kubelet`(노드 로컬)로 교체한다(공식 문서 권고) — 우리 대상은 kind 단일 노드라 `discovery.kubernetes`가 기본이다.
- **배포 산출물은 설정 파일 하나로 끝나지 않는다(HIGH-T3 상환)**. 파드가 실제로 뜨려면 4종이 필요하다: ① SA + ClusterRole(`nodes`·`nodes/proxy`·`nodes/metrics`에 `get`·`list`·`watch`) + Binding = `k8s-rbac.yaml`, ② **설정을 파드에 싣는 ConfigMap** = `k8s-configmap.yaml`, ③ **Deployment**(단일 레플리카 — `role="node"` 클러스터 와이드 수집이므로 DaemonSet이 아니다) = `k8s-deployment.yaml`, ④ `OTLP_ENDPOINT` 주입(Deployment `env`). 검증은 `--dry-run=server`(RBAC 미적용 — 인가 경로는 실적용 후에만 확인 가능) + 실적용 후 파드 Ready 두 단계다.
- **범위 한정(A7)**: kind `v2-p3`에서만 검증, `v2-p2`는 건드리지 않는다.
- **미확인**: 파드 재시작 수 등 클러스터 상태 계열은 cAdvisor가 아니라 `kube-state-metrics`가 소유한다 — 본 계획 범위 밖(D-4).

### 4.7 PVE 장비 텔레메트리 — 2안 비교와 택1

| 축 | ① `prometheus-pve-exporter` → `prometheus.scrape` | ② PVE 내장 metric server → `otelcol.receiver.influxdb` |
|---|---|---|
| **운영 부담** | 컨테이너 +1(태그 고정). PVE 측 설정 변경 없음 | 컨테이너 +0. 단 PVE 측 Metric Server 설정 추가 + Alloy 인바운드 리스너 개설 |
| **보안 경계** | **풀 모델 — 신규 인바운드 호스트 포트 0.** Alloy가 격리망에서 9221을 당긴다 | **푸시 모델** — PVE가 Alloy에 접속해야 하므로 Alloy 인바운드 포트(기본 8086)를 호스트에 발행해야 한다 → §4.1 경계 문언 2항과 정면 충돌 |
| **메트릭 커버리지** | 공식 문서 + 재검증 확인: `pve_up{id}`(`node/…`·`qemu/102`·`lxc/101`)·`pve_guest_info{id,name,node,type,tags}`·`pve_node_info{id,level,name,nodeid}`·`pve_storage_info{id,node,storage,plugintype,content}`·`pve_cpu_usage_ratio{id}`·`pve_memory_{size,usage}_bytes{id}`·`pve_disk_{size,usage}_bytes{id}` — **§6 매핑에 필요한 node/vmid를 직접 준다** | **미확인(A9)** |
| **신뢰** | 서드파티(커뮤니티) — 태그 고정 + 격리망 + RO 토큰으로 완화 | **벤더 네이티브 — 이 축만 ② 우위** |

**택1: ①.** **결정 제약은 (가) 신규 인바운드 호스트 포트 0, (나) semconv 매핑에 필요한 라벨(node·vmid)이 공식 문서로 확인된다** — 둘이다. ②는 신뢰 축에서만 우위이고 그 우위가 (가)를 깨고 (나)를 미확인으로 남기는 대가를 상쇄하지 못한다. ②는 "서드파티 프로세스 도입이 정책상 금지될 때"의 기록된 대안이며, 전환 시 PVE `influxdbproto=http` + Alloy `otelcol.receiver.influxdb`(둘 다 실재 확인)를 쓰되 **먼저 필드 집합을 실측해 A9를 해소**해야 한다.

**PVE 조각은 별도 파일로 켠다** — `pve.alloy.example`를 `pve.alloy`로 복사할 때만 활성화된다(Alloy 디렉터리 로드). 저장소 관례(`config.yaml.example`) 승계이고, compose `profiles: ["pve"]`와 짝을 이뤄 PVE를 선택 의존으로 만든다(H-2). **스크랩 주소도 이 파일에 있다** — 환경변수 `:?` 가드가 profile을 무시하고 비-PVE 경로를 깨뜨리기 때문이다(MEDIUM-T4 실측).

```alloy
// deploy/alloy/pve.alloy.example → pve.alloy 로 복사할 때만 활성화된다.
prometheus.scrape "pve" {
  targets = [{
    __address__   = "pve-exporter:9221",
    __metrics_path__ = "/pve",
    __param_target  = "PLACEHOLDER-REPLACE-WITH-PVE-HOST",   // 복사 후 반드시 교체
    __param_cluster = "1",
    __param_node    = "1",
    job = "proxmox-ve",
  }]
  scrape_interval = "60s"
  scrape_timeout  = "30s"
  forward_to = [otelcol.receiver.prometheus.bridge.receiver]
}
```

**보안**: exporter에는 RO 토큰만 준다(A8). `PVE_VERIFY_SSL=false`는 `pve-provisioning.md` §0의 사설망(RFC1918) 전제와 같은 조건에서만. 네트워크 격리는 §4.1-4.

### 4.8 런북 착지 — `docs/DEPLOY_DOCKER_COMPOSE.md` §7.5 신설

§7.3(내부망 DNS)과 같은 형식으로 **§7.5 "텔레메트리 수집(선택)"** 을 신설한다. 기존 절은 수정하지 않는다.

1. §4.1의 보안 경계 문언 6항 **그대로**.
2. `-f` 2개 사용법 — `up`·`ps`·`logs`·`down` 전부 동일하게 두 파일을 지정해야 함(§7.3 형식 승계).
3. `deploy/.env` 신규 항목 7종의 의미와 필수/선택 구분 + **PVE 스크랩 주소는 `pve.alloy` 파일에 적는다**는 안내.
4. 기동 절차(**모든 명령이 `-f` 2개와 `--env-file`을 갖는다** — 하나라도 빠지면 `no such service` 또는 `required variable … is missing a value`로 실패한다). **PVE 프로파일을 활성화했다면 기동 후 첫 스크랩 로그를 반드시 확인한다** — 스크랩 주소는 파일 플레이스홀더라 compose 기동 시점에 검증되지 않으므로, 교체 누락은 `alloy` 로그의 첫 스크랩 실패로만 드러난다:
   ```bash
   cd /path/to/ops-admin
   cp -n deploy/.env.example deploy/.env && chmod 600 deploy/.env
   mkdir -p deploy/grafana/provisioning/{dashboards,plugins}
   # PVE를 쓸 때만: cp deploy/alloy/pve.alloy.example deploy/alloy/pve.alloy
   #                그리고 pve.alloy의 PLACEHOLDER-REPLACE-WITH-PVE-HOST를 실제 주소로 교체
   docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env config --quiet
   docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env up -d
   docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env logs --tail=100 alloy
   ```
5. **§12 체크리스트에 3줄 추가**(기존 무수정): "Alloy·Mimir·pve-exporter에 호스트 포트 발행이 없다" / "Grafana는 루프백 바인드·익명 접근 차단이며 관리자 비밀번호를 교체했다" / "PVE exporter에 읽기 전용 토큰만 주입했고 `ops-admin-telemetry` 망에만 있다". **§13 제한 사항에 1줄**: "텔레메트리 Mimir는 단일 인스턴스 filesystem 구성이며 고가용성을 제공하지 않는다."

### 4.9 텔레메트리 리댁션 규칙 (보존 제약 #6·#7 시행 문언)

- **`ConnectionView.Material`은 어떤 텔레메트리 경로에도 실리지 않는다.** 현행 14종 라벨(`connection`·`provider`·`op`·`code`·`mode`·`operation`·`status`·`kind`·`purpose`·`backend`)에 자격 자료가 들어갈 자리가 없음이 골든(`metrics_test.go:118-218`)으로 증명된다. **새 라벨을 추가하는 어떤 변경도 이 골든을 통과해야 하며 자유 문자열 라벨은 도입하지 않는다.**
- **프로바이더 원시 에러는 restricted diagnostics 전용이다**(스펙 §18.1 `Audit record`). `provider_api_errors_total`의 `code`는 **코드값**이지 메시지가 아니다 — 어떤 대시보드·Alloy 규칙도 원시 에러 문자열을 라벨·주석으로 승격시키지 않는다.
- 설정 파일에 자격 값을 직접 쓰지 않는다 — 전부 `sys.env(…)`/`${…}`로 받고 유일한 보관처는 `deploy/.env`(권한 600, git 미커밋)다.

---

## 5. C 트랙 — 후속 과제로 분리 (D-1 기각)

**본 계획은 C 트랙을 확정하지 않는다.** 사용자 확정으로 v1 `monitor:query` 재사용이 기각됐고, `/api/v2` 전용 라우트 설계는 별도 후속 과제다.

- 기각 근거(질의 이력 전역 DELETE·감사 행 증폭·임의 PromQL·오류 본문 에코·권한면 교차)와 패널 14종·PromQL·"미상" 렌더 계약은 **`v2-phase1/otel-c-track-handoff.md`** 에 인수인계 자산으로 보존한다.
- 본 계획의 어떤 Phase·claim도 C 트랙을 포함하지 않으며 **스펙에 질의 계약을 박지 않는다**(r1 P6 삭제 — H-11 상환). `api/v2` 정합 검토와 후속 과제의 골든 재생성 의무는 §1.3.

---

## 6. semconv 매핑 — `v2-phase1/otel-semconv-mapping.md`

전수 표(어휘 21종)와 URN 좌변은 길이 계약(≤650줄)상 **별도 파일**에 있다. 그 파일이 P6 산출물의 정본이다. 여기에는 계획 차원의 결정만 남긴다.

1. **semconv v1.44.0 고정**(2026-08-04 릴리스). 버전을 올리면 표의 stability 마커를 재확인한다.
2. **발명 금지** — registry에 실재하는 속성만 매핑하고, 대응이 없으면 `opsadmin.*`로 적는다. `otel.`·`opentelemetry.` 및 예약 루트(`k8s.`·`host.`·`cloud.`·`container.`·`service.`·`network.`·`process.`)를 침범하지 않는다. **판정은 하드코딩 목록이 아니라 registry 원본 대조로 한다**(C-12).
3. **stability 실측**(registry.yaml 원본 파싱): **Stable `k8s.*`는 42종**이고 우리 표는 그 부분집합이다 — r1의 "10종이 전부"는 거짓이었다. `k8s.service.name`은 Development로 **실재**하고, `k8s.configmap.name`·`k8s.secret.name`은 부재. `host.*`는 전부 Development — 이에 의존하는 대시보드는 semconv 업그레이드로 깨질 수 있음을 §4.8 런북에 1줄 남긴다.
4. **속성이 질의 가능해지는 경로**: OTLP 리소스 속성은 Mimir에서 **`target_info` 시리즈의 라벨**로 착지하고 메트릭과는 **`on (job, instance)`** 로 조인한다. 스모크 실측에서 `job="ops-admin-control-plane"`·`instance="stub:80"`이 targets-map의 `job` 키로부터 그대로 착지함을 확인했다. `up`·`target_info`·`scrape_duration_seconds`·`scrape_samples_scraped`·`scrape_samples_post_metric_relabeling`·`scrape_series_added`는 **함께 생기는 합성 시리즈**이며 드리프트가 아니다.
5. **Grafana 딥링크 템플릿**: `{GRAFANA_BASE}/explore?left={"datasource":"ops-admin-mimir","queries":[{"expr":"<식>"}],"range":{"from":"now-6h","to":"now"}}`(URL 인코딩). `{GRAFANA_BASE}` = `OTEL_GRAFANA_BASE`(§4.2), 데이터소스 uid = `ops-admin-mimir`(§4.4). **`connection` 라벨값은 커넥션 UID이므로 UID→표시명 해석은 딥링크 생성 측이 한다** — Mimir에는 이름이 없다.

## 7. 확장 연동 — `otel-m18-ext` 과제와의 경계

`otel-m18-ext`(`v2-phase1/metrics-ext-plan.md`)가 착지하면 패밀리가 **14종 → 16종**, §18.2 표가 **18행**이 된다.

**신설물의 정확한 이름**(H-5 상환 — r1은 기각된 (a)안을 적었다):
- `inventory_sync_total{connection, mode, status}` counter — `status` 어휘는 **`succeeded`·`partial`·`failed`**. 모든 `RunSync` 호출이 정확히 한 시리즈를 증분하며, run 행을 만들지 못한 pre-flight 실패는 `failed`로 센다. **`metrics-ext-plan.md` §2.1.2가 "sync duration에 `status` 라벨 추가"(a)안을 명시 기각하고 이 별도 카운터 (b)안을 확정했다.**
- `inventory_sync_last_success_timestamp_seconds{connection}` gauge — 엔진 레인 기동 시 `inventory_sync_run` 원장에서 복원되므로 프로세스 재시작이 시리즈를 비우지 않는다.
- `provider_health`에 `provider` 라벨 추가.
- `inventory_sync_partial_total`은 **유지**된다(§18.2 M1 계약 행). `inventory_sync_duration_seconds`의 라벨도 `{connection, mode}` 그대로다.

| 확장 | 본 계획에서 바뀌는 것 |
|---|---|
| `provider_health` + `provider` | A 트랙 설정 무변경. 시계열 신원이 바뀌므로 **착지 순서 제약**이 생긴다(아래) |
| `inventory_sync_total` 신설 | A 트랙 설정 무변경. §6 표·§4.x 설정 어느 것도 패밀리 이름을 열거하지 않는다 |
| `inventory_sync_last_success_timestamp_seconds` 신설 | 동일 — 무변경 |

**주의**: 존재하지 않는 라벨 매처는 오류가 아니라 **빈 결과**다. 착지 확인 전에 `{status="partial"}`을 쓰면 알람이 조용히 침묵한다.

**경계 규칙(H-6 상환 — r1의 "겹치는 유일한 지점은 골든"은 거짓이었다)**:
- 두 계획은 **같은 파일 `docs/architecture/multi-infrastructure-control-plane-v2.md`를 수정한다.** 병행 계획은 §18.2 표(라벨 열 + 2행 삽입 + 후행 문단), 본 계획은 **§18.5 신설**. 절 번호가 겹치지 않으므로 충돌하지 않는다.
- **절 번호·좌표 배정 주체는 본 계획이다** — semconv 매핑은 **§18.5**로 고정한다(`### 18.4 AI tool path authorization`가 이미 점유). 병행 계획은 새 절을 만들지 않고 §18.2 표만 고친다.
- **본 계획 본문은 스펙을 행 번호로 인용하지 않는다**(절 제목 기준). 병행 계획의 표 삽입으로 행 좌표가 밀려도 본 계획의 인용이 어긋나지 않는다.
- `backend/internal/infra/metrics/metrics_test.go` 수정 권한은 **`otel-m18-ext`에만** 있다. 본 계획의 보존 제약 #4·claim C-11은 "**본 과제의 커밋이** 그 파일을 바꾸지 않는다"는 뜻이며, 병행 과제가 먼저 착지해 파일이 달라지는 것은 위반이 아니다 — claim 판정은 **본 과제 브랜치의 merge-base 대비 diff**로 한다.

**착지 순서(팀리드 확정)**: **`otel-m18-ext` P1(`provider_health`에 `provider` 라벨) → 본 계획 P2(Alloy 스크랩 개시).** 스크랩 개시 전에는 전환 비용이 0이지만(`deploy/alloy/` 미존재 실측), 개시 후에는 라벨 변경이 TSDB에 고아 시리즈를 남겨 코드 revert로 회수되지 않는다. **제약은 P1에만 걸린다 — 본 계획의 P1·P3~P6은 병행 가능하며 두 계획을 직렬화하지 않는다.**

---

## 8. Phase 분해 (각 ≤5파일, 독립 검증 단위)

의존: **유일한 실제 간선은 `P2 → P3`** 다(런북 §7.5가 P2의 오버레이를 문서화한다). **P1·P4·P5·P6은 서로도, P2/P3와도 의존이 없어 전부 병행 가능하다.** 적대검토가 지적한 "k8s Phase가 오버레이에 의존한다"는 허위 간선은 제거됐다 — k8s(P4)는 §4.6의 독립 OTLP 체인이라 P2 산출물을 참조하지 않는다.
**전 Phase가 문서·설정 파일만 만진다. 어떤 Phase도 `backend/`·`web/`을 수정하지 않는다.**

### P1 — assessment 개정
파일(1): `v2-phase1/otel-telemetry-assessment.md`(수정 — R-1~R-6). 검증: claim C-14.

### P2 — 수신 측(Mimir + Grafana) + 오버레이 골격
**착지 순서 제약**: 이 Phase가 스크랩을 개시한다. **`otel-m18-ext` P1(`provider_health`에 `provider` 라벨 추가)이 먼저 착지해야 한다** — 개시 후 라벨을 바꾸면 TSDB에 고아 시리즈가 남아 코드 revert로 회수되지 않는다. 제약은 그 P1 하나에만 걸리며 나머지 Phase는 병행 가능하다.
파일(5): `docker-compose.otel.yml`(신규 — §4.2 전문), `deploy/mimir/mimir.yaml`(신규 — §4.3), `deploy/grafana/provisioning/datasources/mimir.yaml`(신규 — §4.4), `deploy/grafana/provisioning/{dashboards,plugins}/.gitkeep`(신규 2파일을 1항목으로 계수), `deploy/.env.example`(수정 — 신규 7항목 추가만).
검증: claim C-1~C-4, C-6.

### P3 — Alloy 기본 설정 + 런북
파일(3): `deploy/alloy/config.alloy`(신규 — §4.5), `deploy/alloy/pve.alloy.example`(신규 — §4.7), `docs/DEPLOY_DOCKER_COMPOSE.md`(수정 — §7.5 신설 + §12 3줄 + §13 1줄).
검증: claim C-5, C-7, C-8, C-15.

### P4 — k8s 레시피 (배포 가능 집합)
파일(4): `deploy/alloy/config-k8s.alloy`(§4.6), `k8s-rbac.yaml`(SA·ClusterRole·Binding), `k8s-configmap.yaml`, `k8s-deployment.yaml`(단일 레플리카 + SA 참조 + `OTLP_ENDPOINT` env) — 전부 `deploy/alloy/` 아래 신규. 검증: claim C-5, C-9. **P2·P3와 의존 없음.**

### P5 — PVE 런북 보강
파일(1): `v2-phase1/pve-provisioning.md`(수정 — 신설 절 "텔레메트리용 RO 토큰 재사용": RO 토큰만 주입·OPS 토큰 금지·`PVE_VERIFY_SSL`는 §0 사설망 전제와 동일 조건·격리망 배치). 검증: claim C-10.

### P6 — semconv 매핑 스펙 착지
파일(2): `v2-phase1/otel-semconv-mapping.md`(수정 — 표 확정), `docs/architecture/multi-infrastructure-control-plane-v2.md`(수정 — **§18.5 "리소스 텔레메트리 semconv 매핑"** 신설: 표 21행 + 버전 고정 + `target_info` 조인 규칙 전재). 검증: claim C-12, C-13.

---

## 9. 위험 지점과 검증 방법 (실측 결과 포함)

- **R-1 (r1 최상위 — 해소) 메트릭 이름의 OTLP 왕복 보존 — 실측 확인됨.** 절차(2026-09-08): §18.2 골든 전문(`metrics_test.go:118-218`, 101행)을 nginx 스텁이 `/internal/metrics`로 서빙 → `grafana/alloy:v1.19.2`(`prometheus.scrape` → `otelcol.receiver.prometheus` → `otelcol.exporter.otlphttp`) → `grafana/mimir:3.2.0`(`/otlp/v1/metrics`) → `GET /prometheus/api/v1/label/__name__/values`.
  결과: **14종 패밀리 이름 전부 보존** — `provider_health`·`provider_api_latency_seconds_{bucket,sum,count}`·`provider_api_errors_total`·`provider_rate_limit_total`·`inventory_sync_duration_seconds_{bucket,sum,count}`·`inventory_sync_resource_changes_total`·`inventory_sync_partial_total`·`provider_task_duration_seconds_{bucket,sum,count}`·`provider_task_failures_total`·`provider_task_retries_total`·`worker_queue_depth`·`worker_lease_expired_total`·`resource_stale_total`·`secret_access_total`. 라벨 보존도 확인(`provider_health{connection="conn-aliyun",instance="stub:80",job="ops-admin-control-plane"}`). 합성 시리즈 `up`·`target_info`·`scrape_*` 4종 동반.
  **이 결과는 위 두 버전에 한정된다** — 버전을 올리면 claim C-8로 재확인한다.
- **R-2 (해소) PVE 선택 의존.** `profiles: ["pve"]` + `${VAR:-}` + 스크랩 주소의 파일화. 실측: PVE 환경변수 없이 `config --quiet` 종료 0, `config --services` = `alloy api grafana mimir mysql web`, `--profile pve` 추가 시 `pve-exporter` 포함. **`:?` 가드는 profile 제외 서비스에도 적용돼 비-PVE 경로를 깨뜨림을 실측 확인**했으므로 fail-fast는 `pve.alloy` 플레이스홀더로 옮겼다(MEDIUM-T4).
- **R-3 노출 확대.** 오버레이가 `api`에 `ports:`를 추가하거나 pve-exporter를 `ops-admin` 망에 두면 경계가 무너진다. 검증: C-2·C-3·C-4.
- **R-4 `validate`가 잡지 못하는 런타임 거부.** 실측: `scrape_interval=5s` + 기본 `scrape_timeout=10s` 구성이 **`validate` EXIT=0을 받고도 `alloy run`에서 `scrape_timeout (10s) greater than scrape_interval (5s)`로 기동 실패**했다. 따라서 C-5는 문법 검증에 그치고, **런타임 게이트를 C-15가 담당**한다. 본 계획의 설정은 30s/10s·60s/30s로 이 제약을 만족한다.
- **R-5 Mimir 데이터 소실** — 단일 인스턴스 filesystem. 완화: 명명 볼륨 + 런북 §13 1줄. 소실은 관측 데이터 한정, ops-admin 원본과 무관.
- **R-6 Grafana 노출** — 루프백 + 익명·가입 차단 + 관리자 비번 필수(`:?`). 검증: C-6. **R-7 PVE 자격 오배치** — OPS 토큰이 exporter에 들어가면 텔레메트리 컨테이너가 게스트 변경 권한을 갖는다. 완화: P5 금지 문언 + `.env.example` 주석. 검증: C-10.
- **R-12 distroless 이미지가 진단·헬스체크를 무력화한다.** 실측: `grafana/mimir:3.2.0`은 `/bin/mimir` 단독(셸 없음) → 어떤 `CMD-SHELL` healthcheck도 영구 unhealthy가 되고 `exec`도 불가. `grafana/alloy:v1.19.2`는 `wget`·`curl`·`busybox`·`nc`·`python3` 전무. 완화: Mimir healthcheck 제거 + `depends_on: [mimir]`, 진단은 `logs`·`bash /dev/tcp`·일회용 curl 컨테이너(§4.1 문언2·A3). 검증: C-3·C-15.
- **R-13 스크랩 성공 ≠ 적재 성공.** 수신처 미해석·거부는 **export 오류이지 스크랩 오류가 아니다** — Alloy는 스크랩을 계속 성공시키며 적재를 100% 잃을 수 있다. 기동 판정(C-15)으로는 부족하고 **적재를 직접 판정하는 C-8**이 별도로 필요하다.
- **R-8 kind 픽스처 오염** — P4 명령이 `--context kind-v2-p3`만 쓴다(C-9). **R-9 semconv 발명** — C-12. **R-10 태그 드리프트** — `latest` 금지, C-2.
- **R-11 병행 과제 착지 경합.** §7 경계 규칙 + 착지 순서. 검증: C-11(merge-base diff).

---

## 10. 롤백 가능성 판단

**높음 — 전 산출물이 추가형 문서·설정이고 ops-admin 런타임 코드 경로가 없다.** r1 대비: 스펙 조항 신설이 **§18.5 하나뿐**이고(질의 계약 절 삭제 — H-11 상환) 아직 어떤 문서도 인용하지 않으므로 단일 revert로 고아가 생기지 않는다.

| 산출물 | 롤백 |
|---|---|
| `docker-compose.otel.yml`·`deploy/{alloy,mimir,grafana}/*` | 파일 삭제 + `-f` 인자 제거. 오버레이를 로드하지 않으면 기존 스택은 **완전히 동일**하게 뜬다 |
| `deploy/.env.example` 추가 항목 | 미사용 변수 — 제거만. 기존 스택은 참조하지 않는다. `deploy/alloy/k8s-*.yaml`도 미적용 상태면 삭제로 끝난다 |
| 문서 수정(assessment·DEPLOY·pve-provisioning) | `git revert` 단일 커밋 |
| 스펙 §18.5 신설(P6) | `git revert` 단일 커밋. **§18.4는 건드리지 않으므로 기존 인바운드 참조 2건(`v2.md` 리스크 레지스터·`task3-plan.md:249`)은 영향 없다** |
| Mimir에 적재된 시리즈 | ops-admin 밖 데이터. 볼륨 삭제로 제거 가능하나, **`provider_health` 라벨 변경 후에는 고아 시리즈가 남는다** → 그래서 착지 순서 제약(§7·P2)이 있다 |

스키마 변경 0 · 프로덕션 제어흐름 변경 0 · `go.mod` 변경 0 → 데이터 롤백 불필요. 각 Phase 독립 커밋 시 P3→P2 순으로 revert한다(P3가 P2의 오버레이를 전제).

---

## 11. 검증 요구 (claims)

**전제 — 모든 명령은 저장소 루트에서 실행하며, 아래 준비를 먼저 한다**(복사해 그대로 실행 가능):
```bash
cd /mnt/d/DEV/acc0mplish/ops-admin
cp -n deploy/.env.example deploy/.env && chmod 600 deploy/.env
# deploy/.env에 최소 OTEL_GRAFANA_ADMIN_PASSWORD 및 기존 필수 항목을 채운 뒤 진행
export COMPOSE="docker compose -f docker-compose.yml -f docker-compose.otel.yml --env-file deploy/.env"
# Alloy·Mimir 이미지에는 HTTP 클라이언트가 없다(A3). 조회는 일회용 컨테이너로 한다.
CURL() { docker run --rm --network ops-admin-telemetry curlimages/curl:8.11.1 -s "$@"; }
```
`[스택]` = 컨테이너 기동이 필요한 claim. 그 외는 파일만으로 판정 가능.

1. **C-1** 오버레이가 기존 3서비스를 재정의하지 않는다 —
   `grep -cE '^  (mysql|api|web):' docker-compose.otel.yml` → `0`
2. **C-2** 전 이미지가 태그 고정이다 —
   `grep -cE '^[[:space:]]*image:.*:latest' docker-compose.otel.yml` → `0` 이고
   `grep -oE '^[[:space:]]*image:.*' docker-compose.otel.yml | grep -cvE ':[0-9]+\.[0-9]+\.[0-9]+$'` → `0`
3. **C-3** 호스트 포트는 Grafana 루프백 하나뿐이고 pve-exporter는 격리망에만 있다 —
   `grep -cE '^[[:space:]]*ports:' docker-compose.otel.yml` → `1`, `grep -c '127.0.0.1:3000:3000' docker-compose.otel.yml` → `1`,
   `$COMPOSE --profile pve config | python3 -c "import sys,yaml;d=yaml.safe_load(sys.stdin);print(sorted(d['services']['pve-exporter']['networks']))"` → `['ops-admin-telemetry']`,
   같은 방식으로 `alloy` → `['ops-admin', 'ops-admin-telemetry']`.
   **Mimir에 healthcheck가 없다**(distroless — R-12): `grep -A6 '^  mimir:' docker-compose.otel.yml | grep -c healthcheck` → `0`
4. **C-4** PVE 없이도 조립이 성공하고 pve-exporter가 렌더되지 않는다 —
   `$COMPOSE config --quiet; echo $?` → `0` 이고
   `$COMPOSE config --services | sort | tr '\n' ' '` → `alloy api grafana mimir mysql web `
   그리고 `$COMPOSE --profile pve config --services | grep -c pve-exporter` → `1`.
   기존 파일 무변경: `git diff --name-only -- docker-compose.yml docker-compose.dns.yml` → 빈 출력
5. **C-5** Alloy 설정이 **문법** 검증을 통과한다(문법만 — 런타임 게이트는 C-15) —
   `cp -n deploy/alloy/pve.alloy.example deploy/alloy/pve.alloy` 후
   `docker run --rm -v "$PWD/deploy/alloy:/cfg:ro" grafana/alloy:v1.19.2 validate /cfg; echo $?` → `0`
   `mkdir -p /tmp/k8s-cfg && cp deploy/alloy/config-k8s.alloy /tmp/k8s-cfg/ && docker run --rm -v /tmp/k8s-cfg:/cfg:ro grafana/alloy:v1.19.2 validate /cfg; echo $?` → `0`
   모든 `prometheus.scrape` 블록에서 `scrape_timeout ≤ scrape_interval` 육안 확인(R-4),
   `grep -c -- '--disable-reporting' docker-compose.otel.yml` → `1`(MEDIUM-T7),
   `grep -c 'PLACEHOLDER-REPLACE-WITH-PVE-HOST' deploy/alloy/pve.alloy.example` → `1`, 그리고 `git ls-files deploy/alloy/pve.alloy` → 빈 출력(실주소는 커밋되지 않는다)
6. **C-6** Grafana 노출 정책이 설정과 문서 양쪽에 있다 —
   `grep -c 'GF_AUTH_ANONYMOUS_ENABLED: "false"' docker-compose.otel.yml` → `1`,
   `grep -c 'GF_USERS_ALLOW_SIGN_UP: "false"' docker-compose.otel.yml` → `1`,
   `grep -c 'OTEL_GRAFANA_ADMIN_PASSWORD:?' docker-compose.otel.yml` → `1`,
   `grep -c 'multitenancy_enabled: false' deploy/mimir/mimir.yaml` → `1`,
   `grep -c 'X-Scope-OrgID' deploy/grafana/provisioning/datasources/mimir.yaml` → `1`(주석 형태의 탈출구),
   `docs/DEPLOY_DOCKER_COMPOSE.md`에 §4.1 경계 문언 6항이 전부 존재(grep 6건)
7. **C-7** `deploy/.env.example`에 신규 7항목이 있고 `OTEL_GRAFANA_BASE`가 정의됐다 —
   `for v in OTEL_GRAFANA_ADMIN_PASSWORD OTEL_OTLP_ENDPOINT OTEL_GRAFANA_BASE OTEL_PVE_USER OTEL_PVE_TOKEN_NAME OTEL_PVE_TOKEN_VALUE OTEL_PVE_VERIFY_SSL; do grep -q "^#\?$v" deploy/.env.example || echo "MISSING $v"; done` → 빈 출력.
   `grep -c OTEL_PVE_TARGET deploy/.env.example` → `0`(스크랩 주소는 `pve.alloy`에 있다 — MEDIUM-T4)
8. **C-8** `[스택]` OTLP 왕복 후 14종 이름이 보존된다(§9 R-1 재현) —
   `$COMPOSE up -d && sleep 120` 후
   `CURL 'http://mimir:9009/prometheus/api/v1/label/__name__/values'` 결과에
   `provider_health`·`provider_api_latency_seconds_bucket`·`provider_api_errors_total`·`provider_rate_limit_total`·`inventory_sync_duration_seconds_bucket`·`inventory_sync_resource_changes_total`·`inventory_sync_partial_total`·`provider_task_duration_seconds_bucket`·`provider_task_failures_total`·`provider_task_retries_total`·`worker_queue_depth`·`worker_lease_expired_total`·`resource_stale_total`·`secret_access_total` 14종이 **전부** 포함.
   **전제(H-4)**: 커넥션 ≥1건 등록 + health sweep 1회 이상 경과. 그 전에는 패밀리가 정상적으로 생략되므로 이 claim은 미충족이 아니라 **미실행**으로 기록한다. 결과는 `v2-phase1/otel-series-names-<date>.txt`로 저장한다
9. **C-9** k8s RBAC 매니페스트가 서버 검증을 통과한다 —
   `kubectl --context kind-v2-p3 apply --dry-run=server -f deploy/alloy/k8s-rbac.yaml; echo $?` → `0`.
   `grep -c 'nodes/proxy' deploy/alloy/k8s-rbac.yaml` → `1` 이상. **`kind-v2-p2` 컨텍스트는 어떤 명령에도 등장하지 않는다**
10. **C-10** PVE 자격 경계가 문서화됐다 —
    `grep -c 'OPS 토큰' v2-phase1/pve-provisioning.md` → `1` 이상(금지 문언),
    `grep -c 'PVEAuditor' v2-phase1/pve-provisioning.md` → `1` 이상,
    `grep -c 'ops-admin-telemetry' v2-phase1/pve-provisioning.md` → `1` 이상
11. **C-11** 본 과제가 코드·골든을 바꾸지 않았다 —
    **작업은 main이 아닌 전용 브랜치에서 수행한다**(r1 실패 원인: main 위에서는 `main...HEAD`가 공집합이라 무조건 통과).
    `git rev-parse --abbrev-ref HEAD` → `main`이 **아님**을 먼저 확인한 뒤
    `git diff --name-only main...HEAD | grep -E '^(backend/|web/|go\.(mod|sum)|docs/security/)'` → 빈 출력.
    3점 형식은 merge-base 기준이므로 main이 전진해도 유효하다
12. **C-12** 매핑 표가 어휘 21종을 전수 덮고 `k8s.*` 속성을 발명하지 않았다 —
    `sed -n '/M1ResourceKinds = /,/^}/p;/Phase2ResourceKindExtensions = /,/^}/p' backend/internal/infra/contract/resource_kind.go | grep -oE '"[a-z_]+\.[a-z_]+"' | tr -d '"' | sort -u | while read k; do grep -q "\`$k\`" v2-phase1/otel-semconv-mapping.md || echo "MISSING $k"; done` → 빈 출력.
    **`k8s.*` 판정은 하드코딩 목록이 아니라 registry 원본 대조로 한다**(r1의 "실재 10종" 기준은 거짓 — Stable `k8s.*`는 42종):
    `curl -sL https://raw.githubusercontent.com/open-telemetry/semantic-conventions/v1.44.0/model/k8s/registry.yaml -o /tmp/k8s-reg.yaml` 후
    `grep -oE '\`k8s\.[a-z_.]+\`' v2-phase1/otel-semconv-mapping.md | tr -d '\`' | sort -u | while read a; do grep -q "id: $a\$" /tmp/k8s-reg.yaml || echo "INVENTED $a"; done` → **`k8s.configmap.name`·`k8s.secret.name` 2건만** 출력(둘은 registry 부재를 명시하려고 적은 것). 그 2건은 각 등장 행에 `미정의` 또는 `대응 없음`이 함께 있어야 한다 — `grep -n 'k8s\.configmap\.name\|k8s\.secret\.name' v2-phase1/otel-semconv-mapping.md` 육안 확인.
    그 외 이름이 출력되면 **발명**이며 규칙1 위반이다
13. **C-13** 스펙 착지가 **§18.5 하나**이고 기존 §18.4를 건드리지 않았다 —
    `grep -c '^### 18\.4 AI tool path authorization' docs/architecture/multi-infrastructure-control-plane-v2.md` → `1`,
    `grep -c '^### 18\.5 ' docs/architecture/multi-infrastructure-control-plane-v2.md` → `1`,
    `grep -c '^### 18\.6 ' docs/architecture/multi-infrastructure-control-plane-v2.md` → `0`,
    `git diff main...HEAD -- docs/architecture/multi-infrastructure-control-plane-v2.md | grep -c '^-.*18\.4'` → `0`
14. **C-14** assessment 개정 6건이 착지했다 — R-1(`§5`에 "확정 — 사용자 2026-09-08") · R-2(`## 9` 제목에 "해소 기록") · R-3(질문3에 "k8s + PVE") · R-4(§8-4에 "Alloy" 정정 + 공식 URL) · R-5(§6에 "v1.44.0") · R-6(§3.1에 "Mimir" + "9009") 각각 grep 1건 이상
15. **C-15** `[스택]` **런타임** 기동이 성공한다(C-5가 못 잡는 층) —
    `$COMPOSE up -d && sleep 90` 후
    `$COMPOSE logs alloy | grep -c 'level=error'` → `0`,
    `CURL -o /dev/null -w '%{http_code}\n' http://mimir:9009/ready` → `200`,
    `$COMPOSE exec -T alloy bash -c 'exec 3<>/dev/tcp/api/8082; printf "GET /internal/metrics HTTP/1.0\r\n\r\n" >&3; cat <&3' | tail -n +8 | head -1` → `# TYPE provider_health gauge` **또는 빈 출력**(빈 출력은 커넥션 미등록 상태의 정상 결과 — H-4).
    **C-15가 녹색이어도 적재 성공은 증명되지 않는다**(R-13 — export 오류는 스크랩 오류가 아니다). 적재 판정은 C-8이 단독으로 담당한다

---

## 12. 보존 제약 (③구현 프롬프트에 verbatim 복사)

1. **`backend/` 코드 변경 0** — A 트랙은 설정·문서만. C 트랙은 신규 코드가 생기지만 그건 **별도 후속 과제로 분리**하고 본 계획은 인터페이스 계약까지만 확정한다(범위 폭주 방지).
2. `metrics` 패키지 순도(`metrics.go:8-10` 표준 라이브러리만) 불변.
3. §10.2 "No new runtime dependencies in M1"(스펙 :976) 불변 — A 트랙이 이 조항을 지키는 것이 채택 근거다.
4. `/internal/metrics` 렌더 골든(`backend/internal/infra/metrics/metrics_test.go:118-218`) 무변경.
5. 라우트 골든(`docs/security/route-inventory.txt`) 무변경.
6. `ConnectionView.Material`은 평문이고 "로그·직렬화 금지" 계약이다(`backend/internal/infra/contract/adapter.go` Material 주석, 보존 제약 #7) — 어떤 텔레메트리 경로도 이 값을 실어 나르지 않는다.
7. 프로바이더 원시 에러는 restricted diagnostics 전용(§18.1) — OTLP/대시보드로 새어나가지 않게 하는 규칙 명시.
8. 기존 docker-compose 스택 기동 동작 무변경(추가만, 기존 서비스 정의 수정 금지).

> **r2 주석(제약 자체는 위 문언 그대로 유지)**: #3·#4의 행 좌표(`:976`, `:118-218`)는 원문 보존을 위해 그대로 두되, **판정은 절 제목·함수명 기준**으로 한다(§10.2 `Operation definition` 문단 / `TestRenderFullFamilyGolden`). #4·#5는 **본 과제 브랜치의 merge-base 대비 diff**로 판정한다 — 병행 과제 `otel-m18-ext`가 골든을 바꾸는 것은 본 제약의 위반이 아니다(§7). #5는 **본 계획에만** 구속력이 있으며 C 트랙 후속 과제는 골든 재생성을 자기 범위에 포함한다(§1.3).

---

## 13. 미해결 · 사용자 확인 필요

| # | 항목 | 처리 |
|---|---|---|
| ~~D-1~~·~~D-2~~·~~D-3~~ | C 트랙 권한면 / LGTM 형상 / 멀티테넌시 | **전부 해소.** D-1 기각→`otel-c-track-handoff.md` 분리 · D-2 별도 Mimir→§4.3 착지 · D-3 재판정(A6, 강제 아닌 배포 자세) |
| **D-4** | PVE 내장 metric server 필드 집합 (A9) | 미확인 — 경로 ②를 대안으로 보류한 근거. ② 전환 시 실측 선행 필요 |
| **D-5** | k8s 검증 대상 (A7) | kind `v2-p3` 기준으로만 claim 작성. 프로덕션 형상 클러스터 검증이 필요한지 |
| **D-6** | kube-state-metrics 배포 | 범위 밖. 파드 재시작 등 클러스터 상태 계열이 필요하면 후속 과제 |
| **D-7** | Mimir 보존 기간 | `compactor_blocks_retention_period: 30d`를 기본값으로 제안. 운영 요구에 따라 조정 |
