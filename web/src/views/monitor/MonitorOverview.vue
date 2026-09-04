<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { queryMonitorOverview } from '../../api/monitor'

const router = useRouter()
const loading = ref(false)
const overview = ref({ severity: [], recentEvents: [], trend: [], recentActivities: [] })
const rangeType = ref('7d')
const customRange = ref([])
const refreshedAt = ref('')

function formatDate(value) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60000
  return new Date(date.getTime() - offset).toISOString().slice(0, 10)
}

function resolveRange() {
  const end = new Date()
  const start = new Date(end)
  if (rangeType.value === '3d') start.setDate(start.getDate() - 2)
  if (rangeType.value === '7d') start.setDate(start.getDate() - 6)
  if (rangeType.value === 'custom' && customRange.value?.length === 2) {
    return { startDate: customRange.value[0], endDate: customRange.value[1] }
  }
  return { startDate: formatDate(start), endDate: formatDate(end) }
}

const rangeLabel = computed(() => ({ today: '오늘', '3d': '최근 3일', '7d': '최근 7일', custom: '사용자 정의 기간' }[rangeType.value] || '오늘'))
const monitorAbnormalCount = computed(() => Number(overview.value.unhealthyDatasourceCount || 0) + Number(overview.value.evalFailedRuleCount || 0))
const trendMax = computed(() => Math.max(1, ...(overview.value.trend || []).flatMap(item => [Number(item.triggered || 0), Number(item.recovered || 0)])))
const totalSeverity = computed(() => (overview.value.severity || []).reduce((sum, item) => sum + Number(item.count || 0), 0))

const riskCards = computed(() => [
  { label: '활성 Alert', value: overview.value.firingCount || 0, hint: '현재 주의가 필요한 Alert', tone: 'danger', query: {} },
  { label: '미인계', value: overview.value.unclaimedCount || 0, hint: '담당자 인계를 기다리는 중', tone: 'warning', query: { status: 'unclaimed' } },
  { label: 'P0 / P1', value: overview.value.criticalCount || 0, hint: '높은 우선순위 리스크 Event', tone: 'danger', query: { severity: 'critical' } },
  { label: '모니터링 오류', value: monitorAbnormalCount.value, hint: `${overview.value.unhealthyDatasourceCount || 0} Datasource · ${overview.value.evalFailedRuleCount || 0} Rule`, tone: 'neutral', path: '/monitor/datasources' }
])

const qualityCards = computed(() => [
  { label: 'Datasource Health', value: ratio(overview.value.healthyDatasourceCount, overview.value.datasourceCount), detail: `${overview.value.healthyDatasourceCount || 0} / ${overview.value.datasourceCount || 0} 정상` },
  { label: 'Rule 실행 성공률', value: ratio(overview.value.successfulRuleCount, overview.value.activeRuleCount), detail: `${overview.value.successfulRuleCount || 0} / ${overview.value.activeRuleCount || 0} 정상` },
  { label: 'Notification 성공률', value: ratio(overview.value.notificationSuccessCount, overview.value.notificationTotalCount), detail: `${overview.value.notificationSuccessCount || 0} / ${overview.value.notificationTotalCount || 0} 성공` },
  { label: '평균 복구 시간', value: formatDuration(overview.value.mttrSeconds), detail: `평균 인계 ${formatDuration(overview.value.mttaSeconds)}` }
])

function ratio(value, total) {
  const numerator = Number(value || 0)
  const denominator = Number(total || 0)
  if (!denominator) return '-'
  return `${Math.round((numerator / denominator) * 100)}%`
}

function statusType(status) {
  return ({ pending: 'warning', firing: 'danger', claimed: 'warning', silenced: 'info', recovered: 'success', resolved: 'success' }[status] || 'info')
}

function statusText(status) {
  return ({ pending: '대기 중', firing: '발생', claimed: '인계됨', silenced: '차단', recovered: '복구', resolved: '해결됨' }[status] || status || '-')
}

function activityText(type) {
  return ({ recovered: 'Alert 복구', datasource: 'Datasource 오류', notification: 'Notification 실패', rule: 'Rule 실행 실패' }[type] || '모니터링 Event')
}

function activityType(type) {
  return ({ recovered: 'success', datasource: 'danger', notification: 'warning', rule: 'danger' }[type] || 'info')
}

