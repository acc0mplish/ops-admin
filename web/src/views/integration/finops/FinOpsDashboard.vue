<script setup>
import { computed, onMounted, ref } from 'vue'
import { queryFinOpsAccounts, queryFinOpsDashboard } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const data = ref({ trend: [], providerDistribution: [] })
const accounts = ref([])
const rangeMonths = ref(6)
const colors = ['#3d68f5', '#7850f2', '#18aebb', '#ff9b2f', '#2d93d8']
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const monthKey = date => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
const localDate = date => `${monthKey(date)}-${String(date.getDate()).padStart(2, '0')}`
const currentMonth = computed(() => monthKey(new Date()))
const rangeLabel = computed(() => `最近${rangeMonths.value}个月（含本月）`)
const monthKeys = computed(() => Array.from({ length: rangeMonths.value }, (_, index) => {
  const now = new Date()
  return monthKey(new Date(now.getFullYear(), now.getMonth() - rangeMonths.value + index + 1, 1))
}))
const params = computed(() => ({ start: `${monthKeys.value[0]}-01`, end: localDate(new Date()) }))
const monthlyTrend = computed(() => {
  const values = {}
  for (const item of data.value.trend || []) {
    const month = String(item.date || '').slice(0, 7)
    values[month] = (values[month] || 0) + Number(item.amount || 0)
  }
  return monthKeys.value.map(month => ({
    month,
    label: `${Number(month.slice(5))}月`,
    amount: values[month] || 0,
    isPartial: month === currentMonth.value,
  }))
})
const maxTrend = computed(() => Math.max(...monthlyTrend.value.map(item => item.amount), 1))
const chartPoints = computed(() => monthlyTrend.value.map((item, index) => {
  const x = monthlyTrend.value.length === 1 ? 50 : 7 + index * 86 / (monthlyTrend.value.length - 1)
  const y = 76 - item.amount / maxTrend.value * 59
  return { ...item, x, y }
}))
const chartPolyline = computed(() => chartPoints.value.map(item => `${item.x},${item.y}`).join(' '))
const chartArea = computed(() => `7,82 ${chartPolyline.value} 93,82`)
const currentMonthCost = computed(() => monthlyTrend.value.find(item => item.month === currentMonth.value)?.amount || 0)
const previousMonthCost = computed(() => monthlyTrend.value.at(-2)?.amount || 0)
const monthOverMonth = computed(() => previousMonthCost.value ? (currentMonthCost.value - previousMonthCost.value) / previousMonthCost.value * 100 : null)
const latestSyncAt = computed(() => {
  const values = accounts.value.map(item => item.lastSyncAt).filter(Boolean).sort((a, b) => new Date(b) - new Date(a))
  return values[0] ? new Date(values[0]).toLocaleString('zh-CN', { hour12: false }) : '-'
})
const providers = computed(() => {
  const total = Number(data.value.totalCost || 0)
  return (data.value.providerDistribution || []).map((item, index) => ({ ...item, color: colors[index % colors.length], percent: total ? Number(item.amount || 0) / total * 100 : 0 }))
})
const donutStyle = computed(() => {
  let point = 0
  const segments = providers.value.map(item => {
    const end = point + item.percent
    const segment = `${item.color} ${point}% ${end}%`
    point = end
    return segment
  })
  return { background: `conic-gradient(${segments.length ? segments.join(', ') : '#e9effa 0 100%'})` }
})

