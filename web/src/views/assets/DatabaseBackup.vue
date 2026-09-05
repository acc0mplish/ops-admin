<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
import { at } from '../../utils/asset-i18n'
import {
  deleteDBMSBackupPlan,
  downloadDBMSBackup,
  queryDBMSBackupPlanList,
  queryDBMSBackupRecordList,
  queryDBMSSchemaTree,
  runDBMSBackupPlan,
  runDBMSManualBackup,
  saveDBMSBackupPlan
} from '../../api/dbms'
import OpsCronEditor from '../ops/components/OpsCronEditor.vue'

const activeTab = ref('plans')
const databases = ref([])
const schemaOptions = ref([])
const schemaLoading = ref(false)
const planLoading = ref(false)
const recordLoading = ref(false)
const saving = ref(false)
const planDialogVisible = ref(false)
const manualDialogVisible = ref(false)
const plans = ref([])
const records = ref([])
const planTotal = ref(0)
const recordTotal = ref(0)
const timer = ref()

const planQuery = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })
const recordQuery = reactive({ pageNum: 1, pageSize: 10, databaseId: undefined, status: '', triggerType: '' })
const planForm = reactive({
  id: undefined,
  name: '',
  databaseId: undefined,
  schemaName: '',
  cronExpr: '0 0 2 * * *',
  retentionDays: 7,
  status: 1,
  description: ''
})
const manualForm = reactive({ databaseId: undefined, schemaName: '' })

const selectedPlanDatabase = computed(() => databases.value.find((item) => item.id === planForm.databaseId))
const selectedManualDatabase = computed(() => databases.value.find((item) => item.id === manualForm.databaseId))

async function loadDatabases() {
  const data = await queryAssetDatabaseList({ pageNum: 1, pageSize: 500, status: '1' })
  databases.value = (data.list || []).filter((item) => ['mysql', 'postgresql'].includes(String(item.dbType || '').toLowerCase()))
}

function databaseLabel(item) {
  return `${item.name} (${String(item.dbType || 'mysql').toUpperCase()})`
}

function schemaLabel(database, schema) {
  if (String(database?.dbType || '').toLowerCase() === 'postgresql') {
    return `${database.dbName || database.name} / ${schema}`
  }
  return schema
}

async function loadSchemaOptions(databaseId) {
  schemaOptions.value = []
  if (!databaseId) return
  schemaLoading.value = true
  try {
    const data = await queryDBMSSchemaTree(databaseId)
    schemaOptions.value = (data.schemas || []).map((item) => item.name)
  } finally {
    schemaLoading.value = false
  }
}

async function loadPlans() {
  planLoading.value = true
  try {
    const data = await queryDBMSBackupPlanList(planQuery)
    plans.value = data.list || []
    planTotal.value = data.total || 0
  } finally {
    planLoading.value = false
  }
}

async function loadRecords() {
  recordLoading.value = true
  try {
    const data = await queryDBMSBackupRecordList(recordQuery)
    records.value = data.list || []
    recordTotal.value = data.total || 0
  } finally {
    recordLoading.value = false
  }
}

function resetPlanForm() {
  Object.assign(planForm, {
    id: undefined,
    name: '',
    databaseId: undefined,
    schemaName: '',
    cronExpr: '0 0 2 * * *',
    retentionDays: 7,
    status: 1,
    description: ''
  })
}

function openPlan(row) {
  resetPlanForm()
  if (row) Object.assign(planForm, row)
  planDialogVisible.value = true
}

async function submitPlan() {
  if (!planForm.name.trim() || !planForm.databaseId || !planForm.schemaName) {
    ElMessage.warning(at('selectPlanNameAndDb'))
    return
  }
  saving.value = true
  try {
    await saveDBMSBackupPlan({ ...planForm })
    planDialogVisible.value = false
    ElMessage.success(at('planSaved'))
    await loadPlans()
  } finally {
    saving.value = false
  }
}

async function removePlan(row) {
  await ElMessageBox.confirm(at('deletePlanConfirm', { name: row.name }), at('confirmDeleteTitle'), { type: 'warning' })
  await deleteDBMSBackupPlan(row.id)
  ElMessage.success(at('planDeleted'))
  await loadPlans()
}

async function runPlan(row) {
  await ElMessageBox.confirm(at('runPlanConfirm', { name: row.name }), at('manualRunTitle'), { type: 'warning' })
  await runDBMSBackupPlan(row.id)
  ElMessage.success(at('backupTaskStarted'))
  activeTab.value = 'records'
  await loadRecords()
}

