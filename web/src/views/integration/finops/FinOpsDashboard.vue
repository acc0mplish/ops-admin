<script setup>
import { computed, onMounted, ref } from 'vue'
import FinOpsHeader from './FinOpsHeader.vue'
import { queryFinOpsDashboard } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const data = ref({ trend: [], providerDistribution: [] })

function currentMonth() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

const month = ref(currentMonth())
const params = computed(() => ({ month: month.value || currentMonth() }))
const monthLabel = computed(() => {
  const [year, value] = params.value.month.split('-')
  return `${year} 年 ${Number(value)} 月`
})
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const maxTrend = computed(() => Math.max(...(data.value.trend || []).map(item => Number(item.amount)), 1))
const maxProvider = computed(() => Math.max(...(data.value.providerDistribution || []).map(item => Number(item.amount)), 1))

async function load() {
  loading.value = true
  try {
    data.value = await queryFinOpsDashboard(params.value) || { trend: [], providerDistribution: [] }
  } finally {
    loading.value = false
  }
}

function changeMonth() {
  if (month.value) load()
}

onMounted(load)
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section v-loading="loading" class="finops-panel">
      <div class="finops-head">
        <div>
          <h2>费用看板</h2>
          <p>按账单月汇总多云支出，并查看选定月份内的每日费用变化。</p>
        </div>
        <div class="finops-actions">
          <span class="finops-period-label">账单月份</span>
          <el-date-picker
            v-model="month"
            type="month"
            value-format="YYYY-MM"
            format="YYYY 年 MM 月"
            placeholder="选择账单月份"
            :clearable="false"
            @change="changeMonth"
          />
          <el-button type="primary" @click="load">刷新</el-button>
        </div>
      </div>
      <div class="finops-metrics">
        <div class="finops-metric"><span>{{ monthLabel }}总费用</span><strong>¥ {{ money(data.totalCost) }}</strong></div>
        <div class="finops-metric"><span>云账号</span><strong>{{ data.accountCount || 0 }}</strong></div>
        <div class="finops-metric"><span>账单明细</span><strong>{{ data.recordCount || 0 }}</strong></div>
        <div class="finops-metric"><span>预计可节省</span><strong>¥ {{ money(data.estimatedSaving) }}</strong></div>
      </div>
    </section>
    <div class="finops-grid-2">
      <section class="finops-panel">
        <div class="finops-head">
          <div><h2>月内费用趋势</h2><p>{{ monthLabel }}按账单日期聚合</p></div>
        </div>
        <div v-if="data.trend?.length" class="finops-bars">
          <div v-for="item in data.trend" :key="item.date" class="finops-bar-row">
            <span>{{ item.date }}</span>
            <div class="finops-bar-track"><div class="finops-bar-fill" :style="{ width: `${item.amount / maxTrend * 100}%` }" /></div>
            <strong>¥ {{ money(item.amount) }}</strong>
          </div>
        </div>
        <el-empty v-else :description="`${monthLabel}暂无账单数据`" />
      </section>
      <section class="finops-panel">
        <div class="finops-head">
          <div><h2>云厂商分布</h2><p>{{ monthLabel }}多云费用对比</p></div>
        </div>
        <div v-if="data.providerDistribution?.length" class="finops-bars">
          <div v-for="item in data.providerDistribution" :key="item.name" class="finops-bar-row">
            <span class="finops-provider">{{ item.name }}</span>
            <div class="finops-bar-track"><div class="finops-bar-fill" :style="{ width: `${item.amount / maxProvider * 100}%` }" /></div>
            <strong>¥ {{ money(item.amount) }}</strong>
          </div>
        </div>
        <el-empty v-else :description="`${monthLabel}暂无云厂商费用`" />
      </section>
    </div>
  </div>
</template>

<style scoped>
.finops-period-label {
  color: #64748b;
  font-size: 13px;
  white-space: nowrap;
}
</style>
