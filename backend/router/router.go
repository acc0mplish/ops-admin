package router

import (
	"ops-admin/backend/config"
	"ops-admin/backend/controller"
	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/middleware"
	"ops-admin/backend/service"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// v2APIPrefix is the engine-level prefix of the v2 API tree (group root
// /api/v2/infra). The replay/golden tests normalize v2 routes through it
// (M12); the operation table stores the group-suffix paths (/infra/...), so
// the prefix excludes the group name.
const v2APIPrefix = "/api/v2"

// New assembles the full engine. v2API is the injected V2 infra API (plan
// M8/M9): main builds the stack ONCE (compose.Build), assembles the task
// engine over it and hands the fully-wired API in here — the mutation
// handlers then carry the engine instead of degrading 503. nil keeps the
// Phase 2 self-assembly path (compose.Build inside NewInfraAPI) — the
// engine-less boot is byte-for-byte what it was: the v1 tree and the V2 read
// routes are untouched by the engine lane's absence (R11).
func New(cfg *config.Config, db *gorm.DB, v2API *v2.InfraAPI) (*gin.Engine, *service.Service) {
	if cfg.App.Mode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	// Route the tree on the escaped path: V2 infra resource uids are URNs
	// whose tail carries '/' (urn:k8s:<ctx>:workload:<ns>/<kind>/<name>, §8.1),
	// so a %2F-encoded uid segment must stay ONE segment while routing. With
	// gin's default (decoded-path) matching such uids 404 on every
	// /resources/:uid route — surfaced by the Phase 3 Slice A trace E2E
	// (backend/e2e/slicea) against a real kind cluster. UnescapePathValues
	// stays at its default (true): handlers see the decoded uid. Callers that
	// send the slash unescaped still get 404 — escaping is the contract.
	engine.UseRawPath = true
	engine.Use(gin.Logger(), gin.Recovery(), middleware.CORS())
	_ = os.MkdirAll("uploads", 0o755)
	engine.Static("/uploads", "./uploads")

	svc := service.New(db)
	svc.ConfigureCertificate(service.CertificateRuntimeConfig{
		Email:                 cfg.SSL.ACMEEmail,
		ProductionCA:          cfg.SSL.ProductionCA,
		StagingCA:             cfg.SSL.StagingCA,
		DNSPollingSeconds:     cfg.SSL.DNSPollingSeconds,
		DNSPropagationSeconds: cfg.SSL.DNSPropagationSeconds,
		ExpiryWarningDays:     cfg.SSL.ExpiryWarningDays,
	})
	ctl := controller.New(svc)

	engine.GET("/ping", ctl.Ping)

	api := engine.Group("/api/v1")
	{
		api.POST("/login", ctl.Login)
		api.POST("/auth/refresh", ctl.RefreshToken)
		api.POST("/auth/logout", ctl.Logout)
		api.GET("/systemConfig/public", ctl.GetSystemConfig)
		api.GET("/integration/public/:token", ctl.GetPublicIntegrationNavigation)
		api.GET("/asset/terminal/ws", ctl.AssetTerminalWS)
		api.GET("/k8s/pod/terminal/ws", ctl.K8sPodTerminalWS)
	}

	authGroup := api.Group("")
	authGroup.Use(middleware.Auth(db), middleware.OperationLog(db))
	// Authenticated v1 tree — the registration blocks live in
	// routes_v1_system.go, routes_v1_ops.go and routes_v1_infra.go (Phase A
	// pure move): statements are verbatim, only the group receiver name and
	// one indentation level changed. Group construction and the group-level
	// middlewares stay here — the middleware order is locked by the C4 grep.
	registerSystem(authGroup, db, ctl)
	registerOps(authGroup, db, ctl)
	registerInfra(authGroup, db, ctl)

	// V2 infra read API (plan PR 23 — spec §16.1 GET subset, J7): additive
	// group, the /api/v1 tree above stays untouched. GET-only non-sensitive
	// reads — Auth + OperationLog reuse, no opdef grant (sensitive-routes
	// golden stays unchanged).
	v2Group := engine.Group("/api/v2/infra")
	v2Group.Use(middleware.Auth(db), middleware.OperationLog(db))
	registerV2(v2Group, db, v2API)

	return engine, svc
}
