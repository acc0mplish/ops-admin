package model

import (
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// ProviderConnection is spec §7.2 verbatim. M1 model set — plan phase1 §3.2.
type ProviderConnection struct { // §7.2 verbatim
	ID             uint             `json:"id" gorm:"primaryKey"`
	UID            string           `json:"uid" gorm:"size:64;not null;uniqueIndex"`
	ProviderType   string           `json:"providerType" gorm:"size:64;not null;index"`
	Name           string           `json:"name" gorm:"size:128;not null"`
	Endpoint       string           `json:"endpoint" gorm:"size:255;not null"`
	GatewayID      *uint            `json:"gatewayId" gorm:"index"`             // legacy AssetGateway FK until AccessRoute (M2)
	TLSProfile     contract.JSONMap `json:"-" gorm:"serializer:json;type:text"` // CA/verify posture; no secret material
	ConfigJSON     contract.JSONMap `json:"configJson" gorm:"serializer:json;type:text"`
	Status         string           `json:"status" gorm:"size:32;index"`
	Version        string           `json:"version" gorm:"size:64"`
	CapabilityHash string           `json:"capabilityHash" gorm:"size:128"`
	LastHealthAt   *time.Time       `json:"lastHealthAt"`
	CreatedAt      time.Time        `json:"createTime"`
	UpdatedAt      time.Time        `json:"updateTime"`
}

func (ProviderConnection) TableName() string { return "provider_connection" }

// ProviderContext is spec §7.3 verbatim — ParentID 없음(§7.3 명시).
type ProviderContext struct { // §7.3 verbatim — ParentID 없음(§7.3 명시)
	ID           uint
	UID          string           `gorm:"size:64;not null;uniqueIndex"`
	ConnectionID uint             `gorm:"not null;index"`
	Kind         string           `gorm:"size:64;not null"` // contract.ProviderContextKinds
	ExternalID   string           `gorm:"size:255;index"`
	Name         string           `gorm:"size:128"`
	Status       string           `gorm:"size:32"`
	MetadataJSON contract.JSONMap `gorm:"serializer:json;type:text"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ProviderContext) TableName() string { return "provider_context" }

// ProviderCredentialBinding is spec §7.4 verbatim. Purpose draws from the
// closed §7.4 vocabulary — the secrets broker (internal/infra/secrets) is the
// only reader, gated on this column.
type ProviderCredentialBinding struct { // §7.4 verbatim
	ID                   uint
	ProviderConnectionID uint   `gorm:"not null;index"`
	ProviderContextID    *uint  `gorm:"index"`
	Purpose              string `gorm:"size:32;not null;index"` // inventory, operations, billing, console, monitoring, backup (§7.4 주석)
	SecretRefID          uint   `gorm:"not null;index"`
	Status               string `gorm:"size:32"`
	LastValidatedAt      *time.Time
}

func (ProviderCredentialBinding) TableName() string { return "provider_credential_binding" }
