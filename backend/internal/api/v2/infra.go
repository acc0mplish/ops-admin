// Package v2 is the Phase 2 read-only infra API (plan PR 23 — spec §16.1
// GET subset, J7): provider-types, provider-connections, resources, and the
// resource detail. Every handler is a GET over the V2 tables only — no
// opdef grant is attached (non-sensitive reads; the v1 route group stays
// untouched), the router wraps the group in Auth + OperationLog, secret
// material never leaves the secrets tables (claim 14 — T52 redaction), and
// stale_source rows are treated absent on the read side (§5.4c).
//
// The planned landing for the resource read is the PR 22 projection
// (inventory/projection.go §3.5 public-generation rule); PR 22 lands in a
// parallel worktree, so this package carries the same semantics locally —
// latest committed succeeded run per context, stale connections joined away
// — and the convergence note lives at listResources.
package v2

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/internal/infra/compose"
	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// InfraAPI serves the V2 read routes over one database handle and the
// shared adapter registry (compose.Build — adapters register exactly once).
type InfraAPI struct {
	db       *gorm.DB
	registry *registry.Registry
}

// NewInfraAPI assembles the V2 stack over db. Registration-time failures are
// programming errors (the adapter set is compile-time static), so instead of
// refusing to boot the whole v1 process the API degrades to a nil registry
// and the provider-type route reports the outage.
func NewInfraAPI(db *gorm.DB) *InfraAPI {
	stack, err := compose.Build(db)
	if err != nil {
		log.Printf("infra v2: stack build failed: %v", err)
		return NewInfraAPIWithRegistry(db, nil)
	}
	return NewInfraAPIWithRegistry(db, stack.Registry)
}

// NewInfraAPIWithRegistry builds the API over an explicit registry (the
// degraded nil-registry path is the stack-build-failure case above).
func NewInfraAPIWithRegistry(db *gorm.DB, reg *registry.Registry) *InfraAPI {
	return &InfraAPI{db: db, registry: reg}
}

// Register wires the GET subset onto group. The group is created by the
// router with Auth + OperationLog already attached; no opdef middleware is
// applied here (plan J7 — GET-only, non-sensitive).
func (a *InfraAPI) Register(group *gin.RouterGroup) {
	group.GET("/provider-types", a.ListProviderTypes)
	group.GET("/provider-connections", a.ListProviderConnections)
	group.GET("/resources", a.ListResources)
	group.GET("/resources/:uid", a.GetResource)
}

// providerTypeView is the §16.1 provider-types row — descriptor fields only;
// the typed ConfigSchema builder stays server-side (schema UI is Phase 3).
type providerTypeView struct {
	Type            string   `json:"type"`
	AdapterVersion  string   `json:"adapterVersion"`
	ProtocolVersion string   `json:"protocolVersion"`
	ContextKinds    []string `json:"contextKinds"`
	BuiltIn         bool     `json:"builtIn"`
}

// ListProviderTypes lists the registered provider descriptors from the
// shared registry (§3.8 — registry 조회만, no provider branching).
func (a *InfraAPI) ListProviderTypes(c *gin.Context) {
	if a.registry == nil {
		httpx.Failed(c, http.StatusServiceUnavailable, "infra v2 stack unavailable")
		return
	}
	names := a.registry.ProviderTypeNames()
	items := make([]providerTypeView, 0, len(names))
	for _, name := range names {
		descriptor, _, ok := a.registry.ProviderType(name)
		if !ok {
			continue
		}
		items = append(items, providerTypeView{
			Type:            descriptor.Type,
			AdapterVersion:  descriptor.AdapterVersion,
			ProtocolVersion: descriptor.ProtocolVersion,
			ContextKinds:    descriptor.ContextKinds,
			BuiltIn:         descriptor.BuiltIn,
		})
	}
	httpx.Success(c, gin.H{"items": items, "total": len(items)})
}

