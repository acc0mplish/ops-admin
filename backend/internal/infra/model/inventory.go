package model

import (
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// InfraResource is spec §8.1 verbatim. Identity rule UNIQUE(context, kind,
// external_urn) is expressed as the GORM tag-synthesized composite unique
// uq_infra_resource_identity (r2 C / E-3a — tag composites survive
// re-AutoMigrate; raw DDL indexes carry no such preservation guarantee).
type InfraResource struct { // §8.1 verbatim
	ID             uint
	UID            string           `gorm:"size:64;not null;uniqueIndex"`
	ContextID      uint             `gorm:"not null;uniqueIndex:uq_infra_resource_identity,priority:1"`
	Kind           string           `gorm:"size:64;not null;uniqueIndex:uq_infra_resource_identity,priority:2"` // contract.IsKnownResourceKind
	Subtype        string           `gorm:"size:128"`
	ExternalID     string           `gorm:"size:255;index"`
	ExternalURN    string           `gorm:"size:512;not null;uniqueIndex:uq_infra_resource_identity,priority:3"`
	Name           string           `gorm:"size:255"`
	DisplayName    string           `gorm:"size:255"`
	LifecycleState string           `gorm:"size:32"`
	HealthState    string           `gorm:"size:32"`
	ManagedState   string           `gorm:"size:32;default:discovered"` // discovered, imported, managed, orphaned, tombstoned
	LabelsJSON     contract.JSONMap `gorm:"serializer:json;type:text"`
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
	DeletedAt      *time.Time `gorm:"index"`
}

// ResourceObservation is spec §8.2 verbatim.
type ResourceObservation struct { // §8.2 verbatim
	ID                uint
	ResourceID        uint             `gorm:"not null;index:idx_obs_res_time"`
	GenerationUID     string           `gorm:"size:64;not null;index"`
	ObservationHash   string           `gorm:"size:128"`
	NormalizerVersion string           `gorm:"size:64"`
	NormalizedJSON    contract.JSONMap `gorm:"serializer:json;type:text"`
	RawJSON           contract.JSONMap `gorm:"serializer:json;type:text"`
	ObservedAt        time.Time        `gorm:"index:idx_obs_res_time,sort:desc"` // (resource_id, observed_at DESC) §8.2
}

// InfraRelationship is spec §8.4 verbatim — FK to infra_resource,
// cross-context 지원.
type InfraRelationship struct { // §8.4 verbatim — FK to infra_resource, cross-context 지원
	ID             uint
	FromResourceID uint             `gorm:"not null;index"`
	ToResourceID   uint             `gorm:"not null;index"`
	Type           string           `gorm:"size:64;not null"` // runs_on, backed_by, deployed_to, points_to (§8.4 예시)
	Source         string           `gorm:"size:32;not null"` // discovery, user, policy, application_binding
	GenerationUID  string           `gorm:"size:64;not null;index"`
	AttributesJSON contract.JSONMap `gorm:"serializer:json;type:text"`
	LastSeenAt     time.Time
}

// InventorySyncRun is spec §9.1 verbatim.
type InventorySyncRun struct { // §9.1 verbatim
	ID           uint
	UID          string `gorm:"size:64;not null;uniqueIndex"`
	ConnectionID uint   `gorm:"not null;index"`
	ContextID    uint   `gorm:"not null;index"`
	Mode         string `gorm:"size:32;not null"` // full, incremental, targeted
	Status       string `gorm:"size:32;not null;index"`
	Cursor       string `gorm:"type:text"`
	SeenCount    int
	CreatedCount int
	UpdatedCount int
	MissingCount int
	ErrorCode    string `gorm:"size:128"`
	StartedAt    time.Time
	CommittedAt  *time.Time
	FinishedAt   *time.Time
}

func (InfraResource) TableName() string { return "infra_resource" }

func (ResourceObservation) TableName() string { return "resource_observation" }

func (InfraRelationship) TableName() string { return "infra_relationship" }

func (InventorySyncRun) TableName() string { return "inventory_sync_run" }
