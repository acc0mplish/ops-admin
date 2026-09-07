// Cloud-account backfill — the §5.4 propagation for the cloud sources (V2
// Phase 4 C1, plan §2 N11 / §3.3): asset_cloud_account rows become
// provider_connection + provider_context(Kind=account) + secret_ref +
// inventory binding, with the credential collapsed into ONE SecretRef
// carrying a sealed JSON blob (J4). FinOps accounts whose (normalized
// provider, accessKey) match a cloud account reuse that chain for a billing
// binding and get their provider_connection_uid link backfilled; every other
// finops account gets its own chain. Read-only on the v1 tables (their
// P-class plaintext stays until the M2 cutover — §13-9), idempotent across
// re-runs, incrementally checkpointed on source updated_at, and errors are
// isolated per account (J6): one broken account skips itself and reports,
// never halting the rest of the pipeline.
package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	v1model "ops-admin/backend/model"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/util"
)

// CloudBackfillReport is the cloud propagation result — the sync report
// artifact's cloudBackfill section. Nothing credential-shaped lives here.
type CloudBackfillReport struct {
	Created         int       `json:"created"`
	Updated         int       `json:"updated"`
	Unchanged       int       `json:"unchanged"`
	MarkedStale     int       `json:"markedStale"`
	Skipped         int       `json:"skipped"`
	CollapsedShared int       `json:"collapsedShared"`
	FinopsLinked    int       `json:"finopsLinked"`
	Failed          int       `json:"failed"`
	FailedSources   []string  `json:"failedSources"`
	Checkpoint      time.Time `json:"checkpoint"`
}

// cloudCredential is the J4 SecretRef blob shape — ONE sealed JSON document
// per credential set (the broker's Value is a single string, so the 2–3
// cloud credential components must ride together; A4).
type cloudCredential struct {
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	BillingToken string `json:"billingToken,omitempty"`
}

// assetCloudAccountRow is the read-only projection of one v1 cloud account
// row (§3.3 mapping — gorm default naming verified against
// model.AssetCloudAccount).
type assetCloudAccountRow struct {
	ID          uint
	Name        string
	Provider    string
	AccessKey   string
	SecretKey   string
	Regions     []byte
	Region      string
	Description string
	UpdatedAt   time.Time
}

// finopsAccountRow is the read-only projection of one v1 finops account row.
type finopsAccountRow struct {
	ID                    uint
	Name                  string
	Provider              string
	AccessKey             string
	SecretKey             string
	BillingToken          string
	ProviderConnectionUID string
	UpdatedAt             time.Time
}

// cloudChain holds the pieces one propagated cloud account produced — the
// collapse matching below needs the normalized provider, the decrypted
// accessKey (process memory only — 보존 제약 #3), and the chain row ids.
type cloudChain struct {
	normalizedProvider string
	accessKey          string
	secretKey          string
	connection         model.ProviderConnection
	inventoryRefID     uint
}