async function load() {
  loading.value = true
  try {
    const [dashboard, accountRows] = await Promise.all([queryFinOpsDashboard(params.value), queryFinOpsAccounts()])
    data.value = dashboard || { trend: [], providerDistribution: [] }
    accounts.value = accountRows || []
  } finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="finops-page finops-dashboard" v-loading="loading">
    <section class="finops-panel finops-dashboard-head">
      <div>
        <h2>费用看板</h2>
        <p>{{ rangeLabel }}多云费用总览；当前月费用统计截至当前。</p>
      </div>
      <div class="finops-actions">
        <span class="finops-period-label">时间范围</span>
        <el-radio-group v-model="rangeMonths" @change="load">
          <el-radio-button :value="3">最近3个月</el-radio-button>
          <el-radio-button :value="6">最近6个月</el-radio-button>
          <el-radio-button :value="12">最近12个月</el-radio-button>
        </el-radio-group>
        <el-button type="primary" @click="load">刷新</el-button>
      </div>
    </section>

    <section class="finops-summary-cards">
      <article class="finops-summary-card"><span class="finops-summary-icon blue">¥</span><div><small>{{ rangeLabel }}累计费用</small><strong>¥ {{ money(data.totalCost) }}</strong><p>已纳入 {{ data.recordCount || 0 }} 条账单</p></div></article>
      <article class="finops-summary-card"><span class="finops-summary-icon purple">▣</span><div><small>本月费用</small><strong>¥ {{ money(currentMonthCost) }}</strong><p>截至当前账期</p></div></article>
      <article class="finops-summary-card"><span class="finops-summary-icon green">↘</span><div><small>上月费用</small><strong>¥ {{ money(previousMonthCost) }}</strong><p>环比 <b :class="monthOverMonth > 0 ? 'rise' : 'fall'">{{ monthOverMonth === null ? '—' : `${monthOverMonth > 0 ? '+' : ''}${monthOverMonth.toFixed(2)}%` }}</b></p></div></article>
      <article class="finops-summary-card"><span class="finops-summary-icon orange">↻</span><div><small>最近同步时间</small><strong class="sync-value">{{ latestSyncAt }}</strong><p>{{ data.accountCount || 0 }} 个云账号</p></div></article>
    </section>

    <div class="finops-dashboard-grid">
      <section class="finops-panel finops-trend-card">
        <div class="finops-card-title"><div><h2>{{ rangeLabel }}费用趋势</h2><p>单位：人民币</p></div><span>按月</span></div>
        <div class="finops-chart-legend"><i></i>实际费用</div>
        <svg class="finops-line-chart" viewBox="0 0 100 92" preserveAspectRatio="none" aria-label="月度费用趋势">
          <path class="grid" d="M7 17H93M7 47H93M7 77H93" />
          <polygon :points="chartArea" class="area" />
          <polyline :points="chartPolyline" class="line" />
          <g v-for="point in chartPoints" :key="point.month"><circle :cx="point.x" :cy="point.y" r="1.15" /><text :x="point.x" :y="point.y - 3" text-anchor="middle">{{ money(point.amount) }}</text><text :x="point.x" y="90" text-anchor="middle">{{ point.label }}</text></g>
        </svg>
      </section>
      <section class="finops-panel finops-composition-card">
        <div class="finops-card-title"><div><h2>费用构成</h2><p>{{ rangeLabel }}</p></div></div>
        <div v-if="providers.length" class="finops-composition">
          <div class="finops-donut" :style="donutStyle"><div><strong>¥{{ money(data.totalCost) }}</strong><span>费用总计</span></div></div>
          <div class="finops-provider-list"><div v-for="item in providers" :key="item.name"><i :style="{ background: item.color }"></i><b>{{ item.name }}</b><span>{{ item.percent.toFixed(1) }}%</span><em>¥{{ money(item.amount) }}</em></div></div>
        </div>
        <el-empty v-else description="当前范围暂无费用数据" />
      </section>
    </div>
  </div>
</template>

<style scoped>
.finops-dashboard-head { display:flex; align-items:center; justify-content:space-between; gap:20px; }
.finops-dashboard-head h2,.finops-card-title h2 { margin:0 0 5px; font-size:20px; }.finops-dashboard-head p,.finops-card-title p { margin:0; color:#7b8ba4; }
.finops-summary-cards{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}.finops-summary-card{display:flex;gap:15px;padding:22px 24px;border-radius:20px;background:#fff;box-shadow:0 10px 28px rgba(62,89,151,.09)}.finops-summary-card small{color:#7385a4}.finops-summary-card strong{display:block;margin:7px 0;font-size:25px}.finops-summary-card p{margin:0;color:#8b9ab2;font-size:13px}.finops-summary-icon{display:grid;place-items:center;width:52px;height:52px;border-radius:16px;color:#fff;font-size:25px;font-weight:800}.blue{background:#4169f5}.purple{background:#8147ed}.green{background:#16b980}.orange{background:#f2993e}.sync-value{font-size:16px!important;white-space:nowrap}.rise{color:#ef5b63}.fall{color:#18ae80}.finops-dashboard-grid{display:grid;grid-template-columns:1.42fr .95fr;gap:18px}.finops-trend-card,.finops-composition-card{min-height:320px;border:0;border-radius:24px;box-shadow:0 10px 28px rgba(62,89,151,.08)}.finops-card-title{display:flex;justify-content:space-between}.finops-card-title>span{color:#6e7e9a}.finops-chart-legend{display:flex;align-items:center;gap:7px;margin-top:24px;color:#6d7d98;font-size:13px}.finops-chart-legend i{width:20px;height:3px;background:#3d68f5}.finops-line-chart{width:100%;height:225px;margin-top:10px;overflow:visible}.finops-line-chart .grid{stroke:#e7edf7;stroke-width:.55}.finops-line-chart .area{fill:url(#none);fill:#3d68f5;opacity:.14}.finops-line-chart .line{fill:none;stroke:#3d68f5;stroke-width:1.25}.finops-line-chart circle{fill:#fff;stroke:#3d68f5;stroke-width:.9}.finops-line-chart text{fill:#70809e;font-size:3px}.finops-composition{display:flex;align-items:center;gap:25px;min-height:235px}.finops-donut{position:relative;width:180px;height:180px;border-radius:50%;flex:none}.finops-donut:after{content:'';position:absolute;inset:38px;border-radius:50%;background:#fff}.finops-donut>div{position:absolute;inset:0;z-index:1;display:grid;place-content:center;text-align:center}.finops-donut strong{font-size:19px}.finops-donut span{margin-top:5px;color:#7b8ba4}.finops-provider-list{display:grid;gap:13px;flex:1}.finops-provider-list>div{display:grid;grid-template-columns:10px auto 48px auto;align-items:center;gap:7px;font-size:13px}.finops-provider-list i{width:10px;height:10px;border-radius:50%}.finops-provider-list b{color:#162947}.finops-provider-list span{color:#152946;font-weight:700}.finops-provider-list em{color:#7b8ba4;font-style:normal;text-align:right;white-space:nowrap}@media(max-width:1100px){.finops-summary-cards{grid-template-columns:repeat(2,1fr)}.finops-dashboard-grid{grid-template-columns:1fr}}@media(max-width:680px){.finops-dashboard-head{align-items:flex-start;flex-direction:column}.finops-summary-cards{grid-template-columns:1fr}.finops-composition{flex-direction:column;align-items:flex-start}}
</style>
