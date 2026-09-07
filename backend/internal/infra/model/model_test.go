// External test package (model_test): T41 must run the migration runner over
// the model set, and migrate imports model — an in-package test would form an
// import cycle. Only exported identifiers are asserted.
package model_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/testutil"
)

// specTables is the M1 model set PR 16 + PR 17 deliver (plan §2 PR 16 —
// §7.2–7.5, §8.1/8.2/8.4/§9.1; plan §2 PR 17 — §13.1 task 3종). T18 and the
// PR 17 task-table assertions both go through this table.
func specTables(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"provider_connection":         &model.ProviderConnection{},
		"provider_context":            &model.ProviderContext{},
		"provider_credential_binding": &model.ProviderCredentialBinding{},
		"secret_ref":                  &model.SecretRef{},
		"infra_resource":              &model.InfraResource{},
		"resource_observation":        &model.ResourceObservation{},
		"inventory_sync_run":          &model.InventorySyncRun{},
		"infra_relationship":          &model.InfraRelationship{},
		// PR 17 (plan §2 file 14): 태스크 3테이블 존재·TableName 단얫.
		"provider_task": &model.ProviderTask{},
		"task_attempt":  &model.TaskAttempt{},
		"task_event":    &model.TaskEvent{},
	}
}

// parseColumns resolves a model's DB column names through the GORM schema
// parser, so T18 asserts the mapped column set rather than Go field names.
func parseColumns(t *testing.T, modelType any) map[string]bool {
	t.Helper()
	s, err := schema.Parse(modelType, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse(%T): %v", modelType, err)
	}
	cols := make(map[string]bool, len(s.FieldsByDBName))
	for name := range s.FieldsByDBName {
		cols[name] = true
	}
	return cols
}

