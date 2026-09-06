<template>
  <div class="infra-overview">
    <div class="page-header">
      <div><h2 class="page-title">{{ inft('overviewTitle') }}</h2><p class="page-desc">{{ inft('overviewDesc') }}</p></div>
      <el-button @click="loadData">{{ inft('refresh') }}</el-button>
    </div>

    <el-row :gutter="16" class="stat-row">
      <el-col :span="6"><el-card shadow="never"><div class="stat-value">{{ providerTypes.length }}</div><div class="stat-label">{{ inft('statProviderTypes') }}</div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat-value">{{ connections.length }}</div><div class="stat-label">{{ inft('statConnections') }}</div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat-value">{{ activeCount }}</div><div class="stat-label">{{ inft('statActiveConnections') }}</div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat-value">{{ resourceTotal }}</div><div class="stat-label">{{ inft('statResources') }}</div></el-card></el-col>
    </el-row>

    <el-card shadow="never" class="section-card">
      <template #header><span>{{ inft('typesTitle') }}</span></template>
      <el-table :data="providerTypes" size="small">
        <el-table-column prop="type" :label="inft('typeCol')" min-width="140" />
        <el-table-column prop="adapterVersion" :label="inft('adapterVersion')" width="130" />
        <el-table-column prop="protocolVersion" :label="inft('protocolVersion')" width="140" />
        <el-table-column :label="inft('contextKinds')" min-width="160">
          <template #default="{ row }">{{ (row.contextKinds || []).join(', ') }}</template>
        </el-table-column>
        <el-table-column :label="inft('builtin')" width="100">
          <template #default="{ row }">{{ row.builtIn ? inft('builtinYes') : inft('builtinNo') }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>{{ inft('connectionsTitle') }}</span></template>
      <el-table :data="connections" size="small">
        <el-table-column prop="name" :label="inft('connectionName')" min-width="160" />
        <el-table-column prop="providerType" :label="inft('typeCol')" width="130" />
        <el-table-column prop="endpoint" :label="inft('endpointCol')" min-width="200" show-overflow-tooltip />
        <el-table-column :label="inft('statusCol')" width="110">
          <template #default="{ row }"><el-tag :type="statusTag(row.status).type" size="small">{{ statusTag(row.status).label }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="version" :label="inft('versionCol')" width="110" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listInfraProviderConnections, listInfraProviderTypes, listInfraResources } from '../../api/infra'
import { inft } from '../../utils/infra-i18n'

const providerTypes = ref([])
const connections = ref([])
const resourceTotal = ref(0)

const activeCount = computed(() => connections.value.filter((item) => item.status === 'active').length)

function statusTag(status) {
  const map = { active: { key: 'statusActive', type: 'success' }, inactive: { key: 'statusInactive', type: 'info' } }
  const entry = map[status] || { key: 'statusUnknown', type: 'info' }
  return { label: inft(entry.key), type: entry.type }
}

async function loadData() {
  try {
    const [types, conns, resources] = await Promise.all([
      listInfraProviderTypes(),
      listInfraProviderConnections(),
      listInfraResources({ page: 1, pageSize: 1 })
    ])
    providerTypes.value = types?.data?.items || []
    connections.value = conns?.data?.items || []
    resourceTotal.value = resources?.data?.total || 0
  } catch (error) {
    ElMessage.error(inft('loadFailed'))
  }
}

onMounted(loadData)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; }
.page-desc { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.stat-row { margin-bottom: 16px; }
.stat-value { font-size: 26px; font-weight: 600; }
.stat-label { color: var(--el-text-color-secondary); font-size: 13px; margin-top: 4px; }
.section-card { margin-bottom: 16px; }
</style>
