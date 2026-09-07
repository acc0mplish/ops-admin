package store

import (
	"errors"
	"fmt"
	"time"

	"ops-admin/backend/model"
	"ops-admin/backend/opdef"

	"gorm.io/gorm"
)

// Route/permission seeding vocabulary and seeders — split from seed.go
// (p5 followup, 650-line rule). Bodies are moved verbatim; Seed() step
// order in seed.go is unchanged.

const (
	// RoutePermissionsRootValue is the hidden root menu holding route
	// permissions that have no seeded page menu. It carries menu_status 0, so
	// the sidebar export (menu_status = 1 AND menu_type IN (1,2)) never shows
	// it, while the grant lookup only reads the leaf's status. Exported so the
	// service layer can keep operator mutations away from this system row.
	RoutePermissionsRootValue = "route-permissions"
	// RoutePermissionsMarkerValue is the one-shot migration marker: it exists
	// in sys_menu once every existing role has been granted the route
	// permissions, and its presence makes the migration a complete no-op.
	// Exported so the service layer can keep operator mutations away from it —
	// losing the marker would re-run migrateRoleRoutePermissionsOnce and
	// re-grant every role its full route vocabulary.
	RoutePermissionsMarkerValue = "route-permissions:granted:v1"
	// PVEGuestOperatePermission is the one PVE guest-operation permission the
	// three proxmox opdefs share (compose PVEGuestOperatePermission — plan
	// phase5 E-2). It is declared here (not in opdef) because the opdef table
	// is intentionally unchanged in this phase — zero prior code, zero new
	// route vocabulary — and grantRoutePermissionMenus reads the opdef
	// vocabulary only, so this permission cannot ride that path.
	PVEGuestOperatePermission = "infra:pve:guest:operate"
)

func seedSuperRolePermissions(db *gorm.DB) error {
	var role model.Role
	if err := db.Where("role_key = ?", "super-admin").First(&role).Error; err != nil {
		return err
	}
	var menus []model.Menu
	if err := db.Find(&menus).Error; err != nil {
		return err
	}
	for _, menu := range menus {
		var count int64
		if err := db.Model(&model.RoleMenu{}).Where("role_id = ? and menu_id = ?", role.ID, menu.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedRoutePermissionMenus (always-on) ensures every route permission string
// in the opdef table has a status-1 sys_menu row the grant tables can point
// at. Values that already exist (seeded page menus and buttons) are reused
// untouched — their row is never modified; values with no seeded row are
// created as type-3 leaves under the hidden status-0 route-permissions root.
func seedRoutePermissionMenus(db *gorm.DB) error {
	root, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: "Route Permissions", MenuType: 1, URL: "", Value: RoutePermissionsRootValue, MenuStatus: 0, Sort: 99})
	if err != nil {
		return err
	}
	// The menu model declares gorm default:1, so a create with the zero value
	// would be stored as status 1; the hidden root must stay status 0.
	if root.MenuStatus != 0 {
		if err := db.Model(&model.Menu{}).Where("id = ?", root.ID).Update("menu_status", 0).Error; err != nil {
			return err
		}
	}
	for i, permission := range opdef.PermissionStrings() {
		var count int64
		if err := db.Model(&model.Menu{}).Where("value = ?", permission).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		name := canonicalPermissionName(permission)
		if name == "" {
			name = "Permission"
		}
		menu := model.Menu{ParentID: root.ID, MenuName: name, MenuType: 3, Value: permission, MenuStatus: 1, Sort: i + 1, CreatedAt: time.Now()}
		if err := db.Create(&menu).Error; err != nil {
			return err
		}
	}
	return nil
}

// grantRoutePermissionMenus grants every status-1 route permission menu to one
// role, skipping grants that already exist.
func grantRoutePermissionMenus(db *gorm.DB, roleID uint) error {
	var menus []model.Menu
	if err := db.Where("value IN ? AND menu_status = 1", opdef.PermissionStrings()).Find(&menus).Error; err != nil {
		return err
	}
	for _, menu := range menus {
		var count int64
		if err := db.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id = ?", roleID, menu.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&model.RoleMenu{RoleID: roleID, MenuID: menu.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedSuperAdminRoutePermissions (always-on) re-grants the full route
// vocabulary to super-admin on every boot, keeping the existing
// "super-admin holds everything" invariant when new definitions appear later.
func seedSuperAdminRoutePermissions(db *gorm.DB) error {
	var role model.Role
	if err := db.Where("role_key = ?", "super-admin").First(&role).Error; err != nil {
		return err
	}
	return grantRoutePermissionMenus(db, role.ID)
}

// seedPVEGuestOperatePermission (always-on, plan phase5 M8 — r1.4 HIGH-1
// 재계약) creates the sys_menu leaf for the PVE guest-operation permission and
// grants it to super-admin. grantRoutePermissionMenus cannot cover it: that
// grant selects menus by the opdef vocabulary (value IN opdef.PermissionStrings)
// and the opdef table is unchanged in this phase, so without this seeder the
// permission would have no menu row at all and adminHasAnyPermission's join on
// sys_menu.value would reject every PVE operation. Same idempotent
// count-then-create shape as seedRoutePermissionMenus — reruns skip existing
// rows and leave row counts unchanged.
func seedPVEGuestOperatePermission(db *gorm.DB) error {
	var root model.Menu
	if err := db.Where("value = ?", RoutePermissionsRootValue).First(&root).Error; err != nil {
		return fmt.Errorf("seed pve permission: route-permissions root missing (seedRoutePermissionMenus must run first): %w", err)
	}
	var leaf model.Menu
	err := db.Where("value = ?", PVEGuestOperatePermission).First(&leaf).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		leaf = model.Menu{ParentID: root.ID, MenuName: "PVE Guest Operate", MenuType: 3, Value: PVEGuestOperatePermission, MenuStatus: 1, Sort: 1000, CreatedAt: time.Now()}
		if err := db.Create(&leaf).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var role model.Role
	if err := db.Where("role_key = ?", "super-admin").First(&role).Error; err != nil {
		return err
	}
	var count int64
	if err := db.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id = ?", role.ID, leaf.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: leaf.ID}).Error
}

// migrateRoleRoutePermissionsOnce (one-shot, marker-gated) grants the full
// route vocabulary to every role that exists at migration time and then
// creates the marker row. With the marker present it is a complete no-op, so
// roles created afterwards start with zero route permissions and operator
// revocations survive reboots. The grant pass and the marker creation run in
// one transaction so a failure cannot leave the router enforcement without
// the corresponding grants.
func migrateRoleRoutePermissionsOnce(db *gorm.DB) error {
	var marker model.Menu
	err := db.Where("value = ?", RoutePermissionsMarkerValue).First(&marker).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var roles []model.Role
		if err := tx.Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			if err := grantRoutePermissionMenus(tx, role.ID); err != nil {
				return err
			}
		}
		var root model.Menu
		if err := tx.Where("value = ?", RoutePermissionsRootValue).First(&root).Error; err != nil {
			return err
		}
		return tx.Create(&model.Menu{ParentID: root.ID, MenuName: "Route Permissions Granted (v1)", MenuType: 3, Value: RoutePermissionsMarkerValue, MenuStatus: 1, CreatedAt: time.Now()}).Error
	})
}
