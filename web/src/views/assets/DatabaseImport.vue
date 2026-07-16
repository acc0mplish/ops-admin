<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
import {
  analyzeDBMSSQL,
  createDBMSBackupImportTask,
  createDBMSBatchSQLTask,
  createDBMSImportTask,
  precheckDBMSImportTask,
  queryDBMSBackupRecordList,
  queryDBMSSchemaTree,
  queryDBMSTaskList
} from '../../api/dbms'

const activeTab = ref('import')
const databases = ref([])
const sourceTree = ref([])
const targetTree = ref([])
const backupTargetTree = ref([])
const backupRecords = ref([])
const backupRecordsLoading = ref(false)
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

const backupForm = reactive({
  sourceType: 'record',
  backupRecordId: undefined,
  databaseId: undefined,
  schemaName: '',
  fileName: '',
  fileContent: ''
})

const historyQuery = reactive({ pageNum: 1, pageSize: 10 })
const sourceSchemas = computed(() => sourceTree.value || [])
const targetSchemas = computed(() => targetTree.value || [])
const backupTargetSchemas = computed(() => backupTargetTree.value || [])
const selectedBackupRecord = computed(() => backupRecords.value.find((item) => item.id === backupForm.backupRecordId))
const selectedBackupDatabase = computed(() => databases.value.find((item) => item.id === backupForm.databaseId))
const relationalDatabases = computed(() => databases.value.filter((item) => ['mysql', 'postgresql'].includes(String(item.dbType || '').toLowerCase())))
const mysqlDatabases = computed(() => databases.value.filter((item) => String(item.dbType || '').toLowerCase() === 'mysql'))
const sourceTables = computed(() => sourceSchemas.value.find((item) => item.name === importForm.sourceSchema)?.tables || [])

async function loadDatabases() {
	const data = await queryAssetDatabaseList({ pageNum: 1, pageSize: 500, status: '1' })
	databases.value = data.list || []
}

function databaseLabel(item, withAccess = false) {
	const type = String(item.dbType || '').toUpperCase()
	const address = `${item.host || '-'}:${item.port || '-'}`
	return withAccess
		? `${item.name} / ${type} / ${item.accessMode === 'readonly' ? '只读' : '读写'}`
		: `${item.name} (${type} · ${address})`
}

async function loadBackupRecords() {
  backupRecordsLoading.value = true
  try {
    const data = await queryDBMSBackupRecordList({ pageNum: 1, pageSize: 200, status: 'success' })
    backupRecords.value = data.list || []
  } finally {
    backupRecordsLoading.value = false
  }
}

async function loadBackupTargetTree() {
  backupTargetTree.value = []
  backupForm.schemaName = ''
  if (!backupForm.databaseId) return
  const data = await queryDBMSSchemaTree(backupForm.databaseId)
  backupTargetTree.value = data.schemas || []
  backupForm.schemaName = data.defaultSchema || backupTargetTree.value[0]?.name || ''
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

async function readBackupFile(event) {
  const file = event.target.files?.[0]
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.sql')) {
    ElMessage.warning('备份导入仅支持 .sql 文件')
    event.target.value = ''
    return
  }
  if (file.size > 50 * 1024 * 1024) {
    ElMessage.warning('备份文件不能超过 50MB')
    event.target.value = ''
    return
  }
  backupForm.fileName = file.name
  backupForm.fileContent = await file.text()
}

