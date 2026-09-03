<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { confirmRiskOperation } from '../../composables/useRiskConfirm'
import { queryAssetDatabaseList } from '../../api/asset'
import DatabaseConnectionTree from './database/DatabaseConnectionTree.vue'
import {
  analyzeDBMSSQL,
  analyzeRedisCommand,
  createDBMSSchema,
  createDBMSExportTask,
  createDBMSImportTask,
  deleteDBMSTableRow,
  downloadDBMSTaskFile,
  executeDBMSSQL,
  executeRedisCommand,
  insertDBMSTableRow,
  precheckDBMSImportTask,
  queryDBMSSchemaTree,
  queryDBMSCharsetOptions,
  queryDBMSSQLHistory,
  queryDBMSResourceData,
  queryDBMSTableData,
  queryDBMSTaskList,
  queryDBMSWorkbench,
  updateDBMSTableRow
} from '../../api/dbms'

const route = useRoute()
const router = useRouter()

const databaseId = computed(() => Number(route.params.id || 0))

const loading = ref(false)
const treeLoading = ref(false)
const sqlRunning = ref(false)
const redisRunning = ref(false)
const dataLoading = ref(false)
const historyLoading = ref(false)
const taskLoading = ref(false)
const rowDialogVisible = ref(false)
const redisKeyDialogVisible = ref(false)
const rollbackDialogVisible = ref(false)
const importDialogVisible = ref(false)
const createDatabaseVisible = ref(false)
const creatingDatabase = ref(false)
const charsetOptionsLoading = ref(false)
const sqlConfirmVisible = ref(false)
const sqlAcknowledgement = ref('')
const activeTab = ref('data')
const rowDialogMode = ref('insert')
const redisKeyDialogMode = ref('create')

const connection = ref(null)
const schemaTree = ref([])
const resultColumns = ref([])
const resultRows = ref([])
const selectedColumns = ref([])
const selectedRows = ref([])
const selectedTotal = ref(0)
const selectedPrimaryKeys = ref([])
const resourceIndexes = ref([])
const resourceType = ref('')
const historyList = ref([])
const historyTotal = ref(0)
const taskList = ref([])
const taskTotal = ref(0)
const importDatabaseOptions = ref([])
const importSchemaTree = ref([])
const rollbackSQL = ref('')
const rollbackConfidence = ref('')
const pendingSQL = ref('')
const sqlAnalysis = ref(null)
const importPrecheck = ref(null)
const importPrechecking = ref(false)

const selectedSchema = ref('')
const selectedTable = ref('')
const treeKeyword = ref('')
const schemaTablePages = reactive({})
const schemaTablePageSize = 20
const showSuggestions = ref(false)
const currentToken = ref('')
const activeSuggestionIndex = ref(0)
const currentLine = ref(1)
const sqlText = ref('SELECT *\nFROM your_table\nLIMIT 50;')
const redisCommandText = ref('GET key')
const sqlScrollTop = ref(0)
const sqlScrollLeft = ref(0)
const sqlEditorRef = ref(null)
const editorWrapRef = ref(null)
const treeRef = ref(null)
const taskTimer = ref(null)
const sqlFavorites = ref([])

const execMeta = reactive({
  sqlType: '',
  rowsAffected: 0,
  durationMs: 0
})

const tableQuery = reactive({
  pageNum: 1,
  pageSize: 25
})

const historyQuery = reactive({
  pageNum: 1,
  pageSize: 20
})

const taskQuery = reactive({
  pageNum: 1,
  pageSize: 20
})

const tableFilter = reactive({
  key: '',
  text: ''
})

const resultFilter = reactive({
  key: '',
  text: ''
})

const editingCell = reactive({
  rowKey: '',
  column: ''
})

const pendingCellValue = ref('')
const editingOriginalRow = ref(null)

const rowForm = reactive({})
const rowOriginal = ref({})
const redisKeyForm = reactive({
  key: '',
  type: 'string',
  value: '',
  ttl: undefined
})

const importForm = reactive({
  sourceDatabaseId: undefined,
  sourceSchema: '',
  sourceTable: '',
  createIfMissing: true,
  truncateTarget: false
})

const createDatabaseForm = reactive({
  name: '',
  charset: 'utf8mb4',
  collation: 'utf8mb4_0900_ai_ci'
})

const mysqlCharsetOptions = ref([])
const availableCollations = ref([])

const sqlKeywordPool = [
  'SELECT', 'FROM', 'WHERE', 'ORDER BY', 'GROUP BY', 'HAVING', 'LIMIT', 'OFFSET',
  'INSERT INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE FROM', 'CREATE TABLE', 'ALTER TABLE',
  'DROP TABLE', 'SHOW TABLES', 'SHOW DATABASES', 'DESCRIBE', 'EXPLAIN', 'LEFT JOIN',
  'RIGHT JOIN', 'INNER JOIN', 'UNION ALL', 'COUNT', 'SUM', 'MIN', 'MAX', 'AVG', 'NOW()'
]

const baseSqlSnippets = [
  { label: 'SELECT', text: 'SELECT *\nFROM table_name\nLIMIT 50;' },
  { label: 'UPDATE', text: "UPDATE table_name\nSET column_name = 'value'\nWHERE id = 1;" },
  { label: 'INSERT', text: "INSERT INTO table_name (column_1, column_2)\nVALUES ('value_1', 'value_2');" },
  { label: 'DELETE', text: 'DELETE FROM table_name\nWHERE id = 1;' }
]

const redisCommandSnippets = [
  { label: 'GET', text: 'GET key' },
  { label: 'MGET', text: 'MGET key1 key2' },
  { label: 'TTL', text: 'TTL key' },
  { label: 'TYPE', text: 'TYPE key' },
  { label: 'SCAN', text: 'SCAN 0 MATCH * COUNT 100' },
  { label: 'HGETALL', text: 'HGETALL hash_key' },
  { label: 'LRANGE', text: 'LRANGE list_key 0 -1' },
  { label: 'SMEMBERS', text: 'SMEMBERS set_key' },
  { label: 'ZRANGE', text: 'ZRANGE zset_key 0 -1 WITHSCORES' },
  { label: 'SET', text: 'SET key value' },
  { label: 'DEL', text: 'DEL key' },
  { label: 'EXPIRE', text: 'EXPIRE key 3600' }
]

const sourceSchemas = computed(() => importSchemaTree.value || [])
const isReadOnly = computed(() => connection.value?.accessMode === 'readonly')
const capabilities = computed(() => connection.value?.capabilities || {})
const supportsSQL = computed(() => capabilities.value.sql !== false)
const isRedis = computed(() => connection.value?.dbType === 'redis')
const isPostgres = computed(() => connection.value?.dbType === 'postgresql')
const supportsCreateDatabase = computed(() => !isReadOnly.value && ['mysql', 'postgresql'].includes(connection.value?.dbType))
const createDatabaseObjectLabel = computed(() => (isPostgres.value ? 'Schema' : 'Database'))
const sqlSnippets = computed(() => [
  ...baseSqlSnippets,
  isPostgres.value
    ? {
        label: 'Table 보기',
        text: "SELECT table_schema, table_name\nFROM information_schema.tables\nWHERE table_type = 'BASE TABLE'\n  AND table_schema NOT IN ('pg_catalog', 'information_schema')\n  AND table_schema NOT LIKE 'pg_%'\nORDER BY table_schema, table_name;"
      }
    : { label: 'SHOW TABLES', text: 'SHOW TABLES;' }
])
const supportsTableData = computed(() => capabilities.value.tableData !== false)
const supportsResourceData = computed(() => capabilities.value.resourceData === true)
const supportsExport = computed(() => capabilities.value.export === true || (capabilities.value.export === undefined && capabilities.value.transfer !== false))
const supportsImport = computed(() => capabilities.value.import === true || (capabilities.value.import === undefined && capabilities.value.transfer !== false))
const canEditRows = computed(() => supportsTableData.value && !isReadOnly.value && selectedPrimaryKeys.value.length > 0 && capabilities.value.rowEdit !== false)
const canManageRedisKeys = computed(() => isRedis.value && !isReadOnly.value && capabilities.value.keyEdit !== false)
const redisKeyTypes = ['string', 'hash', 'list', 'set', 'zset']
const redisKeyValuePlaceholder = computed(() => {
  if (redisKeyForm.type === 'hash') return '{"field":"value"}'
  if (redisKeyForm.type === 'zset') return '[{"member":"user:1","score":100}]'
  if (['list', 'set'].includes(redisKeyForm.type)) return '["value-1", "value-2"]'
  return 'String Value를 입력하십시오.'
})
const sourceTables = computed(() => {
  const schema = sourceSchemas.value.find((item) => item.name === importForm.sourceSchema)
  return schema?.tables || []
})

const filterableColumns = computed(() => selectedColumns.value.map((item) => item.name))

const allSuggestionItems = computed(() => {
  const pool = new Set(sqlKeywordPool)
  for (const schema of schemaTree.value) {
    pool.add(schema.name)
    for (const table of schema.tables || []) {
      pool.add(table.name)
      pool.add(`${schema.name}.${table.name}`)
    }
  }
  for (const col of selectedColumns.value) {
    pool.add(col.name)
  }
  return Array.from(pool)
})

