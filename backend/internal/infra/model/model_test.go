package model

import (
	"strings"
	"sync"
	"testing"

	"reflect"

	"gorm.io/gorm/schema"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/testutil"
)

// specTables is the M1 model set PR 16 delivers (plan §2 PR 16 — §7.2–7.5,
// §8.1/8.2/8.4/§9.1). T18 asserts every table name verbatim.
func specTables(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"provider_connection":         &ProviderConnection{},
		"provider_context":            &ProviderContext{},
		"provider_credential_binding": &ProviderCredentialBinding{},
		"secret_ref":                  &SecretRef{},
		"infra_resource":              &InfraResource{},
		"resource_observation":        &ResourceObservation{},
		"inventory_sync_run":          &InventorySyncRun{},
		"infra_relationship":          &InfraRelationship{},
	}
}

// parseColumns resolves a model's DB column names through the GORM schema
// parser, so T18 asserts the mapped column set rather than Go field names.
func parseColumns(t *testing.T, model any) map[string]bool {
	t.Helper()
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse(%T): %v", model, err)
	}
	cols := make(map[string]bool, len(s.FieldsByDBName))
	for name := range s.FieldsByDBName {
		cols[name] = true
	}
	return cols
}

// T18 — TestInfraModelsMatchSpecVocab: table names for all 8 M1 models,
// spec field presence, and the vocabulary anchors (§7.4 purposes are echoed
// by the secrets broker, SecretRef backend default, contract resource kinds).
func TestInfraModelsMatchSpecVocab(t *testing.T) {
	for table, model := range specTables(t) {
		if got := tableOf(model); got != table {
			t.Errorf("%T.TableName() = %q, want %q", model, got, table)
		}
	}

	fieldSets := map[string]map[string]bool{
		"provider_connection":         parseColumns(t, &ProviderConnection{}),
		"provider_context":            parseColumns(t, &ProviderContext{}),
		"provider_credential_binding": parseColumns(t, &ProviderCredentialBinding{}),
		"secret_ref":                  parseColumns(t, &SecretRef{}),
		"infra_resource":              parseColumns(t, &InfraResource{}),
		"resource_observation":        parseColumns(t, &ResourceObservation{}),
		"inventory_sync_run":          parseColumns(t, &InventorySyncRun{}),
		"infra_relationship":          parseColumns(t, &InfraRelationship{}),
	}
	want := map[string][]string{
		"provider_connection":         {"id", "uid", "provider_type", "name", "endpoint", "gateway_id", "tls_profile", "config_json", "status", "version", "capability_hash", "last_health_at"},
		"provider_context":            {"id", "uid", "connection_id", "kind", "external_id", "name", "status", "metadata_json"},
		"provider_credential_binding": {"id", "provider_connection_id", "provider_context_id", "purpose", "secret_ref_id", "status", "last_validated_at"},
		"secret_ref":                  {"id", "uid", "backend", "path", "version", "key_id", "ciphertext", "rotated_at"},
		"infra_resource":              {"id", "uid", "context_id", "kind", "subtype", "external_id", "external_urn", "name", "display_name", "lifecycle_state", "health_state", "managed_state", "labels_json", "first_seen_at", "last_seen_at", "deleted_at"},
		"resource_observation":        {"id", "resource_id", "generation_uid", "observation_hash", "normalizer_version", "normalized_json", "raw_json", "observed_at"},
		"inventory_sync_run":          {"id", "uid", "connection_id", "context_id", "mode", "status", "cursor", "seen_count", "created_count", "updated_count", "missing_count", "error_code", "started_at", "committed_at", "finished_at"},
		"infra_relationship":          {"id", "from_resource_id", "to_resource_id", "type", "source", "generation_uid", "attributes_json", "last_seen_at"},
	}
	for table, fields := range want {
		for _, col := range fields {
			if !fieldSets[table][col] {
				t.Errorf("%s: missing spec column %q", table, col)
			}
		}
	}

	// §7.5 — SecretRef.Backend defaults to the M1 internal backend (read off
	// the struct's gorm tag — the schema parser normalizes defaults away).
	secretRefType := reflect.TypeOf(SecretRef{})
	backendField, ok := secretRefType.FieldByName("Backend")
	if !ok || backendField.Tag.Get("gorm") != "size:32;not null;default:internal" {
		t.Errorf("SecretRef.Backend gorm tag = %q, want the §7.5 default:internal form", backendField.Tag.Get("gorm"))
	}

	// §8.1 — InfraResource.Kind is validated against the closed M1 vocabulary.
	if !contract.IsKnownResourceKind("compute.vm") || contract.IsKnownResourceKind("made.up.kind") {
		t.Error("contract.IsKnownResourceKind deviates from the M1 vocabulary")
	}
	// §7.3 — ProviderContext.Kind draws from contract.ProviderContextKinds.
	if len(contract.ProviderContextKinds) == 0 {
		t.Error("contract.ProviderContextKinds is empty — provider_context.kind has no vocabulary")
	}
}

