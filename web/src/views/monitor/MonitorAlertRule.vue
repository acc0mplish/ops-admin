<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { mt } from '../../utils/monitor-i18n'
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifyRuleOptions } from '../../api/ops'
import MonitorAlertRuleEditor from './alert-rule/MonitorAlertRuleEditor.vue'
import MonitorAlertRuleDialogs from './alert-rule/MonitorAlertRuleDialogs.vue'

// I5-I (i5-plan §3.8·CW-DB) — 1,037행 분할 잔류본. 룰 폼은 alert-rule/
// MonitorAlertRuleEditor.vue, 프리뷰/템플릿/일괄 다이얼로그는 ...Dialogs.vue로
// 이동(원본 좌표는 커밋 메시지). 자식은 :page 주입(§5 #11 승계).
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
const ruleDialogsRef = ref(null)

// 원본 :353-362 restoreTemplateSelection은 템플릿 테이블 ref가 자식
// (MonitorAlertRuleDialogs)에 귀속되어 위임 호출로 대체했다.
async function restoreTemplateSelection() {
  await ruleDialogsRef.value?.restoreTemplateSelection()
}
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
const enabledOnPage = computed(() => rows.value.filter((item) => item.status === 1).length)
const failedOnPage = computed(() => rows.value.filter((item) => item.lastEvalStatus === 'failed').length)

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



// I5-I 자식 주입 번들 — reactive 래핑으로 ref/computed가 언랩되어
// 자식 템플릿의 page.x / v-model="page.x" 재배선이 동작한다(§5 #11).
const page = reactive({
  dialogVisible,
  copyMode,
  isEdit,
  saving,
  form,
  previewVisible,
  previewLoading,
  previewError,
  previewResult,
  templateDialogVisible,
  templateKeyword,
  templateLoading,
  templateGroups,
  selectedTemplateGroupId,
  selectedTemplateMap,
  managedTemplates,
  templateImporting,
  batchNotifyVisible,
  batchNotifySaving,
  batchNotifyRuleId,
  batchNotifyRepeatIntervalMinutes,
  batchNotifyMaxCount,
  batchNotifyRecoveryEnabled,
  batchTimingVisible,
  batchTimingSaving,
  batchTimingAction,
  batchTimingValue,
  selectedRuleIds,
  queryDrafts,
  previousAlertType,
  notifyRuleOptions,
logDatasourceOptions,
availableDatasourceOptions,
  datasourceOptionsForType,
  alertTypeName,
applyDatasourceScope,
  applyRuleTemplate,
  openTemplateDialog,
  selectTemplateGroup,
  loadManagedTemplates,
  clearSelection,
  loadData
})

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

    <MonitorAlertRuleEditor :page="page" />

    <MonitorAlertRuleDialogs ref="ruleDialogsRef" :page="page" />
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
