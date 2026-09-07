//go:build e2e

package slicea

// The N16 harness: child-process backend management, the HTTP client for the
// full §25 trace surface, the L3 measurement instrument (kubectl subprocess
// JSON — plan §0.1) and direct SQL probes into the dedicated schema.
//
// Assumptions this harness encodes (each is a plan judgment):
//   - the child backend is a real binary, not an in-process router: crash
//     injection must kill real OS state (J9 — "백엔드 바이너리를 자식 프로세스로 관리")
//   - engine knobs ride OPS_ADMIN_ENGINE_* env (J10/A6): poll 250ms, lease 5s,
//     grace 500ms. The lease LOWER BOUND stays max(LeaseSeconds, CallTimeout=30s)
//     + grace ≈ 30.5s (T-8) — the crash leg reads lease_expires_at from the
//     task row and waits past it before restarting (H1 — deterministic requeue).
//   - cluster registration goes through the same v1 k8s_cluster row the CLI
//     backfill reads (plaintext kubeconfig, §4.4); backfill+sync run through
//     the sync-inventory subcommand, never a parallel reimplementation.

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	restartOperationName  = "k8s.workload.restart"
	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

	enginePollMS     = 250
	engineLeaseSecs  = 5
	engineGraceMS    = 500
	terminalBudget   = 120 * time.Second // §0.7 회복 예산 상한
	childReadyBudget = 180 * time.Second // first boot = full schema DDL on dev MySQL (WSL2 disk is slow); reboots are fast
)

type harness struct {
	kubeContext string
	namespace   string
	deployName  string

	binPath    string
	runDir     string // child cwd: holds config.yaml
	configPath string
	dataDir    string
	port       string
	baseURL    string

	adminPassword string
	masterKeys    string

	dbAdmin *sql.DB // server-level (schema create/drop)
	db      *sql.DB // schema-selected

	child        *exec.Cmd
	childLogPath string
	childLogFile *os.File

	connUID     string
	resourceUID string
	token       string
}

// --- assembly ---------------------------------------------------------------

