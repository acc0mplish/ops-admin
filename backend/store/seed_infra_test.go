package store

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"ops-admin/backend/model"
)

// infraMenuValues are the PR 23 Infrastructure menu rows (plan §17.3 —
// spec-explicit seed.go exception). The shell is read-only v2 read pages, so
// no type-3 button rows exist for them.
var infraMenuValues = []string{"infra", "infra:overview", "infra:providers", "infra:resources"}

// TestInfraMenuSeedIdempotent is T53 (plan §6): the Infrastructure menu seed
// rows land once, reseeding changes nothing (ensureMenu upsert-only), and the
// pre-existing menu set is untouched by the new block.
func TestInfraMenuSeedIdempotent(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	var before []model.Menu
	if err := db.Order("id").Find(&before).Error; err != nil {
		t.Fatal(err)
	}
	beforeByValue := map[string]model.Menu{}
	for _, menu := range before {
		beforeByValue[menu.Value] = menu
	}
	for _, value := range infraMenuValues {
		if _, ok := beforeByValue[value]; !ok {
			t.Fatalf("infra menu row missing after first seed: %s", value)
		}
	}

	if err := Seed(db); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	var after []model.Menu
	if err := db.Order("id").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("reseed changed the menu row count: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Value != after[i].Value {
			t.Fatalf("reseed changed existing menu row %d: %s -> %s", before[i].ID, before[i].Value, after[i].Value)
		}
	}

	// The infrastructure root is parented at the tree root and the three
	// children hang under it (§17 Infrastructure section shape).
	root := beforeByValue["infra"]
	if root.ParentID != 0 || root.URL != "/infra" {
		t.Fatalf("unexpected infra root row: %+v", root)
	}
	for value, url := range map[string]string{
		"infra:overview":  "/infra/overview",
		"infra:providers": "/infra/providers",
		"infra:resources": "/infra/resources",
	} {
		child := beforeByValue[value]
		if child.ParentID != root.ID || child.URL != url {
			t.Fatalf("unexpected infra child row %s: %+v", value, child)
		}
	}
}

// TestInfraI18nParity is the T53 i18n half: infra-i18n.js must define flat ko
// and en blocks with identical key sets. scripts/check-i18n-parity.mjs is the
// standing gate for the whole dictionary catalog; this test pins the new
// dictionary's key parity inside the go test suite the phase gates run.
func TestInfraI18nParity(t *testing.T) {
	raw, err := os.ReadFile("../../web/src/utils/infra-i18n.js")
	if err != nil {
		t.Fatalf("read infra-i18n.js: %v", err)
	}
	source := string(raw)
	koKeys := dictKeys(source, "ko")
	enKeys := dictKeys(source, "en")
	if len(koKeys) == 0 || len(enKeys) == 0 {
		t.Fatalf("infra-i18n.js ko/en blocks missing or empty (ko=%d en=%d)", len(koKeys), len(enKeys))
	}
	for key := range koKeys {
		if !enKeys[key] {
			t.Errorf("en key missing: %s", key)
		}
	}
	for key := range enKeys {
		if !koKeys[key] {
			t.Errorf("ko key missing: %s", key)
		}
	}
}

// dictKeys extracts the top-level identifier keys of `const <name> = { ... }`
// in the flat single-line-entry shape every web/src/utils/*-i18n.js dict
// module uses (`key: 'value',` lines until the closing brace).
func dictKeys(source, name string) map[string]bool {
	marker := "const " + name + " = {"
	start := strings.Index(source, marker)
	if start < 0 {
		return nil
	}
	keys := map[string]bool{}
	entryRe := regexp.MustCompile(`^\s*([A-Za-z0-9_$]+)\s*:`)
	for _, line := range strings.Split(source[start+len(marker):], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "}") {
			break
		}
		if match := entryRe.FindStringSubmatch(line); match != nil {
			keys[match[1]] = true
		}
	}
	return keys
}
