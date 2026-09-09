package opdef

import (
	"net/http"
	"strings"
	"testing"
)

// TestV2InfraDefsRegistered pins the §16.2 v2 non-GET surface (plan N12 /
// 보존 제약 #3): exactly five POST definitions, registered before any route
// may exist. Sensitive-route golden (285→290) and the replay baseline
// (240→245) both count these rows.
func TestV2InfraDefsRegistered(t *testing.T) {
	want := map[string]Def{
		"POST /infra/resources/:uid/operations/:name/plan":    {Permission: "assets:k8s:workload:restart"},
		"POST /infra/resources/:uid/operations/:name/execute": {Permission: "assets:k8s:workload:restart"},
		"POST /infra/tasks/:uid/approve":                      {Permission: "ops:job:approve"},
		"POST /infra/tasks/:uid/reject":                       {Permission: "ops:job:approve"},
		"POST /infra/tasks/:uid/cancel":                       {Permission: "ops:job:approve"},
	}
	byKey := map[string]Def{}
	for _, d := range v2infraDefs {
		byKey[d.Method+" "+d.Path] = d
	}
	if len(v2infraDefs) != len(want) {
		t.Fatalf("v2infraDefs holds %d rows, contract is %d", len(v2infraDefs), len(want))
	}
	for key, wantDef := range want {
		got, ok := byKey[key]
		if !ok {
			t.Fatalf("missing v2 operation definition: %s", key)
		}
		if got.Permission != wantDef.Permission {
			t.Fatalf("%s permission %q, want %q (J4 — existing vocabulary reuse only)", key, got.Permission, wantDef.Permission)
		}
		if !got.Mutating || got.Method != http.MethodPost {
			t.Fatalf("%s must be a mutating POST", key)
		}
		if err := Validate(got); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
}

// vocabularyInheritedFromV1 lists permission strings whose v1 source row Phase
// 6 E1 (2026-09-10) deleted. E1 removed the v1 restart opdef, so the sole owner
// of "assets:k8s:workload:restart" is now the v2 plan/execute def below. This
// is inheritance, not new vocabulary — J4's purpose (the v2 batch invents no
// permission strings) still holds.
var vocabularyInheritedFromV1 = map[string]struct{}{
	"assets:k8s:workload:restart": {},
}

// TestV2InfraDefsReuseExistingVocabulary is the J4 preservation assertion
// (claim 11): the v2 batch introduces zero new permission strings — every
// permission it uses must already exist in the v1 table or be a named
// inheritance from a v1 row E1 removed.
func TestV2InfraDefsReuseExistingVocabulary(t *testing.T) {
	v1 := map[string]struct{}{}
	for _, d := range All() {
		if strings.HasPrefix(d.Path, "/infra/") {
			continue
		}
		for _, permission := range permissionStrings(d) {
			v1[permission] = struct{}{}
		}
	}
	for _, d := range v2infraDefs {
		if _, ok := v1[d.Permission]; !ok {
			if _, inherited := vocabularyInheritedFromV1[d.Permission]; !inherited {
				t.Fatalf("%s %s introduces new permission %q — v2 reuse contract (J4) violated", d.Method, d.Path, d.Permission)
			}
		}
	}
}

// TestV2InfraDefsInAll pins the M11 merge: All() exposes the v2 batch so the
// seeder grants the vocabulary and the artifacts carry the rows.
func TestV2InfraDefsInAll(t *testing.T) {
	count := 0
	for _, d := range All() {
		if strings.HasPrefix(d.Path, "/infra/") {
			count++
		}
	}
	if count != len(v2infraDefs) {
		t.Fatalf("All() carries %d v2 rows, want %d", count, len(v2infraDefs))
	}
}

// TestV2DynamicMiddlewareFallsBackToRepresentative covers the J4 dynamic
// helper: the registry resolution wins when it knows the operation, and the
// opdef representative (golden/seed canonical source) is enforced when it
// does not — enforcement must be visible even pre-registration (zero-grant
// oracle G-4 scans the route regardless of registry state).
func TestV2DynamicMiddlewareFallsBackToRepresentative(t *testing.T) {
	def := Def{Method: http.MethodPost, Path: "/infra/resources/:uid/operations/:name/plan", Permission: "assets:k8s:workload:restart", Mutating: true, Risk: RiskMedium}

	if perm, ok := v2ResolvedPermission(def, func(string) (string, bool) { return "ops:other:verb", true }, "k8s.workload.restart"); !ok || perm != "ops:other:verb" {
		t.Fatalf("registry resolution must win, got %q ok=%v", perm, ok)
	}
	if perm, ok := v2ResolvedPermission(def, func(string) (string, bool) { return "", false }, "unknown.operation"); !ok || perm != "assets:k8s:workload:restart" {
		t.Fatalf("unregistered operation must fall back to the representative, got %q ok=%v", perm, ok)
	}
}