// assembleHarness builds the whole trace environment or reports a skip
// reason. Every failure here is environmental (no cluster, no DB, no kubectl)
// — the suite skips rather than fails (CI 없는 환경 스킵).
func assembleHarness() (*harness, error) {
	h := &harness{
		kubeContext:   envOr("SLICEA_CONTEXT", "kind-v2-p3"),
		namespace:     envOr("SLICEA_NS", "v3-seed"),
		deployName:    envOr("SLICEA_DEPLOY", "restart-target"),
		adminPassword: envOr("SLICEA_ADMIN_PASSWORD", "slicea-e2e-admin"),
		masterKeys:    resolveMasterKeys(),
	}
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH")
	}
	if out, err := exec.Command("kubectl", "--context", h.kubeContext, "get", "ns", h.namespace).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("kind context %s unreachable (need v2-p3 fixture — see v2-phase1/k8s-fixture/README.md): %v: %s",
			h.kubeContext, err, strings.TrimSpace(string(out)))
	}

	mysqlHost := envOr("SLICEA_MYSQL_HOST", "127.0.0.1")
	mysqlPort := envOr("SLICEA_MYSQL_PORT", "3306")
	mysqlUser := envOr("SLICEA_MYSQL_USER", "root")
	mysqlPass := envOr("SLICEA_MYSQL_PASSWORD", "123456")
	schema := envOr("SLICEA_SCHEMA", "ops_admin_p3e2e") // VK-7 — dedicated, never shared

	dsn := func(dbName string) string {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local", mysqlUser, mysqlPass, mysqlHost, mysqlPort, dbName)
	}
	admin, err := sql.Open("mysql", dsn(""))
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	if err := admin.Ping(); err != nil {
		return nil, fmt.Errorf("dev MySQL unreachable at %s:%s: %w", mysqlHost, mysqlPort, err)
	}
	h.dbAdmin = admin
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + schema); err != nil {
		return nil, fmt.Errorf("drop stale schema %s: %w", schema, err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + schema + " CHARACTER SET utf8mb4"); err != nil {
		return nil, fmt.Errorf("create schema %s: %w", schema, err)
	}
	h.db, err = sql.Open("mysql", dsn(schema))
	if err != nil {
		return nil, fmt.Errorf("mysql open (%s): %w", schema, err)
	}

	runDir, err := os.MkdirTemp("", "slicea-e2e-*")
	if err != nil {
		return nil, err
	}
	h.runDir = runDir
	h.dataDir = filepath.Join(runDir, "data")
	if err := os.MkdirAll(h.dataDir, 0o755); err != nil {
		return nil, err
	}
	h.configPath = filepath.Join(runDir, "config.yaml")
	h.childLogPath = filepath.Join(runDir, "backend.log")
	h.port = freePort()
	h.baseURL = "http://127.0.0.1:" + h.port
	if err := os.WriteFile(h.configPath, []byte(fmt.Sprintf(`app:
  name: ops-admin-slicea
  port: "%s"
  mode: release
db:
  host: %s
  port: "%s"
  user: %s
  password: "%s"
  name: %s
  log-mode: false
security:
  credential-key: ""
`, h.port, mysqlHost, mysqlPort, mysqlUser, mysqlPass, schema)), 0o600); err != nil {
		return nil, err
	}

	bin, err := buildBackend()
	if err != nil {
		return nil, fmt.Errorf("build backend: %w", err)
	}
	h.binPath = bin

	// Fixture: the committed N17 seed, applied idempotently BEFORE any
	// baseline snapshot so an apply-induced rollout never pollutes RS counting.
	seedPath, err := repoFile("v2-phase1/k8s-fixture/seed.yaml")
	if err != nil {
		return nil, err
	}
	if out, err := exec.Command("kubectl", "--context", h.kubeContext, "apply", "-f", seedPath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("apply seed fixture: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("kubectl", "--context", h.kubeContext, "-n", h.namespace,
		"rollout", "status", "deploy/"+h.deployName, "--timeout=180s").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("seed rollout: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return h, nil
}

// release tears the environment down: child first (port freed), then the
// dedicated schema — dropped on green, KEPT on red for the dirty runbook.
func (h *harness) release(code int) {
	h.stopBackend()
	if h.db != nil {
		_ = h.db.Close()
	}
	schema := envOr("SLICEA_SCHEMA", "ops_admin_p3e2e")
	if code == 0 {
		if _, err := h.dbAdmin.Exec("DROP DATABASE IF EXISTS " + schema); err != nil {
			fmt.Printf("slicea: WARN — schema %s left in place (drop failed: %v)\n", schema, err)
		} else {
			fmt.Printf("slicea: dedicated schema %s dropped (green run)\n", schema)
		}
		_ = os.RemoveAll(h.runDir)
	} else {
		fmt.Printf("slicea: RED run — schema %s and child log %s KEPT for inspection (dirty runbook: v2-phase1/k8s-fixture/README.md)\n",
			schema, h.childLogPath)
	}
	if h.dbAdmin != nil {
		_ = h.dbAdmin.Close()
	}
}

// resolveMasterKeys keeps the run reproducible: explicit SLICEA_MASTER_KEYS,
// then the deployment's OPS_SECRET_MASTER_KEYS, then the rotation key file
// recipe (k20260906:…), then a built-in constant. Any value works — seal and
// unseal share the process family's env.
func resolveMasterKeys() string {
	if v := strings.TrimSpace(os.Getenv("SLICEA_MASTER_KEYS")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("OPS_SECRET_MASTER_KEYS")); v != "" {
		return v
	}
	if material, err := os.ReadFile("/tmp/rotation-new-key.txt"); err == nil && len(strings.TrimSpace(string(material))) > 0 {
		return "k20260906:" + strings.TrimSpace(string(material))
	}
	return "k20260906:slicea-e2e-master-key-material-fixed-48-bytes!!"
}

// bootTrace runs the remaining environment steps that need a test context:
// child boot (migrations+seeds+engine lane), v1 cluster registration into the
// dedicated schema, backfill+sync, and trace-target discovery.
func (h *harness) bootTrace(t *testing.T) {
	t.Helper()
	h.startBackend(t)
	h.registerCluster(t)
	h.runSyncCLI(t)
	h.discoverResource(t)
}

// --- child backend ----------------------------------------------------------

// buildBackend compiles the module's main package once (module root is two
// levels above this package dir).
func buildBackend() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	moduleRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // .../backend
	bin := filepath.Join(os.TempDir(), fmt.Sprintf("ops-admin-slicea-%d", os.Getpid()))
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return bin, nil
}

// repoFile resolves a repo-root-relative asset path from this source file.
func repoFile(rel string) (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))) // .../ops-admin
	path := filepath.Join(root, rel)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("repo asset %s: %w", rel, err)
	}
	return path, nil
}

