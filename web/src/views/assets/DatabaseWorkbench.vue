<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
import {
  createDBMSExportTask,
  createDBMSImportTask,
  deleteDBMSTableRow,
  downloadDBMSTaskFile,
  executeDBMSSQL,
  insertDBMSTableRow,
  queryDBMSSchemaTree,
  queryDBMSSQLHistory,
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
const dataLoading = ref(false)
const historyLoading = ref(false)
const taskLoading = ref(false)
const rowDialogVisible = ref(false)
const rollbackDialogVisible = ref(false)
const importDialogVisible = ref(false)
const activeTab = ref('data')
const rowDialogMode = ref('insert')

const connection = ref(null)
const schemaTree = ref([])
const resultColumns = ref([])
const resultRows = ref([])
const selectedColumns = ref([])
const selectedRows = ref([])
const selectedTotal = ref(0)
const selectedPrimaryKeys = ref([])
const historyList = ref([])
const historyTotal = ref(0)
const taskList = ref([])
const taskTotal = ref(0)
const importDatabaseOptions = ref([])
const importSchemaTree = ref([])
const rollbackSQL = ref('')

const selectedSchema = ref('')
const selectedTable = ref('')
const treeKeyword = ref('')
const showSuggestions = ref(false)
const currentToken = ref('')
const activeSuggestionIndex = ref(0)
const currentLine = ref(1)
const sqlText = ref('SELECT *\nFROM your_table\nLIMIT 50;')
const sqlScrollTop = ref(0)
const sqlScrollLeft = ref(0)
const sqlEditorRef = ref(null)
const editorWrapRef = ref(null)
const treeRef = ref(null)
const taskTimer = ref(null)

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

const importForm = reactive({
  sourceDatabaseId: undefined,
  sourceSchema: '',
  sourceTable: '',
  createIfMissing: true,
  truncateTarget: false
})

const sqlKeywordPool = [
  'SELECT', 'FROM', 'WHERE', 'ORDER BY', 'GROUP BY', 'HAVING', 'LIMIT', 'OFFSET',
  'INSERT INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE FROM', 'CREATE TABLE', 'ALTER TABLE',
  'DROP TABLE', 'SHOW TABLES', 'SHOW DATABASES', 'DESCRIBE', 'EXPLAIN', 'LEFT JOIN',
  'RIGHT JOIN', 'INNER JOIN', 'UNION ALL', 'COUNT', 'SUM', 'MIN', 'MAX', 'AVG', 'NOW()'
]

const sqlSnippets = [
  { label: 'SELECT', text: 'SELECT *\nFROM table_name\nLIMIT 50;' },
  { label: 'UPDATE', text: "UPDATE table_name\nSET column_name = 'value'\nWHERE id = 1;" },
  { label: 'INSERT', text: "INSERT INTO table_name (column_1, column_2)\nVALUES ('value_1', 'value_2');" },
  { label: 'DELETE', text: 'DELETE FROM table_name\nWHERE id = 1;' },
  { label: 'SHOW TABLES', text: 'SHOW TABLES;' }
]

const sourceSchemas = computed(() => importSchemaTree.value || [])
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

function normalizeTree(data) {
  return (data.schemas || []).map((schema) => ({
    id: `schema:${schema.name}`,
    label: schema.name,
    name: schema.name,
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

function syncEditorMetrics() {
  const el = sqlEditorRef.value
  if (!el) return
  sqlScrollTop.value = el.scrollTop
  sqlScrollLeft.value = el.scrollLeft
  const before = sqlText.value.slice(0, el.selectionStart)
  currentLine.value = before.split('\n').length
}

function treeFilterMethod(value, data) {
  if (!value) return true
  return String(data.label || data.name || '').toLowerCase().includes(String(value).toLowerCase())
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
  ElMessage.success('单元格已更新')
  cancelCellEdit()
  await Promise.all([loadTableData(), loadHistory()])
}

async function loadConnection() {
  connection.value = await queryDBMSWorkbench(databaseId.value)
}

async function loadTree() {
  treeLoading.value = true
  try {
    const data = await queryDBMSSchemaTree(databaseId.value)
    schemaTree.value = normalizeTree(data)
    if (!selectedSchema.value && data.defaultSchema) {
      selectedSchema.value = data.defaultSchema
    }
  } finally {
    treeLoading.value = false
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

async function initialize() {
  loading.value = true
  try {
    await Promise.all([loadConnection(), loadTree(), loadHistory(), loadTasks()])
  } finally {
    loading.value = false
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

function onTreeNodeClick(node) {
  if (!node.isTable) return
  selectedSchema.value = node.schema
  selectedTable.value = node.name
  tableQuery.pageNum = 1
  activeTab.value = 'data'
  loadTableData()
}

function selectedSQLText() {
  const editor = sqlEditorRef.value
  if (!editor) return sqlText.value.trim()
  const selected = sqlText.value.slice(editor.selectionStart, editor.selectionEnd).trim()
  return selected || sqlText.value.trim()
}

async function runSQL() {
  const statement = selectedSQLText()
  if (!statement) {
    ElMessage.warning('请输入要执行的 SQL')
    return
  }
  sqlRunning.value = true
  resetExecMeta()
  try {
    const data = await executeDBMSSQL({
      databaseId: databaseId.value,
      schema: selectedSchema.value || connection.value?.dbName || '',
      sqlText: statement
    })
    resultColumns.value = data.columns || []
    resultRows.value = data.rows || []
    execMeta.sqlType = data.sqlType || ''
    execMeta.rowsAffected = Number(data.rowsAffected || 0)
    execMeta.durationMs = Number(data.durationMs || 0)
    activeTab.value = 'result'
    ElMessage.success('SQL 执行完成')
    await Promise.all([loadHistory(), loadTasks()])
    if (selectedTable.value) {
      await loadTableData()
    }
  } finally {
    sqlRunning.value = false
  }
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
  rollbackDialogVisible.value = true
}

async function copyRollback(row) {
  await navigator.clipboard.writeText(row.rollbackSql || '')
  ElMessage.success('回滚 SQL 已复制')
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
  if (rowDialogMode.value === 'insert') {
    await insertDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      row: { ...rowForm }
    })
    ElMessage.success('新增成功')
  } else {
    await updateDBMSTableRow({
      databaseId: databaseId.value,
      schema: selectedSchema.value,
      table: selectedTable.value,
      original: rowOriginal.value,
      current: { ...rowForm }
    })
    ElMessage.success('更新成功')
  }
  rowDialogVisible.value = false
  await Promise.all([loadTableData(), loadHistory()])
}

async function handleDeleteRow(row) {
  await ElMessageBox.confirm(`确认删除 ${selectedTable.value} 当前数据行吗？`, '提示', { type: 'warning' })
  await deleteDBMSTableRow({
    databaseId: databaseId.value,
    schema: selectedSchema.value,
    table: selectedTable.value,
    row
  })
  ElMessage.success('删除成功')
  await Promise.all([loadTableData(), loadHistory()])
}

async function createExportTask() {
  if (!selectedSchema.value || !selectedTable.value) {
    ElMessage.warning('请先选择要导出的表')
    return
  }
  await createDBMSExportTask({
    databaseId: databaseId.value,
    schema: selectedSchema.value,
    table: selectedTable.value,
    includeData: true
  })
  ElMessage.success('导出任务已创建')
  activeTab.value = 'tasks'
  await loadTasks()
}

async function openImportDialog() {
  const list = await queryAssetDatabaseList({ pageNum: 1, pageSize: 200, keyword: '', dbType: 'mysql', status: '1' })
  importDatabaseOptions.value = list.list || []
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
    ElMessage.warning('请先选择当前工作台目标表')
    return
  }
  await createDBMSImportTask({
    sourceDatabaseId: importForm.sourceDatabaseId,
    sourceSchema: importForm.sourceSchema,
    sourceTable: importForm.sourceTable,
    targetDatabaseId: databaseId.value,
    targetSchema: selectedSchema.value,
    targetTable: selectedTable.value,
    createIfMissing: importForm.createIfMissing,
    truncateTarget: importForm.truncateTarget
  })
  importDialogVisible.value = false
  ElMessage.success('导入任务已创建')
  activeTab.value = 'tasks'
  await loadTasks()
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
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'running') return '执行中'
  return '等待中'
}

watch(treeKeyword, (value) => {
  treeRef.value?.filter(value)
})

watch(() => importForm.sourceDatabaseId, loadImportSchemas)

watch([() => tableFilter.key, () => tableFilter.text], () => {
  tableQuery.pageNum = 1
  if (selectedSchema.value && selectedTable.value) {
    loadTableData()
  }
})

onMounted(initialize)

onBeforeUnmount(() => {
  if (taskTimer.value) {
    window.clearInterval(taskTimer.value)
  }
})
</script>

<template>
  <div v-loading="loading" class="dbms-page">
    <div class="dbms-header page-card">
      <div class="dbms-header-main">
        <div class="dbms-breadcrumb">
          <el-button link type="primary" @click="router.push('/assets/databases')">数据库管理</el-button>
          <span>/</span>
          <span>{{ connection?.name || '数据库工作台' }}</span>
        </div>
        <h2 class="page-title">{{ connection?.name || '数据库工作台' }}</h2>
        <p class="page-desc">面向 MySQL 的 SQL 编辑、库表浏览、结果筛选、任务化导入导出和执行记录管理台。</p>
      </div>
      <div class="connection-metas">
        <span>{{ connection?.host }}:{{ connection?.port }}</span>
        <span>{{ connection?.dbName || '-' }}</span>
        <span>{{ connection?.version || '-' }}</span>
      </div>
    </div>

    <div class="dbms-layout">
      <aside class="dbms-sidebar page-card">
        <div class="sidebar-top">
          <el-input v-model="treeKeyword" clearable placeholder="搜索库 / 表" />
          <el-button @click="loadTree">刷新</el-button>
        </div>
        <el-tree
          ref="treeRef"
          v-loading="treeLoading"
          node-key="id"
          class="schema-tree"
          :data="schemaTree"
          :props="{ label: 'label', children: 'children' }"
          :filter-node-method="treeFilterMethod"
          default-expand-all
          @node-click="onTreeNodeClick"
        >
          <template #default="{ data }">
            <div class="tree-node">
              <span>{{ data.label }}</span>
              <small v-if="data.isTable && Number.isFinite(data.rows)">{{ data.rows }}</small>
            </div>
          </template>
        </el-tree>
      </aside>

      <section class="dbms-main">
        <div class="dbms-editor page-card">
          <div class="panel-head">
            <div>
              <h3>SQL 编辑器</h3>
              <p>支持语法高亮、关键字/库表字段补全、常用语句模板和快捷执行。</p>
            </div>
            <div class="panel-actions">
              <el-button :loading="sqlRunning" type="primary" @click="runSQL">执行 SQL</el-button>
              <el-button :disabled="!selectedTable" @click="createExportTask">导出任务</el-button>
              <el-button @click="openImportDialog">导入任务</el-button>
            </div>
          </div>

          <div class="snippet-row">
            <el-button v-for="item in sqlSnippets" :key="item.label" size="small" plain @click="insertSnippet(item.text)">
              {{ item.label }}
            </el-button>
            <span class="snippet-hint">`Ctrl + Enter` 可执行当前选中 SQL</span>
          </div>

          <div ref="editorWrapRef" class="editor-shell">
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
                placeholder="请输入 SQL，支持执行选中的语句。"
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
            <span>类型：{{ execMeta.sqlType }}</span>
            <span>影响行数：{{ execMeta.rowsAffected }}</span>
            <span>耗时：{{ execMeta.durationMs }} ms</span>
          </div>
        </div>

        <div class="dbms-content page-card">
          <div class="panel-head">
            <div>
              <h3>工作区</h3>
              <p v-if="selectedTable">{{ selectedSchema }} / {{ selectedTable }}</p>
              <p v-else>先从左侧选中一张表，或直接执行 SQL 查看结果。</p>
            </div>
            <div v-if="selectedTable" class="panel-actions">
              <el-button type="primary" plain @click="openInsertRow">新增数据</el-button>
              <el-button @click="loadTableData">刷新数据</el-button>
            </div>
          </div>

          <el-tabs v-model="activeTab">
            <el-tab-pane label="表数据" name="data">
              <div class="filter-row">
                <el-select v-model="tableFilter.key" clearable placeholder="筛选字段" style="width: 180px">
                  <el-option v-for="item in filterableColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="tableFilter.text" clearable placeholder="输入筛选值，支持模糊匹配" style="width: 280px" />
              </div>
              <el-table v-loading="dataLoading" :data="selectedRows" border height="380">
                <el-table-column v-for="(col, index) in selectedColumns" :key="col.name" :label="col.name" min-width="160">
                  <template #default="{ row, $index }">
                    <el-input
                      v-if="isEditingCell(row, col.name, $index)"
                      v-model="pendingCellValue"
                      size="small"
                      @keyup.enter="commitCellEdit(row, col.name)"
                      @blur="commitCellEdit(row, col.name)"
                    />
                    <button v-else type="button" class="cell-button" @click="startCellEdit(row, col.name, $index)">
                      {{ row[col.name] ?? '-' }}
                    </button>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="150" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" @click="openEditRow(row)">编辑</el-button>
                    <el-button link type="danger" @click="handleDeleteRow(row)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="pager">
                <el-pagination
                  v-model:current-page="tableQuery.pageNum"
                  v-model:page-size="tableQuery.pageSize"
                  :total="selectedTotal"
                  layout="total, sizes, prev, pager, next"
                  @current-change="loadTableData"
                  @size-change="loadTableData"
                />
              </div>
            </el-tab-pane>

            <el-tab-pane label="SQL 结果" name="result">
              <div class="filter-row">
                <el-select v-model="resultFilter.key" clearable placeholder="筛选字段" style="width: 180px">
                  <el-option v-for="item in resultColumns" :key="item" :label="item" :value="item" />
                </el-select>
                <el-input v-model="resultFilter.text" clearable placeholder="筛选结果集" style="width: 280px" />
              </div>
              <el-table :data="filteredResultRows" border height="380">
                <el-table-column v-for="col in resultColumns" :key="col" :prop="col" :label="col" min-width="160" />
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="执行记录" name="history">
              <el-table v-loading="historyLoading" :data="historyList" border height="380">
                <el-table-column prop="sqlType" label="类型" width="100" />
                <el-table-column prop="schemaName" label="库" width="120" />
                <el-table-column prop="tableName" label="表" width="140" />
                <el-table-column prop="sqlText" label="SQL" min-width="320" show-overflow-tooltip />
                <el-table-column label="状态" width="90">
                  <template #default="{ row }">
                    <el-tag :type="row.status === 1 ? 'success' : 'danger'" effect="light">
                      {{ row.status === 1 ? '成功' : '失败' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" label="影响行数" width="110" />
                <el-table-column prop="durationMs" label="耗时(ms)" width="100" />
                <el-table-column prop="createTime" label="执行时间" min-width="160" />
                <el-table-column label="回滚 SQL" width="150">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="openRollback(row)">查看</el-button>
                    <el-button link type="primary" :disabled="!row.rollbackSql" @click="copyRollback(row)">复制</el-button>
                  </template>
                </el-table-column>
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

            <el-tab-pane label="导入导出任务" name="tasks">
              <el-table v-loading="taskLoading" :data="taskList" border height="380">
                <el-table-column prop="taskType" label="类型" width="90">
                  <template #default="{ row }">{{ row.taskType === 'export' ? '导出' : '导入' }}</template>
                </el-table-column>
                <el-table-column label="源" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.sourceDatabase }} / {{ row.sourceSchema }} / {{ row.sourceTable }}</span>
                    <span v-else>{{ row.databaseName }} / {{ row.schemaName }} / {{ row.tableName }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="目标" min-width="220">
                  <template #default="{ row }">
                    <span v-if="row.taskType === 'import'">{{ row.targetDatabase }} / {{ row.targetSchema }} / {{ row.targetTable }}</span>
                    <span v-else>-</span>
                  </template>
                </el-table-column>
                <el-table-column label="状态" width="110">
                  <template #default="{ row }">
                    <el-tag :type="taskStatusType(row.status)" effect="light">{{ taskStatusText(row.status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="进度" width="180">
                  <template #default="{ row }">
                    <el-progress :percentage="Number(row.progress || 0)" :status="row.status === 'failed' ? 'exception' : row.status === 'success' ? 'success' : ''" />
                  </template>
                </el-table-column>
                <el-table-column prop="rowsAffected" label="行数" width="90" />
                <el-table-column prop="message" label="说明" min-width="180" show-overflow-tooltip />
                <el-table-column prop="createTime" label="创建时间" min-width="160" />
                <el-table-column label="操作" width="120">
                  <template #default="{ row }">
                    <el-button
                      v-if="row.taskType === 'export' && row.status === 'success'"
                      link
                      type="primary"
                      @click="downloadTask(row)"
                    >
                      下载
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

    <el-dialog v-model="rowDialogVisible" :title="rowDialogMode === 'insert' ? '新增数据行' : '编辑数据行'" width="720px">
      <el-form label-width="140px">
        <el-form-item v-for="col in selectedColumns" :key="col.name" :label="`${col.name} (${col.columnType})`">
          <el-input v-model="rowForm[col.name]" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rowDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRow">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rollbackDialogVisible" title="回滚 SQL" width="860px">
      <pre class="rollback-box">{{ rollbackSQL || '当前记录没有回滚 SQL' }}</pre>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="创建导入任务" width="680px">
      <el-form label-width="120px">
        <el-form-item label="源数据库">
          <el-select v-model="importForm.sourceDatabaseId" filterable style="width: 100%">
            <el-option
              v-for="item in importDatabaseOptions"
              :key="item.id"
              :label="`${item.name} (${item.host}:${item.port})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="源库">
          <el-select v-model="importForm.sourceSchema" filterable style="width: 100%">
            <el-option v-for="item in sourceSchemas" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="源表">
          <el-select v-model="importForm.sourceTable" filterable style="width: 100%">
            <el-option v-for="item in sourceTables" :key="item.name" :label="item.name" :value="item.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标库表">
          <div>{{ selectedSchema || '-' }} / {{ selectedTable || '-' }}</div>
        </el-form-item>
        <el-form-item label="自动建表">
          <el-switch v-model="importForm.createIfMissing" />
        </el-form-item>
        <el-form-item label="清空目标表">
          <el-switch v-model="importForm.truncateTarget" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitImportTask">开始导入</el-button>
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
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 16px;
  min-height: 760px;
}

.dbms-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
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

@media (max-width: 1320px) {
  .dbms-layout {
    grid-template-columns: 1fr;
  }

  .dbms-header {
    flex-direction: column;
  }
}
</style>