// providerConnectionView is the §16.1 connection row: identity, posture and
// source provenance only. ConfigJSON is withheld entirely (it carries
// connection_mode/gateway posture keys today but is free-form by §7.2), and
// the secret is surfaced as the inventory-purpose SecretRef UID alone —
// never ciphertext, never key material (claim 14).
type providerConnectionView struct {
	UID          string     `json:"uid"`
	ProviderType string     `json:"providerType"`
	Name         string     `json:"name"`
	Endpoint     string     `json:"endpoint"`
	Status       string     `json:"status"`
	Version      string     `json:"version"`
	GatewayID    *uint      `json:"gatewayId"`
	SourceModel  string     `json:"sourceModel"`
	SourceID     uint       `json:"sourceId"`
	SecretRefUID string     `json:"secretRefUid"`
	LastHealthAt *time.Time `json:"lastHealthAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ListProviderConnections lists live (§5.4c — stale_source rows treated
// absent on the whole V2 read side) provider connections with the inventory
// secret reduced to its UID.
func (a *InfraAPI) ListProviderConnections(c *gin.Context) {
	var rows []model.ProviderConnection
	if err := a.db.Where("stale_source = ?", false).Order("id").Find(&rows).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]providerConnectionView, 0, len(rows))
	for _, row := range rows {
		view := providerConnectionView{
			UID:          row.UID,
			ProviderType: row.ProviderType,
			Name:         row.Name,
			Endpoint:     row.Endpoint,
			Status:       row.Status,
			Version:      row.Version,
			GatewayID:    row.GatewayID,
			SourceModel:  row.SourceModel,
			SourceID:     row.SourceID,
			LastHealthAt: row.LastHealthAt,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		view.SecretRefUID = a.inventorySecretRefUID(row.ID)
		items = append(items, view)
	}
	httpx.Success(c, gin.H{"items": items, "total": len(items)})
}

// inventorySecretRefUID resolves the UID of the connection's inventory-purpose
// secret reference (§7.4 Purpose vocabulary; the broker stays the only
// decryptor — this route exposes the reference, not the material).
func (a *InfraAPI) inventorySecretRefUID(connectionID uint) string {
	var binding model.ProviderCredentialBinding
	err := a.db.Where("provider_connection_id = ? AND purpose = ?", connectionID, "inventory").First(&binding).Error
	if err != nil {
		return ""
	}
	var ref model.SecretRef
	if err := a.db.Select("uid").Where("id = ?", binding.SecretRefID).First(&ref).Error; err != nil {
		return ""
	}
	return ref.UID
}

// resourceView is the §16.1 resource row — normalized posture and lifecycle
// timestamps; labels come from the adapter raw keys (§8.1) and carry no
// secret material (secret observations are metadata-only upstream).
type resourceView struct {
	UID            string           `json:"uid"`
	Kind           string           `json:"kind"`
	Subtype        string           `json:"subtype"`
	ExternalURN    string           `json:"externalUrn"`
	DisplayName    string           `json:"displayName"`
	LifecycleState string           `json:"lifecycleState"`
	HealthState    string           `json:"healthState"`
	ManagedState   string           `json:"managedState"`
	Labels         contract.JSONMap `json:"labels"`
	FirstSeenAt    time.Time        `json:"firstSeenAt"`
	LastSeenAt     time.Time        `json:"lastSeenAt"`
}

func newResourceView(res model.InfraResource) resourceView {
	return resourceView{
		UID:            res.UID,
		Kind:           res.Kind,
		Subtype:        res.Subtype,
		ExternalURN:    res.ExternalURN,
		DisplayName:    res.DisplayName,
		LifecycleState: res.LifecycleState,
		HealthState:    res.HealthState,
		ManagedState:   res.ManagedState,
		Labels:         res.LabelsJSON,
		FirstSeenAt:    res.FirstSeenAt,
		LastSeenAt:     res.LastSeenAt,
	}
}

// liveResources scopes the resource query to the §5.4c read side: rows whose
// context belongs to a stale_source connection are absent, as are
// tombstoned (soft-deleted) rows. This is the local stand-in for the PR 22
// projection (§3.5) noted at the package comment.
func liveResources(db *gorm.DB) *gorm.DB {
	return db.Model(&model.InfraResource{}).
		Joins("JOIN provider_context pc ON pc.id = infra_resource.context_id").
		Joins("JOIN provider_connection pconn ON pconn.id = pc.connection_id AND pconn.stale_source = ?", false).
		Where("infra_resource.deleted_at IS NULL")
}

// ListResources lists resources with an optional kind filter and paging
// (§3.8 — stale_source 제외·kind 필터·페이징).
func (a *InfraAPI) ListResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := liveResources(a.db)
	if kind := c.Query("kind"); kind != "" {
		query = query.Where("infra_resource.kind = ?", kind)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	var rows []model.InfraResource
	if err := query.Order("infra_resource.id").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]resourceView, 0, len(rows))
	for _, row := range rows {
		items = append(items, newResourceView(row))
	}
	httpx.Success(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

// observationView is the latest-observation attachment of the resource
// detail (§3.8). Raw payloads are already size-capped (64KiB) and secret
// observations are metadata-only upstream — data values never reach this
// route (T52 redaction).
type observationView struct {
	GenerationUID     string           `json:"generationUid"`
	ObservationHash   string           `json:"observationHash"`
	NormalizerVersion string           `json:"normalizerVersion"`
	Normalized        contract.JSONMap `json:"normalized"`
	Raw               contract.JSONMap `json:"raw"`
	ObservedAt        time.Time        `json:"observedAt"`
}

// GetResource returns one resource by UID plus its latest observation.
func (a *InfraAPI) GetResource(c *gin.Context) {
	var res model.InfraResource
	err := liveResources(a.db).Where("infra_resource.uid = ?", c.Param("uid")).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Failed(c, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	response := gin.H{"resource": newResourceView(res)}
	var obs model.ResourceObservation
	if err := a.db.Where("resource_id = ?", res.ID).Order("observed_at DESC, id DESC").First(&obs).Error; err == nil {
		response["observation"] = observationView{
			GenerationUID:     obs.GenerationUID,
			ObservationHash:   obs.ObservationHash,
			NormalizerVersion: obs.NormalizerVersion,
			Normalized:        obs.NormalizedJSON,
			Raw:               obs.RawJSON,
			ObservedAt:        obs.ObservedAt,
		}
	}
	httpx.Success(c, response)
}