func (h *harness) childEnv() []string {
	return append(os.Environ(),
		"OPS_ADMIN_INITIAL_PASSWORD="+h.adminPassword,
		"OPS_SECRET_MASTER_KEYS="+h.masterKeys,
		fmt.Sprintf("OPS_ADMIN_ENGINE_POLL_INTERVAL_MS=%d", enginePollMS),
		fmt.Sprintf("OPS_ADMIN_ENGINE_LEASE_SECONDS=%d", engineLeaseSecs),
		fmt.Sprintf("OPS_ADMIN_ENGINE_REAPER_GRACE_MS=%d", engineGraceMS),
	)
}

// startBackend boots the child and waits for a successful login — boot covers
// migrations, seeds, compose and the engine lane, so login IS the readiness
// probe.
func (h *harness) startBackend(t *testing.T) {
	t.Helper()
	if h.child != nil {
		t.Fatal("harness bug: child already running")
	}
	logFile, err := os.OpenFile(h.childLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open child log: %v", err)
	}
	cmd := exec.Command(h.binPath)
	cmd.Dir = h.runDir
	cmd.Env = h.childEnv()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		t.Fatalf("start backend child: %v", err)
	}
	h.child = cmd
	h.childLogFile = logFile
	h.token = "" // fresh process, fresh session
	h.waitReadyAndLogin(t)
	h.assertEngineOverrideLine(t)
}

// killBackend9 is the crash injection: SIGKILL, no drain, no graceful stop —
// the in-flight attempt dies mid-run and its lease is the only recovery hint.
func (h *harness) killBackend9(t *testing.T) {
	t.Helper()
	if h.child == nil || h.child.Process == nil {
		t.Fatal("harness bug: no child to kill")
	}
	if err := h.child.Process.Kill(); err != nil { // Go Kill == SIGKILL
		t.Fatalf("SIGKILL child: %v", err)
	}
	_ = h.child.Wait() // reaps; "signal: killed" is the expected outcome
	h.closeChildLog()
	h.child = nil
	fmt.Printf("slicea: child SIGKILLed at %s\n", time.Now().Format(time.RFC3339Nano))
}

// stopBackend is the planned shutdown (SIGTERM → F-7 graceful order → wait,
// hard kill after 12s).
func (h *harness) stopBackend() {
	if h.child == nil || h.child.Process == nil {
		return
	}
	_ = h.child.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = h.child.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(12 * time.Second):
		_ = h.child.Process.Kill()
		<-done
	}
	h.closeChildLog()
	h.child = nil
}

func (h *harness) closeChildLog() {
	if h.childLogFile != nil {
		_ = h.childLogFile.Close()
		h.childLogFile = nil
	}
}

