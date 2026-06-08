import http from './http'

export const queryDBMSWorkbench = (databaseId) => http.get('/api/v1/dbms/workbench', { params: { databaseId } })
export const queryDBMSSchemaTree = (databaseId) => http.get('/api/v1/dbms/schema/tree', { params: { databaseId } })
export const queryDBMSTableData = (params) => http.get('/api/v1/dbms/table/data', { params })
export const executeDBMSSQL = (data) => http.post('/api/v1/dbms/sql/execute', data)
export const queryDBMSSQLHistory = (params) => http.get('/api/v1/dbms/sql/history', { params })
export const insertDBMSTableRow = (data) => http.post('/api/v1/dbms/table/row/insert', data)
export const updateDBMSTableRow = (data) => http.put('/api/v1/dbms/table/row/update', data)
export const deleteDBMSTableRow = (data) => http.delete('/api/v1/dbms/table/row/delete', { data })
export const createDBMSExportTask = (data) => http.post('/api/v1/dbms/task/export', data)
export const createDBMSImportTask = (data) => http.post('/api/v1/dbms/task/import', data)
export const queryDBMSTaskList = (params) => http.get('/api/v1/dbms/task/list', { params })
export const downloadDBMSTaskFile = (params) => http.get('/api/v1/dbms/task/download', { params, responseType: 'blob' })
