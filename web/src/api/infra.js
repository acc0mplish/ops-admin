import http from './http'

// V2 infra read API (plan PR 23 — /api/v2/infra GET subset)
export const listInfraProviderTypes = () => http.get('/api/v2/infra/provider-types')
export const listInfraProviderConnections = () => http.get('/api/v2/infra/provider-connections')
export const listInfraResources = (params) => http.get('/api/v2/infra/resources', { params })
export const getInfraResource = (uid) => http.get(`/api/v2/infra/resources/${uid}`)