// waitReadyAndLogin polls the login endpoint until the child answers with a
// token (connection refused = still booting).
func (h *harness) waitReadyAndLogin(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(childReadyBudget)
	var lastErr string
	for time.Now().Before(deadline) {
		body := fmt.Sprintf(`{"username":"admin","password":%q}`, h.adminPassword)
		resp, err := http.Post(h.baseURL+"/api/v1/login", "application/json", strings.NewReader(body))
		if err == nil {
			var envelope struct {
				Code int `json:"code"`
				Data struct {
					Token string `json:"token"`
				} `json:"data"`
			}
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if json.Unmarshal(raw, &envelope) == nil && envelope.Code == 200 && envelope.Data.Token != "" {
				h.token = envelope.Data.Token
				return
			}
			lastErr = fmt.Sprintf("login answered code=%d body=%s", resp.StatusCode, truncate(string(raw), 200))
		} else {
			lastErr = err.Error()
		}
		time.Sleep(250 * time.Millisecond)
	}
	h.dumpChildLogTail()
	t.Fatalf("backend child not ready within %s (last: %s)", childReadyBudget, lastErr)
}

// assertEngineOverrideLine pins the VK-8 startup summary to the env values
// this suite injected (R5 — a mis-tuned pair must be visible at boot). The
// log is append-mode across boots, so the LAST matching line is the current
// process's config.
func (h *harness) assertEngineOverrideLine(t *testing.T) {
	t.Helper()
	want := fmt.Sprintf("poll_interval=%dms lease_seconds=%d reaper_grace=%dms", enginePollMS, engineLeaseSecs, engineGraceMS)
	logBytes, err := os.ReadFile(h.childLogPath)
	if err != nil {
		t.Fatalf("read child log: %v", err)
	}
	var lastLine string
	for _, line := range strings.Split(string(logBytes), "\n") {
		if strings.Contains(line, "v2 task engine starting:") {
			lastLine = strings.TrimSpace(line)
		}
	}
	if lastLine == "" {
		t.Fatalf("no \"v2 task engine starting:\" line in child log (engine lane missing?) — see %s", h.childLogPath)
	}
	if !strings.Contains(lastLine, want) {
		t.Fatalf("engine override not applied — want %q in %q", want, lastLine)
	}
	fmt.Printf("slicea: engine config line: %s\n", lastLine)
}

func (h *harness) dumpChildLogTail() {
	logBytes, _ := os.ReadFile(h.childLogPath)
	lines := strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	fmt.Printf("---- child log tail (%s) ----\n%s\n---------------------------\n", h.childLogPath, strings.Join(lines, "\n"))
}

// --- fixture registration + sync (same paths run-fixture.sh drives) --------

// registerCluster inserts the v1 k8s_cluster row (plaintext kubeconfig, §4.4)
// into the dedicated schema and derives the deterministic backfill connection
// UID (inventory/sync.go SourceKeyUID).
func (h *harness) registerCluster(t *testing.T) {
	t.Helper()
	kubeconfig, err := exec.Command("kubectl", "config", "view", "--minify", "--flatten", "--context", h.kubeContext).Output()
	if err != nil {
		t.Fatalf("minify kubeconfig: %v", err)
	}
	apiServer, err := exec.Command("kubectl", "config", "view", "--minify", "--flatten", "--context", h.kubeContext,
		"-o", `jsonpath={.clusters[0].cluster.server}`).Output()
	if err != nil {
		t.Fatalf("kubeconfig api server: %v", err)
	}
	verRaw, err := exec.Command("kubectl", "--context", h.kubeContext, "version", "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl version: %v", err)
	}
	var ver struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if err := json.Unmarshal(verRaw, &ver); err != nil {
		t.Fatalf("decode kubectl version: %v", err)
	}
	nodesRaw, err := exec.Command("kubectl", "--context", h.kubeContext, "get", "nodes", "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl get nodes: %v", err)
	}
	var nodes struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	res, err := h.db.Exec(`INSERT INTO k8s_cluster
		(name, status, api_server, version, node_count, env, tags, connection_mode, description, kube_config, created_at, updated_at)
		VALUES (?, 'running', ?, ?, ?, 'dev', '[]', 'direct', 'slicea e2e fixture', ?, NOW(3), NOW(3))`,
		h.kubeContext, strings.TrimSpace(string(apiServer)), ver.ServerVersion.GitVersion, len(nodes.Items), string(kubeconfig))
	if err != nil {
		t.Fatalf("insert k8s_cluster: %v", err)
	}
	clusterID, _ := res.LastInsertId()
	sum := sha256.Sum256([]byte(fmt.Sprintf("backfill|k8s_cluster|%d", clusterID)))
	h.connUID = hex.EncodeToString(sum[:])[:32]
	fmt.Printf("slicea: cluster registered id=%d connection_uid=%s\n", clusterID, h.connUID)
}

