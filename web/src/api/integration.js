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

export const queryAIKnowledgeDocuments = (params = {}) => http.get('/api/v1/integration/ai/knowledge-base/list', { params })
export const saveAIKnowledgeDocument = (data) => http.post('/api/v1/integration/ai/knowledge-base/save', data)
export const uploadAIKnowledgeDocument = (data) => http.post('/api/v1/integration/ai/knowledge-base/upload', data)
export const deleteAIKnowledgeDocument = (id) => http.delete('/api/v1/integration/ai/knowledge-base/delete', { params: { id } })

export const queryAITools = () => http.get('/api/v1/integration/ai/tool/list')
export const updateAITool = (data) => http.put('/api/v1/integration/ai/tool/update', data)
export const executeAITool = (data) => http.post('/api/v1/integration/ai/tool/execute', data)
export const confirmAIAction = (id) => http.post('/api/v1/integration/ai/action/confirm', { id })
export const rejectAIAction = (id) => http.post('/api/v1/integration/ai/action/reject', { id })

export const queryFinOpsAccounts = (params = {}) => http.get('/api/v1/integration/finops/account/list', { params })
export const saveFinOpsAccount = (data) => http.post('/api/v1/integration/finops/account/save', data)
// The FinOps account delete handler reads its identifier from the query string.
export const deleteFinOpsAccount = (id) => http.delete('/api/v1/integration/finops/account/delete', { params: { id } })
export const testFinOpsAccount = (data) => http.post('/api/v1/integration/finops/account/test', data)
export const queryFinOpsDashboard = (params = {}) => http.get('/api/v1/integration/finops/dashboard', { params })
export const queryFinOpsBreakdown = (params = {}) => http.get('/api/v1/integration/finops/breakdown', { params })
export const queryFinOpsLatestBreakdownMonth = (params = {}) => http.get('/api/v1/integration/finops/breakdown/latest-month', { params })
export const queryFinOpsResources = (params = {}) => http.get('/api/v1/integration/finops/resource/list', { params })
export const queryFinOpsRecommendations = (params = {}) => http.get('/api/v1/integration/finops/recommendation/list', { params })
// AI recommendation generation waits for the configured model response.  It must
// not inherit the 15-second timeout used by ordinary list/query requests.
// A JSON-repair retry may follow the initial model request.
export const generateFinOpsRecommendations = (data = {}) =>
  http.post('/api/v1/integration/finops/recommendation/generate', data, { timeout: 150000 })
export const updateFinOpsRecommendation = (data) => http.put('/api/v1/integration/finops/recommendation/status', data)
export const deleteFinOpsRecommendation = (id) => http.delete('/api/v1/integration/finops/recommendation/delete', { params: { id } })
export const queryFinOpsSyncLogs = (params = {}) => http.get('/api/v1/integration/finops/sync/logs', { params })
export const triggerFinOpsSync = (payload) => http.post('/api/v1/integration/finops/sync/trigger', typeof payload === 'object' ? payload : { accountId: payload })
export const importFinOpsCosts = (data) => http.post('/api/v1/integration/finops/cost/import', data)
