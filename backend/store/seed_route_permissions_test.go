package store

import (
	"slices"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
	"ops-admin/backend/opdef"
)

// newSeedTestDB opens a single-connection in-memory sqlite database and runs
// the full AutoMigrate plus the real Seed against it (M-2 single connection;
// the R2 probe proved the full 90-model migration works on sqlite).
func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("OPS_ADMIN_INITIAL_PASSWORD", "seed-test-password")
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
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// routePermissionMenus returns the sys_menu rows holding route permission
// values.
func routePermissionMenus(t *testing.T, db *gorm.DB) []model.Menu {
	t.Helper()
	permissions := opdef.PermissionStrings()
	if len(permissions) == 0 {
		t.Fatal("opdef vocabulary is empty")
	}
	var menus []model.Menu
	if err := db.Where("value IN ?", permissions).Find(&menus).Error; err != nil {
		t.Fatal(err)
	}
	return menus
}

// countRouteGrants counts the route-permission grants one role holds through
// the same join the permission middleware uses.
func countRouteGrants(t *testing.T, db *gorm.DB, roleID uint) int64 {
	t.Helper()
	var count int64
	if err := db.Table("sys_role_menu rm").
		Joins("JOIN sys_menu m ON m.id=rm.menu_id").
		Where("rm.role_id = ? AND m.value IN ? AND m.menu_status = 1", roleID, opdef.PermissionStrings()).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// missingRouteGrants lists route permissions a role has no grant for.
func missingRouteGrants(t *testing.T, db *gorm.DB, roleID uint) []string {
	t.Helper()
	menus := routePermissionMenus(t, db)
	held := map[uint]bool{}
	var grants []model.RoleMenu
	if err := db.Where("role_id = ?", roleID).Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	for _, grant := range grants {
		held[grant.MenuID] = true
	}
	missing := []string{}
	for _, menu := range menus {
		if menu.MenuStatus == 1 && !held[menu.ID] {
			missing = append(missing, menu.Value)
		}
	}
	return missing
}

func markerRow(t *testing.T, db *gorm.DB) (model.Menu, bool) {
	t.Helper()
	var marker model.Menu
	err := db.Where("value = ?", routePermissionsMarkerValue).First(&marker).Error
	if err != nil {
		return marker, false
	}
	return marker, true
}

func findRole(t *testing.T, db *gorm.DB, roleKey string) model.Role {
	t.Helper()
	var role model.Role
	if err := db.Where("role_key = ?", roleKey).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}

func createRoleRow(t *testing.T, db *gorm.DB, roleKey string) model.Role {
	t.Helper()
	role := model.Role{RoleName: roleKey, RoleKey: roleKey, Status: 1, Description: "test role", CreatedAt: time.Now()}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}

// TestRoutePermissionSeedCreatesMenus is T6: after Seed every route permission
// string exists in sys_menu with menu_status 1; newly created rows are type-3
// leaves under the hidden status-0 root, while pre-existing page menus keep
// their seeded type-2 shape untouched (preservation constraint 7).
func TestRoutePermissionSeedCreatesMenus(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	menus := routePermissionMenus(t, db)
	if len(menus) != len(opdef.PermissionStrings()) {
		t.Fatalf("sys_menu holds %d of %d route permission values", len(menus), len(opdef.PermissionStrings()))
	}
	for _, menu := range menus {
		if menu.MenuStatus != 1 {
			t.Fatalf("route permission %q has menu_status %d, want 1", menu.Value, menu.MenuStatus)
		}
	}
	var root model.Menu
	if err := db.Where("value = ?", routePermissionsRootValue).First(&root).Error; err != nil {
		t.Fatal(err)
	}
	if root.MenuStatus != 0 {
		t.Fatalf("hidden root menu_status = %d, want 0 (sidebar must never see it)", root.MenuStatus)
	}
	for _, menu := range menus {
		if menu.ID == root.ID {
			continue
		}
		if menu.ParentID == root.ID && menu.MenuType != 3 {
			t.Fatalf("hidden-root permission %q has menu_type %d, want 3", menu.Value, menu.MenuType)
		}
	}
	// the seeded page menu reused as a route permission keeps its original
	// row untouched (type-2 page shape, preservation constraint 7)
	var pageMenu model.Menu
	if err := db.Where("value = ?", "monitor:query").First(&pageMenu).Error; err != nil {
		t.Fatal(err)
	}
	if pageMenu.MenuType != 2 || pageMenu.ParentID == root.ID {
		t.Fatalf("reused page menu monitor:query must stay a seeded type-2 page row, got type=%d parent=%d", pageMenu.MenuType, pageMenu.ParentID)
	}
}

// TestFirstSeedGrantsEveryExistingRoleAndCreatesMarker is T7: the first Seed
// grants the full route vocabulary to every existing role (here the seeded
// super-admin) and creates the one-shot marker row.
func TestFirstSeedGrantsEveryExistingRoleAndCreatesMarker(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	super := findRole(t, db, "super-admin")
	if missing := missingRouteGrants(t, db, super.ID); len(missing) > 0 {
		t.Fatalf("super-admin missing %d route grants after first seed: %v", len(missing), missing)
	}
	marker, ok := markerRow(t, db)
	if !ok {
		t.Fatal("one-shot marker row was not created")
	}
	if marker.MenuType != 3 || marker.MenuStatus != 1 {
		t.Fatalf("marker row shape wrong: menu_type=%d menu_status=%d, want 3/1", marker.MenuType, marker.MenuStatus)
	}
}

// TestSeedIsIdempotent is T8: once the boot sequence has fully converged,
// running Seed must not change menu, grant, or marker row counts — in
// particular the marker must never be duplicated. The first re-run after the
// migration boot lets the pre-existing super-admin all-menu regrant pick up
// the two rows our steps added (hidden root + marker), which is the existing
// step's semantics; stability from the next boot on is what is contractual.
func TestSeedIsIdempotent(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	counts := func() (menus, grants, markers int64) {
		if err := db.Model(&model.Menu{}).Count(&menus).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&model.RoleMenu{}).Count(&grants).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&model.Menu{}).Where("value = ?", routePermissionsMarkerValue).Count(&markers).Error; err != nil {
			t.Fatal(err)
		}
		return menus, grants, markers
	}
	menus, grants, _ := counts()
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	menus2, grants2, markers2 := counts()
	if menus2 != menus || grants2 != grants {
		t.Fatalf("seed not idempotent: menus %d->%d, grants %d->%d", menus, menus2, grants, grants2)
	}
	if markers2 != 1 {
		t.Fatalf("marker rows after three seeds = %d, want 1", markers2)
	}
}

