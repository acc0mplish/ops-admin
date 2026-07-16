import http from './http'

export const queryNavigationGroups = (params = {}) => http.get('/api/v1/integration/navigation/group/list', { params })
export const saveNavigationGroup = (data) => http.post('/api/v1/integration/navigation/group/save', data)
export const deleteNavigationGroup = (id) => http.delete('/api/v1/integration/navigation/group/delete', { data: { id } })
export const regenerateNavigationGroupToken = (id) => http.post('/api/v1/integration/navigation/group/token', { id })

export const queryNavigations = (params = {}) => http.get('/api/v1/integration/navigation/list', { params })
export const saveNavigation = (data) => http.post('/api/v1/integration/navigation/save', data)
export const deleteNavigation = (id) => http.delete('/api/v1/integration/navigation/delete', { data: { id } })

export const queryPublicNavigation = (token) => http.get(`/api/v1/integration/public/${encodeURIComponent(token)}`)

export const queryAIModels = () => http.get('/api/v1/integration/ai/model/list')
export const saveAIModel = (data) => http.post('/api/v1/integration/ai/model/save', data)
export const deleteAIModel = (id) => http.delete('/api/v1/integration/ai/model/delete', { data: { id } })
export const testAIModel = (data) => http.post('/api/v1/integration/ai/model/test', data)

export const queryAIConversations = (params = {}) => http.get('/api/v1/integration/ai/conversation/list', { params })
export const queryAIConversationDetail = (id) => http.get('/api/v1/integration/ai/conversation/detail', { params: { id } })
export const saveAIConversation = (data) => http.post('/api/v1/integration/ai/conversation/save', data)
export const deleteAIConversation = (id) => http.delete('/api/v1/integration/ai/conversation/delete', { data: { id } })
export const sendAIChat = (data) => http.post('/api/v1/integration/ai/chat/send', data)

export const queryAITools = () => http.get('/api/v1/integration/ai/tool/list')
export const updateAITool = (data) => http.put('/api/v1/integration/ai/tool/update', data)
export const executeAITool = (data) => http.post('/api/v1/integration/ai/tool/execute', data)
export const confirmAIAction = (id) => http.post('/api/v1/integration/ai/action/confirm', { id })
export const rejectAIAction = (id) => http.post('/api/v1/integration/ai/action/reject', { id })
