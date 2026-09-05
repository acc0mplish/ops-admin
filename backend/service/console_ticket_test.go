package service

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"ops-admin/backend/model"
)

// newConsoleTicketDB opens a single-connection in-memory sqlite database
// (M-2) migrated with only the models the console-ticket path touches, plus
// the role/menu triple the AdminHasPermission wrapper queries.
func newConsoleTicketDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.ConsoleTicket{}, &model.Menu{}, &model.Role{}, &model.RoleMenu{}, &model.AdminRole{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestConsoleTicketMintConsumeRoundtrip is T1: a minted ticket consumes
// exactly once against its bound resource and records the consumed marker.
func TestConsoleTicketMintConsumeRoundtrip(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	minted, err := svc.MintConsoleTicket(ConsoleResourceAssetHost, "12", ConsoleProtocolAssetTerminal, 7)
	if err != nil {
		t.Fatal(err)
	}
	if minted.Ticket == "" {
		t.Fatal("mint produced an empty ticket value")
	}
	if minted.ExpiresIn != int(ConsoleTicketTTL.Seconds()) {
		t.Fatalf("expiresIn %d, want %d", minted.ExpiresIn, int(ConsoleTicketTTL.Seconds()))
	}

	var row model.ConsoleTicket
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if len(row.TicketHash) != 64 || row.TicketHash == minted.Ticket {
		t.Fatalf("stored hash %q must be a 64-char digest, never the ticket value", row.TicketHash)
	}
	if row.ResourceID != "12" || row.ResourceType != ConsoleResourceAssetHost || row.Protocol != ConsoleProtocolAssetTerminal {
		t.Fatalf("binding mismatch: %+v", row)
	}
	if row.UserID != 7 {
		t.Fatalf("ticket owner %d, want 7", row.UserID)
	}
	if row.ConsumedAt != nil {
		t.Fatal("fresh ticket must be unconsumed")
	}

	if err := svc.ConsumeConsoleTicket(minted.Ticket, ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "12"); err != nil {
		t.Fatalf("consume of a fresh ticket failed: %v", err)
	}
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ConsumedAt == nil {
		t.Fatal("consumed ticket must record consumed_at")
	}
}

// TestConsoleTicketExpiredRejected is T2: a ticket whose expiry passed is
// rejected with the invalid marker.
func TestConsoleTicketExpiredRejected(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	minted, err := svc.MintConsoleTicket(ConsoleResourceK8sPod, "3/default/pod-a", ConsoleProtocolK8sPodTerminal, 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.ConsoleTicket{}).Where("1=1").Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeConsoleTicket(minted.Ticket, ConsoleProtocolK8sPodTerminal, ConsoleResourceK8sPod, "3/default/pod-a"); !errors.Is(err, ErrConsoleTicketInvalid) {
		t.Fatalf("expired ticket must be ErrConsoleTicketInvalid, got %v", err)
	}
}

// TestConsoleTicketReuseRejected is T3: the single-use property — a second
// consume of the same ticket value fails (the atomic UPDATE matches no row
// because consumed_at IS NULL no longer holds). Concurrency itself is not
// reproducible on a single-connection sqlite handle; the atomicity argument
// is the single UPDATE statement plus this sequential replay.
func TestConsoleTicketReuseRejected(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	minted, err := svc.MintConsoleTicket(ConsoleResourceAssetHost, "42", ConsoleProtocolAssetTerminal, 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeConsoleTicket(minted.Ticket, ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "42"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeConsoleTicket(minted.Ticket, ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "42"); !errors.Is(err, ErrConsoleTicketInvalid) {
		t.Fatalf("reused ticket must be ErrConsoleTicketInvalid, got %v", err)
	}
}

// TestConsoleTicketBindingMismatchRejected covers the unit half of T4/T5: a
// valid ticket presented against another resource or another protocol is the
// mismatch marker, and the mismatched attempt must not burn the ticket.
func TestConsoleTicketBindingMismatchRejected(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	minted, err := svc.MintConsoleTicket(ConsoleResourceAssetHost, "42", ConsoleProtocolAssetTerminal, 7)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		protocol     string
		resourceType string
		resourceID   string
	}{
		{"other host", ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "43"},
		{"asset ticket on k8s endpoint", ConsoleProtocolK8sPodTerminal, ConsoleResourceK8sPod, "42/default/pod-a"},
	}
	for _, tc := range cases {
		if err := svc.ConsumeConsoleTicket(minted.Ticket, tc.protocol, tc.resourceType, tc.resourceID); !errors.Is(err, ErrConsoleTicketMismatch) {
			t.Fatalf("%s: want ErrConsoleTicketMismatch, got %v", tc.name, err)
		}
	}
	var row model.ConsoleTicket
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ConsumedAt != nil {
		t.Fatal("mismatched attempts must not consume the ticket")
	}
}

// TestConsoleTicketUnknownValueRejected covers the no-such-ticket branch of
// the consume gate.
func TestConsoleTicketUnknownValueRejected(t *testing.T) {
	svc := &Service{db: newConsoleTicketDB(t)}
	if err := svc.ConsumeConsoleTicket("bogus-ticket", ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "42"); !errors.Is(err, ErrConsoleTicketInvalid) {
		t.Fatalf("unknown ticket must be ErrConsoleTicketInvalid, got %v", err)
	}
	if err := svc.ConsumeConsoleTicket("", ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "42"); !errors.Is(err, ErrConsoleTicketInvalid) {
		t.Fatalf("empty ticket must be ErrConsoleTicketInvalid, got %v", err)
	}
}

// TestCanonicalConsoleResourceID is the Q1 contract: raw integer forms are
// normalized to the canonical decimal string on both the mint and the consume
// side, so "0123" and "+123" denote the same bound resource as "123" and
// negative or non-numeric forms are rejected outright.
func TestCanonicalConsoleResourceID(t *testing.T) {
	cases := []struct {
		name         string
		resourceType string
		raw          string
		want         string
	}{
		{"plain host id", ConsoleResourceAssetHost, "123", "123"},
		{"zero-padded host id", ConsoleResourceAssetHost, "0123", "123"},
		{"plain pod key", ConsoleResourceK8sPod, "3/default/nginx-abc", "3/default/nginx-abc"},
		{"zero-padded cluster", ConsoleResourceK8sPod, "03/default/nginx-abc", "3/default/nginx-abc"},
	}
	for _, tc := range cases {
		got, err := CanonicalConsoleResourceID(tc.resourceType, tc.raw)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: canonical %q, want %q", tc.name, got, tc.want)
		}
	}

	rejected := []struct {
		name         string
		resourceType string
		raw          string
	}{
		{"negative host id", ConsoleResourceAssetHost, "-1"},
		{"signed host id", ConsoleResourceAssetHost, "+123"},
		{"zero host id", ConsoleResourceAssetHost, "0"},
		{"garbage host id", ConsoleResourceAssetHost, "abc"},
		{"unknown resource type", "serverless_fn", "1"},
		{"two-segment pod key", ConsoleResourceK8sPod, "3/default"},
		{"empty namespace", ConsoleResourceK8sPod, "3//nginx-abc"},
		{"empty pod", ConsoleResourceK8sPod, "3/default/"},
		{"garbage cluster", ConsoleResourceK8sPod, "x/default/nginx-abc"},
		{"namespace with slash", ConsoleResourceK8sPod, "3/de/fault/nginx-abc"},
	}
	for _, tc := range rejected {
		if got, err := CanonicalConsoleResourceID(tc.resourceType, tc.raw); err == nil {
			t.Fatalf("%s: want rejection, got %q", tc.name, got)
		}
	}
}

// TestConsoleTicketMintCanonicalizesBindingKey proves the mint side stores
// the canonical form even when the request carried a raw one.
func TestConsoleTicketMintCanonicalizesBindingKey(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	minted, err := svc.MintConsoleTicket(ConsoleResourceAssetHost, "0123", ConsoleProtocolAssetTerminal, 7)
	if err != nil {
		t.Fatal(err)
	}
	// Consume with the canonical id of the same resource: one resource, one key.
	if err := svc.ConsumeConsoleTicket(minted.Ticket, ConsoleProtocolAssetTerminal, ConsoleResourceAssetHost, "123"); err != nil {
		t.Fatalf("canonical consume failed: %v", err)
	}
}

// TestConsoleTicketProtocolPairEnforced rejects mint requests whose protocol
// does not match the fixed protocol of the resource type.
func TestConsoleTicketProtocolPairEnforced(t *testing.T) {
	svc := &Service{db: newConsoleTicketDB(t)}
	cases := []struct {
		name         string
		resourceType string
		protocol     string
	}{
		{"host ticket with k8s protocol", ConsoleResourceAssetHost, ConsoleProtocolK8sPodTerminal},
		{"pod ticket with asset protocol", ConsoleResourceK8sPod, ConsoleProtocolAssetTerminal},
		{"unknown resource type", "serverless_fn", ConsoleProtocolAssetTerminal},
		{"unknown protocol", ConsoleResourceAssetHost, "exec"},
	}
	for _, tc := range cases {
		if _, err := svc.MintConsoleTicket(tc.resourceType, "1", tc.protocol, 7); !errors.Is(err, ErrConsoleResourceInvalid) {
			t.Fatalf("%s: want ErrConsoleResourceInvalid, got %v", tc.name, err)
		}
	}
}

// TestConsoleTicketMintPrunesExpired keeps the table bounded (R-8): tickets
// expired more than 24h ago are deleted on mint, recent ones survive.
func TestConsoleTicketMintPrunesExpired(t *testing.T) {
	db := newConsoleTicketDB(t)
	svc := &Service{db: db}

	stale := model.ConsoleTicket{ID: "stale", TicketHash: "stale-hash", ResourceType: ConsoleResourceAssetHost, ResourceID: "1", Protocol: ConsoleProtocolAssetTerminal, UserID: 7, ExpiresAt: time.Now().Add(-25 * time.Hour)}
	recent := model.ConsoleTicket{ID: "recent", TicketHash: "recent-hash", ResourceType: ConsoleResourceAssetHost, ResourceID: "2", Protocol: ConsoleProtocolAssetTerminal, UserID: 7, ExpiresAt: time.Now().Add(-time.Minute)}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.MintConsoleTicket(ConsoleResourceAssetHost, "3", ConsoleProtocolAssetTerminal, 7); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.ConsoleTicket{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("table holds %d rows after pruning, want 2 (fresh + new)", count)
	}
	if err := db.Where("id = ?", "stale").First(&model.ConsoleTicket{}).Error; err == nil {
		t.Fatal("stale ticket must have been pruned")
	}
}

// TestServiceAdminHasPermissionWrapper drives the M-1 wrapper against the
// real role/menu grant tables: only a granted, enabled menu value passes.
func TestServiceAdminHasPermissionWrapper(t *testing.T) {
	db := newConsoleTicketDB(t)
	menu := model.Menu{Value: "assets:host:terminal", MenuStatus: 1}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	disabled := model.Menu{Value: "assets:k8s:pod:terminal", MenuStatus: 0}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	role := model.Role{RoleName: "terminal-op", RoleKey: "terminal-op", Status: 1}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AdminRole{AdminID: 7, RoleID: role.ID}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{db: db}
	if !svc.AdminHasPermission(7, "assets:host:terminal") {
		t.Fatal("granted admin must hold assets:host:terminal")
	}
	if svc.AdminHasPermission(7, "assets:k8s:pod:terminal") {
		t.Fatal("disabled menu must not count as a grant")
	}
	if svc.AdminHasPermission(8, "assets:host:terminal") {
		t.Fatal("unwired admin must not hold the grant")
	}
	if svc.AdminHasPermission(7, "") {
		t.Fatal("empty permission must be rejected without querying")
	}
}
