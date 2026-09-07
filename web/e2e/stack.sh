#!/usr/bin/env bash
# Playwright UI-flow stack bootstrap (plan N14/J11 — "스택 기동을 잡이 소유").
#
# Brings up the same stack the N16 Go trace exercises, but keeps it alive for
# a real browser session:
#   dedicated schema (VK-7) → backend child (migrations+seed+engine lane,
#   OPS_ADMIN_ENGINE_* overrides, J10) → v1 k8s_cluster registration + sync-
#   inventory (the CLI subcommand, never a parallel reimplementation) → vite
#   dev server in the foreground (Playwright's webServer URL).
#
# Modes:
#   prepare   schema + run dir + config + backend build (no processes)
#   register  k8s_cluster row + sync-inventory (needs one backend boot first)
#   serve     prepare + seed apply + backend boot + register + vite (webServer)
#   serve-b   Slice B (plan N17 — PR 30): prepare (no kind cluster needed) +
#             backend boot + seed_cloud (mock aliyun account → V2 rows) +
#             interim-marked artifact + vite (webServer)
#   serve-c   Slice C (plan N9 — PR 31c): prepare (no kind cluster needed) +
#             backend boot + seed_pve (the proxmox normalizer renders the V2
#             rows through TestSliceCSeedWriter — no hand-written JSON, R10) +
#             vite (webServer)
#   compare   J11's §15 segment: boot + register + sync + compare-inventory CLI
#   down      stop the backend, drop the schema (unless STACK_KEEP=1)
#
# Environment (all defaulted; same names as the N16 harness where shared):
#   SLICEA_CONTEXT kind context        (default kind-v2-p3 — v2-p3 2호기 ONLY)
#   E2E_SCHEMA     dedicated schema    (default ops_admin_p3ui)
#   E2E_ADMIN_PASSWORD initial admin password (default slicea-e2e-admin)
#   E2E_BACKEND_PORT backend port       (default 8082 — vite.config.js proxy target)
#   E2E_WEB_PORT   vite dev port       (default 8080)
#   MYSQL_HOST/PORT/USER/PASSWORD  DB coordinates (defaults 127.0.0.1/3306/root/123456)
#   MYSQL_CONTAINER docker container carrying the mysql client when no local
#                  client binary exists (default ops-admin-mysql-dev; in CI the
#                  service container is discovered via its image)
#   STACK_KEEP=1   keep the schema and logs on exit (dirty runbook — VK-12)
set -euo pipefail

E2E_SCHEMA="${E2E_SCHEMA:-ops_admin_p3ui}"
SLICEA_CONTEXT="${SLICEA_CONTEXT:-kind-v2-p3}"
CLUSTER_NAME="${CLUSTER_NAME:-kind-v2-p3}"
E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-slicea-e2e-admin}"
# 18082: a dedicated port so the lane never contends with a developer's own
# backend on the vite default 8082 (the proxy target rides E2E_API_TARGET).
BACKEND_PORT="${E2E_BACKEND_PORT:-18082}"
# 18080: the lane's own web port — the developer's 8080 dev server must not
# contend with the webServer (strictPort fails the lane instead of stealing).
WEB_PORT="${E2E_WEB_PORT:-18080}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-ops-admin-mysql-dev}"
MASTER_KEYS="${SLICEA_MASTER_KEYS:-k20260906:slicea-e2e-master-key-material-fixed-48-bytes!!}"

STACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$STACK_DIR/../.." && pwd)"
RUN_DIR="$STACK_DIR/.stack"
STATE="$RUN_DIR/state.env"
CONFIG="$RUN_DIR/config.yaml"
BIN="$RUN_DIR/ops-admin"
LOG="$RUN_DIR/backend.log"
SEED="$REPO_ROOT/v2-phase1/k8s-fixture/seed.yaml"

log() { printf '[e2e-stack] %s\n' "$*"; }
die() { printf '[e2e-stack] FAIL: %s\n' "$*" >&2; exit 1; }

