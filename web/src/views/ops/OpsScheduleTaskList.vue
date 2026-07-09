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
  queryOpsScriptOptions,
  runOpsScheduleTask,
  updateOpsScheduleTask,
  updateOpsScheduleTaskStatus
} from '../../api/ops'
import OpsTargetSelector from './components/OpsTargetSelector.vue'
import OpsCronEditor from './components/OpsCronEditor.vue'

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
  parameters: '',
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
  notifyRuleId: undefined
})

const selectedScriptTimeout = computed(() => {
  const current = scriptOptions.value.find((item) => Number(item.id) === Number(form.scriptId))
  return current?.timeoutSeconds || 300
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    taskType: 'script',
    templateId: undefined,
    scriptId: undefined,
    parameters: '',
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
    notifyRuleId: undefined
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
    parameters: data.parameters || '',
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
    notifyRuleId: data.notifyRuleId || undefined
  })
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
    parameters: data.parameters || '',
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
    notifyRuleId: data.notifyRuleId || undefined
  })
  dialogVisible.value = true
}

function applyTemplate(templateId) {
  const selected = templateOptions.value.find((item) => Number(item.id) === Number(templateId))
  if (!selected) return
  form.taskType = selected.taskType || 'script'
  form.scriptId = selected.scriptId || undefined
  form.parameters = selected.parameters || ''
  form.httpMethod = selected.httpMethod || 'GET'
  form.url = selected.url || ''
  form.headersJson = selected.headersJson || '{}'
  form.body = selected.body || ''
  form.expectedStatus = selected.expectedStatus || 200
  form.timeoutSeconds = selected.timeoutSeconds || 10
  if (selected.cronExpr) {
    form.cronExpr = selected.cronExpr
  }
  if (selected.description && !form.description) {
    form.description = selected.description
  }
}