// T18 — TestInfraModelsMatchSpecVocab: table names for all M1 models, spec
// field presence, and the vocabulary anchors (§7.4 purposes are echoed by the
// secrets broker, SecretRef backend default, contract resource kinds).
// PR 17 extends the same assertions to the task 3-table set (§13.1).
func TestInfraModelsMatchSpecVocab(t *testing.T) {
	for table, modelType := range specTables(t) {
		if got := tableOf(modelType); got != table {
			t.Errorf("%T.TableName() = %q, want %q", modelType, got, table)
		}
	}

	fieldSets := map[string]map[string]bool{
		"provider_connection":         parseColumns(t, &model.ProviderConnection{}),
		"provider_context":            parseColumns(t, &model.ProviderContext{}),
		"provider_credential_binding": parseColumns(t, &model.ProviderCredentialBinding{}),
		"secret_ref":                  parseColumns(t, &model.SecretRef{}),
		"infra_resource":              parseColumns(t, &model.InfraResource{}),
		"resource_observation":        parseColumns(t, &model.ResourceObservation{}),
		"inventory_sync_run":          parseColumns(t, &model.InventorySyncRun{}),
		"infra_relationship":          parseColumns(t, &model.InfraRelationship{}),
		"provider_task":               parseColumns(t, &model.ProviderTask{}),
		"task_attempt":                parseColumns(t, &model.TaskAttempt{}),
		"task_event":                  parseColumns(t, &model.TaskEvent{}),
	}
	want := map[string][]string{
		"provider_connection":         {"id", "uid", "provider_type", "name", "endpoint", "gateway_id", "tls_profile", "config_json", "status", "version", "capability_hash", "last_health_at", "source_model", "source_id", "source_updated_at", "stale_source"},
		"provider_context":            {"id", "uid", "connection_id", "kind", "external_id", "name", "status", "metadata_json"},
		"provider_credential_binding": {"id", "provider_connection_id", "provider_context_id", "purpose", "secret_ref_id", "status", "last_validated_at"},
		"secret_ref":                  {"id", "uid", "backend", "path", "version", "key_id", "ciphertext", "rotated_at"},
		"infra_resource":              {"id", "uid", "context_id", "kind", "subtype", "external_id", "external_urn", "name", "display_name", "lifecycle_state", "health_state", "managed_state", "labels_json", "first_seen_at", "last_seen_at", "deleted_at"},
		"resource_observation":        {"id", "resource_id", "generation_uid", "observation_hash", "normalizer_version", "normalized_json", "raw_json", "observed_at"},
		"inventory_sync_run":          {"id", "uid", "connection_id", "context_id", "mode", "status", "cursor", "seen_count", "created_count", "updated_count", "missing_count", "error_code", "started_at", "committed_at", "finished_at"},
		"infra_relationship":          {"id", "from_resource_id", "to_resource_id", "type", "source", "generation_uid", "attributes_json", "last_seen_at"},
		// §13.1/§13.2/§13.4 조립형 (plan §3.4). step0003 (PR 18) Expand 컬럼
		// (approval 3종·idempotency_key)은 이 시점의 칼럼 집합에 없다.
		"provider_task": {"id", "uid", "operation_name", "operation_version", "resource_uid", "payload_json", "status", "attempt_count", "max_attempts", "next_attempt_at", "lease_expires_at", "version", "cancel_requested", "error_code", "error_message", "call_timeout_seconds", "deadline_at", "started_at", "finished_at"},
		"task_attempt":  {"id", "task_id", "attempt_no", "worker_id", "handle_ref", "started_at", "finished_at", "error_code", "error_message"},
		"task_event":    {"id", "task_id", "attempt_no", "type", "actor", "data_json", "at"},
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
	secretRefType := reflect.TypeOf(model.SecretRef{})
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

// T-step0004 — TestProviderConnectionSourceColumns (plan §2 PR 21 file 4):
// after the full step list runs, provider_connection carries the §5.4
// provenance columns and the composite (source_model, source_id) index —
// and T41's exact table set above stays untouched (12 tables, additive
// columns only).
func TestProviderConnectionSourceColumns(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	for _, column := range []string{"source_model", "source_id", "source_updated_at", "stale_source"} {
		exists, err := columnInTable(db, "provider_connection", column)
		if err != nil {
			t.Fatalf("column %q existence: %v", column, err)
		}
		if !exists {
			t.Errorf("provider_connection.%s missing after step 0004", column)
		}
	}

	// The composite (source_model, source_id) index must cover both columns.
	rows, err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'provider_connection' AND sql IS NOT NULL`).Rows()
	if err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	defer rows.Close()
	covered := map[string]bool{}
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			t.Fatalf("scan index sql: %v", err)
		}
		for _, column := range []string{"source_model", "source_id"} {
			if strings.Contains(sql, column) {
				covered[column] = true
			}
		}
	}
	for _, column := range []string{"source_model", "source_id"} {
		if !covered[column] {
			t.Errorf("no provider_connection index covers %q — §5.4 incremental checkpoint needs the composite (source_model, source_id) index", column)
		}
	}
}

func columnInTable(db *gorm.DB, table, column string) (bool, error) {
	var count int64
	// The pragma table-valued function takes its argument in call position —
	// inline the table name (a test-constant identifier, step0003 관례) rather
	// than relying on bind support inside pragma calls.
	if err := db.Raw(
		fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?`, table), column).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableOf(modelType any) string {
	type namer interface{ TableName() string }
	if n, ok := modelType.(namer); ok {
		return n.TableName()
	}
	return ""
}

// T19 — TestResourceIdentityUnique: the identity rule UNIQUE(context, kind,
// external_urn) must hold through GORM tag synthesis (r2 C / E-3a) and keep
// holding after a re-migration.
func TestResourceIdentityUnique(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.InfraResource{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	seed := model.InfraResource{UID: "res-1", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	dup := model.InfraResource{UID: "res-2", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("duplicate (context, kind, external_urn) insert was accepted — identity unique missing")
	}

	// A different urn in the same context is a distinct resource.
	other := model.InfraResource{UID: "res-3", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:2"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("same context, distinct urn insert rejected: %v", err)
	}
	// The same urn in another context is a distinct resource too.
	crossCtx := model.InfraResource{UID: "res-4", ContextID: 8, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&crossCtx).Error; err != nil {
		t.Fatalf("distinct context, same urn insert rejected: %v", err)
	}

	// E-3a — the tag-synthesized unique must survive a re-migration.
	if err := db.AutoMigrate(&model.InfraResource{}); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	dupAgain := model.InfraResource{UID: "res-5", ContextID: 7, Kind: "compute.vm", ExternalURN: "urn:vm:1"}
	if err := db.Create(&dupAgain).Error; err == nil {
		t.Fatal("identity unique did not survive re-AutoMigrate (E-3a)")
	}
}

// T20 — TestObservationCompositeIndex: (resource_id, observed_at DESC) must
// exist as a composite index (§8.2).
func TestObservationCompositeIndex(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := db.AutoMigrate(&model.ResourceObservation{}); err != nil {
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

// T41 — TestMigrationAppliesExactV2TableSet (r2 H): after the full step list
// runs, the sqlite_master table set is EXACTLY equal to the spec §3.2 listing
// under the A1 12-table ruling — no extras, no missing. This replaces the old
// grep-count check for claim 4.
func TestMigrationAppliesExactV2TableSet(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	if err := migrate.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	rows, err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Rows()
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite_master row: %v", err)
		}
		got[name] = true
	}

	// 스펙 §3.2 나열 그대로 (A1: 12종이 M1 전부 — "13" 선언 대비 1종 부족은 계획 §0.1 판정).
	// + sys_operation_log (V2 Phase 3 step0005 — plan §3.5 J3): NOT a new
	// table (보존 제약 #6 forbids a separate audit table) — the step EXTENDs
	// the existing v1 audit table with the §18.1 columns, which brings it
	// under migration management on a fresh schema.
	want := map[string]bool{
		"schema_migration":            true,
		"provider_connection":         true,
		"provider_context":            true,
		"provider_credential_binding": true,
		"secret_ref":                  true,
		"infra_resource":              true,
		"resource_observation":        true,
		"inventory_sync_run":          true,
		"infra_relationship":          true,
		"provider_task":               true,
		"task_attempt":                true,
		"task_event":                  true,
		"sys_operation_log":           true,
	}

	for name := range got {
		if !want[name] {
			t.Errorf("migration created a table outside the §3.2 set: %q", name)
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("migration is missing a §3.2 table: %q", name)
		}
	}
}
