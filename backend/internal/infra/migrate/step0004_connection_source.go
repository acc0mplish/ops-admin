package migrate

import (
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// step0004ConnectionSource — version 4: the provider_connection source-
// provenance Expand (§5.4 propagation rules; plan PR 21, J3). The four
// columns (source_model, source_id, source_updated_at, stale_source) and the
// composite index (source_model, source_id) are plain model columns/tags, so
// AutoMigrate carries them on both dialects — no raw DDL postlude is needed.
//
// Postlude contract (W-4/W-5): step 0004 touches provider_connection ONLY —
// it does not touch provider_task, so the EnsureProviderTaskGuards postlude
// duty does not arise. 0000–0003 remain frozen.
var step0004ConnectionSource = Step{
	Version: 4,
	Name:    "connection_source",
	Run: func(db *gorm.DB) error {
		if err := db.AutoMigrate(&model.ProviderConnection{}); err != nil {
			return fmt.Errorf("migrate: step 0004 AutoMigrate provider_connection: %w", err)
		}
		return nil
	},
}
