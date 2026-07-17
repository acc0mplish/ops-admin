<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { queryFinOpsAccounts, queryFinOpsResources } from '../../../api/integration'
import './finops.css'

const loading = ref(false), accounts = ref([]), rows = ref([]), accountId = ref(''), range = ref([]), regions = ref([]), resourceTypes = ref([]), selectedRegions = ref([]), selectedTypes = ref([])
const ready = computed(() => Boolean(accountId.value && range.value?.length === 2))
const money = value => Number(value || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
async function load() { if (!ready.value) { ElMessage.warning('请先选择云账号、开始日期和结束日期'); return } loading.value = true; try { const data = await queryFinOpsResources({ account_id: accountId.value, start: range.value[0], end: range.value[1], region: selectedRegions.value.join(','), resource_type: selectedTypes.value.join(',') }); rows.value = data?.items || []; regions.value = data?.regions || []; resourceTypes.value = data?.resourceTypes || [] } finally { loading.value = false } }
function resetFilters() { selectedRegions.value = []; selectedTypes.value = []; rows.value = []; regions.value = []; resourceTypes.value = [] }
onMounted(async () => { accounts.value = await queryFinOpsAccounts() || [] })
</script>

<template>
  <div class="finops-page finops-resources" v-loading="loading">
    <section class="finops-panel finops-resource-head"><div><h2>资源拆分</h2><p>选择云账号和日期范围后，按资源归集账单，并可继续按地域、资源类型筛选。</p></div></section>
    <section class="finops-panel finops-resource-filter"><div class="finops-filter"><el-select v-model="accountId" placeholder="请选择云账号" style="width:210px" @change="resetFilters"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select><el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期"/><el-select v-model="selectedRegions" multiple collapse-tags placeholder="全部地域" style="width:180px" :disabled="!ready"><el-option v-for="item in regions" :key="item" :label="item" :value="item" /></el-select><el-select v-model="selectedTypes" multiple collapse-tags placeholder="全部资源类型" style="width:200px" :disabled="!ready"><el-option v-for="item in resourceTypes" :key="item" :label="item" :value="item" /></el-select><el-button type="primary" :disabled="!ready" @click="load">开始拆分</el-button></div><p v-if="!ready" class="finops-filter-hint">请先完成云账号和日期范围选择，系统不会自动查询资源数据。</p></section>
    <section class="finops-panel finops-resource-table"><div class="finops-card-title"><div><h2>资源费用明细</h2><p>共 {{ rows.length }} 个资源</p></div></div><el-table v-if="rows.length" :data="rows" stripe><el-table-column prop="resourceName" label="资源名称" min-width="200" show-overflow-tooltip><template #default="{row}">{{ row.resourceName || row.resourceId || '未关联资源' }}</template></el-table-column><el-table-column prop="resourceId" label="资源 ID" min-width="190" show-overflow-tooltip/><el-table-column prop="resourceType" label="资源类型" min-width="150"><template #default="{row}">{{ row.resourceType || '未分类' }}</template></el-table-column><el-table-column prop="provider" label="云厂商" width="110"/><el-table-column prop="region" label="地域" width="150"><template #default="{row}">{{ row.region || '未提供' }}</template></el-table-column><el-table-column label="费用" width="150" align="right"><template #default="{row}"><b class="finops-money">¥ {{ money(row.cost) }}</b></template></el-table-column><el-table-column prop="recordCount" label="账单条目" width="105" align="center"/></el-table><el-empty v-else :description="ready ? '点击“开始拆分”查询资源费用' : '请选择云账号和日期范围'" /></section>
  </div>
</template>

<style scoped>
.finops-resource-head h2{margin:0 0 6px;font-size:22px}.finops-resource-head p,.finops-card-title p{margin:0;color:#7a8ba7}.finops-resource-filter,.finops-resource-table{border:0;border-radius:22px;box-shadow:0 10px 28px rgba(62,89,151,.07)}.finops-filter-hint{margin:15px 0 0;color:#8392aa;font-size:13px}.finops-card-title{display:flex;justify-content:space-between;margin-bottom:20px}
</style>
