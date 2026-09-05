<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { mt } from '../../utils/monitor-i18n'
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifyRuleOptions } from '../../api/ops'
import {
  batchUpdateMonitorAlertRules,
  deleteMonitorAlertRule,
  monitorAlertRuleInfo,
  monitorAlertTemplateInfo,
  previewMonitorAlertRule,
  queryMonitorAlertRuleList,
  queryMonitorAlertTemplateGroups,
  queryMonitorAlertTemplateList,
  queryMonitorDatasourceOptions,
  runMonitorAlertRule,
  saveMonitorAlertRule,
  updateMonitorAlertRuleStatus
} from '../../api/monitor'

const route = useRoute()

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const previewResult = ref({ results: [] })
const isEdit = ref(false)
const copyMode = ref(false)
const templateDialogVisible = ref(false)
const batchNotifyVisible = ref(false)
const batchNotifySaving = ref(false)
const batchNotifyRuleId = ref()
const batchNotifyRepeatIntervalMinutes = ref(30)
const batchNotifyMaxCount = ref(0)
const batchNotifyRecoveryEnabled = ref(true)
const batchTimingVisible = ref(false)
const batchTimingSaving = ref(false)
const batchTimingAction = ref('')
const batchTimingValue = ref(undefined)
const batchLoading = ref(false)
const runningRuleId = ref()
const ruleTableRef = ref()
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const datasourceOptions = ref([])
const notifyRuleOptions = ref([])
const managedTemplates = ref([])
const templateGroups = ref([])
const templateLoading = ref(false)
const templateImporting = ref(false)
const templateTableRef = ref()
const restoringTemplateSelection = ref(false)
const selectedTemplateMap = ref(new Map())
const templateKeyword = ref('')
const selectedTemplateGroupId = ref(0)
const previousAlertType = ref('metric')
const queryDrafts = reactive({ metric: '', log: '', victorialogs: '' })
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', status: '', severity: '', alertType: '' })
const form = reactive({
  id: undefined,
  name: '',
  alertType: 'metric',
  datasourceScope: 'specific',
  datasourceId: undefined,
  queryText: '',
  logIndex: '_all',
  logTimeRangeSeconds: 300,
  comparator: '>',
  threshold: 0,
  forSeconds: 0,
  evalIntervalSeconds: 60,
  notifyRepeatIntervalMinutes: 30,
  maxNotifyCount: 0,
  severity: 'P2',
  labelsJson: '{}',
  annotationsJson: '{}',
  notifyEnabled: false,
  notifyRuleId: undefined,
  notifyRecoveryEnabled: true,
  status: 2,
  description: ''
})

const metricDatasourceOptions = computed(() => datasourceOptions.value.filter((item) => ['prometheus', 'victoriametrics'].includes(item.type)))
const logDatasourceOptions = computed(() => datasourceOptions.value.filter((item) => item.type === 'elasticsearch'))
const victoriaLogsDatasourceOptions = computed(() => datasourceOptions.value.filter((item) => item.type === 'victorialogs'))
const availableDatasourceOptions = computed(() => {
  if (form.alertType === 'log') return logDatasourceOptions.value
  if (form.alertType === 'victorialogs') return victoriaLogsDatasourceOptions.value
  return metricDatasourceOptions.value
})
const alertTypeLabel = computed(() => alertTypeName(form.alertType))
const isLogAlert = computed(() => ['log', 'victorialogs'].includes(form.alertType))
const enabledOnPage = computed(() => rows.value.filter((item) => item.status === 1).length)
const failedOnPage = computed(() => rows.value.filter((item) => item.lastEvalStatus === 'failed').length)
const batchTimingTitle = computed(() => batchTimingAction.value === 'update_for_seconds' ? mt('batchDurationChange') : mt('batchEvalChange'))
const batchTimingLabel = computed(() => batchTimingAction.value === 'update_for_seconds' ? 'Duration' : uiT('evaluationInterval'))
const batchTimingTip = computed(() => batchTimingAction.value === 'update_for_seconds'
  ? mt('batchDurationTip', { count: selectedRuleIds.value.length })
  : mt('batchEvalTip', { count: selectedRuleIds.value.length }))
const batchTimingMin = computed(() => batchTimingAction.value === 'update_for_seconds' ? 0 : 15)
const batchTimingMax = computed(() => batchTimingAction.value === 'update_for_seconds' ? 86400 : 3600)
const templateGroupTree = computed(() => {
  const map = new Map(templateGroups.value.map((item) => [item.id, { ...item, children: [] }]))
  const roots = []
  map.forEach((item) => {
    const parent = map.get(item.parentId)
    if (parent) parent.children.push(item)
    else roots.push(item)
  })
  const decorate = (item) => {
    item.children = item.children.map(decorate)
    item.totalCount = Number(item.count || 0) + item.children.reduce((sum, child) => sum + child.totalCount, 0)
    return item
  }
  return roots.map(decorate)
})
const queryLabel = computed(() => {
  if (form.alertType === 'log') return 'Elasticsearch Query'
  if (form.alertType === 'victorialogs') return 'LogsQL Query'
  return 'PromQL'
})
const queryPlaceholder = computed(() => {
  if (form.alertType === 'log') return mt('logQueryPlaceholderEs')
  if (form.alertType === 'victorialogs') return mt('logQueryPlaceholderVl')
  return mt('logQueryPlaceholderProm')
})
const queryHint = computed(() => {
  if (form.alertType === 'log') return mt('queryHintEs')
  if (form.alertType === 'victorialogs') return mt('queryHintVl')
  return mt('queryHintProm')
})

function alertTypeName(type) {
  return ({ metric: 'Metric Alert', log: 'Elasticsearch Log Alert', victorialogs: 'VictoriaLogs Log Alert' }[type] || 'Metric Alert')
}

function alertTypeTag(type) {
  if (type === 'victorialogs') return 'success'
  return type === 'log' ? 'warning' : 'primary'
}

function evalStatusText(status) {
  return ({ success: mt('success'), ok: mt('success'), failed: mt('failed'), running: mt('running') }[status] || mt('notRun'))
}

function evalStatusType(status) {
  if (status === 'failed') return 'danger'
  if (status === 'success' || status === 'ok') return 'success'
  if (status === 'running') return 'warning'
  return 'info'
}

function formatEvalTime(value) {
  if (!value) return mt('notEvaluated')
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
const dialogTitle = computed(() => (copyMode.value ? mt('duplicateAlertRule') : (isEdit.value ? mt('editAlertRule') : mt('addAlertRule'))))

const visibleRuleTemplates = computed(() => managedTemplates.value)
const selectedManagedTemplates = computed(() => Array.from(selectedTemplateMap.value.values()))

function formatNotifyInterval(seconds) {
  const value = Number(seconds) || 1800
  if (value % 86400 === 0) return mt('durDay', { count: value / 86400 })
  if (value % 3600 === 0) return mt('durHour', { count: value / 3600 })
  if (value % 60 === 0) return mt('durMinute', { count: value / 60 })
  return mt('durSecond', { count: value })
}

function datasourceOptionsForType(type) {
  if (type === 'log') return logDatasourceOptions.value
  if (type === 'victorialogs') return victoriaLogsDatasourceOptions.value
  return metricDatasourceOptions.value
}

function templateGroupPath(id) {
  const byId = new Map(templateGroups.value.map((item) => [item.id, item]))
  const parts = []
  let item = byId.get(id)
  while (item) {
    parts.unshift(item.name)
    item = byId.get(item.parentId)
  }
  return parts.join(' / ') || mt('uncategorized')
}

function defaultQueryForType(type) {
  if (type === 'log') return 'level:ERROR OR level:FATAL'
  if (type === 'victorialogs') return '_msg:~"(?i)(error|fatal)"'
  return ''
}

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    alertType: 'metric',
    datasourceScope: 'specific',
    datasourceId: metricDatasourceOptions.value[0]?.id,
    queryText: '',
    logIndex: '_all',
    logTimeRangeSeconds: 300,
    comparator: '>',
    threshold: 0,
    forSeconds: 0,
    evalIntervalSeconds: 60,
    notifyRepeatIntervalMinutes: 30,
    maxNotifyCount: 0,
    severity: 'P2',
    labelsJson: '{}',
    annotationsJson: '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    status: 2,
    description: ''
  })
  previousAlertType.value = 'metric'
  Object.assign(queryDrafts, { metric: '', log: '', victorialogs: '' })
}

