<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
import {
  analyzeDBMSSQL,
  createDBMSBatchSQLTask,
  createDBMSImportTask,
  precheckDBMSImportTask,
  queryDBMSSchemaTree,
  queryDBMSTaskList
} from '../../api/dbms'

const activeTab = ref('import')
const databases = ref([])
const sourceTree = ref([])
const targetTree = ref([])
const precheck = ref(null)
const checking = ref(false)
const submitting = ref(false)
const historyLoading = ref(false)
const history = ref([])
const historyTotal = ref(0)

const importForm = reactive({
  sourceDatabaseId: undefined,
  sourceSchema: '',
  sourceTable: '',
  targetDatabaseId: undefined,
  targetSchema: '',
  targetTable: '',
  createIfMissing: true,
  truncateTarget: false
})

const batchForm = reactive({
  databaseId: undefined,
  schema: '',
  sqlText: '',
  fileName: '',
  executionMode: 'sequential'
})

const historyQuery = reactive({ pageNum: 1, pageSize: 10 })
const sourceSchemas = computed(() => sourceTree.value || [])
const targetSchemas = computed(() => targetTree.value || [])
const sourceTables = computed(() => sourceSchemas.value.find((item) => item.name === importForm.sourceSchema)?.tables || [])

async function loadDatabases() {
  const data = await queryAssetDatabaseList({ pageNum: 1, pageSize: 500, status: '1', dbType: 'mysql' })
  databases.value = data.list || []
}

async function loadSourceTree() {
  sourceTree.value = []
  importForm.sourceSchema = ''
  importForm.sourceTable = ''
  if (!importForm.sourceDatabaseId) return
  const data = await queryDBMSSchemaTree(importForm.sourceDatabaseId)
  sourceTree.value = data.schemas || []
  importForm.sourceSchema = data.defaultSchema || sourceTree.value[0]?.name || ''
}

async function loadTargetTree() {
  targetTree.value = []
  importForm.targetSchema = ''
  batchForm.schema = ''
  const databaseId = activeTab.value === 'batch' ? batchForm.databaseId : importForm.targetDatabaseId
  if (!databaseId) return
  const data = await queryDBMSSchemaTree(databaseId)
  targetTree.value = data.schemas || []
  const defaultSchema = data.defaultSchema || targetTree.value[0]?.name || ''
  if (activeTab.value === 'batch') batchForm.schema = defaultSchema
  else importForm.targetSchema = defaultSchema
}

function resetPrecheck() {
  precheck.value = null
}

async function runPrecheck() {
  checking.value = true
  try {
    precheck.value = await precheckDBMSImportTask({ ...importForm })
  } finally {
    checking.value = false
  }
}

async function createImport() {
  if (!precheck.value?.ready) {
    ElMessage.warning('请先完成并通过导入预检查')
    return
  }
  submitting.value = true
  try {
    await createDBMSImportTask({ ...importForm })
    ElMessage.success('数据导入任务已创建')
    activeTab.value = 'history'
    await loadHistory()
  } finally {
    submitting.value = false
  }
}

async function readSQLFile(event) {
  const file = event.target.files?.[0]
  if (!file) return
  if (file.size > 10 * 1024 * 1024) {
    ElMessage.warning('SQL 文件不能超过 10MB')
    event.target.value = ''
    return
  }
  batchForm.fileName = file.name
  batchForm.sqlText = await file.text()
}

