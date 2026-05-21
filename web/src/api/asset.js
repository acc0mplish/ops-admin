import http from './http'

export const queryAssetHostList = (params) => http.get('/api/v1/asset/host/list', { params })
export const assetHostInfo = (id) => http.get('/api/v1/asset/host/info', { params: { id } })
export const downloadAssetHostTemplate = () => http.get('/api/v1/asset/host/template', { responseType: 'blob' })
export const addAssetHost = (data) => http.post('/api/v1/asset/host/add', data)
export const importAssetHosts = (data) =>
  http.post('/api/v1/asset/host/import', data, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
export const updateAssetHost = (data) => http.put('/api/v1/asset/host/update', data)
export const syncAssetHostsFromCloud = (data) => http.post('/api/v1/asset/host/cloudSync', data)
export const syncAssetHost = (id) => http.post('/api/v1/asset/host/sync', { id })
export const deleteAssetHost = (id) => http.delete('/api/v1/asset/host/delete', { data: { id } })
export const batchSyncAssetHosts = (ids) => http.post('/api/v1/asset/host/batch/sync', { ids })
export const batchDeleteAssetHosts = (ids) => http.delete('/api/v1/asset/host/batch/delete', { data: { ids } })
export const batchReplaceAssetHostCredential = (data) => http.put('/api/v1/asset/host/batch/credential', data)

export const queryAssetHostGroupList = (params) => http.get('/api/v1/asset/hostGroup/list', { params })
export const assetHostGroupInfo = (id) => http.get('/api/v1/asset/hostGroup/info', { params: { id } })
export const addAssetHostGroup = (data) => http.post('/api/v1/asset/hostGroup/add', data)
export const updateAssetHostGroup = (data) => http.put('/api/v1/asset/hostGroup/update', data)
export const deleteAssetHostGroup = (id) => http.delete('/api/v1/asset/hostGroup/delete', { data: { id } })

export const queryAssetCredentialList = (params) => http.get('/api/v1/asset/credential/list', { params })
export const queryAssetCredentialOptions = () => http.get('/api/v1/asset/credential/options')
export const assetCredentialInfo = (id) => http.get('/api/v1/asset/credential/info', { params: { id } })
export const addAssetCredential = (data) => http.post('/api/v1/asset/credential/add', data)
export const updateAssetCredential = (data) => http.put('/api/v1/asset/credential/update', data)
export const deleteAssetCredential = (id) => http.delete('/api/v1/asset/credential/delete', { data: { id } })

export const queryAssetCloudAccountList = (params) => http.get('/api/v1/asset/cloudAccount/list', { params })
export const queryAssetCloudAccountOptions = () => http.get('/api/v1/asset/cloudAccount/options')
export const assetCloudAccountInfo = (id) => http.get('/api/v1/asset/cloudAccount/info', { params: { id } })
export const addAssetCloudAccount = (data) => http.post('/api/v1/asset/cloudAccount/add', data)
export const updateAssetCloudAccount = (data) => http.put('/api/v1/asset/cloudAccount/update', data)
export const deleteAssetCloudAccount = (id) => http.delete('/api/v1/asset/cloudAccount/delete', { data: { id } })
