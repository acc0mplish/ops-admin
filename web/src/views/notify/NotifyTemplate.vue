<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyTemplate,
  notifyTemplateInfo,
  queryNotifyTemplateList,
  saveNotifyTemplate
} from '../../api/ops'
import { queryMonitorAlertEventList } from '../../api/monitor'
import { nt } from '../../utils/notify-i18n'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const templateDialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const templateCategory = ref('all')
const templateScopeCategory = ref('all')
const previewEventId = ref()
const previewEvents = ref([])
const previewLoading = ref(false)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', channelType: '', scope: '', status: '' })
const form = reactive({ id: undefined, name: '', channelType: 'dingtalk', scope: 'monitor', title: '', content: '', status: 1, description: '' })

const scopeOptions = [
  { label: 'Monitor Alert', value: 'monitor' },
  { label: 'Schedule Task', value: 'schedule' },
  { label: 'Job Orchestration', value: 'job' },
  { label: 'CI/CD Pipeline', value: 'pipeline' }
]
const filterScopeOptions = scopeOptions

const channelTypes = [
  { label: 'DingTalk Bot', value: 'dingtalk', tone: 'ding' },
  { label: 'WeCom Bot', value: 'wecom', tone: 'wecom' },
  { label: 'Feishu Bot', value: 'feishu', tone: 'feishu' },
  { label: nt('userDefinedWebhook'), value: 'webhook', tone: 'webhook' }
]

const commonTemplates = [
  { id: 'ding-alert', scope: 'monitor', channelType: 'dingtalk', name: 'DingTalk - Monitor Alert Notification', title: '【{{severity}}】{{alertName}} -- {{status}}', content: nt('seedDingAlertContent'), description: nt('seedDingAlertDesc') },
  { id: 'ding-schedule', scope: 'schedule', channelType: 'dingtalk', name: 'DingTalk - Schedule Task Notification', title: '【Schedule Task】{{taskName}} -- {{status}}', content: nt('seedDingScheduleContent'), description: nt('seedDingScheduleDesc') },
  { id: 'ding-job', scope: 'job', channelType: 'dingtalk', name: 'DingTalk - Job Orchestration Notification', title: nt('seedJobTitle'), content: nt('seedDingJobContent'), description: nt('seedDingJobDesc') },
  { id: 'wecom-alert', scope: 'monitor', channelType: 'wecom', name: 'WeCom - Monitor Alert Markdown', title: '【{{severity}}】{{alertName}} -- {{status}}', content: nt('seedWecomAlertContent'), description: nt('seedWecomAlertDesc') },
  { id: 'wecom-schedule', scope: 'schedule', channelType: 'wecom', name: 'WeCom - Schedule Task Notification', title: '【Schedule Task】{{taskName}} -- {{status}}', content: nt('seedWecomScheduleContent'), description: nt('seedWecomScheduleDesc') },
  { id: 'wecom-job', scope: 'job', channelType: 'wecom', name: 'WeCom - Job Orchestration Notification', title: nt('seedJobTitle'), content: nt('seedWecomJobContent'), description: nt('seedWecomJobDesc') },
  { id: 'feishu-alert', scope: 'monitor', channelType: 'feishu', name: 'Feishu - Alert Card Message', title: '【{{severity}}】{{alertName}} -- {{status}}', content: nt('seedFeishuAlertContent'), description: nt('seedFeishuAlertDesc') },
  { id: 'feishu-schedule', scope: 'schedule', channelType: 'feishu', name: 'Feishu - Schedule Task Notification Card', title: '【Schedule Task】{{taskName}} -- {{status}}', content: nt('seedFeishuScheduleContent'), description: nt('seedFeishuScheduleDesc') },
  { id: 'feishu-job', scope: 'job', channelType: 'feishu', name: 'Feishu - Job Orchestration Notification Card', title: nt('seedJobTitle'), content: nt('seedFeishuJobContent'), description: nt('seedFeishuJobDesc') },
  { id: 'webhook-alert', scope: 'monitor', channelType: 'webhook', name: nt('seedWebhookAlertName'), title: '{{alertName}}', content: nt('seedWebhookAlertContent'), description: nt('seedWebhookAlertDesc') },
  { id: 'webhook-schedule', scope: 'schedule', channelType: 'webhook', name: nt('seedWebhookScheduleName'), title: '【Schedule Task】{{taskName}} -- {{status}}', content: nt('seedWebhookScheduleContent'), description: nt('seedWebhookScheduleDesc') },
  { id: 'webhook-job', scope: 'job', channelType: 'webhook', name: nt('seedWebhookJobName'), title: nt('seedJobTitle'), content: nt('seedWebhookJobContent'), description: nt('seedWebhookJobDesc') },
  { id: 'ding-pipeline', scope: 'pipeline', channelType: 'dingtalk', name: 'DingTalk - CI/CD Pipeline Notification', title: '【{{status}}】{{appName}} · {{stageName}}', content: nt('seedDingPipelineContent'), description: nt('seedDingPipelineDesc') },
  { id: 'wecom-pipeline', scope: 'pipeline', channelType: 'wecom', name: 'WeCom - CI/CD Pipeline Notification', title: '【{{status}}】{{appName}} · {{stageName}}', content: nt('seedWecomPipelineContent'), description: nt('seedWecomPipelineDesc') },
  { id: 'feishu-pipeline', scope: 'pipeline', channelType: 'feishu', name: 'Feishu - CI/CD Pipeline Notification Card', title: '【{{status}}】{{appName}} · {{stageName}}', content: nt('seedFeishuPipelineContent'), description: nt('seedFeishuPipelineDesc') },
  { id: 'webhook-pipeline', scope: 'pipeline', channelType: 'webhook', name: nt('seedWebhookPipelineName'), title: '【{{status}}】{{appName}} · {{stageName}}', content: nt('seedWebhookPipelineContent'), description: nt('seedWebhookPipelineDesc') }
]

