package migrate

import (
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// step0001InfraFoundation creates the M1 infrastructure foundation tables via
// model AutoMigrate (plan §2 PR 16, file 4): the 4 base models (§7.2–7.5) and
// the 4 inventory models (§8.1/8.2/8.4/§9.1). The step is frozen with these 8
// models at PR 16 merge (W-4); any later schema change lands as step 0004+.
// The infra_resource identity unique UNIQUE(context, kind, external_urn) is
// tag-synthesized by AutoMigrate (uq_infra_resource_identity) — no raw DDL
// reinforcement here (r2 C / E-3a).
var step0001InfraFoundation = Step{
	Version: 1,
	Name:    "infra_foundation",
	Run: func(db *gorm.DB) error {
		return db.AutoMigrate(
			&model.ProviderConnection{},
			&model.ProviderContext{},
			&model.ProviderCredentialBinding{},
			&model.SecretRef{},
			&model.InfraResource{},
			&model.ResourceObservation{},
			&model.InventorySyncRun{},
			&model.InfraRelationship{},
		)
	},
}
