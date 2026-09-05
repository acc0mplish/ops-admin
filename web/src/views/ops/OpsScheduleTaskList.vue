<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import {
  addOpsScheduleTask,
  batchDeleteOpsScheduleTask,
  deleteOpsScheduleTask,
  opsScheduleTaskInfo,
  queryNotifyRuleOptions,
  queryOpsScheduleTaskList,
  queryOpsScheduleTemplateList,
  opsScheduleTemplateInfo,
  queryOpsScriptOptions,
  runOpsScheduleTask,
  updateOpsScheduleTask,
  updateOpsScheduleTaskStatus
} from '../../api/ops'
import OpsTargetSelector from './components/OpsTargetSelector.vue'
import OpsCronEditor from './components/OpsCronEditor.vue'
import { ot } from '../../utils/ops-i18n'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const selectedRows = ref([])
const scriptOptions = ref([])
const hostOptions = ref([])
const groupOptions = ref([])
const templateOptions = ref([])
const notifyRuleOptions = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  taskType: '',
  status: ''
})

const form = reactive({
  id: undefined,
  name: '',
  taskType: 'script',
  templateId: undefined,
  scriptId: undefined,
  variables: {},
  hostIds: [],
  groupId: undefined,
  concurrency: 5,
  httpMethod: 'GET',
  url: '',
  headersJson: '{\n  "User-Agent": "OpsAdmin-Scheduler"\n}',
  body: '',
  expectedStatus: 200,
  timeoutSeconds: 10,
  cronExpr: '0 */5 * * * *',
  description: '',
  status: 1,
  notifyEnabled: false,
  notifyRuleId: undefined,
  notifyOnFailureOnly: false
})

const selectedScriptTimeout = computed(() => {
  const current = scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId))
  return current?.timeoutSeconds || 300
})

const selectedScriptVariables = computed(() => {
  const current = scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId))
  return current?.variables || []
})

function syncScriptVariables(values = {}) {
  const next = {}
  selectedScriptVariables.value.forEach((variable) => {
    next[variable.name] = values[variable.name] ?? (variable.secret ? '' : (variable.defaultValue || ''))
  })
  form.variables = next
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    taskType: 'script',
    templateId: undefined,
    scriptId: undefined,
    variables: {},
    hostIds: [],
    groupId: undefined,
    concurrency: 5,
    httpMethod: 'GET',
    url: '',
    headersJson: '{\n  "User-Agent": "OpsAdmin-Scheduler"\n}',
    body: '',
    expectedStatus: 200,
    timeoutSeconds: 10,
    cronExpr: '0 */5 * * * *',
    description: '',
    status: 1,
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyOnFailureOnly: false
  })
}

watch(
  () => form.taskType,
  (value) => {
    if (value === 'script') {
      form.httpMethod = 'GET'
      form.url = ''
      form.body = ''
      form.expectedStatus = 200
      form.timeoutSeconds = 10
    } else {
      form.scriptId = undefined
      form.hostIds = []
      form.groupId = undefined
      form.concurrency = 1
    }
  }
)

watch(() => form.scriptId, () => {
  if (form.taskType === 'script') syncScriptVariables(form.variables)
})

async function loadBaseOptions() {
  const [scripts, hosts, groups, templates, notifyRules] = await Promise.all([
    queryOpsScriptOptions(),
    queryAssetHostList({ pageNum: 1, pageSize: 1000 }),
    queryAssetHostGroupList(),
    queryOpsScheduleTemplateList({ pageNum: 1, pageSize: 1000, status: '1' }),
    queryNotifyRuleOptions({ scope: 'schedule' })
  ])
  scriptOptions.value = scripts || []
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
  templateOptions.value = templates.list || []
  notifyRuleOptions.value = notifyRules || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsScheduleTaskList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, {
    pageNum: 1,
    pageSize: 10,
    keyword: '',
    taskType: '',
    status: ''
  })
  loadData()
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

function openCreateFromTemplate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await opsScheduleTaskInfo(row.id)
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    taskType: data.taskType || 'script',
    templateId: data.templateId || undefined,
    scriptId: data.scriptId || undefined,
    variables: data.variables || {},
    hostIds: data.hostIds || [],
    groupId: data.groupIds?.[0] || undefined,
    concurrency: data.concurrency || 5,
    httpMethod: data.httpMethod || 'GET',
    url: data.url || '',
    headersJson: data.headersJson || '{}',
    body: data.body || '',
    expectedStatus: data.expectedStatus || 200,
    timeoutSeconds: data.timeoutSeconds || 10,
    cronExpr: data.cronExpr || '0 */5 * * * *',
    description: data.description || '',
    status: data.status || 1,
    notifyEnabled: !!data.notifyEnabled,
    notifyRuleId: data.notifyRuleId || undefined,
    notifyOnFailureOnly: !!data.notifyOnFailureOnly
  })
  syncScriptVariables(data.variables || {})
  dialogVisible.value = true
}

