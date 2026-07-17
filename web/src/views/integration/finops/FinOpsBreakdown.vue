<script setup>
import { computed, onMounted, ref } from 'vue'
import { queryFinOpsAccounts, queryFinOpsBreakdown, queryFinOpsLatestBreakdownMonth } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const accounts = ref([])
const accountId = ref('')
const month = ref(monthKey(new Date()))
const advancedDimension = ref('account')
const data = ref({ service: [], region: [], advanced: [] })
function monthKey(date) { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}` }
function normalizeMonth(value) { const match = String(value || '').match(/^(\d{4})-(\d{1,2})/); return match ? `${match[1]}-${match[2].padStart(2, '0')}` : monthKey(new Date()) }
const title = computed(() => { const [year, value] = normalizeMonth(month.value).split('-'); return `费用拆分分析 — ${year}年${Number(value)}月` })
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const totalCost = computed(() => data.value.service.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const maxValue = rows => Math.max(...rows.map(item => Number(item.amount || 0)), 1)
const paramsFor = (targetMonth, dimension) => ({ month: normalizeMonth(targetMonth), dimension, ...(accountId.value ? { account_id: accountId.value } : {}) })
const fetchDimension = (targetMonth, dimension) => queryFinOpsBreakdown(paramsFor(targetMonth, dimension))
async function load(targetMonth = month.value, knownService = null) {
  loading.value = true
  try {
    const current = normalizeMonth(targetMonth)
    const [service, region, advanced] = await Promise.all([knownService || fetchDimension(current, 'service'), fetchDimension(current, 'region'), fetchDimension(current, advancedDimension.value)])
    month.value = current
    data.value = { service: service || [], region: region || [], advanced: advanced || [] }
  } finally { loading.value = false }
}
async function loadDefaultMonth() {
  const current = monthKey(new Date())
  const currentService = await fetchDimension(current, 'service')
  if (currentService?.length) return load(current, currentService)
  const latest = await queryFinOpsLatestBreakdownMonth(accountId.value ? { account_id: accountId.value } : {})
  return load(latest?.month || current, latest?.month ? null : [])
}
onMounted(async () => { accounts.value = await queryFinOpsAccounts() || []; await loadDefaultMonth() })
</script>

<template>
  <div class="finops-page finops-breakdown" v-loading="loading">
    <section class="finops-panel finops-breakdown-head"><div><h2>{{ title }}</h2><p>所有统计均限定在所选自然月，展示汇总拆分结果。</p></div><div class="finops-filter"><el-select v-model="accountId" clearable placeholder="全部账号" style="width:180px" @change="loadDefaultMonth"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select><el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY-MM" :clearable="false" @change="load(month)"/><el-button type="primary" @click="load">查询</el-button></div></section>
    <section class="finops-breakdown-total"><span>当月费用总额</span><strong>¥ {{ money(totalCost) }}</strong><small>按产品与地域归集</small></section>
    <div class="finops-breakdown-grid">
      <section class="finops-panel finops-breakdown-card"><div class="finops-card-title"><div><h2>按云产品拆分</h2><p>本月主要产品成本</p></div></div><div v-if="data.service.length" class="finops-bars"><div v-for="item in data.service" :key="item.name" class="finops-bar-row"><span :title="item.name">{{ item.name || '未分类' }}</span><div class="finops-bar-track"><div class="finops-bar-fill" :style="{ width: `${Number(item.amount || 0) / maxValue(data.service) * 100}%` }"/></div><strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong></div></div><el-empty v-else description="该月暂无费用数据" /></section>
      <section class="finops-panel finops-breakdown-card"><div class="finops-card-title"><div><h2>按地域拆分</h2><p>本月区域成本分布</p></div></div><div v-if="data.region.length" class="finops-bars"><div v-for="item in data.region" :key="item.name" class="finops-bar-row"><span :title="item.name">{{ item.name || '未分类' }}</span><div class="finops-bar-track"><div class="finops-bar-fill region" :style="{ width: `${Number(item.amount || 0) / maxValue(data.region) * 100}%` }"/></div><strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong></div></div><el-empty v-else description="该月暂无地域账单数据" /></section>
    </div>
    <section class="finops-panel finops-breakdown-card"><div class="finops-card-title"><div><h2>高级维度拆分</h2><p>按账号、资源或标签查看本月归集</p></div><el-select v-model="advancedDimension" style="width:140px" @change="load(month)"><el-option label="按云账号" value="account"/><el-option label="按资源" value="resource"/><el-option label="按标签" value="tag"/></el-select></div><div v-if="data.advanced.length" class="finops-bars"><div v-for="item in data.advanced" :key="item.name" class="finops-bar-row"><span :title="item.name">{{ item.name || '未分类' }}</span><div class="finops-bar-track"><div class="finops-bar-fill advanced" :style="{ width: `${Number(item.amount || 0) / maxValue(data.advanced) * 100}%` }"/></div><strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong></div></div><el-empty v-else description="该维度暂无可归集数据" /></section>
  </div>
</template>

<style scoped>
.finops-breakdown-head{display:flex;justify-content:space-between;align-items:center;gap:20px}.finops-breakdown-head h2{margin:0 0 5px;font-size:21px}.finops-breakdown-head p,.finops-card-title p{margin:0;color:#7a8ba7}.finops-breakdown-total{display:grid;gap:5px;padding:24px 28px;border-radius:22px;background:linear-gradient(135deg,#edf3ff,#fff);border:1px solid #dbe5fc}.finops-breakdown-total span,.finops-breakdown-total small{color:#7083a1}.finops-breakdown-total strong{font-size:32px;color:#173667}.finops-breakdown-grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.finops-breakdown-card{border:0;border-radius:22px;box-shadow:0 10px 28px rgba(62,89,151,.07)}.finops-card-title{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:20px}.region{background:#7c55ee}.advanced{background:#1ab88a}@media(max-width:900px){.finops-breakdown-head{align-items:flex-start;flex-direction:column}.finops-breakdown-grid{grid-template-columns:1fr}}
</style>
