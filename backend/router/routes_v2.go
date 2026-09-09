package router

import (
	"ops-admin/backend/internal/api/v2"
	"ops-admin/backend/opdef"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerV2 mounts the V2 infra API (read routes and operation routes) on
// the v2 group. Signature note (Phase A, team-lead adjudication — plan T-11
// wording superseded): the function receives the already-constructed v2
// group, not the engine — group construction and group-level middleware
// attachment remain in router.go (plan §3.2 ":535,536 불변"; the C4 grep
// contract covers the whole directory). Body moved verbatim from
// router.go:537-543 (nil fallback + Register + RegisterOperations).
func registerV2(v2Group *gin.RouterGroup, db *gorm.DB, v2API *v2.InfraAPI) {
	// V2 Phase 3 (plan M8): the §16.2 mutation surface joins the same group —
	// five non-GET POSTs (every one opdef-registered; sensitive golden
	// 285→290, replay baseline 240→245) plus three reads. Grants are dynamic
	// (J4): V2DynamicMiddleware resolves the registry def's RequiredPermission
	// for the operations routes and falls back to the opdef representative —
	// ops:job:approve for the task verbs, whose route has no :name — so the
	// enforced vocabulary stays registry-canonical without a second grant
	// path. The engine is injected through v2API (main's startEngineLane —
	// plan M9); a nil injection keeps the Phase 2 self-assembly below and the
	// mutation handlers degrade 503 (R11: the engine lane never gates v1).
	if v2API == nil {
		v2API = v2.NewInfraAPI(db)
	}
	v2API.Register(v2Group)
	v2API.RegisterOperations(v2Group, func(def opdef.Def) gin.HandlerFunc {
		return opdef.V2DynamicMiddleware(db, def, v2API.ResolveOperationPermission)
	})
}