// RunCloudAccountBackfill propagates the v1 cloud sources into the V2
// tables (§3.3 rules 1–6):
//
//   - rule 1: provider normalization {alicloud→aliyun, tencentcloud→tencent}
//     applied to BOTH the asset_cloud_account and the finops sides.
//   - rule 2: the v1 credential columns (P-class plaintext) are sealed once —
//     decrypt-if-envelope-else-plaintext → JSON blob → EncryptSecretV2 — and
//     plaintext never leaves process memory; an unsealable component halts
//     that account only (row-scoped UNKNOWN handling, R3/J6).
//   - rule 3: finops collapse per A7 — exact credential equality shares the
//     inventory SecretRef for the billing binding; a rotated secret re-points
//     billing at its own SecretRef while reusing the connection; a different
//     accessKey gets a separate chain.
//   - rule 4: the incremental checkpoint lives on the connection's
//     source_updated_at — re-runs touch changed rows only.
//   - rule 5: v1 rows that disappeared mark their chains stale_source.
func RunCloudAccountBackfill(ctx context.Context, db *gorm.DB) (CloudBackfillReport, error) {
	report := CloudBackfillReport{}

	var accounts []assetCloudAccountRow
	err := db.WithContext(ctx).Table("asset_cloud_account").
		Select("id, name, provider, access_key, secret_key, regions, region, description, updated_at").
		Where("status = ?", 1).
		Order("id").Scan(&accounts).Error
	if err != nil {
		return report, fmt.Errorf("inventory: read asset_cloud_account (read-only source): %w", err)
	}

	chains := map[uint]*cloudChain{}
	for i := range accounts {
		row := &accounts[i]
		if row.UpdatedAt.After(report.Checkpoint) {
			report.Checkpoint = row.UpdatedAt
		}
		normalized, ok := normalizeCloudProvider(row.Provider)
		if !ok {
			report.Skipped++
			continue
		}
		credential, err := readCloudCredential(row.AccessKey, row.SecretKey, "")
		if err != nil {
			report.Failed++
			report.FailedSources = append(report.FailedSources, fmt.Sprintf("asset_cloud_account:%d", row.ID))
			continue
		}
		if credential.AccessKey == "" {
			// A registered cloud account without an AccessKey cannot back a
			// sync — defensive skip, never copied (k8s empty-kubeconfig
			// precedent).
			report.Skipped++
			continue
		}
		chain, created, updated, err := propagateCloudAccount(ctx, db, row, normalized, credential)
		if err != nil {
			report.Failed++
			report.FailedSources = append(report.FailedSources, fmt.Sprintf("asset_cloud_account:%d", row.ID))
			continue
		}
		chains[row.ID] = chain
		switch {
		case created:
			report.Created++
		case updated:
			report.Updated++
		default:
			report.Unchanged++
		}
	}

	if err := propagateFinopsAccounts(ctx, db, chains, &report); err != nil {
		return report, err
	}

	stale, err := markStaleSourceModel(ctx, db, "asset_cloud_account")
	if err != nil {
		return report, err
	}
	staleFinops, err := markStaleSourceModel(ctx, db, "integration_finops_account")
	if err != nil {
		return report, err
	}
	report.MarkedStale = stale + staleFinops
	return report, nil
}

// normalizeCloudProvider maps the legacy v1 provider vocabulary onto the V2
// provider types (J6 rule 1 — the same lexicon the legacy sync switch uses).
// Note for the R2 v2 proof machine (plan J8/E2): these literals are the
// §5.4 propagation mapping table, not a core branch — they select no
// behaviour, they only re-lexicon the source column.
func normalizeCloudProvider(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "aliyun", "alicloud":
		return "aliyun", true
	case "tencent", "tencentcloud":
		return "tencent", true
	default:
		return "", false
	}
}

// readCloudCredential assembles the J4 blob from the v1 columns. Each
// component is decrypt-if-envelope-else-plaintext (sourceKubeSecret posture):
// P-class plaintext stays plaintext in process memory, an envelope component
// is opened, and anything that claims to be an envelope but fails to open is
// the row-scoped UNKNOWN halt (§4.3 — ambiguity is reported, never
// interpreted).
func readCloudCredential(accessKey, secretKey, billingToken string) (cloudCredential, error) {
	plain := func(value string) (string, error) {
		if value == "" {
			return "", nil
		}
		if _, isEnvelope := envelopeKeyID(value); isEnvelope {
			return util.DecryptSecretV2(value)
		}
		return value, nil
	}
	out := cloudCredential{}
	var err error
	if out.AccessKey, err = plain(accessKey); err != nil {
		return out, err
	}
	if out.SecretKey, err = plain(secretKey); err != nil {
		return out, err
	}
	if out.BillingToken, err = plain(billingToken); err != nil {
		return out, err
	}
	return out, nil
}

// sealCloudCredential assembles the J4 JSON blob and seals it exactly once —
// the only SecretRef-shaped output of the backfill (§7.5, 보존 제약 #3).
func sealCloudCredential(credential cloudCredential) (keyID, ciphertext string, err error) {
	blob, err := json.Marshal(credential)
	if err != nil {
		return "", "", err
	}
	sealed, err := util.EncryptSecretV2(string(blob))
	if err != nil {
		return "", "", err
	}
	keyID, ok := envelopeKeyID(sealed)
	if !ok {
		return "", "", fmt.Errorf("sealed cloud credential is not a v2 envelope")
	}
	return keyID, sealed, nil
}

// cloudRegions parses the v1 regions JSON column into the ConfigJSON/
// MetadataJSON payload (§14.2 — regions in MetadataJSON).
func cloudRegions(raw []byte) []any {
	var regions []any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &regions); err != nil {
			return nil
		}
	}
	return regions
}