# --- mysql client: local binary, else docker exec into the container ---------
mysql_exec() {
  if command -v mysql >/dev/null 2>&1; then
    MYSQL_PWD="$MYSQL_PASSWORD" mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" "$@"
  else
    local container="$MYSQL_CONTAINER"
    if ! docker inspect "$container" >/dev/null 2>&1; then
      container="$(docker ps --filter ancestor=mysql:8.0 --format '{{.ID}}' | head -1)"
      [ -n "$container" ] || die "no mysql client binary and no mysql:8.0 container found"
    fi
    docker exec -i -e MYSQL_PWD="$MYSQL_PASSWORD" "$container" mysql -u"$MYSQL_USER" "$@"
  fi
}

# --- mode: prepare ------------------------------------------------------------
prepare() {
  command -v go >/dev/null 2>&1 || die "go not found"
  # Slice B (serve-b) never touches a kind cluster — the cloud fixture is
  # DB-seeded V2 rows, so REQUIRE_KIND=0 skips the fixture guards entirely.
  if [ "${REQUIRE_KIND:-1}" = "1" ]; then
    command -v kubectl >/dev/null 2>&1 || die "kubectl not found"
    kubectl --context "$SLICEA_CONTEXT" get ns v3-seed >/dev/null 2>&1 \
      || die "context $SLICEA_CONTEXT has no v3-seed namespace (create the v2-p3 fixture first — v2-phase1/k8s-fixture/README.md)"
  fi
  if command -v ss >/dev/null 2>&1; then
    if ss -ltnp "sport = :$BACKEND_PORT" 2>/dev/null | grep -q LISTEN; then
      # A previous lane run may have leaked its own backend child — reap only
      # that binary, never an unrelated process on the port.
      if ss -ltnp "sport = :$BACKEND_PORT" 2>/dev/null | grep -q "$BIN"; then
        log "reaping a stale backend child from a previous run"
        pkill -f "$BIN" || true
        sleep 1
      fi
      ss -ltn "sport = :$BACKEND_PORT" | grep -q LISTEN \
        && die "port $BACKEND_PORT still in use by a foreign process — the vite proxy targets it exclusively"
    fi
  fi

  mkdir -p "$RUN_DIR/data"
  : > "$LOG"
  log "creating dedicated schema $E2E_SCHEMA (VK-7)"
  mysql_exec -e "DROP DATABASE IF EXISTS $E2E_SCHEMA; CREATE DATABASE $E2E_SCHEMA CHARACTER SET utf8mb4;"

  cat > "$CONFIG" <<EOF
app:
  name: ops-admin-e2e
  port: "$BACKEND_PORT"
  mode: release
db:
  host: $MYSQL_HOST
  port: "$MYSQL_PORT"
  user: $MYSQL_USER
  password: "$MYSQL_PASSWORD"
  name: $E2E_SCHEMA
  log-mode: false
security:
  credential-key: ""
EOF

  log "building backend binary"
  (cd "$REPO_ROOT/backend" && go build -o "$BIN" .)

  cat > "$STATE" <<EOF
E2E_SCHEMA=$E2E_SCHEMA
CLUSTER_NAME=$CLUSTER_NAME
EOF
}