const visibleTemplates = computed(() => commonTemplates.filter((item) =>
  (templateCategory.value === 'all' || item.channelType === templateCategory.value) &&
  (templateScopeCategory.value === 'all' || item.scope === templateScopeCategory.value)
))
const selectedChannel = computed(() => channelTypes.find((item) => item.value === form.channelType) || channelTypes[0])
const scopeVariables = {
  all: ['scope', 'event', 'targetName', 'status', 'summary', 'detail', 'startedAt', 'finishedAt'],
  monitor: ['alertName', 'severity', 'datasourceName', 'instance', 'value', 'threshold'],
  schedule: ['taskName', 'taskType', 'cronExpr', 'duration', 'triggerType'],
  job: ['jobName', 'jobHistoryId', 'stepName', 'stepMessage', 'triggerType', 'notifyAt'],
  pipeline: ['pipelineName', 'pipelineRunId', 'appName', 'env', 'branch', 'imageTag', 'stageName', 'notifyAt']
}
const availableVariables = computed(() => [...scopeVariables.all, ...(scopeVariables[form.scope] || [])])

const previewContext = reactive({
  scope: 'monitor', event: 'firing', targetName: nt('sampleHostCpu'), status: nt('statusFiring'), summary: nt('sampleCpuSummary'),
  detail: 'instance="10.0.0.12:9100", job="node-exporter"', startedAt: '2026-07-10 18:30:00', finishedAt: '-',
  alertName: nt('sampleHostCpu'), severity: 'P1', datasourceName: nt('samplePrometheus'), instance: '10.0.0.12:9100', value: '93.42', threshold: '90',
  taskName: nt('sampleHealthCheck'), taskType: 'HTTP Probe', cronExpr: '0 */5 * * * *', duration: nt('sampleDuration'),
  jobName: nt('sampleRollingDeploy'), jobHistoryId: '1024', stepName: nt('sampleDeployStep'), stepMessage: nt('sampleDeployDone'), triggerType: nt('sampleManualTrigger'), notifyAt: '2026-07-15 11:05:35',
  statusColor: '#F53F3F', statusTone: 'warning'
})

function renderPreview(value) {
  return String(value || '').replace(/{{\s*([^}]+)\s*}}/g, (_, key) => previewContext[key.trim()] ?? `{{${key}}}`)
}