// runSyncCLI runs the sync-inventory subcommand against the dedicated schema:
// §5.4 backfill (operations binding included — J12(1)) then one sync run.
func (h *harness) runSyncCLI(t *testing.T) {
	t.Helper()
	cmd := exec.Command(h.binPath, "sync-inventory", "--config", h.configPath, "--connection", h.connUID, "--data", h.dataDir)
	cmd.Dir = h.runDir
	cmd.Env = h.childEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sync-inventory failed: %v\n%s", err, string(out))
	}
	var report struct {
		Status   string         `json:"status"`
		Seen     int            `json:"seenCount"`
		Outcomes map[string]int `json:"outcomes"`
	}
	// The CLI prints the JSON report first, then a trailing "report artifact"
	// line — decode just the leading JSON value.
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&report); err != nil {
		t.Fatalf("decode sync report: %v\n%s", err, truncate(string(out), 300))
	}
	fmt.Printf("slicea: sync-inventory status=%s seen=%d outcomes=%v\n", report.Status, report.Seen, report.Outcomes)

	// J12(1) evidence in the dedicated schema: the operations binding must
	// exist or every execution would fail credential_error.
	var purposes string
	if err := h.db.QueryRow(`SELECT GROUP_CONCAT(purpose ORDER BY purpose)
		FROM provider_credential_binding b
		JOIN provider_connection c ON c.id = b.provider_connection_id
		WHERE c.uid = ?`, h.connUID).Scan(&purposes); err != nil {
		t.Fatalf("probe credential bindings: %v", err)
	}
	if !strings.Contains(purposes, "operations") {
		t.Fatalf("operations purpose binding missing after backfill (got %q)", purposes)
	}
	fmt.Printf("slicea: credential purposes=%s\n", purposes)
}