function buildPayload() {
  return {
    id: form.id,
    name: form.name,
    taskType: form.taskType,
    templateId: form.templateId,
    scriptId: form.scriptId,
    parameters: form.parameters,
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
    notifyRuleId: form.notifyEnabled ? form.notifyRuleId : undefined
  }
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入任务名称')
    return
  }
  if (form.notifyEnabled && !form.notifyRuleId) {
    ElMessage.warning('请选择通知规则')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await updateOpsScheduleTask(buildPayload())
      ElMessage.success('任务已更新')
    } else {
      await addOpsScheduleTask(buildPayload())
      ElMessage.success('任务已创建')
    }
    dialogVisible.value = false
    await loadData()
    await loadBaseOptions()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除任务“${row.name}”吗？`, '提示', { type: 'warning' })
  await deleteOpsScheduleTask(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

async function handleRun(row) {
  await ElMessageBox.confirm(`确认立即执行任务“${row.name}”吗？`, '提示', { type: 'warning' })
  await runOpsScheduleTask(row.id)
  ElMessage.success('任务已触发，请前往任务日志查看执行结果')
  await loadData()
}

async function batchDelete() {
  if (!selectedRows.value.length) {
    ElMessage.warning('请先选择任务')
    return
  }
  await ElMessageBox.confirm(`确认删除选中的 ${selectedRows.value.length} 个任务吗？`, '提示', { type: 'warning' })
  await batchDeleteOpsScheduleTask(selectedRows.value.map((item) => item.id))
  ElMessage.success('批量删除成功')
  await loadData()
}

async function batchUpdateStatus(status) {
  if (!selectedRows.value.length) {
    ElMessage.warning('请先选择任务')
    return
  }
  await updateOpsScheduleTaskStatus({
    ids: selectedRows.value.map((item) => item.id),
    status
  })
  ElMessage.success(status === 1 ? '批量启用成功' : '批量禁用成功')
  await loadData()
}

async function toggleRowStatus(row) {
  await updateOpsScheduleTaskStatus({
    ids: [row.id],
    status: row.status === 1 ? 2 : 1
  })
  ElMessage.success(row.status === 1 ? '任务已禁用' : '任务已启用')
  await loadData()
}

function handleSelectionChange(value) {
  selectedRows.value = value
}

function statusLabel(value) {
  return value === 1 ? '启用' : '禁用'
}

function taskTypeLabel(value) {
  return value === 'http' ? 'HTTP 探针' : '脚本任务'
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
        <h2 class="page-title">任务列表</h2>
        <p class="page-desc">统一维护脚本任务和 HTTP 探针任务，支持批量启用、禁用、删除和立即执行。</p>
      </div>
      <div class="header-actions">
        <el-button @click="openCreateFromTemplate">基于模板新建</el-button>
        <el-button type="primary" @click="openCreate">新建任务</el-button>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="搜索任务名称 / 描述 / 地址" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable placeholder="任务类型" style="width: 140px">
          <el-option label="脚本任务" value="script" />
          <el-option label="HTTP 探针" value="http" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
          <el-option label="启用" value="1" />
          <el-option label="禁用" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">搜索</el-button>
        <el-button @click="resetQuery">重置</el-button>
      </div>
      <div class="toolbar-right">
        <el-button @click="batchUpdateStatus(1)">批量启用</el-button>
        <el-button @click="batchUpdateStatus(2)">批量禁用</el-button>
        <el-button type="danger" plain @click="batchDelete">批量删除</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="name" label="任务名称" min-width="180" />
      <el-table-column label="任务类型" width="120">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column prop="cronExpr" label="Cron 表达式" min-width="160" />
      <el-table-column prop="scriptName" label="脚本 / 地址" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">{{ row.taskType === 'http' ? row.url : row.scriptName }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="light">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="lastStatus" label="最近结果" width="110" />
      <el-table-column prop="lastSummary" label="最近摘要" min-width="220" show-overflow-tooltip />
      <el-table-column prop="lastRunAt" label="最近执行" width="180" />
      <el-table-column prop="nextRunAt" label="下次执行" width="180" />
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-button link type="success" @click="handleRun(row)">立即执行</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link @click="handleCopy(row)">复制</el-button>
          <el-button link :type="row.status === 1 ? 'warning' : 'success'" @click="toggleRowStatus(row)">
            {{ row.status === 1 ? '禁用' : '启用' }}
          </el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑任务' : '新建任务'" width="min(1080px, 92vw)">
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="任务名称" required>
              <el-input v-model="form.name" placeholder="例如：生产环境 Nginx 巡检" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="任务模板">
              <el-select v-model="form.templateId" clearable filterable placeholder="可选，选择后自动带入模板内容" style="width: 100%" @change="applyTemplate">
                <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="任务类型">
              <el-radio-group v-model="form.taskType">
                <el-radio value="script">脚本任务</el-radio>
                <el-radio value="http">HTTP 探针</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="Cron 表达式" required>
              <OpsCronEditor v-model="form.cronExpr" />
            </el-form-item>
          </el-col>

          <el-col :span="8">
            <el-form-item label="消息通知">
              <el-switch v-model="form.notifyEnabled" />
            </el-form-item>
          </el-col>
          <el-col v-if="form.notifyEnabled" :span="16">
            <el-form-item label="通知规则" required>
              <el-select v-model="form.notifyRuleId" filterable placeholder="选择通知规则" style="width: 100%">
                <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>

          <template v-if="form.taskType === 'script'">
            <el-col :span="12">
              <el-form-item label="脚本" required>
                <el-select v-model="form.scriptId" filterable placeholder="选择脚本库中的脚本" style="width: 100%">
                  <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="并发数">
                <el-input-number v-model="form.concurrency" :min="1" :max="10" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="超时秒数">
                <el-input :model-value="selectedScriptTimeout" disabled>
                  <template #append>s</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="执行参数">
                <el-input v-model="form.parameters" placeholder="例如：--env prod --dry-run" />
              </el-form-item>
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
              <el-form-item label="请求方法">
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
              <el-form-item label="探针地址" required>
                <el-input v-model="form.url" placeholder="例如：https://example.com/healthz" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="超时秒数">
                <el-input-number v-model="form.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="期望状态码">
                <el-input-number v-model="form.expectedStatus" :min="100" :max="599" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="18">
              <el-form-item label="请求头 JSON">
                <el-input v-model="form.headersJson" type="textarea" :rows="4" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="请求体">
                <el-input v-model="form.body" type="textarea" :rows="5" />
              </el-form-item>
            </el-col>
          </template>

          <el-col :span="24">
            <el-form-item label="描述">
              <el-input v-model="form.description" type="textarea" :rows="3" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
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
</style>
