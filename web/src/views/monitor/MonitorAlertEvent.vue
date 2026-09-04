<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchUpdateMonitorAlertEvents, claimMonitorAlertEvent, queryMonitorAlertEventDetail, queryMonitorAlertEventList, resolveMonitorAlertEvent, triggerMonitorAlertAction } from '../../api/monitor'
import { queryOpsJobList, queryOpsScriptList } from '../../api/ops'

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
const actionForm = reactive({ actionType: 'job', targetId: undefined, operator: '시스템 관리자', summary: '' })

function statusType(status) {
  if (status === 'pending') return 'warning'
  if (status === 'firing') return 'danger'
  if (status === 'claimed') return 'warning'
  if (status === 'silenced') return 'info'
  if (status === 'recovered') return 'success'
  return 'info'
}

function statusText(status) {
  return ({ pending: '대기 중', firing: '트리거 중', claimed: '인계됨', silenced: '차단됨', recovered: '복구됨', resolved: '종료됨' }[status] || status)
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
  const { value } = await ElMessageBox.prompt('인계자 또는 처리 설명을 입력하십시오.', 'Alert 인계', { inputPlaceholder: '예: 홍길동 조사 중' })
  await claimMonitorAlertEvent({ id: row.id, claimedBy: value, handleNote: value })
  ElMessage.success('인계했습니다.')
  await loadData()
}

async function handleResolve(row) {
  const { value } = await ElMessageBox.prompt('처리 설명을 입력하십시오.', 'Alert 종료', { inputPlaceholder: '예: 서비스가 복구되었습니다.' })
  await resolveMonitorAlertEvent({ id: row.id, handleNote: value })
  ElMessage.success('종료했습니다.')
  await loadData()
}

function handleSelectionChange(selection) {
  selectedEventIds.value = selection.map((item) => item.id)
}

async function handleBatchAction(action) {
  if (!selectedEventIds.value.length) return
  if (action === 'delete') {
    await ElMessageBox.confirm(`선택한 ${selectedEventIds.value.length}건의 Alert Event 기록을 삭제하시겠습니까? 이 작업은 Alert Rule을 삭제하지 않습니다.`, '일괄 삭제 확인', { type: 'warning' })
    await batchUpdateMonitorAlertEvents({ ids: selectedEventIds.value, action: 'delete' })
    ElMessage.success('Alert Event를 일괄 삭제했습니다.')
    selectedEventIds.value = []
    await loadData()
    return
  }
  const isClaim = action === 'claim'
  const { value } = await ElMessageBox.prompt(isClaim ? '인계자 또는 처리 설명을 입력하십시오.' : '일괄 처리 설명을 입력하십시오.', isClaim ? 'Alert 일괄 인계' : 'Alert 일괄 종료', {
    inputPlaceholder: isClaim ? '예: 홍길동 조사 중' : '예: 문제를 처리했으며 서비스가 복구되었습니다.'
  })
  await batchUpdateMonitorAlertEvents({
    ids: selectedEventIds.value,
    action,
    claimedBy: isClaim ? value : '',
    handleNote: value
  })
  ElMessage.success(isClaim ? '인계 가능한 Alert을 일괄 인계했습니다.' : '종료되지 않은 Alert을 일괄 종료했습니다.')
  selectedEventIds.value = []
  await loadData()
}

