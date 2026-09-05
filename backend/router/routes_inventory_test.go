package router

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/config"
	"ops-admin/backend/opdef"
	"ops-admin/backend/store"
)

// updateArtifacts rewrites the committed docs/security artifacts from the
// live table (golden-file regeneration; the usual flow is to review the diff).
var updateArtifacts = flag.Bool("update", false, "rewrite the committed docs/security artifacts from the live table")

// newArtifactEngine boots the real router against a single-connection
// in-memory sqlite database (M-2), with the full migration and seed run first
// so the permission middleware queries behave exactly as in production.
func newArtifactEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("OPS_ADMIN_INITIAL_PASSWORD", "inventory-test-password")
	t.Setenv("OPS_ADMIN_JWT_SECRET", "inventory-test-jwt-secret")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := store.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := store.Seed(db); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	engine, svc := New(cfg, db)
	t.Cleanup(func() {
		_ = svc.Shutdown(context.Background())
		// router.New creates the uploads dir relative to the test working
		// directory; keep the repository tree clean.
		_ = os.RemoveAll("uploads")
	})
	return engine, db
}

// routeInventoryLines serializes every registered route — no filtering
// (H-1/M-3): the static uploads pair and public routes stay in the dump —
// sorted so the output is deterministic.
func routeInventoryLines(engine *gin.Engine) []string {
	lines := make([]string, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		lines = append(lines, fmt.Sprintf("%s %s -> %s", route.Method, route.Path, route.Handler))
	}
	sort.Strings(lines)
	return lines
}

// sensitiveRouteLines renders the opdef operation table in the artifact
// format of plan §2.2, sorted so table declaration order cannot leak into the
// bytes (M-1 determinism contract).
func sensitiveRouteLines(defs []opdef.Def) []string {
	lines := make([]string, 0, len(defs))
	for _, d := range defs {
		lines = append(lines, opdef.FormatLine(d))
	}
	sort.Strings(lines)
	return lines
}

func artifactHeader(method string, regenerate string, count int) []string {
	return []string{
		"# " + method + " — do not edit by hand.",
		"# " + regenerate,
		fmt.Sprintf("# %d entries.", count),
	}
}

// writeOrCompareArtifact regenerates the artifact when -update is passed and
// always compares against the committed file — the byte-level golden diff
// that gates route drift (G-3).
func writeOrCompareArtifact(t *testing.T, name string, header []string, lines []string) {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "security", name)
	content := strings.Join(append(header, lines...), "\n") + "\n"
	if *updateArtifacts {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("artifact docs/security/%s missing — regenerate with: go test ./router/ -run TestRouteInventoryArtifact -update (%v)", name, err)
	}
	if string(committed) != content {
		t.Fatalf("artifact docs/security/%s drifted from the live table — regenerate with -update and review the diff", name)
	}
}

// TestRouteInventoryArtifact is T12: the committed route-inventory.txt must be
// byte-identical to a fresh dump of the live engine, and the body must hold
// exactly 437 routes (H-1: 1 ping + 7 public + 427 authGroup + 2 uploads
// static — gin's Static registers GET and HEAD).
func TestRouteInventoryArtifact(t *testing.T) {
	engine, _ := newArtifactEngine(t)
	lines := routeInventoryLines(engine)
	if len(lines) != 437 {
		t.Fatalf("route inventory holds %d routes, contract is 437", len(lines))
	}
	header := artifactHeader(
		"Route inventory generated from the live gin engine",
		"Regenerate with: cd backend && go test ./router/ -run TestRouteInventoryArtifact -update",
		len(lines),
	)
	writeOrCompareArtifact(t, "route-inventory.txt", header, lines)
}

// TestSensitiveRoutesArtifact is T13: the committed sensitive-routes.txt must
// be byte-identical to a fresh render of the opdef table, and shuffling the
// table's declaration order must not change a single byte (M-1 determinism).
func TestSensitiveRoutesArtifact(t *testing.T) {
	defs := opdef.All()
	lines := sensitiveRouteLines(defs)
	if len(lines) != len(defs) {
		t.Fatalf("sensitive artifact holds %d lines for %d definitions", len(lines), len(defs))
	}
	header := artifactHeader(
		"Sensitive-route classification generated from backend/opdef",
		"Regenerate with: cd backend && go test ./router/ -run TestSensitiveRoutesArtifact -update",
		len(lines),
	)

	shuffled := slices.Clone(defs)
	random := rand.New(rand.NewSource(1))
	random.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	if reshuffled := sensitiveRouteLines(shuffled); !slices.Equal(reshuffled, lines) {
		t.Fatal("sensitive artifact bytes depend on table declaration order — the generator must sort")
	}
	writeOrCompareArtifact(t, "sensitive-routes.txt", header, lines)
}
