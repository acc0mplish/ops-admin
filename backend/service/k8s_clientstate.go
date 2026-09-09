// k8s_clientstate.go — Service k8s client-state capsule (Phase D2, E5 seam #13, plan §J2).
package service

import (
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/singleflight"

	"ops-admin/backend/model"
)

// k8sClientState groups the five Kubernetes client-state fields absorbed from
// Service in Phase D2 (step 2a). Service MUST hold it by pointer only:
// a value copy would duplicate the sync.Mutex and singleflight.Group and
// break mutual exclusion (plan §12 #12; go vet copylocks, claim C13).
// The capsule is the gate — all access stays within package service and the
// Lock/Unlock statements at call sites keep their exact pre-D2 order, so the
// lock scopes are preserved byte-for-byte.
//
// Construction contract: k8sState is non-nil ONLY when Service is built via
// New(). A zero-value Service literal (&Service{db: db}, as used in tests
// that never touch k8s paths) leaves k8sState nil — reading it panics, unlike
// the pre-D2 zero-value maps which were safe to read. Any future test that
// exercises overview-cache or gateway-SSH paths MUST construct the Service
// with New() (D2 review MEDIUM #1, 2026-09-10).
type k8sClientState struct {
	// Gateway SSH connections are multiplexed by ssh.Client. Keeping one client
	// per gateway avoids repeating the public-network SSH handshake on every
	// Kubernetes API request.
	gatewaySSHMu      sync.Mutex
	gatewaySSHClients map[uint]*ssh.Client
	// Cluster overview is relatively expensive for gateway clusters. A brief
	// cache avoids duplicate page-load requests while singleflight coalesces
	// concurrent refreshes for the same cluster.
	k8sOverviewMu    sync.Mutex
	k8sOverviewCache map[uint]k8sOverviewCacheEntry
	k8sOverviewGroup singleflight.Group
}

const k8sOverviewCacheTTL = 15 * time.Second

type k8sOverviewCacheEntry struct {
	detail    model.K8sClusterDetail
	expiresAt time.Time
}