async function createBatchTask() {
  if (!batchForm.databaseId || !batchForm.schema || !batchForm.sqlText.trim()) {
    ElMessage.warning('请选择数据库、Schema 并提供 SQL 内容')
    return
  }
  const analysis = await analyzeDBMSSQL({
    databaseId: batchForm.databaseId,
    schema: batchForm.schema,
    sqlText: batchForm.sqlText
  })
  if (analysis.accessMode === 'readonly' && analysis.writeOperation) {
    ElMessage.error('目标数据库为只读模式，不能执行批量写入')
    return
  }
  await ElMessageBox.confirm(
    `目标：${analysis.databaseName}/${analysis.schema}；共 ${analysis.statementCount} 条 SQL；风险等级：${analysis.riskLevel}`,
    '确认执行批量 SQL',
    { type: analysis.riskLevel === 'high' ? 'error' : 'warning', confirmButtonText: '确认创建任务' }
  )
  submitting.value = true
  try {
    await createDBMSBatchSQLTask({ ...batchForm, confirmed: true })
    ElMessage.success('批量 SQL 任务已创建')
    activeTab.value = 'history'
    await loadHistory()
  } finally {
    submitting.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const data = await queryDBMSTaskList({ ...historyQuery, taskType: 'import,batch_sql' })
    history.value = data.list || []
    historyTotal.value = data.total || 0
  } finally {
    historyLoading.value = false
  }
}

function taskTypeText(type) {
  return type === 'batch_sql' ? '批量 SQL' : '数据导入'
}

function statusType(status) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'info'
}

watch(() => importForm.sourceDatabaseId, loadSourceTree)
watch(() => importForm.targetDatabaseId, loadTargetTree)
watch(() => batchForm.databaseId, loadTargetTree)
watch(
  () => ({ ...importForm }),
  resetPrecheck,
  { deep: true }
)
watch(activeTab, (tab) => {
  if (tab === 'history') loadHistory()
})

onMounted(async () => {
  await loadDatabases()
  await loadHistory()
})
</script>