// propagateCloudAccount applies one cloud source row to the V2 tables and
// reports whether it created the chain, updated it, or left it unchanged.
func propagateCloudAccount(ctx context.Context, db *gorm.DB, row *assetCloudAccountRow, normalized string, credential cloudCredential) (*cloudChain, bool, bool, error) {
	keyID, ciphertext, err := sealCloudCredential(credential)
	if err != nil {
		return nil, false, false, fmt.Errorf("seal credential blob: %w", err)
	}

	regions := cloudRegions(row.Regions)
	connectionUID := SourceKeyUID("asset_cloud_account", row.ID)
	config := contract.JSONMap{
		"regions": regions, "region": row.Region,
		"description": row.Description, "sourceProvider": row.Provider,
	}

	var existing model.ProviderConnection
	loadErr := db.WithContext(ctx).Where("uid = ?", connectionUID).First(&existing).Error
	switch {
	case loadErr == gorm.ErrRecordNotFound:
		conn := model.ProviderConnection{
			UID: connectionUID, ProviderType: normalized,
			Name: row.Name, Endpoint: "", // A3 — cloud endpoint resolves per region at discover time
			Status: "active", ConfigJSON: config,
			SourceModel: "asset_cloud_account", SourceID: row.ID, SourceUpdatedAt: &row.UpdatedAt,
		}
		if err := db.Create(&conn).Error; err != nil {
			return nil, false, false, fmt.Errorf("create provider_connection: %w", err)
		}
		pctx := model.ProviderContext{
			UID:          SourceKeyUIDSalted("asset_cloud_account", row.ID, "context"),
			ConnectionID: conn.ID, Kind: "account",
			ExternalID: credential.AccessKey, Name: row.Name,
			Status: "active",
			MetadataJSON: contract.JSONMap{
				"regions": regions, "sourceProvider": row.Provider,
			},
		}
		if err := db.Create(&pctx).Error; err != nil {
			return nil, false, false, fmt.Errorf("create provider_context: %w", err)
		}
		ref := model.SecretRef{
			UID:        SourceKeyUIDSalted("asset_cloud_account", row.ID, "secret"),
			Backend:    "internal",
			Path:       "backfill/asset_cloud_account/" + strconvID(row.ID),
			KeyID:      keyID,
			Ciphertext: ciphertext,
		}
		if err := db.Create(&ref).Error; err != nil {
			return nil, false, false, fmt.Errorf("create secret_ref: %w", err)
		}
		binding := model.ProviderCredentialBinding{
			ProviderConnectionID: conn.ID,
			ProviderContextID:    &pctx.ID,
			Purpose:              contract.CredentialPurposeInventory,
			SecretRefID:          ref.ID,
			Status:               "active",
		}
		if err := db.Create(&binding).Error; err != nil {
			return nil, false, false, fmt.Errorf("create provider_credential_binding(inventory): %w", err)
		}
		return &cloudChain{
			normalizedProvider: normalized, accessKey: credential.AccessKey, secretKey: credential.SecretKey,
			connection: conn, inventoryRefID: ref.ID,
		}, true, false, nil
	case loadErr != nil:
		return nil, false, false, fmt.Errorf("load backfilled connection: %w", loadErr)
	}

	// §5.4a — reprocess only when the source moved past the checkpoint.
	if existing.SourceUpdatedAt != nil && !row.UpdatedAt.After(*existing.SourceUpdatedAt) {
		return &cloudChain{
			normalizedProvider: normalized, accessKey: credential.AccessKey, secretKey: credential.SecretKey,
			connection: existing, inventoryRefID: inventoryRefIDFor(ctx, db, existing.ID),
		}, false, false, nil
	}

	sourceUpdatedAt := row.UpdatedAt
	if err := db.Model(&model.ProviderConnection{}).Where("id = ?", existing.ID).
		Select("name", "config_json", "provider_type", "source_model", "source_id", "source_updated_at").
		Updates(model.ProviderConnection{
			Name: row.Name, ProviderType: normalized, ConfigJSON: config,
			SourceModel: "asset_cloud_account", SourceID: row.ID, SourceUpdatedAt: &sourceUpdatedAt,
		}).Error; err != nil {
		return nil, false, false, fmt.Errorf("update backfilled connection: %w", err)
	}
	existing.Name, existing.ProviderType = row.Name, normalized
	// Struct + Select — the context's metadata_json column serializer applies
	// to model-field writes only (map updates double-encode the JSON text).
	if err := db.Model(&model.ProviderContext{}).Where("connection_id = ?", existing.ID).
		Select("name", "external_id", "metadata_json").
		Updates(model.ProviderContext{
			Name: row.Name, ExternalID: credential.AccessKey,
			MetadataJSON: contract.JSONMap{"regions": regions, "sourceProvider": row.Provider},
		}).Error; err != nil {
		return nil, false, false, fmt.Errorf("refresh backfilled context: %w", err)
	}
	refID, err := refreshCloudSecretRef(db, existing.ID, contract.CredentialPurposeInventory, keyID, ciphertext)
	if err != nil {
		return nil, false, false, err
	}
	return &cloudChain{
		normalizedProvider: normalized, accessKey: credential.AccessKey, secretKey: credential.SecretKey,
		connection: existing, inventoryRefID: refID,
	}, false, true, nil
}