function statusStyle(status) {
  if (['recovered', 'resolved', 'success'].includes(status)) return { color: '#00B42A', tone: 'info' }
  if (['firing', 'failed'].includes(status)) return { color: '#F53F3F', tone: 'warning' }
  return { color: '#FF7D00', tone: 'comment' }
}

function variableToken(key) {
  return `{{${key}}}`
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    channelType: 'dingtalk',
    scope: 'monitor',
    title: '【{{severity}}】{{alertName}}',
    content: nt('seedResetContent'),
    status: 1,
    description: ''
  })
}

function scopeLabel(value) {
  if (!value) return nt('scopeNone')
  if (value === 'all') return nt('scopeRequired')
  return scopeOptions.find((item) => item.value === value)?.label || value
}

async function preparePreview() {
  previewEventId.value = undefined
  if (form.scope === 'monitor') {
    await loadPreviewEvents()
    return
  }
  const success = statusStyle('success')
  if (form.scope === 'schedule') {
    Object.assign(previewContext, {
      scope: 'schedule', event: 'success', targetName: nt('sampleHealthCheck'), status: nt('success'),
      summary: nt('sampleHttpSummary'), detail: 'GET https://api.example.com/health -> 200',
      startedAt: '2026-07-15 10:00:00', finishedAt: '2026-07-15 10:00:02', taskName: nt('sampleHealthCheck'),
      taskType: 'HTTP Probe', cronExpr: '0 */5 * * * *', duration: nt('sampleDuration'), triggerType: nt('sampleScheduleTrigger'),
      statusColor: success.color, statusTone: success.tone
    })
    return
  }
  if (form.scope === 'job') {
    Object.assign(previewContext, {
      scope: 'job', event: 'notify', targetName: nt('sampleRollingDeploy'), status: 'Notification',
      summary: nt('sampleJobSummary'), detail: nt('sampleJobDetail'),
      startedAt: '2026-07-15 10:58:10', finishedAt: '2026-07-15 11:05:35', jobName: nt('sampleRollingDeploy'),
      jobHistoryId: '1024', stepName: nt('sampleDeployStep'), stepMessage: nt('sampleDeployDone'), triggerType: nt('sampleManualTrigger'),
      notifyAt: '2026-07-15 11:05:35', statusColor: '#3370FF', statusTone: 'info'
    })
    return
  }
  if (form.scope === 'pipeline') {
    Object.assign(previewContext, {
      scope: 'pipeline', event: 'notify', targetName: nt('sampleDeployPipeline'), status: 'Notification',
      summary: nt('samplePipelineSummary'), detail: nt('samplePipelineDetail'),
      startedAt: '2026-07-21 10:00:00', finishedAt: '2026-07-21 10:05:35',
      pipelineName: nt('sampleDeployPipeline'), pipelineRunId: '1024', appName: nt('sampleOrderService'), env: 'prod', branch: 'main', imageTag: 'v20260721.1', stageName: nt('sampleDeployStep'), notifyAt: '2026-07-21 10:05:35', statusColor: '#3370FF', statusTone: 'info'
    })
    return
  }
  Object.assign(previewContext, { scope: 'all', event: 'notify', targetName: nt('samplePlatformNotify'), status: 'Notification', summary: nt('sampleGenericSummary'), detail: nt('sampleGenericDetail'), statusColor: '#3370FF', statusTone: 'info' })
}

function parseLabels(raw) {
  try {
    return JSON.parse(raw || '{}') || {}
  } catch (_) {
    return {}
  }
}

function applyPreviewEvent(id) {
  const event = previewEvents.value.find((item) => Number(item.id) === Number(id))
  if (!event) return
  const labels = parseLabels(event.labelsJson)
  const style = statusStyle(event.status)
  Object.assign(previewContext, {
    scope: 'monitor',
    event: event.status || 'firing',
    targetName: event.ruleName || event.metric || 'Alert Event',
    status: ({ pending: nt('statusPending'), firing: nt('statusFiring'), claimed: nt('statusClaimed'), silenced: nt('statusSilenced'), recovered: nt('statusRecovered'), resolved: nt('statusResolved') }[event.status] || event.status || '-'),
    summary: event.summary || '-',
    detail: event.labelsJson || '-',
    startedAt: event.firstTriggerAt || event.lastTriggerAt || '-',
    finishedAt: event.recoveredAt || '-',
    alertName: event.ruleName || event.metric || 'Alert Event',
    severity: event.severity || '-',
    datasourceName: event.datasourceName || labels.datasource || '-',
    instance: labels.instance || labels.pod || labels.service || '-',
    value: event.currentValue ?? '-',
    threshold: event.threshold ?? '-',
    statusColor: style.color,
    statusTone: style.tone
  })
}

