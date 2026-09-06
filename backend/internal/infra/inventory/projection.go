// Public read-side projection over the V2 inventory tables (plan §2 PR 22
// file 2, §3.5 r2 공개 generation 규약): the compare CLI reads the shadow
// state back through these queries — never through internals — and PR 23's
// GET /resources consumes the same projection (§3.8).
package inventory

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// PublicGeneration identifies the published read point (§3.5 r2): the
// GenerationUID of the latest InventorySyncRun whose pages all completed —
// CommittedAt IS NOT NULL and Status = succeeded. Observations are 1:1 with
// runs (§8.2 "Every normalized record carries a generation"), so the run UID
// is the generation UID.
type PublicGeneration struct {
	GenerationUID string
	CommittedAt   time.Time
}

// LatestAuthoritativeGeneration selects the public generation of one context:
// the newest committed succeeded run (CommittedAt DESC). A context without a
// committed run has no public generation — the caller must not compare.
func LatestAuthoritativeGeneration(db *gorm.DB, contextID uint) (PublicGeneration, error) {
	var run model.InventorySyncRun
	err := db.Where("context_id = ? AND status = ? AND committed_at IS NOT NULL", contextID, RunStatusSucceeded).
		Order("committed_at DESC, id DESC").First(&run).Error
	if err != nil {
		if isRecordNotFound(err) {
			return PublicGeneration{}, fmt.Errorf("inventory: no authoritative generation for context %d — run sync-inventory first", contextID)
		}
		return PublicGeneration{}, fmt.Errorf("inventory: load authoritative generation: %w", err)
	}
	return PublicGeneration{GenerationUID: run.UID, CommittedAt: *run.CommittedAt}, nil
}

// ProjectedResource is one row of the public projection: the live resource
// with the observation that carries its current published state.
type ProjectedResource struct {
	UID           string
	Kind          string
	Subtype       string
	ExternalID    string
	DisplayName   string
	ExternalURN   string
	GenerationUID string // the generation whose observation holds the state below
	ObservedAt    time.Time
	Normalized    contract.JSONMap
	Raw           contract.JSONMap
}

// ProjectResources reads the public inventory of one context (§15.1 "read
// back through the V2 projection queries"): live resources joined to their
// latest observation from a PUBLISHED generation, with stale_source
// connections reading as absent (§5.4c, A5 — context is 1:1 with its
// connection, so the stale flag filters at context granularity).
//
// Unchanged resources keep the observation of the older generation that last
// changed them, which is their current published state; observations from
// partial/failed runs are excluded by the subquery, so a partial generation
// never leaks into the projection (N8).
func ProjectResources(db *gorm.DB, contextID uint, generationUID string) ([]ProjectedResource, error) {
	// §5.4c — stale_source reads as absent.
	var stale int64
	err := db.Model(&model.ProviderContext{}).
		Joins("JOIN provider_connection ON provider_connection.id = provider_context.connection_id").
		Where("provider_context.id = ? AND provider_connection.stale_source = ?", contextID, true).
		Count(&stale).Error
	if err != nil {
		return nil, fmt.Errorf("inventory: stale_source guard: %w", err)
	}
	if stale > 0 {
		return nil, nil
	}

	var resources []model.InfraResource
	if err := db.Where("context_id = ? AND deleted_at IS NULL", contextID).
		Order("kind, subtype, external_id").Find(&resources).Error; err != nil {
		return nil, fmt.Errorf("inventory: project resources: %w", err)
	}
	if len(resources) == 0 {
		return []ProjectedResource{}, nil
	}
	ids := make([]uint, 0, len(resources))
	for _, res := range resources {
		ids = append(ids, res.ID)
	}

	// Observations from published generations only (N8), newest first; the
	// first hit per resource is its published state.
	published := db.Model(&model.InventorySyncRun{}).Select("uid").
		Where("context_id = ? AND status = ? AND committed_at IS NOT NULL", contextID, RunStatusSucceeded)
	var observations []model.ResourceObservation
	if err := db.Where("resource_id IN ? AND generation_uid IN (?)", ids, published).
		Order("observed_at DESC, id DESC").Find(&observations).Error; err != nil {
		return nil, fmt.Errorf("inventory: project observations: %w", err)
	}
	latest := make(map[uint]model.ResourceObservation, len(resources))
	for _, obs := range observations {
		if _, seen := latest[obs.ResourceID]; !seen {
			latest[obs.ResourceID] = obs
		}
	}

	rows := make([]ProjectedResource, 0, len(resources))
	for _, res := range resources {
		obs, ok := latest[res.ID]
		if !ok {
			// Never part of a published generation — not on the public read
			// side yet.
			continue
		}
		rows = append(rows, ProjectedResource{
			UID: res.UID, Kind: res.Kind, Subtype: res.Subtype,
			ExternalID: res.ExternalID, DisplayName: res.DisplayName, ExternalURN: res.ExternalURN,
			GenerationUID: obs.GenerationUID, ObservedAt: obs.ObservedAt,
			Normalized: obs.NormalizedJSON, Raw: obs.RawJSON,
		})
	}
	return rows, nil
}
