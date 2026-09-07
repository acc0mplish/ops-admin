import http from './http'

// V2 infra read API (plan PR 23 — /api/v2/infra GET subset)
export const listInfraProviderTypes = () => http.get('/api/v2/infra/provider-types')
export const listInfraProviderConnections = () => http.get('/api/v2/infra/provider-connections')
export const listInfraResources = (params) => http.get('/api/v2/infra/resources', { params })
// Compute family read (plan J9/M12 — PR 30): the backend kindPrefix filter
// narrows to every kind under the family; the view pins "compute.".
export const listInfraComputeResources = (params) => http.get('/api/v2/infra/resources', { params: { kindPrefix: 'compute.', ...params } })
// UIDs ride the path, so they must be percent-encoded — the backend router
// matches the escaped path (router.go UseRawPath), and a raw separator inside
// a uid would both split the route and 404.
export const getInfraResource = (uid) => http.get(`/api/v2/infra/resources/${encodeURIComponent(uid)}`)

// V2 Phase 3 mutation API (plan PR 25 — §16.2 operations + task approval chain).
// The operations list is the registry opdef × resource kind intersection (F-9);
// plan is stateless; execute requires an Idempotency-Key header (§13.4) and
// answers 201 on first submit, 200 on key replay. Task verbs are key-free by
// design (r2 L1 — they converge on the same terminal state on reissue).
export const listInfraResourceOperations = (uid) => http.get(`/api/v2/infra/resources/${encodeURIComponent(uid)}/operations`)
export const planInfraOperation = (uid, name) => http.post(
  `/api/v2/infra/resources/${encodeURIComponent(uid)}/operations/${encodeURIComponent(name)}/plan`,
  {}
)
export const executeInfraOperation = (uid, name, payload, idempotencyKey) => http.post(
  `/api/v2/infra/resources/${encodeURIComponent(uid)}/operations/${encodeURIComponent(name)}/execute`,
  payload,
  { headers: { 'Idempotency-Key': idempotencyKey } }
)

export const listInfraTasks = (params) => http.get('/api/v2/infra/tasks', { params })
export const getInfraTask = (uid) => http.get(`/api/v2/infra/tasks/${encodeURIComponent(uid)}`)
export const listInfraTaskEvents = (uid) => http.get(`/api/v2/infra/tasks/${encodeURIComponent(uid)}/events`)
export const approveInfraTask = (uid) => http.post(`/api/v2/infra/tasks/${encodeURIComponent(uid)}/approve`)
export const rejectInfraTask = (uid) => http.post(`/api/v2/infra/tasks/${encodeURIComponent(uid)}/reject`)
export const cancelInfraTask = (uid) => http.post(`/api/v2/infra/tasks/${encodeURIComponent(uid)}/cancel`)
