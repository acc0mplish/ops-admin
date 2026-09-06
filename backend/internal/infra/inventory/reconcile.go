// Reconciliation vocabulary and per-resource state transitions — plan §3.3
// (§9.3 outcome 어휘 verbatim 10종, r2 정정) over the §8.1 resource rows.
package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
)

// isRecordNotFound unwraps the gorm not-found sentinel (First vs Take differ
// in wrapping only pre-2.x; errors.Is covers both).
func isRecordNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// §9.3 outcome vocabulary — verbatim 10종 (r2 정정: 스펙 어휘 그대로).
const (
	OutcomeCreated            = "created"
	OutcomeUpdated            = "updated"
	OutcomeUnchanged          = "unchanged"
	OutcomeStaleCandidate     = "stale_candidate"
	OutcomeTombstoned         = "tombstoned"
	OutcomeIdentityConflict   = "identity_conflict"
	OutcomeNormalizationFail  = "normalization_failed"
	OutcomePermissionDenied   = "permission_denied"
	OutcomeRateLimited        = "rate_limited"
	OutcomePartial            = "partial"
	outcomeVocabularyCapacity = 10
)

// newOutcomeMap returns the §9.3 outcome map with every key present at zero —
// report consumers read counts, not key existence.
func newOutcomeMap() map[string]int {
	return make(map[string]int, outcomeVocabularyCapacity)
}

// Run statuses — the InventorySyncRun.Status values Phase 2 uses (§9.1/9.2).
const (
	RunStatusRunning   = "running"
	RunStatusPartial   = "partial"
	RunStatusSucceeded = "succeeded"
	RunStatusFailed    = "failed"
)

const (
	// StaleCandidateAfterAbsences — §9.2 "repeated absence": 2회 연속 부재.
	StaleCandidateAfterAbsences = 2
	// TombstoneAfterAbsences — tombstone은 임계+유예 후 (기본 3회, plan §3.3).
	TombstoneAfterAbsences = 3
)

// absenceLabelKey — the consecutive-absence counter lives inside the
// resource's LabelsJSON under this reserved key. §8.1 labels carry provider
// labels; a dedicated column was rejected (additive schema stays at step
// 0004's four columns), so the counter rides the existing JSON column under
// a namespaced key the V2 read side never surfaces as a label.
const absenceLabelKey = "v2.absenceRuns"

// resourceSeen records what one completed run observed for the linkage and
// absence passes.
type resourceSeen struct {
	id      uint
	kind    string
	subtype string
	name    string
	raw     contract.JSONMap
}

