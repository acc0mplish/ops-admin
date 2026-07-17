<script setup>
import { computed, onMounted, ref } from 'vue'
import { queryFinOpsAccounts, queryFinOpsBreakdown, queryFinOpsLatestBreakdownMonth } from '../../../api/integration'
import './finops.css'

const loading = ref(false); const accounts = ref([]); const accountId = ref(''); const month = ref(monthKey(new Date())); const data = ref({ service: [], region: [], detail: [] })
function monthKey(date) { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}` }
function normalizeMonth(value) { const matched = String(value || '').match(/^(\d{4})-(\d{1,2})/); return matched ? `${matched[1]}-${matched[2].padStart(2, '0')}` : monthKey(new Date()) }
const title = computed(() => { const [year, value] = normalizeMonth(month.value).split('-'); return `费用拆分分析 — ${year}年${Number(value)}月` })
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const totalCost = computed(() => data.value.service.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const maxValue = rows => Math.max(...rows.map(item => Number(item.amount || 0)), 1)
const paramsFor = (targetMonth, dimension) => ({ month: normalizeMonth(targetMonth), dimension, ...(accountId.value ? { account_id: accountId.value } : {}) })
const fetchDimension = (targetMonth, dimension) => queryFinOpsBreakdown(paramsFor(targetMonth, dimension))
async function load(targetMonth = month.value, knownService = null) { loading.value = true; try { const current = normalizeMonth(targetMonth); const [service, region, detail] = await Promise.all([knownService || fetchDimension(current, 'service'), fetchDimension(current, 'region'), fetchDimension(current, 'detail')]); month.value = current; data.value = { service: service || [], region: region || [], detail: detail || [] } } finally { loading.value = false } }
async function loadDefaultMonth() { const current = monthKey(new Date()); const currentService = await fetchDimension(current, 'service'); if (currentService?.length) return load(current, currentService); const latest = await queryFinOpsLatestBreakdownMonth(accountId.value ? { account_id: accountId.value } : {}); return load(latest?.month || current, latest?.month ? null : []) }
onMounted(async () => { accounts.value = await queryFinOpsAccounts() || []; await loadDefaultMonth() })
</script>

<template>
  <div class="finops-page finops-breakdown" v-loading="loading">
    <section class="finops-panel finops-breakdown-head"><div><h2>{{ title }}</h2><p>所有统计均限定在所选自然月；费用明细按日展示。</p></div><div class="finops-filter"><el-select v-model="accountId" clearable placeholder="全部账号" style="width:180px" @change="loadDefaultMonth"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select><el-date-picker v-model="month" type="month" value-format="YYYY-MM" format="YYYY-MM" :clearable="false" @change="load(month)"/><el-button type="primary" @click="load">查询</el-button></div></section>
    <section class="finops-breakdown-total"><span>当月费用总额</span><strong>¥ {{ money(totalCost) }}</strong><small>按产品与地域归集</small></section>
    <div class="finops-breakdown-grid">
      <section class="finops-panel finops-breakdown-card"><div class="finops-card-title"><div><h2>按云产品拆分</h2><p>本月主要产品成本</p></div></div><div v-if="data.service.length" class="finops-bars"><div v-for="item in data.service" :key="item.name" class="finops-bar-row"><span :title="item.name">{{ item.name || '未分类' }}</span><div class="finops-bar-track"><div class="finops-bar-fill" :style="{ width: `${Number(item.amount || 0) / maxValue(data.service) * 100}%` }"/></div><strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong></div></div><el-empty v-else description="该月暂无费用数据" /></section>
      <section class="finops-panel finops-breakdown-card"><div class="finops-card-title"><div><h2>按地域拆分</h2><p>本月区域成本分布</p></div></div><div v-if="data.region.length" class="finops-bars"><div v-for="item in data.region" :key="item.name" class="finops-bar-row"><span :title="item.name">{{ item.name || '未分类' }}</span><div class="finops-bar-track"><div class="finops-bar-fill region" :style="{ width: `${Number(item.amount || 0) / maxValue(data.region) * 100}%` }"/></div><strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong></div></div><el-empty v-else description="该月暂无地域账单数据" /></section>
    </div>
    <section class="finops-panel finops-detail-card"><div class="finops-card-title"><div><h2>费用明细</h2><p>按天费用账单；新同步账单将带回地域、实例和实例 ID。</p></div><el-tag type="info">{{ data.detail.length }} 条</el-tag></div><el-table :data="data.detail" size="small" max-height="460" stripe><el-table-column prop="billingDate" label="账单日期" width="130"/><el-table-column prop="service" label="云产品" min-width="190"/><el-table-column prop="region" label="地域" min-width="140"><template #default="{row}">{{ row.region || '未提供' }}</template></el-table-column><el-table-column prop="resourceName" label="实例" min-width="190" show-overflow-tooltip><template #default="{row}">{{ row.resourceName || '未提供' }}</template></el-table-column><el-table-column prop="resourceId" label="实例 ID" min-width="190" show-overflow-tooltip><template #default="{row}">{{ row.resourceId || '未提供' }}</template></el-table-column><el-table-column label="费用" width="160" align="right"><template #default="{row}"><b class="finops-money">{{ row.currency || 'CNY' }} {{ money(row.amount) }}</b></template></el-table-column></el-table><el-empty v-if="!data.detail.length" description="该月暂无费用明细" /></section>
  </div>
</template>

<style scoped>
.finops-breakdown-head{display:flex;justify-content:space-between;align-items:center;gap:20px}.finops-breakdown-head h2{margin:0 0 5px;font-size:21px}.finops-breakdown-head p,.finops-card-title p{margin:0;color:#7a8ba7}.finops-breakdown-total{display:grid;gap:5px;padding:24px 28px;border-radius:22px;background:linear-gradient(135deg,#edf3ff,#fff);border:1px solid #dbe5fc}.finops-breakdown-total span,.finops-breakdown-total small{color:#7083a1}.finops-breakdown-total strong{font-size:32px;color:#173667}.finops-breakdown-grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.finops-breakdown-card,.finops-detail-card{border:0;border-radius:22px;box-shadow:0 10px 28px rgba(62,89,151,.07)}.finops-card-title{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:20px}.region{background:#7c55ee}.finops-detail-card{padding-bottom:10px}@media(max-width:900px){.finops-breakdown-head{align-items:flex-start;flex-direction:column}.finops-breakdown-grid{grid-template-columns:1fr}}
</style>
