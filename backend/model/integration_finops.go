package model

import "time"

type IntegrationFinOpsAccount struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	Name              string     `json:"name" gorm:"size:128;not null;index"`
	Provider          string     `json:"provider" gorm:"size:32;not null;index"`
	AccountIdentifier string     `json:"accountIdentifier" gorm:"size:128;index"`
	AccessKey         string     `json:"-" gorm:"size:512"`
	SecretKey         string     `json:"-" gorm:"size:1024"`
	Region            string     `json:"region" gorm:"size:128;index"`
	Currency          string     `json:"currency" gorm:"size:16;not null;default:CNY"`
	BillingEndpoint   string     `json:"billingEndpoint" gorm:"size:2048"`
	BillingToken      string     `json:"-" gorm:"size:2048"`
	SyncEnabled       bool       `json:"syncEnabled" gorm:"not null;default:false;index"`
	SyncFrequency     string     `json:"syncFrequency" gorm:"size:32;not null;default:daily"`
	Status            int        `json:"status" gorm:"not null;default:1;index"`
	LastSyncAt        *time.Time `json:"lastSyncAt"`
	NextSyncAt        *time.Time `json:"nextSyncAt" gorm:"index"`
	Description       string     `json:"description" gorm:"size:500"`
	CreatedAt         time.Time  `json:"createTime"`
	UpdatedAt         time.Time  `json:"updateTime"`
}

func (IntegrationFinOpsAccount) TableName() string { return "integration_finops_account" }

type IntegrationFinOpsCostRecord struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	AccountID      uint      `json:"accountId" gorm:"not null;index;uniqueIndex:uk_finops_cost_record,priority:1"`
	Provider       string    `json:"provider" gorm:"size:32;not null;index"`
	ExternalID     string    `json:"externalId" gorm:"size:191;not null;uniqueIndex:uk_finops_cost_record,priority:2"`
	BillingDate    time.Time `json:"billingDate" gorm:"not null;index"`
	Service        string    `json:"service" gorm:"size:191;index"`
	Region         string    `json:"region" gorm:"size:128;index"`
	ResourceID     string    `json:"resourceId" gorm:"size:191;index"`
	ResourceName   string    `json:"resourceName" gorm:"size:191;index"`
	ResourceType   string    `json:"resourceType" gorm:"size:128;index"`
	ResourceConfig string    `json:"resourceConfig" gorm:"size:255"`
	Tags           string    `json:"tags" gorm:"type:text"`
	Amount         float64   `json:"amount" gorm:"type:decimal(20,6);not null;default:0"`
	OriginalPrice  float64   `json:"originalPrice" gorm:"type:decimal(20,6);not null;default:0"`
	Discount       float64   `json:"discount" gorm:"type:decimal(20,6);not null;default:0"`
	ActualPayment  float64   `json:"actualPayment" gorm:"type:decimal(20,6);not null;default:0"`
	Currency       string    `json:"currency" gorm:"size:16;not null;default:CNY"`
	UsageQuantity  float64   `json:"usageQuantity" gorm:"type:decimal(20,6);not null;default:0"`
	UsageUnit      string    `json:"usageUnit" gorm:"size:64"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (IntegrationFinOpsCostRecord) TableName() string { return "integration_finops_cost_record" }

type IntegrationFinOpsRecommendation struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	AccountID         uint      `json:"accountId" gorm:"not null;index"`
	Provider          string    `json:"provider" gorm:"size:32;not null;index"`
	Category          string    `json:"category" gorm:"size:64;index"`
	Strategy          string    `json:"strategy" gorm:"size:32;index"`
	ModelName         string    `json:"modelName" gorm:"size:128"`
	AnalysisMonth     string    `json:"analysisMonth" gorm:"size:7;index"`
	AnalysisAccountID uint      `json:"analysisAccountId" gorm:"index"`
	Priority          string    `json:"priority" gorm:"size:16;index"`
	Title             string    `json:"title" gorm:"size:255;not null"`
	Description       string    `json:"description" gorm:"type:text"`
	ResourceID        string    `json:"resourceId" gorm:"size:191;index"`
	CurrentCost       float64   `json:"currentCost" gorm:"type:decimal(20,6);not null;default:0"`
	Saving            float64   `json:"saving" gorm:"type:decimal(20,6);not null;default:0"`
	Status            string    `json:"status" gorm:"size:32;not null;default:open;index"`
	CreatedAt         time.Time `json:"createTime"`
	UpdatedAt         time.Time `json:"updateTime"`
}

func (IntegrationFinOpsRecommendation) TableName() string {
	return "integration_finops_recommendation"
}

type IntegrationFinOpsSyncLog struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	AccountID    uint       `json:"accountId" gorm:"not null;index"`
	Provider     string     `json:"provider" gorm:"size:32;not null;index"`
	TriggerType  string     `json:"triggerType" gorm:"size:32;not null;index"`
	BillingMonth string     `json:"billingMonth" gorm:"size:7;index"`
	Status       string     `json:"status" gorm:"size:32;not null;index"`
	StartedAt    time.Time  `json:"startedAt" gorm:"not null;index"`
	FinishedAt   *time.Time `json:"finishedAt"`
	RecordCount  int        `json:"recordCount" gorm:"not null;default:0"`
	TotalAmount  float64    `json:"totalAmount" gorm:"type:decimal(20,6);not null;default:0"`
	// Source* describes the provider response. RecordCount and TotalAmount are
	// the final persisted monthly snapshot, the same basis used by the dashboard.
	SourceRecordCount int       `json:"sourceRecordCount" gorm:"not null;default:0"`
	SourceTotalAmount float64   `json:"sourceTotalAmount" gorm:"type:decimal(20,6);not null;default:0"`
	DeduplicatedCount int       `json:"deduplicatedCount" gorm:"not null;default:0"`
	SnapshotVerified  bool      `json:"snapshotVerified" gorm:"not null;default:false"`
	Message           string    `json:"message" gorm:"type:text"`
	CreatedAt         time.Time `json:"createTime"`
}

func (IntegrationFinOpsSyncLog) TableName() string { return "integration_finops_sync_log" }