async function loadPreviewEvents() {
  previewLoading.value = true
  try {
    const data = await queryMonitorAlertEventList({ pageNum: 1, pageSize: 100, keyword: '' })
    previewEvents.value = data.list || []
    const first = previewEvents.value.find((item) => ['firing', 'claimed', 'pending'].includes(item.status)) || previewEvents.value[0]
    if (first) {
      previewEventId.value = first.id
      applyPreviewEvent(first.id)
    }
  } finally {
    previewLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifyTemplateList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function openCreate() {
  isEdit.value = false
  resetForm()
  await preparePreview()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  Object.assign(form, await notifyTemplateInfo(row.id))
  if (!form.scope || form.scope === 'all') form.scope = undefined
  await preparePreview()
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateCategory.value = form.channelType || 'all'
  templateScopeCategory.value = form.scope || 'all'
  templateDialogVisible.value = true
}

async function applyTemplate(template) {
  const { id, ...templateForm } = template
  Object.assign(form, { id: undefined, ...templateForm, status: 1 })
  isEdit.value = false
  templateDialogVisible.value = false
  await preparePreview()
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.content.trim()) {
    ElMessage.warning(nt('warnRequiredFields'))
    return
  }
  if (!form.scope || form.scope === 'all') {
    ElMessage.warning(nt('warnScopeRequired'))
    return
  }
  saving.value = true
  try {
    await saveNotifyTemplate(form)
    ElMessage.success(nt('saved'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(nt('deleteConfirm', { name: row.name }), nt('notice'), { type: 'warning' })
  await deleteNotifyTemplate(row.id)
  ElMessage.success(nt('deleted'))
  await loadData()
}

function channelTypeLabel(value) {
  return channelTypes.find((item) => item.value === value)?.label || value
}

watch(() => form.scope, preparePreview)

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ nt('pageTitle') }}</h2>
        <p class="page-desc">{{ nt('pageDesc') }}</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">{{ nt('importTemplates') }}</el-button><el-button type="primary" @click="openCreate">{{ nt('addTemplate') }}</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="nt('searchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.channelType" clearable :placeholder="nt('channelType')" style="width: 180px"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select>
      <el-select v-model="query.scope" clearable :placeholder="nt('applyScope')" style="width: 150px"><el-option v-for="item in filterScopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
      <el-select v-model="query.status" clearable :placeholder="nt('status')" style="width: 120px"><el-option :label="nt('actionEnable')" value="1" /><el-option :label="nt('actionDisable')" value="2" /></el-select>
      <el-button type="primary" @click="loadData">{{ nt('search') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" :label="nt('templateName')" min-width="180" />
      <el-table-column :label="nt('channelType')" width="180"><template #default="{ row }"><el-tag effect="light">{{ channelTypeLabel(row.channelType) }}</el-tag></template></el-table-column>
      <el-table-column :label="nt('applyScope')" width="120"><template #default="{ row }"><el-tag effect="plain" :type="row.scope === 'all' ? 'warning' : 'info'">{{ scopeLabel(row.scope) }}</el-tag></template></el-table-column>
      <el-table-column prop="title" :label="nt('title')" min-width="220" show-overflow-tooltip />
      <el-table-column prop="description" :label="nt('description')" min-width="240" show-overflow-tooltip />
      <el-table-column :label="nt('status')" width="100" align="center"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? nt('enabled') : nt('disabled') }}</el-tag></template></el-table-column>
      <el-table-column prop="updateTime" :label="nt('updatedAt')" width="180" />
      <el-table-column :label="nt('actions')" width="150" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">{{ nt('edit') }}</el-button><el-button link type="danger" @click="handleDelete(row)">{{ nt('delete') }}</el-button></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? nt('editTemplate') : nt('addTemplateTitle')" width="1180px" top="5vh">
      <el-form label-width="100px">
        <el-row :gutter="18">
          <el-col :span="8"><el-form-item :label="nt('templateName')" required><el-input v-model="form.name" :placeholder="nt('namePlaceholder')" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="nt('channelType')"><el-select v-model="form.channelType" style="width: 100%"><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="nt('applyScope')" required><el-select v-model="form.scope" :placeholder="nt('scopePlaceholder')" style="width: 100%"><el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
          <el-col :span="18"><el-form-item :label="nt('title')"><el-input v-model="form.title" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item :label="nt('status')"><el-radio-group v-model="form.status"><el-radio :value="1">{{ nt('actionEnable') }}</el-radio><el-radio :value="2">{{ nt('actionDisable') }}</el-radio></el-radio-group></el-form-item></el-col>
        </el-row>
        <div class="template-workbench">
          <section class="editor-pane">
            <div class="pane-title"><strong>{{ nt('templateContent') }}</strong><el-button link type="primary" @click="openTemplateDialog">{{ nt('importTemplates') }}</el-button></div>
            <el-input v-model="form.content" class="template-editor" type="textarea" :rows="18" :placeholder="nt('contentPlaceholder')" />
            <div class="variable-list"><span>{{ nt('availableVars', { scope: scopeLabel(form.scope) }) }}</span><code v-for="key in availableVariables" :key="key">{{ variableToken(key) }}</code></div>
          </section>
          <section class="preview-pane" :class="`preview-${selectedChannel.tone}`">
            <div class="pane-title preview-title"><strong>{{ nt('sendPreview') }}</strong><div><template v-if="form.scope === 'monitor'"><el-select v-model="previewEventId" :loading="previewLoading" filterable clearable :placeholder="nt('selectAlertEvent')" style="width: 220px" @change="applyPreviewEvent"><el-option v-for="item in previewEvents" :key="item.id" :label="`${item.ruleName || item.metric} / ${item.status}`" :value="item.id" /></el-select><el-button link type="primary" @click="loadPreviewEvents">{{ nt('refreshList') }}</el-button></template><el-tag v-else effect="plain">{{ nt('sampleDataFor', { scope: scopeLabel(form.scope) }) }}</el-tag></div></div>
            <div v-if="form.channelType === 'feishu'" class="feishu-card">
              <div class="feishu-header" :style="{ background: previewContext.statusColor }"><span>{{ scopeLabel(form.scope) }}</span><strong>{{ renderPreview(form.title) || nt('notificationTitle') }}</strong></div>
              <pre>{{ renderPreview(form.content) }}</pre>
              <div class="feishu-footer">Ops Admin</div>
            </div>
            <div v-else-if="form.channelType === 'wecom'" class="wecom-message"><strong>{{ renderPreview(form.title) || nt('notificationTitle') }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else-if="form.channelType === 'dingtalk'" class="dingtalk-message"><div class="ding-top">DINGTALK BOT</div><strong>{{ renderPreview(form.title) || nt('notificationTitle') }}</strong><pre>{{ renderPreview(form.content) }}</pre></div>
            <div v-else class="webhook-message"><span>POST / webhook</span><pre>{{ JSON.stringify({ title: renderPreview(form.title), content: renderPreview(form.content), status: previewContext.status, targetName: previewContext.targetName }, null, 2) }}</pre></div>
          </section>
        </div>
        <el-form-item :label="nt('description')" style="margin-top: 18px"><el-input v-model="form.description" type="textarea" :rows="2" :placeholder="nt('descPlaceholder')" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ nt('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ nt('save') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" :title="nt('importDialogTitle')" width="980px" top="8vh">
      <div class="template-import-head"><div><strong>{{ nt('selectStyle') }}</strong><span>{{ nt('selectStyleDesc') }}</span></div><div class="import-filters"><el-select v-model="templateScopeCategory" style="width: 150px"><el-option :label="nt('allScopes')" value="all" /><el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select><el-select v-model="templateCategory" style="width: 190px"><el-option :label="nt('allChannels')" value="all" /><el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></div></div>
      <div class="template-card-grid">
        <button v-for="item in visibleTemplates" :key="item.id" type="button" class="import-card" @click="applyTemplate(item)">
          <div class="template-tags"><el-tag size="small" :type="item.channelType === 'feishu' ? 'primary' : item.channelType === 'wecom' ? 'success' : item.channelType === 'dingtalk' ? 'warning' : 'info'">{{ channelTypeLabel(item.channelType) }}</el-tag><el-tag size="small" effect="plain" type="info">{{ scopeLabel(item.scope) }}</el-tag></div>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.title }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">{{ nt('cancel') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.header-actions { display: flex; gap: 10px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.template-workbench { display: grid; grid-template-columns: minmax(0, 1.08fr) minmax(360px, .92fr); gap: 18px; }
.editor-pane, .preview-pane { min-height: 500px; border: 1px solid #dbe5f4; border-radius: 8px; overflow: hidden; }
.editor-pane { background: #fff; }
.preview-pane { padding: 14px; background: #f5f7fb; }
.pane-title { display: flex; align-items: center; justify-content: space-between; height: 44px; padding: 0 14px; border-bottom: 1px solid #e3eaf5; color: #263d65; }
.pane-title span { color: #8491a9; font-size: 12px; }
.preview-title > div { display: flex; align-items: center; gap: 6px; }
.template-editor :deep(.el-textarea__inner) { min-height: 370px !important; border: 0; border-radius: 0; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.65; }
.variable-list { display: flex; flex-wrap: wrap; gap: 6px; padding: 12px 14px; border-top: 1px solid #e3eaf5; color: #71809b; font-size: 12px; }
.variable-list span { width: 100%; margin-bottom: 2px; font-weight: 600; }
.variable-list code { padding: 3px 6px; border-radius: 4px; background: #eff3fb; color: #4567c7; }
.preview-pane pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-family: inherit; font-size: 13px; line-height: 1.7; }
.dingtalk-message, .wecom-message, .feishu-card, .webhook-message { margin-top: 20px; overflow: hidden; border-radius: 8px; box-shadow: 0 8px 20px rgba(27, 47, 84, .12); }
.dingtalk-message { padding: 18px; background: #fff; border-left: 4px solid #2f77ff; }
.ding-top { margin-bottom: 14px; color: #2f77ff; font-size: 11px; font-weight: 700; letter-spacing: .8px; }
.dingtalk-message strong, .wecom-message strong { display: block; margin-bottom: 12px; color: #1f2f4e; font-size: 17px; }
.wecom-message { padding: 18px; background: #fff; border-top: 4px solid #1aad19; }
.feishu-card { background: #fff; border: 1px solid #dce5f3; }
.feishu-header { padding: 16px 18px; color: #fff; background: #3370ff; }
.feishu-header span { display: block; opacity: .78; font-size: 12px; }
.feishu-header strong { display: block; margin-top: 6px; font-size: 17px; }
.feishu-card pre { padding: 18px; color: #263d65; }
.feishu-footer { padding: 10px 18px; border-top: 1px solid #edf1f7; color: #97a4b8; font-size: 12px; }
.webhook-message { background: #10182b; color: #d8e4ff; }
.webhook-message span { display: block; padding: 10px 14px; color: #8eb0ff; background: #18233d; font: 12px Consolas, monospace; }
.webhook-message pre { padding: 16px; font-family: Consolas, Monaco, monospace; color: #d8e4ff; }
.template-import-head { display: flex; align-items: center; justify-content: space-between; gap: 18px; margin-bottom: 16px; }
.template-import-head strong { display: block; color: #20355c; }
.template-import-head span { display: block; margin-top: 5px; color: #8491a9; font-size: 13px; }
.import-filters, .template-tags { display: flex; align-items: center; gap: 8px; }
.template-card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; max-height: 520px; overflow: auto; padding-right: 4px; }
.import-card { min-height: 148px; padding: 16px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .2s, box-shadow .2s; }
.import-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.import-card strong { display: block; margin-top: 12px; color: #1d3154; }
.import-card p { min-height: 36px; margin: 7px 0; color: #7282a0; font-size: 13px; line-height: 1.45; }
.import-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 900px) { .template-workbench { grid-template-columns: 1fr; } .template-card-grid { grid-template-columns: 1fr; } }
</style>