async function openAction(row) {
  current.value = row
  Object.assign(actionForm, { actionType: 'job', targetId: undefined, operator: '시스템 관리자', summary: `Alert 처리: ${row.ruleName}` })
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
  ElMessage.success(actionForm.actionType === 'job' ? 'Job을 트리거했습니다.' : '진단 Script 처리를 기록했습니다.')
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
        <p>트리거, 인계, 차단, 복구, 종료 상태를 한곳에서 확인하고 진단 Script 또는 Job을 바로 트리거할 수 있습니다.</p>
      </div>
      <el-button @click="loadData">새로 고침</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="Rule / Metric 검색" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="상태" style="width: 140px">
        <el-option label="인계 안 됨" value="unclaimed" />
        <el-option label="대기 중" value="pending" />
        <el-option label="트리거 중" value="firing" />
        <el-option label="인계됨" value="claimed" />
        <el-option label="차단됨" value="silenced" />
        <el-option label="복구됨" value="recovered" />
        <el-option label="종료됨" value="resolved" />
      </el-select>
      <el-select v-model="query.severity" clearable placeholder="Severity" style="width: 120px">
        <el-option label="P0 / P1" value="critical" />
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-button type="primary" @click="loadData">검색</el-button>
    </div>

    <div v-if="selectedEventIds.length" class="batch-toolbar">
      <span>Alert Event <b>{{ selectedEventIds.length }}</b>건을 선택했습니다</span>
      <el-button size="small" type="primary" @click="handleBatchAction('claim')">일괄 인계</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('resolve')">일괄 종료</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">일괄 삭제</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="ruleName" label="Rule" min-width="170" />
      <el-table-column prop="severity" label="Severity" width="90" />
      <el-table-column label="상태" width="110">
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="인계 정보" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <div v-if="row.claimedBy" class="claim-info">
            <strong>{{ row.claimedBy }}</strong>
            <span>인계: {{ row.handleNote || '인계 설명 미입력' }}</span>
            <span v-if="row.status === 'resolved'">종료: {{ row.resolveNote || '종료 설명 미입력' }}</span>
            <small>{{ row.updateTime || '-' }}</small>
          </div>
          <div v-else-if="row.status === 'resolved'" class="claim-info claim-none">
            <strong>인계 안 됨 / 종료됨</strong>
            <span>종료: {{ row.resolveNote || '종료 설명 미입력' }}</span>
          </div>
          <span v-else class="claim-empty">인계 안 됨</span>
        </template>
      </el-table-column>
      <el-table-column prop="metric" label="Metric" min-width="170" show-overflow-tooltip />
      <el-table-column label="노이즈 감소 적중" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.silenced">차단: {{ row.silenceRuleName || '-' }}</span>
          <span v-else-if="row.aggregateRuleName">집계: {{ row.aggregateRuleName }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="현재 값" width="120">
        <template #default="{ row }">{{ Number(row.currentValue || 0).toFixed(4) }}</template>
      </el-table-column>
      <el-table-column prop="threshold" label="Threshold" width="100" />
      <el-table-column label="마지막 트리거 시각" width="156">
        <template #default="{ row }"><div class="trigger-time"><span>{{ formatTriggerTime(row.lastTriggerAt).date }}</span><small>{{ formatTriggerTime(row.lastTriggerAt).time }}</small></div></template>
      </el-table-column>
      <el-table-column label="작업" width="280" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">상세</el-button>
          <el-button link type="primary" @click="openAction(row)">연동 처리</el-button>
          <el-button v-if="row.status === 'firing'" link type="primary" @click="handleClaim(row)">인계</el-button>
          <el-button v-if="row.status === 'firing' || row.status === 'claimed' || row.status === 'silenced'" link type="warning" @click="handleResolve(row)">종료</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[20, 50, 100, 200]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-drawer v-model="detailVisible" title="Alert Event 상세" size="72%" destroy-on-close>
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
            <el-descriptions-item label="현재 값">{{ Number(detail.event?.currentValue || 0).toFixed(4) }}</el-descriptions-item>
            <el-descriptions-item label="Threshold">{{ detail.event?.threshold ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="첫 트리거">{{ formatTime(detail.event?.firstTriggerAt) }}</el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="notification-state">
          <div>
            <span>Notification 판단</span>
            <strong>{{ detail.notificationState?.reason || 'Notification 판단 없음' }}</strong>
          </div>
          <div><span>전송됨</span><strong>{{ detail.notificationState?.notifyCount || 0 }}회</strong></div>
          <div><span>다음 전송 가능 시각</span><strong>{{ formatTime(detail.notificationState?.nextNotifyAt) }}</strong></div>
          <div><span>집계 Rule</span><strong>{{ detail.notificationState?.aggregationRule || '-' }}</strong></div>
        </section>

        <section class="detail-grid">
          <div class="detail-panel">
            <div class="detail-panel-head"><h4>Event Timeline</h4><span>트리거, Notification, 인계와 복구 전 과정</span></div>
            <el-empty v-if="!detail.timelines?.length" description="이 History Event에는 Timeline 기록이 없습니다" :image-size="70" />
            <el-timeline v-else>
              <el-timeline-item v-for="item in detail.timelines" :key="`${item.id}-${item.eventType}-${item.createTime}`" :timestamp="formatTime(item.createTime)" placement="top">
                <strong>{{ item.title }}</strong>
                <p>{{ item.detail || '-' }}</p>
                <small v-if="item.operator">처리자: {{ item.operator }}</small>
              </el-timeline-item>
            </el-timeline>
          </div>
          <div class="detail-panel">
            <div class="detail-panel-head"><h4>Event Label</h4><span>지문(Fingerprint), Routing, 집계 판단에 사용</span></div>
            <pre>{{ prettyJSON(detail.event?.labelsJson) }}</pre>
          </div>
        </section>

        <section class="detail-panel">
          <div class="detail-panel-head"><h4>Notification 전송 기록</h4><span>최근 20회 Channel 전송 결과</span></div>
          <el-table :data="detail.notifyLogs || []" border empty-text="Notification을 전송하지 않았습니다">
            <el-table-column prop="channelName" label="Notification Channel" min-width="150" />
            <el-table-column prop="event" label="Event" width="100" />
            <el-table-column prop="status" label="상태" width="100" />
            <el-table-column prop="attemptCount" label="시도 횟수" width="100" />
            <el-table-column prop="durationMs" label="소요 시간(ms)" width="100" />
            <el-table-column prop="errorText" label="실패 원인" min-width="220" show-overflow-tooltip />
            <el-table-column label="시각" width="180"><template #default="{ row }">{{ formatTime(row.createTime) }}</template></el-table-column>
          </el-table>
        </section>
      </div>
    </el-drawer>

    <el-dialog v-model="actionVisible" title="Alert 연동 처리" width="620px">
      <el-form :model="actionForm" label-width="100px">
        <el-form-item label="Alert Rule">
          <el-input :model-value="current.ruleName" disabled />
        </el-form-item>
        <el-form-item label="처리 유형">
          <el-radio-group v-model="actionForm.actionType">
            <el-radio value="job">Job 트리거</el-radio>
            <el-radio value="script">진단 Script</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="actionForm.actionType === 'job' ? 'Job 선택' : 'Script 선택'" required>
          <el-select v-model="actionForm.targetId" filterable clearable style="width: 100%">
            <el-option
              v-for="item in actionForm.actionType === 'job' ? jobs : scripts"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="처리자">
          <el-input v-model="actionForm.operator" />
        </el-form-item>
        <el-form-item label="설명">
          <el-input v-model="actionForm.summary" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionVisible = false">취소</el-button>
        <el-button type="primary" @click="submitAction">처리 확인</el-button>
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
