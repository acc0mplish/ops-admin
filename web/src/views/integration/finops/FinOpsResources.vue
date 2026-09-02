<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { queryFinOpsAccounts, queryFinOpsResources } from '../../../api/integration'
import { ft } from '../../../utils/finops-i18n'
import './finops.css'

const loading = ref(false)
const accounts = ref([])
const rows = ref([])
const accountId = ref('')
const range = ref([])
const regions = ref([])
const resourceTypes = ref([])
const selectedRegions = ref([])
const selectedTypes = ref([])
const ready = computed(() => Boolean(accountId.value && range.value?.length === 2))
const money = value => Number(value || 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

async function load() {
  if (!ready.value) {
    ElMessage.warning(ft('selectAccountRange'))
    return
  }
  const params = { account_id: accountId.value, start: range.value[0], end: range.value[1] }
  if (selectedRegions.value.length) params.region = selectedRegions.value.join(',')
  if (selectedTypes.value.length) params.resource_type = selectedTypes.value.join(',')
  loading.value = true
  try {
    const data = await queryFinOpsResources(params)
    rows.value = data?.items || []
    regions.value = data?.regions || []
    resourceTypes.value = data?.resourceTypes || []
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  selectedRegions.value = []
  selectedTypes.value = []
  rows.value = []
  regions.value = []
  resourceTypes.value = []
}

watch([accountId, range], () => { resetFilters(); if (ready.value) load() }, { deep: true })
watch([selectedRegions, selectedTypes], () => { if (ready.value && (regions.value.length || resourceTypes.value.length)) load() }, { deep: true })
onMounted(async () => { accounts.value = await queryFinOpsAccounts() || [] })
</script>

<template>
  <div class="finops-page finops-resources" v-loading="loading">
    <section class="finops-panel finops-resource-head"><h2>{{ ft('resourceBreakdown') }}</h2><p>{{ ft('resourceBreakdownDesc') }}</p></section>
    <section class="finops-panel finops-resource-filter">
      <div class="finops-filter">
        <el-select v-model="accountId" :placeholder="ft('selectCloudAccount')" style="width:220px"><el-option v-for="item in accounts" :key="item.id" :label="item.name" :value="item.id" /></el-select>
        <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" :start-placeholder="ft('startDate')" :end-placeholder="ft('endDate')" />
        <el-button type="primary" :disabled="!ready" @click="load">{{ ft('startBreakdown') }}</el-button>
      </div>
      <p v-if="!ready" class="finops-filter-hint">{{ ft('filterHint') }}</p>
    </section>
    <section class="finops-panel finops-resource-table">
      <div class="finops-card-title">
        <div><h2>{{ ft('resourceCostDetail') }}</h2><p>{{ ft('resourceCount', { count: rows.length }) }}</p></div>
        <div class="finops-filter finops-detail-filters">
          <el-select v-model="selectedRegions" multiple collapse-tags clearable :placeholder="ft('allRegions')" style="width:180px" :disabled="!ready"><el-option v-for="item in regions" :key="item" :label="item" :value="item" /></el-select>
          <el-select v-model="selectedTypes" multiple collapse-tags clearable :placeholder="ft('allResourceTypes')" style="width:200px" :disabled="!ready"><el-option v-for="item in resourceTypes" :key="item" :label="item" :value="item" /></el-select>
        </div>
      </div>
      <el-table v-if="rows.length" :data="rows" stripe>
        <el-table-column prop="resourceName" :label="ft('resourceName')" min-width="200" show-overflow-tooltip><template #default="{ row }">{{ row.resourceName || row.resourceId || ft('unlinkedResource') }}</template></el-table-column>
        <el-table-column prop="resourceId" label="Resource ID" min-width="190" show-overflow-tooltip />
        <el-table-column prop="resourceType" :label="ft('resourceType')" min-width="150"><template #default="{ row }">{{ row.resourceType || ft('uncategorized') }}</template></el-table-column>
        <el-table-column prop="resourceConfig" :label="ft('resourceConfig')" min-width="170" show-overflow-tooltip><template #default="{ row }">{{ row.resourceConfig || '-' }}</template></el-table-column>
        <el-table-column prop="provider" :label="ft('cloudProvider')" width="110" />
        <el-table-column prop="region" :label="ft('region')" width="150"><template #default="{ row }">{{ row.region || ft('unavailable') }}</template></el-table-column>
        <el-table-column :label="ft('originalPrice')" width="135" align="right"><template #default="{ row }">¥ {{ money(row.originalPrice) }}</template></el-table-column>
        <el-table-column :label="ft('discount')" width="135" align="right"><template #default="{ row }"><span class="finops-discount">- ¥ {{ money(row.discount) }}</span></template></el-table-column>
        <el-table-column :label="ft('actualPayment')" width="145" align="right"><template #default="{ row }"><b class="finops-money">¥ {{ money(row.actualPayment ?? row.cost) }}</b></template></el-table-column>
        <el-table-column prop="recordCount" :label="ft('billingEntries')" width="105" align="center" />
      </el-table>
      <el-empty v-else :description="ready ? ft('noResourceCost') : ft('selectAccountRange')" />
    </section>
  </div>
</template>

<style scoped>
.finops-discount{color:#16a36f;font-weight:600}.finops-resource-head h2{margin:0 0 6px;font-size:22px}.finops-resource-head p,.finops-card-title p{margin:0;color:#7a8ba7}.finops-resource-filter,.finops-resource-table{border:0;border-radius:22px;box-shadow:0 10px 28px rgba(62,89,151,.07)}.finops-filter-hint{margin:15px 0 0;color:#8392aa;font-size:13px}.finops-card-title{display:flex;justify-content:space-between;gap:18px;align-items:flex-start;margin-bottom:20px}.finops-detail-filters{justify-content:flex-end}@media(max-width:900px){.finops-card-title{flex-direction:column}.finops-detail-filters{justify-content:flex-start}}
</style>
