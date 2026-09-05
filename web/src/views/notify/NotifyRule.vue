<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNotifyRule,
  notifyRuleInfo,
  queryNotifyChannelOptions,
  queryNotifyRuleList,
  queryNotifyTemplateOptions,
  saveNotifyRule,
  testNotifyRule
} from '../../api/ops'
import { nt } from '../../utils/notify-i18n'

const loading = ref(false)
const saving = ref(false)
const optionsLoading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const activeStep = ref(0)
const rows = ref([])
const total = ref(0)
const templateOptions = ref([])
const channelOptions = ref([])

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', scope: '', status: '' })
const form = reactive({
  id: undefined,
  name: '',
  scope: 'monitor',
  events: ['firing', 'recovered'],
  templateId: undefined,
  channelIds: [],
  status: 1,
  description: ''
})

const scopeOptions = [
  { label: 'Job Orchestration', value: 'job' },
  { label: 'CI/CD Pipeline', value: 'pipeline' },
  { label: 'Schedule Task', value: 'schedule' },
  { label: 'Monitor Alert', value: 'monitor' }
]
const filterScopeOptions = scopeOptions

const eventOptions = [
  { label: nt('success'), value: 'success' },
  { label: nt('failed'), value: 'failed' },
  { label: nt('pendingManualReview'), value: 'waiting_approval' },
  { label: nt('rejected'), value: 'rejected' },
  { label: nt('firing'), value: 'firing' },
  { label: nt('recovered'), value: 'recovered' },
  { label: nt('allEvents'), value: 'all' }
]

const channelTypeLabels = {
  dingtalk: 'DingTalk Bot',
  wecom: 'WeCom Bot',
  feishu: 'Feishu Bot',
  webhook: nt('customWebhookShort')
}

const selectedTemplate = computed(() => templateOptions.value.find((item) => Number(item.id) === Number(form.templateId)))
const compatibleTemplateOptions = computed(() => templateOptions.value.filter((item) => {
  const templateScope = item.scope || 'all'
  if (form.scope === 'all') return true
  if (templateScope === 'all') return true
  return templateScope === form.scope
}))
const selectedChannels = computed(() => channelOptions.value.filter((item) => form.channelIds.map(Number).includes(Number(item.id))))
const wizardSteps = [
  { title: nt('step1Title'), description: nt('step1Desc') },
  { title: nt('step2Title'), description: nt('step2Desc') },
  { title: nt('step3Title'), description: nt('step3Desc') }
]
const routeWarnings = computed(() => {
  const warnings = []
  if (!form.scope || form.scope === 'all') warnings.push(nt('warnScopeSpecific'))
  if (!form.events.length) warnings.push(nt('warnSelectEvent'))
  if (!selectedTemplate.value) warnings.push(nt('warnSelectTemplate'))
  if (selectedTemplate.value && !compatibleTemplateOptions.value.some((item) => Number(item.id) === Number(selectedTemplate.value.id))) {
    warnings.push(nt('warnScopeMismatch'))
  }
  if (!selectedChannels.value.length) warnings.push(nt('warnSelectChannel'))
  const templateType = selectedTemplate.value?.channelType
  if (templateType && selectedChannels.value.some((item) => item.channelType !== templateType)) {
    warnings.push(nt('warnTypeMismatch'))
  }
  return warnings
})

function scopeLabel(value) {
  if (!value) return nt('ruleScopeNone')
  if (value === 'all') return nt('ruleScopeRequired')
  return scopeOptions.find((item) => item.value === value)?.label || value
}

function templateScopeLabel(value) {
  return value === 'all' ? nt('genericTemplate') : scopeLabel(value)
}

function eventLabel(value) {
  return eventOptions.find((item) => item.value === value)?.label || value
}

function typeLabel(value) {
  return channelTypeLabels[value] || value || '-'
}

function defaultEventsForScope(scope) {
  if (scope === 'monitor') return ['firing', 'recovered']
  if (scope === 'job') return ['failed', 'waiting_approval', 'rejected']
  if (scope === 'pipeline') return ['success', 'failed', 'waiting_approval', 'rejected']
  return ['success', 'failed']
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    scope: 'monitor',
    events: ['firing', 'recovered'],
    templateId: undefined,
    channelIds: [],
    status: 1,
    description: ''
  })
}

