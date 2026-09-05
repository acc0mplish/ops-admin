package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
)

// v2EnvelopePrefix is the version tag of the v2 secret envelope:
//
//	v2:<key_id>:<base64url(nonce || ciphertext)>
//
// key_id names the master key inside the configured key set, which makes
// rotation self-describing: the reader selects the key by ID, never by trial.
// The ciphertext layout (base64url(nonce‖ct), AES-256-GCM, key = sha256(seed))
// is identical to the legacy envelope in secret.go; only the version prefix
// and key routing are new.
const v2EnvelopePrefix = "v2:"

// legacyMasterKeyID is the reserved key id of the implicit key set entry that
// carries the pre-migration credential seed chain. The id is rejected in
// OPS_SECRET_MASTER_KEYS because the entry is always derived, never configured.
const legacyMasterKeyID = "legacy"

// SecretFormat is the classification bucket of a stored secret value.
type SecretFormat string

const (
	// FormatNotSecret: the field is outside the §4.1 inventory, or the value
	// lives in a mixed-declaration column whose owning declaration says it is
	// not a secret (NOT_SECRET-by-declaration).
	FormatNotSecret SecretFormat = "not_secret"
	// FormatEmpty: NULL or empty string; optional secrets are legitimate.
	FormatEmpty SecretFormat = "empty"
	// FormatV2: well-formed v2 envelope with a registered key id.
	FormatV2 SecretFormat = "v2"
	// FormatLegacy: legacy envelope that decrypts with the legacy key.
	FormatLegacy SecretFormat = "legacy"
	// FormatPlaintext: P-class value without an envelope (pre-migration).
	FormatPlaintext SecretFormat = "plaintext"
	// FormatUnknown: claims neither format. Never falls through to plaintext;
	// runtime reads fail closed and migration halts for quarantine.
	FormatUnknown SecretFormat = "unknown"
)

// FieldCounts aggregates classification buckets for one field (or a set of
// fields) and backs the inventory report.
type FieldCounts struct {
	Total     int `json:"total"`
	V2        int `json:"v2"`
	Legacy    int `json:"legacy"`
	Plaintext int `json:"plaintext"`
	Empty     int `json:"empty"`
	Unknown   int `json:"unknown"`
	NotSecret int `json:"notSecret"`
}

// AggregateFormats counts classification buckets. Mixed-declaration values
// rejected by the declared gate arrive as FormatNotSecret and land in the
// NotSecret bucket, so declaration-exempt values never inflate PLAINTEXT.
func AggregateFormats(formats []SecretFormat) FieldCounts {
	counts := FieldCounts{Total: len(formats)}
	for _, format := range formats {
		switch format {
		case FormatV2:
			counts.V2++
		case FormatLegacy:
			counts.Legacy++
		case FormatPlaintext:
			counts.Plaintext++
		case FormatEmpty:
			counts.Empty++
		case FormatUnknown:
			counts.Unknown++
		case FormatNotSecret:
			counts.NotSecret++
		}
	}
	return counts
}

// masterKeyEntry is one configured key id with its raw seed material. The AES
// key is derived exactly like the legacy path: sha256(seed) (secret.go:44).
type masterKeyEntry struct {
	id       string
	material string
}

var secretMasterKeys struct {
	sync.RWMutex
	// configured distinguishes "setter ran" from "setter never ran". When the
	// setter never ran, OPS_SECRET_MASTER_KEYS is parsed at first use so a
	// deployment that only sets the environment variable gets the configured
	// key set without a code-path change.
	configured bool
	entries    []masterKeyEntry
}

// ConfigureSecretMasterKeys parses and stores the ordered master key set. The
// first entry is the current key the writer uses; later entries remain
// readable by key id. An empty (or whitespace-only) spec resets to the
// implicit set — the official reset for tests and for opting out of an env
// value picked up at first use. The reserved id "legacy" is rejected: the
// legacy entry is always derived from the credential seed chain instead.
func ConfigureSecretMasterKeys(spec string) error {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		secretMasterKeys.Lock()
		secretMasterKeys.configured = true
		secretMasterKeys.entries = nil
		secretMasterKeys.Unlock()
		return nil
	}
	entries, err := parseMasterKeySpec(trimmed)
	if err != nil {
		return err
	}
	secretMasterKeys.Lock()
	secretMasterKeys.configured = true
	secretMasterKeys.entries = entries
	secretMasterKeys.Unlock()
	return nil
}

func parseMasterKeySpec(spec string) ([]masterKeyEntry, error) {
	seen := make(map[string]struct{})
	entries := make([]masterKeyEntry, 0, strings.Count(spec, ",")+1)
	for _, part := range strings.Split(spec, ",") {
		id, material, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("master key entry %q must use key_id:key_material", part)
		}
		id = strings.TrimSpace(id)
		material = strings.TrimSpace(material)
		if id == "" {
			return nil, fmt.Errorf("master key entry %q has an empty key id", part)
		}
		if strings.IndexFunc(id, unicode.IsSpace) >= 0 {
			return nil, fmt.Errorf("master key id %q must not contain whitespace", id)
		}
		if id == legacyMasterKeyID {
			return nil, fmt.Errorf("master key id %q is reserved and derived from the credential seed chain", legacyMasterKeyID)
		}
		if material == "" {
			return nil, fmt.Errorf("master key entry %q has empty key material", part)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("duplicate master key id %q", id)
		}
		seen[id] = struct{}{}
		entries = append(entries, masterKeyEntry{id: id, material: material})
	}
	return entries, nil
}