// observationHash — the §8.2 payload hash the unchanged/updated split reads.
// Both payload maps marshal deterministically (sorted keys via json.Marshal
// on a flat map is not sorted, so hash the re-marshalled canonical form).
func observationHash(res contract.DiscoveredResource) string {
	payload := contract.JSONMap{
		"urn":        res.ExternalURN,
		"kind":       res.Kind,
		"subtype":    res.Subtype,
		"normalized": res.Normalized,
		"raw":        res.Raw,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// resourceStates extracts the lifecycle/health vocabulary from the
// normalized payload (pod phase 그대로, node conditions → healthy/degraded —
// the adapter normalizer's state keys).
func resourceStates(res contract.DiscoveredResource) (lifecycle, health string) {
	if res.Normalized != nil {
		lifecycle, _ = res.Normalized["lifecycleState"].(string)
		health, _ = res.Normalized["healthState"].(string)
	}
	return lifecycle, health
}

func absenceCountOf(res *model.InfraResource) int {
	if res.LabelsJSON == nil {
		return 0
	}
	n, _ := res.LabelsJSON[absenceLabelKey].(float64)
	return int(n)
}

// withAbsenceCount returns a copy of the label map with the absence counter
// set (or removed at zero) — immutability per coding style.
func withAbsenceCount(labels contract.JSONMap, n int) contract.JSONMap {
	out := make(contract.JSONMap, len(labels)+1)
	for k, v := range labels {
		out[k] = v
	}
	if n <= 0 {
		delete(out, absenceLabelKey)
	} else {
		out[absenceLabelKey] = n
	}
	return out
}

// latestObservationHash loads the most recent observation hash of a resource
// (empty when it has none).
func latestObservationHash(db *gorm.DB, resourceID uint) (string, bool, error) {
	var obs model.ResourceObservation
	err := db.Where("resource_id = ?", resourceID).Order("observed_at DESC, id DESC").First(&obs).Error
	if err != nil {
		if isRecordNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return obs.ObservationHash, true, nil
}

// storeObservation writes one generation-stamped observation row.
func storeObservation(db *gorm.DB, resourceID uint, generationUID string, res contract.DiscoveredResource, at time.Time) error {
	return db.Create(&model.ResourceObservation{
		ResourceID:      resourceID,
		GenerationUID:   generationUID,
		ObservationHash: observationHash(res),
		NormalizedJSON:  res.Normalized,
		RawJSON:         res.Raw,
		ObservedAt:      at,
	}).Error
}

// clearAbsence resets the absence ladder on a re-sighting: the counter is
// dropped and an orphaned resource returns to discovered (tombstoned rows
// stay tombstoned — §9.3 tombstone is terminal for Phase 2).
func clearAbsence(res *model.InfraResource) {
	res.LabelsJSON = withAbsenceCount(res.LabelsJSON, 0)
	if res.ManagedState == "orphaned" {
		res.ManagedState = "discovered"
	}
}

// noteAbsence advances the absence ladder for one completed generation
// without the resource. Returns the outcome it produced ("" when the ladder
// did not fire): 2nd consecutive absence → stale_candidate + orphaned, 3rd
// → tombstoned + soft-delete.
func noteAbsence(db *gorm.DB, res *model.InfraResource, now time.Time) string {
	if res.DeletedAt != nil || res.ManagedState == "tombstoned" {
		return ""
	}
	count := absenceCountOf(res) + 1
	res.LabelsJSON = withAbsenceCount(res.LabelsJSON, count)
	switch {
	case count >= TombstoneAfterAbsences:
		res.ManagedState = "tombstoned"
		res.DeletedAt = &now
		return OutcomeTombstoned
	case count >= StaleCandidateAfterAbsences:
		res.ManagedState = "orphaned"
		return OutcomeStaleCandidate
	default:
		return ""
	}
}

// deriveRelationships builds the §8.4 discovery relationships for one
// completed run from adapter linkage metadata: pod Raw["nodeName"] → node
// runs_on, pvc Raw["volumeName"] → pv backed_by. The pass indexes the whole
// collected run, so the linkage fires regardless of page boundaries; the
// generation stamp is the run UID (§8.4).
func (r *SyncRunner) deriveRelationships(db *gorm.DB, generationUID string, seen []resourceSeen, at time.Time) error {
	nodes := map[string]uint{}   // display name → resource id (orchestration.node)
	volumes := map[string]uint{} // display name → resource id (persistent_volume subtype)
	for _, s := range seen {
		if s.kind == "orchestration.node" {
			nodes[s.name] = s.id
		}
		if s.kind == "storage.volume" && s.subtype == "persistent_volume" {
			volumes[s.name] = s.id
		}
	}

	type link struct {
		from uint
		to   uint
		typ  string
	}
	links := make([]link, 0, 2)
	for _, s := range seen {
		switch s.kind {
		case "orchestration.pod":
			if nodeName, _ := s.raw["nodeName"].(string); nodeName != "" {
				if to, ok := nodes[nodeName]; ok {
					links = append(links, link{from: s.id, to: to, typ: "runs_on"})
				}
			}
		case "storage.volume":
			// The referring side is the volumeName carrier — the pv target
			// never carries it, so the Raw metadata alone disambiguates.
			if volumeName, _ := s.raw["volumeName"].(string); volumeName != "" {
				if to, ok := volumes[volumeName]; ok {
					links = append(links, link{from: s.id, to: to, typ: "backed_by"})
				}
			}
		}
	}

	for _, l := range links {
		var existing model.InfraRelationship
		err := db.Where("from_resource_id = ? AND to_resource_id = ? AND type = ?", l.from, l.to, l.typ).
			First(&existing).Error
		switch {
		case err == nil:
			existing.GenerationUID = generationUID
			existing.LastSeenAt = at
			if err := db.Save(&existing).Error; err != nil {
				return err
			}
		case isRecordNotFound(err):
			rel := model.InfraRelationship{
				FromResourceID: l.from, ToResourceID: l.to, Type: l.typ,
				Source: "discovery", GenerationUID: generationUID, LastSeenAt: at,
			}
			if err := db.Create(&rel).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}