<template>
  <div class="database-tool-page">
    <header class="tool-header">
      <div>
        <h1>数据导入</h1>
        <p>通过预检查的数据迁移、批量 SQL 执行和统一任务历史。</p>
      </div>
    </header>

    <section class="tool-panel">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="数据导入" name="import">
          <el-form label-width="110px" class="operation-form">
            <div class="section-title">源数据</div>
            <el-form-item label="源数据库" required>
              <el-select v-model="importForm.sourceDatabaseId" filterable>
                <el-option v-for="item in databases" :key="item.id" :label="`${item.name} (${item.host}:${item.port})`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="源 Schema" required>
              <el-select v-model="importForm.sourceSchema" filterable>
                <el-option v-for="item in sourceSchemas" :key="item.name" :label="item.name" :value="item.name" />
              </el-select>
            </el-form-item>
            <el-form-item label="源表" required>
              <el-select v-model="importForm.sourceTable" filterable>
                <el-option v-for="item in sourceTables" :key="item.name" :label="item.name" :value="item.name" />
              </el-select>
            </el-form-item>

            <div class="section-title">目标位置</div>
            <el-form-item label="目标数据库" required>
              <el-select v-model="importForm.targetDatabaseId" filterable>
                <el-option v-for="item in databases" :key="item.id" :label="`${item.name} / ${item.accessMode === 'readonly' ? '只读' : '读写'}`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="目标 Schema" required>
              <el-select v-model="importForm.targetSchema" filterable>
                <el-option v-for="item in targetSchemas" :key="item.name" :label="item.name" :value="item.name" />
              </el-select>
            </el-form-item>
            <el-form-item label="目标表" required><el-input v-model="importForm.targetTable" placeholder="默认使用源表名称" /></el-form-item>
            <el-form-item label="导入策略">
              <el-checkbox v-model="importForm.createIfMissing">目标表不存在时自动建表</el-checkbox>
              <el-checkbox v-model="importForm.truncateTarget">导入前清空目标表</el-checkbox>
            </el-form-item>
            <el-form-item>
              <el-button :loading="checking" @click="runPrecheck">执行预检查</el-button>
              <el-button type="primary" :loading="submitting" :disabled="!precheck?.ready" @click="createImport">创建导入任务</el-button>
            </el-form-item>
          </el-form>
          <div v-if="precheck" class="precheck-panel" :class="{ danger: !precheck.ready }">
            <div><strong>{{ precheck.ready ? '预检查通过' : '预检查未通过' }}</strong><el-tag :type="precheck.ready ? 'success' : 'danger'">{{ precheck.estimatedRows }} 行</el-tag></div>
            <p>可映射字段 {{ precheck.commonColumns?.length || 0 }} 个，缺失字段 {{ precheck.missingColumns?.length || 0 }} 个。</p>
            <ul v-if="precheck.warnings?.length"><li v-for="item in precheck.warnings" :key="item">{{ item }}</li></ul>
          </div>
        </el-tab-pane>

        <el-tab-pane label="批量 SQL" name="batch">
          <el-form label-width="110px" class="operation-form">
            <el-form-item label="目标数据库" required>
              <el-select v-model="batchForm.databaseId" filterable>
                <el-option v-for="item in databases" :key="item.id" :label="`${item.name} / ${item.env || '-'} / ${item.accessMode === 'readonly' ? '只读' : '读写'}`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="Schema" required>
              <el-select v-model="batchForm.schema" filterable>
                <el-option v-for="item in targetSchemas" :key="item.name" :label="item.name" :value="item.name" />
              </el-select>
            </el-form-item>
            <el-form-item label="SQL 文件">
              <input type="file" accept=".sql,text/plain" @change="readSQLFile" />
              <span class="field-hint">支持 .sql 文本文件，最大 10MB</span>
            </el-form-item>
            <el-form-item label="SQL 内容" required>
              <el-input v-model="batchForm.sqlText" type="textarea" :rows="14" placeholder="粘贴 SQL，或从上方选择 SQL 文件" />
            </el-form-item>
            <el-form-item label="执行方式">
              <el-radio-group v-model="batchForm.executionMode">
                <el-radio value="sequential">顺序执行，失败即停止</el-radio>
                <el-radio value="transaction">事务执行，失败全部回滚</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="submitting" @click="createBatchTask">风险检查并执行</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="导入历史" name="history">
          <div class="history-toolbar"><el-button @click="loadHistory">刷新</el-button></div>
          <el-table v-loading="historyLoading" :data="history" border>
            <el-table-column label="类型" width="110"><template #default="{ row }">{{ taskTypeText(row.taskType) }}</template></el-table-column>
            <el-table-column prop="databaseName" label="目标数据库" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column prop="fileName" label="文件" min-width="160" />
            <el-table-column prop="executionMode" label="执行方式" width="110" />
            <el-table-column label="进度" width="150"><template #default="{ row }"><el-progress :percentage="row.progress || 0" :stroke-width="8" /></template></el-table-column>
            <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column prop="rowsAffected" label="影响行数" width="110" />
            <el-table-column prop="message" label="结果" min-width="260" show-overflow-tooltip />
            <el-table-column prop="createTime" label="创建时间" width="180" />
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="historyQuery.pageNum" :total="historyTotal" layout="total, prev, pager, next" @current-change="loadHistory" /></div>
        </el-tab-pane>
      </el-tabs>
    </section>
  </div>
</template>

<style scoped>
.database-tool-page { display: flex; flex-direction: column; gap: 16px; }
.tool-header, .tool-panel { border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; }
.tool-header { padding: 22px 24px; }
.tool-header h1 { margin: 0; color: #10213f; font-size: 26px; }
.tool-header p { margin: 8px 0 0; color: #71809a; }
.tool-panel { padding: 18px 22px; }
.operation-form { max-width: 880px; padding-top: 12px; }
.operation-form :deep(.el-select) { width: 100%; }
.section-title { margin: 8px 0 16px; padding-left: 10px; border-left: 3px solid #3772db; color: #172744; font-weight: 700; }
.field-hint { margin-left: 12px; color: #8290a8; font-size: 12px; }
.precheck-panel { margin: 12px 0 20px 110px; max-width: 740px; padding: 16px; border: 1px solid #a9ddbd; border-radius: 8px; background: #f1faf5; }
.precheck-panel.danger { border-color: #efb4b4; background: #fff5f5; }
.precheck-panel > div { display: flex; align-items: center; justify-content: space-between; }
.precheck-panel p, .precheck-panel ul { margin: 10px 0 0; }
.history-toolbar, .pager { display: flex; justify-content: flex-end; margin-bottom: 12px; }
.pager { margin: 16px 0 0; }
</style>
