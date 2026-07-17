<script setup>
import { computed, onMounted, ref } from 'vue'
import FinOpsHeader from './FinOpsHeader.vue'
import { queryFinOpsAccounts, queryFinOpsBreakdown } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const accounts = ref([])
const rows = ref([])
const range = ref([])
const accountId = ref('')
const dimension = ref('service')
const dimensions = [
  { value: 'provider', label: '云厂商' }, { value: 'account', label: '云账号' },
  { value: 'region', label: '区域' }, { value: 'service', label: '云服务' },
  { value: 'resource', label: '资源' }, { value: 'tag', label: '标签' }
]
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const maxValue = computed(() => Math.max(...rows.value.map(item => Number(item.amount || 0)), 1))

async function loadAccounts() { accounts.value = await queryFinOpsAccounts() || [] }
async function load() {
  loading.value = true
  try {
    const params = { dimension: dimension.value }
    if (accountId.value) params.accountId = accountId.value
    if (range.value?.length) Object.assign(params, { start: range.value[0], end: range.value[1] })
    rows.value = await queryFinOpsBreakdown(params) || []
  } finally { loading.value = false }
}
onMounted(async () => { await loadAccounts(); await load() })
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section v-loading="loading" class="finops-panel">
      <div class="finops-head">
        <div><h2>费用拆分</h2><p>按云厂商、账号、区域、服务、资源或标签定位主要成本来源。</p></div>
        <div class="finops-filter">
          <el-select v-model="dimension" style="width:130px"><el-option v-for="item in dimensions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
          <el-select v-model="accountId" clearable placeholder="全部账号" style="width:180px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
          <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
          <el-button type="primary" @click="load">查询</el-button>
        </div>
      </div>
      <div v-if="rows.length" class="finops-bars">
        <div v-for="item in rows" :key="item.name" class="finops-bar-row">
          <span :title="item.name">{{ item.name }}</span>
          <div class="finops-bar-track"><div class="finops-bar-fill" :style="{ width: `${Number(item.amount || 0) / maxValue * 100}%` }" /></div>
          <strong>¥ {{ money(item.amount) }} <small>{{ Number(item.percent || 0).toFixed(1) }}%</small></strong>
        </div>
      </div>
      <el-empty v-else description="当前筛选条件暂无费用明细" />
    </section>
  </div>
</template>
