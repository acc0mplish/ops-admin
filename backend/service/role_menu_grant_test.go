package service

import (
	"testing"

	"ops-admin/backend/model"
)

// TestAssignRoleMenusPreservesHiddenGrants pins the grant-preservation
// contract: the role menu tree (Role.vue) only ever carries visible menus, so
// a visible-only payload must not erase the hidden route-permission grants
// the v1 migration seeded under the hidden root.
func TestAssignRoleMenusPreservesHiddenGrants(t *testing.T) {
	db := newMenuGuardDB(t)
	pageID, _, markerID := seedMenuGuardRows(t, db)
	role := model.Role{RoleName: "operator", RoleKey: "operator", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	for _, menuID := range []uint{pageID, markerID} {
		if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menuID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{db: db}

	// The UI round trip sends back only what ListMenus showed.
	if err := svc.AssignRoleMenus(RoleMenuPayload{ID: role.ID, MenuIDs: []uint{pageID}}); err != nil {
		t.Fatal(err)
	}

	var ids []uint
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ?", role.ID).Order("menu_id asc").Pluck("menu_id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != pageID || ids[1] != markerID {
		t.Fatalf("grants = %v, want the visible payload %d plus the preserved hidden grant %d", ids, pageID, markerID)
	}
}

// TestAssignRoleMenusStillUpdatesVisibleGrants guards against over-preservation:
// dropping a visible menu from the payload must still revoke its grant, and
// adding one must still insert it.
func TestAssignRoleMenusStillUpdatesVisibleGrants(t *testing.T) {
	db := newMenuGuardDB(t)
	pageID, _, _ := seedMenuGuardRows(t, db)
	extra := model.Menu{ParentID: 0, MenuName: "Reports", MenuType: 1, Value: "reports", MenuStatus: 1}
	if err := db.Create(&extra).Error; err != nil {
		t.Fatal(err)
	}
	role := model.Role{RoleName: "operator", RoleKey: "operator", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: pageID}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}

	// Swap the visible payload: pageID out, extra in.
	if err := svc.AssignRoleMenus(RoleMenuPayload{ID: role.ID, MenuIDs: []uint{extra.ID}}); err != nil {
		t.Fatal(err)
	}
	var ids []uint
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ?", role.ID).Pluck("menu_id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != extra.ID {
		t.Fatalf("grants = %v, want exactly the new payload %d", ids, extra.ID)
	}

	// An empty payload clears the visible grants.
	if err := svc.AssignRoleMenus(RoleMenuPayload{ID: role.ID, MenuIDs: []uint{}}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ?", role.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("empty payload must clear the grants, got %d", count)
	}
}

// TestAssignRoleMenusDoesNotDuplicatePayloadGrants: a hidden menu id that
// reaches the payload anyway (out-of-band caller) must not produce a second
// grant row next to the preserved one.
func TestAssignRoleMenusDoesNotDuplicatePayloadGrants(t *testing.T) {
	db := newMenuGuardDB(t)
	_, _, markerID := seedMenuGuardRows(t, db)
	role := model.Role{RoleName: "operator", RoleKey: "operator", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: markerID}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}

	if err := svc.AssignRoleMenus(RoleMenuPayload{ID: role.ID, MenuIDs: []uint{markerID, markerID}}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id = ?", role.ID, markerID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("hidden grant must stay a single row, got %d", count)
	}
}
