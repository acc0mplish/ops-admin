import http from './http'

export const queryNavigationGroups = (params = {}) => http.get('/api/v1/integration/navigation/group/list', { params })
export const saveNavigationGroup = (data) => http.post('/api/v1/integration/navigation/group/save', data)
export const deleteNavigationGroup = (id) => http.delete('/api/v1/integration/navigation/group/delete', { data: { id } })
export const regenerateNavigationGroupToken = (id) => http.post('/api/v1/integration/navigation/group/token', { id })

export const queryNavigations = (params = {}) => http.get('/api/v1/integration/navigation/list', { params })
export const saveNavigation = (data) => http.post('/api/v1/integration/navigation/save', data)
export const deleteNavigation = (id) => http.delete('/api/v1/integration/navigation/delete', { data: { id } })

export const queryPublicNavigation = (token) => http.get(`/api/v1/integration/public/${encodeURIComponent(token)}`)