function handleAlertTypeChange(type) {
  queryDrafts[previousAlertType.value] = form.queryText
  form.queryText = queryDrafts[type] || defaultQueryForType(type)
  previousAlertType.value = type
  form.datasourceId = form.datasourceScope === 'specific' ? datasourceOptionsForType(type)[0]?.id : undefined
}

function applyDatasourceScope() {
  form.datasourceId = form.datasourceScope === 'specific' ? availableDatasourceOptions.value[0]?.id : undefined
}

async function loadOptions() {
  const [datasources, notifyRules] = await Promise.allSettled([
    queryMonitorDatasourceOptions(),
    queryNotifyRuleOptions({ scope: 'monitor' })
  ])
  datasourceOptions.value = datasources.status === 'fulfilled' ? (datasources.value || []) : []
  notifyRuleOptions.value = notifyRules.status === 'fulfilled' ? (notifyRules.value || []) : []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  copyMode.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  copyMode.value = false
  const data = await monitorAlertRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    alertType: data.alertType || 'metric',
    datasourceScope: data.datasourceScope || 'specific',
    queryText: data.promql || '',
    logIndex: data.logIndex || '_all',
    logTimeRangeSeconds: data.logTimeRangeSeconds || 300,
    notifyRepeatIntervalMinutes: Math.max(1, Math.round((data.notifyRepeatIntervalSeconds || 1800) / 60)),
    maxNotifyCount: data.maxNotifyCount || 0,
    labelsJson: data.labelsJson || '{}',
    annotationsJson: data.annotationsJson || '{}'
  })
  previousAlertType.value = form.alertType
  queryDrafts[form.alertType] = form.queryText
  dialogVisible.value = true
}

async function openCopy(row) {
  const data = await monitorAlertRuleInfo(row.id)
  isEdit.value = false
  copyMode.value = true
  Object.assign(form, {
    ...data,
    id: undefined,
    name: `${data.name} - Copy`,
    alertType: data.alertType || 'metric',
    datasourceScope: data.datasourceScope || 'specific',
    queryText: data.promql || '',
    logIndex: data.logIndex || '_all',
    logTimeRangeSeconds: data.logTimeRangeSeconds || 300,
    notifyRepeatIntervalMinutes: Math.max(1, Math.round((data.notifyRepeatIntervalSeconds || 1800) / 60)),
    maxNotifyCount: data.maxNotifyCount || 0,
    labelsJson: data.labelsJson || '{}',
    annotationsJson: data.annotationsJson || '{}'
  })
  form.status = 2
  previousAlertType.value = form.alertType
  queryDrafts[form.alertType] = form.queryText
  dialogVisible.value = true
}

async function loadManagedTemplates(groupId = selectedTemplateGroupId.value) {
  templateLoading.value = true
  try {
    const data = await queryMonitorAlertTemplateList({ pageNum: 1, pageSize: 200, keyword: templateKeyword.value, groupId: groupId || '' })
    managedTemplates.value = data.list || []
  } finally {
    await nextTick()
    await restoreTemplateSelection()
    templateLoading.value = false
  }
}

async function selectTemplateGroup(id = 0) {
  selectedTemplateGroupId.value = id
  await loadManagedTemplates(id)
}

async function openTemplateDialog() {
  templateKeyword.value = ''
  selectedTemplateGroupId.value = 0
  selectedTemplateMap.value = new Map()
  templateDialogVisible.value = true
  templateLoading.value = true
  try {
    const [groups] = await Promise.all([queryMonitorAlertTemplateGroups(), loadManagedTemplates(0)])
    templateGroups.value = groups || []
  } finally {
    templateLoading.value = false
  }
}

function templateAlertType(template) {
  if (template.datasourceType === 'victorialogs') return 'victorialogs'
  if (template.datasourceType === 'elasticsearch') return 'log'
  return 'metric'
}

function templateWithType(template) {
  return { ...template, type: templateAlertType(template) }
}

async function restoreTemplateSelection() {
  if (!templateTableRef.value) return
  restoringTemplateSelection.value = true
  templateTableRef.value.clearSelection()
  managedTemplates.value.forEach((item) => {
    if (selectedTemplateMap.value.has(item.id)) templateTableRef.value.toggleRowSelection(item, true)
  })
  await nextTick()
  restoringTemplateSelection.value = false
}

function handleTemplateSelectionChange(selection) {
  if (restoringTemplateSelection.value) return
  const next = new Map(selectedTemplateMap.value)
  managedTemplates.value.forEach((item) => next.delete(item.id))
  selection.forEach((item) => next.set(item.id, item))
  selectedTemplateMap.value = next
}

function clearTemplateSelection() {
  selectedTemplateMap.value = new Map()
  templateTableRef.value?.clearSelection()
}

function buildTemplateImportPayload(template) {
  const alertType = templateAlertType(template)
  const datasource = datasourceOptionsForType(alertType)[0]
  if (!datasource) throw new Error(mt('noDatasourceForTemplate', { name: template.name, type: alertTypeName(alertType) }))
  return {
    name: template.name,
    alertType,
    datasourceScope: 'specific',
    datasourceId: datasource.id,
    query: template.queryText,
    logIndex: template.logIndex || '_all',
    logTimeRangeSeconds: template.logTimeRangeSeconds || 300,
    comparator: template.comparator || '>',
    threshold: Number(template.threshold || 0),
    forSeconds: Number(template.forSeconds || 0),
    evalIntervalSeconds: Number(template.evalIntervalSeconds || 60),
    severity: template.severity || 'P2',
    labelsJson: template.labelsJson || '{}',
    annotationsJson: template.annotationsJson || '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    notifyRepeatIntervalSeconds: 1800,
    maxNotifyCount: 0,
    status: 2,
    description: template.description || ''
  }
}

async function importSelectedTemplates() {
  const templates = selectedManagedTemplates.value
  if (!templates.length) {
    ElMessage.warning(mt('selectTemplatesFirst'))
    return
  }
  const invalid = templates.filter((item) => !datasourceOptionsForType(templateAlertType(item))[0])
  if (invalid.length) {
    ElMessage.error(mt('templatesNoDatasource', { names: invalid.map((item) => item.name).join(', ') }))
    return
  }
  try {
    await ElMessageBox.confirm(
      mt('batchCreateConfirm', { count: templates.length }),
      mt('batchCreateTitle'),
      { type: 'warning', confirmButtonText: mt('batchCreateConfirmBtn', { count: templates.length }), cancelButtonText: mt('cancel') }
    )
  } catch {
    return
  }
  templateImporting.value = true
  const succeeded = []
  const failed = []
  try {
    for (const template of templates) {
      try {
        await saveMonitorAlertRule(buildTemplateImportPayload(template))
        succeeded.push(template)
      } catch (error) {
        failed.push({ template, message: error?.message || mt('createFailedShort') })
      }
    }
    if (succeeded.length) {
      ElMessage.success(mt('createdDisabledRules', { count: succeeded.length }))
      const remaining = new Map(selectedTemplateMap.value)
      succeeded.forEach((item) => remaining.delete(item.id))
      selectedTemplateMap.value = remaining
      await loadData()
    }
    if (failed.length) {
      await ElMessageBox.alert(
        failed.map((item) => `${item.template.name}: ${item.message}`).join('\n'),
        mt('failedToCreateRules', { count: failed.length }),
        { type: 'error', confirmButtonText: mt('backToReview') }
      )
      restoreTemplateSelection()
    } else {
      templateDialogVisible.value = false
    }
  } finally {
    templateImporting.value = false
  }
}

