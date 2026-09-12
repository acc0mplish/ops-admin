<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { at } from '../../utils/asset-i18n'
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createDBMSSchema, queryDBMSCharsetOptions, queryDBMSResourceData, queryDBMSTableData, queryDBMSWorkbench } from '../../api/dbms'
import DatabaseConnectionTree from './database/DatabaseConnectionTree.vue'
import DatabaseSchemaTree from './database/DatabaseSchemaTree.vue'
import DatabaseSqlEditor from './database/DatabaseSqlEditor.vue'
import DatabaseWorkspacePanel from './database/DatabaseWorkspacePanel.vue'
import DatabaseWorkbenchDialogs from './database/DatabaseWorkbenchDialogs.vue'
import DatabaseRedisKeyDialog from './database/DatabaseRedisKeyDialog.vue'
import { useDbmsSchemaTree } from '../../composables/useDbmsSchemaTree'
import { useDbmsSqlEditor } from '../../composables/useDbmsSqlEditor'
import { useDbmsTransferTasks } from '../../composables/useDbmsTransferTasks'
import { useDbmsSqlExecution } from '../../composables/useDbmsSqlExecution'
import { useDbmsRedis } from '../../composables/useDbmsRedis'
import { useDbmsRowOps } from '../../composables/useDbmsRowOps'

// I5-H (i5-plan §3.8·CW-WB) — DatabaseWorkbench.vue 2,456행 분할 잔류본.
// 스크립트 본체는 composables/useDbms*.js 6종, 템플릿/CSS 덩어리는 database/
// 자식 5종으로 이동했고 각 커밋 메시지에 원본 좌표가 있다. 자식은 `:page`
// 주입 패턴(§5 #11 승계)으로 상태를 받는다 — 상태 소유는 뷰+컴포저블에 남고
// 자식은 page.x 재배선만 수행한다.
const route = useRoute()
const router = useRouter()

const databaseId = computed(() => Number(route.params.id || 0))

const loading = ref(false)
const dataLoading = ref(false)
const activeTab = ref('data')

const connection = ref(null)
const selectedColumns = ref([])
const selectedRows = ref([])
const selectedTotal = ref(0)
const selectedPrimaryKeys = ref([])
const resourceIndexes = ref([])
const resourceType = ref('')
const selectedSchema = ref('')
const selectedTable = ref('')

const tableQuery = reactive({
  pageNum: 1,
  pageSize: 25
})

const tableFilter = reactive({
  key: '',
  text: ''
})

const isReadOnly = computed(() => connection.value?.accessMode === 'readonly')
const capabilities = computed(() => connection.value?.capabilities || {})
const supportsSQL = computed(() => capabilities.value.sql !== false)
const isRedis = computed(() => connection.value?.dbType === 'redis')
const isPostgres = computed(() => connection.value?.dbType === 'postgresql')
const supportsCreateDatabase = computed(() => !isReadOnly.value && ['mysql', 'postgresql'].includes(connection.value?.dbType))
const createDatabaseObjectLabel = computed(() => (isPostgres.value ? 'Schema' : 'Database'))
const supportsTableData = computed(() => capabilities.value.tableData !== false)
const supportsResourceData = computed(() => capabilities.value.resourceData === true)
const supportsExport = computed(() => capabilities.value.export === true || (capabilities.value.export === undefined && capabilities.value.transfer !== false))
const supportsImport = computed(() => capabilities.value.import === true || (capabilities.value.import === undefined && capabilities.value.transfer !== false))
const canEditRows = computed(() => supportsTableData.value && !isReadOnly.value && selectedPrimaryKeys.value.length > 0 && capabilities.value.rowEdit !== false)
const canManageRedisKeys = computed(() => isRedis.value && !isReadOnly.value && capabilities.value.keyEdit !== false)

const filterableColumns = computed(() => selectedColumns.value.map((item) => item.name))

const tree = useDbmsSchemaTree({ databaseId, selectedSchema })

const editorBridges = {}
const editor = useDbmsSqlEditor({
  databaseId,
  isPostgres,
  schemaTree: tree.schemaTree,
  selectedColumns,
  bridges: editorBridges
})

const transfer = useDbmsTransferTasks({ databaseId, selectedSchema, selectedTable, activeTab })

const execution = useDbmsSqlExecution({
  databaseId,
  connection,
  selectedSchema,
  selectedTable,
  sqlText: editor.sqlText,
  selectedSQLText: editor.selectedSQLText,
  supportsSQL,
  activeTab,
  loadHistory: transfer.loadHistory,
  loadTasks: transfer.loadTasks,
  loadTableData
})

// ctrl+enter 브릿지 — 실행 도메인이 편집기 뒤에 생성되어 늦게 주입한다.
editorBridges.runSQL = execution.runSQL

const redis = useDbmsRedis({
  databaseId,
  connection,
  isRedis,
  selectedSchema,
  selectedTable,
  selectedRows,
  selectedColumns,
  schemaTree: tree.schemaTree,
  execMeta: execution.execMeta,
  resultColumns: execution.resultColumns,
  resultRows: execution.resultRows,
  activeTab,
  resetExecMeta: execution.resetExecMeta,
  loadTree: tree.loadTree,
  loadHistory: transfer.loadHistory,
  loadResourceData
})