// TestNewRoleAfterSeedGetsZeroRouteGrants is T9 (CR-1a): roles created after
// the one-shot migration must start with zero route permissions.
func TestNewRoleAfterSeedGetsZeroRouteGrants(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	operator := createRoleRow(t, db, "operator")
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if got := countRouteGrants(t, db, operator.ID); got != 0 {
		t.Fatalf("newly created role holds %d route grants, want 0", got)
	}
}

// TestRevokedGrantSurvivesReseed is T10 (CR-1b): a route grant an operator
// revoked must not be restored by re-running Seed.
func TestRevokedGrantSurvivesReseed(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	operator := createRoleRow(t, db, "operator")
	menus := routePermissionMenus(t, db)
	slices.SortFunc(menus, func(a, b model.Menu) int {
		switch {
		case a.Value < b.Value:
			return -1
		case a.Value > b.Value:
			return 1
		default:
			return 0
		}
	})
	if err := db.Create(&model.RoleMenu{RoleID: operator.ID, MenuID: menus[0].ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("role_id = ? AND menu_id = ?", operator.ID, menus[0].ID).Delete(&model.RoleMenu{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if got := countRouteGrants(t, db, operator.ID); got != 0 {
		t.Fatalf("revoked grant restored by reseed: role holds %d route grants, want 0", got)
	}
}

// TestSuperAdminRegrantCoversRecreatedMenuOnlyForSuperAdmin is T11: a route
// permission menu recreated after the fact reaches super-admin (always-on
// regrant) but never ordinary roles (one-shot marker gate).
func TestSuperAdminRegrantCoversRecreatedMenuOnlyForSuperAdmin(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	operator := createRoleRow(t, db, "operator")
	menus := routePermissionMenus(t, db)
	slices.SortFunc(menus, func(a, b model.Menu) int {
		switch {
		case a.Value < b.Value:
			return -1
		case a.Value > b.Value:
			return 1
		default:
			return 0
		}
	})
	victim := menus[0]
	if err := db.Where("menu_id = ?", victim.ID).Delete(&model.RoleMenu{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.Menu{}, victim.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	var recreated model.Menu
	if err := db.Where("value = ?", victim.Value).First(&recreated).Error; err != nil {
		t.Fatalf("recreated route menu missing after reseed: %v", err)
	}
	super := findRole(t, db, "super-admin")
	var superHolds int64
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id = ?", super.ID, recreated.ID).Count(&superHolds).Error; err != nil {
		t.Fatal(err)
	}
	if superHolds == 0 {
		t.Fatalf("super-admin must hold the recreated route menu %q", victim.Value)
	}
	if got := countRouteGrants(t, db, operator.ID); got != 0 {
		t.Fatalf("ordinary role holds %d route grants after reseed, want 0", got)
	}
}
