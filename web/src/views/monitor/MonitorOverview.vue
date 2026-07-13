<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { queryMonitorOverview } from '../../api/monitor'

const router = useRouter()
const loading = ref(false)
const overview = ref({ severity: [], recentEvents: [] })

const riskCards = computed(() => [
  { key: 'unclaimed', label: '未认领告警', value: overview.value.unclaimedCount || 0, hint: '需要值班人员及时接手', tone: 'danger', route: { status: 'unclaimed' } },
  { key: 'critical', label: 'P0 / P1 告警', value: overview.value.criticalCount || 0, hint: '当前高优先级风险', tone: 'warning', route: { severity: 'critical' } },
  { key: 'datasource', label: '异常数据源', value: overview.value.unhealthyDatasourceCount || 0, hint: `共 ${overview.value.datasourceCount || 0} 个数据源`, tone: 'neutral', path: '/monitor/datasources' },
  { key: 'evaluation', label: '评估失败规则', value: overview.value.evalFailedRuleCount || 0, hint: `共 ${overview.value.ruleCount || 0} 条规则`, tone: 'neutral', path: '/monitor/alert-rules' },
  { key: 'notification', label: '通知失败', value: overview.value.notificationFailedCount || 0, hint: '最近 24 小时', tone: 'neutral', path: '/notify/send-logs' },
  { key: 'today', label: '今日新告警', value: overview.value.todayTriggeredCount || 0, hint: '当天首次生成的事件', tone: 'primary', path: '/monitor/alert-events' }
])

function statusType(status) {
  return ({ pending: 'warning', firing: 'danger', claimed: 'warning', silenced: 'info', recovered: 'success', resolved: 'info' }[status] || 'info')
}

function statusText(status) {
  return ({ pending: '等待持续', firing: '触发中', claimed: '已认领', silenced: '已屏蔽', recovered: '已恢复', resolved: '已关闭' }[status] || status || '-')
}

function formatDuration(seconds) {
  const value = Number(seconds || 0)
  if (!value) return '-'
  if (value < 60) return `${value} 秒`
  if (value < 3600) return `${Math.round(value / 60)} 分钟`
  return `${(value / 3600).toFixed(1)} 小时`
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function openCard(card) {
  if (card.path) {
    router.push(card.path)
    return
  }
  router.push({ path: '/monitor/alert-events', query: card.route || {} })
}

function openEvent(row) {
  router.push({ path: '/monitor/alert-events', query: { eventId: row.id } })
}

async function loadData() {
  loading.value = true
  try {
    overview.value = await queryMonitorOverview()
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-overview" v-loading="loading">
    <section class="page-header">
      <div>
        <h1>监控概览</h1>
        <p>从当前风险开始值班，快速定位未认领告警、异常数据源和通知故障。</p>
      </div>
      <el-button @click="loadData">刷新</el-button>
    </section>

    <section class="risk-grid">
      <button v-for="card in riskCards" :key="card.key" type="button" :class="['risk-card', card.tone]" @click="openCard(card)">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
        <small>{{ card.hint }}</small>
      </button>
    </section>

    <section class="service-strip">
      <div><span>当前告警</span><strong>{{ overview.firingCount || 0 }}</strong></div>
      <div><span>今日恢复</span><strong>{{ overview.todayRecoveredCount || 0 }}</strong></div>
      <div><span>平均认领时间</span><strong>{{ formatDuration(overview.mttaSeconds) }}</strong></div>
      <div><span>平均恢复时间</span><strong>{{ formatDuration(overview.mttrSeconds) }}</strong></div>
    </section>

    <section class="content-grid">
      <div class="panel severity-panel">
        <div class="panel-head">
          <div><h2>当前告警等级</h2><p>仅统计等待、触发中和已认领事件</p></div>
        </div>
        <div class="severity-list">
          <button v-for="item in overview.severity || []" :key="item.severity" type="button" class="severity-row" @click="router.push({ path: '/monitor/alert-events', query: { severity: item.severity } })">
            <b>{{ item.severity }}</b>
            <el-progress :percentage="Math.min(100, Number(item.count || 0) * 20)" :show-text="false" :stroke-width="8" />
            <strong>{{ item.count || 0 }}</strong>
          </button>
        </div>
      </div>

      <div class="panel event-panel">
        <div class="panel-head">
          <div><h2>当前告警</h2><p>历史恢复记录不再占用值班首屏</p></div>
          <el-button link type="primary" @click="router.push('/monitor/alert-events')">查看全部</el-button>
        </div>
        <el-table :data="overview.recentEvents || []" empty-text="当前没有待处理告警" @row-click="openEvent">
          <el-table-column prop="ruleName" label="规则" min-width="170" />
          <el-table-column prop="severity" label="等级" width="76" />
          <el-table-column label="状态" width="104">
            <template #default="{ row }"><el-tag :type="statusType(row.status)" effect="light">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="summary" label="摘要" min-width="260" show-overflow-tooltip />
          <el-table-column label="最近触发" width="178"><template #default="{ row }">{{ formatTime(row.lastTriggerAt) }}</template></el-table-column>
        </el-table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.monitor-overview { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; padding: 22px 24px; background: #fff; border: 1px solid #e3eaf5; border-radius: 8px; }
.page-header h1 { margin: 0 0 7px; color: #10213f; font-size: 26px; }
.page-header p, .panel-head p { margin: 0; color: #7a89a4; }
.risk-grid { display: grid; grid-template-columns: repeat(6, minmax(150px, 1fr)); gap: 12px; }
.risk-card { min-width: 0; padding: 17px 18px; text-align: left; border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .2s, box-shadow .2s; }
.risk-card:hover { border-color: #5d78e8; box-shadow: 0 8px 20px rgba(35, 61, 120, .09); }
.risk-card span { display: block; color: #657590; font-size: 13px; }
.risk-card strong { display: block; margin: 8px 0 5px; color: #142847; font-size: 28px; line-height: 1; }
.risk-card small { color: #97a3b7; }
.risk-card.danger strong { color: #dc3f48; }
.risk-card.warning strong { color: #d97706; }
.risk-card.primary strong { color: #3567d6; }
.service-strip { display: grid; grid-template-columns: repeat(4, 1fr); border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; }
.service-strip > div { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; padding: 14px 18px; border-right: 1px solid #e8edf5; }
.service-strip > div:last-child { border-right: 0; }
.service-strip span { color: #74829b; }
.service-strip strong { color: #1a3154; font-size: 18px; }
.content-grid { display: grid; grid-template-columns: 330px minmax(0, 1fr); gap: 16px; }
.panel { min-width: 0; padding: 18px; border: 1px solid #e0e8f4; border-radius: 8px; background: #fff; }
.panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.panel-head h2 { margin: 0 0 5px; color: #172b4d; font-size: 17px; }
.panel-head p { font-size: 12px; }
.severity-list { display: flex; flex-direction: column; gap: 8px; }
.severity-row { display: grid; grid-template-columns: 40px 1fr 32px; align-items: center; gap: 10px; width: 100%; padding: 10px 8px; border: 0; border-radius: 6px; background: transparent; color: #344566; cursor: pointer; }
.severity-row:hover { background: #f4f7fc; }
.severity-row b { text-align: left; }
.severity-row strong { text-align: right; }
.event-panel :deep(.el-table__row) { cursor: pointer; }
@media (max-width: 1400px) { .risk-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px) { .risk-grid, .service-strip { grid-template-columns: repeat(2, 1fr); } .content-grid { grid-template-columns: 1fr; } .service-strip > div { border-bottom: 1px solid #e8edf5; } }
</style>
