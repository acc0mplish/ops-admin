<script setup>
import { onMounted, ref } from 'vue'
import FinOpsHeader from './FinOpsHeader.vue'
import { queryFinOpsAccounts, queryFinOpsResources } from '../../../api/integration'
import './finops.css'

const loading = ref(false)
const accounts = ref([])
const rows = ref([])
const accountId = ref('')
const range = ref([])

const money = value => Number(value || 0).toLocaleString('zh-CN', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2
})

async function loadAccounts() {
  accounts.value = await queryFinOpsAccounts() || []
}

async function load() {
  loading.value = true
  try {
    const params = {}
    if (accountId.value) params.accountId = accountId.value
    if (range.value?.length) Object.assign(params, { start: range.value[0], end: range.value[1] })
    rows.value = await queryFinOpsResources(params) || []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadAccounts()
  await load()
})
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section class="finops-panel" v-loading="loading">
      <div class="finops-head">
        <div>
          <h2>资源拆分</h2>
          <p>查看每个云资源的账单数量、累计费用与所属范围。</p>
        </div>
        <div class="finops-filter">
          <el-select v-model="accountId" clearable placeholder="全部云账号" style="width: 180px">
            <el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
          <el-button type="primary" @click="load">查询</el-button>
        </div>
      </div>
      <el-table :data="rows" stripe>
        <el-table-column prop="resourceName" label="资源名称" min-width="200" show-overflow-tooltip />
        <el-table-column prop="resourceId" label="资源 ID" min-width="180" show-overflow-tooltip />
        <el-table-column prop="resourceType" label="资源类型" width="150" />
        <el-table-column prop="provider" label="云厂商" width="115" />
        <el-table-column prop="accountName" label="云账号" min-width="150" />
        <el-table-column prop="region" label="区域" width="140" />
        <el-table-column label="费用" width="140">
          <template #default="{ row }"><b class="finops-money">￥{{ money(row.cost) }}</b></template>
        </el-table-column>
        <el-table-column prop="recordCount" label="账单条目" width="100" />
      </el-table>
      <el-empty v-if="!rows.length" description="当前筛选条件暂无资源费用" />
    </section>
  </div>
</template>
