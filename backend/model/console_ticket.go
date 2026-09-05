package model

import "time"

// ConsoleTicket is the one-time credential that upgrades an authenticated
// session into a single terminal connection. Only the SHA-256 hash of the
// ticket value is stored — the plaintext lives solely in the mint response.
// A row is bound to exactly one resource and one protocol, expires quickly,
// and is consumed by a single atomic UPDATE.
type ConsoleTicket struct {
	ID           string     `json:"id" gorm:"primaryKey;size:32"`
	TicketHash   string     `json:"-" gorm:"size:64;uniqueIndex;not null"`
	ResourceType string     `json:"resourceType" gorm:"size:32;not null"`
	ResourceID   string     `json:"resourceId" gorm:"size:512;not null"`
	Protocol     string     `json:"protocol" gorm:"size:32;not null"`
	UserID       uint       `json:"userId" gorm:"index;not null"`
	ExpiresAt    time.Time  `json:"expiresAt" gorm:"index;not null"`
	ConsumedAt   *time.Time `json:"consumedAt" gorm:"index"`
	CreatedAt    time.Time  `json:"createdAt"`
}

func (ConsoleTicket) TableName() string {
	return "sys_console_ticket"
}