const suggestions = computed(() => {
  const keyword = currentToken.value.trim().toLowerCase()
  if (!keyword) return []
  return allSuggestionItems.value
    .filter((item) => item.toLowerCase().includes(keyword))
    .slice(0, 14)
})

const highlightedSQL = computed(() => highlightSQL(sqlText.value))
const sqlLines = computed(() => Array.from({ length: Math.max(sqlText.value.split('\n').length, 1) }, (_, index) => index + 1))

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

function escapeHTML(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

function highlightSQL(source) {
  let html = escapeHTML(source)
  html = html.replace(/(--.*)$/gm, '<span class="token-comment">$1</span>')
  html = html.replace(/('(?:''|[^'])*')/g, '<span class="token-string">$1</span>')
  html = html.replace(/\b(\d+)\b/g, '<span class="token-number">$1</span>')
  html = html.replace(/\b(SELECT|FROM|WHERE|ORDER BY|GROUP BY|HAVING|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE FROM|CREATE TABLE|ALTER TABLE|DROP TABLE|SHOW|TABLES|DATABASES|DESCRIBE|EXPLAIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|UNION ALL|COUNT|SUM|AVG|MIN|MAX|NOW)\b/gi, '<span class="token-keyword">$1</span>')
  return html
}

function sqlFavoritesStorageKey() {
  return `ops-admin.dbms.sql-favorites.${databaseId.value}`
}

function loadSqlFavorites() {
  try {
    const saved = JSON.parse(localStorage.getItem(sqlFavoritesStorageKey()) || '[]')
    sqlFavorites.value = Array.isArray(saved) ? saved.filter((item) => item?.name && item?.sqlText).slice(0, 30) : []
  } catch {
    sqlFavorites.value = []
  }
}

function persistSqlFavorites() {
  localStorage.setItem(sqlFavoritesStorageKey(), JSON.stringify(sqlFavorites.value.slice(0, 30)))
}

function basicFormatSQL(source) {
  const parts = String(source || '').split(/('(?:''|[^'])*'|"(?:""|[^"])*")/g)
  return parts.map((part, index) => {
    if (index % 2) return part
    return part
      .replace(/\s+/g, ' ')
      .replace(/\b(SELECT|FROM|WHERE|GROUP BY|ORDER BY|HAVING|LIMIT|OFFSET|INSERT INTO|VALUES|UPDATE|SET|DELETE FROM|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|UNION ALL|CREATE TABLE|ALTER TABLE|DROP TABLE)\b/gi, (_, keyword) => `\n${keyword.toUpperCase()}`)
      .replace(/\s*,\s*/g, ', ')
  }).join('').replace(/^\s+|\s+$/g, '').replace(/;\s*(?=\S)/g, ';\n\n')
}

function formatSQL() {
  const editor = sqlEditorRef.value
  const source = selectedSQLText()
  if (!source) return ElMessage.warning('Format할 SQL을 입력하십시오.')
  const formatted = basicFormatSQL(source)
  if (editor && editor.selectionStart !== editor.selectionEnd) {
    const before = sqlText.value.slice(0, editor.selectionStart)
    const after = sqlText.value.slice(editor.selectionEnd)
    sqlText.value = before + formatted + after
    nextTick(() => editor.setSelectionRange(before.length, before.length + formatted.length))
  } else {
    sqlText.value = formatted
  }
  ElMessage.success('SQL Format을 완료했습니다.')
}

async function saveCurrentSQL() {
  const statement = selectedSQLText()
  if (!statement) return ElMessage.warning('Favorite에 저장할 SQL을 입력하십시오.')
  const { value } = await ElMessageBox.prompt('이 SQL의 이름을 지정하면 이후 Editor에 바로 불러올 수 있습니다.', 'SQL Favorite', {
    inputPlaceholder: '예: 최근 24시간 Order 조회',
    inputPattern: /\S+/,
    inputErrorMessage: 'Favorite 이름을 입력하십시오.',
    confirmButtonText: '저장',
    cancelButtonText: '취소'
  })
  sqlFavorites.value.unshift({ id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`, name: value.trim(), sqlText: statement, updatedAt: Date.now() })
  persistSqlFavorites()
  ElMessage.success('SQL을 Favorite에 저장했습니다.')
}

function applySqlFavorite(item) {
  sqlText.value = item.sqlText
  nextTick(() => sqlEditorRef.value?.focus())
}

function removeSqlFavorite(item) {
  sqlFavorites.value = sqlFavorites.value.filter((current) => current.id !== item.id)
  persistSqlFavorites()
}

function reuseHistorySQL(row) {
  sqlText.value = row.sqlText || ''
  activeTab.value = 'result'
  nextTick(() => sqlEditorRef.value?.focus())
}

function exportResultCSV() {
  if (!resultColumns.value.length || !filteredResultRows.value.length) return ElMessage.warning('Export할 SQL 결과가 없습니다.')
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

function normalizeTree(data) {
  return (data.schemas || []).map((schema) => ({
    id: `schema:${schema.name}`,
    label: schema.name,
    name: schema.name,
    isSchema: true,
    tableCount: Number(schema.tableCount || schema.tables?.length || 0),
    tables: schema.tables || [],
    children: (schema.tables || []).map((table) => ({
      id: `table:${schema.name}.${table.name}`,
      label: table.name,
      name: table.name,
      schema: schema.name,
      rows: table.rows,
      isTable: true
    }))
  }))
}

const pagedSchemaTree = computed(() => {
  const keyword = treeKeyword.value.trim().toLowerCase()
  return schemaTree.value
    .map((schema) => {
      const allChildren = schema.children || []
      const schemaMatched = schema.name.toLowerCase().includes(keyword)
      const children = keyword && !schemaMatched
        ? allChildren.filter((table) => {
          const fullName = `${schema.name}.${table.name}`.toLowerCase()
          return String(table.name || '').toLowerCase().includes(keyword) || fullName.includes(keyword)
        })
        : allChildren
      const totalPages = Math.max(1, Math.ceil(children.length / schemaTablePageSize))
      const currentPage = Math.min(Math.max(Number(schemaTablePages[schema.name] || 1), 1), totalPages)
      const start = (currentPage - 1) * schemaTablePageSize
      return {
        ...schema,
        children: children.slice(start, start + schemaTablePageSize),
        visibleTableCount: children.length,
        currentPage,
        totalPages
      }
    })
    .filter((schema) => !keyword || schema.children.length > 0 || schema.name.toLowerCase().includes(keyword))
})

function syncEditorMetrics() {
  const el = sqlEditorRef.value
  if (!el) return
  sqlScrollTop.value = el.scrollTop
  sqlScrollLeft.value = el.scrollLeft
  const before = sqlText.value.slice(0, el.selectionStart)
  currentLine.value = before.split('\n').length
}

function treeFilterMethod() {
  return true
}

function changeSchemaTablePage(schema, page) {
  const nextPage = Math.min(Math.max(page, 1), schema.totalPages || 1)
  schemaTablePages[schema.name] = nextPage
}

function rowKeyFor(row, index = 0) {
  const keys = selectedPrimaryKeys.value || []
  if (keys.length) {
    return keys.map((key) => `${key}:${row[key] ?? ''}`).join('|')
  }
  return `${index}:${JSON.stringify(row)}`
}

function isEditingCell(row, column, index) {
  return editingCell.rowKey === rowKeyFor(row, index) && editingCell.column === column
}

function startCellEdit(row, column, index) {
  if (!canEditRows.value) {
    ElMessage.warning(isReadOnly.value ? '현재 Database는 Read-only Mode입니다.' : '현재 Table에 Primary Key가 없어 직접 편집할 수 없습니다.')
    return
  }
  editingCell.rowKey = rowKeyFor(row, index)
  editingCell.column = column
  pendingCellValue.value = row[column] == null ? '' : String(row[column])
  editingOriginalRow.value = { ...row }
}

function cancelCellEdit() {
  editingCell.rowKey = ''
  editingCell.column = ''
  pendingCellValue.value = ''
  editingOriginalRow.value = null
}

async function commitCellEdit(row, column) {
  if (!editingOriginalRow.value) {
    cancelCellEdit()
    return
  }
  const current = { ...row, [column]: pendingCellValue.value }
  await updateDBMSTableRow({
    databaseId: databaseId.value,
    schema: selectedSchema.value,
    table: selectedTable.value,
    original: editingOriginalRow.value,
    current
  })
  ElMessage.success('Cell을 업데이트했습니다.')
  cancelCellEdit()
  await Promise.all([loadTableData(), loadHistory()])
}

async function loadConnection() {
  connection.value = await queryDBMSWorkbench(databaseId.value)
  loadSqlFavorites()
}

async function loadTree() {
  treeLoading.value = true
  try {
    const data = await queryDBMSSchemaTree(databaseId.value)
    schemaTree.value = normalizeTree(data)
    Object.keys(schemaTablePages).forEach((key) => delete schemaTablePages[key])
    if (!selectedSchema.value && data.defaultSchema) {
      selectedSchema.value = data.defaultSchema
    }
  } finally {
    treeLoading.value = false
  }
}

async function loadDatabaseCharsetOptions(charset = createDatabaseForm.charset) {
  charsetOptionsLoading.value = true
  try {
    const data = await queryDBMSCharsetOptions({ databaseId: databaseId.value, charset })
    mysqlCharsetOptions.value = (data.charsets || []).map((item) => ({ label: item.name, value: item.name, defaultCollation: item.defaultCollation }))
    availableCollations.value = (data.collations || []).map((item) => ({ label: item.name, value: item.name, isDefault: item.isDefault }))
    if (!mysqlCharsetOptions.value.some((item) => item.value === createDatabaseForm.charset)) {
      createDatabaseForm.charset = mysqlCharsetOptions.value.find((item) => item.value === 'utf8mb4')?.value || mysqlCharsetOptions.value[0]?.value || ''
      if (createDatabaseForm.charset && createDatabaseForm.charset !== charset) {
        await loadDatabaseCharsetOptions(createDatabaseForm.charset)
        return
      }
    }
    if (!availableCollations.value.some((item) => item.value === createDatabaseForm.collation)) {
      createDatabaseForm.collation = availableCollations.value.find((item) => item.isDefault)?.value || availableCollations.value[0]?.value || ''
    }
  } finally {
    charsetOptionsLoading.value = false
  }
}

async function openCreateDatabase() {
  createDatabaseForm.name = ''
  if (!isPostgres.value) {
    createDatabaseForm.charset = connection.value?.charset || 'utf8mb4'
    createDatabaseForm.collation = ''
    createDatabaseVisible.value = true
    await loadDatabaseCharsetOptions()
    return
  }
  createDatabaseVisible.value = true
}

async function onCreateDatabaseCharsetChange() {
  createDatabaseForm.collation = ''
  await loadDatabaseCharsetOptions(createDatabaseForm.charset)
}

async function submitCreateDatabase() {
  const name = createDatabaseForm.name.trim()
  if (!name) {
    ElMessage.warning(`${createDatabaseObjectLabel.value} 이름을 입력하십시오.`)
    return
  }
  creatingDatabase.value = true
  try {
    const result = await createDBMSSchema({
      databaseId: databaseId.value,
      name,
      charset: isPostgres.value ? '' : createDatabaseForm.charset,
      collation: isPostgres.value ? '' : createDatabaseForm.collation
    })
    selectedSchema.value = result.name || name
    selectedTable.value = ''
    await loadTree()
    createDatabaseVisible.value = false
    ElMessage.success(`${createDatabaseObjectLabel.value} ${name}을(를) 생성했습니다.`)
  } finally {
    creatingDatabase.value = false
  }
}

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

async function loadTableData() {
  if (!supportsTableData.value) return
  if (!selectedSchema.value || !selectedTable.value) return
  dataLoading.value = true
  try {
    const data = await queryDBMSTableData({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      pageNum: tableQuery.pageNum,
      pageSize: tableQuery.pageSize,
      filterKey: tableFilter.key,
      filterText: tableFilter.text
    })
    selectedColumns.value = data.columns || []
    selectedRows.value = data.rows || []
    selectedTotal.value = data.total || 0
    selectedPrimaryKeys.value = data.primaryKeys || []
  } finally {
    dataLoading.value = false
  }
}

async function loadResourceData() {
  if (!supportsResourceData.value || !selectedSchema.value || !selectedTable.value) return
  dataLoading.value = true
  try {
    const data = await queryDBMSResourceData({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      pageNum: tableQuery.pageNum,
      pageSize: tableQuery.pageSize,
      filterKey: tableFilter.key,
      filterText: tableFilter.text
    })
    selectedColumns.value = data.columns || []
    selectedRows.value = data.rows || []
    selectedTotal.value = Number(data.total || 0)
    selectedPrimaryKeys.value = []
    resourceIndexes.value = data.indexes || []
    resourceType.value = data.resourceType || ''
  } finally {
    dataLoading.value = false
  }
}

function refreshSelectedData() {
  return supportsResourceData.value ? loadResourceData() : loadTableData()
}

function formatResourceValue(value) {
  if (value == null) return '-'
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return String(value)
    }
  }
  return String(value)
}

async function initialize() {
  loading.value = true
  try {
    await Promise.all([loadConnection(), loadTree(), loadHistory(), loadTasks()])
  } finally {
    loading.value = false
  }
}

function switchDatabase(database) {
  if (!database?.id || Number(database.id) === databaseId.value) return
  if (database.connectStatus !== 1) {
    ElMessage.warning('현재 Database가 연결되지 않았습니다. Connection 설정을 확인하십시오.')
    return
  }
  router.replace({ name: 'DatabaseWorkbench', params: { id: database.id } })
}

function resetWorkbenchState() {
  selectedSchema.value = ''
  selectedTable.value = ''
  treeKeyword.value = ''
  Object.keys(schemaTablePages).forEach((key) => delete schemaTablePages[key])
  schemaTree.value = []
  resultColumns.value = []
  resultRows.value = []
  selectedColumns.value = []
  selectedRows.value = []
  selectedTotal.value = 0
  selectedPrimaryKeys.value = []
  resourceIndexes.value = []
  resourceType.value = ''
  historyList.value = []
  taskList.value = []
  execMeta.sqlType = ''
  execMeta.rowsAffected = 0
  execMeta.durationMs = 0
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

function onTreeNodeClick(node) {
  if (node.isSchema) {
    selectedSchema.value = node.name
    const firstTable = node.children?.[0]
    if (!firstTable) {
      selectedTable.value = ''
      selectedColumns.value = []
      selectedRows.value = []
      selectedTotal.value = 0
      activeTab.value = 'data'
      ElMessage.info(`Schema ${node.name}에 탐색 가능한 Table이 없습니다.`)
      return
    }
    treeRef.value?.setCurrentKey(firstTable.id)
    onTreeNodeClick(firstTable)
    return
  }
  if (!node.isTable) return
  selectedSchema.value = node.schema
  selectedTable.value = node.name
  tableQuery.pageNum = 1
  activeTab.value = 'data'
  if (supportsResourceData.value) {
    loadResourceData()
    return
  }
  if (!supportsTableData.value) {
    ElMessage.info('현재 Database Type은 Connection과 Structure 탐색을 지원합니다. Data 편집은 추후 Version에서 제공합니다.')
    return
  }
  loadTableData()
}

function selectedSQLText() {
  const editor = sqlEditorRef.value
  if (!editor) return sqlText.value.trim()
  const selected = sqlText.value.slice(editor.selectionStart, editor.selectionEnd).trim()
  return selected || sqlText.value.trim()
}

function isProductionEnvironment(value) {
  const environment = String(value || '').toLowerCase()
  return environment.includes('prod') || environment.includes('\u751f\u4ea7')
}

function sqlConfirmationText() {
  return isProductionEnvironment(sqlAnalysis.value?.environment) ? '운영 환경' : '실행 확인'
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
  ElMessage.success('SQL 실행을 완료했습니다.')
  await Promise.all([loadHistory(), loadTasks()])
  if (selectedTable.value) {
    await loadTableData()
  }
}

async function runSQL() {
  if (!supportsSQL.value) {
    ElMessage.warning('현재 Database Type은 SQL Workbench를 사용하지 않습니다. 왼쪽 Resource Tree에서 Connection과 Structure를 확인하십시오.')
    return
  }
  const statement = selectedSQLText()
  if (!statement) {
    ElMessage.warning('실행할 SQL을 입력하십시오.')
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
    ElMessage.warning(`실행을 확인하려면 “${sqlConfirmationText()}”을(를) 입력하십시오.`)
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

async function executeRedisCommandText(confirmed = false) {
  const data = await executeRedisCommand({
    databaseId: databaseId.value,
    commandText: redisCommandText.value.trim(),
    confirmed
  })
  resultColumns.value = data.columns || []
  resultRows.value = data.rows || []
  execMeta.sqlType = `REDIS ${data.command || ''}`.trim()
  execMeta.rowsAffected = Number(data.rowsAffected || 0)
  execMeta.durationMs = Number(data.durationMs || 0)
  activeTab.value = 'result'
  ElMessage.success('Redis Command 실행을 완료했습니다.')
  await loadHistory()
  if (selectedTable.value) await loadResourceData()
}

async function runRedisCommand() {
  if (!isRedis.value) return
  if (!redisCommandText.value.trim()) {
    ElMessage.warning('Redis Command를 입력하십시오.')
    return
  }
  redisRunning.value = true
  resetExecMeta()
  try {
    const analysis = await analyzeRedisCommand({
      databaseId: databaseId.value,
      commandText: redisCommandText.value.trim()
    })
    if (analysis.writeOperation) {
      await confirmRiskOperation({
        operation: `Redis Write Command: ${analysis.command}`,
        targetSummary: `${connection.value?.databaseName || connection.value?.name || databaseId.value}`,
        production: isProductionEnvironment(connection.value?.environment),
        destructive: true
      })
    }
    await executeRedisCommandText(analysis.writeOperation)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  } finally {
    redisRunning.value = false
  }
}

function quoteRedisArgument(value) {
  return JSON.stringify(String(value ?? ''))
}

function resetRedisKeyForm() {
  redisKeyForm.key = ''
  redisKeyForm.type = 'string'
  redisKeyForm.value = ''
  redisKeyForm.ttl = undefined
}

function openRedisKeyCreate() {
  resetRedisKeyForm()
  redisKeyDialogMode.value = 'create'
  redisKeyDialogVisible.value = true
}

function openRedisKeyEdit(row) {
  if (row.type === 'stream') {
    ElMessage.warning('Stream Type은 아직 Visual Editing을 지원하지 않습니다. 위 Redis Command Console을 사용하십시오.')
    return
  }
  redisKeyDialogMode.value = 'edit'
  redisKeyForm.key = row.key || ''
  redisKeyForm.type = row.type || 'string'
  redisKeyForm.value = row.value || ''
  redisKeyForm.ttl = Number(row.ttlSeconds) >= 0 ? Number(row.ttlSeconds) : undefined
  redisKeyDialogVisible.value = true
}

function parseRedisValueArguments() {
  const raw = redisKeyForm.value ?? ''
  if (redisKeyForm.type === 'string') return [quoteRedisArgument(raw)]
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch {
    throw new Error('Complex Type Value는 유효한 JSON이어야 합니다.')
  }
  if (redisKeyForm.type === 'hash') {
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('Hash Value는 JSON Object여야 합니다.')
    const entries = Object.entries(parsed)
    if (!entries.length) throw new Error('Hash에는 하나 이상의 Field가 필요합니다.')
    return entries.flatMap(([field, value]) => [quoteRedisArgument(field), quoteRedisArgument(value)])
  }
  if (redisKeyForm.type === 'zset') {
    if (!Array.isArray(parsed) || !parsed.length) throw new Error('Sorted Set Value는 비어 있지 않은 JSON Array여야 합니다.')
    return parsed.flatMap((item) => {
      if (!item || item.member === undefined || Number.isNaN(Number(item.score))) throw new Error('Sorted Set Element에는 member와 score가 필요합니다.')
      return [quoteRedisArgument(item.score), quoteRedisArgument(item.member)]
    })
  }
  if (!Array.isArray(parsed) || !parsed.length) throw new Error('List 또는 Set Value는 비어 있지 않은 JSON Array여야 합니다.')
  return parsed.map((item) => quoteRedisArgument(item))
}

function redisTTLCommand(key) {
  if (redisKeyForm.ttl === undefined || redisKeyForm.ttl === null || redisKeyForm.ttl === '') return ''
  const ttl = Number(redisKeyForm.ttl)
  if (!Number.isInteger(ttl) || ttl < -1) throw new Error('TTL은 -1 또는 0 이상의 정수 초로 설정해야 합니다.')
  return ttl === -1 ? `PERSIST ${quoteRedisArgument(key)}` : `EXPIRE ${quoteRedisArgument(key)} ${ttl}`
}

function buildRedisKeyCommands() {
  const key = redisKeyForm.key.trim()
  if (!key) throw new Error('Key 이름을 입력하십시오.')
  const valueArgs = parseRedisValueArguments()
  const quotedKey = quoteRedisArgument(key)
  const editing = redisKeyDialogMode.value === 'edit'
  let command = ''
  switch (redisKeyForm.type) {
    case 'string':
      command = `SET ${quotedKey} ${valueArgs[0]}${editing && redisKeyForm.ttl === undefined ? ' KEEPTTL' : ''}`
      break
    case 'hash': command = `HSET ${quotedKey} ${valueArgs.join(' ')}`; break
    case 'list': command = `RPUSH ${quotedKey} ${valueArgs.join(' ')}`; break
    case 'set': command = `SADD ${quotedKey} ${valueArgs.join(' ')}`; break
    case 'zset': command = `ZADD ${quotedKey} ${valueArgs.join(' ')}`; break
    default: throw new Error('지원하지 않는 Redis Key Type입니다.')
  }
  const commands = []
  if (editing && redisKeyForm.type !== 'string') commands.push(`DEL ${quotedKey}`)
  commands.push(command)
  const ttlCommand = redisTTLCommand(key)
  if (ttlCommand) commands.push(ttlCommand)
  return commands
}

async function submitRedisKey() {
  let commands
  try {
    commands = buildRedisKeyCommands()
  } catch (error) {
    ElMessage.warning(error.message || 'Key 내용이 올바르지 않습니다.')
    return
  }
  const action = redisKeyDialogMode.value === 'create' ? '추가' : '업데이트'
  try {
    await confirmRiskOperation({
      operation: `${action} Redis Key`,
      targetSummary: redisKeyForm.key,
      production: isProductionEnvironment(connection.value?.environment),
      destructive: redisKeyDialogMode.value === 'edit'
    })
  } catch {
    return
  }
  redisRunning.value = true
  try {
    for (const commandText of commands) {
      await executeRedisCommand({ databaseId: databaseId.value, commandText, confirmed: true })
    }
    redisKeyDialogVisible.value = false
    await Promise.all([loadTree(), loadHistory()])
    selectedSchema.value = schemaTree.value?.[0]?.name || selectedSchema.value
    selectedTable.value = redisKeyForm.key
    await loadResourceData()
    ElMessage.success(`Redis Key ${action}을(를) 완료했습니다.`)
  } finally {
    redisRunning.value = false
  }
}

async function deleteRedisKey(row) {
  try {
    await confirmRiskOperation({
      operation: 'Redis Key 삭제',
      targetSummary: row.key,
      production: isProductionEnvironment(connection.value?.environment),
      destructive: true
    })
  } catch {
    return
  }
  redisRunning.value = true
  try {
    await executeRedisCommand({ databaseId: databaseId.value, commandText: `DEL ${quoteRedisArgument(row.key)}`, confirmed: true })
    if (selectedTable.value === row.key) {
      selectedTable.value = ''
      selectedRows.value = []
      selectedColumns.value = []
    }
    await Promise.all([loadTree(), loadHistory()])
    ElMessage.success('Redis Key를 삭제했습니다.')
  } finally {
    redisRunning.value = false
  }
}

function riskTagType(level) {
  if (level === 'high') return 'danger'
  if (level === 'medium') return 'warning'
  return 'success'
}

function insertSnippet(text) {
  sqlText.value = text
  nextTick(() => {
    sqlEditorRef.value?.focus()
    syncEditorMetrics()
  })
}

function updateAutocomplete() {
  const editor = sqlEditorRef.value
  if (!editor) return
  const before = sqlText.value.slice(0, editor.selectionStart)
  const match = before.match(/[A-Za-z_][A-Za-z0-9_.]*$/)
  currentToken.value = match ? match[0] : ''
  activeSuggestionIndex.value = 0
  showSuggestions.value = !!currentToken.value && suggestions.value.length > 0
  syncEditorMetrics()
}

function applySuggestion(value) {
  const editor = sqlEditorRef.value
  if (!editor) return
  const cursor = editor.selectionStart
  const before = sqlText.value.slice(0, cursor)
  const after = sqlText.value.slice(cursor)
  const replacedBefore = before.replace(/[A-Za-z_][A-Za-z0-9_.]*$/, value)
  sqlText.value = replacedBefore + after
  showSuggestions.value = false
  nextTick(() => {
    editor.focus()
    const pos = replacedBefore.length
    editor.setSelectionRange(pos, pos)
    syncEditorMetrics()
  })
}

function onEditorKeydown(event) {
  if (showSuggestions.value && suggestions.value.length) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      activeSuggestionIndex.value = (activeSuggestionIndex.value + 1) % suggestions.value.length
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      activeSuggestionIndex.value = (activeSuggestionIndex.value - 1 + suggestions.value.length) % suggestions.value.length
      return
    }
    if (event.key === 'Tab' || event.key === 'Enter') {
      event.preventDefault()
      applySuggestion(suggestions.value[activeSuggestionIndex.value])
      return
    }
    if (event.key === 'Escape') {
      showSuggestions.value = false
      return
    }
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'enter') {
    event.preventDefault()
    runSQL()
  }
}

function openRollback(row) {
  rollbackSQL.value = row.rollbackSql || ''
  rollbackConfidence.value = row.rollbackConfidence || ''
  rollbackDialogVisible.value = true
}

async function copyRollback(row) {
  await navigator.clipboard.writeText(row.rollbackSql || '')
  ElMessage.success('Rollback SQL을 복사했습니다.')
}

function openInsertRow() {
  rowDialogMode.value = 'insert'
  rowOriginal.value = {}
  Object.keys(rowForm).forEach((key) => delete rowForm[key])
  for (const col of selectedColumns.value) {
    rowForm[col.name] = null
  }
  rowDialogVisible.value = true
}

function openEditRow(row) {
  rowDialogMode.value = 'update'
  rowOriginal.value = { ...row }
  Object.keys(rowForm).forEach((key) => delete rowForm[key])
  for (const col of selectedColumns.value) {
    rowForm[col.name] = row[col.name] ?? null
  }
  rowDialogVisible.value = true
}

async function submitRow() {
  await confirmRiskOperation({
    operation: rowDialogMode.value === 'insert' ? 'Table Data 추가' : 'Table Data 업데이트',
    targetSummary: `${selectedSchema.value}.${selectedTable.value}`,
    production: isProductionEnvironment(connection.value?.environment),
    destructive: rowDialogMode.value === 'update'
  })
  if (rowDialogMode.value === 'insert') {
    await insertDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      row: { ...rowForm }
    })
    ElMessage.success('추가했습니다.')
  } else {
    await updateDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      original: rowOriginal.value,
      current: { ...rowForm }
    })
    ElMessage.success('업데이트했습니다.')
  }
  rowDialogVisible.value = false
  await Promise.all([loadTableData(), loadHistory()])
}

async function handleDeleteRow(row) {
  await confirmRiskOperation({
    operation: 'Table Data Row 삭제',
    targetSummary: `${selectedSchema.value}.${selectedTable.value}`,
    production: isProductionEnvironment(connection.value?.environment),
    destructive: true
  })
  await deleteDBMSTableRow({
    databaseId: databaseId.value,
    schema: selectedSchema.value,
    table: selectedTable.value,
    row
  })
  ElMessage.success('삭제했습니다.')
  await Promise.all([loadTableData(), loadHistory()])
}

async function createExportTask() {
  if (!selectedSchema.value || !selectedTable.value) {
    ElMessage.warning('Export할 Table을 먼저 선택하십시오.')
    return
  }
  await createDBMSExportTask({
    databaseId: databaseId.value,
    schema: selectedSchema.value,
    table: selectedTable.value,
    includeData: true
  })
  ElMessage.success('Export Task를 생성했습니다.')
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
    ElMessage.warning('현재 Workbench의 Target Table을 먼저 선택하십시오.')
    return
  }
  if (!importPrecheck.value?.ready) {
    ElMessage.warning('Import Precheck를 먼저 완료하십시오.')
    return
  }
  await createDBMSImportTask(importPayload())
  importDialogVisible.value = false
  ElMessage.success('Import Task를 생성했습니다.')
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
    ElMessage.warning('Source Database, Source Table 및 Target Table을 모두 선택하십시오.')
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
  if (status === 'success') return '성공'
  if (status === 'failed') return '실패'
  if (status === 'running') return '실행 중'
  return '대기 중'
}

watch(treeKeyword, () => {
  Object.keys(schemaTablePages).forEach((key) => delete schemaTablePages[key])
})

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

watch([() => tableFilter.key, () => tableFilter.text], () => {
  tableQuery.pageNum = 1
  if (selectedSchema.value && selectedTable.value) {
    refreshSelectedData()
  }
})

onMounted(initialize)

watch(databaseId, async (next, previous) => {
  if (!previous || next === previous) return
  resetWorkbenchState()
  await initialize()
})

onBeforeUnmount(() => {
  if (taskTimer.value) {
    window.clearInterval(taskTimer.value)
  }
})
</script>

<template>
  <div v-loading="loading" class="dbms-page">
    <header class="dbms-console-bar">
      <div class="dbms-console-identity">
        <span>DBMS WORKBENCH</span>
        <strong>{{ connection?.name || connection?.databaseName || 'Database Workbench' }}</strong>
        <small>{{ connection?.host || connection?.address || 'Connection을 선택하면 Object 탐색, Statement 실행 및 결과 확인이 가능합니다.' }}</small>
      </div>
      <div class="dbms-console-status">
        <el-tag effect="plain">{{ (connection?.dbType || 'DBMS').toUpperCase() }}</el-tag>
        <el-tag v-if="connection?.environment" type="info" effect="plain">{{ connection.environment }}</el-tag>
        <el-tag :type="isReadOnly ? 'warning' : 'success'" effect="plain">{{ isReadOnly ? 'Read-only Connection' : 'Writable Connection' }}</el-tag>
      </div>
    </header>
    <div class="dbms-layout">
      <aside class="dbms-sidebar page-card">
        <section class="sidebar-section connection-section">
          <div class="sidebar-section-title">
            <strong>Database Connection</strong>
            <span>Connection을 클릭해 전환</span>
          </div>
          <DatabaseConnectionTree :active-id="databaseId" @select="switchDatabase" />
        </section>

        <section class="sidebar-section schema-section">
          <div class="sidebar-section-title">
            <strong>Database / Table Structure</strong>
            <div class="schema-title-actions">
              <el-button v-if="supportsCreateDatabase" type="primary" link @click="openCreateDatabase">
                추가{{ createDatabaseObjectLabel }}
              </el-button>
              <span>{{ connection?.dbName || '전체 Database' }}</span>
            </div>
          </div>
          <div class="sidebar-top">
            <el-input v-model="treeKeyword" clearable placeholder="Database 또는 Table 검색" />
            <el-button @click="loadTree">새로고침</el-button>
          </div>
          <el-tree
            ref="treeRef"
            v-loading="treeLoading"
            node-key="id"
            class="schema-tree"
            :data="pagedSchemaTree"
            :props="{ label: 'label', children: 'children' }"
            :filter-node-method="treeFilterMethod"
            default-expand-all
            :expand-on-click-node="false"
            @node-click="onTreeNodeClick"
          >
            <template #default="{ data }">
              <div class="tree-node">
                <span>{{ data.label }}</span>
                <small v-if="data.isTable && Number.isFinite(data.rows)">{{ data.rows }}</small>
                <template v-else-if="data.isSchema">
                  <span class="schema-node-meta" @click.stop>
                    <small>{{ data.visibleTableCount ?? data.tableCount }}</small>
                    <span v-if="data.totalPages > 1" class="schema-pagination">
                      <el-button
                        text
                        size="small"
                        :disabled="data.currentPage <= 1"
                        title="이전 Page"
                        @click.stop="changeSchemaTablePage(data, data.currentPage - 1)"
                      >이전 Page</el-button>
                      <span>{{ data.currentPage }}/{{ data.totalPages }}</span>
                      <el-button
                        text
                        size="small"
                        :disabled="data.currentPage >= data.totalPages"
                        title="다음 Page"
                        @click.stop="changeSchemaTablePage(data, data.currentPage + 1)"
                      >다음 Page</el-button>
                    </span>
                  </span>
                </template>
              </div>
            </template>
          </el-tree>
        </section>
      </aside>

      <section class="dbms-main">
        <div class="dbms-editor page-card">
          <div class="panel-head">
            <div>
              <h3>{{ supportsSQL ? 'SQL Editor' : isRedis ? 'Redis Command Console' : `${connection?.dbType?.toUpperCase() || 'Database'} Resource Explorer` }}</h3>
              <p v-if="supportsSQL">Syntax Highlighting, Keyword 및 Database/Table Column 자동완성, 자주 쓰는 Query Template과 Quick Execution을 지원합니다.</p>
              <p v-else-if="isRedis">일반적인 Redis Read/Write Command를 지원합니다. Write 작업은 재확인 후 Execution History에 기록됩니다.</p>
              <p v-else>현재 Type은 Connection Test와 Resource Structure 탐색을 지원합니다. Document, Key-Value 및 Data 편집 기능은 Type별로 단계적으로 제공합니다.</p>
            </div>
            <div class="panel-actions">
              <el-button v-if="supportsSQL" :loading="sqlRunning" type="primary" @click="runSQL">SQL 실행</el-button>
              <el-button v-if="supportsSQL" @click="formatSQL">Format</el-button>
              <el-button v-if="supportsSQL" @click="saveCurrentSQL">SQL Favorite</el-button>
              <el-dropdown v-if="supportsSQL && sqlFavorites.length" trigger="click">
                <el-button>Favorite {{ sqlFavorites.length }}</el-button>
                <template #dropdown>
                  <el-dropdown-menu class="sql-favorite-menu">
                    <el-dropdown-item v-for="item in sqlFavorites" :key="item.id" class="sql-favorite-item">
                      <span @click="applySqlFavorite(item)">{{ item.name }}</span>
                      <el-button link type="danger" size="small" @click.stop="removeSqlFavorite(item)">삭제</el-button>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <el-button v-if="!supportsSQL && isRedis" :loading="redisRunning" type="primary" @click="runRedisCommand">Redis Command 실행</el-button>
              <el-button :disabled="!supportsExport || !selectedTable" @click="createExportTask">Export Task</el-button>
              <el-button :disabled="!supportsImport || isReadOnly" @click="openImportDialog">Import Task</el-button>
            </div>
          </div>

          <div v-if="supportsSQL" class="snippet-row">
            <el-button v-for="item in sqlSnippets" :key="item.label" size="small" plain @click="insertSnippet(item.text)">
              {{ item.label }}
            </el-button>
              <span class="snippet-hint">`Ctrl + Enter`로 선택한 SQL을 실행합니다. Format과 Favorite는 선택한 SQL에만 적용되며 선택 영역이 없으면 전체 내용에 적용됩니다.</span>
          </div>

          <div v-else-if="isRedis" class="redis-command-console">
            <div class="redis-command-head">
              <strong>Redis Command Console</strong>
              <span>Read Command는 즉시 실행할 수 있고 Write Command는 확인이 필요합니다. 고위험 관리 Command는 비활성화되어 있습니다.</span>
            </div>
            <div class="redis-command-snippets">
              <el-button v-for="item in redisCommandSnippets" :key="item.label" size="small" plain @click="redisCommandText = item.text">
                {{ item.label }}
              </el-button>
            </div>
            <el-input
              v-model="redisCommandText"
              class="redis-command-input"
              type="textarea"
              :rows="6"
              spellcheck="false"
              placeholder="예: GET key, HGETALL hash_key 또는 SET key value"
              @keydown.ctrl.enter.prevent="runRedisCommand"
              @keydown.meta.enter.prevent="runRedisCommand"
            />
            <div class="redis-command-hint">Quote가 포함된 Parameter를 지원합니다. Ctrl + Enter로 실행하며 기록은 아래 History에 보존됩니다.</div>
          </div>

          <el-alert v-if="!supportsSQL && !isRedis" class="database-capability-alert" type="info" :closable="false" show-icon title="현재 Database는 SQL Type이 아닙니다.">
            왼쪽에서 Database와 Resource를 선택해 Structure를 확인할 수 있습니다. MySQL은 전체 Data 편집, Import/Export 및 Backup 기능을 제공합니다.
          </el-alert>

          <div v-if="supportsSQL" ref="editorWrapRef" class="editor-shell">
            <div class="line-gutter" :style="{ transform: `translateY(-${sqlScrollTop}px)` }">
              <div v-for="line in sqlLines" :key="line" class="line-number" :class="{ active: line === currentLine }">
                {{ line }}
              </div>
            </div>
            <div class="editor-layer">
              <pre class="sql-highlight" :style="{ transform: `translate(${-sqlScrollLeft}px, ${-sqlScrollTop}px)` }" v-html="highlightedSQL + '\n'"></pre>
              <textarea
                ref="sqlEditorRef"
                v-model="sqlText"
                class="sql-editor"
                spellcheck="false"
                placeholder="SQL을 입력하십시오. 선택한 Statement를 실행할 수 있습니다."
                @input="updateAutocomplete"
                @click="updateAutocomplete"
                @keyup="updateAutocomplete"
                @keydown="onEditorKeydown"
                @scroll="syncEditorMetrics"
                @blur="setTimeout(() => (showSuggestions = false), 150)"
              />
            </div>
            <div v-if="showSuggestions && suggestions.length" class="autocomplete-panel">
              <button
                v-for="(item, index) in suggestions"
                :key="item"
                type="button"
                class="autocomplete-item"
                :class="{ active: index === activeSuggestionIndex }"
                @mousedown.prevent="applySuggestion(item)"
              >
                {{ item }}
              </button>
            </div>
          </div>

          <div v-if="execMeta.sqlType" class="exec-meta">
            <span>Type：{{ execMeta.sqlType }}</span>
            <span>Affected Rows：{{ execMeta.rowsAffected }}</span>
            <span>Duration: {{ execMeta.durationMs }} ms</span>
          </div>
        </div>

        <div class="dbms-content page-card">
          <div class="panel-head">
            <div>
              <h3>Workspace</h3>
              <p v-if="selectedTable">{{ selectedSchema }} / {{ selectedTable }}</p>
              <p v-else>왼쪽에서 Table을 선택하거나 SQL을 직접 실행해 결과를 확인하십시오.</p>
            </div>
            <div v-if="selectedTable || isRedis" class="panel-actions">
              <el-button v-if="canManageRedisKeys" type="primary" plain @click="openRedisKeyCreate">Key 추가</el-button>
              <el-button v-else-if="selectedTable" type="primary" plain :disabled="!canEditRows" @click="openInsertRow">Data 추가</el-button>
              <el-button @click="refreshSelectedData">Data 새로고침</el-button>
            </div>
          </div>

          <el-tabs v-model="activeTab">
            <el-tab-pane label="Table Data" name="data">
              <div class="filter-row">
                <el-select v-model="tableFilter.key" clearable placeholder="Filter Column" style="width: 180px">
                  <el-option v-for="item in filterableColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="tableFilter.text" clearable placeholder="Filter Value 입력, Fuzzy Match 지원" style="width: 280px" />
              </div>
              <el-alert
                v-if="supportsResourceData"
                class="resource-readonly-alert"
                type="info"
                :closable="false"
                show-icon
                :title="isRedis && canManageRedisKeys ? '현재 Redis Connection은 Key 직접 관리를 지원합니다.' : '현재 Resource는 Read-only로 탐색합니다.'"
              >
                <template v-if="resourceType === 'collection'">Column 기준으로 MongoDB Document를 Filter할 수 있으며 아래에 Collection Index 요약을 표시합니다.</template>
                <template v-else-if="resourceType === 'key'">
                  <span v-if="canManageRedisKeys">Redis Key 추가, 수정, 삭제를 지원합니다. 모든 Write 작업은 확인 후 Audit에 기록됩니다.</span>
                  <span v-else>Redis Key의 Type, TTL 및 Value 요약을 표시하며 Key를 직접 변경하지 않습니다.</span>
                </template>
                <template v-else>PostgreSQL Table Data는 Pagination과 Column Filter를 지원합니다. SQL Editor에서 EXPLAIN ANALYZE로 Execution Plan을 확인할 수 있습니다.</template>
              </el-alert>
              <el-table v-if="supportsResourceData" v-loading="dataLoading" :data="selectedRows" border height="380">
                <el-table-column v-for="col in selectedColumns" :key="col.name" :label="col.name" min-width="180" show-overflow-tooltip>
                  <template #default="{ row }">{{ formatResourceValue(row[col.name]) }}</template>
                </el-table-column>
                <el-table-column v-if="isRedis" label="작업" width="150" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!canManageRedisKeys || row.type === 'stream'" @click="openRedisKeyEdit(row)">수정</el-button>
                    <el-button link type="danger" :disabled="!canManageRedisKeys" @click="deleteRedisKey(row)">삭제</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <el-table v-else v-loading="dataLoading" :data="selectedRows" border height="380">
                <el-table-column v-for="(col, index) in selectedColumns" :key="col.name" :label="col.name" min-width="160">
                  <template #default="{ row, $index }">
                    <el-input
                      v-if="isEditingCell(row, col.name, $index)"
                      v-model="pendingCellValue"
                      size="small"
                      @keyup.enter="commitCellEdit(row, col.name)"
                      @blur="commitCellEdit(row, col.name)"
                    />
                    <button v-else type="button" class="cell-button" :class="{ disabled: !canEditRows }" @click="startCellEdit(row, col.name, $index)">
                      {{ row[col.name] ?? '-' }}
                    </button>
                  </template>
                </el-table-column>
                <el-table-column label="작업" width="150" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!canEditRows" @click="openEditRow(row)">수정</el-button>
                    <el-button link type="danger" :disabled="!canEditRows" @click="handleDeleteRow(row)">삭제</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div v-if="supportsResourceData && resourceIndexes.length" class="resource-indexes">
                <strong>Collection Index</strong>
                <el-tag v-for="(item, index) in resourceIndexes" :key="index" effect="plain">
                  {{ formatResourceValue(item.name || item.key || item) }}
                </el-tag>
              </div>
              <div class="pager">
                <el-pagination
                  v-model:current-page="tableQuery.pageNum"
                  v-model:page-size="tableQuery.pageSize"
                  :total="selectedTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="refreshSelectedData"
                  @size-change="refreshSelectedData"
                />
              </div>
            </el-tab-pane>

            <el-tab-pane label="SQL 결과" name="result">
              <div class="filter-row result-filter-row">
                <el-select v-model="resultFilter.key" clearable placeholder="Filter Column" style="width: 180px">
                  <el-option v-for="item in resultColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="resultFilter.text" clearable placeholder="Result Set Filter" style="width: 280px" />
                <el-button :disabled="!filteredResultRows.length" @click="exportResultCSV">CSV Export</el-button>
              </div>
              <el-table :data="filteredResultRows" border height="380">
                <el-table-column v-for="col in resultColumns" :key="col" :prop="col" :label="col" min-width="160" />
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="Execution History" name="history">
              <el-table v-loading="historyLoading" :data="historyList" border height="380">
                <el-table-column prop="executionId" label="Execution ID" min-width="190" show-overflow-tooltip />
                <el-table-column prop="sqlType" label="Type" width="100" />
                <el-table-column prop="environment" label="Environment" width="90" />
                <el-table-column prop="schemaName" label="Database" width="120" />
                <el-table-column prop="tableName" label="Table" width="140" />
                <el-table-column prop="sqlText" label="SQL" min-width="320" show-overflow-tooltip />
                <el-table-column label="상태" width="90">
                  <template #default="{ row }">
                    <el-tag :type="row.status === 1 ? 'success' : 'danger'" effect="light">
                      {{ row.status === 1 ? '성공' : '실패' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" label="Affected Rows" width="110" />
                <el-table-column prop="durationMs" label="Duration (ms)" width="100" />
                <el-table-column prop="operator" label="Operator" width="110" />
                <el-table-column prop="clientIp" label="Client IP" width="140" />
                <el-table-column prop="createTime" label="Executed At" min-width="160" />
                <el-table-column label="Rollback SQL" width="150">
                  <template #default="{ row }">
                    <el-tag v-if="row.rollbackSql" :type="row.rollbackConfidence === 'high' ? 'success' : 'warning'" size="small" effect="plain">
                      {{ row.rollbackConfidence === 'high' ? '높은 신뢰도' : '검토 필요' }}
                    </el-tag>
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="openRollback(row)">보기</el-button>
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="copyRollback(row)">복사</el-button>
                  </template>
                </el-table-column>
                <el-table-column label="작업" width="80" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="reuseHistorySQL(row)">재사용</el-button></template></el-table-column>
              </el-table>
              <div class="pager">
                <el-pagination
                  v-model:current-page="historyQuery.pageNum"
                  v-model:page-size="historyQuery.pageSize"
                  :total="historyTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="loadHistory"
                  @size-change="loadHistory"
                />
              </div>
            </el-tab-pane>

            <el-tab-pane label="Import / Export Task" name="tasks">
              <el-table v-loading="taskLoading" :data="taskList" border height="380">
                <el-table-column prop="taskType" label="Type" width="90">
                  <template #default="{ row }">{{ row.taskType === 'export' ? 'Export' : 'Import' }}</template>
                </el-table-column>
                <el-table-column label="Source" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.sourceDatabase }} / {{ row.sourceSchema }} / {{ row.sourceTable }}</span>
                    <span v-else>{{ row.databaseName }} / {{ row.schemaName }} / {{ row.tableName }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="Target" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.targetDatabase }} / {{ row.targetSchema }} / {{ row.targetTable }}</span>
                    <span v-else>-</span>
                  </template>
                </el-table-column>
                <el-table-column label="상태" width="110">
                  <template #default="{ row }">
                    <el-tag :type="taskStatusType(row.status)" effect="light">{{ taskStatusText(row.status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="진행률" width="180">
                  <template #default="{ row }">
                    <el-progress :percentage="Number(row.progress || 0)" :status="row.status === 'failed' ? 'exception' : row.status === 'success' ? 'success' : ''" />
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" label="Row 수" width="90" />
                <el-table-column prop="message" label="설명" min-width="180" show-overflow-tooltip />
                <el-table-column prop="createTime" label="Created At" min-width="160" />
                <el-table-column label="작업" width="120">
                  <template #default="{ row }">
                    <el-button
                      v-if="row.taskType === 'export' && row.status === 'success'"
                      link
                      type="primary"
                      @click="downloadTask(row)"
                    >
                      Download
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="pager">
                <el-pagination
                  v-model:current-page="taskQuery.pageNum"
                  v-model:page-size="taskQuery.pageSize"
                  :total="taskTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="loadTasks"
                  @size-change="loadTasks"
                />
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </section>
    </div>

    <el-dialog v-model="createDatabaseVisible" :title="`${createDatabaseObjectLabel} 추가`" width="520px" destroy-on-close>
      <el-alert
        :title="isPostgres ? '현재 연결된 PostgreSQL Database에 Schema를 생성합니다. 생성 후 왼쪽 Structure Tree에 즉시 표시됩니다.' : '현재 MySQL Instance에 Database를 직접 생성합니다. 생성 후 왼쪽 Structure Tree에 즉시 표시됩니다.'"
        type="info"
        :closable="false"
        show-icon
      />
      <el-form label-width="112px" class="create-database-form">
        <el-form-item :label="`${createDatabaseObjectLabel} 이름`" required>
          <el-input v-model="createDatabaseForm.name" maxlength="63" show-word-limit placeholder="예: ops_reporting" @keyup.enter="submitCreateDatabase" />
        </el-form-item>
        <template v-if="!isPostgres">
          <el-form-item label="Character Set">
            <el-select v-model="createDatabaseForm.charset" style="width: 100%" :loading="charsetOptionsLoading" @change="onCreateDatabaseCharsetChange">
              <el-option v-for="item in mysqlCharsetOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="Collation">
            <el-select v-model="createDatabaseForm.collation" style="width: 100%" :loading="charsetOptionsLoading" :disabled="charsetOptionsLoading">
              <el-option v-for="item in availableCollations" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
        </template>
        <p class="form-help">Letter 또는 underscore로 시작해야 하며 letter, number, underscore만 사용할 수 있습니다.</p>
      </el-form>
      <template #footer>
        <el-button @click="createDatabaseVisible = false">취소</el-button>
        <el-button type="primary" :loading="creatingDatabase" @click="submitCreateDatabase">생성 확인</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="sqlConfirmVisible" title="SQL Write 실행 확인" width="780px">
      <div v-if="sqlAnalysis" class="sql-risk-panel">
        <div class="sql-risk-summary">
          <el-tag :type="riskTagType(sqlAnalysis.riskLevel)" effect="dark">
            {{ sqlAnalysis.riskLevel === 'high' ? 'High Risk' : sqlAnalysis.riskLevel === 'medium' ? 'Medium Risk' : 'Low Risk' }}
          </el-tag>
          <strong>{{ sqlAnalysis.databaseName }} / {{ sqlAnalysis.schema }}</strong>
          <span>{{ sqlAnalysis.environment || 'Environment 미지정' }}</span>
        </div>
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="SQL Type">{{ sqlAnalysis.sqlType }}</el-descriptions-item>
          <el-descriptions-item label="Statement 수">{{ sqlAnalysis.statementCount }}</el-descriptions-item>
          <el-descriptions-item label="Access Mode">{{ sqlAnalysis.accessMode === 'readonly' ? 'Read-only' : 'Read / Write' }}</el-descriptions-item>
        </el-descriptions>
        <ul class="risk-reasons">
          <li v-for="item in sqlAnalysis.reasons" :key="item">{{ item }}</li>
        </ul>
        <pre class="confirm-sql">{{ pendingSQL }}</pre>
        <el-alert
          v-if="sqlAnalysis.accessMode === 'readonly'"
          title="현재 Database는 Read-only Mode이므로 Backend가 이 SQL 실행을 거부합니다."
          type="error"
          :closable="false"
          show-icon
        />
        <el-alert v-else title="Target Environment, Database 및 SQL 내용을 확인하십시오. Write 작업은 자동 복구되지 않을 수 있습니다." type="warning" :closable="false" show-icon />
        <el-form-item class="sql-acknowledgement" :label="`실행을 확인하려면 “${sqlConfirmationText()}”을(를) 입력하십시오.`">
          <el-input v-model="sqlAcknowledgement" :placeholder="sqlConfirmationText()" autocomplete="off" />
        </el-form-item>
      </div>
      <template #footer>
        <el-button @click="sqlConfirmVisible = false">취소</el-button>
        <el-button
          type="danger"
          :loading="sqlRunning"
          :disabled="sqlAnalysis?.accessMode === 'readonly' || sqlAcknowledgement !== sqlConfirmationText()"
          @click="confirmSQLExecution"
        >
          실행 확인
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="redisKeyDialogVisible" :title="redisKeyDialogMode === 'create' ? 'Redis Key 추가' : 'Redis Key 수정'" width="720px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="Redis Write는 즉시 적용되며 Execution Audit에 기록됩니다."
        class="redis-key-alert"
      />
      <el-form label-width="112px" class="redis-key-form">
        <el-form-item label="Key 이름" required>
          <el-input v-model="redisKeyForm.key" :disabled="redisKeyDialogMode === 'edit'" placeholder="예: app:config" />
        </el-form-item>
        <el-form-item label="Data Type">
          <el-select v-model="redisKeyForm.type" :disabled="redisKeyDialogMode === 'edit'" style="width: 100%">
            <el-option v-for="type in redisKeyTypes" :key="type" :label="type" :value="type" />
          </el-select>
        </el-form-item>
        <el-form-item label="Value" required>
          <el-input v-model="redisKeyForm.value" type="textarea" :rows="8" :placeholder="redisKeyValuePlaceholder" />
        </el-form-item>
        <el-form-item label="TTL (초)">
          <el-input-number v-model="redisKeyForm.ttl" :min="-1" :precision="0" controls-position="right" />
          <span class="redis-key-help">비워두면 TTL을 변경하지 않습니다. -1은 만료 없음입니다.</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="redisKeyDialogVisible = false">취소</el-button>
        <el-button type="primary" :loading="redisRunning" @click="submitRedisKey">저장 확인</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rowDialogVisible" :title="rowDialogMode === 'insert' ? 'Data 추가행' : 'Data Row 수정'" width="720px">
      <el-form label-width="140px">
        <el-form-item v-for="col in selectedColumns" :key="col.name" :label="`${col.name} (${col.columnType})`">
          <el-input v-model="rowForm[col.name]" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rowDialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submitRow">저장</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rollbackDialogVisible" title="Rollback SQL" width="860px">
      <el-alert
        :title="rollbackConfidence === 'high' ? '높은 신뢰도: Primary Key와 변경 전 Data를 기준으로 생성했습니다. 실행 전에도 반드시 검토하십시오.' : '이 Rollback SQL은 신뢰도가 제한적입니다. 수동 검토 후 사용하십시오.'"
        :type="rollbackConfidence === 'high' ? 'success' : 'warning'"
        :closable="false"
        show-icon
        class="rollback-alert"
      />
      <pre class="rollback-box">{{ rollbackSQL || '현재 Record에는 Rollback SQL이 없습니다.' }}</pre>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="Import Task 생성" width="680px">
      <el-form label-width="120px">
        <el-form-item label="Source Database">
          <el-select v-model="importForm.sourceDatabaseId" filterable style="width: 100%">
              <el-option
                v-for="item in importDatabaseOptions"
                :key="item.id"
                :label="`${item.name} (${String(item.dbType || 'mysql').toUpperCase()} · ${item.host}:${item.port})`"
                :value="item.id"
              />
          </el-select>
        </el-form-item>
        <el-form-item label="Source Database">
          <el-select v-model="importForm.sourceSchema" filterable style="width: 100%">
            <el-option v-for="item in sourceSchemas" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source Table">
          <el-select v-model="importForm.sourceTable" filterable style="width: 100%">
            <el-option v-for="item in sourceTables" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="Target Table">
          <div>{{ selectedSchema || '-' }} / {{ selectedTable || '-' }}</div>
        </el-form-item>
        <el-form-item label="Table 자동 생성">
          <el-switch v-model="importForm.createIfMissing" />
        </el-form-item>
        <el-form-item label="Target Table 비우기">
          <el-switch v-model="importForm.truncateTarget" />
        </el-form-item>
        <el-form-item label="Precheck">
          <el-button :loading="importPrechecking" @click="runImportPrecheck">Column 및 영향 범위 검사</el-button>
        </el-form-item>
        <div v-if="importPrecheck" class="import-precheck" :class="{ danger: !importPrecheck.ready }">
          <div class="precheck-head">
            <strong>{{ importPrecheck.ready ? 'Precheck 통과' : 'Precheck 실패' }}</strong>
            <el-tag :type="importPrecheck.ready ? 'success' : 'danger'">{{ importPrecheck.estimatedRows }} 행</el-tag>
          </div>
          <p>Column Mapping: {{ importPrecheck.commonColumns?.length || 0 }}개, 누락 Column: {{ importPrecheck.missingColumns?.length || 0 }}개</p>
          <p v-if="importPrecheck.missingColumns?.length">누락: {{ importPrecheck.missingColumns.join(', ') }}</p>
          <ul v-if="importPrecheck.warnings?.length">
            <li v-for="item in importPrecheck.warnings" :key="item">{{ item }}</li>
          </ul>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">취소</el-button>
        <el-button type="primary" :disabled="!importPrecheck?.ready" @click="submitImportTask">Import 시작</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.dbms-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dbms-console-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  min-height: 72px;
  padding: 14px 18px;
  border: 1px solid #dce6f3;
  border-left: 4px solid #4569d8;
  border-radius: 12px;
  background: linear-gradient(120deg, #ffffff, #f5f8ff);
  box-shadow: 0 5px 14px rgba(35, 65, 115, .05);
}

.dbms-console-identity span,
.dbms-console-identity strong,
.dbms-console-identity small { display: block; }
.dbms-console-identity span { color: #5871b2; font-size: 10px; font-weight: 800; letter-spacing: .12em; }
.dbms-console-identity strong { margin-top: 3px; color: #1c3154; font-size: 18px; }
.dbms-console-identity small { margin-top: 3px; color: #75849b; font-size: 12px; }
.dbms-console-status { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }

.dbms-header,
.dbms-sidebar,
.dbms-editor,
.dbms-content {
  border-radius: 14px;
}

.dbms-header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
}

.dbms-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
}

.page-desc {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
}

.connection-metas {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--el-text-color-secondary);
}

.dbms-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
  min-height: 760px;
}

.dbms-sidebar {
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 0;
  overflow: hidden;
}

.sidebar-section {
  min-height: 0;
  padding: 16px;
}

.connection-section {
  flex: 0 0 auto;
  max-height: 340px;
  overflow: auto;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.schema-section {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
}

.sidebar-section-title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.sidebar-section-title strong {
  color: var(--el-text-color-primary);
  font-size: 15px;
}

.sidebar-section-title span {
  overflow: hidden;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-title-actions {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.schema-title-actions .el-button {
  flex: none;
  padding: 0;
}

.create-database-form {
  margin-top: 18px;
}

.form-help {
  margin: -8px 0 0 112px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.sidebar-top {
  display: flex;
  gap: 10px;
}

.schema-tree {
  flex: 1;
  overflow: auto;
}

.tree-node {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.tree-node small {
  color: var(--el-text-color-secondary);
}

.schema-node-meta,
.schema-pagination {
  display: inline-flex;
  align-items: center;
}

.schema-node-meta {
  margin-left: auto;
  gap: 6px;
}

.schema-pagination {
  gap: 2px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.schema-pagination .el-button {
  min-width: auto;
  height: 20px;
  padding: 0 2px;
  font-size: 11px;
}

.dbms-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 14px;
}

.panel-head h3 {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
}

.panel-head p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
}

.panel-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.snippet-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 12px;
}

.database-capability-alert {
  margin-top: 16px;
}

.redis-command-console {
  margin-top: 16px;
  padding: 16px;
  border: 1px solid #253653;
  border-radius: 10px;
  background: #0f172a;
}

.redis-command-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
  margin-bottom: 12px;
  color: #e2e8f0;
}

.redis-command-head span,
.redis-command-hint {
  color: #94a3b8;
  font-size: 12px;
}

.redis-command-snippets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.redis-command-input :deep(.el-textarea__inner) {
  min-height: 132px;
  border-color: #334155;
  background: #111827;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  line-height: 1.65;
}

.redis-command-input :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset;
}

.redis-command-hint {
  margin-top: 10px;
}

.resource-readonly-alert {
  margin-bottom: 12px;
}

.resource-indexes {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.snippet-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: auto;
}

.editor-shell {
  position: relative;
  display: grid;
  grid-template-columns: 56px minmax(0, 1fr);
  min-height: 260px;
  max-height: 380px;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: 14px;
  background: #0f172a;
}

.line-gutter {
  padding-top: 14px;
  background: rgba(15, 23, 42, 0.92);
  border-right: 1px solid rgba(148, 163, 184, 0.16);
}

.line-number {
  height: 22px;
  padding: 0 12px 0 0;
  text-align: right;
  color: #64748b;
  font-size: 13px;
  line-height: 22px;
  font-family: Consolas, 'Courier New', monospace;
}

.line-number.active {
  color: #f8fafc;
}

.editor-layer {
  position: relative;
  overflow: hidden;
}

.sql-highlight,
.sql-editor {
  margin: 0;
  padding: 14px 16px;
  font-size: 14px;
  line-height: 22px;
  font-family: Consolas, 'Courier New', monospace;
  white-space: pre;
}

.sql-highlight {
  position: absolute;
  inset: 0;
  overflow: hidden;
  color: #e2e8f0;
  pointer-events: none;
}

:deep(.token-keyword) {
  color: #60a5fa;
  font-weight: 600;
}

:deep(.token-string) {
  color: #fbbf24;
}

:deep(.token-number) {
  color: #34d399;
}

:deep(.token-comment) {
  color: #64748b;
}

.sql-editor {
  position: relative;
  z-index: 1;
  width: 100%;
  min-height: 260px;
  height: 100%;
  border: none;
  resize: none;
  outline: none;
  background: transparent;
  color: transparent;
  caret-color: #f8fafc;
  overflow: auto;
}

.autocomplete-panel {
  position: absolute;
  left: 76px;
  top: calc(100% - 6px);
  width: 280px;
  max-height: 240px;
  overflow: auto;
  background: #111827;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 12px;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.38);
  z-index: 30;
}

.autocomplete-item {
  width: 100%;
  display: block;
  border: none;
  background: transparent;
  text-align: left;
  padding: 10px 12px;
  color: #e5e7eb;
  cursor: pointer;
}

.autocomplete-item:hover,
.autocomplete-item.active {
  background: rgba(59, 130, 246, 0.18);
}

.exec-meta {
  margin-top: 12px;
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  color: var(--el-text-color-secondary);
}

.filter-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.cell-button {
  display: block;
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  text-align: left;
  color: inherit;
  cursor: pointer;
  min-height: 22px;
}

.cell-button:hover {
  color: var(--el-color-primary);
}

.cell-button.disabled {
  color: var(--el-text-color-secondary);
  cursor: not-allowed;
}

.sql-risk-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sql-risk-summary,
.precheck-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sql-risk-summary span {
  color: var(--el-text-color-secondary);
}

.risk-reasons {
  margin: 0;
  padding-left: 20px;
  color: #a16207;
}

.confirm-sql {
  max-height: 260px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  line-height: 1.6;
  white-space: pre-wrap;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.rollback-box {
  margin: 0;
  min-height: 240px;
  max-height: 520px;
  overflow: auto;
  padding: 16px;
  border-radius: 12px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.7;
  font-family: Consolas, 'Courier New', monospace;
  white-space: pre-wrap;
}

.rollback-alert {
  margin-bottom: 12px;
}

.sql-acknowledgement {
  margin: 16px 0 0;
}

.redis-key-alert {
  margin-bottom: 18px;
}

.redis-key-form .el-form-item:last-child {
  margin-bottom: 0;
}

.redis-key-help {
  margin-left: 12px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.import-precheck {
  margin: 0 0 12px 120px;
  padding: 14px;
  border: 1px solid #b7e4ca;
  border-radius: 8px;
  background: #f0f9f4;
  color: #315947;
}

.import-precheck.danger {
  border-color: #f2b8b5;
  background: #fff5f5;
  color: #8f3232;
}

.import-precheck p {
  margin: 8px 0 0;
}

.import-precheck ul {
  margin: 8px 0 0;
  padding-left: 20px;
}

@media (max-width: 1080px) {
  .dbms-layout {
    grid-template-columns: 1fr;
  }

  .dbms-header {
    flex-direction: column;
  }

  .dbms-console-bar { align-items: flex-start; flex-direction: column; }
  .dbms-console-status { justify-content: flex-start; }
}
</style>
