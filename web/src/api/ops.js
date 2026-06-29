import http from './http'

export const queryOpsScriptList = (params) => http.get('/api/v1/ops/script/list', { params })
export const queryOpsScriptOptions = () => http.get('/api/v1/ops/script/options')
export const opsScriptInfo = (id) => http.get('/api/v1/ops/script/info', { params: { id } })
export const addOpsScript = (data) => http.post('/api/v1/ops/script/add', data)
export const updateOpsScript = (data) => http.put('/api/v1/ops/script/update', data)
export const updateOpsScriptStatus = (data) => http.put('/api/v1/ops/script/status', data)
export const deleteOpsScript = (id) => http.delete('/api/v1/ops/script/delete', { data: { id } })

export const executeOpsCommand = (data) => http.post('/api/v1/ops/exec/command', data)
export const executeOpsScript = (data) => http.post('/api/v1/ops/exec/script', data)
export const executeOpsFileDispatch = (data) =>
  http.post('/api/v1/ops/exec/file-dispatch', data, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 600000
  })

export const queryOpsExecHistory = (params) => http.get('/api/v1/ops/exec/history', { params })
export const queryOpsExecHistoryDetail = (id) => http.get('/api/v1/ops/exec/history/detail', { params: { id } })

export const queryOpsScheduleTaskList = (params) => http.get('/api/v1/ops/schedule/task/list', { params })
export const opsScheduleTaskInfo = (id) => http.get('/api/v1/ops/schedule/task/info', { params: { id } })
export const addOpsScheduleTask = (data) => http.post('/api/v1/ops/schedule/task/add', data)
export const updateOpsScheduleTask = (data) => http.put('/api/v1/ops/schedule/task/update', data)
export const updateOpsScheduleTaskStatus = (data) => http.put('/api/v1/ops/schedule/task/status', data)
export const runOpsScheduleTask = (id) => http.post('/api/v1/ops/schedule/task/run', { id })
export const deleteOpsScheduleTask = (id) => http.delete('/api/v1/ops/schedule/task/delete', { data: { id } })
export const batchDeleteOpsScheduleTask = (ids) => http.delete('/api/v1/ops/schedule/task/batch/delete', { data: { ids } })

export const queryOpsScheduleLogList = (params) => http.get('/api/v1/ops/schedule/log/list', { params })
export const opsScheduleLogInfo = (id) => http.get('/api/v1/ops/schedule/log/info', { params: { id } })

export const queryOpsScheduleTemplateList = (params) => http.get('/api/v1/ops/schedule/template/list', { params })
export const opsScheduleTemplateInfo = (id) => http.get('/api/v1/ops/schedule/template/info', { params: { id } })
export const addOpsScheduleTemplate = (data) => http.post('/api/v1/ops/schedule/template/add', data)
export const updateOpsScheduleTemplate = (data) => http.put('/api/v1/ops/schedule/template/update', data)
export const deleteOpsScheduleTemplate = (id) => http.delete('/api/v1/ops/schedule/template/delete', { data: { id } })

export const queryOpsJobList = (params) => http.get('/api/v1/ops/job/list', { params })
export const opsJobInfo = (id) => http.get('/api/v1/ops/job/info', { params: { id } })
export const addOpsJob = (data) => http.post('/api/v1/ops/job/add', data)
export const updateOpsJob = (data) => http.put('/api/v1/ops/job/update', data)
export const runOpsJob = (id) => http.post('/api/v1/ops/job/run', { id })
export const deleteOpsJob = (id) => http.delete('/api/v1/ops/job/delete', { data: { id } })

export const queryOpsJobHistoryList = (params) => http.get('/api/v1/ops/job/history/list', { params })
export const opsJobHistoryDetail = (id) => http.get('/api/v1/ops/job/history/detail', { params: { id } })
export const approveOpsJobHistory = (data) => http.post('/api/v1/ops/job/history/approve', data)
export const rejectOpsJobHistory = (data) => http.post('/api/v1/ops/job/history/reject', data)

export const queryOpsJobTemplateList = (params) => http.get('/api/v1/ops/job/template/list', { params })
export const queryOpsJobTemplateOptions = () => http.get('/api/v1/ops/job/template/options')
export const opsJobTemplateInfo = (id) => http.get('/api/v1/ops/job/template/info', { params: { id } })
export const addOpsJobTemplate = (data) => http.post('/api/v1/ops/job/template/add', data)
export const updateOpsJobTemplate = (data) => http.put('/api/v1/ops/job/template/update', data)
export const deleteOpsJobTemplate = (id) => http.delete('/api/v1/ops/job/template/delete', { data: { id } })
