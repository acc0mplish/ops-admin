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
    ElMessage.warning('Plan 이름을 입력하고 Backup할 Business DB를 선택하십시오.')
    return
  }
  saving.value = true
  try {
    await saveDBMSBackupPlan({ ...planForm })
    planDialogVisible.value = false
    ElMessage.success('Backup Plan을 저장했습니다.')
    await loadPlans()
  } finally {
    saving.value = false
  }
}

async function removePlan(row) {
  await ElMessageBox.confirm(`Backup Plan “${row.name}”을(를) 삭제하시겠습니까?`, '삭제 확인', { type: 'warning' })
  await deleteDBMSBackupPlan(row.id)
  ElMessage.success('Backup Plan을 삭제했습니다.')
  await loadPlans()
}

async function runPlan(row) {
  await ElMessageBox.confirm(`Backup Plan “${row.name}”을(를) 즉시 실행하시겠습니까?`, '수동 실행', { type: 'warning' })
  await runDBMSBackupPlan(row.id)
  ElMessage.success('Backup Task를 시작했습니다.')
  activeTab.value = 'records'
  await loadRecords()
}

async function runManual() {
  if (!manualForm.databaseId || !manualForm.schemaName) {
    ElMessage.warning('Database 연결과 Backup할 Business DB를 선택하십시오.')
    return
  }
  saving.value = true
  try {
    await runDBMSManualBackup({ ...manualForm })
    manualDialogVisible.value = false
    activeTab.value = 'records'
    ElMessage.success('수동 Backup을 시작했습니다.')
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
        <h1>Backup 관리</h1>
        <p>Database Backup Plan, 수동 Backup, 다운로드 가능한 Backup Record를 통합 관리합니다.</p>
      </div>
      <div class="header-actions">
        <el-button @click="manualDialogVisible = true">수동 Backup</el-button>
        <el-button type="primary" @click="openPlan()">Backup Plan 추가</el-button>
      </div>
    </header>

    <section class="tool-panel">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="Backup Plan" name="plans">
          <div class="toolbar">
            <el-input v-model="planQuery.keyword" clearable placeholder="Plan 또는 Database 검색" @keyup.enter="loadPlans" />
            <el-select v-model="planQuery.status" clearable placeholder="전체 상태">
              <el-option label="활성화" value="1" />
              <el-option label="비활성화" value="2" />
            </el-select>
            <el-button type="primary" @click="loadPlans">조회</el-button>
          </div>
          <el-table v-loading="planLoading" :data="plans" border>
            <el-table-column prop="name" label="Plan 이름" min-width="180" />
            <el-table-column prop="databaseName" label="Database" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column prop="cronExpr" label="Cron" width="160" />
            <el-table-column prop="retentionDays" label="보존 일수" width="100" />
            <el-table-column prop="lastRunAt" label="마지막 실행" width="180" />
            <el-table-column prop="nextRunAt" label="다음 실행" width="180" />
            <el-table-column label="상태" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '활성화' : '비활성화' }}</el-tag></template></el-table-column>
            <el-table-column label="작업" width="220" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="runPlan(row)">즉시 Backup</el-button>
                <el-button link type="primary" @click="openPlan(row)">수정</el-button>
                <el-button link type="danger" @click="removePlan(row)">삭제</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="planQuery.pageNum" :total="planTotal" layout="total, prev, pager, next" @current-change="loadPlans" /></div>
        </el-tab-pane>

        <el-tab-pane label="Backup 목록" name="records">
          <div class="toolbar">
            <el-select v-model="recordQuery.databaseId" clearable filterable placeholder="전체 Database">
              <el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" />
            </el-select>
            <el-select v-model="recordQuery.triggerType" clearable placeholder="Trigger 방식">
              <el-option label="수동" value="manual" />
              <el-option label="Plan" value="schedule" />
            </el-select>
            <el-select v-model="recordQuery.status" clearable placeholder="상태">
              <el-option label="실행 중" value="running" />
              <el-option label="성공" value="success" />
              <el-option label="실패" value="failed" />
            </el-select>
            <el-button type="primary" @click="loadRecords">조회</el-button>
          </div>
          <el-table v-loading="recordLoading" :data="records" border>
            <el-table-column prop="databaseName" label="Database" min-width="150" />
            <el-table-column prop="schemaName" label="Schema" width="130" />
            <el-table-column label="Trigger 방식" width="100"><template #default="{ row }">{{ row.triggerType === 'schedule' ? 'Plan' : '수동' }}</template></el-table-column>
            <el-table-column prop="planName" label="Backup Plan" min-width="150" />
            <el-table-column label="상태" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column prop="fileName" label="Backup 파일" min-width="220" show-overflow-tooltip />
            <el-table-column label="크기" width="100"><template #default="{ row }">{{ formatSize(row.fileSize) }}</template></el-table-column>
            <el-table-column prop="operator" label="실행자" width="110" />
            <el-table-column prop="message" label="결과" min-width="180" show-overflow-tooltip />
            <el-table-column prop="createTime" label="생성 시각" width="180" />
            <el-table-column label="작업" width="100" fixed="right"><template #default="{ row }"><el-button link type="primary" :disabled="row.status !== 'success'" @click="downloadRecord(row)">Download</el-button></template></el-table-column>
          </el-table>
          <div class="pager"><el-pagination v-model:current-page="recordQuery.pageNum" :total="recordTotal" layout="total, prev, pager, next" @current-change="loadRecords" /></div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="planDialogVisible" :title="planForm.id ? 'Backup Plan 수정' : 'Backup Plan 추가'" width="min(900px, 92vw)">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="Plan 이름" required><el-input v-model="planForm.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="Database 연결" required><el-select v-model="planForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item :label="selectedPlanDatabase?.dbType === 'postgresql' ? 'Business Schema' : 'Business DB'" required><el-select v-model="planForm.schemaName" :loading="schemaLoading" :disabled="!planForm.databaseId" filterable :placeholder="selectedPlanDatabase?.dbType === 'postgresql' ? 'Backup할 Business Schema를 선택하십시오:' : 'Backup할 Business DB를 선택하십시오:'"><el-option v-for="schema in schemaOptions" :key="schema" :label="schemaLabel(selectedPlanDatabase, schema)" :value="schema" /></el-select><div v-if="selectedPlanDatabase" class="schema-hint">연결됨: {{ selectedPlanDatabase.host }}:{{ selectedPlanDatabase.port }}</div></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="보존 일수"><el-input-number v-model="planForm.retentionDays" :min="1" :max="365" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="상태"><el-switch v-model="planForm.status" :active-value="1" :inactive-value="2" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="실행 주기"><OpsCronEditor v-model="planForm.cronExpr" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item label="설명"><el-input v-model="planForm.description" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="planDialogVisible = false">취소</el-button><el-button type="primary" :loading="saving" @click="submitPlan">저장</el-button></template>
    </el-dialog>

    <el-dialog v-model="manualDialogVisible" title="수동 Backup" width="520px">
      <el-form label-width="90px">
        <el-form-item label="Database 연결" required><el-select v-model="manualForm.databaseId" filterable><el-option v-for="item in databases" :key="item.id" :label="databaseLabel(item)" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="selectedManualDatabase?.dbType === 'postgresql' ? 'Business Schema' : 'Business DB'" required><el-select v-model="manualForm.schemaName" :loading="schemaLoading" :disabled="!manualForm.databaseId" filterable :placeholder="selectedManualDatabase?.dbType === 'postgresql' ? 'Backup할 Business Schema를 선택하십시오:' : 'Backup할 Business DB를 선택하십시오:'"><el-option v-for="schema in schemaOptions" :key="schema" :label="schemaLabel(selectedManualDatabase, schema)" :value="schema" /></el-select><div v-if="selectedManualDatabase" class="schema-hint">{{ schemaLabel(selectedManualDatabase, manualForm.schemaName || (selectedManualDatabase.dbType === 'postgresql' ? '선택한 Business Schema' : '선택한 Business DB')) }}의 Table 구조, Primary Key, Data를 Backup합니다.</div></el-form-item>
      </el-form>
      <template #footer><el-button @click="manualDialogVisible = false">취소</el-button><el-button type="primary" :loading="saving" @click="runManual">Backup 시작</el-button></template>
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