function formatDuration(seconds) {
  const value = Number(seconds || 0)
  if (!value) return '-'
  if (value < 60) return `${value}초`
  if (value < 3600) return `${Math.round(value / 60)}분`
  if (value < 86400) return `${(value / 3600).toFixed(1)}시간`
  return `${(value / 86400).toFixed(1)}일`
}

function formatLiveDuration(value) {
  if (!value) return '-'
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000))
  return formatDuration(seconds)
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function severityPercent(count) {
  if (!totalSeverity.value) return 0
  return Math.round((Number(count || 0) / totalSeverity.value) * 100)
}

function openRisk(card) {
  if (card.path) return router.push(card.path)
  router.push({ path: '/monitor/alert-events', query: card.query || {} })
}

function openEvent(row) {
  router.push({ path: '/monitor/alert-events', query: { eventId: row.id } })
}

async function loadData() {
  loading.value = true
  try {
    overview.value = await queryMonitorOverview(resolveRange())
    refreshedAt.value = overview.value.refreshedAt || new Date().toISOString()
  } finally {
    loading.value = false
  }
}

function changeRange(type) {
  rangeType.value = type
  if (type !== 'custom') loadData()
}

function applyCustomRange() {
  if (customRange.value?.length === 2) loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-overview" v-loading="loading">
    <section class="overview-header">
      <div>
        <p class="eyebrow">MONITORING OVERVIEW</p>
        <h1>모니터링 개요</h1>
        <p class="header-subtitle">현재 시스템 상태, 리스크와 Event 한눈에 보기</p>
      </div>
      <div class="header-actions">
        <span class="refreshed">마지막 새로고침: {{ formatTime(refreshedAt) }}</span>
        <el-button-group>
          <el-button :type="rangeType === 'today' ? 'primary' : 'default'" @click="changeRange('today')">오늘</el-button>
          <el-button :type="rangeType === '3d' ? 'primary' : 'default'" @click="changeRange('3d')">3일</el-button>
          <el-button :type="rangeType === '7d' ? 'primary' : 'default'" @click="changeRange('7d')">7일</el-button>
          <el-button :type="rangeType === 'custom' ? 'primary' : 'default'" @click="changeRange('custom')">사용자 정의</el-button>
        </el-button-group>
        <el-date-picker
          v-if="rangeType === 'custom'"
          v-model="customRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="~"
          start-placeholder="시작일"
          end-placeholder="종료일"
          :clearable="false"
          @change="applyCustomRange"
        />
        <el-button :loading="loading" @click="loadData">새로고침</el-button>
      </div>
    </section>

    <section>
      <div class="section-title"><h2>현재 리스크</h2><span>실시간 리스크는 통계 기간의 영향을 받지 않습니다</span></div>
      <div class="risk-grid">
        <button v-for="card in riskCards" :key="card.label" type="button" :class="['risk-card', card.tone]" @click="openRisk(card)">
          <span>{{ card.label }}</span><strong>{{ card.value }}</strong><small>{{ card.hint }}</small>
        </button>
      </div>
    </section>

    <section class="top-grid">
      <article class="panel quality-panel"><div class="panel-head"><div><h2>모니터링 품질</h2><p>데이터 파이프라인, Rule과 Notification의 실행 품질</p></div></div><div class="quality-grid"><div v-for="item in qualityCards" :key="item.label" class="quality-card"><span>{{ item.label }}</span><strong>{{ item.value }}</strong><small>{{ item.detail }}</small></div></div></article>
      <article class="panel trend-panel">
        <div class="panel-head"><div><h2>Alert 추세</h2><p>{{ rangeLabel }} 신규 / 복구 추세</p></div><div class="legend"><i class="new" />신규 <i class="recovered" />복구</div></div>
        <div class="trend-chart">
          <div v-for="item in overview.trend || []" :key="item.date" class="trend-item">
            <el-tooltip placement="top" :show-after="120" popper-class="trend-tooltip">
              <template #content>
                <div class="trend-tooltip-content"><b>{{ item.date }}</b><span>신규 {{ item.triggered || 0 }}건</span><span>복구 {{ item.recovered || 0 }}건</span></div>
              </template>
              <div class="bars" tabindex="0" :aria-label="`${item.date}: 신규 ${item.triggered || 0}건, 복구 ${item.recovered || 0}건`"><span class="trend-bar new" :style="{ height: `${Math.max(3, Number(item.triggered || 0) / trendMax * 100)}%` }" /><span class="trend-bar recovered" :style="{ height: `${Math.max(3, Number(item.recovered || 0) / trendMax * 100)}%` }" /></div>
            </el-tooltip>
            <small>{{ item.date }}</small>
          </div>
        </div>
      </article>
      <article class="panel activities-panel"><div class="panel-head"><div><h2>최근 Event</h2><p>Alert 복구, Datasource 오류, Notification 실패와 Rule 실행 실패</p></div></div><el-empty v-if="!(overview.recentActivities || []).length" description="주의가 필요한 모니터링 Event가 없습니다" :image-size="48" /><div v-else class="activity-list"><div v-for="(item, index) in overview.recentActivities || []" :key="`${item.type}-${index}`" class="activity"><el-tag :type="activityType(item.type)" effect="light">{{ activityText(item.type) }}</el-tag><div><b>{{ item.title || '-' }}</b><p>{{ item.detail || '-' }}</p></div><time>{{ formatTime(item.time) }}</time></div></div></article>
    </section>

    <section class="work-grid">
      <article class="panel pending-panel">
        <div class="panel-head"><div><h2>현재 처리 대기 Alert</h2><p>우선순위, 상태, 담당자와 지속 시간으로 빠르게 찾기</p></div><el-button link type="primary" @click="router.push('/monitor/alert-events')">전체 보기</el-button></div>
        <el-empty v-if="!(overview.recentEvents || []).length" class="pending-empty" description="현재 처리 대기 Alert가 없으며 시스템이 정상 운영 중입니다" :image-size="48" />
        <el-table v-else :data="overview.recentEvents || []" @row-click="openEvent">
          <el-table-column prop="severity" label="우선순위" width="84"><template #default="{ row }"><el-tag :type="row.severity === 'P0' || row.severity === 'P1' ? 'danger' : row.severity === 'P2' ? 'warning' : 'info'" effect="light">{{ row.severity }}</el-tag></template></el-table-column>
          <el-table-column prop="ruleName" label="Alert Rule" min-width="190" />
          <el-table-column label="상태" width="105"><template #default="{ row }"><el-tag :type="statusType(row.status)" effect="light">{{ statusText(row.status) }}</el-tag></template></el-table-column>
          <el-table-column prop="claimedBy" label="담당자" width="130"><template #default="{ row }">{{ row.claimedBy || '미인계' }}</template></el-table-column>
          <el-table-column label="지속 시간" width="120"><template #default="{ row }">{{ formatLiveDuration(row.firstTriggerAt) }}</template></el-table-column>
          <el-table-column prop="summary" label="요약" min-width="280" show-overflow-tooltip />
        </el-table>
      </article>
      <article class="panel severity-panel">
        <div class="panel-head"><div><h2>Alert Severity 분포</h2><p>현재 처리 대기 Alert를 P0 / P1 / P2 / P3로 표시</p></div></div>
        <div class="severity-list">
          <button v-for="item in overview.severity || []" :key="item.severity" class="severity-row" type="button" @click="router.push({ path: '/monitor/alert-events', query: { severity: item.severity } })">
            <b>{{ item.severity }}</b><div class="severity-track"><span :class="item.severity.toLowerCase()" :style="{ width: `${severityPercent(item.count)}%` }" /></div><strong>{{ item.count || 0 }}</strong>
          </button>
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
.monitor-overview { display: flex; flex-direction: column; gap: 16px; color: #172b4d; }
.overview-header, .panel { background: #fff; border: 1px solid #e2eaf5; border-radius: 12px; }
.overview-header { display: flex; justify-content: space-between; align-items: center; gap: 24px; padding: 22px 24px; }
.eyebrow { margin: 0 0 6px; color: #5378d8; font-size: 12px; font-weight: 700; letter-spacing: .08em; }.overview-header h1 { margin: 0; font-size: 26px; }.header-subtitle { margin: 7px 0 0; color: #7b8da9; }.header-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: 8px; }.refreshed { color: #8a98ad; font-size: 13px; white-space: nowrap; }.header-actions :deep(.el-date-editor) { width: 246px; }
.section-title { display: flex; align-items: baseline; gap: 10px; margin: 2px 0 10px; }.section-title h2, .panel-head h2 { margin: 0; font-size: 18px; }.section-title span, .panel-head p { margin: 0; color: #8190a7; font-size: 13px; }.risk-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }.risk-card { padding: 18px 20px; text-align: left; border: 1px solid #e3eaf5; border-radius: 10px; background: #fff; cursor: pointer; transition: transform .2s, box-shadow .2s; }.risk-card:hover { transform: translateY(-2px); box-shadow: 0 10px 24px rgba(36, 58, 103, .1); }.risk-card span { display: block; color: #70809a; font-size: 14px; }.risk-card strong { display: block; margin: 9px 0 5px; color: #18345e; font-size: 30px; line-height: 1; }.risk-card small { color: #99a6b8; }.risk-card.danger strong { color: #e35658; }.risk-card.warning strong { color: #d88924; }
.top-grid { display: grid; grid-template-columns: minmax(260px, .85fr) minmax(420px, 1.3fr) minmax(300px, .85fr); gap: 16px; }.work-grid { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(340px, .75fr); gap: 16px; }.panel { min-width: 0; padding: 18px 20px; }.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }.panel-head h2 { margin-bottom: 5px; }.legend { color: #7f8ea3; font-size: 12px; white-space: nowrap; }.legend i { display: inline-block; width: 8px; height: 8px; margin: 0 5px 0 10px; border-radius: 2px; }.legend .new, .trend-bar.new { background: #5b7cf6; }.legend .recovered, .trend-bar.recovered { background: #2ac393; }
.trend-chart { display: flex; align-items: flex-end; gap: 8px; height: 142px; padding: 8px 4px 0; border-bottom: 1px solid #edf1f7; }.trend-item { display: flex; flex: 1; flex-direction: column; align-items: center; min-width: 20px; gap: 5px; height: 100%; }.bars { display: flex; align-items: flex-end; justify-content: center; gap: 3px; width: 100%; height: 112px; cursor: help; outline: 0; background: linear-gradient(to top, #eef2f8 1px, transparent 1px); background-size: 100% 28px; }.bars:focus-visible { outline: 2px solid #5b7cf6; outline-offset: 2px; border-radius: 4px; }.trend-bar { display: block; width: min(12px, 34%); min-height: 2px; border-radius: 3px 3px 0 0; }.trend-item small { color: #8593a8; font-size: 11px; white-space: nowrap; }.trend-tooltip-content { display: grid; gap: 4px; min-width: 106px; }.trend-tooltip-content b { color: #fff; font-size: 12px; }.trend-tooltip-content span { font-size: 12px; }
.severity-list { display: flex; flex-direction: column; gap: 10px; }.severity-row { display: grid; grid-template-columns: 36px minmax(0, 1fr) 28px; align-items: center; gap: 10px; padding: 8px 4px; border: 0; background: transparent; cursor: pointer; color: #40516d; text-align: left; }.severity-track { overflow: hidden; height: 8px; border-radius: 10px; background: #edf1f6; }.severity-track span { display: block; height: 100%; border-radius: inherit; background: #6e84e8; }.severity-track .p0 { background: #e55353; }.severity-track .p1 { background: #f08a45; }.severity-track .p2 { background: #e8bd39; }.severity-track .p3 { background: #51b99b; }.severity-row strong { text-align: right; }
.pending-panel :deep(.el-table__row) { cursor: pointer; }.pending-empty { padding: 2px 0; }.pending-empty :deep(.el-empty__description) { margin-top: 4px; }.quality-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }.quality-card { padding: 14px; border-radius: 8px; background: #f7f9fd; }.quality-card span, .quality-card small { display: block; color: #7e8ca3; font-size: 12px; }.quality-card strong { display: block; margin: 7px 0 5px; color: #1c3d6e; font-size: 21px; }.activity-list { display: flex; flex-direction: column; }.activity { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid #edf1f6; }.activity:last-child { border-bottom: 0; }.activity b { display: block; color: #334967; font-size: 13px; }.activity p { overflow: hidden; margin: 3px 0 0; color: #8592a5; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.activity time { color: #9aa6b7; font-size: 12px; white-space: nowrap; }
@media (max-width: 1250px) { .top-grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }.activities-panel { grid-column: span 2; } }
@media (max-width: 1050px) { .overview-header { align-items: flex-start; flex-direction: column; }.header-actions { justify-content: flex-start; }.top-grid, .work-grid { grid-template-columns: 1fr; }.activities-panel { grid-column: auto; } }
@media (max-width: 720px) { .risk-grid, .quality-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }.activity { grid-template-columns: auto 1fr; }.activity time { display: none; } }
</style>