async function createBackupImport() {
  const hasSource = backupForm.sourceType === 'record' ? backupForm.backupRecordId : backupForm.fileContent
  if (!hasSource || !backupForm.databaseId || !backupForm.schemaName) {
    ElMessage.warning('请选择备份来源、目标数据库和 Schema')
    return
  }
  if (selectedBackupDatabase.value?.accessMode === 'readonly') {
    ElMessage.error('目标数据库为只读模式，不能恢复备份')
    return
  }
  const sourceName = backupForm.sourceType === 'record'
    ? selectedBackupRecord.value?.fileName
    : backupForm.fileName
  await ElMessageBox.confirm(
    `即将把“${sourceName || 'SQL 备份'}”恢复到 ${selectedBackupDatabase.value?.name}/${backupForm.schemaName}。备份中的同名对象可能被覆盖，请确认目标无误。`,
    '确认恢复数据库备份',
    { type: 'warning', confirmButtonText: '确认并创建恢复任务', cancelButtonText: '取消' }
  )
  submitting.value = true
  try {
    await createDBMSBackupImportTask({
      backupRecordId: backupForm.sourceType === 'record' ? backupForm.backupRecordId : 0,
      databaseId: backupForm.databaseId,
      schemaName: backupForm.schemaName,
      fileName: backupForm.sourceType === 'file' ? backupForm.fileName : '',
      fileContent: backupForm.sourceType === 'file' ? backupForm.fileContent : '',
      confirmed: true
    })
    ElMessage.success('备份恢复任务已创建')
    activeTab.value = 'history'
    await loadHistory()
  } finally {
    submitting.value = false
  }
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
    const data = await queryDBMSTaskList({ ...historyQuery, taskType: 'import,backup_import,batch_sql' })
    history.value = data.list || []
    historyTotal.value = data.total || 0
  } finally {
    historyLoading.value = false
  }
}

