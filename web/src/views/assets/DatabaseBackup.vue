<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetDatabaseList } from '../../api/asset'
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
  const data = await queryAssetDatabaseList({ pageNum: 1, pageSize: 500, status: '1', dbType: 'mysql' })
  databases.value = data.list || []
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
    ElMessage.warning('请填写计划名称并选择需要备份的业务库')
    return
  }
  saving.value = true
  try {
    await saveDBMSBackupPlan({ ...planForm })
    planDialogVisible.value = false
    ElMessage.success('备份计划已保存')
    await loadPlans()
  } finally {
    saving.value = false
  }
}

async function removePlan(row) {
  await ElMessageBox.confirm(`确认删除备份计划“${row.name}”吗？`, '删除确认', { type: 'warning' })
  await deleteDBMSBackupPlan(row.id)
  ElMessage.success('备份计划已删除')
  await loadPlans()
}

async function runPlan(row) {
  await ElMessageBox.confirm(`立即执行备份计划“${row.name}”？`, '手动执行', { type: 'warning' })
  await runDBMSBackupPlan(row.id)
  ElMessage.success('备份任务已启动')
  activeTab.value = 'records'
  await loadRecords()
}

async function runManual() {
  if (!manualForm.databaseId || !manualForm.schemaName) {
    ElMessage.warning('请选择数据库连接和需要备份的业务库')
    return
  }
  saving.value = true
  try {
    await runDBMSManualBackup({ ...manualForm })
    manualDialogVisible.value = false
    activeTab.value = 'records'
    ElMessage.success('手动备份已启动')
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
        <h1>备份管理</h1>
        <p>统一维护数据库备份计划、手动备份和可下载的备份记录。</p>
      </div>
      <div class="header-actions">
        <el-button @click="manualDialogVisible = true">手动备份</el-button>
        <el-button type="primary" @click="openPlan()">新增备份计划</el-button>
      </div>
    </header>

    <section class="tool-panel">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="备份计划" name="plans">
          <div class="toolbar">
            <el-input v-model="planQuery.keyword" clearable placeholder="搜索计划或数据库" @keyup.enter="loadPlans" />
            <el-select v-model="planQuery.status" clearable placeholder="全部状态">
              <el-option label="启用" value="1" />
              <el-option label="禁用" value="2" />
            </el-select>
            <el-button type="primary" @click="loadPlans">查询</el-button>
          </div>
          <el-table v-loading="planLoading" :data="plans" border>
            <el-table-column prop="name" label="计划名称" min-width="180" />
            <el-table-column prop="databaseName" label="数据库" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column prop="cronExpr" label="Cron" width="160" />
            <el-table-column prop="retentionDays" label="保留天数" width="100" />
            <el-table-column prop="lastRunAt" label="上次执行" width="180" />
            <el-table-column prop="nextRunAt" label="下次执行" width="180" />
            <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column>
            <el-table-column label="操作" width="220" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="runPlan(row)">立即备份</el-button>
                <el-button link type="primary" @click="openPlan(row)">编辑</el-button>
                <el-button link type="danger" @click="removePlan(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="planQuery.pageNum" :total="planTotal" layout="total, prev, pager, next" @current-change="loadPlans" /></div>
        </el-tab-pane>

        <el-tab-pane label="备份列表" name="records">
          <div class="toolbar">
            <el-select v-model="recordQuery.databaseId" clearable filterable placeholder="全部数据库">
              <el-option v-for="item in databases" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
            <el-select v-model="recordQuery.triggerType" clearable placeholder="触发方式">
              <el-option label="手动" value="manual" />
              <el-option label="计划" value="schedule" />
            </el-select>
            <el-select v-model="recordQuery.status" clearable placeholder="状态">
              <el-option label="执行中" value="running" />
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
            </el-select>
            <el-button type="primary" @click="loadRecords">查询</el-button>
          </div>
          <el-table v-loading="recordLoading" :data="records" border>
            <el-table-column prop="databaseName" label="数据库" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column label="触发方式" width="100"><template #default="{ row }">{{ row.triggerType === 'schedule' ? '计划' : '手动' }}</template></el-table-column>
            <el-table-column prop="planName" label="备份计划" min-width="150" />
            <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column prop="fileName" label="备份文件" min-width="220" show-overflow-tooltip />
            <el-table-column label="大小" width="100"><template #default="{ row }">{{ formatSize(row.fileSize) }}</template></el-table-column>
            <el-table-column prop="operator" label="执行人" width="110" />
            <el-table-column prop="message" label="结果" min-width="180" show-overflow-tooltip />
            <el-table-column prop="createTime" label="创建时间" width="180" />
            <el-table-column label="操作" width="100" fixed="right"><template #default="{ row }"><el-button link type="primary" :disabled="row.status !== 'success'" @click="downloadRecord(row)">下载</el-button></template></el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="recordQuery.pageNum" :total="recordTotal" layout="total, prev, pager, next" @current-change="loadRecords" /></div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="planDialogVisible" :title="planForm.id ? '编辑备份计划' : '新增备份计划'" width="min(900px, 92vw)">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="计划名称" required><el-input v-model="planForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="数据库连接" required><el-select v-model="planForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="业务库" required><el-select v-model="planForm.schemaName" :loading="schemaLoading" :disabled="!planForm.databaseId" filterable placeholder="请选择需要备份的业务库"><el-option v-for="schema in schemaOptions" :key="schema" :label="schema" :value="schema" /></el-select><div v-if="selectedPlanDatabase" class="schema-hint">已连接：{{ selectedPlanDatabase.host }}:{{ selectedPlanDatabase.port }}</div></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="保留天数"><el-input-number v-model="planForm.retentionDays" :min="1" :max="365" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="状态"><el-switch v-model="planForm.status" :active-value="1" :inactive-value="2" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="执行周期"><OpsCronEditor v-model="planForm.cronExpr" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="描述"><el-input v-model="planForm.description" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="planDialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitPlan">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="manualDialogVisible" title="手动备份" width="520px">
      <el-form label-width="90px">
        <el-form-item label="数据库连接" required><el-select v-model="manualForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="业务库" required><el-select v-model="manualForm.schemaName" :loading="schemaLoading" :disabled="!manualForm.databaseId" filterable placeholder="请选择需要备份的业务库"><el-option v-for="schema in schemaOptions" :key="schema" :label="schema" :value="schema" /></el-select><div v-if="selectedManualDatabase" class="schema-hint">将备份 {{ manualForm.schemaName || '所选业务库' }} 的结构、数据和对象定义。</div></el-form-item>
      </el-form>
      <template #footer><el-button @click="manualDialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="runManual">开始备份</el-button></template>
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
