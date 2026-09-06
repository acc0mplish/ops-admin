package middleware

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// newPermissionTestDB opens a single-connection in-memory sqlite database
// migrated with only the grant/role/menu tables the authorization query joins.
func newPermissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
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
	if err := db.AutoMigrate(&model.Menu{}, &model.Role{}, &model.RoleMenu{}, &model.AdminRole{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestAdminHasAnyPermissionRequiresActiveRole: a member of a disabled
// sys_role must not pass the authorization query even while the
// role_menu → menu grant chain is intact (LOW-6).
func TestAdminHasAnyPermissionRequiresActiveRole(t *testing.T) {
	db := newPermissionTestDB(t)

	menu := model.Menu{MenuName: "Host Terminal", MenuType: 3, Value: "assets:host:terminal", MenuStatus: 1}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	role := model.Role{RoleName: "operator", RoleKey: "operator", Status: 0}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	// The gorm default:1 tag silently turns a zero-value create into an
	// enabled role, so the disabled state is written explicitly here.
	if err := db.Model(&model.Role{}).Where("id = ?", role.ID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AdminRole{AdminID: 7, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}

	if adminHasAnyPermission(db, 7, []string{"assets:host:terminal"}) {
		t.Fatal("disabled sys_role must not grant a permission")
	}

	if err := db.Model(&model.Role{}).Where("id = ?", role.ID).Update("status", 1).Error; err != nil {
		t.Fatal(err)
	}
	if !adminHasAnyPermission(db, 7, []string{"assets:host:terminal"}) {
		t.Fatal("enabled sys_role with an intact grant chain must pass")
	}
	if AdminHasPermission(db, 7, "assets:host:terminal") != true {
		t.Fatal("AdminHasPermission wrapper must agree with the any-permission query")
	}
	if AdminHasPermission(db, 7, " ") || AdminHasPermission(db, 7, "") {
		t.Fatal("blank permission strings must never pass")
	}
}
