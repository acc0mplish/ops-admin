import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  createDBMSExportTask,
  createDBMSImportTask,
  downloadDBMSTaskFile,
  precheckDBMSImportTask,
  queryDBMSSchemaTree,
  queryDBMSSQLHistory,
  queryDBMSTaskList
} from '../api/dbms'
import { at } from '../utils/asset-i18n'
import { queryAssetDatabaseList } from '../api/asset'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 42-43·65-70·77-78 history/task/import state · 110-118 queries ·
// 193·223-226 source schemas/tables · 568-597 loadHistory·loadTasks ·
// 701-711 setupTaskPolling · 96 taskTimer · 1092-1101 rollback open·copy ·
// 1169-1252 export/import/download · 1254-1266 status helpers · 1272-1286
// import watches. onBeforeUnmount polling cleanup moves with the timer.
export function useDbmsTransferTasks({ databaseId, selectedSchema, selectedTable, activeTab }) {
  const historyLoading = ref(false)
  const taskLoading = ref(false)
  const historyList = ref([])
  const historyTotal = ref(0)
  const taskList = ref([])
  const taskTotal = ref(0)
  const taskTimer = ref(null)
  const rollbackDialogVisible = ref(false)
  const rollbackSQL = ref('')
  const rollbackConfidence = ref('')

  const historyQuery = reactive({
    pageNum: 1,
    pageSize: 20
  })

  const taskQuery = reactive({
    pageNum: 1,
    pageSize: 20
  })

  const importDialogVisible = ref(false)
  const importDatabaseOptions = ref([])
  const importSchemaTree = ref([])
  const importPrecheck = ref(null)
  const importPrechecking = ref(false)

  const importForm = reactive({
    sourceDatabaseId: undefined,
    sourceSchema: '',
    sourceTable: '',
    createIfMissing: true,
    truncateTarget: false
  })

  const sourceSchemas = computed(() => importSchemaTree.value || [])

  const sourceTables = computed(() => {
    const schema = sourceSchemas.value.find((item) => item.name === importForm.sourceSchema)
    return schema?.tables || []
  })

  async function loadHistory() {
    historyLoading.value = true
    try {
      const data = await queryDBMSSQLHistory({
        databaseId: databaseId.value,
        pageNum: historyQuery.pageNum,
        pageSize: historyQuery.pageSize
      })
      historyList.value = data.list || []
      historyTotal.value = data.total || 0
    } finally {
      historyLoading.value = false
    }
  }

  async function loadTasks() {
    taskLoading.value = true
    try {
      const data = await queryDBMSTaskList({
        databaseId: databaseId.value,
        pageNum: taskQuery.pageNum,
        pageSize: taskQuery.pageSize
      })
      taskList.value = data.list || []
      taskTotal.value = data.total || 0
      setupTaskPolling()
    } finally {
      taskLoading.value = false
    }
  }

  function setupTaskPolling() {
    const hasRunningTask = taskList.value.some((item) => ['pending', 'running'].includes(item.status))
    if (hasRunningTask && !taskTimer.value) {
      taskTimer.value = window.setInterval(loadTasks, 3000)
      return
    }
    if (!hasRunningTask && taskTimer.value) {
      window.clearInterval(taskTimer.value)
      taskTimer.value = null
    }
  }

  function openRollback(row) {
    rollbackSQL.value = row.rollbackSql || ''
    rollbackConfidence.value = row.rollbackConfidence || ''
    rollbackDialogVisible.value = true
  }

  async function copyRollback(row) {
    await navigator.clipboard.writeText(row.rollbackSql || '')
    ElMessage.success(at('rollbackSqlCopied'))
  }

  async function createExportTask() {
    if (!selectedSchema.value || !selectedTable.value) {
      ElMessage.warning(at('selectTableToExport'))
      return
    }
    await createDBMSExportTask({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      includeData: true
    })
    ElMessage.success(at('exportTaskCreated'))
    activeTab.value = 'tasks'
    await loadTasks()
  }

  async function openImportDialog() {
    const list = await queryAssetDatabaseList({ pageNum: 1, pageSize: 200, keyword: '', status: '1' })
    importDatabaseOptions.value = (list.list || []).filter((item) => ['mysql', 'postgresql'].includes(String(item.dbType || '').toLowerCase()))
    importPrecheck.value = null
    importDialogVisible.value = true
  }

  async function loadImportSchemas() {
    importForm.sourceSchema = ''
    importForm.sourceTable = ''
    importSchemaTree.value = []
    if (!importForm.sourceDatabaseId) return
    const data = await queryDBMSSchemaTree(importForm.sourceDatabaseId)
    importSchemaTree.value = data.schemas || []
    importForm.sourceSchema = data.defaultSchema || importSchemaTree.value[0]?.name || ''
  }

  async function submitImportTask() {
    if (!selectedSchema.value || !selectedTable.value) {
      ElMessage.warning(at('selectTargetTableFirst'))
      return
    }
    if (!importPrecheck.value?.ready) {
      ElMessage.warning(at('runImportPrecheckFirst'))
      return
    }
    await createDBMSImportTask(importPayload())
    importDialogVisible.value = false
    ElMessage.success(at('importTaskCreated'))
    activeTab.value = 'tasks'
    await loadTasks()
  }

  function importPayload() {
    return {
      sourceDatabaseId: importForm.sourceDatabaseId,
      sourceSchema: importForm.sourceSchema,
      sourceTable: importForm.sourceTable,
      targetDatabaseId: databaseId.value,
      targetSchema: selectedSchema.value,
      targetTable: selectedTable.value,
      createIfMissing: importForm.createIfMissing,
      truncateTarget: importForm.truncateTarget
    }
  }

  async function runImportPrecheck() {
    if (!importForm.sourceDatabaseId || !importForm.sourceSchema || !importForm.sourceTable || !selectedSchema.value || !selectedTable.value) {
      ElMessage.warning(at('selectSourceAndTarget'))
      return
    }
    importPrechecking.value = true
    try {
      importPrecheck.value = await precheckDBMSImportTask(importPayload())
    } finally {
      importPrechecking.value = false
    }
  }

  async function downloadTask(task) {
    const response = await downloadDBMSTaskFile({ id: task.id })
    const blob = new Blob([response.data], { type: 'application/sql' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = task.fileName || `dbms-task-${task.id}.sql`
    link.click()
    URL.revokeObjectURL(link.href)
  }

  function taskStatusType(status) {
    if (status === 'success') return 'success'
    if (status === 'failed') return 'danger'
    if (status === 'running') return 'warning'
    return 'info'
  }

  function taskStatusText(status) {
    if (status === 'success') return at('statusSuccess')
    if (status === 'failed') return at('statusFailed')
    if (status === 'running') return at('statusRunning')
    return at('statusPending')
  }

  watch(() => importForm.sourceDatabaseId, loadImportSchemas)
  watch(
    () => [
      importForm.sourceDatabaseId,
      importForm.sourceSchema,
      importForm.sourceTable,
      importForm.createIfMissing,
      importForm.truncateTarget,
      selectedSchema.value,
      selectedTable.value
    ],
    () => {
      importPrecheck.value = null
    }
  )

  onBeforeUnmount(() => {
    if (taskTimer.value) {
      window.clearInterval(taskTimer.value)
    }
  })

  return {
    historyLoading,
    historyList,
    historyTotal,
    historyQuery,
    taskLoading,
    taskList,
    taskTotal,
    taskQuery,
    rollbackDialogVisible,
    rollbackSQL,
    rollbackConfidence,
    importDialogVisible,
    importDatabaseOptions,
    importSchemaTree,
    importForm,
    importPrecheck,
    importPrechecking,
    sourceSchemas,
    sourceTables,
    loadHistory,
    loadTasks,
    openRollback,
    copyRollback,
    createExportTask,
    openImportDialog,
    loadImportSchemas,
    submitImportTask,
    importPayload,
    runImportPrecheck,
    downloadTask,
    taskStatusType,
    taskStatusText
  }
}