// secretMasterKeySet resolves the effective key set: id → derived AES key,
// plus the id of the current (writer) key. The implicit "legacy" entry is
// derived from the live credential seed chain on every consultation — never
// materialized at package init, so a later ConfigureCredentialKey call is
// still reflected by the keys used afterwards. With no configured entries the
// set is {current: legacy}, so deployments without OPS_SECRET_MASTER_KEYS keep
// writing self-readable envelopes under the legacy id.
func secretMasterKeySet() (map[string][]byte, string, error) {
	secretMasterKeys.RLock()
	configured, entries := secretMasterKeys.configured, secretMasterKeys.entries
	secretMasterKeys.RUnlock()
	if !configured {
		spec := strings.TrimSpace(os.Getenv("OPS_SECRET_MASTER_KEYS"))
		if spec != "" {
			parsed, err := parseMasterKeySpec(spec)
			if err != nil {
				return nil, "", err
			}
			entries = parsed
		}
	}
	keys := make(map[string][]byte, len(entries)+1)
	current := legacyMasterKeyID
	for index, entry := range entries {
		keys[entry.id] = deriveSecretKey(entry.material)
		if index == 0 {
			current = entry.id
		}
	}
	keys[legacyMasterKeyID] = credentialKey()
	return keys, current, nil
}

// deriveSecretKey mirrors the legacy AES-256 key derivation in secret.go:44
// (sha256 of the raw seed). The key material in OPS_SECRET_MASTER_KEYS is the
// raw seed string, not a digest.
func deriveSecretKey(seed string) []byte {
	digest := sha256.Sum256([]byte(seed))
	return digest[:]
}

// sealSecretAESGCM produces base64url(nonce‖ct) with AES-256-GCM, matching the
// legacy envelope layout of EncryptSecret (secret.go:48-63) byte for byte.
func sealSecretAESGCM(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// openSecretAESGCM inverts sealSecretAESGCM with the same truncation and
// authentication checks as DecryptSecret (secret.go:65-86).
func openSecretAESGCM(key []byte, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("v2 secret payload is truncated")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt v2 secret: %w", err)
	}
	return string(plain), nil
}

// EncryptSecretV2 seals plain as a v2 envelope under the current key. Empty
// input stays empty: optional secrets are persisted without an envelope, which
// ClassifySecret reports as EMPTY rather than plaintext.
func EncryptSecretV2(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	keys, current, err := secretMasterKeySet()
	if err != nil {
		return "", err
	}
	sealed, err := sealSecretAESGCM(keys[current], plain)
	if err != nil {
		return "", err
	}
	return v2EnvelopePrefix + current + ":" + sealed, nil
}

// parseV2Envelope splits a v2 envelope into key id and ciphertext. Exactly
// three colon-separated parts, a non-empty key id and rawurl-decodable
// ciphertext are required; anything else is not a v2 envelope.
func parseV2Envelope(value string) (string, []byte, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" || parts[1] == "" {
		return "", nil, fmt.Errorf("value is not a v2 secret envelope")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", nil, fmt.Errorf("malformed v2 secret envelope: %w", err)
	}
	return parts[1], payload, nil
}

// DecryptSecretV2 opens a v2 envelope, routing to the key named by the key id.
// It is strict: values without the v2 prefix are rejected here instead of
// being probed as legacy or plaintext — format probing belongs to
// ClassifySecret, which orders the rules for both readers and migration.
func DecryptSecretV2(value string) (string, error) {
	keyID, payload, err := parseV2Envelope(value)
	if err != nil {
		return "", err
	}
	keys, _, err := secretMasterKeySet()
	if err != nil {
		return "", err
	}
	key, ok := keys[keyID]
	if !ok {
		return "", fmt.Errorf("unknown v2 master key id %q", keyID)
	}
	return openSecretAESGCM(key, payload)
}

// ClassifySecret buckets a stored value by the §4.3 ordered rule. It is a pure
// function of (registry, value) with one documented exception: mixed-
// declaration columns consult the caller-supplied declaredSecret gate, and a
// non-declared value there is NOT_SECRET-by-declaration regardless of its
// shape. Malformed "v2:"-prefixed values and E-class values that claim neither
// format are UNKNOWN — ambiguity is reported, never interpreted as plaintext.
func ClassifySecret(value string, field SecretField, declaredSecret bool) SecretFormat {
	registered, ok := LookupSecretField(field.Table, field.Column)
	if !ok {
		return FormatNotSecret
	}
	if registered.MixedDeclaration && !declaredSecret {
		return FormatNotSecret
	}
	if value == "" {
		return FormatEmpty
	}
	if strings.HasPrefix(value, v2EnvelopePrefix) {
		if keyID, _, err := parseV2Envelope(value); err == nil {
			if keys, _, keyErr := secretMasterKeySet(); keyErr == nil {
				if _, known := keys[keyID]; known {
					return FormatV2
				}
			}
		}
		return FormatUnknown
	}
	if registered.Class == ClassELegacy {
		if _, err := DecryptSecret(value); err == nil {
			return FormatLegacy
		}
		return FormatUnknown
	}
	if registered.Class == ClassPlaintext {
		return FormatPlaintext
	}
	return FormatUnknown
}

// ReadSecretField is the single dual-format reader shared by runtime paths and
// the migration tooling so the two cannot disagree. UNKNOWN fails closed; P-
// class plaintext and declaration-exempt values pass through verbatim.
func ReadSecretField(value string, field SecretField, declaredSecret bool) (string, error) {
	switch ClassifySecret(value, field, declaredSecret) {
	case FormatEmpty:
		return "", nil
	case FormatNotSecret:
		return value, nil
	case FormatV2:
		return DecryptSecretV2(value)
	case FormatLegacy:
		return DecryptSecret(value)
	case FormatPlaintext:
		return value, nil
	default:
		return "", fmt.Errorf("secret value for %s.%s claims neither the v2 nor the legacy format", field.Table, field.Column)
	}
}
