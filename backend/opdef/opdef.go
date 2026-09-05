// Package opdef is the single source of truth for operation definitions:
// every sensitive route's permission strings, mutation flag, risk rating and
// the known plaintext secret response fields recorded for later redaction
// work. The router attaches opdef.Middleware to sensitive routes, the seeder
// grants opdef permission menus to roles, and the docs/security artifacts are
// generated from this table. Permission strings are never declared inline in
// the router.
package opdef

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"ops-admin/backend/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Risk ratings recorded per definition. They are metadata for later policy
// work — permission enforcement only reads the permission strings.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Def is one operation definition. Exactly one of Permission, AnyOf and
// CreateEdit must be set; Mutating must equal (Method != GET).
type Def struct {
	Method     string
	Path       string
	Permission string    // single required permission (RequiredPermission)
	AnyOf      []string  // any-one-of set for shared read endpoints
	CreateEdit [2]string // {create, edit} for shared save endpoints
	Mutating   bool
	Risk       string
	Redaction  []string // secret response fields returned in plaintext (redaction ledger, not enforced here)
}

// permissionPattern is the permission vocabulary: 2-4 lowercase segments
// starting with [a-z] and continuing with [a-z0-9-], joined by colons
// (e.g. assets:database:sql:execute, domains:ssl:download-key). The hyphen is
// required by the pre-existing "download-key" seed vocabulary that must stay
// byte-identical (preservation constraint 1).
var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?::[a-z][a-z0-9-]*){1,3}$`)

// All returns the full operation table. Callers must treat the result as
// read-only; the physical order of the underlying tables is not contractual —
// artifact generators sort their output (determinism contract M-1).
func All() []Def {
	defs := make([]Def, 0, len(domainDefs)+len(systemDefs)+len(assetDefs)+len(integrationDefs)+len(opsDefs)+len(monitorDefs))
	defs = append(defs, domainDefs...)
	defs = append(defs, systemDefs...)
	defs = append(defs, assetDefs...)
	defs = append(defs, integrationDefs...)
	defs = append(defs, opsDefs...)
	defs = append(defs, monitorDefs...)
	return defs
}

// Validate checks one definition against the table invariants (T2/T3/T4).
func Validate(d Def) error {
	if d.Path == "" || !strings.HasPrefix(d.Path, "/") {
		return fmt.Errorf("path %q must be non-empty and start with /", d.Path)
	}
	switch d.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return fmt.Errorf("method %q must be one of GET|POST|PUT|DELETE", d.Method)
	}
	specified := 0
	if d.Permission != "" {
		specified++
	}
	if len(d.AnyOf) > 0 {
		specified++
	}
	if d.CreateEdit != [2]string{} {
		specified++
	}
	if specified != 1 {
		return fmt.Errorf("exactly one of Permission/AnyOf/CreateEdit must be set, got %d", specified)
	}
	for _, permission := range permissionStrings(d) {
		if !permissionPattern.MatchString(permission) {
			return fmt.Errorf("permission %q violates the vocabulary pattern", permission)
		}
	}
	if d.Mutating != (d.Method != http.MethodGet) {
		return fmt.Errorf("Mutating=%v must equal (Method != GET) for %s", d.Mutating, d.Method)
	}
	switch d.Risk {
	case RiskLow, RiskMedium, RiskHigh:
	default:
		return fmt.Errorf("risk %q must be one of low|medium|high", d.Risk)
	}
	return nil
}

// permissionStrings flattens whichever permission shape the definition uses.
func permissionStrings(d Def) []string {
	switch {
	case d.Permission != "":
		return []string{d.Permission}
	case len(d.AnyOf) > 0:
		return d.AnyOf
	default:
		return d.CreateEdit[:]
	}
}

// Middleware turns a definition into the gin middleware that enforces it by
// delegating to the untouched permission middleware. This is the only place
// route-level permission checks originate.
func Middleware(db *gorm.DB, d Def) gin.HandlerFunc {
	switch {
	case d.Permission != "":
		return middleware.RequirePermission(db, d.Permission)
	case len(d.AnyOf) > 0:
		return middleware.RequireAnyPermission(db, d.AnyOf...)
	default:
		return middleware.RequireCreateOrEditPermission(db, d.CreateEdit[0], d.CreateEdit[1])
	}
}

// FormatLine renders the sensitive-routes artifact line for one definition
// (plan §2.2). AnyOf/CreateEdit sets are joined with "|".
func FormatLine(d Def) string {
	permissions := strings.Join(permissionStrings(d), "|")
	return fmt.Sprintf("%s %s\tpermission=%s\tmutating=%s\trisk=%s\treduction=%s",
		d.Method, d.Path, permissions, strconv.FormatBool(d.Mutating), d.Risk, strconv.FormatBool(len(d.Redaction) > 0))
}
