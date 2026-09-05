package opdef

import "net/http"

// domainDefs migrates the 36 pre-existing domains grants byte-identically from
// router.go (preservation constraint 1 — the strings, the grant shapes and the
// deny behaviour are unchanged; only their declaration home moves here, so the
// sensitive-route artifacts and the seed migration can consume one source).
// TestDomainDefsSnapshot pins them against the r1 baseline.
var domainDefs = []Def{
	{Method: http.MethodGet, Path: "/domain/public/accounts", Permission: "domains:account:list", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/accounts/options", AnyOf: []string{"domains:account:list", "domains:public:list", "domains:ssl:view"}, Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/accounts/detail", Permission: "domains:account:list", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/public/accounts/save", CreateEdit: [2]string{"domains:account:add", "domains:account:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/domain/public/accounts/delete", Permission: "domains:account:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/domain/public/accounts/test", Permission: "domains:account:test", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/domain/public/domains", Permission: "domains:public:list", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/public/domains/sync", Permission: "domains:public:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/domain/public/records", AnyOf: []string{"domains:public:list", "domains:public:record"}, Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/public/records/mutate", Permission: "domains:public:record", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/public/records/batch", Permission: "domains:public:batch", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/domain/internal/settings", AnyOf: []string{"domains:settings:view", "domains:internal:list", "domains:query:test"}, Risk: RiskLow},
	{Method: http.MethodPut, Path: "/domain/internal/settings", Permission: "domains:settings:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/domain/internal/zones", Permission: "domains:internal:list", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/internal/zones/save", CreateEdit: [2]string{"domains:internal:zone:add", "domains:internal:zone:edit"}, Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/domain/internal/zones/delete", Permission: "domains:internal:zone:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodGet, Path: "/domain/internal/records", AnyOf: []string{"domains:internal:list", "domains:internal:record"}, Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/internal/records/save", Permission: "domains:internal:record", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/domain/internal/records/delete", Permission: "domains:internal:record", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPost, Path: "/domain/internal/records/batch", Permission: "domains:internal:record", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/internal/query", Permission: "domains:query:test", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodGet, Path: "/domain/audit", Permission: "domains:audit:list", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates", Permission: "domains:ssl:view", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates/detail", Permission: "domains:ssl:view", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates/domain-options", Permission: "domains:ssl:view", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates/tasks", Permission: "domains:ssl:view", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates/audits", Permission: "domains:ssl:view", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/domain/public/certificates/sync", Permission: "domains:ssl:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/public/certificates/upload", Permission: "domains:ssl:upload", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/public/certificates/apply", Permission: "domains:ssl:apply", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/public/certificates/renew", Permission: "domains:ssl:renew", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/domain/public/certificates/resync", Permission: "domains:ssl:sync", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/domain/public/certificates/renew-settings", Permission: "domains:ssl:settings", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/domain/public/certificates/delete", Permission: "domains:ssl:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodGet, Path: "/domain/public/certificates/download", Permission: "domains:ssl:download", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/domain/public/certificates/download-private", Permission: "domains:ssl:download-key", Risk: RiskLow},
}