function applyRuleTemplate(template) {
  Object.assign(form, {
    id: undefined,
    name: template.name,
    alertType: template.type,
    datasourceScope: 'specific',
    datasourceId: datasourceOptionsForType(template.type)[0]?.id,
    queryText: template.queryText,
    logIndex: template.logIndex || '_all',
    logTimeRangeSeconds: template.logTimeRangeSeconds || 300,
    comparator: template.comparator,
    threshold: template.threshold,
    forSeconds: template.forSeconds || 0,
    evalIntervalSeconds: template.evalIntervalSeconds,
    notifyRepeatIntervalMinutes: 30,
    maxNotifyCount: 0,
    severity: template.severity,
    labelsJson: template.labelsJson || '{}',
    annotationsJson: template.annotationsJson || '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    status: 2,
    description: template.description
  })
  isEdit.value = false
  copyMode.value = false
  previousAlertType.value = template.type
  queryDrafts[template.type] = template.queryText
  templateDialogVisible.value = false
  dialogVisible.value = true
}

async function applyManagedTemplate(id) {
  if (!id) return
  const template = await monitorAlertTemplateInfo(id)
  applyRuleTemplate({
    ...template,
    type: ['elasticsearch', 'victorialogs'].includes(template.datasourceType) ? (template.datasourceType === 'victorialogs' ? 'victorialogs' : 'log') : 'metric',
    queryText: template.queryText
  })
}

function validateJsonFields() {
  for (const [label, value] of [['Label JSON', form.labelsJson], ['Annotation JSON', form.annotationsJson]]) {
    try {
      JSON.parse(value || '{}')
    } catch {
      ElMessage.warning(mt('invalidJsonFormat', { label }))
      return false
    }
  }
  return true
}

async function submit() {
  if (!form.name.trim() || !form.queryText.trim() || (form.datasourceScope === 'specific' && !form.datasourceId)) {
    ElMessage.warning(mt('enterRuleRequired', { datasource: form.datasourceScope === 'specific' ? mt('dsAnd') : '', query: form.alertType === 'log' ? mt('esQueryLabel') : mt('promqlLabel') }))
    return
  }
  if (form.notifyEnabled && !form.notifyRuleId) {
    ElMessage.warning(mt('selectNotifyRule'))
    return
  }
  if (!validateJsonFields()) return
  if (form.status === 1 && form.datasourceScope === 'all') {
    await ElMessageBox.confirm(mt('globalRuleConfirm'), mt('globalRuleTitle'), { type: 'warning', confirmButtonText: mt('enableConfirmBtn'), cancelButtonText: mt('backToReview') })
  }
  saving.value = true
  try {
    await saveMonitorAlertRule({
      ...form,
      query: form.queryText,
      notifyRepeatIntervalSeconds: form.notifyRepeatIntervalMinutes * 60
    })
    ElMessage.success(mt('savedMsg'))
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

function buildRulePayload() {
  return {
    ...form,
    query: form.queryText,
    notifyRepeatIntervalSeconds: form.notifyRepeatIntervalMinutes * 60
  }
}

async function handlePreview() {
  if (!form.queryText.trim() || (form.datasourceScope === 'specific' && !form.datasourceId)) {
    ElMessage.warning(mt('enterQueryFirst', { datasource: form.datasourceScope === 'specific' ? mt('dsAnd') : '', query: form.alertType === 'log' ? mt('esQueryLabel') : mt('promqlLabel') }))
    return
  }
  previewVisible.value = true
  previewLoading.value = true
  previewError.value = ''
  previewResult.value = { results: [] }
  try {
    previewResult.value = await previewMonitorAlertRule(buildRulePayload())
  } catch (error) {
    previewError.value = error?.message || mt('previewFailedMsg')
  } finally {
    previewLoading.value = false
  }
}

function sampleLabel(labels) {
  if (!labels || !Object.keys(labels).length) return '-'
  return Object.entries(labels).map(([key, value]) => `${key}=${value}`).join(', ')
}

async function handleStatus(row) {
  const nextStatus = row.status === 1 ? 2 : 1
  if (nextStatus === 2) {
    await ElMessageBox.confirm(mt('disableRuleConfirm', { name: row.name }), mt('disableRuleTitle'), { type: 'warning', confirmButtonText: mt('disableConfirmBtn'), cancelButtonText: mt('cancel') })
  }
  await updateMonitorAlertRuleStatus({ id: row.id, status: nextStatus })
  ElMessage.success(mt('statusUpdated'))
  await loadData()
}

async function handleRun(row) {
  runningRuleId.value = row.id
  try {
    await runMonitorAlertRule(row.id)
    ElMessage.success(mt('evalSubmitted'))
    await new Promise((resolve) => setTimeout(resolve, 800))
    await loadData()
  } finally {
    runningRuleId.value = undefined
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(mt('deleteRuleConfirm', { name: row.name }), mt('noticeTitle'), { type: 'warning' })
  await deleteMonitorAlertRule(row.id)
  ElMessage.success(mt('deletedMsg'))
  await loadData()
}

function handleSelectionChange(selection) {
  selectedRuleIds.value = selection.map((item) => item.id)
}

async function handleBatchStatus(status) {
  if (!selectedRuleIds.value.length) return
  const action = status === 1 ? 'enable' : 'disable'
  const label = status === 1 ? mt('enabledOption') : mt('disabledOption')
  await ElMessageBox.confirm(mt('batchStatusConfirm', { count: selectedRuleIds.value.length, action: label }), mt('batchStatusTitle'), { type: 'warning' })
  batchLoading.value = true
  try {
    await batchUpdateMonitorAlertRules({ ids: selectedRuleIds.value, action })
    ElMessage.success(mt('batchStatusDone', { action: label }))
    clearSelection()
    await loadData()
  } finally {
    batchLoading.value = false
  }
}

function clearSelection() {
  selectedRuleIds.value = []
  ruleTableRef.value?.clearSelection()
}

function openBatchNotify() {
  if (!selectedRuleIds.value.length) return
  batchNotifyRuleId.value = undefined
  batchNotifyRepeatIntervalMinutes.value = 30
  batchNotifyMaxCount.value = 0
  batchNotifyRecoveryEnabled.value = true
  batchNotifyVisible.value = true
}

function openBatchTiming(action) {
  if (!selectedRuleIds.value.length) return
  batchTimingAction.value = action
  batchTimingValue.value = undefined
  batchTimingVisible.value = true
}

function searchRules() {
  query.pageNum = 1
  loadData()
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, keyword: '', status: '', severity: '', alertType: '' })
  loadData()
}

function handleRuleCommand(command, row) {
  if (command === 'copy') openCopy(row)
  if (command === 'status') handleStatus(row)
  if (command === 'delete') handleDelete(row)
}

async function submitBatchNotify() {
  if (!batchNotifyRuleId.value) {
    ElMessage.warning(mt('selectNotifyRule'))
    return
  }
  if (batchNotifyRepeatIntervalMinutes.value < 1 || batchNotifyRepeatIntervalMinutes.value > 10080) {
    ElMessage.warning(mt('repeatRangeMsg'))
    return
  }
  if (batchNotifyMaxCount.value < 0 || batchNotifyMaxCount.value > 1000) {
    ElMessage.warning(mt('maxSendRangeMsg'))
    return
  }
  batchNotifySaving.value = true
  try {
    await batchUpdateMonitorAlertRules({
      ids: selectedRuleIds.value,
      action: 'enable_notify',
      notifyRuleId: batchNotifyRuleId.value,
      notifyRepeatIntervalSeconds: batchNotifyRepeatIntervalMinutes.value * 60,
      maxNotifyCount: batchNotifyMaxCount.value,
      notifyRecoveryEnabled: batchNotifyRecoveryEnabled.value
    })
    ElMessage.success(mt('batchNotifyDone'))
    batchNotifyVisible.value = false
    clearSelection()
    await loadData()
  } finally {
    batchNotifySaving.value = false
  }
}

async function submitBatchTiming() {
  if (batchTimingValue.value === undefined || batchTimingValue.value === null) {
    ElMessage.warning(mt('enterField', { label: batchTimingLabel.value }))
    return
  }
  batchTimingSaving.value = true
  try {
    const isDuration = batchTimingAction.value === 'update_for_seconds'
    await batchUpdateMonitorAlertRules({
      ids: selectedRuleIds.value,
      action: batchTimingAction.value,
      ...(isDuration ? { forSeconds: batchTimingValue.value } : { evalIntervalSeconds: batchTimingValue.value })
    })
    ElMessage.success(mt('batchChanged', { label: batchTimingLabel.value }))
    batchTimingVisible.value = false
    clearSelection()
    await loadData()
  } finally {
    batchTimingSaving.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadOptions(), loadData()])
  if (route.query.templateId) await applyManagedTemplate(route.query.templateId)
})
</script>