function taskTypeText(type) {
  if (type === 'batch_sql') return '批量 SQL'
  if (type === 'backup_import') return '备份导入'
  return '数据导入'
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
watch(() => backupForm.databaseId, loadBackupTargetTree)
watch(() => backupForm.sourceType, () => {
  backupForm.backupRecordId = undefined
  backupForm.fileName = ''
  backupForm.fileContent = ''
})
watch(
  () => ({ ...importForm }),
  resetPrecheck,
  { deep: true }
)
watch(activeTab, (tab) => {
  if (tab === 'history') loadHistory()
})

onMounted(async () => {
  await Promise.all([loadDatabases(), loadBackupRecords(), loadHistory()])
})
</script>

<template>
  <div class="database-tool-page">
    <header class="tool-header">
      <div>
        <h1>数据导入</h1>
        <p>统一完成跨库数据迁移、数据库备份恢复、批量 SQL 执行和任务追踪。</p>
      </div>
    </header>

    <section class="tool-panel">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="数据导入" name="import">
          <el-form label-width="110px" class="operation-form">
            <div class="section-title">源数据</div>
            <el-form-item label="源数据库" required>
              <el-select v-model="importForm.sourceDatabaseId" filterable>
                <el-option v-for="item in relationalDatabases" :key="item.id" :label="databaseLabel(item)" :value="item.id" />
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

        <el-tab-pane label="备份导入" name="backup">
          <div class="backup-import-layout">
            <el-form label-width="112px" class="operation-form backup-import-form">
              <div class="section-title">选择备份来源</div>
              <el-form-item label="备份来源" required>
                <el-radio-group v-model="backupForm.sourceType">
                  <el-radio-button value="record">平台备份</el-radio-button>
                  <el-radio-button value="file">本地 SQL 文件</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="backupForm.sourceType === 'record'" label="备份记录" required>
                <el-select v-model="backupForm.backupRecordId" filterable :loading="backupRecordsLoading" placeholder="选择备份管理中的成功记录">
                  <el-option
                    v-for="item in backupRecords"
                    :key="item.id"
                    :label="`${item.databaseName} / ${item.schemaName} / ${item.fileName}`"
                    :value="item.id"
                  >
                    <div class="backup-option">
                      <strong>{{ item.fileName }}</strong>
                      <span>{{ item.databaseName }}/{{ item.schemaName }} · {{ item.createTime }}</span>
                    </div>
                  </el-option>
                </el-select>
                <el-button class="inline-refresh" @click="loadBackupRecords">刷新</el-button>
              </el-form-item>
              <el-form-item v-else label="备份文件" required>
                <label class="file-picker">
                  <input type="file" accept=".sql,text/plain" @change="readBackupFile" />
                  <span>{{ backupForm.fileName || '选择 SQL 备份文件' }}</span>
                </label>
                <span class="field-hint">支持平台内置逻辑备份及标准 MySQL SQL 文件，最大 50MB</span>
              </el-form-item>

              <div class="section-title target-title">选择恢复目标</div>
              <el-form-item label="目标数据库" required>
                <el-select v-model="backupForm.databaseId" filterable placeholder="选择可写的 MySQL 连接">
                  <el-option
                    v-for="item in mysqlDatabases"
                    :key="item.id"
                    :label="`${item.name} / ${item.env || '-'} / ${item.accessMode === 'readonly' ? '只读' : '读写'}`"
                    :value="item.id"
                    :disabled="item.accessMode === 'readonly'"
                  />
                </el-select>
              </el-form-item>
              <el-form-item label="目标 Schema" required>
                <el-select v-model="backupForm.schemaName" filterable placeholder="选择恢复到的业务库">
                  <el-option v-for="item in backupTargetSchemas" :key="item.name" :label="item.name" :value="item.name" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="submitting" @click="createBackupImport">恢复前确认</el-button>
              </el-form-item>
            </el-form>

            <aside class="restore-guide">
              <div class="restore-guide-title">恢复说明</div>
              <div class="restore-step"><span>1</span><div><strong>校验来源</strong><p>仅允许成功的平台备份或 `.sql` 文件。</p></div></div>
              <div class="restore-step"><span>2</span><div><strong>锁定目标</strong><p>恢复始终写入当前选择的连接和 Schema。</p></div></div>
              <div class="restore-step"><span>3</span><div><strong>异步执行</strong><p>关闭页面不会中断，可在导入历史查看进度。</p></div></div>
              <el-alert title="恢复可能覆盖同名表、视图和例程，生产库操作前请确认已有可用备份。" type="warning" :closable="false" show-icon />
            </aside>
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
.backup-import-layout { display: grid; grid-template-columns: minmax(620px, 880px) minmax(280px, 360px); gap: 28px; align-items: start; padding-top: 12px; }
.backup-import-form { max-width: none; padding: 0; }
.backup-import-form :deep(.el-form-item__content > .el-select) { flex: 1; width: 0; }
.target-title { margin-top: 26px; }
.inline-refresh { margin-left: 10px; }
.backup-option { display: flex; flex-direction: column; line-height: 1.35; }
.backup-option span { color: #8794aa; font-size: 12px; }
.file-picker { display: inline-flex; align-items: center; min-width: 220px; height: 34px; padding: 0 14px; border: 1px solid #cfd9e8; border-radius: 4px; color: #315d9b; background: #f7faff; cursor: pointer; }
.file-picker:hover { border-color: #5b83e6; background: #f2f6ff; }
.file-picker input { display: none; }
.restore-guide { padding: 20px; border: 1px solid #dce5f2; border-radius: 8px; background: #f8faff; }
.restore-guide-title { margin-bottom: 18px; color: #172744; font-size: 16px; font-weight: 700; }
.restore-step { display: flex; gap: 12px; margin-bottom: 18px; }
.restore-step > span { display: grid; flex: 0 0 26px; width: 26px; height: 26px; border-radius: 50%; place-items: center; color: #fff; background: #3972d8; font-size: 12px; font-weight: 700; }
.restore-step strong { color: #273854; }
.restore-step p { margin: 4px 0 0; color: #7a889f; font-size: 12px; line-height: 1.6; }
@media (max-width: 1100px) {
  .backup-import-layout { grid-template-columns: 1fr; }
  .restore-guide { max-width: 760px; margin-left: 112px; }
}
</style>
