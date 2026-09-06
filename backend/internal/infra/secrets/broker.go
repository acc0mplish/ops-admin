// Package secrets holds the v2 credential broker: the only component allowed
// to turn a secret_ref row back into plaintext material, gated on the §7.4
// credential-binding purposes. The M1 vocabulary lives here; §3.5 hands its
// promotion into contract constants to the §3.6 extension (PR 17).
package secrets

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/util"
)

// CredentialPurposes is the closed §7.4 purpose vocabulary (M1).
var CredentialPurposes = []string{
	"inventory",
	"operations",
	"billing",
	"console",
	"monitoring",
	"backup",
}

func knownCredentialPurpose(p string) bool {
	for _, candidate := range CredentialPurposes {
		if candidate == p {
			return true
		}
	}
	return false
}

// Broker resolves purpose-scoped credential material from SecretRef rows.
// M1 internal backend: Ciphertext is always a v2 envelope (§4.2); the broker
// reads through util.DecryptSecretV2 only — the same implementation the
// migration tool used (§4.3). Results are never cached (short-lived = a
// per-call view, §1 J7). Secrets are never serialized through model JSON.
type Broker struct{ db *gorm.DB }

// NewBroker returns a broker reading through db.
func NewBroker(db *gorm.DB) *Broker {
	return &Broker{db: db}
}

// ResolvedSecret is one per-call view of decrypted material. Value is plaintext
// and must never be logged, serialized, or embedded in an error message.
type ResolvedSecret struct {
	SecretRefUID string
	Purpose      string // §7.4 어휘로 정합 검사된 값
	Value        string // 평문 material — 로깅·직렬화 금지
}

// Resolve finds the binding for (connectionUID, purpose), loads the bound
// SecretRef, decrypts, and returns one-time material. Purpose mismatch and
// v2-envelope parse failure are hard errors (UNKNOWN never falls through,
// §4.3).
func (b *Broker) Resolve(ctx context.Context, connectionUID, purpose string) (ResolvedSecret, error) {
	if !knownCredentialPurpose(purpose) {
		return ResolvedSecret{}, fmt.Errorf("secrets: purpose %q is outside the §7.4 vocabulary", purpose)
	}

	var conn model.ProviderConnection
	if err := b.db.WithContext(ctx).Where("uid = ?", connectionUID).First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedSecret{}, fmt.Errorf("secrets: connection %q not found", connectionUID)
		}
		return ResolvedSecret{}, fmt.Errorf("secrets: load connection: %w", err)
	}

	var binding model.ProviderCredentialBinding
	err := b.db.WithContext(ctx).
		Where("provider_connection_id = ? AND purpose = ?", conn.ID, purpose).
		Order("id").
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedSecret{}, fmt.Errorf("secrets: no credential binding for connection %q with purpose %q", connectionUID, purpose)
		}
		return ResolvedSecret{}, fmt.Errorf("secrets: load credential binding: %w", err)
	}

	var ref model.SecretRef
	if err := b.db.WithContext(ctx).First(&ref, binding.SecretRefID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedSecret{}, fmt.Errorf("secrets: bound secret_ref %d not found", binding.SecretRefID)
		}
		return ResolvedSecret{}, fmt.Errorf("secrets: load secret_ref: %w", err)
	}

	plain, err := util.DecryptSecretV2(ref.Ciphertext)
	if err != nil {
		// Hard error — no legacy or plaintext fallback (§4.3). The wrapped
		// error carries key-id metadata only, never material.
		return ResolvedSecret{}, fmt.Errorf("secrets: decrypt secret_ref %q: %w", ref.UID, err)
	}

	return ResolvedSecret{
		SecretRefUID: ref.UID,
		Purpose:      binding.Purpose,
		Value:        plain,
	}, nil
}
