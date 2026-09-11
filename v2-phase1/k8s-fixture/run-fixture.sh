#!/usr/bin/env bash
# v2 Phase 3 Slice A fixture 실행 스크립트 (계획 N19 — L1: 문서가 아닌 명령으로 재현)
#
# *** DEPRECATED — V2 Phase 6 I-a (phase6-plan §J5 S7) ***
# 이 스크립트의 등록 흐름은 더 이상 작동하지 않는다: §5.4 v1 백필이 I-a에서
# 제거됐으므로 3단(k8s_cluster upsert)→4단(sync)의 체인 형성이 일어나지
# 않는다. 등록은 두 경로로 대체한다 —
#   • 실등록      : backend `register-k8s` CLI (kubeconfig 경로 수령·즉시 봉인)
#   • e2e 재현    : backend/e2e/slicea harness (register-shaped V2 체인 직접
#                   삽입 — inventory·operations 바인딩 포함, J12(1))
# 시드 적용(2단)만 필요하면 kubectl apply -f seed.yaml을 직접 실행한다.
#
# 절차 (계획 §5 Phase F / N18 README와 동일 — I-a 이전 기록 보존):
#   1. kind 2호기 v2-p3 클러스터 사전 확인 (생성은 하지 않는다 — 보존 제약 #11)
#   2. 시드 적용        — seed.yaml 멱등 apply + rollout 대기
#   3. v1 등록          — k8s_cluster upsert (kubeconfig 평문·updated_at touch)
#   4. 백필 + sync 재실행 — ops-admin sync-inventory 서브커맨드 (CLI가 전부 수행)
#   5. 검증            — operations 바인딩·restart-target 리소스 실측
#
# 환경변수 (전부 기본값 있음):
#   SLICEA_CONTEXT         kubeconfig 컨텍스트      (기본 kind-v2-p3)
#   SLICEA_CLUSTER_NAME    k8s_cluster 등록 이름     (기본 kind-v2-p3)
#   SLICEA_MYSQL_CONTAINER dev MySQL 컨테이너       (기본 ops-admin-mysql-dev)
#   SLICEA_DB              등록 대상 스키마          (기본 ops_admin — dev 공유 스키마.
#                          E2E 전용 스키마 ops_admin_p3e2e는 E2E(TestMain)가 자체 생성·해제 — VK-7)
#   OPS_ADMIN_CONFIG       백필·sync에 쓸 config.yaml (기본 backend/config.yaml)
#
# kubeconfig는 v1(평문)·V2(SecretRef 봉인) 양 레인이 읽는 단일 소스다:
# backfill이 k8s_cluster.kube_config를 읽어 봉인 복사본을 만든다 (§4.4 — v1은 평문 유지).
set -euo pipefail

CONTEXT="${SLICEA_CONTEXT:-kind-v2-p3}"
CLUSTER_NAME="${SLICEA_CLUSTER_NAME:-kind-v2-p3}"
MYSQL_CONTAINER="${SLICEA_MYSQL_CONTAINER:-ops-admin-mysql-dev}"
DB_NAME="${SLICEA_DB:-ops_admin}"
CONFIG="${OPS_ADMIN_CONFIG:-backend/config.yaml}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN="$ROOT/.tmp/ops-admin-fixture"
export MYSQL_PWD="${SLICEA_MYSQL_PASSWORD:-123456}"
MYSQL_USER="${SLICEA_MYSQL_USER:-root}"

log() { printf '[run-fixture] %s\n' "$*"; }
die() { printf '[run-fixture] FAIL: %s\n' "$*" >&2; exit 1; }

