package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/router"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

// registerMetricsRoute wires /internal/metrics onto the assembled engine —
// the §18.2 internal scrape endpoint (P6). router.New의 조립 표면 밖 등록이므로
// 라우트 골든(route-inventory.txt·sensitive-routes.txt)과 무관하다(가정 A8).
// 인증 게이트는 없다 — §18.2의 "internal-only"는 배포 경계(리버스 프록시
// 비노출·망 분리) 시행 가정이다(가정 A1). 스택 실패(R11)로 render가 nil이면
// 등록하지 않는다 — 라우트 미등록 nil 가드가 claim 9.
func registerMetricsRoute(engine *gin.Engine, renderMetrics func() string) {
	if engine == nil || renderMetrics == nil {
		return
	}
	engine.GET("/internal/metrics", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(renderMetrics()))
	})
}

func main() {
	// Read-only subcommand, dispatched before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "inventory-secrets" {
		os.Exit(runSecretInventory(os.Args[2:]))
	}
	// Offline secret-migration subcommands (§4.4 Steps 2-3), dispatched
	// before any server startup path.
	if len(os.Args) >= 2 && os.Args[1] == "reencrypt-secrets" {
		os.Exit(runReencryptSecrets(os.Args[2:]))
	}
	if len(os.Args) >= 2 && os.Args[1] == "verify-secrets" {
		os.Exit(runVerifySecrets(os.Args[2:]))
	}
	// V2 shadow sync subcommand (Phase 2 PR 21): one sync run + the dated
	// report artifact, dispatched before any server startup path. The §5.4
	// v1 backfill this command once ran first was retired in V2 Phase 6 I-a
	// — register-k8s is the registration path now.
	if len(os.Args) >= 2 && os.Args[1] == "sync-inventory" {
		os.Exit(runSyncInventory(os.Args[2:]))
	}
	// compare-inventory was closed in V2 Phase 6 G1 (D-16): the §15 k8s
	// pairing ended with the C53 verdict, so this dispatch only answers the
	// closure notice — kept so stale invocations fail loudly instead of
	// silently falling through to server startup.
	if len(os.Args) >= 2 && os.Args[1] == "compare-inventory" {
		os.Exit(runCompareInventory(os.Args[2:]))
	}
	// V2 cloud compare subcommand (Phase 4 PR 30b E1): the §15 cloud vm paired
	// run + the dated artifact + the gate, dispatched before any server
	// startup path. Its own dispatch and file — main_compare.go stays frozen
	// until the Phase 2 gate closes (plan §4 CLI freeze).
	if len(os.Args) >= 2 && os.Args[1] == "compare-inventory-cloud" {
		os.Exit(runCompareCloudInventory(os.Args[2:]))
	}
	// V2 Phase 5 (PR 31c N8): the PVE connection registration CLI (plan J6·
	// §3.3) — validate against the provider, upsert the connection/context/
	// secret/binding chain, then exit. Dispatched before any server startup
	// path.
	if len(os.Args) >= 2 && os.Args[1] == "register-pve" {
		os.Exit(runRegisterPVE(os.Args[2:]))
	}
	// V2 Phase 6 H0 (plan §3.2): the Kubernetes cluster registration CLI —
	// validate /version, upsert the connection/context/sealed-kubeconfig/
	// inventory-binding chain, then exit. Dispatched before any server
	// startup path.
	if len(os.Args) >= 2 && os.Args[1] == "register-k8s" {
		os.Exit(runRegisterK8s(os.Args[2:]))
	}
	// The unregister side of register-k8s: delete the --name registration
	// chain in one transaction, guarded by --confirm before any IO.
	if len(os.Args) >= 2 && os.Args[1] == "unregister-k8s" {
		os.Exit(runUnregisterK8s(os.Args[2:]))
	}
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	// G-5 gate: a production startup without any secret key source fails;
	// the development fallback is permitted only when GO_ENV=development is
	// explicit.
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		log.Fatalf("secret key source missing: %v", err)
	}

	db, err := store.NewDB(cfg)
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
	}

	// v2 versioned migration runner (PR 15): fail-closed — a runner failure
	// blocks v1 startup (W-1).
	if err := migrate.Run(context.Background(), db); err != nil {
		log.Fatalf("v2 schema migration failed: %v", err)
	}

	if err := store.AutoMigrate(db); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	if err := store.Seed(db); err != nil {
		log.Fatalf("seed data failed: %v", err)
	}
	if err := store.CanonicalizeSeedLocalization(db); err != nil {
		log.Fatalf("canonicalize seed localization failed: %v", err)
	}

	// V2 Phase 3 (plan M9): the engine lane assembles AFTER the schema and
	// seeds are ready — compose once → engine (J6 hook) → fully-wired v2 API.
	// A stack failure disables this lane alone (R11); v1 boots unchanged.
	taskEngine, v2API, healthSweep, renderMetrics := startEngineLane(db)

	engine, svc := router.New(cfg, db, v2API)
	// §18.2 P6 — the internal metrics scrape route. nil render (stack failure)
	// keeps the route unregistered (claim 9).
	registerMetricsRoute(engine, renderMetrics)
	server := &http.Server{Addr: ":" + cfg.App.Port, Handler: engine, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("start server failed: %v", err)
		}
	}()
	stop, cancelSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignal()
	<-stop.Done()
	// F-7 shutdown order (engine_config.go gracefulShutdown): engine loops
	// stop first — in-flight engine work reaches its terminal commit while
	// the HTTP server still serves — then service cleanup, then the HTTP
	// drain inside the 10s budget.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gracefulShutdown(ctx, taskEngine, svc, server)
	// §18.2 P6 — the health sweep is not an in-flight-work loop (5분 틱 계측
	// 회로), so its stop trails the F-7 order; nil 수신자 Stop은 no-op다.
	healthSweep.Stop()
}