func tableOf(model any) string {
	type namer interface{ TableName() string }
	if n, ok := model.(namer); ok {
		return n.TableName()
	}
	return ""
}

// T19 — TestResourceIdentityUnique: the identity rule UNIQUE(context, kind,
// external_urn) must hold through GORM tag synthesis (r2 C / E-3a) and keep
// holding after a re-migration.
func TestResourceIdentityUnique(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&InfraResource{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	seed := InfraResource{UID: "res-1", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	dup := InfraResource{UID: "res-2", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("duplicate (context, kind, external_urn) insert was accepted — identity unique missing")
	}

	// A different urn in the same context is a distinct resource.
	other := InfraResource{UID: "res-3", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:2"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("same context, distinct urn insert rejected: %v", err)
	}
	// The same urn in another context is a distinct resource too.
	crossCtx := InfraResource{UID: "res-4", ContextID: 8, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&crossCtx).Error; err != nil {
		t.Fatalf("distinct context, same urn insert rejected: %v", err)
	}

	// E-3a — the tag-synthesized unique must survive a re-migration.
	if err := db.AutoMigrate(&InfraResource{}); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	dupAgain := InfraResource{UID: "res-5", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&dupAgain).Error; err == nil {
		t.Fatal("identity unique did not survive re-AutoMigrate (E-3a)")
	}
}

// T20 — TestObservationCompositeIndex: (resource_id, observed_at DESC) must
// exist as a composite index (§8.2).
func TestObservationCompositeIndex(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&ResourceObservation{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	var indexSQL string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_obs_res_time'`).Scan(&indexSQL).Error; err != nil {
		t.Fatalf("index lookup: %v", err)
	}
	if indexSQL == "" {
		t.Fatal("idx_obs_res_time not found in sqlite_master — §8.2 composite index missing")
	}

	rows, err := db.Raw(`PRAGMA index_info('idx_obs_res_time')`).Rows()
	if err != nil {
		t.Fatalf("pragma index_info: %v", err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		// index_info columns: (seqno, cid, name).
		var seq, cid int
		var name string
		if err := rows.Scan(&seq, &cid, &name); err != nil {
			t.Fatalf("scan index_info: %v", err)
		}
		columns = append(columns, name)
	}
	if len(columns) != 2 {
		t.Fatalf("idx_obs_res_time columns = %v, want [resource_id observed_at]", columns)
	}
	if columns[0] != "resource_id" || columns[1] != "observed_at" {
		t.Errorf("idx_obs_res_time columns = %v, want [resource_id observed_at]", columns)
	}
	if !strings.Contains(strings.ToUpper(indexSQL), "DESC") {
		t.Errorf("idx_obs_res_time DDL %q lacks observed_at DESC ordering", indexSQL)
	}
}