# --- mode: register -----------------------------------------------------------
# Same deterministic registration the N16 harness performs: a v1 k8s_cluster
# row (plaintext kubeconfig, §4.4) whose id feeds the backfill connection UID.
# Requires a booted backend — the v1 tables (k8s_cluster, admin, seeds) exist
# only after one boot of store.AutoMigrate + store.Seed; the CLI subcommands
# run just the v2 migration set.
register() {
  [ -f "$BIN" ] || die "run 'stack.sh prepare' first"
  [ -f "$RUN_DIR/backend-booted" ] || die "backend has never booted — the v1 schema does not exist yet"
  local kubeconfig api_server k8s_version node_count
  kubeconfig="$(kubectl config view --minify --flatten --context "$SLICEA_CONTEXT")"
  [ -n "$kubeconfig" ] || die "minified kubeconfig is empty"
  api_server="$(kubectl config view --minify --flatten --context "$SLICEA_CONTEXT" -o jsonpath='{.clusters[0].cluster.server}')"
  k8s_version="$(kubectl --context "$SLICEA_CONTEXT" version -o json 2>/dev/null \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["serverVersion"]["gitVersion"])')"
  node_count="$(kubectl --context "$SLICEA_CONTEXT" get nodes -o name | wc -l | tr -d ' ')"

  # SQL literal safety: move the kubeconfig through base64 / FROM_BASE64.
  local kube_b64 cluster_id conn_uid
  kube_b64="$(printf '%s' "$kubeconfig" | base64 -w0)"
  mysql_exec -e "INSERT INTO $E2E_SCHEMA.k8s_cluster
    (name, status, api_server, version, node_count, env, tags, connection_mode,
     description, kube_config, created_at, updated_at)
    VALUES ('$CLUSTER_NAME', 'running', '$api_server', '$k8s_version', $node_count,
      'dev', '[]', 'direct', 'Playwright UI-flow fixture',
      CONVERT(FROM_BASE64('$kube_b64') USING utf8mb4), NOW(3), NOW(3));"
  cluster_id="$(mysql_exec -N -s -e "SELECT id FROM $E2E_SCHEMA.k8s_cluster WHERE name='$CLUSTER_NAME';")"
  [ -n "$cluster_id" ] || die "k8s_cluster insert failed"
  conn_uid="$(printf 'backfill|k8s_cluster|%s' "$cluster_id" | sha256sum | cut -c1-32)"
  log "cluster registered id=$cluster_id connection_uid=$conn_uid"

  log "running sync-inventory (backfill + one sync)"
  (cd "$RUN_DIR" && OPS_ADMIN_INITIAL_PASSWORD="$E2E_ADMIN_PASSWORD" \
    OPS_SECRET_MASTER_KEYS="$MASTER_KEYS" \
    "$BIN" sync-inventory --config "$CONFIG" --connection "$conn_uid" --data "$RUN_DIR/data" \
    > "$RUN_DIR/sync-report.json")
  echo "$cluster_id" > "$RUN_DIR/cluster-id"
  echo "$conn_uid" > "$RUN_DIR/connection-uid"
}

# --- backend child ------------------------------------------------------------
start_backend() {
  log "starting backend on :$BACKEND_PORT"
  # exec inside the subshell: the recorded pid IS the backend binary, so
  # stop_backend's kill cannot orphan it (a wrapped `cmd &` records a bash
  # fork instead, and the child survives).
  (
    cd "$RUN_DIR"
    exec env OPS_ADMIN_INITIAL_PASSWORD="$E2E_ADMIN_PASSWORD" \
      OPS_SECRET_MASTER_KEYS="$MASTER_KEYS" \
      OPS_ADMIN_ENGINE_POLL_INTERVAL_MS=250 \
      OPS_ADMIN_ENGINE_LEASE_SECONDS=5 \
      OPS_ADMIN_ENGINE_REAPER_GRACE_MS=500 \
      "$BIN"
  ) >> "$LOG" 2>&1 &
  echo $! > "$RUN_DIR/backend.pid"
  # Readiness probe = login (boot covers migrations, seeds, compose, engine).
  local body
  for _ in $(seq 1 360); do
    body="$(curl -s -m 2 -X POST -H 'Content-Type: application/json' \
      -d "{\"username\":\"admin\",\"password\":\"$E2E_ADMIN_PASSWORD\"}" \
      "http://127.0.0.1:$BACKEND_PORT/api/v1/login" || true)"
    if printf '%s' "$body" | grep -q '"token":"'; then
      log "backend ready (login ok)"
      touch "$RUN_DIR/backend-booted"
      return 0
    fi
    sleep 0.5
  done
  tail -30 "$LOG" >&2 || true
  die "backend not ready within 180s (log: $LOG)"
}

stop_backend() {
  if [ -f "$RUN_DIR/backend.pid" ]; then
    local pid; pid="$(cat "$RUN_DIR/backend.pid")"
    if kill "$pid" >/dev/null 2>&1; then
      # Escalate: a graceful drain may stall — the lane must never leave the
      # child holding the port or the DB schema.
      local waited=0
      while kill -0 "$pid" >/dev/null 2>&1 && [ "$waited" -lt 10 ]; do
        sleep 1; waited=$((waited + 1))
      done
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$RUN_DIR/backend.pid"
  fi
}

