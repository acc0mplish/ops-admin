package model

import "time"

// SecretRef is spec §7.5 verbatim. Ciphertext is always a v2 envelope
// (§4.2; Step 4 completed so the legacy format is retired) — the secrets
// broker is the only decryptor and reads through util.DecryptSecretV2 only.
type SecretRef struct { // §7.5 verbatim — Ciphertext는 항상 v2 envelope(§4.2; Step 4 완료로 legacy 폐기)
	ID         uint
	UID        string `gorm:"size:64;not null;uniqueIndex"`
	Backend    string `gorm:"size:32;not null;default:internal"` // internal (M1); vault etc. deferred
	Path       string `gorm:"size:255"`
	Version    string `gorm:"size:64"`
	KeyID      string `gorm:"size:64"` // v2 envelope key id (§4.2)
	Ciphertext string `gorm:"type:text;not null"`
	RotatedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SecretRef) TableName() string { return "secret_ref" }