async function runManual() {
  if (!manualForm.databaseId || !manualForm.schemaName) {
    ElMessage.warning(at('selectDbAndSchema'))
    return
  }
  saving.value = true
  try {
    await runDBMSManualBackup({ ...manualForm })
    manualDialogVisible.value = false
    activeTab.value = 'records'
    ElMessage.success(at('manualBackupStarted'))
    await loadRecords()
  } finally {
    saving.value = false
  }
}

async function downloadRecord(row) {
  const response = await downloadDBMSBackup({ id: row.id })
  const blob = new Blob([response.data], { type: 'application/sql' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = row.fileName || `database-backup-${row.id}.sql`
  link.click()
  URL.revokeObjectURL(link.href)
}

function statusType(status) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'info'
}

function formatSize(size) {
  const value = Number(size || 0)
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

onMounted(async () => {
  await Promise.all([loadDatabases(), loadPlans(), loadRecords()])
  timer.value = window.setInterval(() => {
    if (activeTab.value === 'records' && records.value.some((item) => item.status === 'running')) loadRecords()
  }, 3000)
})

watch(() => planForm.databaseId, async (databaseId, previousId) => {
  if (databaseId !== previousId) planForm.schemaName = ''
  await loadSchemaOptions(databaseId)
})

watch(() => manualForm.databaseId, async (databaseId, previousId) => {
  if (databaseId !== previousId) manualForm.schemaName = ''
  await loadSchemaOptions(databaseId)
})

onBeforeUnmount(() => {
  if (timer.value) window.clearInterval(timer.value)
})
</script>

<template>
  <div class="database-tool-page">
    <header class="tool-header">
      <div>
        <h1>{{ at('backupManageTitle') }}</h1>
        <p>{{ at('backupManageDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-button @click="manualDialogVisible = true">{{ at('manualBackup') }}</el-button>
        <el-button type="primary" @click="openPlan()">{{ at('addPlanButton') }}</el-button>
      </div>
    </header>

    <section class="tool-panel">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="Backup Plan" name="plans">
          <div class="toolbar">
            <el-input v-model="planQuery.keyword" clearable :placeholder="at('planSearchPlaceholder')" @keyup.enter="loadPlans" />
            <el-select v-model="planQuery.status" clearable :placeholder="at('allStatusPlaceholder')">
              <el-option :label="at('enabled')" value="1" />
              <el-option :label="at('disabled')" value="2" />
            </el-select>
            <el-button type="primary" @click="loadPlans">{{ at('queryButton') }}</el-button>
          </div>
          <el-table v-loading="planLoading" :data="plans" border>
            <el-table-column prop="name" :label="at('planNameColumn')" min-width="180" />
            <el-table-column prop="databaseName" label="Database" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column prop="cronExpr" label="Cron" width="160" />
            <el-table-column prop="retentionDays" :label="at('retentionDaysColumn')" width="100" />
            <el-table-column prop="lastRunAt" :label="at('lastRunColumn')" width="180" />
            <el-table-column prop="nextRunAt" :label="at('nextRunColumn')" width="180" />
            <el-table-column :label="at('status')" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? at('enabled') : at('disabled') }}</el-tag></template></el-table-column>
            <el-table-column :label="at('actions')" width="220" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="runPlan(row)">{{ at('backupNow') }}</el-button>
                <el-button link type="primary" @click="openPlan(row)">{{ at('edit') }}</el-button>
                <el-button link type="danger" @click="removePlan(row)">{{ at('delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="planQuery.pageNum" :total="planTotal" layout="total, prev, pager, next" @current-change="loadPlans" /></div>
        </el-tab-pane>

        <el-tab-pane :label="at('recordsTabLabel')" name="records">
          <div class="toolbar">
            <el-select v-model="recordQuery.databaseId" clearable filterable :placeholder="at('allDatabasesPlaceholder')">
              <el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" />
            </el-select>
            <el-select v-model="recordQuery.triggerType" clearable :placeholder="at('triggerTypeLabel')">
              <el-option :label="at('manualOption')" value="manual" />
              <el-option label="Plan" value="schedule" />
            </el-select>
            <el-select v-model="recordQuery.status" clearable :placeholder="at('statusSelect')">
              <el-option :label="at('statusRunning')" value="running" />
              <el-option :label="at('statusSuccess')" value="success" />
              <el-option :label="at('statusFailed')" value="failed" />
            </el-select>
            <el-button type="primary" @click="loadRecords">{{ at('queryButton') }}</el-button>
          </div>
          <el-table v-loading="recordLoading" :data="records" border>
            <el-table-column prop="databaseName" label="Database" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column :label="at('triggerTypeLabel')" width="100"><template #default="{ row }">{{ row.triggerType === 'schedule' ? 'Plan' : at('manualOption') }}</template></el-table-column>
            <el-table-column prop="planName" label="Backup Plan" min-width="150" />
            <el-table-column :label="at('status')" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column prop="fileName" :label="at('backupFileLabel')" min-width="220" show-overflow-tooltip />
            <el-table-column :label="at('sizeColumn')" width="100"><template #default="{ row }">{{ formatSize(row.fileSize) }}</template></el-table-column>
            <el-table-column prop="operator" :label="at('operatorColumn')" width="110" />
            <el-table-column prop="message" :label="at('resultColumn')" min-width="180" show-overflow-tooltip />
            <el-table-column prop="createTime" :label="at('createdAtColumn')" width="180" />
            <el-table-column :label="at('actions')" width="100" fixed="right"><template #default="{ row }"><el-button link type="primary" :disabled="row.status !== 'success'" @click="downloadRecord(row)">Download</el-button></template></el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="recordQuery.pageNum" :total="recordTotal" layout="total, prev, pager, next" @current-change="loadRecords" /></div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="planDialogVisible" :title="planForm.id ? at('editPlanTitle') : at('addPlanButton')" width="min(900px, 92vw)">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item :label="at('planNameColumn')" required><el-input v-model="planForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="at('dbConnectionLabel')" required><el-select v-model="planForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="selectedPlanDatabase?.dbType === 'postgresql' ? 'Business Schema' : 'Business DB'" required><el-select v-model="planForm.schemaName" :loading="schemaLoading" :disabled="!planForm.databaseId" filterable :placeholder="selectedPlanDatabase?.dbType === 'postgresql' ? at('selectBusinessSchemaPlaceholder') : at('selectBusinessDbPlaceholder')"><el-option v-for="schema in schemaOptions" :key="schema" :label="schemaLabel(selectedPlanDatabase, schema)" :value="schema" /></el-select><div v-if="selectedPlanDatabase" class="schema-hint">{{ at('dbConnected') }}: {{ selectedPlanDatabase.host }}:{{ selectedPlanDatabase.port }}</div></el-form-item></el-col>
          <el-col :span="6"><el-form-item :label="at('retentionDaysColumn')"><el-input-number v-model="planForm.retentionDays" :min="1" :max="365" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item :label="at('status')"><el-switch v-model="planForm.status" :active-value="1" :inactive-value="2" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item :label="at('scheduleLabel')"><OpsCronEditor v-model="planForm.cronExpr" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item :label="at('description')"><el-input v-model="planForm.description" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="planDialogVisible = false">{{ at('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submitPlan">{{ at('save') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="manualDialogVisible" :title="at('manualBackup')" width="520px">
      <el-form label-width="90px">
        <el-form-item :label="at('dbConnectionLabel')" required><el-select v-model="manualForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="selectedManualDatabase?.dbType === 'postgresql' ? 'Business Schema' : 'Business DB'" required><el-select v-model="manualForm.schemaName" :loading="schemaLoading" :disabled="!manualForm.databaseId" filterable :placeholder="selectedManualDatabase?.dbType === 'postgresql' ? at('selectBusinessSchemaPlaceholder') : at('selectBusinessDbPlaceholder')"><el-option v-for="schema in schemaOptions" :key="schema" :label="schemaLabel(selectedManualDatabase, schema)" :value="schema" /></el-select><div v-if="selectedManualDatabase" class="schema-hint">{{ at('schemaBackupDesc', { target: schemaLabel(selectedManualDatabase, manualForm.schemaName || (selectedManualDatabase.dbType === 'postgresql' ? at('selectedBusinessSchema') : at('selectedBusinessDb'))) }) }}</div></el-form-item>
      </el-form>
      <template #footer><el-button @click="manualDialogVisible = false">{{ at('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="runManual">{{ at('startBackup') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.database-tool-page { display: flex; flex-direction: column; gap: 16px; }
.tool-header, .tool-panel { border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; }
.tool-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 22px 24px; }
.tool-header h1 { margin: 0; color: #10213f; font-size: 26px; }
.tool-header p { margin: 8px 0 0; color: #71809a; }
.header-actions, .toolbar { display: flex; gap: 10px; flex-wrap: wrap; }
.tool-panel { padding: 18px 22px; }
.toolbar { margin-bottom: 16px; }
.toolbar .el-input { width: 260px; }
.toolbar .el-select { width: 170px; }
.pager { display: flex; justify-content: flex-end; margin-top: 16px; }
:deep(.el-dialog .el-select) { width: 100%; }
.schema-hint { margin-top: 6px; color: #8491a7; font-size: 12px; line-height: 1.5; }
</style>