// discoverResource finds the trace target through the published API (list →
// match on externalUrn) — no normalizer knowledge duplicated here.
func (h *harness) discoverResource(t *testing.T) {
	t.Helper()
	status, _, body := h.do(t, http.MethodGet, "/api/v2/infra/resources?kind=orchestration.workload&pageSize=100", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("list resources: status %d: %s", status, truncate(string(body), 300))
	}
	var envelope struct {
		Data struct {
			Items []struct {
				UID         string `json:"uid"`
				ExternalURN string `json:"externalUrn"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode resources: %v", err)
	}
	suffix := h.namespace + "/deployment/" + h.deployName
	for _, item := range envelope.Data.Items {
		if strings.HasSuffix(item.ExternalURN, suffix) {
			h.resourceUID = item.UID
			fmt.Printf("slicea: trace target %s (urn %s)\n", h.resourceUID, item.ExternalURN)
			return
		}
	}
	t.Fatalf("no workload resource with externalUrn suffix %q among %d items", suffix, len(envelope.Data.Items))
}

// --- HTTP trace client ------------------------------------------------------

func (h *harness) do(t *testing.T, method, path string, body any, headers map[string]string) (int, http.Header, []byte) {
	t.Helper()
	if h.token == "" {
		t.Fatal("harness bug: no token — login first")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.baseURL+path, reader)
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s read body: %v", method, path, err)
	}
	return resp.StatusCode, resp.Header, raw
}

// resourcePath escapes a URN uid into ONE path segment — the router routes on
// the escaped path (router.go UseRawPath), a raw '/' would 404.
func resourcePath(uid string) string {
	return url.PathEscape(uid)
}

func (h *harness) planRestart(t *testing.T) map[string]any {
	t.Helper()
	path := "/api/v2/infra/resources/" + resourcePath(h.resourceUID) + "/operations/" + restartOperationName + "/plan"
	status, _, body := h.do(t, http.MethodPost, path, map[string]any{}, nil)
	if status != http.StatusOK {
		t.Fatalf("plan: status %d: %s", status, truncate(string(body), 300))
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}
	return envelope.Data
}

// executeRestart returns (httpStatus, replayHeaderPresent, taskUID).
func (h *harness) executeRestart(t *testing.T, frozen, idempotencyKey string) (int, bool, string) {
	t.Helper()
	path := "/api/v2/infra/resources/" + resourcePath(h.resourceUID) + "/operations/" + restartOperationName + "/execute"
	status, header, body := h.do(t, http.MethodPost, path,
		map[string]any{"restartedAt": frozen},
		map[string]string{"Idempotency-Key": idempotencyKey})
	var envelope struct {
		Data struct {
			Task struct {
				UID    string `json:"uid"`
				Status string `json:"status"`
			} `json:"task"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode execute response (status %d): %v: %s", status, err, truncate(string(body), 300))
	}
	if envelope.Data.Task.UID == "" && status < 400 {
		t.Fatalf("execute response carries no task uid: %s", truncate(string(body), 300))
	}
	return status, header.Get("Idempotency-Replayed") == "true", envelope.Data.Task.UID
}

func (h *harness) approveTask(t *testing.T, uid string) {
	t.Helper()
	status, _, body := h.do(t, http.MethodPost, "/api/v2/infra/tasks/"+uid+"/approve", map[string]any{}, nil)
	if status != http.StatusOK {
		t.Fatalf("approve %s: status %d: %s", uid, status, truncate(string(body), 300))
	}
}

func (h *harness) getTask(t *testing.T, uid string) map[string]any {
	t.Helper()
	status, _, body := h.do(t, http.MethodGet, "/api/v2/infra/tasks/"+uid, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("get task %s: status %d: %s", uid, status, truncate(string(body), 300))
	}
	var envelope struct {
		Data struct {
			Task map[string]any `json:"task"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode task %s: %v", uid, err)
	}
	return envelope.Data.Task
}

func (h *harness) getTaskEvents(t *testing.T, uid string) []map[string]any {
	t.Helper()
	status, _, body := h.do(t, http.MethodGet, "/api/v2/infra/tasks/"+uid+"/events", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("get task events %s: status %d: %s", uid, status, truncate(string(body), 300))
	}
	var envelope struct {
		Data struct {
			Events []map[string]any `json:"events"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode task events %s: %v", uid, err)
	}
	return envelope.Data.Events
}

// getResource fetches resource + latest observation (J6 refreshed-observation
// evidence).
func (h *harness) getResource(t *testing.T) (map[string]any, map[string]any) {
	t.Helper()
	status, _, body := h.do(t, http.MethodGet, "/api/v2/infra/resources/"+resourcePath(h.resourceUID), nil, nil)
	if status != http.StatusOK {
		t.Fatalf("get resource: status %d: %s", status, truncate(string(body), 300))
	}
	var envelope struct {
		Data struct {
			Resource    map[string]any `json:"resource"`
			Observation map[string]any `json:"observation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	return envelope.Data.Resource, envelope.Data.Observation
}

// --- L3 measurement instrument (§0.1) ---------------------------------------

type deployState struct {
	Generation  int64
	RestartedAt string
}

// deployState snapshots generation + the live pod-template restartedAt via a
// single kubectl JSON subprocess — the measurement basis for the increment
// and frozen-value assertions.
func (h *harness) deployState(t *testing.T) deployState {
	t.Helper()
	raw, err := exec.Command("kubectl", "--context", h.kubeContext, "-n", h.namespace,
		"get", "deploy", h.deployName, "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl get deploy: %v", err)
	}
	var deploy struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
		Spec struct {
			Template struct {
				Metadata struct {
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &deploy); err != nil {
		t.Fatalf("decode deploy json: %v", err)
	}
	return deployState{Generation: deploy.Metadata.Generation, RestartedAt: deploy.Spec.Template.Metadata.Annotations[restartedAtAnnotation]}
}

type rsInfo struct {
	Name      string
	CreatedAt time.Time
}

// replicaSets enumerates the namespace's ReplicaSets with creationTimestamps
// — new-RS counting uses creationTimestamp, never name order (k8s rollback
// semantics make lexical order non-evidence).
func (h *harness) replicaSets(t *testing.T) []rsInfo {
	t.Helper()
	raw, err := exec.Command("kubectl", "--context", h.kubeContext, "-n", h.namespace,
		"get", "rs", "-o", "json").Output()
	if err != nil {
		t.Fatalf("kubectl get rs: %v", err)
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode rs json: %v", err)
	}
	out := make([]rsInfo, 0, len(list.Items))
	for _, item := range list.Items {
		at, err := time.Parse(time.RFC3339, item.Metadata.CreationTimestamp)
		if err != nil {
			t.Fatalf("rs %s creationTimestamp %q: %v", item.Metadata.Name, item.Metadata.CreationTimestamp, err)
		}
		out = append(out, rsInfo{Name: item.Metadata.Name, CreatedAt: at})
	}
	return out
}

func replicaSetsCreatedAfter(all []rsInfo, since time.Time) []rsInfo {
	var out []rsInfo
	for _, rs := range all {
		if rs.CreatedAt.After(since) {
			out = append(out, rs)
		}
	}
	return out
}

// --- SQL probes -------------------------------------------------------------

// taskRow reads the durable row straight from the dedicated schema (crash-leg
// timing reads lease_expires_at while the child is down).
func (h *harness) taskRow(t *testing.T, uid string) (status string, leaseExpires *time.Time, attemptCount int) {
	t.Helper()
	var lease sql.NullTime
	err := h.db.QueryRow(`SELECT status, lease_expires_at, attempt_count FROM provider_task WHERE uid = ?`, uid).
		Scan(&status, &lease, &attemptCount)
	if err != nil {
		t.Fatalf("read provider_task %s: %v", uid, err)
	}
	if lease.Valid {
		leaseExpires = &lease.Time
	}
	return status, leaseExpires, attemptCount
}

// syncRunsSince counts sync runs of the trace connection started after the
// given instant — the J6 terminal-hook refresh evidence (the observation
// projection itself cannot show a rollout; see trace_test.go step 7).
func (h *harness) syncRunsSince(t *testing.T, since time.Time) int {
	t.Helper()
	var count int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM inventory_sync_run
		WHERE connection_id = (SELECT id FROM provider_connection WHERE uid = ?)
		  AND started_at > ?`, h.connUID, since).Scan(&count)
	if err != nil {
		t.Fatalf("count sync runs: %v", err)
	}
	return count
}

type auditRow struct {
	ID         int
	URL        string
	Method     string
	Username   string
	IP         string
	StatusCode int
	V2Context  string
}

func (h *harness) auditRowsForTask(t *testing.T, uid string) []auditRow {
	t.Helper()
	rows, err := h.db.Query(`SELECT id, url, method, username, ip, status_code, COALESCE(v2_context,'')
		FROM sys_operation_log WHERE task_uid = ? ORDER BY id`, uid)
	if err != nil {
		t.Fatalf("query sys_operation_log: %v", err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.ID, &r.URL, &r.Method, &r.Username, &r.IP, &r.StatusCode, &r.V2Context); err != nil {
			t.Fatalf("scan audit row: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// --- small helpers ----------------------------------------------------------

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func freePort() string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "18099"
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()
	return port
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// waitFor polls until cond passes or the budget dies, failing with the last
// observed state.
func waitFor(t *testing.T, what string, budget, interval time.Duration, cond func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(budget)
	var last string
	for time.Now().Before(deadline) {
		ok, state := cond()
		if ok {
			return
		}
		last = state
		time.Sleep(interval)
	}
	t.Fatalf("timed out after %s waiting for %s (last: %s)", budget, what, last)
}
