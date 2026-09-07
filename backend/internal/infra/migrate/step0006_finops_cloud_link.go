package migrate

import (
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// step0006FinopsCloudLink — version 6: the §5.4 finops link EXTEND (V2 Phase
// 4 C1, plan §3.3 — J4, N13). integration_finops_account gains a nullable
// provider_connection_uid: the §5.4 propagation (RunCloudAccountBackfill)
// points it at the backfilled provider_connection whose billing binding
// carries the collapsed credential, and SaveFinOpsAccount clears it on a v1
// credential rotation. The column lands as a plain model column/tag, so
// AutoMigrate carries it on both dialects — no raw DDL postlude. Nullable by
// design: existing v1 rows keep NULL and keep reading their own v1 columns
// (the rewire's dual-transition window, J5).
//
// Postlude contract: step 0006 touches integration_finops_account ONLY — no
// V2 core table (preservation constraint #5), so no EnsureProviderTaskGuards
// duty arises. 0000–0005 remain frozen.
var step0006FinopsCloudLink = Step{
	Version: 6,
	Name:    "finops_cloud_link",
	Run: func(db *gorm.DB) error {
		if err := db.AutoMigrate(&model.IntegrationFinOpsAccount{}); err != nil {
			return fmt.Errorf("migrate: step 0006 AutoMigrate integration_finops_account: %w", err)
		}
		return nil
	},
}