mysql_exec() { docker exec -i -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" "$DB_NAME"; }

# DEPRECATED 가드 — 백필 제거(I-a)로 3→4단 체인 형성이 불가하다. 혼란을
# 줄 유령 실행보다 조기 차단이 낫다(본체 최소 변경 — 리뷰 M3).
die "DEPRECATED (V2 Phase 6 I-a): v1 등록→백필 흐름은 제거됐다 — backend 'register-k8s' CLI 또는 e2e slicea harness로 대체한다"

# --- 1. 사전 확인 -------------------------------------------------------------
command -v kubectl >/dev/null || die "kubectl not found"
kubectl --context "$CONTEXT" get ns v3-seed >/dev/null 2>&1 \
  || die "context $CONTEXT has no v3-seed namespace — create the v2-p3 cluster first (see README)"
[ -f "$CONFIG" ] || die "config not found: $CONFIG (set OPS_ADMIN_CONFIG)"
docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -e 'SELECT 1' >/dev/null 2>&1 \
  || die "dev MySQL container $MYSQL_CONTAINER not reachable"

# --- 2. 시드 적용 (멱등) ------------------------------------------------------
log "applying seed.yaml on $CONTEXT"
kubectl --context "$CONTEXT" apply -f "$SCRIPT_DIR/seed.yaml"
log "waiting for restart-target rollout (minReadySeconds=10 adds ~10s)"
kubectl --context "$CONTEXT" -n v3-seed rollout status deploy/restart-target --timeout=180s

# --- 3. v1 등록 (k8s_cluster upsert — kubeconfig 평문·updated_at touch) ------
# current-context가 이 클러스터만 담도록 minify·flatten 한다 (k8s 클라이언트 파서는
# current-context 우선 — 멀티 컨텍스트 kubeconfig의 우발적 타겟 오염 방지, R13).
KUBECONFIG_YAML="$(kubectl config view --minify --flatten --context "$CONTEXT")"
[ -n "$KUBECONFIG_YAML" ] || die "minified kubeconfig is empty"
API_SERVER="$(kubectl config view --minify --flatten --context "$CONTEXT" -o jsonpath='{.clusters[0].cluster.server}')"
K8S_VERSION="$(kubectl --context "$CONTEXT" version -o json 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)["serverVersion"]["gitVersion"])')"
NODE_COUNT="$(kubectl --context "$CONTEXT" get nodes -o name | wc -l | tr -d ' ')"

# SQL 리터럴 안전화 — kubeconfig는 base64로 운반해 FROM_BASE64로 복원한다.
KUBECONFIG_B64="$(printf '%s' "$KUBECONFIG_YAML" | base64 -w0)"
CLUSTER_ID="$(docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -N -s -e \
  "SELECT id FROM $DB_NAME.k8s_cluster WHERE name='$CLUSTER_NAME'")"
if [ -n "$CLUSTER_ID" ]; then
  log "updating existing k8s_cluster id=$CLUSTER_ID (updated_at touch → backfill incremental 재처리)"
  docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" "$DB_NAME" -e \
    "UPDATE k8s_cluster SET kube_config=CONVERT(FROM_BASE64('$KUBECONFIG_B64') USING utf8mb4),
       api_server='$API_SERVER', version='$K8S_VERSION', node_count=$NODE_COUNT,
       status='running', env='dev', updated_at=NOW(3)
     WHERE id=$CLUSTER_ID"
else
  log "inserting k8s_cluster row for $CLUSTER_NAME"
  docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" "$DB_NAME" -e \
    "INSERT INTO k8s_cluster (name, status, api_server, version, node_count, env, tags,
       connection_mode, description, kube_config, created_at, updated_at)
     VALUES ('$CLUSTER_NAME', 'running', '$API_SERVER', '$K8S_VERSION', $NODE_COUNT, 'dev', '[]',
       'direct', 'v2 Phase 3 Slice A fixture (kind 2호기)',
       CONVERT(FROM_BASE64('$KUBECONFIG_B64') USING utf8mb4), NOW(3), NOW(3))"
  CLUSTER_ID="$(docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -N -s -e \
    "SELECT id FROM $DB_NAME.k8s_cluster WHERE name='$CLUSTER_NAME'")"
fi
[ -n "$CLUSTER_ID" ] || die "k8s_cluster upsert failed"

# connection UID는 백필의 결정적 파생값 (inventory/sync.go SourceKeyUID).
CONNECTION_UID="$(printf 'backfill|k8s_cluster|%s' "$CLUSTER_ID" | sha256sum | cut -c1-32)"
log "cluster id=$CLUSTER_ID connection_uid=$CONNECTION_UID"

# --- 4. 백필 + sync 재실행 ---------------------------------------------------
log "building ops-admin binary"
(umask 077 && mkdir -p "$ROOT/.tmp" && cd "$ROOT/backend" && go build -o "$BIN" .)

log "running sync-inventory (backfill + one sync + report artifact)"
(cd "$ROOT/backend" && "$BIN" sync-inventory --config "$CONFIG" --connection "$CONNECTION_UID" --data "$ROOT/.tmp/sync-reports")

# --- 5. 검증 ------------------------------------------------------------------
log "verifying credential bindings (expect inventory + operations on one secret_ref)"
docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -t "$DB_NAME" -e \
  "SELECT b.purpose, COUNT(*) AS bindings FROM provider_credential_binding b
     JOIN provider_connection c ON c.id = b.provider_connection_id
    WHERE c.uid = '$CONNECTION_UID' GROUP BY b.purpose"
OPERATIONS_BINDINGS="$(docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -N -s "$DB_NAME" -e \
  "SELECT COUNT(*) FROM provider_credential_binding b
     JOIN provider_connection c ON c.id = b.provider_connection_id
    WHERE c.uid = '$CONNECTION_UID' AND b.purpose = 'operations'")"
[ "$OPERATIONS_BINDINGS" -ge 1 ] || die "operations purpose binding missing (J12(1)/M18)"

log "verifying restart-target resource in infra_resource"
TARGET_ROWS="$(docker exec -e MYSQL_PWD "$MYSQL_CONTAINER" mysql -u"$MYSQL_USER" -N -s "$DB_NAME" -e \
  "SELECT COUNT(*) FROM infra_resource WHERE external_urn LIKE '%v3-seed/deployment/restart-target' AND deleted_at IS NULL")"
[ "$TARGET_ROWS" -ge 1 ] || die "restart-target resource not found after sync"

log "fixture ready — cluster=$CONTEXT db=$DB_NAME connection_uid=$CONNECTION_UID"
