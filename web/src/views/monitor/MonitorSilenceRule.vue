<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { mt } from '../../utils/monitor-i18n'
import {
  batchUpdateMonitorSilenceRules,
  deleteMonitorSilenceRule,
  monitorSilenceRuleInfo,
  previewMonitorSilenceRule,
  queryMonitorAlertRuleList,
  queryMonitorSilenceRuleList,
  saveMonitorSilenceRule
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const templateDialogVisible = ref(false)
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewData = ref({ matchedRuleCount: 0, matchedRules: [], matchedActiveEventCount: 0, matchedActiveEvents: [] })
const templateType = ref('metric')
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const ruleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', status: '' })
const timeRange = ref([])

const silenceTemplates = [
  { id: 'metric-maintenance', type: 'metric', name: mt('stlMetricMaintenanceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{}', description: mt('stlMetricMaintenanceDesc') },
  { id: 'metric-node', type: 'metric', name: mt('stlMetricNodeName'), matchMode: 'regex', ruleNamePattern: '^Host.*(CPU|메모리|디스크|부하).*', severity: '', matchersJson: '{}', description: mt('stlMetricNodeDesc') },
  { id: 'metric-k8s', type: 'metric', name: mt('stlMetricK8sName'), matchMode: 'regex', ruleNamePattern: '^(Kubernetes|Pod|Deployment|PVC).*', severity: '', matchersJson: '{}', description: mt('stlMetricK8sDesc') },
  { id: 'metric-instance', type: 'metric', name: mt('stlMetricInstanceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"instance":"10.0.0.1"}', description: mt('stlMetricInstanceDesc') },
  { id: 'metric-low-priority', type: 'metric', name: mt('stlMetricLowPriorityName'), matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{}', description: mt('stlMetricLowPriorityDesc') },
  { id: 'es-maintenance', type: 'elasticsearch', name: mt('stlEsMaintenanceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"log"}', description: mt('stlEsMaintenanceDesc') },
  { id: 'es-error', type: 'elasticsearch', name: mt('stlEsErrorName'), matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', matchersJson: '{"alert_type":"log"}', description: mt('stlEsErrorDesc') },
  { id: 'es-datasource', type: 'elasticsearch', name: mt('stlEsDatasourceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"log","datasource":"로그 Datasource 이름"}', description: mt('stlEsDatasourceDesc') },
  { id: 'es-low-priority', type: 'elasticsearch', name: mt('stlEsLowPriorityName'), matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{"alert_type":"log"}', description: mt('stlEsLowPriorityDesc') },
  { id: 'vl-maintenance', type: 'victorialogs', name: mt('stlVlMaintenanceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"victorialogs"}', description: mt('stlVlMaintenanceDesc') },
  { id: 'vl-error', type: 'victorialogs', name: mt('stlVlErrorName'), matchMode: 'regex', ruleNamePattern: '.*(ERROR|오류|실패).*', severity: '', matchersJson: '{"alert_type":"victorialogs"}', description: mt('stlVlErrorDesc') },
  { id: 'vl-datasource', type: 'victorialogs', name: mt('stlVlDatasourceName'), matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"victorialogs","datasource":"로그 Datasource 이름"}', description: mt('stlVlDatasourceDesc') },
  { id: 'vl-low-priority', type: 'victorialogs', name: mt('stlVlLowPriorityName'), matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{"alert_type":"victorialogs"}', description: mt('stlVlLowPriorityDesc') }
]

const visibleSilenceTemplates = computed(() => silenceTemplates.filter((item) => item.type === templateType.value))

const form = reactive({
  id: undefined,
  name: '',
  matchMode: 'select',
  ruleIds: [],
  ruleNamePattern: '',
  severity: '',
  alertType: '',
  matchersJson: '{}',
  startsAt: 0,
  endsAt: 0,
  priority: 100,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    matchMode: 'select',
    ruleIds: [],
    ruleNamePattern: '',
    severity: '',
    alertType: '',
    matchersJson: '{}',
    startsAt: 0,
    endsAt: 0,
    priority: 100,
    status: 1,
    description: ''
  })
  timeRange.value = []
}

function toUnixSeconds(value) {
  return value ? Math.floor(new Date(value).getTime() / 1000) : 0
}

function normalizePayload() {
  const [start, end] = timeRange.value || []
  return {
    ...form,
    ruleIds: form.matchMode === 'select' ? form.ruleIds : [],
    ruleNamePattern: form.matchMode === 'regex' ? form.ruleNamePattern : '',
    startsAt: toUnixSeconds(start),
    endsAt: toUnixSeconds(end)
  }
}

function silenceSchedule(row) {
  if (row.status !== 1) return { text: mt('scheduleDisabled'), type: 'info' }
  const now = Date.now()
  const start = row.startsAt ? new Date(row.startsAt).getTime() : 0
  const end = row.endsAt ? new Date(row.endsAt).getTime() : 0
  if (start && start > now) return { text: mt('schedulePending'), type: 'warning' }
  if (end && end < now) return { text: mt('scheduleExpired'), type: 'info' }
  return { text: mt('scheduleActive'), type: 'success' }
}

function selectedRuleNames(ids = []) {
  if (!ids.length) return mt('allRules')
  return ids
    .map((id) => ruleOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join(', ')
}

async function loadRuleOptions() {
  const data = await queryMonitorAlertRuleList({ pageNum: 1, pageSize: 1000 })
  ruleOptions.value = data.list || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorSilenceRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

function openTemplateDialog() {
  templateType.value = 'metric'
  templateDialogVisible.value = true
}

function applySilenceTemplate(template) {
  resetForm()
  Object.assign(form, {
    name: template.name,
    matchMode: template.matchMode,
    ruleIds: template.ruleIds || [],
    ruleNamePattern: template.ruleNamePattern || '',
    severity: template.severity || '',
    alertType: template.type === 'elasticsearch' ? 'log' : template.type,
    matchersJson: template.matchersJson,
    description: template.description
  })
  templateDialogVisible.value = false
  dialogVisible.value = true
}

function handleSelectionChange(selection) {
  selectedRuleIds.value = selection.map((item) => item.id)
}

async function handleBatchAction(action) {
  if (!selectedRuleIds.value.length) return
  const labels = { enable: mt('enabledOption'), disable: mt('disabledOption'), delete: mt('delete') }
  await ElMessageBox.confirm(mt('silenceBatchConfirm', { count: selectedRuleIds.value.length, action: labels[action] }), mt('batchStatusTitle'), { type: action === 'delete' ? 'warning' : 'info' })
  await batchUpdateMonitorSilenceRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(mt('silenceBatchDone', { action: labels[action] }))
  selectedRuleIds.value = []
  await loadData()
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorSilenceRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    matchMode: data.matchMode || 'regex',
    ruleIds: data.ruleIds || [],
    matchersJson: data.matchersJson || '{}'
  })
  timeRange.value = data.startsAt && data.endsAt ? [data.startsAt, data.endsAt] : []
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning(mt('enterSilenceName'))
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning(mt('enterRulePattern'))
    return
  }
  const [start, end] = timeRange.value || []
  if (start && end && new Date(end).getTime() <= new Date(start).getTime()) {
    ElMessage.warning(mt('endAfterStart'))
    return
  }
  const isGlobalPermanentSilence = form.matchMode === 'select' && !form.ruleIds.length && !form.alertType && !form.severity && form.matchersJson.trim() === '{}' && !start && !end
  if (isGlobalPermanentSilence) {
    await ElMessageBox.confirm(mt('globalSilenceConfirm'), mt('globalSilenceTitle'), { type: 'warning', confirmButtonText: mt('saveAnyway'), cancelButtonText: mt('backToEdit') })
  }
  saving.value = true
  try {
    await saveMonitorSilenceRule(normalizePayload())
    ElMessage.success(mt('savedMsg'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function openPreview() {
  if (!form.name.trim()) {
    ElMessage.warning(mt('enterSilenceNameFirst'))
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning(mt('enterRulePattern'))
    return
  }
  previewLoading.value = true
  try {
    previewData.value = await previewMonitorSilenceRule(normalizePayload())
    previewVisible.value = true
  } finally {
    previewLoading.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(mt('silenceDeleteConfirm', { name: row.name }), mt('noticeTitle'), { type: 'warning' })
  await deleteMonitorSilenceRule(row.id)
  ElMessage.success(mt('deletedMsg'))
  await loadData()
}

onMounted(async () => {
  await loadRuleOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page monitor-silence-page">
    <div class="page-header">
      <div>
        <h2>{{ mt('silenceRuleTitle') }}</h2>
        <p>{{ mt('silencePageDesc') }}</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">{{ mt('importCommonTemplates') }}</el-button><el-button type="primary" @click="openCreate">{{ mt('newSilenceRule') }}</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="mt('silenceSearchPlaceholder')" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable :placeholder="mt('status')" style="width: 120px">
        <el-option :label="mt('enabledOption')" value="1" />
        <el-option :label="mt('disabledOption')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ mt('searchLabel') }}</el-button>
    </div>

    <div v-if="selectedRuleIds.length" class="batch-toolbar">
      <span>{{ mt('selectedSilenceCount', { count: selectedRuleIds.length }) }}</span>
      <el-button size="small" type="success" @click="handleBatchAction('enable')">{{ mt('batchEnable') }}</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('disable')">{{ mt('batchDisable') }}</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">{{ mt('batchDelete') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" :label="mt('nameLabel')" min-width="180" />
      <el-table-column :label="mt('ruleMatching')" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">{{ mt('multiSelectColon', { names: selectedRuleNames(row.ruleIds) }) }}</span>
          <span v-else>{{ mt('regexColon', { pattern: row.ruleNamePattern || '-' }) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="Severity" width="90">
        <template #default="{ row }">{{ row.severity || mt('allShort') }}</template>
      </el-table-column>
      <el-table-column prop="alertType" :label="mt('alertTypeCol')" width="130"><template #default="{ row }">{{ ({ metric: mt('metricShort'), log: mt('esLogShort'), victorialogs: 'VictoriaLogs' }[row.alertType] || mt('allShort')) }}</template></el-table-column>
      <el-table-column prop="matchersJson" :label="mt('labelMatching')" min-width="220" show-overflow-tooltip />
      <el-table-column prop="startsAt" :label="mt('startAtLabel')" width="180" />
      <el-table-column prop="endsAt" :label="mt('endAtLabel')" width="180" />
      <el-table-column :label="mt('scheduleStatus')" width="100"><template #default="{ row }"><el-tag :type="silenceSchedule(row).type">{{ silenceSchedule(row).text }}</el-tag></template></el-table-column>
      <el-table-column :label="mt('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? mt('enabledOption') : mt('disabledOption') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="mt('actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ mt('edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ mt('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? mt('editSilenceRule') : mt('newSilenceRule')" width="820px">
      <el-form label-width="120px">
        <el-form-item :label="mt('nameLabel')" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="mt('ruleMatching')">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">{{ mt('dropdownMultiSelect') }}</el-radio-button>
            <el-radio-button label="regex">{{ mt('regexMatching') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="Alert Rule">
          <el-select v-model="form.ruleIds" multiple filterable clearable :placeholder="mt('ruleIdsPlaceholder')" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else :label="mt('ruleNamePatternLabel')" required>
          <el-input v-model="form.ruleNamePattern" :placeholder="mt('regexPatternPlaceholder')" />
        </el-form-item>
        <el-form-item label="Severity">
          <el-select v-model="form.severity" clearable :placeholder="mt('allSeverity')" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item :label="mt('alertTypeCol')"><el-select v-model="form.alertType" clearable :placeholder="mt('allTypes')" style="width: 100%"><el-option :label="mt('metricAlertType')" value="metric" /><el-option :label="mt('esLogAlertType')" value="log" /><el-option :label="mt('vlAlertType')" value="victorialogs" /></el-select></el-form-item>
        <el-form-item :label="mt('labelMatching')">
          <el-input v-model="form.matchersJson" type="textarea" :rows="3" :placeholder="mt('matchersPlaceholder')" />
        </el-form-item>
        <el-form-item :label="mt('silencePeriod')">
          <el-date-picker v-model="timeRange" type="datetimerange" :start-placeholder="mt('startAtLabel')" :end-placeholder="mt('endAtLabel')" value-format="YYYY-MM-DD HH:mm:ss" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="mt('priorityLabel')"><el-input-number v-model="form.priority" :min="1" :max="1000" /><span class="form-tip">{{ mt('priorityHint') }}</span></el-form-item>
        <el-form-item :label="mt('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ mt('enabledOption') }}</el-radio>
            <el-radio :value="2">{{ mt('disabledOption') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ mt('cancel') }}</el-button>
        <el-button :loading="previewLoading" @click="openPreview">{{ mt('matchPreview') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submit">{{ mt('save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewVisible" :title="mt('silencePreviewTitle')" width="760px">
      <div class="preview-summary"><el-tag type="primary">{{ mt('matchedRuleCountLabel', { count: previewData.matchedRuleCount || 0 }) }}</el-tag><el-tag type="warning">{{ mt('affectedEventCount', { count: previewData.matchedActiveEventCount || 0 }) }}</el-tag></div>
      <el-alert type="info" :closable="false" :title="mt('previewBasisNote')" />
      <h4>{{ mt('matchedRulesTitle') }}</h4>
      <el-table :data="previewData.matchedRules || []" max-height="220" size="small"><el-table-column prop="name" :label="mt('ruleName')" /><el-table-column prop="alertType" :label="mt('typeCol')" width="130" /><el-table-column prop="severity" label="Severity" width="90" /></el-table>
      <h4>{{ mt('affectedEventsTitle') }}</h4>
      <el-table :data="previewData.matchedActiveEvents || []" max-height="220" size="small"><el-table-column prop="ruleName" label="Rule" /><el-table-column prop="status" :label="mt('status')" width="100" /><el-table-column prop="summary" :label="mt('summaryCol')" show-overflow-tooltip /></el-table>
      <template #footer><el-button type="primary" @click="previewVisible = false">{{ mt('confirmTitle') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" :title="mt('importSilenceTemplatesTitle')" width="780px">
      <div class="template-head"><span>{{ mt('templateImportHint') }}</span><el-radio-group v-model="templateType"><el-radio-button label="metric">{{ mt('metricAlertType') }}</el-radio-button><el-radio-button label="elasticsearch">Elasticsearch</el-radio-button><el-radio-button label="victorialogs">VictoriaLogs</el-radio-button></el-radio-group></div>
      <div class="template-grid">
        <button v-for="item in visibleSilenceTemplates" :key="item.id" type="button" class="template-card" @click="applySilenceTemplate(item)">
           <el-tag :type="item.type === 'metric' ? 'primary' : item.type === 'elasticsearch' ? 'warning' : 'success'" size="small">{{ item.type === 'metric' ? mt('metricAlertType') : item.type === 'elasticsearch' ? 'Elasticsearch' : 'VictoriaLogs' }}</el-tag>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.matchMode === 'regex' ? item.ruleNamePattern : item.matchersJson }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">{{ mt('cancel') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.header-actions { display: flex; gap: 10px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.template-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; color: #7282a0; font-size: 13px; }
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.template-card { min-height: 138px; padding: 14px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.template-card strong { display: block; margin-top: 10px; color: #1d3154; }
.template-card p { min-height: 35px; margin: 6px 0; color: #7282a0; font-size: 13px; line-height: 1.4; }
.template-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.form-tip { margin-left: 10px; color: #8090ac; font-size: 12px; }
.preview-summary { display: flex; gap: 10px; margin-bottom: 14px; }
.preview-summary + .el-alert { margin-bottom: 14px; }
.el-dialog h4 { margin: 16px 0 8px; color: #20345b; }
</style>