async function handleCopy(row) {
  const data = await opsScheduleTaskInfo(row.id)
  isEdit.value = false
  Object.assign(form, {
    id: undefined,
    name: `${data.name || row.name}-copy`,
    taskType: data.taskType || 'script',
    templateId: data.templateId || undefined,
    scriptId: data.scriptId || undefined,
    variables: data.variables || {},
    hostIds: data.hostIds || [],
    groupId: data.groupIds?.[0] || undefined,
    concurrency: data.concurrency || 5,
    httpMethod: data.httpMethod || 'GET',
    url: data.url || '',
    headersJson: data.headersJson || '{}',
    body: data.body || '',
    expectedStatus: data.expectedStatus || 200,
    timeoutSeconds: data.timeoutSeconds || 10,
    cronExpr: data.cronExpr || '0 */5 * * * *',
    description: data.description || '',
    status: data.status || 1,
    notifyEnabled: !!data.notifyEnabled,
    notifyRuleId: data.notifyRuleId || undefined,
    notifyOnFailureOnly: !!data.notifyOnFailureOnly
  })
  syncScriptVariables(data.variables || {})
  dialogVisible.value = true
}

async function applyTemplate(templateId) {
  const selected = templateOptions.value.find((item) => Number(item.id) === Number(templateId))
  if (!selected) return
  const template = await opsScheduleTemplateInfo(templateId)
  form.taskType = template.taskType || 'script'
  form.scriptId = template.scriptId || undefined
  syncScriptVariables(template.variables || {})
  form.httpMethod = template.httpMethod || 'GET'
  form.url = template.url || ''
  form.headersJson = template.headersJson || '{}'
  form.body = template.body || ''
  form.expectedStatus = template.expectedStatus || 200
  form.timeoutSeconds = template.timeoutSeconds || 10
  if (template.cronExpr) {
    form.cronExpr = template.cronExpr
  }
  if (template.description && !form.description) {
    form.description = template.description
  }
}

function buildPayload() {
  return {
    id: form.id,
    name: form.name,
    taskType: form.taskType,
    templateId: form.templateId,
    scriptId: form.scriptId,
    variables: form.variables,
    hostIds: form.hostIds,
    groupIds: form.groupId ? [form.groupId] : [],
    concurrency: form.concurrency,
    httpMethod: form.httpMethod,
    url: form.url,
    headersJson: form.headersJson,
    body: form.body,
    expectedStatus: form.expectedStatus,
    timeoutSeconds: form.timeoutSeconds,
    cronExpr: form.cronExpr,
    description: form.description,
    status: form.status,
    notifyEnabled: form.notifyEnabled,
    notifyRuleId: form.notifyEnabled ? form.notifyRuleId : undefined,
    notifyOnFailureOnly: form.notifyEnabled && form.notifyOnFailureOnly
  }
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning(ot('taskNameRequired'))
    return
  }
  if (form.notifyEnabled && !form.notifyRuleId) {
    ElMessage.warning(ot('selectNotificationRuleRequired'))
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await updateOpsScheduleTask(buildPayload())
      ElMessage.success(ot('taskUpdated'))
    } else {
      await addOpsScheduleTask(buildPayload())
      ElMessage.success(ot('taskCreated'))
    }
    dialogVisible.value = false
    await loadData()
    await loadBaseOptions()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(ot('deleteTaskConfirm', { name: row.name }), ot('noticeTitle'), { type: 'warning' })
  await deleteOpsScheduleTask(row.id)
  ElMessage.success(ot('deleteSuccess'))
  await loadData()
}

async function handleRun(row) {
  await ElMessageBox.confirm(ot('runTaskConfirm', { name: row.name }), ot('noticeTitle'), { type: 'warning' })
  await runOpsScheduleTask(row.id)
  ElMessage.success(ot('taskTriggered'))
  await loadData()
}

async function batchDelete() {
  if (!selectedRows.value.length) {
    ElMessage.warning(ot('selectTaskFirst'))
    return
  }
  await ElMessageBox.confirm(ot('batchDeleteTaskConfirm', { count: selectedRows.value.length }), ot('noticeTitle'), { type: 'warning' })
  await batchDeleteOpsScheduleTask(selectedRows.value.map((item) => item.id))
  ElMessage.success(ot('batchDeleteSuccess'))
  await loadData()
}

