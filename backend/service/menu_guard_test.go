package service

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
	"ops-admin/backend/store"
)

// newMenuGuardDB opens a single-connection in-memory sqlite database migrated
// with only the sys_menu / grant tables the menu-management path touches.
func newMenuGuardDB(t *testing.T) *gorm.DB {
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

// seedMenuGuardRows creates the minimal sys_menu shape the route-permission
// seed produces: one visible page menu, the hidden status-0 root, one route
// permission leaf under it and the one-shot migration marker leaf.
func seedMenuGuardRows(t *testing.T, db *gorm.DB) (pageID, rootID, markerID uint) {
	t.Helper()
	page := model.Menu{ParentID: 0, MenuName: "Dashboard", MenuType: 1, URL: "/dashboard", Value: "dashboard", MenuStatus: 1}
	if err := db.Create(&page).Error; err != nil {
		t.Fatal(err)
	}
	root := model.Menu{ParentID: 0, MenuName: "Route Permissions", MenuType: 1, Value: store.RoutePermissionsRootValue, MenuStatus: 0, Sort: 99}
	if err := db.Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	leaf := model.Menu{ParentID: root.ID, MenuName: "Host Terminal", MenuType: 3, Value: "assets:host:terminal", MenuStatus: 1}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatal(err)
	}
	marker := model.Menu{ParentID: root.ID, MenuName: "Route Permissions Granted (v1)", MenuType: 3, Value: store.RoutePermissionsMarkerValue, MenuStatus: 1}
	if err := db.Create(&marker).Error; err != nil {
		t.Fatal(err)
	}
	return page.ID, root.ID, marker.ID
}

// TestListMenusHidesSeededRoutePermissionSubtree locks the menu-management
// visibility contract: the hidden route-permissions root (and its subtree,
// including the one-shot marker) never reaches the menu management UI.
func TestListMenusHidesSeededRoutePermissionSubtree(t *testing.T) {
	db := newMenuGuardDB(t)
	pageID, _, _ := seedMenuGuardRows(t, db)
	svc := &Service{db: db}

	list, err := svc.ListMenus()
	if err != nil {
		t.Fatal(err)
	}
	for _, menu := range list {
		if menu.Value == store.RoutePermissionsRootValue || menu.Value == store.RoutePermissionsMarkerValue {
			t.Fatalf("hidden seeded menu leaked into ListMenus: %q", menu.Value)
		}
		if menu.ParentID != 0 {
			t.Fatalf("hidden subtree descendant leaked into ListMenus: id=%d parent=%d value=%q", menu.ID, menu.ParentID, menu.Value)
		}
	}
	if len(list) != 1 || list[0].ID != pageID {
		t.Fatalf("ListMenus = %+v, want only the page menu id=%d", list, pageID)
	}
}

// TestUpdateMenuBlocksProtectedRows: renaming the marker (or the hidden root)
// would orphan the one-shot migration marker and re-grant every role its full
// route vocabulary on the next boot — the update must be refused.
func TestUpdateMenuBlocksProtectedRows(t *testing.T) {
	db := newMenuGuardDB(t)
	_, rootID, markerID := seedMenuGuardRows(t, db)
	svc := &Service{db: db}

	for _, id := range []uint{rootID, markerID} {
		err := svc.UpdateMenu(MenuPayload{ID: id, ParentID: 0, MenuName: "Renamed", Value: "renamed", MenuType: 1, MenuStatus: 1})
		if !errors.Is(err, ErrProtectedSystemMenu) {
			t.Fatalf("UpdateMenu on protected menu id=%d returned %v, want ErrProtectedSystemMenu", id, err)
		}
		var row model.Menu
		if err := db.First(&row, id).Error; err != nil {
			t.Fatal(err)
		}
		if row.Value != store.RoutePermissionsRootValue && row.Value != store.RoutePermissionsMarkerValue {
			t.Fatalf("protected menu id=%d was mutated: value=%q", id, row.Value)
		}
	}
}

// TestDeleteMenuBlocksProtectedRows: deleting the marker or the hidden root
// makes migrateRoleRoutePermissionsOnce re-run and re-grant every existing
// role the full route vocabulary — the delete must be refused and leave the
// marker row plus its grants untouched.
func TestDeleteMenuBlocksProtectedRows(t *testing.T) {
	db := newMenuGuardDB(t)
	_, rootID, markerID := seedMenuGuardRows(t, db)
	svc := &Service{db: db}

	role := model.Role{RoleName: "operator", RoleKey: "operator", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: markerID}).Error; err != nil {
		t.Fatal(err)
	}

	for _, id := range []uint{rootID, markerID} {
		if err := svc.DeleteMenu(id); !errors.Is(err, ErrProtectedSystemMenu) {
			t.Fatalf("DeleteMenu on protected menu id=%d returned %v, want ErrProtectedSystemMenu", id, err)
		}
	}
	var count int64
	if err := db.Model(&model.Menu{}).Where("value IN ?", []string{store.RoutePermissionsRootValue, store.RoutePermissionsMarkerValue}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("protected menu rows survived count=%d, want 2", count)
	}
	var grants int64
	if err := db.Model(&model.RoleMenu{}).Where("menu_id = ?", markerID).Count(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != 1 {
		t.Fatalf("marker grant rows survived count=%d, want 1", grants)
	}
}

