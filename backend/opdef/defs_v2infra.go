package opdef

import "net/http"

// v2infraDefs is the /api/v2/infra non-GET operation batch (plan §3.4, N12 —
// 보존 제약 #3: no mutation route without a table row). Permission vocabulary
// is strictly reused (J4): plan/execute carry the restart permission
// "assets:k8s:workload:restart" as the REPRESENTATIVE value — the enforced
// permission is resolved per request from the registry OperationDefinition
// (V2DynamicMiddleware); the representative is the canonical source for the
// sensitive-routes golden and the seeder. Sole owner of that vocabulary since
// Phase 6 E1 removed the v1 restart def (2026-09-10). Approve/reject/cancel reuse
// "ops:job:approve" verbatim (J4 — 승인자 인구 동일, 신규 문자열 0).
//
// The three GET v2 routes (GET resources/:uid/operations, GET tasks/:uid,
// GET tasks/:uid/events) are deliberately absent: non-sensitive reads join
// the read group without an opdef grant, same posture as the Phase 2 reads.
//
// I10 J1c adds the §16.1 connection-scoped create pair: same representative
// mechanism, but the registry def (k8s.resource.create) resolves to
// "assets:k8s:workload:yaml" at risk=high — the enforced values come from
// V2DynamicMiddleware at request time, as everywhere above.
var v2infraDefs = []Def{
	{Method: http.MethodPost, Path: "/infra/provider-connections/:uid/operations/:name/plan", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/infra/provider-connections/:uid/operations/:name/execute", Permission: "assets:k8s:workload:yaml", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/infra/resources/:uid/operations/:name/plan", Permission: "assets:k8s:workload:restart", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/infra/resources/:uid/operations/:name/execute", Permission: "assets:k8s:workload:restart", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/infra/tasks/:uid/approve", Permission: "ops:job:approve", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/infra/tasks/:uid/reject", Permission: "ops:job:approve", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/infra/tasks/:uid/cancel", Permission: "ops:job:approve", Mutating: true, Risk: RiskMedium},
}