async function batchUpdateStatus(status) {
  if (!selectedRows.value.length) {
    ElMessage.warning(ot('selectTaskFirst'))
    return
  }
  await updateOpsScheduleTaskStatus({
    ids: selectedRows.value.map((item) => item.id),
    status
  })
  ElMessage.success(status === 1 ? ot('batchEnableSuccess') : ot('batchDisableSuccess'))
  await loadData()
}

async function toggleRowStatus(row) {
  await updateOpsScheduleTaskStatus({
    ids: [row.id],
    status: row.status === 1 ? 2 : 1
  })
  ElMessage.success(row.status === 1 ? ot('taskDisabledMessage') : ot('taskEnabledMessage'))
  await loadData()
}

function handleSelectionChange(value) {
  selectedRows.value = value
}

function statusLabel(value) {
  return value === 1 ? ot('enabled') : ot('disabled')
}

function taskTypeLabel(value) {
  return value === 'http' ? 'HTTP Probe' : 'Script Task'
}

onMounted(async () => {
  await loadBaseOptions()
  await loadData()
})
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ ot('taskList') }}</h2>
        <p class="page-desc">{{ ot('taskListDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-button @click="openCreateFromTemplate">{{ ot('newFromTemplate') }}</el-button>
        <el-button type="primary" @click="openCreate">{{ ot('newTask') }}</el-button>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchTask')" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable :placeholder="ot('taskTypeLabel')" style="width: 140px">
          <el-option label="Script Task" value="script" />
          <el-option label="HTTP Probe" value="http" />
        </el-select>
        <el-select v-model="query.status" clearable :placeholder="ot('status')" style="width: 120px">
          <el-option :label="ot('enabled')" value="1" />
          <el-option :label="ot('disabled')" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
      <div class="toolbar-right">
        <el-button @click="batchUpdateStatus(1)">{{ ot('batchEnable') }}</el-button>
        <el-button @click="batchUpdateStatus(2)">{{ ot('batchDisable') }}</el-button>
        <el-button type="danger" plain @click="batchDelete">{{ ot('batchDelete') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="name" :label="ot('taskName')" min-width="180" />
      <el-table-column :label="ot('taskTypeLabel')" width="120">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column prop="cronExpr" label="Cron Expression" min-width="160" />
      <el-table-column prop="scriptName" :label="ot('scriptOrAddress')" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">{{ row.taskType === 'http' ? row.url : row.scriptName }}</template>
      </el-table-column>
      <el-table-column :label="ot('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="light">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="lastStatus" :label="ot('lastResult')" width="110" />
      <el-table-column prop="lastSummary" :label="ot('lastSummaryLabel')" min-width="220" show-overflow-tooltip />
      <el-table-column prop="lastRunAt" :label="ot('lastRun')" width="180" />
      <el-table-column prop="nextRunAt" :label="ot('nextRun')" width="180" />
      <el-table-column :label="ot('actions')" width="350" fixed="right">
        <template #default="{ row }">
          <el-button link type="success" @click="handleRun(row)">{{ ot('runNow') }}</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ ot('edit') }}</el-button>
          <el-button link @click="handleCopy(row)">{{ ot('duplicate') }}</el-button>
          <el-button link :class="row.status === 1 ? 'schedule-action-disable' : 'schedule-action-enable'" @click="toggleRowStatus(row)">
            {{ row.status === 1 ? ot('disabled') : ot('enabled') }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ ot('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, sizes, prev, pager, next"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? ot('editTask') : ot('newTask')" width="min(1080px, 92vw)">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="ot('taskName')" required>
              <el-input v-model="form.name" :placeholder="ot('taskNameExample')" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Task Template">
              <el-select v-model="form.templateId" clearable filterable :placeholder="ot('templateLoadHint')" style="width: 100%" @change="applyTemplate">
                <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="ot('taskTypeLabel')">
              <el-radio-group v-model="form.taskType">
                <el-radio value="script">Script Task</el-radio>
                <el-radio value="http">HTTP Probe</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="ot('status')">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">{{ ot('enabled') }}</el-radio>
                <el-radio :value="2">{{ ot('disabled') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="Cron Expression" required>
              <OpsCronEditor v-model="form.cronExpr" />
            </el-form-item>
          </el-col>

          <el-col :span="8">
            <el-form-item label="Notification">
              <el-switch v-model="form.notifyEnabled" />
            </el-form-item>
          </el-col>
          <el-col v-if="form.notifyEnabled" :span="16">
            <el-form-item label="Notification Rule" required>
              <el-select v-model="form.notifyRuleId" filterable :placeholder="ot('selectNotificationRulePlaceholder')" style="width: 100%">
                <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="form.notifyEnabled" :span="24">
            <el-form-item :label="ot('notificationPolicy')">
              <el-switch v-model="form.notifyOnFailureOnly" :active-text="ot('notifyOnFailureOnlyLabel')" :inactive-text="ot('notifyEveryRunLabel')" />
              <span class="form-tip">{{ ot('notifyOnFailureOnlyHint') }}</span>
            </el-form-item>
          </el-col>

          <template v-if="form.taskType === 'script'">
            <el-col :span="12">
              <el-form-item label="Script" required>
                <el-select v-model="form.scriptId" filterable :placeholder="ot('selectLibraryScript')" style="width: 100%">
                  <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :label="ot('concurrencyCount')">
                <el-input-number v-model="form.concurrency" :min="1" :max="10" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :label="ot('timeoutSecondsLabel')">
                <el-input :model-value="selectedScriptTimeout" disabled>
                  <template #append>s</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <div class="variable-panel">
                <div class="variable-panel__header">
                  <div>
                    <div class="variable-panel__title">{{ ot('executionVariables') }}</div>
                    <div class="variable-panel__hint">{{ ot('taskVariableHintStart') }}<code>VARIABLE_{{ ot('variableNameWord') }}</code>{{ ot('taskVariableHintEnd') }}</div>
                  </div>
                </div>
                <div v-if="!selectedScriptVariables.length" class="variable-panel__empty">{{ ot('scriptNoVariablesNeeded') }}</div>
                <div v-else class="variable-grid">
                  <div v-for="variable in selectedScriptVariables" :key="variable.name" class="variable-field">
                    <div class="variable-field__label"><code>VARIABLE_{{ variable.name }}</code><el-tag v-if="variable.required" size="small" type="danger" effect="plain">{{ ot('required') }}</el-tag></div>
                    <el-input v-model="form.variables[variable.name]" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :placeholder="variable.secret ? ot('variableKeepExistingHint') : (variable.defaultValue || ot('enterVariableValue'))" />
                    <div v-if="variable.description" class="variable-field__desc">{{ variable.description }}</div>
                  </div>
                </div>
              </div>
            </el-col>
            <el-col :span="24">
              <OpsTargetSelector
                :host-options="hostOptions"
                :group-options="groupOptions"
                :host-ids="form.hostIds"
                :group-id="form.groupId"
                @update:host-ids="form.hostIds = $event"
                @update:group-id="form.groupId = $event"
              />
            </el-col>
          </template>

          <template v-else>
            <el-col :span="6">
              <el-form-item label="Request Method">
                <el-select v-model="form.httpMethod" style="width: 100%">
                  <el-option label="GET" value="GET" />
                  <el-option label="POST" value="POST" />
                  <el-option label="PUT" value="PUT" />
                  <el-option label="PATCH" value="PATCH" />
                  <el-option label="DELETE" value="DELETE" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Probe URL" required>
                <el-input v-model="form.url" :placeholder="ot('probeUrlExample')" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :label="ot('timeoutSecondsLabel')">
                <el-input-number v-model="form.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :label="ot('expectedStatusCode')">
                <el-input-number v-model="form.expectedStatus" :min="100" :max="599" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="18">
              <el-form-item label="Request Header JSON">
                <el-input v-model="form.headersJson" type="textarea" :rows="4" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="Request Body">
                <el-input v-model="form.body" type="textarea" :rows="5" />
              </el-form-item>
            </el-col>
          </template>

          <el-col :span="24">
            <el-form-item :label="ot('description')">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ ot('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ ot('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.header-actions { display: flex; gap: 12px; }
.page-title { margin: 0 0 8px; font-size: 22px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
.toolbar-left, .toolbar-right { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.form-tip { margin-left: 12px; color: #8694ad; font-size: 13px; }
.variable-panel { padding: 16px; border: 1px solid #d9e6ff; border-radius: 10px; background: linear-gradient(135deg, #f8fbff, #fff); }
.variable-panel__header { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.variable-panel__title { color: #172744; font-size: 15px; font-weight: 700; }
.variable-panel__hint, .variable-field__desc { margin-top: 4px; color: #7282a0; font-size: 12px; }
.variable-panel__hint code, .variable-field__label code { color: #3869d9; }
.variable-panel__empty { padding: 12px; color: #8190aa; border: 1px dashed #cbdcff; border-radius: 7px; }
.variable-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.variable-field { min-width: 0; }
.variable-field__label { display: flex; align-items: center; gap: 8px; margin-bottom: 7px; font-size: 13px; font-weight: 600; }
.schedule-action-disable { color: #c87506 !important; font-weight: 600; }
.schedule-action-disable:hover { color: #9a5a00 !important; }
.schedule-action-enable { color: #49a828 !important; font-weight: 600; }
@media (max-width: 720px) { .variable-grid { grid-template-columns: 1fr; } }
</style>