// inventoryRefIDFor loads the inventory binding's SecretRef of an existing
// connection (unchanged rows still need it for the collapse matching).
func inventoryRefIDFor(ctx context.Context, db *gorm.DB, connectionID uint) uint {
	var binding model.ProviderCredentialBinding
	if err := db.WithContext(ctx).Where("provider_connection_id = ? AND purpose = ?", connectionID, contract.CredentialPurposeInventory).
		Order("id").First(&binding).Error; err != nil {
		return 0
	}
	return binding.SecretRefID
}

// refreshCloudSecretRef re-points the connection's SecretRef for one purpose
// when the sealed material changed (rotation, re-seal — the checkpoint
// already gates this to real source updates).
func refreshCloudSecretRef(db *gorm.DB, connectionID uint, purpose, keyID, ciphertext string) (uint, error) {
	var binding model.ProviderCredentialBinding
	if err := db.Where("provider_connection_id = ? AND purpose = ?", connectionID, purpose).
		Order("id").First(&binding).Error; err != nil {
		return 0, fmt.Errorf("inventory: load backfilled credential binding: %w", err)
	}
	var ref model.SecretRef
	if err := db.First(&ref, binding.SecretRefID).Error; err != nil {
		return 0, fmt.Errorf("inventory: load backfilled secret_ref: %w", err)
	}
	if ref.Ciphertext == ciphertext {
		return ref.ID, nil
	}
	if err := db.Model(&model.SecretRef{}).Where("id = ?", ref.ID).Updates(map[string]any{
		"key_id": keyID, "ciphertext": ciphertext,
	}).Error; err != nil {
		return 0, fmt.Errorf("inventory: refresh backfilled secret_ref: %w", err)
	}
	return ref.ID, nil
}