async function loadBaseOptions() {
  optionsLoading.value = true
  try {
    const [templates, channels] = await Promise.all([
      queryNotifyTemplateOptions(),
      queryNotifyChannelOptions()
    ])
    templateOptions.value = Array.isArray(templates) ? templates : []
    channelOptions.value = Array.isArray(channels) ? channels : []
  } catch (error) {
    templateOptions.value = []
    channelOptions.value = []
    ElMessage.error(error?.message || nt('warnLoadOptions'))
  } finally {
    optionsLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifyRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function openCreate() {
  isEdit.value = false
  activeStep.value = 0
  resetForm()
  dialogVisible.value = true
  await loadBaseOptions()
}

async function openEdit(row) {
  isEdit.value = true
  activeStep.value = 0
  dialogVisible.value = true
  const [data] = await Promise.all([
    notifyRuleInfo(row.id),
    loadBaseOptions()
  ])
  Object.assign(form, {
    id: data.id,
    name: data.name || '',
    scope: data.scope === 'all' ? undefined : (data.scope || undefined),
    events: data.events || [],
    templateId: data.templateId,
    channelIds: data.channelIds || [],
    status: data.status || 1,
    description: data.description || ''
  })
}

function handleScopeChange(value) {
  form.events = defaultEventsForScope(value)
  if (!compatibleTemplateOptions.value.some((item) => Number(item.id) === Number(form.templateId))) {
    form.templateId = undefined
    form.channelIds = []
  }
}

function handleEventsChange(values) {
  const last = values[values.length - 1]
  form.events = last === 'all' ? ['all'] : values.filter((item) => item !== 'all')
}

function handleTemplateChange(value) {
  const template = templateOptions.value.find((item) => Number(item.id) === Number(value))
  const templateScope = template?.scope || 'all'
  if (!template || form.scope !== 'all' || templateScope === 'all') return
  form.scope = templateScope
  form.events = defaultEventsForScope(templateScope)
  ElMessage.info(nt('scopeSwitched', { scope: scopeLabel(templateScope) }))
}

function channelDisabled(item) {
  return Boolean(selectedTemplate.value && item.channelType !== selectedTemplate.value.channelType)
}

function nextStep() {
  if (activeStep.value === 0) {
    if (!form.name.trim()) return ElMessage.warning(nt('warnRuleNameFirst'))
    if (!form.events.length) return ElMessage.warning(nt('warnSelectEvent'))
  }
  if (activeStep.value === 1 && routeWarnings.value.length) {
    return ElMessage.warning(routeWarnings.value[0])
  }
  activeStep.value = Math.min(activeStep.value + 1, wizardSteps.length - 1)
}

function previousStep() {
  activeStep.value = Math.max(activeStep.value - 1, 0)
}

async function submit() {
  if (!form.name.trim() || routeWarnings.value.length) {
    ElMessage.warning(routeWarnings.value[0] || nt('warnRuleName'))
    return
  }
  saving.value = true
  try {
    await saveNotifyRule(form)
    ElMessage.success(nt('saved'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleTest(row) {
  await ElMessageBox.confirm(
    nt('testConfirm', { name: row.name }),
    'Notification Rule Test',
    { type: 'warning', confirmButtonText: nt('send'), cancelButtonText: nt('cancel') }
  )
  const result = await testNotifyRule(row.id)
  ElMessage.success(nt('testQueued', { count: result?.queued || 0 }))
}

async function handleDelete(row) {
  await ElMessageBox.confirm(nt('ruleDeleteConfirm', { name: row.name }), nt('ruleDeleteTitle'), { type: 'warning' })
  await deleteNotifyRule(row.id)
  ElMessage.success(nt('deleted'))
  await loadData()
}

function templateName(id) {
  return templateOptions.value.find((item) => Number(item.id) === Number(id))?.name || id || '-'
}

function channelNames(ids) {
  return (ids || [])
    .map((id) => channelOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join(', ') || '-'
}

watch(() => form.templateId, () => {
  if (!selectedTemplate.value) return
  form.channelIds = form.channelIds.filter((id) => {
    const channel = channelOptions.value.find((item) => Number(item.id) === Number(id))
    return channel?.channelType === selectedTemplate.value.channelType
  })
})

onMounted(async () => {
  await loadBaseOptions()
  await loadData()
})
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Notification Rule</h2>
        <p class="page-desc">{{ nt('rulePageDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ nt('addRule') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="nt('ruleSearchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.scope" clearable :placeholder="nt('applyRange')" style="width: 160px">
        <el-option v-for="item in filterScopeOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.status" clearable :placeholder="nt('status')" style="width: 120px">
        <el-option :label="nt('actionEnable')" value="1" />
        <el-option :label="nt('actionDisable')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ nt('search') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" :label="nt('ruleName')" min-width="180" />
      <el-table-column :label="nt('applyRange')" width="130">
        <template #default="{ row }">
          <el-tag :type="row.scope === 'all' ? 'warning' : 'info'" effect="plain">{{ scopeLabel(row.scope) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="nt('triggerEvent')" min-width="230">
        <template #default="{ row }">
          <el-tag v-for="item in row.events || []" :key="item" class="tag" effect="plain">{{ eventLabel(item) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="nt('messageTemplate')" min-width="170">
        <template #default="{ row }">{{ templateName(row.templateId) }}</template>
      </el-table-column>
      <el-table-column label="Notification Channel" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ channelNames(row.channelIds) }}</template>
      </el-table-column>
      <el-table-column :label="nt('status')" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? nt('enabled') : nt('disabled') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="nt('actions')" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleTest(row)">Test</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ nt('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ nt('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? nt('editRuleTitle') : nt('addRuleTitle')" width="880px" destroy-on-close>
      <div class="notify-wizard-steps">
        <button v-for="(step, index) in wizardSteps" :key="step.title" type="button" class="notify-wizard-step" :class="{ active: activeStep === index, done: activeStep > index }" @click="index < activeStep && (activeStep = index)">
          <span>{{ index + 1 }}</span><strong>{{ step.title }}</strong><small>{{ step.description }}</small>
        </button>
      </div>
      <el-form class="notify-wizard-form" label-position="top">
        <template v-if="activeStep === 0">
          <div class="wizard-heading"><h3>{{ nt('step1Heading') }}</h3><p>{{ nt('step1Sub') }}</p></div>
          <el-form-item :label="nt('ruleName')" required><el-input v-model="form.name" :placeholder="nt('ruleNamePlaceholder')" /></el-form-item>
          <el-form-item :label="nt('applyRange')">
            <el-select v-model="form.scope" style="width: 100%" @change="handleScopeChange">
              <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item :label="nt('triggerEvent')" required>
            <el-checkbox-group v-model="form.events" class="event-choice-grid" @change="handleEventsChange">
              <el-checkbox v-for="item in eventOptions" :key="item.value" :value="item.value" border>{{ item.label }}</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </template>
        <template v-else-if="activeStep === 1">
          <div class="wizard-heading"><h3>{{ nt('step2Heading') }}</h3><p>{{ nt('step2Sub') }}</p></div>
          <el-form-item :label="nt('messageTemplate')" required>
          <el-select
            v-model="form.templateId"
            filterable
            :loading="optionsLoading"
            :no-data-text="nt('noTemplatesHint')"
            :placeholder="nt('templatePlaceholder')"
            style="width: 100%"
            @change="handleTemplateChange"
          >
            <el-option v-for="item in compatibleTemplateOptions" :key="item.id" :label="`${item.name} · ${templateScopeLabel(item.scope || 'all')} · ${typeLabel(item.channelType)}`" :value="item.id" />
          </el-select>
          </el-form-item>
          <el-form-item label="Notification Channel" required>
          <el-select v-model="form.channelIds" multiple filterable :loading="optionsLoading" :placeholder="nt('channelPlaceholder')" style="width: 100%">
            <el-option v-for="item in channelOptions" :key="item.id" :label="`${item.name} · ${typeLabel(item.channelType)}`" :value="item.id" :disabled="channelDisabled(item)" />
          </el-select>
          </el-form-item>
          <el-alert v-if="routeWarnings.length" :title="routeWarnings.join('; ')" type="warning" :closable="false" show-icon />
        </template>
        <template v-else>
          <div class="wizard-heading"><h3>{{ nt('step3Heading') }}</h3><p>{{ nt('step3Sub') }}</p></div>
          <div class="route-preview">
            <div class="route-chain">
              <span class="route-node">{{ form.events.length ? form.events.map(eventLabel).join(', ') : nt('eventNotSelected') }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedTemplate?.name || nt('templateNotSelected') }}</span>
              <span class="route-arrow">→</span>
              <span class="route-node">{{ selectedChannels.length ? selectedChannels.map((item) => item.name).join(', ') : nt('channelNotSelected') }}</span>
            </div>
            <div v-if="selectedTemplate" class="route-meta">{{ nt('routeMeta', { scope: templateScopeLabel(selectedTemplate.scope || 'all'), type: typeLabel(selectedTemplate.channelType) }) }}</div>
            <el-alert v-if="routeWarnings.length" :title="routeWarnings.join('; ')" type="warning" :closable="false" show-icon />
            <el-alert v-else :title="nt('routingComplete')" type="success" :closable="false" show-icon />
          </div>
          <el-form-item :label="nt('statusAfterSave')" class="wizard-status-field">
            <el-radio-group v-model="form.status"><el-radio :value="1">{{ nt('activateNow') }}</el-radio><el-radio :value="2">{{ nt('saveInactive') }}</el-radio></el-radio-group>
          </el-form-item>
          <el-form-item :label="nt('descriptionOptional')"><el-input v-model="form.description" type="textarea" :rows="3" :placeholder="nt('ruleDescPlaceholder')" /></el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ nt('cancel') }}</el-button>
        <el-button v-if="activeStep" @click="previousStep">{{ nt('previous') }}</el-button>
        <el-button v-if="activeStep < wizardSteps.length - 1" type="primary" @click="nextStep">{{ nt('next') }}</el-button>
        <el-button v-else type="primary" :loading="saving" @click="submit">{{ nt('saveRule') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.tag { margin-right: 6px; margin-bottom: 4px; }
.pager { display: flex; justify-content: flex-end; }
.notify-wizard-steps { display:grid; grid-template-columns:repeat(3, 1fr); gap:8px; margin:0 0 22px; padding:8px; border:1px solid #e1eaf8; border-radius:12px; background:#f7faff; }
.notify-wizard-step { display:grid; grid-template-columns:28px 1fr; column-gap:8px; align-items:center; min-height:54px; padding:8px 10px; color:#8a9ab3; text-align:left; border:0; border-radius:9px; background:transparent; cursor:default; }
.notify-wizard-step span { display:grid; width:26px; height:26px; place-items:center; grid-row:span 2; border:1px solid #ced9ea; border-radius:50%; color:#7d8da6; background:#fff; font-size:12px; font-weight:700; }
.notify-wizard-step strong { color:inherit; font-size:13px; }
.notify-wizard-step small { margin-top:2px; color:inherit; font-size:12px; }
.notify-wizard-step.done { cursor:pointer; color:#4f73c6; }
.notify-wizard-step.active { color:#1f5fce; background:#fff; box-shadow:0 4px 14px rgba(44, 84, 160, .08); }
.notify-wizard-step.active span { border-color:#4f73ff; color:#fff; background:#4f73ff; }
.notify-wizard-form { min-height:330px; }
.wizard-heading { margin:0 0 18px; }
.wizard-heading h3 { margin:0 0 5px; color:#1f2a44; font-size:18px; }
.wizard-heading p { margin:0; color:#7485a7; font-size:13px; line-height:1.6; }
.event-choice-grid { display:grid; grid-template-columns:repeat(3, minmax(0, 1fr)); gap:10px; width:100%; }
.event-choice-grid :deep(.el-checkbox) { display:flex; width:100%; height:38px; margin-right:0; }
.wizard-status-field { margin-top:18px; }
.route-preview { width: 100%; padding: 14px; border: 1px solid #dce6f5; border-radius: 6px; background: #f7faff; }
.route-chain { display: flex; align-items: center; gap: 10px; min-width: 0; margin-bottom: 10px; }
.route-node { min-width: 0; padding: 7px 10px; overflow: hidden; color: #264d85; text-overflow: ellipsis; white-space: nowrap; border: 1px solid #c9dbf5; border-radius: 5px; background: #fff; }
.route-arrow { flex: 0 0 auto; color: #6b8fc7; font-size: 18px; }
.route-meta { margin-bottom: 10px; color: #7282a0; font-size: 13px; }
@media (max-width: 720px) { .notify-wizard-steps, .event-choice-grid { grid-template-columns:1fr; } .notify-wizard-form { min-height:0; } }
</style>
