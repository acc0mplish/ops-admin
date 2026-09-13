package main

// main_unregister_k8s.go — the "unregister-k8s" subcommand, the reverse of
// register-k8s (main_register_k8s.go). It tears the registration chain down
// in one transaction: the active-task guard → provider_credential_binding →
// provider_context → sealed kubeconfig SecretRef → provider_connection, then
// tombstones the inventory resources under the context (ManagedState
// "tombstoned" + DeletedAt, model/inventory.go:25 vocabulary — the state
// reconcile.go treats as terminal) while the observation/sync-run/task
// history rows survive untouched.
//
// The delete path never reads credential material: the SecretRef row is
// burned whole, so no kubeconfig fragment can reach flags, logs, errors, or
// the stdout report (C58 posture by construction).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ops-admin/backend/config"
	"ops-admin/backend/internal/infra/migrate"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/store"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

// k8sTaskTerminalStatuses is the provider_task.Status terminal set — the
// CASE list of the step0003 active_flag DDL (model/task.go:46) verbatim. A
// connection with a task outside this set is still in play and must not be
// unregistered.
var k8sTaskTerminalStatuses = []string{"succeeded", "failed", "timed_out", "cancelled"}

// k8sManagedStateTombstoned — the ManagedState vocabulary entry for a
// soft-deleted resource (model/inventory.go:25). §9.3: tombstone is terminal
// — a later sync never resurrects the row (inventory/reconcile.go).
const k8sManagedStateTombstoned = "tombstoned"

// k8sUnregisterReport is the stdout artifact — k8sRegisterReport 승계 with
// the delete outcome. The struct is fixed, and the delete path reads no
// credential material, so nothing else can ride it.
type k8sUnregisterReport struct {
	ConnectionUID       string `json:"connectionUid"`
	ContextUID          string `json:"contextUid"`
	Existed             bool   `json:"existed"`
	TombstonedResources int    `json:"tombstonedResources"`
}

// runUnregisterK8s implements the "unregister-k8s" command: parse flags,
// stop at the --confirm gate before any IO, open the database through the
// standard startup path (config → secret-key gate → migrate), then hand the
// delete to unregisterK8sInDB.
func runUnregisterK8s(args []string) int {
	flags := flag.NewFlagSet("unregister-k8s", flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "path to config.yaml")
	name := flags.String("name", "", "connection name — the register-k8s idempotency key (required)")
	confirm := flags.Bool("confirm", false, "acknowledge the irreversible delete of the chain and the sealed kubeconfig ciphertext")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: %v\n", err)
		return 1
	}
	if *name == "" {
		fmt.Fprintln(os.Stderr, "unregister-k8s: --name is required")
		return 1
	}
	// Destructive gate before any IO — the reencrypt-secrets
	// --backup-acknowledged 선례(main_reencrypt.go): a safety acknowledgment
	// flag is consumed before config or the database are touched.
	if !*confirm {
		fmt.Fprintln(os.Stderr, "unregister-k8s: --confirm is required: this deletes the connection/context/binding chain and burns the sealed kubeconfig ciphertext, irreversibly")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: load config: %v\n", err)
		return 1
	}
	util.ConfigureCredentialKey(cfg.Security.CredentialKey)
	if err := util.EnsureSecretKeySource(cfg.Security.CredentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: secret key source: %v\n", err)
		return 1
	}
	db, err := store.NewDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: connect db: %v\n", err)
		return 1
	}
	if err := migrate.Run(context.Background(), db); err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: v2 schema migration: %v\n", err)
		return 1
	}

	report, err := unregisterK8sInDB(db, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "unregister-k8s: encode report: %v\n", err)
		return 1
	}
	return 0
}

// unregisterK8sInDB deletes the registration chain of name. A missing chain
// is not an error — the report carries existed:false (the register-k8s upsert
// 멱등 철학 승계: the desired end state is already reached), and nothing is
// written. The delete itself is one transaction — 부분 삭제 불가.
func unregisterK8sInDB(db *gorm.DB, name string) (k8sUnregisterReport, error) {
	report := k8sUnregisterReport{
		ConnectionUID: k8sRegisterUID(name, ""),
		ContextUID:    k8sRegisterUID(name, "context"),
	}
	if strings.TrimSpace(name) == "" {
		return report, fmt.Errorf("name is required")
	}

	var conn model.ProviderConnection
	err := db.Where("uid = ?", report.ConnectionUID).First(&conn).Error
	if isRecordNotFound(err) {
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("load provider_connection: %w", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// ① 가드 — engine_resolve.go가 conn:<connectionUID>:… 합성 resource_uid를
		// 예약하므로, 커넥션 스코프 태스크는 그 접두로 잡는다. 비종단 행이 하나라도
		// 있으면 삭제를 거부한다(종단 집합은 model/task.go:46 DDL CASE와 동치).
		var active int64
		if err := tx.Model(&model.ProviderTask{}).
			Where("resource_uid LIKE ? AND status NOT IN ?", "conn:"+report.ConnectionUID+":%", k8sTaskTerminalStatuses).
			Count(&active).Error; err != nil {
			return fmt.Errorf("count non-terminal tasks: %w", err)
		}
		if active > 0 {
			return fmt.Errorf("connection has %d non-terminal task(s); settle them before unregistering", active)
		}

		// context 행은 톰스톤 키(context_id)로 쓰이므로 삭제 전에 읽는다.
		var pctx model.ProviderContext
		if err := tx.Where("uid = ?", report.ContextUID).First(&pctx).Error; err != nil {
			return fmt.Errorf("load provider_context: %w", err)
		}

		// ②③④⑤ 역순 삭제 — 등록 체인의 역방향.
		if err := tx.Where("provider_connection_id = ?", conn.ID).
			Delete(&model.ProviderCredentialBinding{}).Error; err != nil {
			return fmt.Errorf("delete provider_credential_binding: %w", err)
		}
		if err := tx.Where("uid = ?", report.ContextUID).Delete(&model.ProviderContext{}).Error; err != nil {
			return fmt.Errorf("delete provider_context: %w", err)
		}
		if err := tx.Where("uid = ?", k8sRegisterUID(name, "secret")).Delete(&model.SecretRef{}).Error; err != nil {
			return fmt.Errorf("delete secret_ref: %w", err)
		}
		if err := tx.Where("uid = ?", report.ConnectionUID).Delete(&model.ProviderConnection{}).Error; err != nil {
			return fmt.Errorf("delete provider_connection: %w", err)
		}

		// ⑥ 톰스톤 — 행은 남기고 ManagedState+DeletedAt만 세팅한다. 이미 톰스톤인
		// 행은 WHERE에서 걸러 세지 않는다: matched==changed가 백엔드 불문 성립해야
		// RowsAffected 카운트가 sqlite(MySQL FOUND_ROWS 옵션 부재)와도 결정적이다.
		// resource_observation·inventory_sync_run·provider_task·task_event·
		// task_attempt는 이 갱신 대상이 아니다 — 이력은 보존이다.
		res := tx.Model(&model.InfraResource{}).
			Where("context_id = ? AND (managed_state <> ? OR deleted_at IS NULL)", pctx.ID, k8sManagedStateTombstoned).
			Updates(map[string]any{"managed_state": k8sManagedStateTombstoned, "deleted_at": time.Now().UTC()})
		if res.Error != nil {
			return fmt.Errorf("tombstone infra_resource: %w", res.Error)
		}
		report.TombstonedResources = int(res.RowsAffected)
		return nil
	})
	if err != nil {
		return report, err
	}
	report.Existed = true
	return report, nil
}