// propagateFinopsAccounts applies the A7 collapse to the finops side.
func propagateFinopsAccounts(ctx context.Context, db *gorm.DB, chains map[uint]*cloudChain, report *CloudBackfillReport) error {
	var rows []finopsAccountRow
	err := db.WithContext(ctx).Table("integration_finops_account").
		Select("id, name, provider, access_key, secret_key, billing_token, provider_connection_uid, updated_at").
		Where("status = ?", 1).
		Order("id").Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("inventory: read integration_finops_account (read-only source): %w", err)
	}

	for i := range rows {
		row := &rows[i]
		normalized, ok := normalizeCloudProvider(row.Provider)
		if !ok {
			// aws/azure/gcp/custom have no V2 chain to bind — outside the
			// collapse vocabulary, reported as skipped.
			report.Skipped++
			continue
		}
		credential, err := readCloudCredential(row.AccessKey, row.SecretKey, row.BillingToken)
		if err != nil {
			report.Failed++
			report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
			continue
		}
		if credential.AccessKey == "" {
			report.Skipped++
			continue
		}

		// A7 — collapse matching on (normalized provider, accessKey) against
		// the cloud chains of this run.
		var match *cloudChain
		for _, chain := range chains {
			if chain != nil && chain.normalizedProvider == normalized && chain.accessKey == credential.AccessKey {
				match = chain
				break
			}
		}

		var conn model.ProviderConnection
		if match != nil {
			conn = match.connection
			// Exact credential equality → share the inventory SecretRef for
			// the billing binding (§7.4 "same SecretRef where shared").
			if match.secretKey == credential.SecretKey {
				created, err := ensureBillingBinding(db, match.connection.ID, match.inventoryRefID)
				if err != nil {
					report.Failed++
					report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
					continue
				}
				if created {
					report.CollapsedShared++
				}
			} else {
				// accessKey matches, secret/token differ — the §7.4 sharing
				// rule forbids one SecretRef for different material: the
				// billing binding points at its own SecretRef under the
				// reused connection.
				keyID, ciphertext, err := sealCloudCredential(credential)
				if err != nil {
					report.Failed++
					report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
					continue
				}
				refID, err := ensureBillingSecretRef(db, SourceKeyUIDSalted("asset_cloud_account", match.connection.SourceID, "secret-billing"), "backfill/asset_cloud_account/"+strconvID(match.connection.SourceID)+"/billing", keyID, ciphertext)
				if err != nil {
					report.Failed++
					report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
					continue
				}
				if _, err := ensureBillingBinding(db, match.connection.ID, refID); err != nil {
					report.Failed++
					report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
					continue
				}
			}
		} else {
			// No cloud account matches — the finops account gets its own
			// chain (provider/accessKey mismatch branch of A7).
			chain, created, err := propagateFinopsOnlyChain(ctx, db, row, normalized, credential)
			if err != nil {
				report.Failed++
				report.FailedSources = append(report.FailedSources, fmt.Sprintf("integration_finops_account:%d", row.ID))
				continue
			}
			conn = chain
			if created {
				report.Created++
			}
		}

		// The §5.4 finops link — written only when it actually moves.
		if row.ProviderConnectionUID != conn.UID {
			if err := db.Model(&v1model.IntegrationFinOpsAccount{}).Where("id = ?", row.ID).
				Update("provider_connection_uid", conn.UID).Error; err != nil {
				return fmt.Errorf("inventory: backfill finops link for account %d: %w", row.ID, err)
			}
			report.FinopsLinked++
		}
	}
	return nil
}

// propagateFinopsOnlyChain builds a standalone chain for a finops account
// that matches no cloud account.
func propagateFinopsOnlyChain(ctx context.Context, db *gorm.DB, row *finopsAccountRow, normalized string, credential cloudCredential) (model.ProviderConnection, bool, error) {
	keyID, ciphertext, err := sealCloudCredential(credential)
	if err != nil {
		return model.ProviderConnection{}, false, fmt.Errorf("seal credential blob: %w", err)
	}
	connectionUID := SourceKeyUID("integration_finops_account", row.ID)
	var conn model.ProviderConnection
	loadErr := db.WithContext(ctx).Where("uid = ?", connectionUID).First(&conn).Error
	if loadErr == nil {
		if err := db.Model(&model.ProviderConnection{}).Where("id = ?", conn.ID).
			Select("name").Updates(model.ProviderConnection{Name: row.Name}).Error; err != nil {
			return conn, false, fmt.Errorf("update backfilled finops connection: %w", err)
		}
		// The finops-only chain carries its credential through the billing
		// purpose, not inventory.
		if _, err := refreshCloudSecretRef(db, conn.ID, contract.CredentialPurposeBilling, keyID, ciphertext); err != nil {
			return conn, false, err
		}
		return conn, false, nil
	}
	if loadErr != nil && loadErr != gorm.ErrRecordNotFound {
		return model.ProviderConnection{}, false, fmt.Errorf("load backfilled finops connection: %w", loadErr)
	}

	conn = model.ProviderConnection{
		UID: connectionUID, ProviderType: normalized,
		Name: row.Name, Endpoint: "", Status: "active",
		ConfigJSON:  contract.JSONMap{"sourceProvider": row.Provider, "purpose": "billing"},
		SourceModel: "integration_finops_account", SourceID: row.ID, SourceUpdatedAt: &row.UpdatedAt,
	}
	if err := db.Create(&conn).Error; err != nil {
		return model.ProviderConnection{}, false, fmt.Errorf("create provider_connection: %w", err)
	}
	pctx := model.ProviderContext{
		UID:          SourceKeyUIDSalted("integration_finops_account", row.ID, "context"),
		ConnectionID: conn.ID, Kind: "account",
		ExternalID: credential.AccessKey, Name: row.Name,
		Status:       "active",
		MetadataJSON: contract.JSONMap{"sourceProvider": row.Provider},
	}
	if err := db.Create(&pctx).Error; err != nil {
		return model.ProviderConnection{}, false, fmt.Errorf("create provider_context: %w", err)
	}
	ref := model.SecretRef{
		UID:        SourceKeyUIDSalted("integration_finops_account", row.ID, "secret"),
		Backend:    "internal",
		Path:       "backfill/integration_finops_account/" + strconvID(row.ID),
		KeyID:      keyID,
		Ciphertext: ciphertext,
	}
	if err := db.Create(&ref).Error; err != nil {
		return model.ProviderConnection{}, false, fmt.Errorf("create secret_ref: %w", err)
	}
	binding := model.ProviderCredentialBinding{
		ProviderConnectionID: conn.ID,
		ProviderContextID:    &pctx.ID,
		Purpose:              contract.CredentialPurposeBilling,
		SecretRefID:          ref.ID,
		Status:               "active",
	}
	if err := db.Create(&binding).Error; err != nil {
		return model.ProviderConnection{}, false, fmt.Errorf("create provider_credential_binding(billing): %w", err)
	}
	return conn, true, nil
}