const rowOps = useDbmsRowOps({
  databaseId,
  selectedSchema,
  selectedTable,
  selectedColumns,
  selectedPrimaryKeys,
  canEditRows,
  isReadOnly,
  connection,
  loadTableData,
  loadHistory: transfer.loadHistory
})

const { loadTree } = tree
const { loadHistory, loadTasks } = transfer

async function loadConnection() {
  connection.value = await queryDBMSWorkbench(databaseId.value)
  editor.loadSqlFavorites()
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
    ElMessage.warning(at('disconnectedDatabaseWarning'))
    return
  }
  router.replace({ name: 'DatabaseWorkbench', params: { id: database.id } })
}

// 재배선 — 트리/결과/이력/태스크 상태가 각 도메인 컴포저블로 이동했다
// (원본 680-699의 전체 리셋 순서는 그대로 보존).
function resetWorkbenchState() {
  selectedSchema.value = ''
  selectedTable.value = ''
  tree.treeKeyword.value = ''
  Object.keys(tree.schemaTablePages).forEach((key) => delete tree.schemaTablePages[key])
  tree.schemaTree.value = []
  execution.resultColumns.value = []
  execution.resultRows.value = []
  selectedColumns.value = []
  selectedRows.value = []
  selectedTotal.value = 0
  selectedPrimaryKeys.value = []
  resourceIndexes.value = []
  resourceType.value = ''
  transfer.historyList.value = []
  transfer.taskList.value = []
  execution.execMeta.sqlType = ''
  execution.execMeta.rowsAffected = 0
  execution.execMeta.durationMs = 0
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
      ElMessage.info(at('schemaNoTables', { name: node.name }))
      return
    }
    tree.treeRef.value?.setCurrentKey(firstTable.id)
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
    ElMessage.info(at('typeBrowsingOnlyInfo'))
    return
  }
  loadTableData()
}

// 재배선 — sqlText·에디터 포커스는 편집기 도메인 소관 (원본 357-361).
function reuseHistorySQL(row) {
  editor.sqlText.value = row.sqlText || ''
  activeTab.value = 'result'
  nextTick(() => editor.sqlEditorRef.value?.focus())
}

const createDatabaseVisible = ref(false)
const creatingDatabase = ref(false)
const charsetOptionsLoading = ref(false)
const createDatabaseForm = reactive({
  name: '',
  charset: 'utf8mb4',
  collation: 'utf8mb4_0900_ai_ci'
})

const mysqlCharsetOptions = ref([])
const availableCollations = ref([])

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
    ElMessage.warning(at('enterObjectName', { label: createDatabaseObjectLabel.value }))
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
    ElMessage.success(at('createObjectSuccess', { label: createDatabaseObjectLabel.value, name }))
  } finally {
    creatingDatabase.value = false
  }
}

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

const page = reactive({
  // 뷰 소관 상태·능력 computed
  loading,
  dataLoading,
  activeTab,
  connection,
  selectedSchema,
  selectedTable,
  selectedColumns,
  selectedRows,
  selectedTotal,
  selectedPrimaryKeys,
  resourceIndexes,
  resourceType,
  tableQuery,
  tableFilter,
  isReadOnly,
  isRedis,
  isPostgres,
  supportsSQL,
  supportsCreateDatabase,
  createDatabaseObjectLabel,
  supportsExport,
  supportsImport,
  canEditRows,
  canManageRedisKeys,
  filterableColumns,
  // 뷰 소관 함수
  loadTableData,
  loadResourceData,
  refreshSelectedData,
  formatResourceValue,
  onTreeNodeClick,
  reuseHistorySQL,
  createDatabaseVisible,
  creatingDatabase,
  charsetOptionsLoading,
  createDatabaseForm,
  mysqlCharsetOptions,
  availableCollations,
  openCreateDatabase,
  onCreateDatabaseCharsetChange,
  submitCreateDatabase,
  // 도메인 컴포저블
  ...tree,
  ...editor,
  ...transfer,
  ...execution,
  ...redis,
  ...rowOps
})
</script>

<template>
  <div v-loading="loading" class="dbms-page">
    <header class="dbms-console-bar">
      <div class="dbms-console-identity">
        <span>{{ uiT('dbmsWorkbench') }}</span>
        <strong>{{ connection?.name || connection?.databaseName || 'Database Workbench' }}</strong>
        <small>{{ connection?.host || connection?.address || at('workbenchIdleHint') }}</small>
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
            <strong>{{ uiT('databaseConnection') }}</strong>
            <span>{{ at('clickConnectionToSwitch') }}</span>
          </div>
          <DatabaseConnectionTree :active-id="databaseId" @select="switchDatabase" />
        </section>

        <DatabaseSchemaTree :page="page" />
      </aside>

      <section class="dbms-main">
        <DatabaseSqlEditor :page="page" />
        <DatabaseWorkspacePanel :page="page" />
      </section>
    </div>

    <DatabaseWorkbenchDialogs :page="page" />
    <DatabaseRedisKeyDialog :page="page" />
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

.dbms-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
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
