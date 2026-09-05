package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/middleware"
	"ops-admin/backend/model"
)

// Console-ticket resource and protocol vocabulary. The protocol is fixed per
// terminal endpoint and validated server-side; the client cannot mint a
// ticket whose protocol does not belong to the resource type.
const (
	ConsoleResourceAssetHost      = "asset_host"
	ConsoleResourceK8sPod         = "k8s_pod"
	ConsoleProtocolAssetTerminal  = "asset-terminal"
	ConsoleProtocolK8sPodTerminal = "k8s-pod-terminal"

	// ConsoleTicketTTL is the fixed one-shot validity window of a minted
	// ticket — the §4.8 "≤30s" upper bound, intentionally not configurable.
	ConsoleTicketTTL = 30 * time.Second
)

var (
	// ErrConsoleTicketInvalid marks unknown, expired or already-consumed
	// tickets (HTTP 401 at the WS gate; legacy fallback is not offered).
	ErrConsoleTicketInvalid = errors.New("console ticket is invalid or expired")
	// ErrConsoleTicketMismatch marks a valid ticket presented against another
	// resource or another protocol (HTTP 403 at the WS gate).
	ErrConsoleTicketMismatch = errors.New("console ticket is bound to another resource or protocol")
	// ErrConsoleResourceInvalid marks a mint request whose resourceId cannot
	// be canonicalized or whose protocol/resourceType pair is unknown.
	ErrConsoleResourceInvalid = errors.New("unknown console resource or protocol pair")
)

// MintedConsoleTicket is the mint response: the one-time plaintext ticket
// value (never stored server-side), its absolute expiry and the TTL.
type MintedConsoleTicket struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
	ExpiresIn int       `json:"expiresIn"`
}

// CanonicalConsoleResourceID normalizes a raw resource identifier into the
// canonical ticket binding key. Minting (the request resourceId) and
// consumption (the WS query parameters) funnel through this single function
// so raw forms such as "0123" or "+123" denote the same bound resource as
// "123" instead of mapping to a phantom one.
func CanonicalConsoleResourceID(resourceType string, raw string) (string, error) {
	switch resourceType {
	case ConsoleResourceAssetHost:
		parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil || parsed == 0 {
			return "", ErrConsoleResourceInvalid
		}
		return strconv.FormatUint(parsed, 10), nil
	case ConsoleResourceK8sPod:
		parts := strings.Split(raw, "/")
		if len(parts) != 3 {
			return "", ErrConsoleResourceInvalid
		}
		cluster, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil || cluster == 0 {
			return "", ErrConsoleResourceInvalid
		}
		namespace, pod := strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		if namespace == "" || pod == "" {
			return "", ErrConsoleResourceInvalid
		}
		return strconv.FormatUint(cluster, 10) + "/" + namespace + "/" + pod, nil
	}
	return "", ErrConsoleResourceInvalid
}

// consoleProtocolForResource returns the one protocol a resource type may be
// minted for.
func consoleProtocolForResource(resourceType string) (string, bool) {
	switch resourceType {
	case ConsoleResourceAssetHost:
		return ConsoleProtocolAssetTerminal, true
	case ConsoleResourceK8sPod:
		return ConsoleProtocolK8sPodTerminal, true
	}
	return "", false
}

// consoleTicketHash digests a ticket value; only this digest is stored.
func consoleTicketHash(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(digest[:])
}

// MintConsoleTicket issues a one-time ticket for the resource. The caller has
// already re-verified the resource-specific permission (M-1); this function
// enforces the protocol pair and the canonical binding key.
func (s *Service) MintConsoleTicket(resourceType string, rawResourceID string, protocol string, userID uint) (MintedConsoleTicket, error) {
	expected, ok := consoleProtocolForResource(resourceType)
	if !ok || protocol != expected {
		return MintedConsoleTicket{}, ErrConsoleResourceInvalid
	}
	resourceID, err := CanonicalConsoleResourceID(resourceType, rawResourceID)
	if err != nil {
		return MintedConsoleTicket{}, ErrConsoleResourceInvalid
	}
	s.pruneExpiredConsoleTickets()

	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return MintedConsoleTicket{}, err
	}
	rowID := make([]byte, 16)
	if _, err := rand.Read(rowID); err != nil {
		return MintedConsoleTicket{}, err
	}
	ticket := base64.RawURLEncoding.EncodeToString(value)
	now := time.Now()
	row := model.ConsoleTicket{
		ID:           hex.EncodeToString(rowID),
		TicketHash:   consoleTicketHash(ticket),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Protocol:     protocol,
		UserID:       userID,
		ExpiresAt:    now.Add(ConsoleTicketTTL),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return MintedConsoleTicket{}, err
	}
	return MintedConsoleTicket{
		Ticket:    ticket,
		ExpiresAt: row.ExpiresAt,
		ExpiresIn: int(ConsoleTicketTTL.Seconds()),
	}, nil
}

// ConsumeConsoleTicket atomically spends a ticket against the given
// protocol/resource binding. Classification: unknown/expired/consumed →
// ErrConsoleTicketInvalid; valid but foreign binding → ErrConsoleTicketMismatch
// (which does not burn the ticket); success requires the single atomic UPDATE
// to affect exactly one row — a raced second consumer fails here, and the
// ticket is spent regardless of what the terminal session does afterwards.
func (s *Service) ConsumeConsoleTicket(ticket string, protocol string, resourceType string, resourceID string) error {
	hash := consoleTicketHash(ticket)
	var row model.ConsoleTicket
	if err := s.db.Where("ticket_hash = ?", hash).First(&row).Error; err != nil {
		return ErrConsoleTicketInvalid
	}
	now := time.Now()
	if row.ConsumedAt != nil || !row.ExpiresAt.After(now) {
		return ErrConsoleTicketInvalid
	}
	if row.Protocol != protocol || row.ResourceType != resourceType || row.ResourceID != resourceID {
		return ErrConsoleTicketMismatch
	}
	result := s.db.Model(&model.ConsoleTicket{}).
		Where("ticket_hash = ? AND consumed_at IS NULL AND expires_at > ?", hash, now).
		Update("consumed_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConsoleTicketInvalid
	}
	return nil
}

// pruneExpiredConsoleTickets deletes tickets expired for more than a day — a
// best-effort bound on table growth; failures are ignored by design.
func (s *Service) pruneExpiredConsoleTickets() {
	_ = s.db.Where("expires_at < ?", time.Now().Add(-24*time.Hour)).
		Delete(&model.ConsoleTicket{}).Error
}

// AdminHasPermission reports whether an admin holds a route-permission menu
// value. The authorization SQL lives once in middleware.AdminHasPermission;
// the console mint path re-verifies the resource-specific permission through
// it because the endpoint-level AnyOf check is deliberately loose.
func (s *Service) AdminHasPermission(adminID uint, permission string) bool {
	return middleware.AdminHasPermission(s.db, adminID, permission)
}