// ensureBillingSecretRef idempotently materializes the billing-purpose
// SecretRef a rotated finops credential needs under a shared connection.
func ensureBillingSecretRef(db *gorm.DB, uid, path, keyID, ciphertext string) (uint, error) {
	var ref model.SecretRef
	err := db.Where("uid = ?", uid).First(&ref).Error
	switch {
	case err == nil:
		if ref.Ciphertext == ciphertext {
			return ref.ID, nil
		}
		if err := db.Model(&model.SecretRef{}).Where("id = ?", ref.ID).Updates(map[string]any{
			"key_id": keyID, "ciphertext": ciphertext,
		}).Error; err != nil {
			return 0, fmt.Errorf("inventory: refresh billing secret_ref: %w", err)
		}
		return ref.ID, nil
	case err == gorm.ErrRecordNotFound:
		ref = model.SecretRef{UID: uid, Backend: "internal", Path: path, KeyID: keyID, Ciphertext: ciphertext}
		if err := db.Create(&ref).Error; err != nil {
			return 0, fmt.Errorf("inventory: create billing secret_ref: %w", err)
		}
		return ref.ID, nil
	default:
		return 0, fmt.Errorf("inventory: load billing secret_ref: %w", err)
	}
}

// ensureBillingBinding idempotently points the connection's billing purpose
// at secretRefID. It reports whether it created the row (the collapse count).
func ensureBillingBinding(db *gorm.DB, connectionID, secretRefID uint) (bool, error) {
	var binding model.ProviderCredentialBinding
	err := db.Where("provider_connection_id = ? AND purpose = ?", connectionID, contract.CredentialPurposeBilling).
		Order("id").First(&binding).Error
	switch {
	case err == nil:
		if binding.SecretRefID == secretRefID {
			return false, nil
		}
		if err := db.Model(&model.ProviderCredentialBinding{}).Where("id = ?", binding.ID).
			Update("secret_ref_id", secretRefID).Error; err != nil {
			return false, fmt.Errorf("inventory: re-point billing binding: %w", err)
		}
		return false, nil
	case err == gorm.ErrRecordNotFound:
		binding = model.ProviderCredentialBinding{
			ProviderConnectionID: connectionID,
			Purpose:              contract.CredentialPurposeBilling,
			SecretRefID:          secretRefID,
			Status:               "active",
		}
		if err := db.Create(&binding).Error; err != nil {
			return false, fmt.Errorf("inventory: create billing binding: %w", err)
		}
		return true, nil
	default:
		return false, fmt.Errorf("inventory: load billing binding: %w", err)
	}
}

// markStaleSourceModel — §5.4b for the cloud sources: v1 rows that
// disappeared mark their V2 connections stale_source=true (marked, never
// deleted; already-stale rows are not re-counted).
func markStaleSourceModel(ctx context.Context, db *gorm.DB, sourceModel string) (int, error) {
	res := db.WithContext(ctx).Model(&model.ProviderConnection{}).
		Where("source_model = ? AND stale_source = ? AND source_id NOT IN (?)",
			sourceModel, false, db.Table(sourceModel).Select("id")).
		Update("stale_source", true)
	if res.Error != nil {
		return 0, fmt.Errorf("inventory: mark stale_source connections (%s): %w", sourceModel, res.Error)
	}
	return int(res.RowsAffected), nil
}