# Shared teardown: reap the backend child, drop the dedicated schema unless
# STACK_KEEP=1 (dirty runbook — VK-12). The run dir with the backend log and
# the sync/compare artifacts is always left in place for inspection.
teardown_stack() {
  stop_backend
  if [ "${STACK_KEEP:-0}" != "1" ]; then
    log "dropping schema $E2E_SCHEMA"
    mysql_exec -e "DROP DATABASE IF EXISTS $E2E_SCHEMA;"
  else
    log "STACK_KEEP=1 — schema $E2E_SCHEMA kept (dirty runbook: v2-phase1/k8s-fixture/README.md)"
  fi
}

# --- mode: serve (Playwright webServer command) -------------------------------
serve() {
  prepare
  log "applying seed fixture (idempotent) and waiting for rollout"
  kubectl --context "$SLICEA_CONTEXT" apply -f "$SEED" >/dev/null
  kubectl --context "$SLICEA_CONTEXT" -n v3-seed rollout status deploy/restart-target --timeout=180s >/dev/null
  # Boot first (creates the v1 schema), then register + sync against it.
  start_backend
  register
  start_vite
}

# --- Slice B cloud fixture (plan N17 — PR 30) ---------------------------------
# Seeds a MOCK cloud account directly into the V2 tables: one aliyun
# connection + account context, two compute.vm resources and one
# orchestration.node (the node proves the view's kindPrefix=compute. filter —
# it must never appear in the Compute Inventory list). This is E-1(b)
# territory: no real credentials exist (§0.1), so the run's artifact carries
# the §13-10 interim marking. Discovery itself is NOT exercised here (E2E
# 범위 한계 — plan §6 M6): gate ③ (UI reads V2) is the only claim.
seed_cloud() {
  [ -f "$RUN_DIR/backend-booted" ] || die "backend has never booted — the V2 schema does not exist yet"
  log "seeding mock aliyun cloud account into V2 tables"
  mysql_exec "$E2E_SCHEMA" <<'SQL'
INSERT INTO provider_connection
  (uid, provider_type, name, endpoint, status, stale_source, source_model, source_id, created_at, updated_at)
VALUES
  ('conn-mock-aliyun', 'aliyun', 'mock-cloud-account', 'http://127.0.0.1:9/mock-aliyun', 'active', 0, 'asset_cloud_account', 1, NOW(3), NOW(3));

INSERT INTO provider_context (uid, connection_id, kind, external_id, name, status, created_at, updated_at)
SELECT 'ctx-mock-aliyun', id, 'account', 'mock-access-key-id', 'mock-cloud-account', 'active', NOW(3), NOW(3)
FROM provider_connection WHERE uid = 'conn-mock-aliyun';

INSERT INTO infra_resource
  (uid, context_id, kind, subtype, external_id, external_urn, name, display_name,
   lifecycle_state, health_state, managed_state, labels_json, first_seen_at, last_seen_at)
SELECT 'res-mock-vm-1', pc.id, 'compute.vm', 'ecs.g7', 'i-mock0001',
       'urn:aliyun:ctx-mock-aliyun:compute.vm:i-mock0001', 'i-mock0001', 'mock-vm-web-01',
       'running', 'healthy', 'discovered', '{"region":"cn-hangzhou"}', NOW(3), NOW(3)
FROM provider_context pc WHERE pc.uid = 'ctx-mock-aliyun';

INSERT INTO infra_resource
  (uid, context_id, kind, subtype, external_id, external_urn, name, display_name,
   lifecycle_state, health_state, managed_state, labels_json, first_seen_at, last_seen_at)
SELECT 'res-mock-vm-2', pc.id, 'compute.vm', 'ecs.c6', 'i-mock0002',
       'urn:aliyun:ctx-mock-aliyun:compute.vm:i-mock0002', 'i-mock0002', 'mock-vm-db-01',
       'running', 'healthy', 'discovered', '{"region":"cn-hangzhou"}', NOW(3), NOW(3)
FROM provider_context pc WHERE pc.uid = 'ctx-mock-aliyun';

INSERT INTO infra_resource
  (uid, context_id, kind, subtype, external_id, external_urn, name, display_name,
   lifecycle_state, health_state, managed_state, first_seen_at, last_seen_at)
SELECT 'res-mock-node', pc.id, 'orchestration.node', '', 'n-mock0001',
       'urn:mock:ctx-mock-aliyun:orchestration.node:n-mock0001', 'n-mock0001', 'mock-control-plane',
       'running', 'healthy', 'discovered', NOW(3), NOW(3)
FROM provider_context pc WHERE pc.uid = 'ctx-mock-aliyun';

INSERT INTO resource_observation
  (resource_id, generation_uid, observation_hash, normalizer_version, normalized_json, raw_json, observed_at)
SELECT r.id, 'gen-mock-1', 'sha256-mock-observation-1', '1',
       '{"displayName":"mock-vm-web-01","region":"cn-hangzhou","zone":"cn-hangzhou-i","cpu":4,"memoryGB":16,"diskGB":80,"os":"Alibaba Cloud Linux","privateIps":["10.0.0.11"],"publicIps":["47.0.0.11"],"instanceType":"ecs.g7.large","status":"Running"}',
       '{"InstanceId":"i-mock0001","InstanceType":"ecs.g7.large","Status":"Running"}',
       NOW(3)
FROM infra_resource r WHERE r.uid = 'res-mock-vm-1';

INSERT INTO resource_observation
  (resource_id, generation_uid, observation_hash, normalizer_version, normalized_json, raw_json, observed_at)
SELECT r.id, 'gen-mock-1', 'sha256-mock-observation-2', '1',
       '{"displayName":"mock-vm-db-01","region":"cn-hangzhou","zone":"cn-hangzhou-h","cpu":8,"memoryGB":32,"diskGB":200,"os":"Ubuntu 22.04","privateIps":["10.0.0.12"],"publicIps":[],"instanceType":"ecs.c6.xlarge","status":"Running"}',
       '{"InstanceId":"i-mock0002","InstanceType":"ecs.c6.xlarge","Status":"Running"}',
       NOW(3)
FROM infra_resource r WHERE r.uid = 'res-mock-vm-2';
SQL
  local vms
  vms="$(mysql_exec -N -s "$E2E_SCHEMA" -e "SELECT COUNT(*) FROM infra_resource WHERE kind='compute.vm';")"
  log "mock cloud fixture seeded (compute.vm rows: $vms)"

  # §13-10 interim marking — machine-detectable top-level "interim": true on
  # the run artifact. This E2E proves gate ③ only; gates ①② stay pending on
  # real credentials (E-1), so M1 must not be declared from this run.
  local artifact="$RUN_DIR/data/slice-b"
  mkdir -p "$artifact"
  cat > "$artifact/artifact.json" <<EOF
{
  "slice": "b",
  "gate": "phase4-gate-3-ui-v2-read",
  "interim": true,
  "interimReason": "E-1(b): mock cloud account (DB-seeded V2 rows) — real Aliyun/Tencent credentials absent (plan §13-10)",
  "schema": "$E2E_SCHEMA",
  "connectionUid": "conn-mock-aliyun",
  "providerType": "aliyun",
  "seededComputeVmRows": $vms,
  "scope": "UI V2 read only — real-credentials discovery (gate 1) and the section-15 pairs (gate 2) are not covered",
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
  log "interim-marked artifact: $artifact/artifact.json"
}

# --- mode: serve-b (Slice B Playwright webServer command) ---------------------
serve_b() {
  REQUIRE_KIND=0
  E2E_SCHEMA="${E2E_SCHEMA:-ops_admin_p4b}"
  # Failure-safe like compare(): the backend child never outlives this mode.
  trap teardown_stack EXIT
  prepare
  start_backend
  seed_cloud
  start_vite
}

# --- Slice C PVE fixture (plan N9 — PR 31c) ------------------------------------
# Seeds a MOCK proxmox account straight into the V2 tables. R10 forbids
# hand-written resource JSON: the rows are rendered by the package's own
# normalizer (TestSliceCSeedWriter, a test helper this script drives with
# SLICEC_SEED_OUT) — one compute.vm, one compute.system_container and one
# compute.hypervisor_node under a cluster context. Discovery itself is NOT
# exercised here (real-endpoint proof stays with the register-pve runbook);
# the UI read path (kindPrefix=compute.) and the opdef × kind intersection
# (pve.guest.* on guest kinds) are the claims.
seed_pve() {
  [ -f "$RUN_DIR/backend-booted" ] || die "backend has never booted — the V2 schema does not exist yet"
  log "generating PVE seed SQL through the proxmox normalizer"
  mkdir -p "$RUN_DIR/data/slice-c"
  (cd "$REPO_ROOT/backend" && SLICEC_SEED_OUT="$RUN_DIR/data/slice-c/seed.sql" \
    go test ./internal/infra/adapter/proxmox -run TestSliceCSeedWriter -count=1 > "$RUN_DIR/data/slice-c/seed-gen.log" 2>&1) \
    || { tail -20 "$RUN_DIR/data/slice-c/seed-gen.log" >&2; die "seed helper failed — see $RUN_DIR/data/slice-c/seed-gen.log"; }
  [ -s "$RUN_DIR/data/slice-c/seed.sql" ] || die "seed helper produced no SQL"
  mysql_exec "$E2E_SCHEMA" < "$RUN_DIR/data/slice-c/seed.sql"
  local compute_rows
  compute_rows="$(mysql_exec -N -s "$E2E_SCHEMA" -e \
    "SELECT COUNT(*) FROM infra_resource WHERE uid LIKE 'res-pve-mock-%';")"
  [ "$compute_rows" = "3" ] || die "expected 3 seeded PVE rows, got $compute_rows"
  log "pve mock fixture seeded (res-pve-mock rows: $compute_rows)"
}

# --- mode: serve-c (Slice C Playwright webServer command) ---------------------
serve_c() {
  REQUIRE_KIND=0
  E2E_SCHEMA="${E2E_SCHEMA:-ops_admin_p5c}"
  # Failure-safe like serve_b(): the backend child never outlives this mode.
  trap teardown_stack EXIT
  prepare
  start_backend
  seed_pve
  start_vite
}

start_vite() {
  # No `exec` for vite: the EXIT trap must survive to reap the backend child
  # and the schema — Playwright SIGTERMs this script when the run ends.
  trap teardown_stack EXIT INT TERM
  log "starting vite dev server on :$WEB_PORT"
  export E2E_API_TARGET="http://127.0.0.1:$BACKEND_PORT"
  cd "$REPO_ROOT/web"
  npx vite --port "$WEB_PORT" --strictPort --host 127.0.0.1 &
  VITE_PID=$!
  wait "$VITE_PID"
}

# --- mode: compare (J11's §15 segment — CLI only) ------------------------------
# prepare → boot (v1 schema) → register + sync → compare-inventory capture.
# A BLOCKER verdict exits non-zero; the artifact under .stack/data is always
# the primary output (§15.4 r2).
compare() {
  # Failure-safe: the backend child must never outlive this mode (CI has one
  # lane per runner, but a local dev backend may share the machine).
  trap teardown_stack EXIT
  prepare
  start_backend
  register
  local cluster_id
  cluster_id="$(cat "$RUN_DIR/cluster-id")"
  # Same key family as the backend child and the CLI runs — the G-5 gate
  # refuses an empty secret source outside explicit development.
  OPS_SECRET_MASTER_KEYS="$MASTER_KEYS" \
    "$BIN" compare-inventory --config "$CONFIG" --cluster "$cluster_id" --data "$RUN_DIR/data"
  log "compare-inventory verdict ok (exit 0) for cluster id=$cluster_id"
}

# --- mode: down ---------------------------------------------------------------
down() {
  stop_backend
  if [ "${STACK_KEEP:-0}" != "1" ] && [ -f "$STATE" ]; then
    log "dropping schema $E2E_SCHEMA"
    mysql_exec -e "DROP DATABASE IF EXISTS $E2E_SCHEMA;"
  fi
}

case "${1:-}" in
  prepare)  prepare ;;
  register) register ;;
  serve)    serve ;;
  serve-b)  serve_b ;;
  serve-c)  serve_c ;;
  compare)  compare ;;
  down)     down ;;
  *) die "usage: stack.sh {prepare|register|serve|serve-b|serve-c|compare|down}" ;;
esac