// TestMenuMutationsStillAllowedForOperatorMenus guards against over-blocking:
// ordinary menus must remain fully editable and deletable.
func TestMenuMutationsStillAllowedForOperatorMenus(t *testing.T) {
	db := newMenuGuardDB(t)
	pageID, _, _ := seedMenuGuardRows(t, db)
	svc := &Service{db: db}

	if err := svc.UpdateMenu(MenuPayload{ID: pageID, ParentID: 0, MenuName: "Dashboard v2", Value: "dashboard", MenuType: 1, URL: "/dashboard", MenuStatus: 1}); err != nil {
		t.Fatalf("UpdateMenu on an ordinary menu must succeed, got %v", err)
	}
	if err := svc.DeleteMenu(pageID); err != nil {
		t.Fatalf("DeleteMenu on an ordinary menu must succeed, got %v", err)
	}
}

// seedHiddenLeaf adds one non-marker route-permission leaf under the hidden
// root — the shape the seed produces for every granted route.
func seedHiddenLeaf(t *testing.T, db *gorm.DB, rootID uint, value string) uint {
	t.Helper()
	leaf := model.Menu{ParentID: rootID, MenuName: "Hidden Leaf", MenuType: 3, Value: value, MenuStatus: 1}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatal(err)
	}
	return leaf.ID
}

// TestUpdateMenuBlocksWholeHiddenSubtree extends the protection to the root's
// descendants: flipping a leaf's menu_status, rewriting its route value or
// re-parenting it to ParentID 0 would make the hidden route vocabulary
// visible or break the grant lookup — every descendant mutation is refused.
func TestUpdateMenuBlocksWholeHiddenSubtree(t *testing.T) {
	db := newMenuGuardDB(t)
	_, rootID, _ := seedMenuGuardRows(t, db)
	leafID := seedHiddenLeaf(t, db, rootID, "assets:host:terminal")
	svc := &Service{db: db}

	for _, payload := range []MenuPayload{
		{ID: leafID, ParentID: rootID, MenuName: "Hidden Leaf", Value: "hijacked:value", MenuType: 3, MenuStatus: 1},
		{ID: leafID, ParentID: 0, MenuName: "Hidden Leaf", Value: "assets:host:terminal", MenuType: 3, MenuStatus: 1},
		{ID: leafID, ParentID: rootID, MenuName: "Hidden Leaf", Value: "assets:host:terminal", MenuType: 3, MenuStatus: 0},
	} {
		if err := svc.UpdateMenu(payload); !errors.Is(err, ErrProtectedSystemMenu) {
			t.Fatalf("UpdateMenu on hidden leaf (status=%d parent=%d value=%q) returned %v, want ErrProtectedSystemMenu", payload.MenuStatus, payload.ParentID, payload.Value, err)
		}
	}
	var row model.Menu
	if err := db.First(&row, leafID).Error; err != nil {
		t.Fatal(err)
	}
	if row.ParentID != rootID || row.Value != "assets:host:terminal" || row.MenuStatus != 1 {
		t.Fatalf("hidden leaf was mutated: %+v", row)
	}
}

// TestDeleteMenuBlocksHiddenSubtree: deleting a hidden leaf would drop a
// route permission grant target while the grants themselves survive — refuse
// the delete and keep the row.
func TestDeleteMenuBlocksHiddenSubtree(t *testing.T) {
	db := newMenuGuardDB(t)
	_, rootID, _ := seedMenuGuardRows(t, db)
	leafID := seedHiddenLeaf(t, db, rootID, "assets:host:terminal")
	svc := &Service{db: db}

	if err := svc.DeleteMenu(leafID); !errors.Is(err, ErrProtectedSystemMenu) {
		t.Fatalf("DeleteMenu on hidden leaf returned %v, want ErrProtectedSystemMenu", err)
	}
	var count int64
	if err := db.Model(&model.Menu{}).Where("id = ?", leafID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("hidden leaf must survive the refused delete, got count=%d", count)
	}
}

// TestGetMenuBlocksHiddenRows: the menu/info endpoint must not serve the raw
// hidden rows even when their ID is known — the management UI contract is
// that these rows do not exist for operators.
func TestGetMenuBlocksHiddenRows(t *testing.T) {
	db := newMenuGuardDB(t)
	_, rootID, markerID := seedMenuGuardRows(t, db)
	leafID := seedHiddenLeaf(t, db, rootID, "assets:host:terminal")
	svc := &Service{db: db}

	for _, id := range []uint{rootID, markerID, leafID} {
		if _, err := svc.GetMenu(id); !errors.Is(err, ErrProtectedSystemMenu) {
			t.Fatalf("GetMenu on hidden id=%d returned %v, want ErrProtectedSystemMenu", id, err)
		}
	}
}

// TestCreateMenuRejectsProtectedValues: a created menu must not impersonate
// the hidden root or the one-shot marker value, or the boot re-migration
// would misread the infrastructure rows.
func TestCreateMenuRejectsProtectedValues(t *testing.T) {
	db := newMenuGuardDB(t)
	svc := &Service{db: db}

	for _, value := range []string{store.RoutePermissionsRootValue, store.RoutePermissionsMarkerValue} {
		err := svc.CreateMenu(MenuPayload{ParentID: 0, MenuName: "Impostor", Value: value, MenuType: 1, MenuStatus: 1})
		if !errors.Is(err, ErrProtectedSystemMenu) {
			t.Fatalf("CreateMenu with protected value %q returned %v, want ErrProtectedSystemMenu", value, err)
		}
	}
	var count int64
	if err := db.Model(&model.Menu{}).Where("value IN ?", []string{store.RoutePermissionsRootValue, store.RoutePermissionsMarkerValue}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("no protected-value row may be created, got %d", count)
	}
}
