package migrate

import (
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// step0005AuditExtend — version 5: the §18.1 audit EXTEND (V2 Phase 3, plan
// §3.5 — J3, N5). The named triplet (task_uid, policy_version, mutating —
// risk_level reuses the existing v1 column) plus the v2_context JSON carrier
// land as plain model columns/tags, so AutoMigrate carries them on both
// dialects — no raw DDL postlude is needed. All four are nullable pointers:
// existing v1 rows keep NULL (F-8 ①), and the J3 trade-off stands — task_uid
// is the only real column (task↔audit join, claim 8's access path), the
// remaining §18.1 fields ride in v2_context and are asserted key-by-key after
// a Go-side JSON decode (F-8 ②).
//
// The v2 record path itself is internal/api/v2/audit.go (request middleware +
// terminal helper): v1 middleware/operation_log.go stays byte-untouched.
//
// Postlude contract (W-4/W-5): step 0005 touches sys_operation_log ONLY —
// it does not touch provider_task, so the EnsureProviderTaskGuards postlude
// duty does not arise. 0000–0004 remain frozen.
var step0005AuditExtend = Step{
	Version: 5,
	Name:    "audit_extend",
	Run: func(db *gorm.DB) error {
		if err := db.AutoMigrate(&model.OperationLog{}); err != nil {
			return fmt.Errorf("migrate: step 0005 AutoMigrate sys_operation_log: %w", err)
		}
		return nil
	},
}
