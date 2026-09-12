import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { analyzeDBMSSQL, executeDBMSSQL } from '../api/dbms'
import { at } from '../utils/asset-i18n'
import { uiT } from '../utils/english-hardcoding-i18n'

// I5-H — extracted from views/assets/DatabaseWorkbench.vue (behavior
// preservation + rewiring contract, i5-plan §3.8). Original coordinates per
// block: 39-52·56-57·73-77·99-103 result/exec state · 125-128 resultFilter ·
// 256-264 filteredResultRows · 266-270 resetExecMeta · 363-374 exportResultCSV
// · 753-760 production phrase · 762-825 analyze/execute/confirm · 1022-1026
// riskTagType. Free variables injected: databaseId·connection·selection refs·
// sqlText·selectedSQLText (useDbmsSqlEditor)·load fns (view/transfer).
// 원본 DatabaseWorkbench.vue:753-756 — 프로덕션 판정 순수 함수. 실행·Redis·
// 행 편집 도메인이 공유하므로 모듈 export로 단일 원천화했다.
export function isProductionEnvironment(value) {
  const environment = String(value || '').toLowerCase()
  return environment.includes('prod') || environment.includes('운영')
}

export function useDbmsSqlExecution({
  databaseId,
  connection,
  selectedSchema,
  selectedTable,
  sqlText,
  selectedSQLText,
  supportsSQL,
  activeTab,
  loadHistory,
  loadTasks,
  loadTableData
}) {
  const sqlRunning = ref(false)
  const sqlConfirmVisible = ref(false)
  const sqlAcknowledgement = ref('')
  const resultColumns = ref([])
  const resultRows = ref([])
  const pendingSQL = ref('')
  const sqlAnalysis = ref(null)

  const execMeta = reactive({
    sqlType: '',
    rowsAffected: 0,
    durationMs: 0
  })

  const resultFilter = reactive({
    key: '',
    text: ''
  })

  const filteredResultRows = computed(() => {
    const key = resultFilter.key.trim()
    const text = resultFilter.text.trim().toLowerCase()
    if (!text) return resultRows.value
    return resultRows.value.filter((row) => {
      if (key) return String(row[key] ?? '').toLowerCase().includes(text)
      return Object.values(row).some((value) => String(value ?? '').toLowerCase().includes(text))
    })
  })

  function resetExecMeta() {
    execMeta.sqlType = ''
    execMeta.rowsAffected = 0
    execMeta.durationMs = 0
  }

  function exportResultCSV() {
    if (!resultColumns.value.length || !filteredResultRows.value.length) return ElMessage.warning(at('noResultToExport'))
    const escapeCSV = (value) => `"${String(value ?? '').replaceAll('"', '""')}"`
    const lines = [resultColumns.value, ...filteredResultRows.value.map((row) => resultColumns.value.map((column) => row[column]))]
      .map((row) => row.map(escapeCSV).join(','))
    const blob = new Blob(['\uFEFF', lines.join('\r\n')], { type: 'text/csv;charset=utf-8' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `sql-result-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`
    link.click()
    URL.revokeObjectURL(link.href)
  }

  function sqlConfirmationText() {
    return isProductionEnvironment(sqlAnalysis.value?.environment) ? at('prodConfirmPhrase') : at('execConfirmPhrase')
  }

  async function executeAnalyzedSQL(statement, confirmed = false) {
    const data = await executeDBMSSQL({
      databaseId: databaseId.value,
      schema: selectedSchema.value || connection.value?.dbName || '',
      sqlText: statement,
      confirmed
    })
    resultColumns.value = data.columns || []
    resultRows.value = data.rows || []
    execMeta.sqlType = data.sqlType || ''
    execMeta.rowsAffected = Number(data.rowsAffected || 0)
    execMeta.durationMs = Number(data.durationMs || 0)
    activeTab.value = 'result'
    ElMessage.success(at('sqlExecutionDone'))
    await Promise.all([loadHistory(), loadTasks()])
    if (selectedTable.value) {
      await loadTableData()
    }
  }

  async function runSQL() {
    if (!supportsSQL.value) {
      ElMessage.warning(at('notSQLWorkbenchWarning'))
      return
    }
    const statement = selectedSQLText()
    if (!statement) {
      ElMessage.warning(uiT('sqlRequired'))
      return
    }
    sqlRunning.value = true
    resetExecMeta()
    try {
      const analysis = await analyzeDBMSSQL({
        databaseId: databaseId.value,
        schema: selectedSchema.value || connection.value?.dbName || '',
        sqlText: statement
      })
      if (analysis.writeOperation) {
        pendingSQL.value = statement
        sqlAnalysis.value = analysis
        sqlAcknowledgement.value = ''
        sqlConfirmVisible.value = true
        return
      }
      await executeAnalyzedSQL(statement)
    } finally {
      sqlRunning.value = false
    }
  }

  async function confirmSQLExecution() {
    if (sqlAcknowledgement.value !== sqlConfirmationText()) {
      ElMessage.warning(at('enterConfirmationPhrase', { phrase: sqlConfirmationText() }))
      return
    }
    sqlRunning.value = true
    try {
      await executeAnalyzedSQL(pendingSQL.value, true)
      sqlConfirmVisible.value = false
    } finally {
      sqlRunning.value = false
    }
  }

  function riskTagType(level) {
    if (level === 'high') return 'danger'
    if (level === 'medium') return 'warning'
    return 'success'
  }

  return {
    sqlRunning,
    sqlConfirmVisible,
    sqlAcknowledgement,
    resultColumns,
    resultRows,
    pendingSQL,
    sqlAnalysis,
    execMeta,
    resultFilter,
    filteredResultRows,
    resetExecMeta,
    exportResultCSV,
    isProductionEnvironment,
    sqlConfirmationText,
    executeAnalyzedSQL,
    runSQL,
    confirmSQLExecution,
    riskTagType
  }
}
