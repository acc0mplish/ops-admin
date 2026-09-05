package opdef

import "net/http"

// integrationDefs covers the /integration/* section of the authGroup. The
// vocabulary is entirely new (plan §2.4 — the seeded menus hold no
// integration:* values). AI and FinOps resource groups share one permission
// per resource instead of one per action. The normal GET reads stay
// auth-only: the AI model list hides its API key behind json:"-",
// the FinOps account list hides accessKey/secretKey/billingToken, and the
// navigation reads are grouped with the group list, which does return the
// public access token in plaintext.
var integrationDefs = []Def{
	{Method: http.MethodGet, Path: "/integration/navigation/group/list", Permission: "integration:navigation:list", Risk: RiskHigh,
		Redaction: []string{"navigationGroup.publicToken"}},
	{Method: http.MethodPost, Path: "/integration/navigation/group/save", Permission: "integration:navigation:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/navigation/group/delete", Permission: "integration:navigation:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/navigation/group/token", Permission: "integration:navigation:token", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/integration/navigation/list", Permission: "integration:navigation:list", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/integration/navigation/save", Permission: "integration:navigation:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/navigation/delete", Permission: "integration:navigation:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/integration/ai/model/save", Permission: "integration:ai:model", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/ai/model/delete", Permission: "integration:ai:model", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/ai/model/test", Permission: "integration:ai:model", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/ai/conversation/save", Permission: "integration:ai:conversation", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/ai/conversation/delete", Permission: "integration:ai:conversation", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/ai/chat/send", Permission: "integration:ai:conversation", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/ai/knowledge-base/save", Permission: "integration:ai:knowledge", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/ai/knowledge-base/upload", Permission: "integration:ai:knowledge", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/ai/knowledge-base/delete", Permission: "integration:ai:knowledge", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/integration/ai/tool/update", Permission: "integration:ai:tool", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/ai/tool/execute", Permission: "integration:ai:tool", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/ai/action/confirm", Permission: "integration:ai:action", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/ai/action/reject", Permission: "integration:ai:action", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/integration/finops/account/save", Permission: "integration:finops:account", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/finops/account/delete", Permission: "integration:finops:account", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/finops/account/test", Permission: "integration:finops:account", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/finops/recommendation/generate", Permission: "integration:finops:recommendation", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/integration/finops/recommendation/status", Permission: "integration:finops:recommendation", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/integration/finops/recommendation/delete", Permission: "integration:finops:recommendation", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/integration/finops/sync/trigger", Permission: "integration:finops:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/integration/finops/cost/import", Permission: "integration:finops:sync", Mutating: true, Risk: RiskMedium},
}