<template>
  <div class="monitor-page monitor-alert-rule-page">
    <section class="page-header">
      <div class="header-icon"><el-icon><Bell /></el-icon></div>
      <div class="header-copy">
        <h2>Alert Rule</h2>
        <p>{{ mt('alertRulePageDesc') }}</p>
      </div>
      <div class="header-metrics"><span>{{ mt('currentActiveOnPage') }} <b>{{ enabledOnPage }}</b></span><span>{{ mt('evalFailedOnPage') }} <b :class="{ danger: failedOnPage }">{{ failedOnPage }}</b></span></div>
      <div class="header-actions">
        <el-button @click="openTemplateDialog"><el-icon><CollectionTag /></el-icon>{{ mt('createFromTemplate') }}</el-button>
        <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon>{{ mt('addRule') }}</el-button>
      </div>
    </section>

    <section class="toolbar-card"><div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="mt('searchRulePlaceholder')" style="width: 260px" @keyup.enter="searchRules"><template #prefix><el-icon><Search /></el-icon></template></el-input>
      <el-select v-model="query.alertType" clearable placeholder="Alert Type" style="width: 130px">
        <el-option label="Metric Alert" value="metric" />
        <el-option label="Elasticsearch Log" value="log" />
        <el-option label="VictoriaLogs" value="victorialogs" />
      </el-select>
      <el-select v-model="query.severity" clearable :placeholder="uiT('severity')" style="width: 120px">
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-select v-model="query.status" clearable :placeholder="mt('status')" style="width: 120px">
        <el-option :label="mt('enabledOption')" value="1" />
        <el-option :label="mt('disabledOption')" value="2" />
      </el-select>
      <el-button type="primary" @click="searchRules">{{ mt('query') }}</el-button>
      <el-button @click="resetQuery">{{ mt('resetLabel') }}</el-button>
      <el-button class="refresh-button" @click="loadData"><el-icon><Refresh /></el-icon>Refresh</el-button>
    </div></section>

    <section class="rule-table-card">
      <div class="table-card-head"><div><strong>{{ mt('ruleList') }}</strong><span>{{ mt('ruleListHint', { count: total }) }}</span></div></div>

    <div v-if="selectedRuleIds.length" v-loading="batchLoading" class="batch-toolbar">
      <span>{{ mt('selectedRules', { count: selectedRuleIds.length }) }}</span>
      <el-button size="small" type="success" @click="handleBatchStatus(1)">{{ mt('batchEnable') }}</el-button>
      <el-button size="small" type="warning" @click="handleBatchStatus(2)">{{ mt('batchDisable') }}</el-button>
      <el-button size="small" type="primary" plain @click="openBatchNotify">{{ mt('batchEnableNotify') }}</el-button>
      <el-button size="small" type="primary" @click="openBatchTiming('update_for_seconds')">{{ mt('batchDurationChange') }}</el-button>
      <el-button size="small" type="info" plain @click="openBatchTiming('update_eval_interval')">{{ mt('batchEvalChange') }}</el-button>
      <el-button size="small" link @click="clearSelection">{{ mt('clearSelectionLabel') }}</el-button>
    </div>

    <el-table ref="ruleTableRef" v-loading="loading" :data="rows" class="rule-table" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column label="Rule" min-width="255">
        <template #default="{ row }"><div class="rule-name-cell"><strong>{{ row.name }}</strong><span><el-tag :type="alertTypeTag(row.alertType)" size="small" effect="plain">{{ alertTypeName(row.alertType) }}</el-tag></span></div></template>
      </el-table-column>
      <el-table-column label="Datasource Scope" min-width="180"><template #default="{ row }"><div class="scope-cell"><strong>{{ row.datasourceName || mt('allTypeDatasources') }}</strong><span>{{ row.datasourceScope === 'all' ? mt('autoMatchAll') : mt('specificDatasource') }}</span></div></template></el-table-column>
      <el-table-column label="Trigger Condition" min-width="320" show-overflow-tooltip><template #default="{ row }"><div class="condition-cell"><code>{{ row.promql }}</code><span>{{ row.comparator }} {{ row.threshold }} · {{ row.alertType === 'metric' ? mt('durationSeconds', { count: row.forSeconds || 0 }) : mt('windowSeconds', { count: row.logTimeRangeSeconds || 300 }) }}</span></div></template></el-table-column>
      <el-table-column :label="uiT('severity')" width="76"><template #default="{ row }"><el-tag :type="['P0','P1'].includes(row.severity) ? 'danger' : (row.severity === 'P2' ? 'warning' : 'info')" size="small">{{ row.severity }}</el-tag></template></el-table-column>
      <el-table-column :label="mt('evalStatusColumn')" min-width="155"><template #default="{ row }"><div class="eval-cell"><span><el-tag :type="evalStatusType(row.lastEvalStatus)" size="small" effect="light">{{ evalStatusText(row.lastEvalStatus) }}</el-tag><em>{{ mt('everySeconds', { count: row.evalIntervalSeconds }) }}</em></span><el-tooltip v-if="row.lastEvalMessage" :content="row.lastEvalMessage" placement="top"><small>{{ formatEvalTime(row.lastEvalAt) }}</small></el-tooltip><small v-else>{{ formatEvalTime(row.lastEvalAt) }}</small></div></template></el-table-column>
      <el-table-column :label="mt('status')" width="90" align="center"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'warning'" size="small" effect="light">{{ row.status === 1 ? mt('enabledOption') : mt('disabledOption') }}</el-tag></template></el-table-column>
      <el-table-column label="Notification" width="108"><template #default="{ row }"><div class="notify-cell"><el-tag class="notification-state" :class="row.notifyEnabled ? 'is-notify-enabled' : 'is-notify-disabled'" size="small" effect="plain">{{ row.notifyEnabled ? mt('activeShort') : mt('inactiveShort') }}</el-tag><span v-if="row.notifyEnabled">{{ formatNotifyInterval(row.notifyRepeatIntervalSeconds) }}</span></div></template></el-table-column>
      <el-table-column :label="mt('actions')" width="168" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :loading="runningRuleId === row.id" @click="handleRun(row)">{{ mt('runShort') }}</el-button>
          <el-button link type="primary" @click="openEdit(row)">{{ mt('edit') }}</el-button>
          <el-dropdown trigger="click" @command="(command) => handleRuleCommand(command, row)"><el-button link type="primary">{{ mt('moreLabel') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="copy">{{ mt('duplicateRule') }}</el-dropdown-item><el-dropdown-item command="status">{{ row.status === 1 ? mt('ruleDisable') : mt('ruleEnable') }}</el-dropdown-item><el-dropdown-item command="delete" divided class="danger-menu-item">{{ mt('deleteRuleShort') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>
    </section>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="min(1040px, 94vw)" top="4vh" class="rule-editor-dialog" destroy-on-close>
      <div class="editor-intro"><div><el-icon><Operation /></el-icon></div><span><strong>{{ mt('executionRuleSetup') }}</strong><small>{{ mt('editorIntroDesc') }}</small></span><el-button plain @click="openTemplateDialog">{{ mt('selectAlertTemplate') }}</el-button></div>
      <el-form label-position="top" class="rule-editor-form">
        <section class="form-section"><div class="section-heading"><span>01</span><div><strong>{{ mt('basicInfo') }}</strong><small>{{ mt('basicInfoDesc') }}</small></div></div>
        <div class="form-grid"><el-form-item :label="mt('ruleName')" required><el-input v-model="form.name" :placeholder="mt('ruleNamePlaceholder')" /></el-form-item><el-form-item :label="mt('ruleStatus')"><el-radio-group v-model="form.status"><el-radio-button :label="2">{{ mt('draftRadio') }}</el-radio-button><el-radio-button :label="1">{{ mt('enabledOption') }}</el-radio-button></el-radio-group></el-form-item></div>
        <el-form-item label="Alert Type" required>
          <el-radio-group v-model="form.alertType" @change="handleAlertTypeChange">
            <el-radio-button label="metric">Metric Alert / PromQL</el-radio-button>
            <el-radio-button label="log">Log Alert / Elasticsearch</el-radio-button>
            <el-radio-button label="victorialogs">Log Alert / VictoriaLogs</el-radio-button>
          </el-radio-group>
        </el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>02</span><div><strong>{{ mt('dsAndQuery') }}</strong><small>{{ mt('dsQueryDesc') }}</small></div></div>
        <el-form-item label="Datasource Scope" required>
          <div class="datasource-scope">
            <el-radio-group v-model="form.datasourceScope" @change="applyDatasourceScope">
              <el-radio-button label="all">{{ mt('matchAllDatasources', { type: alertTypeLabel }) }}</el-radio-button>
              <el-radio-button label="specific">{{ mt('specificDatasource') }}</el-radio-button>
            </el-radio-group>
            <el-select v-if="form.datasourceScope === 'specific'" v-model="form.datasourceId" filterable style="width: 360px" :placeholder="mt('selectDatasource')">
              <el-option v-for="item in availableDatasourceOptions" :key="item.id" :label="`${item.name} (${item.type})`" :value="item.id" />
            </el-select>
            <span v-else class="form-tip">{{ mt('runOnAllDatasources', { type: form.alertType === 'log' ? 'Elasticsearch' : 'Prometheus / VictoriaMetrics' }) }}</span>
          </div>
        </el-form-item>
        <el-form-item v-if="form.alertType === 'log'" label="Log Index" required><el-input v-model="form.logIndex" :placeholder="mt('logIndexPlaceholder')" /></el-form-item>
        <el-form-item :label="queryLabel" required>
          <el-input v-model="form.queryText" type="textarea" :rows="4" :placeholder="queryPlaceholder" />
          <div class="form-tip">{{ queryHint }}</div>
        </el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>03</span><div><strong>Trigger Condition</strong><small>{{ mt('triggerConditionDesc') }}</small></div></div>
        <section class="rule-parameters">
          <div class="parameter-card">
            <span>Comparator</span>
            <el-select v-model="form.comparator"><el-option v-for="item in ['>','>=','<','<=','==','!=']" :key="item" :label="item" :value="item" /></el-select>
            <small>{{ mt('compareThreshold') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ isLogAlert ? 'Match Threshold' : 'Threshold' }}</span>
            <el-input-number v-model="form.threshold" :precision="4" :step="isLogAlert ? 1 : 0.1" controls-position="right" />
            <small>{{ isLogAlert ? mt('windowMatchCount') : mt('seriesCurrentValue') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ isLogAlert ? 'Query Window' : 'Duration' }}</span>
            <div class="parameter-number"><el-input-number v-if="isLogAlert" v-model="form.logTimeRangeSeconds" :min="60" :max="86400" controls-position="right" /><el-input-number v-else v-model="form.forSeconds" :min="0" :max="86400" controls-position="right" /><b>{{ mt('secondUnit') }}</b></div>
            <small>{{ isLogAlert ? mt('recentLogAgg') : mt('consecutiveMatchTrigger') }}</small>
          </div>
          <div class="parameter-card">
            <span>{{ uiT('evaluationInterval') }}</span>
            <div class="parameter-number"><el-input-number v-model="form.evalIntervalSeconds" :min="15" :max="3600" controls-position="right" /><b>{{ mt('secondUnit') }}</b></div>
            <small>{{ mt('systemRuleInterval') }}</small>
          </div>
        </section>
        <el-form-item label="Alert Severity"><el-radio-group v-model="form.severity"><el-radio-button v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" /></el-radio-group></el-form-item></section>

        <section class="form-section"><div class="section-heading"><span>04</span><div><strong>{{ mt('notifyAndContext') }}</strong><small>{{ mt('notifyContextDesc') }}</small></div></div>
        <div class="json-grid"><el-form-item label="Label JSON"><el-input v-model="form.labelsJson" type="textarea" :rows="2" :placeholder="mt('labelJsonPlaceholder')" /></el-form-item><el-form-item label="Annotation JSON"><el-input v-model="form.annotationsJson" type="textarea" :rows="2" :placeholder="mt('annotationJsonPlaceholder')" /></el-form-item></div>
        <div class="notify-toggle" :class="{ 'is-enabled': form.notifyEnabled }">
          <div class="notify-toggle-copy"><div class="notify-toggle-icon"><el-icon><Bell /></el-icon></div><span><strong>Notification</strong><small>{{ form.notifyEnabled ? mt('notifyOnDesc') : mt('notifyOffDesc') }}</small></span></div>
          <div class="notify-toggle-action"><el-tag :type="form.notifyEnabled ? 'success' : 'info'" effect="light">{{ form.notifyEnabled ? mt('activeShort') : mt('inactiveShort') }}</el-tag><el-switch v-model="form.notifyEnabled" /></div>
        </div>
        <div v-if="form.notifyEnabled" class="notify-settings"><el-form-item label="NotificationRule" required><el-select v-model="form.notifyRuleId" filterable style="width: 100%"><el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item><div class="form-grid"><el-form-item label="Repeat Notification Interval"><div class="parameter-number"><el-input-number v-model="form.notifyRepeatIntervalMinutes" :min="1" :max="10080" controls-position="right" /><b>{{ mt('minuteUnit') }}</b></div></el-form-item><el-form-item label="Maximum Send Count"><el-input-number v-model="form.maxNotifyCount" :min="0" :max="1000" controls-position="right" /><div class="form-tip">{{ mt('zeroMeansUnlimited') }}</div></el-form-item></div><el-form-item label="Recovery Notification"><el-switch v-model="form.notifyRecoveryEnabled" /></el-form-item></div>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="form.description" type="textarea" :rows="2" :placeholder="mt('descriptionPlaceholder')" /></el-form-item></section>
      </el-form>
      <template #footer><div class="rule-editor-footer"><span>{{ mt('testFirstHint') }}</span><div><el-button @click="dialogVisible = false">{{ mt('cancel') }}</el-button><el-button :loading="previewLoading" @click="handlePreview">Rule Test</el-button><el-button type="primary" :loading="saving" @click="submit">{{ mt('saveRule') }}</el-button></div></div></template>
    </el-dialog>

    <el-dialog v-model="previewVisible" title="Rule Execution Preview" width="920px" append-to-body>
      <div v-loading="previewLoading" class="preview-body">
        <el-alert v-if="previewError" :title="previewError" type="error" :closable="false" show-icon />
        <el-alert v-else :title="previewResult.explanation || mt('previewRunning')" :type="previewResult.failedDatasourceCount ? 'warning' : 'success'" :closable="false" show-icon />
        <div class="preview-metrics">
          <div><span>Datasource</span><strong>{{ previewResult.datasourceCount || 0 }}</strong></div>
          <div><span>{{ mt('returnedSeries') }}</span><strong>{{ previewResult.totalSeries || 0 }}</strong></div>
          <div><span>{{ mt('expectedAlerts') }}</span><strong class="matched">{{ previewResult.totalMatched || 0 }}</strong></div>
          <div><span>{{ mt('stateQueryFailed') }}</span><strong>{{ previewResult.failedDatasourceCount || 0 }}</strong></div>
        </div>
        <div v-for="item in previewResult.results || []" :key="item.datasourceId" class="preview-source">
          <div class="preview-source-head">
            <strong>{{ item.datasourceName }}</strong>
            <el-tag :type="item.status === 'success' ? 'success' : 'danger'">{{ item.status === 'success' ? mt('querySuccess') : mt('stateQueryFailed') }}</el-tag>
          </div>
          <el-alert v-if="item.error" :title="item.error" type="error" :closable="false" />
          <el-table v-else :data="item.samples || []" border max-height="300" :empty-text="mt('noSeriesReturned')">
            <el-table-column :label="mt('triggeredColumn')" width="100"><template #default="{ row }"><el-tag :type="row.matched ? 'danger' : 'success'">{{ row.matched ? mt('conditionMet') : mt('notFired') }}</el-tag></template></el-table-column>
            <el-table-column :label="mt('currentValue')" width="140"><template #default="{ row }">{{ Number(row.value || 0).toFixed(4) }}</template></el-table-column>
            <el-table-column label="Label" min-width="480" show-overflow-tooltip><template #default="{ row }">{{ sampleLabel(row.labels) }}</template></el-table-column>
          </el-table>
        </div>
      </div>
      <template #footer><el-button type="primary" @click="previewVisible = false">{{ mt('inactiveShort') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" :title="mt('batchCreateFromTemplates')" width="1120px" top="5vh" class="managed-template-dialog" destroy-on-close>
      <div class="template-dialog-head"><div><strong>Alert Template Library</strong><span>{{ mt('templateDialogDesc') }}</span></div><el-input v-model="templateKeyword" clearable :placeholder="mt('searchTemplatePlaceholder')" style="width: 300px" @keyup.enter="loadManagedTemplates()"><template #prefix><el-icon><Search /></el-icon></template><template #append><el-button :aria-label="mt('searchTemplate')" @click="loadManagedTemplates()"><el-icon><Search /></el-icon></el-button></template></el-input></div>
      <div v-loading="templateLoading" class="template-library-layout">
        <aside class="template-groups"><button :class="{ active: !selectedTemplateGroupId }" @click="selectTemplateGroup()"><el-icon><FolderOpened /></el-icon><span>{{ mt('allTemplates') }}</span></button><el-tree :data="templateGroupTree" node-key="id" :default-expand-all="true" :highlight-current="true" :current-node-key="selectedTemplateGroupId" @node-click="(data) => selectTemplateGroup(data.id)"><template #default="{ data }"><span class="template-group-node"><span>{{ data.name }}</span><small>{{ data.totalCount }}</small></span></template></el-tree></aside>
        <div class="rule-template-list">
          <el-table ref="templateTableRef" :data="visibleRuleTemplates" row-key="id" height="520" class="template-select-table" @selection-change="handleTemplateSelectionChange">
            <el-table-column type="selection" width="48" reserve-selection />
            <el-table-column :label="mt('templateName')" min-width="230">
              <template #default="{ row }"><div class="template-name-cell"><strong>{{ row.name }}</strong><span>{{ row.description || mt('templateFallbackDesc') }}</span></div></template>
            </el-table-column>
            <el-table-column label="Template Group" min-width="170"><template #default="{ row }"><el-tag type="primary" size="small" effect="plain">{{ templateGroupPath(row.groupId) }}</el-tag></template></el-table-column>
            <el-table-column :label="uiT('severity')" width="76"><template #default="{ row }"><el-tag :type="['P0','P1'].includes(row.severity) ? 'danger' : (row.severity === 'P2' ? 'warning' : 'info')" effect="light" size="small">{{ row.severity }}</el-tag></template></el-table-column>
            <el-table-column label="Datasource" width="120"><template #default="{ row }"><span class="template-datasource">{{ row.datasourceType === 'victorialogs' ? 'VictoriaLogs' : (row.datasourceType === 'elasticsearch' ? 'Elasticsearch' : 'Prometheus') }}</span></template></el-table-column>
            <el-table-column :label="mt('searchTemplate')" min-width="260" show-overflow-tooltip><template #default="{ row }"><code class="template-query">{{ row.queryText }}</code></template></el-table-column>
            <el-table-column :label="mt('actions')" width="86" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="applyRuleTemplate(templateWithType(row))">{{ mt('configureIndividual') }}</el-button></template></el-table-column>
            <template #empty><el-empty :description="mt('noMatchingTemplates')" /></template>
          </el-table>
        </div>
      </div>
      <template #footer>
        <div class="template-dialog-footer">
          <div class="template-selection-summary"><span>{{ mt('selectedTemplates', { count: selectedManagedTemplates.length }) }}</span><el-button v-if="selectedManagedTemplates.length" link type="primary" @click="clearTemplateSelection">{{ mt('resetSelection') }}</el-button></div>
          <div><el-button @click="templateDialogVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="templateImporting" :disabled="!selectedManagedTemplates.length" @click="importSelectedTemplates">{{ mt('batchCreateRules') }}<span v-if="selectedManagedTemplates.length"> ({{ selectedManagedTemplates.length }})</span></el-button></div>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="batchNotifyVisible" :title="mt('batchEnableNotify')" width="640px" append-to-body>
      <p class="batch-dialog-tip">{{ mt('batchNotifyTip', { count: selectedRuleIds.length }) }}</p>
      <el-form label-position="top" class="batch-notify-form">
        <el-form-item label="NotificationRule" required>
          <el-select v-model="batchNotifyRuleId" filterable style="width: 100%" :placeholder="mt('selectNotifyRule')">
            <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <div class="batch-notify-grid">
          <el-form-item label="Repeat Notification Interval" required>
            <div class="batch-number-input">
              <el-input-number v-model="batchNotifyRepeatIntervalMinutes" :min="1" :max="10080" :step="5" controls-position="right" />
              <span>{{ mt('minuteUnit') }}</span>
            </div>
          </el-form-item>
          <el-form-item label="Maximum Send Count" required>
            <div class="batch-number-input">
              <el-input-number v-model="batchNotifyMaxCount" :min="0" :max="1000" controls-position="right" />
              <span>{{ mt('zeroMeansUnlimitedShort') }}</span>
            </div>
          </el-form-item>
        </div>
        <el-form-item class="batch-recovery-field" label="Recovery Notification">
          <div class="batch-recovery-control">
            <el-switch v-model="batchNotifyRecoveryEnabled" :active-text="mt('activeShort')" :inactive-text="mt('inactiveShort')" />
            <span>{{ mt('recoverySendDesc') }}</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="batchNotifyVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="batchNotifySaving" @click="submitBatchNotify">{{ mt('enableConfirmBtn') }}</el-button></template>
    </el-dialog>

    <el-dialog v-model="batchTimingVisible" :title="batchTimingTitle" width="520px" append-to-body>
      <p class="batch-dialog-tip">{{ batchTimingTip }}</p>
      <el-form label-width="92px">
        <el-form-item :label="batchTimingLabel" required>
          <div class="batch-timing-input">
            <el-input-number v-model="batchTimingValue" :min="batchTimingMin" :max="batchTimingMax" :step="batchTimingAction === 'update_for_seconds' ? 30 : 15" controls-position="right" />
            <span>{{ mt('secondUnit') }}</span>
          </div>
        </el-form-item>
        <div class="batch-dialog-hint">{{ batchTimingAction === 'update_for_seconds' ? mt('timingDurationHint') : mt('timingEvalHint') }}</div>
      </el-form>
      <template #footer><el-button @click="batchTimingVisible = false">{{ mt('cancel') }}</el-button><el-button type="primary" :loading="batchTimingSaving" @click="submitBatchTiming">{{ mt('saveChanges') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 16px; color: #172b4d; }
.page-header { display: flex; align-items: center; gap: 14px; min-height: 90px; padding: 16px 20px; border: 1px solid #dce8fa; border-radius: 16px; background: linear-gradient(108deg, #fff 4%, #f6f9ff 74%, #eef7ff); box-shadow: 0 10px 24px rgba(43, 74, 128, .06); }
.header-icon { display: grid; flex: none; width: 44px; height: 44px; place-items: center; border-radius: 12px; background: #e8f1ff; color: #3477df; font-size: 23px; }.header-copy { min-width: 0; }.page-header h2 { margin: 0 0 5px; font-size: 24px; color: #102747; }.page-header p { margin: 0; color: #7184a3; font-size: 13px; }.header-metrics { display: flex; gap: 18px; margin-left: auto; color: #7587a3; font-size: 12px; }.header-metrics b { margin-left: 4px; color: #18375f; font-size: 18px; }.header-metrics b.danger { color: #d84851; }.header-actions { display: flex; gap: 8px; margin-left: 10px; }
.toolbar-card, .rule-table-card { border: 1px solid #e2eaf6; border-radius: 14px; background: #fff; box-shadow: 0 10px 24px rgba(36, 54, 90, .045); }.toolbar { display: flex; flex-wrap: wrap; gap: 10px; padding: 14px 16px; }.refresh-button { margin-left: auto; }.rule-table-card { overflow: hidden; }.table-card-head { display: flex; align-items: center; min-height: 54px; padding: 0 16px; border-bottom: 1px solid #e8eef6; }.table-card-head strong, .table-card-head span { display: block; }.table-card-head strong { color: #1c365b; font-size: 15px; }.table-card-head span { margin-top: 3px; color: #8392a9; font-size: 12px; }
.batch-toolbar { display: flex; align-items: center; gap: 10px; min-height: 48px; padding: 8px 16px; border-bottom: 1px solid #d7e4fa; background: #f2f6ff; color: #52637f; }.batch-toolbar b { color: #3266d6; }.rule-table :deep(.el-table__header th) { height: 46px; background: #f5f8fc; color: #526681; font-size: 12px; }.rule-table :deep(.el-table__row td) { padding: 11px 0; }.rule-name-cell, .scope-cell, .condition-cell, .eval-cell, .notify-cell { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.rule-name-cell strong, .scope-cell strong { overflow: hidden; color: #1d365b; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }.rule-name-cell span, .eval-cell span { display: flex; align-items: center; gap: 5px; }.rule-name-cell em, .eval-cell em { color: #8795aa; font-size: 11px; font-style: normal; }.scope-cell span, .condition-cell span, .notify-cell span, .eval-cell small { color: #8492a8; font-size: 11px; }.condition-cell code { overflow: hidden; color: #45628f; font: 12px/1.4 Consolas, Monaco, monospace; text-overflow: ellipsis; white-space: nowrap; }.notify-cell { align-items: flex-start; }.pager { display: flex; justify-content: flex-end; padding: 14px 16px; }
:global(.rule-editor-dialog .el-dialog__body) { max-height: calc(100vh - 188px); overflow: auto; padding-top: 14px; }:global(.rule-editor-dialog .el-dialog__footer) { margin-top: 0; padding-top: 12px; border-top: 1px solid #edf1f7; }.editor-intro { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; padding: 13px 14px; border: 1px solid #dce8fa; border-radius: 10px; background: #f3f7ff; }.editor-intro > div { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 9px; background: #e2edff; color: #3872d6; }.editor-intro > span { flex: 1; }.editor-intro strong, .editor-intro small { display: block; }.editor-intro strong { color: #234369; }.editor-intro small { margin-top: 3px; color: #7588a6; }.rule-editor-form { display: flex; flex-direction: column; gap: 14px; }.form-section { padding: 16px; border: 1px solid #e0e8f4; border-radius: 12px; background: #fff; }.section-heading { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }.section-heading > span { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 8px; background: #e9f1ff; color: #3470d7; font-size: 12px; font-weight: 700; }.section-heading strong, .section-heading small { display: block; }.section-heading strong { color: #1e385e; }.section-heading small { margin-top: 2px; color: #8492a8; font-size: 12px; }.form-grid, .json-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }.form-section :deep(.el-form-item:last-child) { margin-bottom: 0; }.datasource-scope { display: flex; align-items: center; gap: 12px; width: 100%; flex-wrap: wrap; }.form-tip { margin-top: 6px; color: #8491a9; font-size: 12px; line-height: 1.55; }.datasource-scope .form-tip { margin: 0; }.rule-parameters { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-bottom: 16px; }.parameter-card { min-height: 126px; padding: 14px; border: 1px solid #dfe7f2; border-radius: 10px; background: #f8faff; display: flex; flex-direction: column; gap: 9px; }.parameter-card > span { color: #51617b; font-size: 13px; font-weight: 700; }.parameter-card :deep(.el-input-number), .parameter-card :deep(.el-select) { width: 100%; }.parameter-card small { color: #8c98ad; font-size: 12px; line-height: 1.4; }.parameter-number { display: flex; align-items: center; gap: 6px; }.parameter-number :deep(.el-input-number) { flex: 1; min-width: 0; }.parameter-number b { color: #71809b; font-size: 12px; font-weight: 500; }.notify-toggle { display: flex; align-items: center; justify-content: space-between; min-height: 58px; padding: 10px 14px; border: 1px solid #e1e9f5; border-radius: 9px; background: #f8faff; }.notify-toggle strong, .notify-toggle small { display: block; }.notify-toggle small { margin-top: 3px; color: #8492a8; }.notify-settings { margin-top: 12px; padding: 14px; border: 1px solid #dbe7f8; border-radius: 10px; background: #f8fbff; }.rule-editor-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; }.rule-editor-footer > span { color: #8190a5; font-size: 12px; }
.preview-body { display: flex; flex-direction: column; gap: 14px; min-height: 220px; }.preview-metrics { display: grid; grid-template-columns: repeat(4, 1fr); border: 1px solid #e1e8f3; border-radius: 8px; background: #f7f9fd; }.preview-metrics > div { padding: 13px 16px; border-right: 1px solid #e1e8f3; }.preview-metrics > div:last-child { border-right: 0; }.preview-metrics span, .preview-metrics strong { display: block; }.preview-metrics span { margin-bottom: 4px; color: #8491a8; font-size: 12px; }.preview-metrics strong { color: #20395f; font-size: 20px; }.preview-metrics strong.matched { color: #dc3f48; }.preview-source { padding: 14px; border: 1px solid #e1e8f3; border-radius: 8px; }.preview-source-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; color: #20395f; }
.template-dialog-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 14px; }.template-dialog-head strong, .template-dialog-head span { display: block; }.template-dialog-head strong { color: #1e3155; }.template-dialog-head span { margin-top: 5px; color: #8491a9; font-size: 12px; }.template-library-layout { display: grid; grid-template-columns: 220px minmax(0, 1fr); min-height: 520px; overflow: hidden; border: 1px solid #e1e9f5; border-radius: 12px; background: #fff; }.template-groups { padding: 12px; border-right: 1px solid #e1e9f5; background: #f8faff; }.template-groups > button { display: flex; align-items: center; width: 100%; gap: 8px; min-height: 36px; padding: 8px 10px; border: 0; border-radius: 7px; background: transparent; color: #506785; cursor: pointer; transition: background-color .18s, color .18s; }.template-groups > button:hover { background: #eef4ff; }.template-groups > button.active { background: #e5efff; color: #2769d8; font-weight: 700; }.template-groups > button span { flex: 1; text-align: left; }.template-groups :deep(.el-tree) { margin-top: 8px; background: transparent; }.template-groups :deep(.el-tree-node__content) { height: 34px; border-radius: 7px; }.template-group-node { display: flex; width: 100%; min-width: 0; gap: 6px; padding-right: 6px; }.template-group-node > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.template-group-node small { margin-left: auto; color: #8a99af; }.rule-template-list { min-width: 0; padding: 12px; background: #fff; }.template-select-table { overflow: hidden; border: 1px solid #e4ebf5; border-radius: 10px; }.template-select-table :deep(.el-table__header th) { height: 44px; background: #f5f8fc; color: #526681; font-size: 12px; }.template-select-table :deep(.el-table__row td) { padding: 10px 0; }.template-select-table :deep(.el-table__row:hover > td) { background: #f7faff; }.template-name-cell { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.template-name-cell strong { overflow: hidden; color: #1b3559; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }.template-name-cell span { overflow: hidden; color: #8492a8; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.template-datasource { color: #536b8b; font-size: 12px; }.template-query { display: block; overflow: hidden; color: #4264c2; font: 12px/1.45 Consolas, Monaco, monospace; text-overflow: ellipsis; white-space: nowrap; }.template-dialog-footer { display: flex; align-items: center; justify-content: space-between; width: 100%; gap: 16px; }.template-dialog-footer > div:last-child { display: flex; gap: 8px; }.template-selection-summary { display: flex; align-items: center; gap: 10px; color: #71819a; font-size: 13px; }.template-selection-summary b { color: #2f67d8; font-size: 16px; }.batch-dialog-tip { margin: 0 0 18px; color: #7282a0; line-height: 1.65; }
@media (max-width: 1000px) { .header-metrics { display: none; }.toolbar { align-items: flex-start; }.refresh-button { margin-left: 0; }.template-library-layout { grid-template-columns: 1fr; }.template-groups { max-height: 190px; overflow: auto; border-right: 0; border-bottom: 1px solid #e1e9f5; }.template-dialog-head { align-items: flex-start; flex-direction: column; }.template-dialog-head :deep(.el-input) { width: 100% !important; } }
@media (max-width: 700px) { .page-header { align-items: flex-start; flex-wrap: wrap; }.header-actions { width: 100%; margin-left: 0; }.form-grid, .json-grid, .rule-parameters, .preview-metrics { grid-template-columns: 1fr; }.rule-editor-footer, .template-dialog-footer { align-items: flex-end; flex-direction: column; gap: 10px; }.rule-editor-footer > span, .template-selection-summary { align-self: flex-start; } }
/* Rule editor refinements: keep the trigger controls compact and scannable. */
.rule-editor-dialog .rule-parameters {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin: 4px 0 18px;
}
.rule-editor-dialog .parameter-card {
  min-height: 150px;
  padding: 16px;
  gap: 12px;
  border-color: #f1d39c;
  border-radius: 14px;
  background: #fffdf8;
  box-shadow: 0 8px 22px rgba(181, 129, 39, .06);
}
.rule-editor-dialog .parameter-card > span { color: #425675; font-size: 14px; }
.rule-editor-dialog .parameter-card small { margin-top: auto; color: #7184a3; }
.rule-editor-dialog .notify-toggle {
  gap: 16px;
  min-height: 74px;
  margin: 2px 0 14px;
  padding: 14px 16px;
  border-color: #cfdced;
  border-radius: 12px;
  background: #f7faff;
  transition: border-color .18s ease, background-color .18s ease, box-shadow .18s ease;
}
.rule-editor-dialog .notify-toggle.is-enabled {
  border-color: #96c1f5;
  background: linear-gradient(100deg, #eff7ff 0%, #f8fbff 100%);
  box-shadow: 0 8px 20px rgba(58, 124, 207, .10);
}
.notify-toggle-copy, .notify-toggle-action { display: flex; align-items: center; }
.notify-toggle-copy { min-width: 0; gap: 12px; }
.notify-toggle-copy > span { min-width: 0; }
.notify-toggle-icon { display: grid; flex: none; width: 38px; height: 38px; place-items: center; border-radius: 10px; background: #e8f1ff; color: #3477df; font-size: 18px; }
.notify-toggle.is-enabled .notify-toggle-icon { background: #dff1e7; color: #2c9b62; }
.notify-toggle strong { color: #244267; font-size: 14px; }
.notify-toggle small { line-height: 1.5; }
.notify-toggle-action { flex: none; gap: 12px; }
.rule-editor-dialog .notify-settings { margin: -2px 0 18px; padding: 16px; border-color: #bcd7f6; border-radius: 12px; background: #f7fbff; box-shadow: inset 3px 0 0 #6aa8f8; }
.batch-timing-input { display: flex; align-items: center; gap: 8px; }
.batch-timing-input :deep(.el-input-number) { width: 220px; }
.batch-timing-input span { color: #60718c; font-size: 13px; }
.batch-dialog-hint { margin: -6px 0 0 92px; color: #8a98ad; font-size: 12px; line-height: 1.55; }
.batch-notify-form :deep(.el-form-item) { margin-bottom: 16px; }
.batch-notify-form :deep(.el-form-item__label) { color: #465a78; font-weight: 600; }
.batch-notify-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.batch-number-input { display: flex; align-items: center; gap: 8px; }
.batch-number-input :deep(.el-input-number) { width: 170px; }
.batch-number-input span { color: #72829a; font-size: 12px; white-space: nowrap; }
.batch-recovery-field { margin-bottom: 0 !important; padding: 13px 14px; border: 1px solid #dce7f7; border-radius: 9px; background: #f8fbff; }
.batch-recovery-field :deep(.el-form-item__label) { margin-bottom: 8px; }
.batch-recovery-control { display: flex; align-items: center; gap: 12px; }
.batch-recovery-control span { color: #7587a2; font-size: 12px; }
.rule-table :deep(.notification-state.is-notify-enabled) { color: #23834b !important; border-color: #9bd7ad !important; background: #ecf9f0 !important; font-weight: 600; }
.rule-table :deep(.notification-state.is-notify-disabled) { color: #b7791f !important; border-color: #edcf8a !important; background: #fff8e8 !important; font-weight: 600; }
@media (max-width: 980px) {
  .rule-editor-dialog .rule-parameters { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 700px) {
  .rule-editor-dialog .rule-parameters { grid-template-columns: 1fr; }
  .rule-editor-dialog .notify-toggle { align-items: flex-start; flex-direction: column; }
  .notify-toggle-action { width: 100%; justify-content: space-between; }
  .batch-notify-grid { grid-template-columns: 1fr; gap: 0; }
}
</style>
