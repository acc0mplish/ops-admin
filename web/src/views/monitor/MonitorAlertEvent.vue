<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchUpdateMonitorAlertEvents, claimMonitorAlertEvent, queryMonitorAlertEventDetail, queryMonitorAlertEventList, resolveMonitorAlertEvent, triggerMonitorAlertAction } from '../../api/monitor'
import { queryOpsJobList, queryOpsScriptList } from '../../api/ops'
import { mt } from '../../utils/monitor-i18n'

const loading = ref(false)
const route = useRoute()
const rows = ref([])
const total = ref(0)
const jobs = ref([])
const scripts = ref([])
const actionVisible = ref(false)
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref({ event: {}, timelines: [], actions: [], notifyLogs: [], notificationState: {} })
const current = ref({})
const selectedEventIds = ref([])
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '', status: '', severity: '' })
const actionForm = reactive({ actionType: 'job', targetId: undefined, operator: mt('systemAdmin'), summary: '' })

function statusType(status) {
  if (status === 'pending') return 'warning'
  if (status === 'firing') return 'danger'
  if (status === 'claimed') return 'warning'
  if (status === 'silenced') return 'info'
  if (status === 'recovered') return 'success'
  return 'info'
}

function statusText(status) {
  return ({ pending: mt('evPending'), firing: mt('evFiring'), claimed: mt('evClaimed'), silenced: mt('evSilenced'), recovered: mt('evRecovered'), resolved: mt('evResolved') }[status] || status)
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const pad = (item) => String(item).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function formatTriggerTime(value) {
  const formatted = formatTime(value)
  if (formatted === '-') return { date: '-', time: '' }
  const [date, time] = formatted.split(' ')
  return { date, time }
}

function prettyJSON(value) {
  try {
    return JSON.stringify(JSON.parse(value || '{}'), null, 2)
  } catch {
    return value || '{}'
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertEventList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadActionOptions() {
  const [jobData, scriptData] = await Promise.all([
    queryOpsJobList({ pageNum: 1, pageSize: 100, status: 1 }),
    queryOpsScriptList({ pageNum: 1, pageSize: 100, status: 1 })
  ])
  jobs.value = jobData.list || []
  scripts.value = scriptData.list || []
}

async function handleClaim(row) {
  const { value } = await ElMessageBox.prompt(mt('enterClaimNote'), mt('claimAlertTitle'), { inputPlaceholder: mt('claimPlaceholder') })
  await claimMonitorAlertEvent({ id: row.id, claimedBy: value, handleNote: value })
  ElMessage.success(mt('claimedMsg'))
  await loadData()
}

async function handleResolve(row) {
  const { value } = await ElMessageBox.prompt(mt('enterResolveNote'), mt('resolveAlertTitle'), { inputPlaceholder: mt('resolvePlaceholder') })
  await resolveMonitorAlertEvent({ id: row.id, handleNote: value })
  ElMessage.success(mt('resolvedMsg'))
  await loadData()
}

function handleSelectionChange(selection) {
  selectedEventIds.value = selection.map((item) => item.id)
}

async function handleBatchAction(action) {
  if (!selectedEventIds.value.length) return
  if (action === 'delete') {
    await ElMessageBox.confirm(mt('evBatchDeleteConfirm', { count: selectedEventIds.value.length }), mt('evBatchDeleteTitle'), { type: 'warning' })
    await batchUpdateMonitorAlertEvents({ ids: selectedEventIds.value, action: 'delete' })
    ElMessage.success(mt('evBatchDeleted'))
    selectedEventIds.value = []
    await loadData()
    return
  }
  const isClaim = action === 'claim'
  const { value } = await ElMessageBox.prompt(isClaim ? mt('enterClaimNote') : mt('enterBatchNote'), isClaim ? mt('batchClaimTitle') : mt('batchResolveTitle'), {
    inputPlaceholder: isClaim ? mt('claimPlaceholder') : mt('batchResolvePlaceholder')
  })
  await batchUpdateMonitorAlertEvents({
    ids: selectedEventIds.value,
    action,
    claimedBy: isClaim ? value : '',
    handleNote: value
  })
  ElMessage.success(isClaim ? mt('batchClaimedDone') : mt('batchResolvedDone'))
  selectedEventIds.value = []
  await loadData()
}

async function openAction(row) {
  current.value = row
  Object.assign(actionForm, { actionType: 'job', targetId: undefined, operator: mt('systemAdmin'), summary: mt('alertProcessSummary', { name: row.ruleName }) })
  await loadActionOptions()
  actionVisible.value = true
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await queryMonitorAlertEventDetail(row.id)
  } finally {
    detailLoading.value = false
  }
}

async function submitAction() {
  const source = actionForm.actionType === 'job' ? jobs.value : scripts.value
  const target = source.find((item) => item.id === actionForm.targetId)
  await triggerMonitorAlertAction({
    id: current.value.id,
    actionType: actionForm.actionType,
    targetId: actionForm.targetId,
    targetName: target?.name || '',
    operator: actionForm.operator,
    summary: actionForm.summary
  })
  ElMessage.success(actionForm.actionType === 'job' ? mt('jobTriggered') : mt('scriptRecorded'))
  actionVisible.value = false
  await loadData()
}

onMounted(async () => {
  query.status = String(route.query.status || '')
  query.severity = String(route.query.severity || '')
  await loadData()
  const eventId = Number(route.query.eventId || 0)
  if (eventId) await openDetail({ id: eventId })
})
</script>

<template>
  <div class="monitor-page monitor-alert-event-page">
    <div class="page-header">
      <div>
        <h2>Alert Event</h2>
        <p>{{ mt('eventPageDesc') }}</p>
      </div>
      <el-button @click="loadData">{{ mt('refreshSpaced') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="mt('eventSearchPlaceholder')" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable :placeholder="mt('status')" style="width: 140px">
        <el-option :label="mt('unclaimedLabel')" value="unclaimed" />
        <el-option :label="mt('evPending')" value="pending" />
        <el-option :label="mt('evFiring')" value="firing" />
        <el-option :label="mt('evClaimed')" value="claimed" />
        <el-option :label="mt('evSilenced')" value="silenced" />
        <el-option :label="mt('evRecovered')" value="recovered" />
        <el-option :label="mt('evResolved')" value="resolved" />
      </el-select>
      <el-select v-model="query.severity" clearable placeholder="Severity" style="width: 120px">
        <el-option label="P0 / P1" value="critical" />
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ mt('searchLabel') }}</el-button>
    </div>

    <div v-if="selectedEventIds.length" class="batch-toolbar">
      <span>{{ mt('selectedEvents', { count: selectedEventIds.length }) }}</span>
      <el-button size="small" type="primary" @click="handleBatchAction('claim')">{{ mt('batchClaim') }}</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('resolve')">{{ mt('batchResolve') }}</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">{{ mt('batchDelete') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="ruleName" label="Rule" min-width="170" />
      <el-table-column prop="severity" label="Severity" width="90" />
      <el-table-column :label="mt('status')" width="110">
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="mt('claimInfoCol')" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <div v-if="row.claimedBy" class="claim-info">
            <strong>{{ row.claimedBy }}</strong>
            <span>{{ mt('claimColon', { note: row.handleNote || mt('noClaimNote') }) }}</span>
            <span v-if="row.status === 'resolved'">{{ mt('resolveColon', { note: row.resolveNote || mt('noResolveNote') }) }}</span>
            <small>{{ row.updateTime || '-' }}</small>
          </div>
          <div v-else-if="row.status === 'resolved'" class="claim-info claim-none">
            <strong>{{ mt('unclaimedResolved') }}</strong>
            <span>{{ mt('resolveColon', { note: row.resolveNote || mt('noResolveNote') }) }}</span>
          </div>
          <span v-else class="claim-empty">{{ mt('unclaimedLabel') }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="metric" label="Metric" min-width="170" show-overflow-tooltip />
      <el-table-column :label="mt('noiseReductionCol')" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.silenced">{{ mt('silencedColon', { rule: row.silenceRuleName || '-' }) }}</span>
          <span v-else-if="row.aggregateRuleName">{{ mt('aggregatedColon', { rule: row.aggregateRuleName }) }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column :label="mt('currentValue')" width="120">
        <template #default="{ row }">{{ Number(row.currentValue || 0).toFixed(4) }}</template>
      </el-table-column>
      <el-table-column prop="threshold" label="Threshold" width="100" />
      <el-table-column :label="mt('lastTriggerCol')" width="156">
        <template #default="{ row }"><div class="trigger-time"><span>{{ formatTriggerTime(row.lastTriggerAt).date }}</span><small>{{ formatTriggerTime(row.lastTriggerAt).time }}</small></div></template>
      </el-table-column>
      <el-table-column :label="mt('actions')" width="280" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ mt('detail') }}</el-button>
          <el-button link type="primary" @click="openAction(row)">{{ mt('linkedAction') }}</el-button>
          <el-button v-if="row.status === 'firing'" link type="primary" @click="handleClaim(row)">{{ mt('claimShort') }}</el-button>
          <el-button v-if="row.status === 'firing' || row.status === 'claimed' || row.status === 'silenced'" link type="warning" @click="handleResolve(row)">{{ mt('resolveShort') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-drawer v-model="detailVisible" :title="mt('eventDetailTitle')" size="72%" destroy-on-close>
      <div v-loading="detailLoading" class="event-detail">
        <section class="detail-summary">
          <div class="detail-title">
            <div>
              <span>{{ detail.event?.severity || '-' }}</span>
              <h3>{{ detail.event?.ruleName || '-' }}</h3>
            </div>
            <el-tag :type="statusType(detail.event?.status)" effect="light">{{ statusText(detail.event?.status) }}</el-tag>
          </div>
          <el-descriptions :column="4" border>
            <el-descriptions-item label="Datasource">{{ detail.event?.datasourceName || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="mt('currentValue')">{{ Number(detail.event?.currentValue || 0).toFixed(4) }}</el-descriptions-item>
            <el-descriptions-item label="Threshold">{{ detail.event?.threshold ?? '-' }}</el-descriptions-item>
            <el-descriptions-item :label="mt('firstTrigger')">{{ formatTime(detail.event?.firstTriggerAt) }}</el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="notification-state">
          <div>
            <span>{{ mt('notifyDecision') }}</span>
            <strong>{{ detail.notificationState?.reason || mt('noNotifyDecision') }}</strong>
          </div>
          <div><span>{{ mt('sentLabel') }}</span><strong>{{ mt('timesCount', { count: detail.notificationState?.notifyCount || 0 }) }}</strong></div>
          <div><span>{{ mt('nextNotifyAt') }}</span><strong>{{ formatTime(detail.notificationState?.nextNotifyAt) }}</strong></div>
          <div><span>{{ mt('aggregationRuleLabel') }}</span><strong>{{ detail.notificationState?.aggregationRule || '-' }}</strong></div>
        </section>

        <section class="detail-grid">
          <div class="detail-panel">
            <div class="detail-panel-head"><h4>Event Timeline</h4><span>{{ mt('timelineDesc') }}</span></div>
            <el-empty v-if="!detail.timelines?.length" :description="mt('noTimelineRecords')" :image-size="70" />
            <el-timeline v-else>
              <el-timeline-item v-for="item in detail.timelines" :key="`${item.id}-${item.eventType}-${item.createTime}`" :timestamp="formatTime(item.createTime)" placement="top">
                <strong>{{ item.title }}</strong>
                <p>{{ item.detail || '-' }}</p>
                <small v-if="item.operator">{{ mt('handlerColon', { name: item.operator }) }}</small>
              </el-timeline-item>
            </el-timeline>
          </div>
          <div class="detail-panel">
            <div class="detail-panel-head"><h4>Event Label</h4><span>{{ mt('eventLabelDesc') }}</span></div>
            <pre>{{ prettyJSON(detail.event?.labelsJson) }}</pre>
          </div>
        </section>

        <section class="detail-panel">
          <div class="detail-panel-head"><h4>{{ mt('notifyLogTitle') }}</h4><span>{{ mt('notifyLogDesc') }}</span></div>
          <el-table :data="detail.notifyLogs || []" border :empty-text="mt('noNotifyLogs')">
            <el-table-column prop="channelName" label="Notification Channel" min-width="150" />
            <el-table-column prop="event" label="Event" width="100" />
            <el-table-column prop="status" :label="mt('status')" width="100" />
            <el-table-column prop="attemptCount" :label="mt('attemptCountCol')" width="100" />
            <el-table-column prop="durationMs" :label="mt('durationMsCol')" width="100" />
            <el-table-column prop="errorText" :label="mt('failureReasonCol')" min-width="220" show-overflow-tooltip />
            <el-table-column :label="mt('timeCol')" width="180"><template #default="{ row }">{{ formatTime(row.createTime) }}</template></el-table-column>
          </el-table>
        </section>
      </div>
    </el-drawer>

    <el-dialog v-model="actionVisible" :title="mt('linkedActionTitle')" width="620px">
      <el-form :model="actionForm" label-width="100px">
        <el-form-item label="Alert Rule">
          <el-input :model-value="current.ruleName" disabled />
        </el-form-item>
        <el-form-item :label="mt('actionTypeLabel')">
          <el-radio-group v-model="actionForm.actionType">
            <el-radio value="job">{{ mt('triggerJob') }}</el-radio>
            <el-radio value="script">{{ mt('diagnosticScript') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="actionForm.actionType === 'job' ? mt('selectJob') : mt('selectScript')" required>
          <el-select v-model="actionForm.targetId" filterable clearable style="width: 100%">
            <el-option
              v-for="item in actionForm.actionType === 'job' ? jobs : scripts"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="mt('handlerLabel')">
          <el-input v-model="actionForm.operator" />
        </el-form-item>
        <el-form-item :label="mt('descriptionLabel')">
          <el-input v-model="actionForm.summary" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionVisible = false">{{ mt('cancel') }}</el-button>
        <el-button type="primary" @click="submitAction">{{ mt('confirmAction') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08);
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.page-header h2 {
  margin: 0 0 8px;
  font-size: 26px;
  color: #10213f;
}
.page-header p {
  margin: 0;
  color: #7282a0;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.claim-info { display: flex; flex-direction: column; gap: 3px; line-height: 1.35; }
.claim-info strong { color: #314b78; font-size: 13px; }
.claim-info span { overflow: hidden; color: #64748b; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.claim-info small { color: #98a4b7; font-size: 11px; }
.claim-none strong, .claim-empty { color: #98a4b7; font-size: 12px; font-weight: 400; }
.trigger-time { display: flex; flex-direction: column; gap: 3px; line-height: 1.2; }
.trigger-time span { color: #35547d; font-size: 12px; font-variant-numeric: tabular-nums; }
.trigger-time small { color: #8494aa; font-size: 11px; font-variant-numeric: tabular-nums; }
.pager {
  display: flex;
  justify-content: flex-end;
}
.event-detail { display: flex; flex-direction: column; gap: 16px; padding: 0 4px 24px; }
.detail-summary, .detail-panel { padding: 18px; border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; }
.detail-title { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.detail-title > div { display: flex; align-items: center; gap: 10px; }
.detail-title span { color: #d9485f; font-weight: 700; }
.detail-title h3 { margin: 0; color: #162b4b; font-size: 20px; }
.detail-summary > p { margin: 10px 0 16px; color: #6f7e98; }
.notification-state { display: grid; grid-template-columns: 2fr repeat(3, 1fr); border: 1px solid #dce6f5; border-radius: 8px; background: #f7f9fd; }
.notification-state > div { min-width: 0; padding: 14px 16px; border-right: 1px solid #e1e8f3; }
.notification-state > div:last-child { border-right: 0; }
.notification-state span, .notification-state strong { display: block; }
.notification-state span { margin-bottom: 5px; color: #8491a8; font-size: 12px; }
.notification-state strong { overflow: hidden; color: #263d61; text-overflow: ellipsis; white-space: nowrap; }
.detail-grid { display: grid; grid-template-columns: 1.15fr .85fr; gap: 16px; }
.detail-panel-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; margin-bottom: 15px; }
.detail-panel-head h4 { margin: 0; color: #1b3153; font-size: 16px; }
.detail-panel-head span { color: #8b98ad; font-size: 12px; }
.detail-panel pre { max-height: 420px; margin: 0; padding: 14px; overflow: auto; color: #dbe7ff; border-radius: 6px; background: #111b2f; font: 13px/1.7 Consolas, Monaco, monospace; }
.detail-panel :deep(.el-timeline-item__wrapper p) { margin: 5px 0; color: #687993; }
.detail-panel :deep(.el-timeline-item__wrapper small) { color: #98a3b5; }
@media (max-width: 1000px) { .detail-grid { grid-template-columns: 1fr; } .notification-state { grid-template-columns: repeat(2, 1fr); } }
</style>
